package cmd

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/output"
)

func TestRunBlockCreateParent(t *testing.T) {
	var gotParent string
	var gotOpts api.BlockOptions
	var gotOrder interface{}

	fake := &fakeClient{
		CreateBlockWithOptionsFunc: func(parentUID string, opts api.BlockOptions, order interface{}) error {
			gotParent = parentUID
			gotOpts = opts
			gotOrder = order
			return nil
		},
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(blockCreateCmd)

	blockCreateParent = "parent-uid"
	blockCreateContent = "hello"
	blockCreateUID = "block-uid"
	blockCreateOrder = "last"
	blockCreateTextAlign = "center"
	blockCreateViewType = "bullet"
	blockCreateChildrenView = "numbered"
	defer func() {
		blockCreateParent = ""
		blockCreateContent = ""
		blockCreateUID = ""
		blockCreateOrder = "last"
		blockCreateTextAlign = ""
		blockCreateViewType = ""
		blockCreateChildrenView = ""
	}()

	if err := runBlockCreate(blockCreateCmd, nil); err != nil {
		t.Fatalf("runBlockCreate failed: %v", err)
	}

	if gotParent != "parent-uid" {
		t.Fatalf("expected parent uid, got %q", gotParent)
	}
	if gotOpts.Content != "hello" || gotOpts.UID != "block-uid" {
		t.Fatalf("unexpected opts: %+v", gotOpts)
	}
	if gotOrder != "last" {
		t.Fatalf("expected order 'last', got %v", gotOrder)
	}
}

func TestRunBlockCreatePageTitle(t *testing.T) {
	var gotLoc api.Location
	fake := &fakeClient{
		CreateBlockAtLocationFunc: func(loc api.Location, opts api.BlockOptions) error {
			gotLoc = loc
			return nil
		},
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(blockCreateCmd)

	blockCreatePageTitle = "My Page"
	blockCreateContent = "content"
	blockCreateOrder = "first"
	defer func() {
		blockCreatePageTitle = ""
		blockCreateContent = ""
		blockCreateOrder = "last"
	}()

	if err := runBlockCreate(blockCreateCmd, nil); err != nil {
		t.Fatalf("runBlockCreate failed: %v", err)
	}

	if gotLoc.PageTitle != "My Page" || gotLoc.Order != "first" {
		t.Fatalf("unexpected location: %+v", gotLoc)
	}
}

func TestRunBlockUpdate(t *testing.T) {
	var gotUID string
	var gotOpts api.BlockOptions
	fake := &fakeClient{
		UpdateBlockWithOptionsFunc: func(uid string, opts api.BlockOptions) error {
			gotUID = uid
			gotOpts = opts
			return nil
		},
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(blockUpdateCmd)

	blockUpdateContent = "updated"
	blockUpdateTextAlign = "right"
	defer func() {
		blockUpdateContent = ""
		blockUpdateTextAlign = ""
	}()

	if err := runBlockUpdate(blockUpdateCmd, []string{"uid-1"}); err != nil {
		t.Fatalf("runBlockUpdate failed: %v", err)
	}

	if gotUID != "uid-1" || gotOpts.Content != "updated" {
		t.Fatalf("unexpected update: %s %+v", gotUID, gotOpts)
	}
}

func TestRunBlockMove(t *testing.T) {
	var gotUID string
	var gotLoc api.Location
	fake := &fakeClient{
		MoveBlockToLocationFunc: func(uid string, loc api.Location) error {
			gotUID = uid
			gotLoc = loc
			return nil
		},
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(blockMoveCmd)

	blockMoveParent = "parent-1"
	blockMoveOrder = "first"
	defer func() {
		blockMoveParent = ""
		blockMoveOrder = "last"
	}()

	if err := runBlockMove(blockMoveCmd, []string{"uid-2"}); err != nil {
		t.Fatalf("runBlockMove failed: %v", err)
	}

	if gotUID != "uid-2" || gotLoc.ParentUID != "parent-1" || gotLoc.Order != "first" {
		t.Fatalf("unexpected move: uid=%s loc=%+v", gotUID, gotLoc)
	}
}

func TestRunBlockDeleteStructured(t *testing.T) {
	var deleted string
	fake := &fakeClient{
		DeleteBlockFunc: func(uid string) error {
			deleted = uid
			return nil
		},
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(blockDeleteCmd)

	if err := runBlockDelete(blockDeleteCmd, []string{"uid-3"}); err != nil {
		t.Fatalf("runBlockDelete failed: %v", err)
	}
	if deleted != "uid-3" {
		t.Fatalf("expected delete call, got %q", deleted)
	}
}

func TestRunBlockDeletePrompt(t *testing.T) {
	deleted := false
	fake := &fakeClient{
		GetBlockByUIDFunc: func(uid string) (json.RawMessage, error) {
			return json.RawMessage(`{"block/string":"hello","block/uid":"uid-4"}`), nil
		},
		DeleteBlockFunc: func(uid string) error {
			deleted = true
			return nil
		},
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatText, false)
	defer restoreCtx()
	setCmdContext(blockDeleteCmd)

	if err := runBlockDelete(blockDeleteCmd, []string{"uid-4"}); err != nil {
		t.Fatalf("runBlockDelete failed: %v", err)
	}
	if deleted {
		t.Fatalf("expected delete to be skipped without --yes")
	}
}

func TestBlockFromMarkdownLocal(t *testing.T) {
	var received localRequest
	localClient := newTestLocalClient(t, func(req localRequest) localResponse {
		received = req
		return localResponse{Success: true}
	})
	restoreClient := withTestClient(t, localClient)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(blockFromMarkdownCmd)

	blockFromMarkdownParent = "parent-uid"
	blockFromMarkdownContent = "- Item"
	blockFromMarkdownOrder = "2"
	defer func() {
		blockFromMarkdownParent = ""
		blockFromMarkdownPageTitle = ""
		blockFromMarkdownDailyNote = ""
		blockFromMarkdownOrder = "last"
		blockFromMarkdownContent = ""
		blockFromMarkdownFile = ""
	}()

	if err := blockFromMarkdownCmd.RunE(blockFromMarkdownCmd, nil); err != nil {
		t.Fatalf("block from-markdown failed: %v", err)
	}

	if received.Action != "data.block.fromMarkdown" {
		t.Fatalf("expected action data.block.fromMarkdown, got %q", received.Action)
	}
	if len(received.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(received.Args))
	}
	argsMap, ok := received.Args[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected args[0] to be a map")
	}
	if argsMap["markdown-string"] != "- Item" {
		t.Fatalf("expected markdown-string '- Item', got %v", argsMap["markdown-string"])
	}
	location, ok := argsMap["location"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected location map in args")
	}
	if location["parent-uid"] != "parent-uid" {
		t.Fatalf("expected parent-uid 'parent-uid', got %v", location["parent-uid"])
	}
	if location["order"] != float64(2) {
		t.Fatalf("expected order 2, got %v", location["order"])
	}
}

func TestBlockFromMarkdownRequiresLocal(t *testing.T) {
	fake := &fakeClient{}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(blockFromMarkdownCmd)

	blockFromMarkdownParent = "parent-uid"
	blockFromMarkdownContent = "- Item"
	defer func() {
		blockFromMarkdownParent = ""
		blockFromMarkdownContent = ""
	}()

	if err := blockFromMarkdownCmd.RunE(blockFromMarkdownCmd, nil); err == nil {
		t.Fatalf("expected error for non-local client")
	}
}

func TestBlockFromMarkdownConflictFlags(t *testing.T) {
	fake := &fakeClient{}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(blockFromMarkdownCmd)

	blockFromMarkdownParent = "parent-uid"
	blockFromMarkdownContent = "- Item"
	blockFromMarkdownFile = "notes.md"
	defer func() {
		blockFromMarkdownParent = ""
		blockFromMarkdownContent = ""
		blockFromMarkdownFile = ""
	}()

	if err := blockFromMarkdownCmd.RunE(blockFromMarkdownCmd, nil); err == nil {
		t.Fatalf("expected error for conflicting markdown flags")
	}
}

func TestParsePropsJSON(t *testing.T) {
	props, err := parsePropsJSON(`{"a":1}`)
	if err != nil {
		t.Fatalf("parsePropsJSON failed: %v", err)
	}
	if props["a"].(float64) != 1 {
		t.Fatalf("unexpected props: %+v", props)
	}

	if _, err := parsePropsJSON("not-json"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if props, err := parsePropsJSON(""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if props != nil {
		t.Fatalf("expected nil props")
	}
}

func TestRunBlockUpdateRequiresFields(t *testing.T) {
	if err := runBlockUpdate(blockUpdateCmd, []string{"uid"}); err == nil {
		t.Fatal("expected error when no fields provided")
	}

	blockUpdateContent = "x"
	defer func() { blockUpdateContent = "" }()

	fake := &fakeClient{
		UpdateBlockWithOptionsFunc: func(uid string, opts api.BlockOptions) error {
			return errors.New("boom")
		},
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(blockUpdateCmd)

	if err := runBlockUpdate(blockUpdateCmd, []string{"uid"}); err == nil {
		t.Fatal("expected error from client")
	}
}
