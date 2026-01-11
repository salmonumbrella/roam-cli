package roamdb

import (
	"encoding/json"
	"testing"
)

func TestPage_UnmarshalJSON_LocalAPI(t *testing.T) {
	// Local API format with colon-prefixed keys
	data := []byte(`{
		":node/title": "Test Page",
		":block/uid": "test-uid-123",
		":edit/time": 1234567890,
		":block/children": [
			{
				":block/string": "Child block",
				":block/uid": "child-uid",
				":block/order": 0
			}
		]
	}`)

	var page Page
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if page.Title != "Test Page" {
		t.Errorf("expected title 'Test Page', got %q", page.Title)
	}
	if page.UID != "test-uid-123" {
		t.Errorf("expected UID 'test-uid-123', got %q", page.UID)
	}
	if page.EditTime != 1234567890 {
		t.Errorf("expected EditTime 1234567890, got %d", page.EditTime)
	}
	if len(page.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(page.Children))
	}
	if page.Children[0].String != "Child block" {
		t.Errorf("expected child content 'Child block', got %q", page.Children[0].String)
	}
}

func TestPage_UnmarshalJSON_StandardAPI(t *testing.T) {
	// Standard API format without colon prefix
	data := []byte(`{
		"node/title": "Standard Page",
		"block/uid": "std-uid-456",
		"block/children": [
			{
				"block/string": "Standard child",
				"block/uid": "std-child",
				"block/order": 0
			}
		]
	}`)

	var page Page
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if page.Title != "Standard Page" {
		t.Errorf("expected title 'Standard Page', got %q", page.Title)
	}
	if page.UID != "std-uid-456" {
		t.Errorf("expected UID 'std-uid-456', got %q", page.UID)
	}
	if len(page.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(page.Children))
	}
}

func TestBlock_UnmarshalJSON_LocalAPI(t *testing.T) {
	// Local API format with colon-prefixed keys
	data := []byte(`{
		":block/string": "Test content",
		":block/uid": "block-uid-123",
		":block/order": 5,
		":block/children": [
			{
				":block/string": "Nested child",
				":block/uid": "nested-uid",
				":block/order": 0
			}
		]
	}`)

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if block.String != "Test content" {
		t.Errorf("expected String 'Test content', got %q", block.String)
	}
	if block.UID != "block-uid-123" {
		t.Errorf("expected UID 'block-uid-123', got %q", block.UID)
	}
	if block.Order != 5 {
		t.Errorf("expected Order 5, got %d", block.Order)
	}
	if len(block.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(block.Children))
	}
	if block.Children[0].String != "Nested child" {
		t.Errorf("expected nested child content 'Nested child', got %q", block.Children[0].String)
	}
}

func TestBlock_UnmarshalJSON_StandardAPI(t *testing.T) {
	// Standard API format without colon prefix
	data := []byte(`{
		"block/string": "Standard block",
		"block/uid": "std-block-789",
		"block/order": 2
	}`)

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if block.String != "Standard block" {
		t.Errorf("expected String 'Standard block', got %q", block.String)
	}
	if block.UID != "std-block-789" {
		t.Errorf("expected UID 'std-block-789', got %q", block.UID)
	}
	if block.Order != 2 {
		t.Errorf("expected Order 2, got %d", block.Order)
	}
}

func TestBlock_UnmarshalJSON_EmptyBlock(t *testing.T) {
	// Edge case: block with UID but empty content
	data := []byte(`{
		":block/uid": "empty-block",
		":block/order": 0
	}`)

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if block.UID != "empty-block" {
		t.Errorf("expected UID 'empty-block', got %q", block.UID)
	}
	if block.String != "" {
		t.Errorf("expected empty String, got %q", block.String)
	}
}
