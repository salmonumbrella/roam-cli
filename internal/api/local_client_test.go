package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestLocalClient_Query tests the Query method with a mock server
func TestLocalClient_Query(t *testing.T) {
	// Create test server that expects Local API format
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP method
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Verify Content-Type
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", ct)
		}

		// Read and verify request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}

		var req localRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("Failed to parse request body: %v", err)
		}

		// Verify action
		if req.Action != "data.q" {
			t.Errorf("Expected action 'data.q', got '%s'", req.Action)
		}

		// Verify query is in args
		if len(req.Args) < 1 {
			t.Errorf("Expected at least one arg, got %d", len(req.Args))
		}
		if query, ok := req.Args[0].(string); !ok || !strings.Contains(query, ":find") {
			t.Errorf("Expected Datalog query in args[0], got %v", req.Args[0])
		}

		// Return mock response
		resp := localResponse{
			Success: true,
			Result:  json.RawMessage(`[["Page 1"], ["Page 2"]]`),
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Extract port from test server URL
	port := extractPort(t, server.URL)

	// Create temp port file
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	// Create client
	client, err := NewLocalClient("test-graph")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Execute query
	results, err := client.Query(`[:find ?title :where [?p :node/title ?title]]`)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Verify results
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

// TestLocalClient_Pull tests the Pull method
func TestLocalClient_Pull(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req localRequest
		json.Unmarshal(body, &req)

		// Verify action
		if req.Action != "data.pull" {
			t.Errorf("Expected action 'data.pull', got '%s'", req.Action)
		}

		// Verify args: [selector, eid]
		if len(req.Args) != 2 {
			t.Errorf("Expected 2 args for pull, got %d", len(req.Args))
		}

		// Return mock entity
		resp := localResponse{
			Success: true,
			Result:  json.RawMessage(`{"block/uid": "abc123", "block/string": "Hello"}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	result, err := client.Pull(12345, "[*]")
	if err != nil {
		t.Fatalf("Pull failed: %v", err)
	}

	// Verify result contains expected data
	if !strings.Contains(string(result), "abc123") {
		t.Errorf("Expected result to contain 'abc123', got %s", string(result))
	}
}

// TestLocalClient_PullMany tests the PullMany method
func TestLocalClient_PullMany(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req localRequest
		json.Unmarshal(body, &req)

		if req.Action != "data.pull-many" {
			t.Errorf("Expected action 'data.pull-many', got '%s'", req.Action)
		}

		resp := localResponse{
			Success: true,
			Result:  json.RawMessage(`[{"block/uid": "a"}, {"block/uid": "b"}]`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	result, err := client.PullMany([]interface{}{1, 2}, "[*]")
	if err != nil {
		t.Fatalf("PullMany failed: %v", err)
	}

	if !strings.Contains(string(result), `"block/uid"`) {
		t.Errorf("Expected result to contain block UIDs, got %s", string(result))
	}
}

// TestLocalClient_CreateBlock tests block creation
func TestLocalClient_CreateBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req localRequest
		json.Unmarshal(body, &req)

		if req.Action != "data.block.create" {
			t.Errorf("Expected action 'data.block.create', got '%s'", req.Action)
		}

		// Verify args structure
		if len(req.Args) != 1 {
			t.Errorf("Expected 1 arg for block create, got %d", len(req.Args))
		}

		// Check that args contains location and block
		argsMap, ok := req.Args[0].(map[string]interface{})
		if !ok {
			t.Errorf("Expected args[0] to be a map")
		} else {
			if _, hasLocation := argsMap["location"]; !hasLocation {
				t.Error("Expected args to contain 'location'")
			}
			if _, hasBlock := argsMap["block"]; !hasBlock {
				t.Error("Expected args to contain 'block'")
			}
		}

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	err := client.CreateBlock("parent-uid", "New content", 0)
	if err != nil {
		t.Fatalf("CreateBlock failed: %v", err)
	}
}

// TestLocalClient_CreateBlockWithOptions tests block creation with extended properties
func TestLocalClient_CreateBlockWithOptions(t *testing.T) {
	var receivedRequest localRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedRequest)

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")

	open := true
	heading := 1
	opts := BlockOptions{
		Content:   "Heading block",
		Open:      &open,
		Heading:   &heading,
		TextAlign: "left",
	}

	err := client.CreateBlockWithOptions("parent-uid", opts, "first")
	if err != nil {
		t.Fatalf("CreateBlockWithOptions failed: %v", err)
	}

	if receivedRequest.Action != "data.block.create" {
		t.Errorf("Expected action 'data.block.create', got '%s'", receivedRequest.Action)
	}

	// Verify args contain the block properties
	argsMap, ok := receivedRequest.Args[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected args[0] to be a map")
	}

	blockMap, ok := argsMap["block"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected block in args")
	}

	if blockMap["string"] != "Heading block" {
		t.Errorf("Expected string 'Heading block', got %v", blockMap["string"])
	}
	if blockMap["open"] != true {
		t.Errorf("Expected open true, got %v", blockMap["open"])
	}
	if blockMap["heading"] != float64(1) {
		t.Errorf("Expected heading 1, got %v", blockMap["heading"])
	}
	if blockMap["text-align"] != "left" {
		t.Errorf("Expected text-align 'left', got %v", blockMap["text-align"])
	}
}

// TestLocalClient_UpdateBlock tests block update
func TestLocalClient_UpdateBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req localRequest
		json.Unmarshal(body, &req)

		if req.Action != "data.block.update" {
			t.Errorf("Expected action 'data.block.update', got '%s'", req.Action)
		}

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	err := client.UpdateBlock("block-uid", "Updated content")
	if err != nil {
		t.Fatalf("UpdateBlock failed: %v", err)
	}
}

// TestLocalClient_UpdateBlockWithOptions tests block update with extended properties
func TestLocalClient_UpdateBlockWithOptions(t *testing.T) {
	var receivedRequest localRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedRequest)

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")

	open := false
	opts := BlockOptions{
		Content: "Collapsed block",
		Open:    &open,
	}

	err := client.UpdateBlockWithOptions("block-uid", opts)
	if err != nil {
		t.Fatalf("UpdateBlockWithOptions failed: %v", err)
	}

	if receivedRequest.Action != "data.block.update" {
		t.Errorf("Expected action 'data.block.update', got '%s'", receivedRequest.Action)
	}

	argsMap, ok := receivedRequest.Args[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected args[0] to be a map")
	}

	blockMap, ok := argsMap["block"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected block in args")
	}

	if blockMap["uid"] != "block-uid" {
		t.Errorf("Expected uid 'block-uid', got %v", blockMap["uid"])
	}
	if blockMap["open"] != false {
		t.Errorf("Expected open false, got %v", blockMap["open"])
	}
}

// TestLocalClient_DeleteBlock tests block deletion
func TestLocalClient_DeleteBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req localRequest
		json.Unmarshal(body, &req)

		if req.Action != "data.block.delete" {
			t.Errorf("Expected action 'data.block.delete', got '%s'", req.Action)
		}

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	err := client.DeleteBlock("block-uid")
	if err != nil {
		t.Fatalf("DeleteBlock failed: %v", err)
	}
}

// TestLocalClient_MoveBlock tests block moving
func TestLocalClient_MoveBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req localRequest
		json.Unmarshal(body, &req)

		if req.Action != "data.block.move" {
			t.Errorf("Expected action 'data.block.move', got '%s'", req.Action)
		}

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	err := client.MoveBlock("block-uid", "new-parent-uid", "last")
	if err != nil {
		t.Fatalf("MoveBlock failed: %v", err)
	}
}

// TestLocalClient_CreatePage tests page creation
func TestLocalClient_CreatePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req localRequest
		json.Unmarshal(body, &req)

		if req.Action != "data.page.create" {
			t.Errorf("Expected action 'data.page.create', got '%s'", req.Action)
		}

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	err := client.CreatePage("New Page")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
}

// TestLocalClient_UpdatePage tests page update
func TestLocalClient_UpdatePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req localRequest
		json.Unmarshal(body, &req)

		if req.Action != "data.page.update" {
			t.Errorf("Expected action 'data.page.update', got '%s'", req.Action)
		}

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	err := client.UpdatePage("page-uid", "Updated Title")
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}
}

// TestLocalClient_DeletePage tests page deletion
func TestLocalClient_DeletePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req localRequest
		json.Unmarshal(body, &req)

		if req.Action != "data.page.delete" {
			t.Errorf("Expected action 'data.page.delete', got '%s'", req.Action)
		}

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	err := client.DeletePage("page-uid")
	if err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}
}

// TestLocalClient_ResponseError tests handling of API error responses
func TestLocalClient_ResponseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := localResponse{
			Success: false,
			Error:   "Graph not found",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	_, err := client.Query(`[:find ?x :where [?x :node/title "test"]]`)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	var apiErr LocalAPIError
	if !errors.As(err, &apiErr) {
		t.Errorf("Expected LocalAPIError, got %T: %v", err, err)
	}

	if !strings.Contains(apiErr.Message, "Graph not found") {
		t.Errorf("Expected error message to contain 'Graph not found', got: %s", apiErr.Message)
	}
}

// TestLocalClient_HTTPError tests handling of HTTP-level errors
func TestLocalClient_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	_, err := client.Query(`[:find ?x :where [?x :node/title "test"]]`)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("Expected error to mention status 500, got: %s", err.Error())
	}
}

// TestDiscoverPort_FileNotFound tests that missing port file returns DesktopNotRunningError
func TestDiscoverPort_FileNotFound(t *testing.T) {
	// Create a temp directory with no port file
	tmpDir := t.TempDir()

	// Save original HOME and restore after test
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Set HOME to temp dir (no .roam-api-port exists)
	os.Setenv("HOME", tmpDir)

	_, err := discoverPort()

	if err == nil {
		t.Fatal("Expected error when port file is missing, got nil")
	}

	var notRunningErr DesktopNotRunningError
	if !errors.As(err, &notRunningErr) {
		t.Errorf("Expected DesktopNotRunningError, got %T: %v", err, err)
	}

	// Verify error message includes helpful instructions
	if !strings.Contains(notRunningErr.Message, "not running") {
		t.Errorf("Expected error message to mention 'not running', got: %s", notRunningErr.Message)
	}

	if !strings.Contains(notRunningErr.Message, "Encrypted local API") {
		t.Errorf("Expected error message to mention 'Encrypted local API' setting, got: %s", notRunningErr.Message)
	}
}

// TestDiscoverPort_InvalidPort tests handling of invalid port content
func TestDiscoverPort_InvalidPort(t *testing.T) {
	tmpDir := t.TempDir()

	// Create port file with invalid content
	portFile := filepath.Join(tmpDir, PortFilePath)
	err := os.WriteFile(portFile, []byte("not-a-number"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test port file: %v", err)
	}

	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	_, err = discoverPort()

	if err == nil {
		t.Fatal("Expected error for invalid port, got nil")
	}

	if !strings.Contains(err.Error(), "invalid port") {
		t.Errorf("Expected error to mention 'invalid port', got: %s", err.Error())
	}
}

// TestDiscoverPort_ValidPort tests successful port discovery
func TestDiscoverPort_ValidPort(t *testing.T) {
	tmpDir := t.TempDir()

	// Create port file with valid port
	portFile := filepath.Join(tmpDir, PortFilePath)
	err := os.WriteFile(portFile, []byte("12345\n"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test port file: %v", err)
	}

	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	port, err := discoverPort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if port != 12345 {
		t.Errorf("Expected port 12345, got %d", port)
	}
}

// TestDiscoverPort_WhitespaceHandling tests trimming of whitespace around port
func TestDiscoverPort_WhitespaceHandling(t *testing.T) {
	tmpDir := t.TempDir()

	// Create port file with extra whitespace
	portFile := filepath.Join(tmpDir, PortFilePath)
	err := os.WriteFile(portFile, []byte("  8080  \n\n"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test port file: %v", err)
	}

	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	port, err := discoverPort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if port != 8080 {
		t.Errorf("Expected port 8080, got %d", port)
	}
}

// TestLocalClient_GraphName tests the GraphName getter
func TestLocalClient_GraphName(t *testing.T) {
	client, err := NewLocalClient("my-graph")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client.GraphName() != "my-graph" {
		t.Errorf("Expected graph name 'my-graph', got '%s'", client.GraphName())
	}
}

// TestLocalClient_URLFormat tests that requests go to the correct URL
func TestLocalClient_URLFormat(t *testing.T) {
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path

		resp := localResponse{
			Success: true,
			Result:  json.RawMessage(`[]`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("my-special-graph")
	client.Query(`[:find ?x :where [?x :node/title "test"]]`)

	expectedPath := "/api/my-special-graph"
	if receivedPath != expectedPath {
		t.Errorf("Expected path '%s', got '%s'", expectedPath, receivedPath)
	}
}

// TestLocalClient_RequestFormat tests the JSON request structure
func TestLocalClient_RequestFormat(t *testing.T) {
	var receivedRequest localRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedRequest)

		resp := localResponse{
			Success: true,
			Result:  json.RawMessage(`[]`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")

	// Test query with args
	client.Query(`[:find ?title :where [?p :node/title ?title]]`, "extra-arg")

	if receivedRequest.Action != "data.q" {
		t.Errorf("Expected action 'data.q', got '%s'", receivedRequest.Action)
	}

	if len(receivedRequest.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(receivedRequest.Args))
	}

	// First arg should be the query
	query, ok := receivedRequest.Args[0].(string)
	if !ok || !strings.Contains(query, ":find") {
		t.Errorf("Expected query in args[0], got %v", receivedRequest.Args[0])
	}

	// Second arg should be the extra arg
	if receivedRequest.Args[1] != "extra-arg" {
		t.Errorf("Expected 'extra-arg' in args[1], got %v", receivedRequest.Args[1])
	}
}

// TestLocalClient_GetPageByTitle tests page retrieval by title
func TestLocalClient_GetPageByTitle(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req localRequest
		json.Unmarshal(body, &req)

		callCount++

		var resp localResponse
		if req.Action == "data.q" {
			// First call: query to find page entity
			resp = localResponse{
				Success: true,
				Result:  json.RawMessage(`[[12345]]`),
			}
		} else if req.Action == "data.pull" {
			// Second call: pull to get page data
			resp = localResponse{
				Success: true,
				Result:  json.RawMessage(`{"node/title": "My Page", "block/children": []}`),
			}
		}

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	result, err := client.GetPageByTitle("My Page")
	if err != nil {
		t.Fatalf("GetPageByTitle failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 API calls (query + pull), got %d", callCount)
	}

	if !strings.Contains(string(result), "My Page") {
		t.Errorf("Expected result to contain 'My Page', got %s", string(result))
	}
}

// TestLocalClient_GetPageByTitle_NotFound tests page not found error
func TestLocalClient_GetPageByTitle_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := localResponse{
			Success: true,
			Result:  json.RawMessage(`[]`), // Empty results
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	_, err := client.GetPageByTitle("Nonexistent Page")

	if err == nil {
		t.Fatal("Expected error for nonexistent page, got nil")
	}

	var notFoundErr NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("Expected NotFoundError, got %T: %v", err, err)
	}
}

// TestLocalClient_GetBlockByUID tests block retrieval by UID
func TestLocalClient_GetBlockByUID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req localRequest
		json.Unmarshal(body, &req)

		var resp localResponse
		if req.Action == "data.q" {
			resp = localResponse{
				Success: true,
				Result:  json.RawMessage(`[[54321]]`),
			}
		} else if req.Action == "data.pull" {
			resp = localResponse{
				Success: true,
				Result:  json.RawMessage(`{"block/uid": "abc123", "block/string": "Hello World"}`),
			}
		}

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	result, err := client.GetBlockByUID("abc123")
	if err != nil {
		t.Fatalf("GetBlockByUID failed: %v", err)
	}

	if !strings.Contains(string(result), "Hello World") {
		t.Errorf("Expected result to contain 'Hello World', got %s", string(result))
	}
}

// TestLocalClient_SearchBlocks tests block search
func TestLocalClient_SearchBlocks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req localRequest
		json.Unmarshal(body, &req)

		// Verify query contains search term
		if len(req.Args) > 0 {
			query, _ := req.Args[0].(string)
			if !strings.Contains(query, "meeting") {
				t.Errorf("Expected query to contain search term 'meeting'")
			}
		}

		resp := localResponse{
			Success: true,
			Result:  json.RawMessage(`[["uid1", "Meeting notes", "Daily"], ["uid2", "Team meeting", "Work"]]`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")
	results, err := client.SearchBlocks("meeting", 10)
	if err != nil {
		t.Fatalf("SearchBlocks failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

// TestLocalClient_ListPages tests page listing
func TestLocalClient_ListPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := localResponse{
			Success: true,
			Result:  json.RawMessage(`[["Page A", "uid1"], ["Page B", "uid2"], ["Page C", "uid3"]]`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	portFile := createTempPortFile(t, port)
	defer os.Remove(portFile)

	client, _ := NewLocalClient("test-graph")

	// Test without limit
	results, err := client.ListPages(false, 0)
	if err != nil {
		t.Fatalf("ListPages failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Test with limit
	results, err = client.ListPages(false, 2)
	if err != nil {
		t.Fatalf("ListPages with limit failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results with limit, got %d", len(results))
	}
}

// TestLocalAPIError_ErrorMethod tests the Error method of LocalAPIError
func TestLocalAPIError_ErrorMethod(t *testing.T) {
	err := LocalAPIError{Message: "Something went wrong"}
	if err.Error() != "Something went wrong" {
		t.Errorf("Expected 'Something went wrong', got '%s'", err.Error())
	}
}

// TestDesktopNotRunningError_ErrorMethod tests the Error method of DesktopNotRunningError
func TestDesktopNotRunningError_ErrorMethod(t *testing.T) {
	err := DesktopNotRunningError{Message: "Desktop not running"}
	if err.Error() != "Desktop not running" {
		t.Errorf("Expected 'Desktop not running', got '%s'", err.Error())
	}
}

func TestLocalClient_Undo(t *testing.T) {
	var receivedRequest localRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedRequest)

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	createTempPortFile(t, port)

	client, _ := NewLocalClient("test-graph")
	err := client.Undo()
	if err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	if receivedRequest.Action != "data.undo" {
		t.Errorf("Expected action 'data.undo', got '%s'", receivedRequest.Action)
	}
}

func TestLocalClient_Redo(t *testing.T) {
	var receivedRequest localRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedRequest)

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	createTempPortFile(t, port)

	client, _ := NewLocalClient("test-graph")
	err := client.Redo()
	if err != nil {
		t.Fatalf("Redo failed: %v", err)
	}

	if receivedRequest.Action != "data.redo" {
		t.Errorf("Expected action 'data.redo', got '%s'", receivedRequest.Action)
	}
}

func TestLocalClient_ReorderBlocks(t *testing.T) {
	var receivedRequest localRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedRequest)

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	createTempPortFile(t, port)

	client, _ := NewLocalClient("test-graph")

	blockUIDs := []string{"uid1", "uid2", "uid3"}
	err := client.ReorderBlocks("parent-uid", blockUIDs)
	if err != nil {
		t.Fatalf("ReorderBlocks failed: %v", err)
	}

	if receivedRequest.Action != "data.block.reorderBlocks" {
		t.Errorf("Expected action 'data.block.reorderBlocks', got '%s'", receivedRequest.Action)
	}

	// Verify args
	if len(receivedRequest.Args) < 1 {
		t.Fatal("Expected args in request")
	}

	argsMap, ok := receivedRequest.Args[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected args[0] to be map")
	}

	if argsMap["parent-uid"] != "parent-uid" {
		t.Errorf("Expected parent-uid 'parent-uid', got %v", argsMap["parent-uid"])
	}
}

func TestLocalClient_AddRemovePageShortcut(t *testing.T) {
	var receivedRequest localRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedRequest)

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	createTempPortFile(t, port)

	client, _ := NewLocalClient("test-graph")

	index := 3
	if err := client.AddPageShortcut("page-uid", &index); err != nil {
		t.Fatalf("AddPageShortcut failed: %v", err)
	}
	if receivedRequest.Action != "data.page.addShortcut" {
		t.Errorf("Expected action 'data.page.addShortcut', got '%s'", receivedRequest.Action)
	}
	if len(receivedRequest.Args) != 2 {
		t.Fatalf("Expected 2 args for addShortcut, got %d", len(receivedRequest.Args))
	}

	if err := client.RemovePageShortcut("page-uid"); err != nil {
		t.Fatalf("RemovePageShortcut failed: %v", err)
	}
	if receivedRequest.Action != "data.page.removeShortcut" {
		t.Errorf("Expected action 'data.page.removeShortcut', got '%s'", receivedRequest.Action)
	}
}

func TestLocalClient_UpsertUser(t *testing.T) {
	var receivedRequest localRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedRequest)

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	createTempPortFile(t, port)

	client, _ := NewLocalClient("test-graph")

	if err := client.UpsertUser("user-uid", "Display Name"); err != nil {
		t.Fatalf("UpsertUser failed: %v", err)
	}

	if receivedRequest.Action != "data.user.upsert" {
		t.Errorf("Expected action 'data.user.upsert', got '%s'", receivedRequest.Action)
	}
}

func TestLocalClient_DeleteFile(t *testing.T) {
	var receivedRequest localRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedRequest)

		resp := localResponse{Success: true}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	createTempPortFile(t, port)

	client, _ := NewLocalClient("test-graph")

	if err := client.DeleteFile("https://example.com/file.txt"); err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	if receivedRequest.Action != "file.delete" {
		t.Errorf("Expected action 'file.delete', got '%s'", receivedRequest.Action)
	}
}

func TestLocalClient_UploadFile(t *testing.T) {
	var receivedRequest localRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedRequest)

		resp := localResponse{
			Success: true,
			Result:  json.RawMessage(`{"url": "https://firebasestorage.googleapis.com/..."}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	createTempPortFile(t, port)

	client, _ := NewLocalClient("test-graph")

	fileData := []byte("test file content")
	url, err := client.UploadFile("test.txt", fileData)
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}

	if receivedRequest.Action != "file.upload" {
		t.Errorf("Expected action 'file.upload', got '%s'", receivedRequest.Action)
	}

	if url == "" {
		t.Error("Expected non-empty URL")
	}
}

func TestLocalClient_DownloadFile(t *testing.T) {
	var receivedRequest localRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedRequest)

		resp := localResponse{
			Success: true,
			Result:  json.RawMessage(`"dGVzdCBmaWxlIGNvbnRlbnQ="`), // base64 of "test file content"
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	port := extractPort(t, server.URL)
	createTempPortFile(t, port)

	client, _ := NewLocalClient("test-graph")

	data, err := client.DownloadFile("https://example.com/file.txt")
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	if receivedRequest.Action != "file.get" {
		t.Errorf("Expected action 'file.get', got '%s'", receivedRequest.Action)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty data")
	}
}

// Helper function to extract port from httptest server URL
func extractPort(t *testing.T, serverURL string) int {
	t.Helper()
	// URL is like "http://127.0.0.1:12345"
	parts := strings.Split(serverURL, ":")
	if len(parts) < 3 {
		t.Fatalf("Unexpected server URL format: %s", serverURL)
	}
	port, err := strconv.Atoi(parts[2])
	if err != nil {
		t.Fatalf("Failed to parse port from URL %s: %v", serverURL, err)
	}
	return port
}

// Helper function to create a temporary port file
func createTempPortFile(t *testing.T, port int) string {
	t.Helper()

	// Create port file in a temp directory that we'll set as HOME
	tmpDir := t.TempDir()
	portFile := filepath.Join(tmpDir, PortFilePath)

	err := os.WriteFile(portFile, []byte(strconv.Itoa(port)), 0o644)
	if err != nil {
		t.Fatalf("Failed to create temp port file: %v", err)
	}

	// Set HOME to temp dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() {
		os.Setenv("HOME", originalHome)
	})

	return portFile
}
