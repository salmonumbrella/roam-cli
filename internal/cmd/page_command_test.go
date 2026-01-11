package cmd

import (
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
