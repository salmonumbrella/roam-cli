package api

import (
	"testing"
)

func TestBlockOptions_ToBlock(t *testing.T) {
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

	block := opts.ToBlock()

	if block.String != "Test content" {
		t.Errorf("Expected String 'Test content', got '%s'", block.String)
	}
	if block.Open == nil || *block.Open != true {
		t.Errorf("Expected Open true, got %v", block.Open)
	}
	if block.Heading == nil || *block.Heading != 2 {
		t.Errorf("Expected Heading 2, got %v", block.Heading)
	}
	if block.TextAlign != "center" {
		t.Errorf("Expected TextAlign 'center', got '%s'", block.TextAlign)
	}
	if block.ChildrenViewType != "numbered" {
		t.Errorf("Expected ChildrenViewType 'numbered', got '%s'", block.ChildrenViewType)
	}
	if block.BlockViewType != "document" {
		t.Errorf("Expected BlockViewType 'document', got '%s'", block.BlockViewType)
	}
}

func TestBlockOptions_ToBlock_MinimalOptions(t *testing.T) {
	opts := BlockOptions{
		Content: "Simple content",
	}

	block := opts.ToBlock()

	if block.String != "Simple content" {
		t.Errorf("Expected String 'Simple content', got '%s'", block.String)
	}
	if block.Open != nil {
		t.Errorf("Expected Open nil, got %v", block.Open)
	}
}

func TestBlockOptions_ApplyToMap(t *testing.T) {
	open := true
	heading := 1

	opts := BlockOptions{
		Content:          "Test",
		Open:             &open,
		Heading:          &heading,
		TextAlign:        "right",
		ChildrenViewType: "bullet",
		BlockViewType:    "numbered",
		Props: map[string]interface{}{
			"custom-key": "custom-value",
		},
	}

	m := map[string]interface{}{
		"string": opts.Content,
	}
	opts.ApplyToMap(m)

	if m["open"] != true {
		t.Errorf("Expected open true, got %v", m["open"])
	}
	if m["heading"] != 1 {
		t.Errorf("Expected heading 1, got %v", m["heading"])
	}
	if m["text-align"] != "right" {
		t.Errorf("Expected text-align 'right', got %v", m["text-align"])
	}
	if m["children-view-type"] != "bullet" {
		t.Errorf("Expected children-view-type 'bullet', got %v", m["children-view-type"])
	}
	if m["block-view-type"] != "numbered" {
		t.Errorf("Expected block-view-type 'numbered', got %v", m["block-view-type"])
	}
	props, ok := m["props"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected props to be map, got %T", m["props"])
	}
	if props["custom-key"] != "custom-value" {
		t.Errorf("Expected props custom-key 'custom-value', got %v", props["custom-key"])
	}
}

func TestBlockOptions_ApplyToMap_NilProps(t *testing.T) {
	opts := BlockOptions{
		Content: "Test",
	}

	m := map[string]interface{}{
		"string": opts.Content,
	}
	opts.ApplyToMap(m)

	if _, ok := m["props"]; ok {
		t.Error("Expected props to not be set when nil")
	}
}
