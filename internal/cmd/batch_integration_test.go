package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/output"
)

func TestRunBatchExecute(t *testing.T) {
	fake := &fakeClient{
		CreateBlockAtLocationFunc:  func(loc api.Location, opts api.BlockOptions) error { return nil },
		UpdateBlockWithOptionsFunc: func(uid string, opts api.BlockOptions) error { return nil },
		MoveBlockToLocationFunc:    func(uid string, loc api.Location) error { return nil },
		DeleteBlockFunc:            func(uid string) error { return nil },
		CreatePageWithOptionsFunc:  func(opts api.PageOptions) error { return nil },
		UpdatePageWithOptionsFunc:  func(uid string, opts api.PageOptions) error { return nil },
		DeletePageFunc:             func(uid string) error { return nil },
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(batchCmd)

	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "batch.json")
	payload := `[
		{"action":"create-page","page":{"title":"Test"}},
		{"action":"create-block","location":{"parent-uid":"p","order":"last"},"block":{"string":"hi"}},
		{"action":"update-block","block":{"uid":"u1","string":"upd"}},
		{"action":"move-block","block":{"uid":"u1"},"location":{"parent-uid":"p2","order":"first"}},
		{"action":"delete-block","block":{"uid":"u1"}},
		{"action":"update-page","page":{"uid":"p","title":"New"}},
		{"action":"delete-page","page":{"uid":"p"}}
	]`
	if err := os.WriteFile(filePath, []byte(payload), 0o644); err != nil {
		t.Fatalf("write batch file: %v", err)
	}

	batchFile = filePath
	batchDryRun = false
	batchNative = false
	defer func() {
		batchFile = ""
		batchDryRun = false
		batchNative = false
	}()

	if err := runBatch(batchCmd, []string{}); err != nil {
		t.Fatalf("runBatch failed: %v", err)
	}
}

func TestRunBatchNative(t *testing.T) {
	fake := &fakeClient{
		ExecuteBatchFunc: func(batch *api.BatchBuilder) error { return nil },
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(batchCmd)

	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "batch.json")
	payload := `[{"action":"create-page","page":{"title":"Test"}}]`
	if err := os.WriteFile(filePath, []byte(payload), 0o644); err != nil {
		t.Fatalf("write batch file: %v", err)
	}

	batchFile = filePath
	batchDryRun = false
	batchNative = true
	defer func() {
		batchFile = ""
		batchDryRun = false
		batchNative = false
	}()

	if err := runBatch(batchCmd, []string{}); err != nil {
		t.Fatalf("runBatch native failed: %v", err)
	}
}

func TestRunImportExecute(t *testing.T) {
	fake := &fakeClient{}
	fake.GetPageByTitleFunc = func(title string) (json.RawMessage, error) {
		return json.RawMessage(`{"node/title":"Test","block/uid":"page-uid"}`), nil
	}
	fake.CreatePageFunc = func(title string) error { return nil }
	fake.CreateBlockFunc = func(parentUID, content string, order interface{}) error { return nil }
	fake.SearchBlocksFunc = func(text string, limit int) ([][]interface{}, error) {
		return [][]interface{}{{"child-uid"}}, nil
	}

	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(importCmd)

	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "import.md")
	if err := os.WriteFile(filePath, []byte("- Parent\n  - Child\n"), 0o644); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	importPage = "Test"
	importParent = ""
	importOrder = "last"
	importDryRun = false
	defer func() {
		importPage = ""
		importParent = ""
		importOrder = "last"
		importDryRun = false
	}()

	if err := runImport(importCmd, []string{filePath}); err != nil {
		t.Fatalf("runImport failed: %v", err)
	}
}
