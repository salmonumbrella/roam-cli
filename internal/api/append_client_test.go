package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppendClient_AppendBlocks(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path
		if r.URL.Path != "/api/graph/test-graph/append-blocks" {
			t.Errorf("Expected path /api/graph/test-graph/append-blocks, got %s", r.URL.Path)
		}

		// Verify headers
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Authorization header")
		}

		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	client := NewAppendClient("test-graph", "test-token", WithAppendBaseURL(server.URL))

	blocks := []AppendBlock{
		{String: "First block"},
		{String: "Second block", Children: []AppendBlock{
			{String: "Nested child"},
		}},
	}

	err := client.AppendBlocks("My Page", blocks)
	if err != nil {
		t.Fatalf("AppendBlocks failed: %v", err)
	}

	// Verify request body
	location, ok := receivedBody["location"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected location in request body")
	}
	page, ok := location["page"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected location.page in request body")
	}
	if page["title"] != "My Page" {
		t.Errorf("Expected page title 'My Page', got %v", page["title"])
	}

	appendBlocks, ok := receivedBody["append-data"].([]interface{})
	if !ok {
		t.Fatal("Expected append-data array")
	}
	if len(appendBlocks) != 2 {
		t.Errorf("Expected 2 blocks, got %d", len(appendBlocks))
	}
}

func TestAppendClient_AppendToDailyNote(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	client := NewAppendClient("test-graph", "test-token", WithAppendBaseURL(server.URL))

	blocks := []AppendBlock{{String: "Daily entry"}}

	err := client.AppendToDailyNote("01-15-2025", blocks)
	if err != nil {
		t.Fatalf("AppendToDailyNote failed: %v", err)
	}

	location, ok := receivedBody["location"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected location in request body")
	}
	page, ok := location["page"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected location.page in request body")
	}
	title, ok := page["title"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected page.title to be map for daily note")
	}
	if title["daily-note-page"] != "01-15-2025" {
		t.Errorf("Expected daily-note-page '01-15-2025', got %v", title["daily-note-page"])
	}
}

func TestAppendClient_AppendToBlock(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	client := NewAppendClient("test-graph", "test-token", WithAppendBaseURL(server.URL))

	blocks := []AppendBlock{{String: "Child block"}}

	err := client.AppendToBlock("block-uid-123", blocks)
	if err != nil {
		t.Fatalf("AppendToBlock failed: %v", err)
	}

	location, ok := receivedBody["location"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected location in request body")
	}
	block, ok := location["block"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected location.block in request body")
	}
	if block["uid"] != "block-uid-123" {
		t.Errorf("Expected block uid 'block-uid-123', got %v", block["uid"])
	}
}

func TestAppendClient_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "invalid token"}`))
	}))
	defer server.Close()

	client := NewAppendClient("test-graph", "bad-token", WithAppendBaseURL(server.URL))

	err := client.AppendBlocks("My Page", []AppendBlock{{String: "test"}})
	if err == nil {
		t.Fatal("Expected error for unauthorized request")
	}

	if _, ok := err.(AuthenticationError); !ok {
		t.Errorf("Expected AuthenticationError, got %T: %v", err, err)
	}
}

func TestAppendClient_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": "rate limit exceeded"}`))
	}))
	defer server.Close()

	client := NewAppendClient("test-graph", "test-token", WithAppendBaseURL(server.URL))

	err := client.AppendBlocks("My Page", []AppendBlock{{String: "test"}})
	if err == nil {
		t.Fatal("Expected error for rate limited request")
	}

	if _, ok := err.(RateLimitError); !ok {
		t.Errorf("Expected RateLimitError, got %T: %v", err, err)
	}
}

func TestAppendClient_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	client := NewAppendClient("test-graph", "test-token", WithAppendBaseURL(server.URL))

	err := client.AppendBlocks("My Page", []AppendBlock{{String: "test"}})
	if err == nil {
		t.Fatal("Expected error for server error response")
	}

	// Should be a generic error, not AuthenticationError or RateLimitError
	if _, ok := err.(AuthenticationError); ok {
		t.Error("Should not be AuthenticationError")
	}
	if _, ok := err.(RateLimitError); ok {
		t.Error("Should not be RateLimitError")
	}
}
