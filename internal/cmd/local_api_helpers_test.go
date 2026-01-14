package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/api"
)

type localRequest struct {
	Action string        `json:"action"`
	Args   []interface{} `json:"args"`
}

type localResponse struct {
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

func newTestLocalClient(t *testing.T, handler func(localRequest) localResponse) *api.LocalClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req localRequest
		_ = json.Unmarshal(body, &req)

		resp := handler(req)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)

	port := extractPort(t, server.URL)
	createTempPortFile(t, port)

	client, err := api.NewLocalClient("test-graph")
	if err != nil {
		t.Fatalf("failed to create local client: %v", err)
	}
	return client
}

func extractPort(t *testing.T, serverURL string) int {
	t.Helper()
	parts := strings.Split(serverURL, ":")
	if len(parts) < 3 {
		t.Fatalf("unexpected server URL format: %s", serverURL)
	}
	port, err := strconv.Atoi(parts[2])
	if err != nil {
		t.Fatalf("failed to parse port from URL %s: %v", serverURL, err)
	}
	return port
}

func createTempPortFile(t *testing.T, port int) {
	t.Helper()
	tmpDir := t.TempDir()
	portFile := filepath.Join(tmpDir, api.PortFilePath)

	if err := os.WriteFile(portFile, []byte(strconv.Itoa(port)), 0o644); err != nil {
		t.Fatalf("failed to create temp port file: %v", err)
	}

	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	t.Cleanup(func() {
		_ = os.Setenv("HOME", originalHome)
	})
}
