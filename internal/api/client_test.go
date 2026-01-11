package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_CreateBlockAtLocation_PageTitle(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient("test-graph", "test-token", WithBaseURL(server.URL))

	loc := Location{
		PageTitle: "My Target Page",
		Order:     "last",
	}
	opts := BlockOptions{
		Content: "Block on page",
	}

	err := client.CreateBlockAtLocation(loc, opts)
	if err != nil {
		t.Fatalf("CreateBlockAtLocation failed: %v", err)
	}

	location, ok := receivedBody["location"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'location' in request body")
	}

	if location["page-title"] != "My Target Page" {
		t.Errorf("Expected page-title 'My Target Page', got %v", location["page-title"])
	}
}

func TestClient_CreateBlockAtLocation_DailyNote(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient("test-graph", "test-token", WithBaseURL(server.URL))

	loc := Location{
		DailyNoteDate: "01-15-2025",
		Order:         "first",
	}
	opts := BlockOptions{
		Content: "Daily note block",
	}

	err := client.CreateBlockAtLocation(loc, opts)
	if err != nil {
		t.Fatalf("CreateBlockAtLocation failed: %v", err)
	}

	location, ok := receivedBody["location"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'location' in request body")
	}

	pageTitle, ok := location["page-title"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected page-title to be map, got %T", location["page-title"])
	}
	if pageTitle["daily-note-page"] != "01-15-2025" {
		t.Errorf("Expected daily-note-page '01-15-2025', got %v", pageTitle["daily-note-page"])
	}
}

func TestClient_CreatePageWithOptions(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient("test-graph", "test-token", WithBaseURL(server.URL))

	opts := PageOptions{
		Title:            "Test Page",
		ChildrenViewType: "numbered",
	}

	err := client.CreatePageWithOptions(opts)
	if err != nil {
		t.Fatalf("CreatePageWithOptions failed: %v", err)
	}

	page, ok := receivedBody["page"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'page' in request body")
	}

	if page["title"] != "Test Page" {
		t.Errorf("Expected title 'Test Page', got %v", page["title"])
	}
	if page["children-view-type"] != "numbered" {
		t.Errorf("Expected children-view-type 'numbered', got %v", page["children-view-type"])
	}
}

func TestClient_UpdateBlockWithOptions(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient("test-graph", "test-token", WithBaseURL(server.URL))

	heading := 3
	opts := BlockOptions{
		Content: "Updated heading",
		Heading: &heading,
	}

	err := client.UpdateBlockWithOptions("block-uid", opts)
	if err != nil {
		t.Fatalf("UpdateBlockWithOptions failed: %v", err)
	}

	block, ok := receivedBody["block"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'block' in request body")
	}

	if block["uid"] != "block-uid" {
		t.Errorf("Expected uid 'block-uid', got %v", block["uid"])
	}
	if block["heading"] != float64(3) {
		t.Errorf("Expected heading 3, got %v", block["heading"])
	}
}

func TestClient_CreateBlockWithOptions(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient("test-graph", "test-token", WithBaseURL(server.URL))

	open := true
	heading := 2
	opts := BlockOptions{
		Content:          "Test content",
		Open:             &open,
		Heading:          &heading,
		TextAlign:        "center",
		ChildrenViewType: "numbered",
		BlockViewType:    "document",
	}

	err := client.CreateBlockWithOptions("parent-uid", opts, "last")
	if err != nil {
		t.Fatalf("CreateBlockWithOptions failed: %v", err)
	}

	// Verify the request body contains all properties
	block, ok := receivedBody["block"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'block' in request body")
	}

	if block["string"] != "Test content" {
		t.Errorf("Expected string 'Test content', got %v", block["string"])
	}
	if block["open"] != true {
		t.Errorf("Expected open true, got %v", block["open"])
	}
	if block["heading"] != float64(2) {
		t.Errorf("Expected heading 2, got %v", block["heading"])
	}
	if block["text-align"] != "center" {
		t.Errorf("Expected text-align 'center', got %v", block["text-align"])
	}
	if block["children-view-type"] != "numbered" {
		t.Errorf("Expected children-view-type 'numbered', got %v", block["children-view-type"])
	}
	if block["block-view-type"] != "document" {
		t.Errorf("Expected block-view-type 'document', got %v", block["block-view-type"])
	}
}

func TestClient_ExecuteBatch(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient("test-graph", "test-token", WithBaseURL(server.URL))

	batch := NewBatchBuilder()
	pageRef := batch.CreatePage(PageOptions{Title: "Batch Page"})
	batch.CreateBlock(Location{ParentUID: pageRef, Order: "last"}, BlockOptions{Content: "Batch block"})

	err := client.ExecuteBatch(batch)
	if err != nil {
		t.Fatalf("ExecuteBatch failed: %v", err)
	}

	if receivedBody["action"] != "batch-actions" {
		t.Errorf("Expected action 'batch-actions', got %v", receivedBody["action"])
	}

	actions, ok := receivedBody["actions"].([]interface{})
	if !ok {
		t.Fatal("Expected 'actions' array in request body")
	}
	if len(actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(actions))
	}
}
