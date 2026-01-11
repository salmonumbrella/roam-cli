package cmd

import "testing"

func TestBuildMarkdownTree(t *testing.T) {
	entries := []markdownEntry{
		{level: 0, content: "Root"},
		{level: 1, content: "Child A"},
		{level: 1, content: "Child B"},
		{level: 2, content: "Grandchild"},
		{level: 0, content: "Second Root"},
	}

	blocks := buildMarkdownTree(entries)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 top-level blocks, got %d", len(blocks))
	}

	if blocks[0].Content != "Root" {
		t.Fatalf("expected first root content, got %q", blocks[0].Content)
	}
	if len(blocks[0].Children) != 2 {
		t.Fatalf("expected 2 children under Root, got %d", len(blocks[0].Children))
	}
	if blocks[0].Children[0].Content != "Child A" {
		t.Fatalf("expected first child content, got %q", blocks[0].Children[0].Content)
	}
	if blocks[0].Children[1].Content != "Child B" {
		t.Fatalf("expected second child content, got %q", blocks[0].Children[1].Content)
	}
	if len(blocks[0].Children[1].Children) != 1 {
		t.Fatalf("expected Child B to have 1 child, got %d", len(blocks[0].Children[1].Children))
	}
	if blocks[0].Children[1].Children[0].Content != "Grandchild" {
		t.Fatalf("expected grandchild content, got %q", blocks[0].Children[1].Children[0].Content)
	}

	if blocks[1].Content != "Second Root" {
		t.Fatalf("expected second root content, got %q", blocks[1].Content)
	}
}
