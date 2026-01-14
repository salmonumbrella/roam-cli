package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/output"
)

func TestPageGetRenderMarkdown(t *testing.T) {
	fake := &fakeClient{
		GetPageByTitleFunc: func(title string) (json.RawMessage, error) {
			return json.RawMessage(`{"node/title":"Test","block/uid":"uid","block/children":[{"block/string":"Child","block/uid":"c1"}]}`), nil
		},
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	out, _, restoreCtx := withTestContext(t, output.FormatText, true)
	defer restoreCtx()
	setCmdContext(pageGetCmd)

	if err := pageGetCmd.Flags().Set("render", "markdown"); err != nil {
		t.Fatalf("set flag failed: %v", err)
	}
	defer func() { _ = pageGetCmd.Flags().Set("render", "text") }()

	if err := pageGetCmd.RunE(pageGetCmd, []string{"Test"}); err != nil {
		t.Fatalf("page get failed: %v", err)
	}

	if !strings.Contains(out.String(), "- Child") {
		t.Fatalf("expected markdown output, got: %s", out.String())
	}
}

func TestPageListStructured(t *testing.T) {
	fake := &fakeClient{
		ListPagesFunc: func(modifiedToday bool, limit int) ([][]interface{}, error) {
			return [][]interface{}{
				{"B", "uid-b", float64(2)},
				{"A", "uid-a", float64(1)},
			}, nil
		},
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	out, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(pageListCmd)

	if err := pageListCmd.Flags().Set("sort", "title"); err != nil {
		t.Fatalf("set sort failed: %v", err)
	}
	if err := pageListCmd.Flags().Set("limit", "1"); err != nil {
		t.Fatalf("set limit failed: %v", err)
	}
	defer func() {
		_ = pageListCmd.Flags().Set("sort", "")
		_ = pageListCmd.Flags().Set("limit", "0")
	}()

	if err := pageListCmd.RunE(pageListCmd, []string{}); err != nil {
		t.Fatalf("page list failed: %v", err)
	}

	var parsed []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1 result, got %d", len(parsed))
	}
	if parsed[0]["title"] != "A" {
		t.Fatalf("expected sorted title A, got %v", parsed[0]["title"])
	}
}

func TestPageUpdateStructured(t *testing.T) {
	var gotUID string
	var gotTitle string
	fake := &fakeClient{
		UpdatePageWithOptionsFunc: func(uid string, opts api.PageOptions) error {
			gotUID = uid
			gotTitle = opts.Title
			return nil
		},
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(pageUpdateCmd)

	if err := pageUpdateCmd.Flags().Set("title", "New Title"); err != nil {
		t.Fatalf("set title failed: %v", err)
	}
	defer func() { _ = pageUpdateCmd.Flags().Set("title", "") }()

	if err := pageUpdateCmd.RunE(pageUpdateCmd, []string{"uid-1"}); err != nil {
		t.Fatalf("page update failed: %v", err)
	}
	if gotUID != "uid-1" || gotTitle != "New Title" {
		t.Fatalf("unexpected update: %s %s", gotUID, gotTitle)
	}
}

func TestPageFromMarkdownLocal(t *testing.T) {
	var received localRequest
	localClient := newTestLocalClient(t, func(req localRequest) localResponse {
		received = req
		return localResponse{Success: true}
	})
	restoreClient := withTestClient(t, localClient)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(pageFromMarkdownCmd)

	pageFromMarkdownContent = "# Heading"
	pageFromMarkdownUID = "page-uid"
	pageFromMarkdownChildrenView = "numbered"
	defer func() {
		pageFromMarkdownContent = ""
		pageFromMarkdownFile = ""
		pageFromMarkdownUID = ""
		pageFromMarkdownChildrenView = ""
	}()

	if err := pageFromMarkdownCmd.RunE(pageFromMarkdownCmd, []string{"My Page"}); err != nil {
		t.Fatalf("page from-markdown failed: %v", err)
	}

	if received.Action != "data.page.fromMarkdown" {
		t.Fatalf("expected action data.page.fromMarkdown, got %q", received.Action)
	}
	if len(received.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(received.Args))
	}
	argsMap, ok := received.Args[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected args[0] to be a map")
	}
	if argsMap["markdown-string"] != "# Heading" {
		t.Fatalf("expected markdown-string, got %v", argsMap["markdown-string"])
	}
	pageMap, ok := argsMap["page"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected page map in args")
	}
	if pageMap["title"] != "My Page" {
		t.Fatalf("expected page title 'My Page', got %v", pageMap["title"])
	}
	if pageMap["uid"] != "page-uid" {
		t.Fatalf("expected uid 'page-uid', got %v", pageMap["uid"])
	}
	if pageMap["children-view-type"] != "numbered" {
		t.Fatalf("expected children-view-type 'numbered', got %v", pageMap["children-view-type"])
	}
}

func TestPageFromMarkdownRequiresLocal(t *testing.T) {
	fake := &fakeClient{}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(pageFromMarkdownCmd)

	pageFromMarkdownContent = "# Heading"
	defer func() { pageFromMarkdownContent = "" }()

	if err := pageFromMarkdownCmd.RunE(pageFromMarkdownCmd, []string{"My Page"}); err == nil {
		t.Fatalf("expected error for non-local client")
	}
}

func TestPageFromMarkdownConflictFlags(t *testing.T) {
	fake := &fakeClient{}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(pageFromMarkdownCmd)

	pageFromMarkdownContent = "# Heading"
	pageFromMarkdownFile = "notes.md"
	defer func() {
		pageFromMarkdownContent = ""
		pageFromMarkdownFile = ""
	}()

	if err := pageFromMarkdownCmd.RunE(pageFromMarkdownCmd, []string{"My Page"}); err == nil {
		t.Fatalf("expected error for conflicting markdown flags")
	}
}

func TestPageFromMarkdownEmptyInput(t *testing.T) {
	fake := &fakeClient{}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(pageFromMarkdownCmd)

	in := &bytes.Buffer{}
	prevIn := pageFromMarkdownCmd.InOrStdin()
	pageFromMarkdownCmd.SetIn(in)
	defer pageFromMarkdownCmd.SetIn(prevIn)

	if err := pageFromMarkdownCmd.RunE(pageFromMarkdownCmd, []string{"My Page"}); err == nil {
		t.Fatalf("expected error for empty markdown input")
	}
}
