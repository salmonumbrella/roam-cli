package api

import (
	"encoding/json"
	"testing"
)

func TestBatchBuilder_CreateAndReference(t *testing.T) {
	b := NewBatchBuilder()

	// Create a page and get its tempid
	pageRef := b.CreatePage(PageOptions{Title: "New Page"})

	// Create a block under that page using the tempid
	blockRef := b.CreateBlock(Location{ParentUID: pageRef}, BlockOptions{Content: "First block"})

	// Create a child block under the first block
	b.CreateBlock(Location{ParentUID: blockRef}, BlockOptions{Content: "Child block"})

	actions := b.Build()

	if len(actions) != 3 {
		t.Fatalf("Expected 3 actions, got %d", len(actions))
	}

	// First action should be create-page with tempid -1
	if actions[0]["action"] != "create-page" {
		t.Errorf("Expected first action 'create-page', got %v", actions[0]["action"])
	}
	page := actions[0]["page"].(map[string]interface{})
	if page["uid"] != -1 {
		t.Errorf("Expected page uid -1, got %v", page["uid"])
	}

	// Second action should reference -1 as parent
	location := actions[1]["location"].(map[string]interface{})
	if location["parent-uid"] != -1 {
		t.Errorf("Expected parent-uid -1, got %v", location["parent-uid"])
	}
	block := actions[1]["block"].(map[string]interface{})
	if block["uid"] != -2 {
		t.Errorf("Expected block uid -2, got %v", block["uid"])
	}

	// Third action should reference -2 as parent
	location2 := actions[2]["location"].(map[string]interface{})
	if location2["parent-uid"] != -2 {
		t.Errorf("Expected parent-uid -2, got %v", location2["parent-uid"])
	}
}

func TestBatchBuilder_JSON(t *testing.T) {
	b := NewBatchBuilder()
	pageRef := b.CreatePage(PageOptions{Title: "Test"})
	b.CreateBlock(Location{ParentUID: pageRef}, BlockOptions{Content: "Block"})

	actions := b.Build()
	jsonBytes, err := json.Marshal(actions)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	// Verify it's valid JSON array
	var parsed []map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if len(parsed) != 2 {
		t.Errorf("Expected 2 actions in JSON, got %d", len(parsed))
	}
}
