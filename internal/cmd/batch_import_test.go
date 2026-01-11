package cmd

import (
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/api"
)

func TestBuildBatchBlocksUsesTempIDs(t *testing.T) {
	batch := api.NewBatchBuilder()

	blocks := []*MarkdownBlock{
		{Content: "Parent", Children: []*MarkdownBlock{{Content: "Child"}}},
	}

	count := buildBatchBlocks(batch, "parent-uid", blocks, "first")
	if count != 2 {
		t.Fatalf("expected 2 blocks created, got %d", count)
	}

	actions := batch.Build()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	firstLoc, ok := actions[0]["location"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected location map for first action")
	}
	if firstLoc["parent-uid"] != "parent-uid" {
		t.Fatalf("expected first parent-uid to be root, got %v", firstLoc["parent-uid"])
	}

	secondLoc, ok := actions[1]["location"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected location map for second action")
	}
	if _, ok := secondLoc["parent-uid"].(int); !ok {
		t.Fatalf("expected second parent-uid to be tempid int, got %T", secondLoc["parent-uid"])
	}
}
