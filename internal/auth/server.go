package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

const (
	// DefaultTimeout is the default timeout for the OAuth flow
	DefaultTimeout = 5 * time.Minute

	// PostSuccessDisplaySeconds is how long the success page displays
	PostSuccessDisplaySeconds = 30
)

var (
	errAuthorization = errors.New("authorization error")
	errMissingCode   = errors.New("missing authorization code")
	errStateMismatch = errors.New("state mismatch - possible CSRF attack")
	errTimeout       = errors.New("authentication timeout")
)

// AuthResult contains the result of a successful OAuth flow
type AuthResult struct {
	Code  string
	State string
}

// AuthServer handles the local OAuth callback server
type AuthServer struct {
	listener net.Listener
	server   *http.Server
	state    string
	codeCh   chan string
	errCh    chan error
	timeout  time.Duration
}

// AuthServerOption configures the auth server
type AuthServerOption func(*AuthServer)

// WithTimeout sets the authentication timeout
func WithTimeout(d time.Duration) AuthServerOption {
	return func(s *AuthServer) {
		s.timeout = d
	}
}

// NewAuthServer creates a new OAuth callback server on a random available port
func NewAuthServer(opts ...AuthServerOption) (*AuthServer, error) {
	// Generate CSRF state token
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	// Listen on random available port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for callback: %w", err)
	}

	s := &AuthServer{
		listener: ln,
		state:    state,
		codeCh:   make(chan string, 1),
		errCh:    make(chan error, 1),
		timeout:  DefaultTimeout,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Port returns the port the server is listening on
func (s *AuthServer) Port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

// State returns the CSRF state token
func (s *AuthServer) State() string {
	return s.state
}

// RedirectURL returns the OAuth redirect URL
func (s *AuthServer) RedirectURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/oauth2/callback", s.Port())
}

// Start starts the HTTP server and begins handling callbacks
func (s *AuthServer) Start(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/callback", s.handleCallback)
	mux.HandleFunc("/", s.handleRoot)

	s.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()

	// Start server
	go func() {
		if err := s.server.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case s.errCh <- err:
			default:
			}
		}
	}()
}

// WaitForAuth waits for the OAuth callback and returns the authorization code
func (s *AuthServer) WaitForAuth(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	select {
	case code := <-s.codeCh:
		return code, nil
	case err := <-s.errCh:
		return "", err
	case <-ctx.Done():
		return "", fmt.Errorf("%w: %v", errTimeout, ctx.Err())
	}
}

// Close shuts down the server gracefully
func (s *AuthServer) Close() error {
	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}

// handleCallback processes the OAuth redirect
func (s *AuthServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Check for errors from OAuth provider
	if errParam := q.Get("error"); errParam != "" {
		errDesc := q.Get("error_description")
		if errDesc == "" {
			errDesc = errParam
		}
		select {
		case s.errCh <- fmt.Errorf("%w: %s", errAuthorization, errDesc):
		default:
		}
		w.WriteHeader(http.StatusOK)
		renderErrorPage(w, errDesc)
		return
	}

	// Validate CSRF state
	if q.Get("state") != s.state {
		select {
		case s.errCh <- errStateMismatch:
		default:
		}
		w.WriteHeader(http.StatusBadRequest)
		renderErrorPage(w, "State mismatch - possible CSRF attack. Please try again.")
		return
	}

	// Extract authorization code
	code := q.Get("code")
	if code == "" {
		select {
		case s.errCh <- errMissingCode:
		default:
		}
		w.WriteHeader(http.StatusBadRequest)
		renderErrorPage(w, "Missing authorization code. Please try again.")
		return
	}

	// Success!
	select {
	case s.codeCh <- code:
	default:
	}
	w.WriteHeader(http.StatusOK)
	renderSuccessPage(w)
}

// handleRoot shows a waiting page
func (s *AuthServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	renderWaitingPage(w)
}

// generateState creates a cryptographically secure random state token
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// OpenBrowser opens the default browser to the given URL
func OpenBrowser(url string) error {
	return openBrowser(url)
}

// PrintAuthURL prints the auth URL to stderr (for manual flow or fallback)
func PrintAuthURL(url string) {
	fmt.Fprintln(os.Stderr, "Opening browser for authorization...")
	fmt.Fprintln(os.Stderr, "If the browser doesn't open, visit this URL:")
	fmt.Fprintln(os.Stderr, url)
}
