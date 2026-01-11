package auth

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"time"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/secrets"
)

//go:embed templates/setup.html
var setupPageTemplate string

//go:embed templates/setup_success.html
var setupSuccessTemplate string

// SetupResult contains the result of a browser-based setup
type SetupResult struct {
	GraphName string
	Token     string
	Mode      string // "cloud" or "encrypted"
	Error     error
}

// SetupServer handles the browser-based authentication setup flow
type SetupServer struct {
	result        chan SetupResult
	shutdown      chan struct{}
	pendingResult *SetupResult
	csrfToken     string
	profile       string
	saveFunc      func(profile string, graphName string, token secrets.Token) error
}

// SetupServerOption configures the setup server
type SetupServerOption func(*SetupServer)

// WithSaveFunc sets a custom save function for testing
func WithSaveFunc(fn func(profile string, graphName string, token secrets.Token) error) SetupServerOption {
	return func(s *SetupServer) {
		s.saveFunc = fn
	}
}

// NewSetupServer creates a new setup server
func NewSetupServer(profile string, opts ...SetupServerOption) (*SetupServer, error) {
	// Generate CSRF token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate CSRF token: %w", err)
	}

	s := &SetupServer{
		result:    make(chan SetupResult, 1),
		shutdown:  make(chan struct{}),
		csrfToken: hex.EncodeToString(tokenBytes),
		profile:   profile,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Start starts the setup server and opens the browser
func (s *SetupServer) Start(ctx context.Context) (*SetupResult, error) {
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleSetup)
	mux.HandleFunc("/validate", s.handleValidate)
	mux.HandleFunc("/submit", s.handleSubmit)
	mux.HandleFunc("/success", s.handleSuccess)
	mux.HandleFunc("/complete", s.handleComplete)

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server in background
	go func() {
		_ = server.Serve(listener)
	}()

	// Print URL first so user can open manually if needed
	fmt.Printf("Open this URL in your browser to authenticate:\n  %s\n", baseURL)
	fmt.Println("Attempting to open browser automatically...")
	if err := OpenBrowser(baseURL); err != nil {
		fmt.Printf("Could not open browser automatically: %v\n", err)
		fmt.Println("Please open the URL manually in your browser.")
	}

	// Wait for result or context cancellation
	select {
	case result := <-s.result:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close() // Force close if graceful shutdown fails
		}
		return &result, nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
		}
		return nil, ctx.Err()
	case <-s.shutdown:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
		}
		if s.pendingResult != nil {
			return s.pendingResult, nil
		}
		return nil, fmt.Errorf("setup canceled")
	}
}

// handleSetup serves the main setup page
func (s *SetupServer) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tmpl, err := template.New("setup").Parse(setupPageTemplate)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]string{
		"CSRFToken": s.csrfToken,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, data)
}

// handleValidate tests credentials without saving
func (s *SetupServer) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify CSRF token
	if r.Header.Get("X-CSRF-Token") != s.csrfToken {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		GraphName      string `json:"graph_name"`
		APIToken       string `json:"api_token"`
		EncryptedGraph bool   `json:"encrypted_graph"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSetupJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Determine mode and create client
	mode := secrets.ModeCloud
	if req.EncryptedGraph {
		mode = secrets.ModeEncrypted
	}

	var roamClient api.RoamAPI
	var clientErr error

	if mode == secrets.ModeEncrypted {
		roamClient, clientErr = api.NewLocalClient(req.GraphName)
		if clientErr != nil {
			writeSetupJSON(w, http.StatusOK, map[string]any{
				"success": false,
				"error":   "Roam desktop app not running. Please start it and enable API access.",
			})
			return
		}
	} else {
		roamClient = api.NewClient(req.GraphName, req.APIToken)
	}

	if clientErr != nil {
		writeSetupJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Failed to create client: %v", clientErr),
		})
		return
	}

	// Test the connection with a simple query
	_, err := roamClient.Query("[:find ?e :where [?e :node/title] :limit 1]")
	if err != nil {
		writeSetupJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}

	writeSetupJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"message":    "Connection successful!",
		"graph_name": req.GraphName,
	})
}

// handleSubmit saves credentials after validation
func (s *SetupServer) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify CSRF token
	if r.Header.Get("X-CSRF-Token") != s.csrfToken {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		GraphName      string `json:"graph_name"`
		APIToken       string `json:"api_token"`
		EncryptedGraph bool   `json:"encrypted_graph"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSetupJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Determine mode
	mode := secrets.ModeCloud
	modeLabel := "cloud"
	if req.EncryptedGraph {
		mode = secrets.ModeEncrypted
		modeLabel = "encrypted"
	}

	// Validate first
	var roamClient api.RoamAPI
	var clientErr error

	if mode == secrets.ModeEncrypted {
		roamClient, clientErr = api.NewLocalClient(req.GraphName)
		if clientErr != nil {
			writeSetupJSON(w, http.StatusOK, map[string]any{
				"success": false,
				"error":   "Roam desktop app not running. Please start it and enable API access.",
			})
			return
		}
	} else {
		roamClient = api.NewClient(req.GraphName, req.APIToken)
	}

	if clientErr != nil {
		writeSetupJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Failed to create client: %v", clientErr),
		})
		return
	}

	// Test connection
	_, err := roamClient.Query("[:find ?e :where [?e :node/title] :limit 1]")
	if err != nil {
		writeSetupJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}

	// Save credentials if saveFunc is provided
	if s.saveFunc != nil {
		// For encrypted graphs, use a placeholder token since no API token is needed
		tokenValue := req.APIToken
		if mode == secrets.ModeEncrypted {
			tokenValue = "local" // Placeholder - local API doesn't need a token
		}
		tok := secrets.Token{
			Profile:      s.profile,
			RefreshToken: tokenValue,
			Mode:         mode,
			CreatedAt:    time.Now().UTC(),
		}
		if err := s.saveFunc(s.profile, req.GraphName, tok); err != nil {
			writeSetupJSON(w, http.StatusOK, map[string]any{
				"success": false,
				"error":   fmt.Sprintf("Failed to save credentials: %v", err),
			})
			return
		}
	}

	// Store pending result
	s.pendingResult = &SetupResult{
		GraphName: req.GraphName,
		Token:     req.APIToken,
		Mode:      mode,
	}

	writeSetupJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"graph_name": req.GraphName,
		"mode":       modeLabel,
	})
}

// handleSuccess serves the success page
func (s *SetupServer) handleSuccess(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.New("success").Parse(setupSuccessTemplate)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]string{
		"GraphName": r.URL.Query().Get("graph"),
		"Mode":      r.URL.Query().Get("mode"),
		"CSRFToken": s.csrfToken,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, data)
}

// handleComplete signals that setup is done
func (s *SetupServer) handleComplete(w http.ResponseWriter, r *http.Request) {
	// Verify CSRF token for POST requests
	if r.Method == http.MethodPost {
		if r.Header.Get("X-CSRF-Token") != s.csrfToken {
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}
	}

	if s.pendingResult != nil {
		s.result <- *s.pendingResult
	}
	close(s.shutdown)
	writeSetupJSON(w, http.StatusOK, map[string]any{"success": true})
}

// writeSetupJSON writes a JSON response
func writeSetupJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
