package cmd

import (
	"encoding/json"
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/roamdb"
)

func TestBlockHelpers(t *testing.T) {
	selector := buildBlockSelector(2)
	if selector == "" {
		t.Fatal("expected selector")
	}

	block := roamdb.Block{String: "hello", UID: "uid", Children: []roamdb.Block{{String: "child", UID: "c"}}}
	printBlockText(block, 0)
}

func TestVerifyBlockHelpers(t *testing.T) {
	fake := &fakeClient{
		QueryFunc: func(query string, args ...interface{}) ([][]interface{}, error) {
			return [][]interface{}{{"uid"}}, nil
		},
	}
	if !verifyBlockCreatedByContent(fake, "hello") {
		t.Fatal("expected verifyBlockCreatedByContent true")
	}
	if !verifyBlockUpdated(fake, "uid", "hello") {
		t.Fatal("expected verifyBlockUpdated true")
	}
	if !verifyBlockMoved(fake, "uid", "parent") {
		t.Fatal("expected verifyBlockMoved true")
	}
}

func TestSearchOutputText(t *testing.T) {
	results := []SearchResult{{UID: "u1", Content: "hello", PageTitle: "Page"}}
	if err := outputSearchResults("q", results, 1, 1); err != nil {
		t.Fatalf("outputSearchResults failed: %v", err)
	}
	if err := outputSearchResults("q", nil, 0, 1); err != nil {
		t.Fatalf("outputSearchResults empty failed: %v", err)
	}
}

func TestAppendParsing(t *testing.T) {
	blocks := parseIndentedBlocks("Parent\n  Child\n")
	if len(blocks) != 1 || len(blocks[0].Children) != 1 {
		t.Fatalf("unexpected blocks: %+v", blocks)
	}
	if count := countAppendBlocks(blocks); count != 2 {
		t.Fatalf("expected 2 blocks, got %d", count)
	}

	payload, err := json.Marshal(blocks)
	if err != nil || len(payload) == 0 {
		t.Fatalf("unexpected json: %v", err)
	}
}
