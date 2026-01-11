package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/api"
)

func TestUIDAndLocationHelpers(t *testing.T) {
	if uid := uidFromAny(float64(12)); uid != "12" {
		t.Fatalf("expected uid 12, got %q", uid)
	}
	if uid := uidFromAny(json.Number("99")); uid != "99" {
		t.Fatalf("expected uid 99, got %q", uid)
	}

	loc, err := locationFromMap(map[string]interface{}{"parent-uid": "abc", "order": 2}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loc.ParentUID != "abc" || loc.Order != 2 {
		t.Fatalf("unexpected location: %+v", loc)
	}

	loc, err = locationFromMap(map[string]interface{}{"page-title": "Home"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loc.PageTitle != "Home" {
		t.Fatalf("expected page title, got %+v", loc)
	}
}

func TestOptionsFromMap(t *testing.T) {
	block := blockOptionsFromMap(map[string]interface{}{
		"string":  "hello",
		"uid":     "u1",
		"open":    true,
		"heading": 2,
		"props":   map[string]interface{}{"a": 1},
	})
	if block.Content != "hello" || block.UID != "u1" {
		t.Fatalf("unexpected block opts: %+v", block)
	}
	if block.Open == nil || !*block.Open || block.Heading == nil || *block.Heading != 2 {
		t.Fatalf("unexpected block flags: %+v", block)
	}

	page := pageOptionsFromMap(map[string]interface{}{
		"title": "My Page",
		"uid":   "p1",
	})
	if page.Title != "My Page" || page.UID != "p1" {
		t.Fatalf("unexpected page opts: %+v", page)
	}
}

func TestExecuteBatchSummary(t *testing.T) {
	fake := &fakeClient{
		CreateBlockAtLocationFunc: func(loc api.Location, opts api.BlockOptions) error { return nil },
		UpdateBlockWithOptionsFunc: func(uid string, opts api.BlockOptions) error {
			if uid == "bad" {
				return api.ValidationError{Message: "bad"}
			}
			return nil
		},
		MoveBlockToLocationFunc:   func(uid string, loc api.Location) error { return nil },
		DeleteBlockFunc:           func(uid string) error { return nil },
		CreatePageWithOptionsFunc: func(opts api.PageOptions) error { return nil },
		UpdatePageWithOptionsFunc: func(uid string, opts api.PageOptions) error { return nil },
		DeletePageFunc:            func(uid string) error { return nil },
	}

	actions := []BatchAction{
		{Action: "create-block", Location: map[string]interface{}{"parent-uid": "p", "order": "last"}, Block: map[string]interface{}{"string": "hi"}},
		{Action: "update-block", Block: map[string]interface{}{"uid": "bad", "string": "oops"}},
		{Action: "move-block", Block: map[string]interface{}{"uid": "u2"}, Location: map[string]interface{}{"parent-uid": "p2", "order": "first"}},
		{Action: "delete-block", Block: map[string]interface{}{"uid": "u3"}},
		{Action: "create-page", Page: map[string]interface{}{"title": "Page"}},
		{Action: "update-page", Page: map[string]interface{}{"uid": "p1", "title": "New"}},
		{Action: "delete-page", Page: map[string]interface{}{"uid": "p2"}},
	}

	summary := executeBatch(fake, actions)
	if summary.Total != len(actions) {
		t.Fatalf("expected total %d, got %d", len(actions), summary.Total)
	}
	if summary.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", summary.Failed)
	}
}

func TestRunImportDryRun(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "notes.md")
	content := "# Heading\n- Bullet\n  - Child\n1. Numbered\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	importPage = "Test"
	importDryRun = true
	defer func() {
		importPage = ""
		importDryRun = false
	}()

	if err := runImport(importCmd, []string{filePath}); err != nil {
		t.Fatalf("runImport failed: %v", err)
	}
}

func TestRunBatchDryRun(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "batch.json")
	payload := `[{"action":"create-page","page":{"title":"Test"}}]`
	if err := os.WriteFile(filePath, []byte(payload), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	batchFile = filePath
	batchDryRun = true
	defer func() {
		batchFile = ""
		batchDryRun = false
	}()

	if err := runBatch(batchCmd, []string{}); err != nil {
		t.Fatalf("runBatch failed: %v", err)
	}
}

func TestParseImportOrder(t *testing.T) {
	order, err := parseImportOrder("first")
	if err != nil || order != "first" {
		t.Fatalf("unexpected order: %v %v", order, err)
	}

	order, err = parseImportOrder("2")
	if err != nil || order.(int) != 2 {
		t.Fatalf("unexpected order: %v %v", order, err)
	}

	if _, err := parseImportOrder("bad"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseMarkdownAndImportBlocks(t *testing.T) {
	markdown := "# Heading\n- Bullet\n  - Child\n1. Numbered\n"
	blocks := parseMarkdown(markdown)
	if len(blocks) == 0 {
		t.Fatal("expected parsed blocks")
	}
	if count := countMarkdownBlocks(blocks); count != 4 {
		t.Fatalf("expected 4 blocks, got %d", count)
	}

	createCalls := 0
	fake := &fakeClient{
		CreateBlockFunc: func(parentUID, content string, order interface{}) error {
			createCalls++
			return nil
		},
		SearchBlocksFunc: func(text string, limit int) ([][]interface{}, error) {
			return [][]interface{}{{"child-parent"}}, nil
		},
	}

	count, err := importBlocks(fake, "parent", blocks, "first")
	if err != nil {
		t.Fatalf("importBlocks failed: %v", err)
	}
	if count != 4 || createCalls < 4 {
		t.Fatalf("expected 4 created blocks, got count=%d calls=%d", count, createCalls)
	}
}
