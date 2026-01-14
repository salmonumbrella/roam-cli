package cmd

import (
	"encoding/json"
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/output"
)

func TestSearchStructured(t *testing.T) {
	fake := &fakeClient{
		QueryFunc: func(query string, args ...interface{}) ([][]interface{}, error) {
			return [][]interface{}{{"uid1", "hello", "Page", "page-uid"}}, nil
		},
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	out, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(searchCmd)

	searchPage = 1
	searchLimit = 50
	searchCaseSensitive = false
	defer func() {
		searchPage = 1
		searchLimit = 50
		searchCaseSensitive = false
	}()

	if err := runSearch(searchCmd, []string{"hello"}); err != nil {
		t.Fatalf("search failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if parsed["count"].(float64) != 1 {
		t.Fatalf("expected 1 result, got %v", parsed["count"])
	}
}

func TestSearchUIStructuredLocal(t *testing.T) {
	var received localRequest
	localClient := newTestLocalClient(t, func(req localRequest) localResponse {
		received = req
		return localResponse{Success: true, Result: json.RawMessage(`[]`)}
	})
	restoreClient := withTestClient(t, localClient)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(searchUICmd)

	searchUIBlocks = false
	searchUIPages = true
	searchUIHideCode = true
	searchUILimit = 10
	searchUIPull = "[:block/uid]"
	defer func() {
		searchUIBlocks = true
		searchUIPages = true
		searchUIHideCode = false
		searchUILimit = 300
		searchUIPull = ""
	}()

	if err := searchUICmd.RunE(searchUICmd, []string{"query"}); err != nil {
		t.Fatalf("search ui failed: %v", err)
	}

	if received.Action != "data.search" {
		t.Fatalf("expected action data.search, got %q", received.Action)
	}
	if len(received.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(received.Args))
	}
	argsMap, ok := received.Args[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected args[0] to be a map")
	}
	if argsMap["search-str"] != "query" {
		t.Fatalf("expected search-str 'query', got %v", argsMap["search-str"])
	}
	if argsMap["search-blocks"] != false {
		t.Fatalf("expected search-blocks false, got %v", argsMap["search-blocks"])
	}
	if argsMap["search-pages"] != true {
		t.Fatalf("expected search-pages true, got %v", argsMap["search-pages"])
	}
	if argsMap["hide-code-blocks"] != true {
		t.Fatalf("expected hide-code-blocks true, got %v", argsMap["hide-code-blocks"])
	}
	if argsMap["limit"] != float64(10) {
		t.Fatalf("expected limit 10, got %v", argsMap["limit"])
	}
	if argsMap["pull"] != "[:block/uid]" {
		t.Fatalf("expected pull pattern, got %v", argsMap["pull"])
	}
}

func TestSearchUIRequiresLocal(t *testing.T) {
	fake := &fakeClient{}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(searchUICmd)

	if err := searchUICmd.RunE(searchUICmd, []string{"query"}); err == nil {
		t.Fatalf("expected error for non-local client")
	}
}

func TestSearchUIRequiresFlags(t *testing.T) {
	localClient := newTestLocalClient(t, func(req localRequest) localResponse {
		return localResponse{Success: true, Result: json.RawMessage(`[]`)}
	})
	restoreClient := withTestClient(t, localClient)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(searchUICmd)

	searchUIBlocks = false
	searchUIPages = false
	defer func() {
		searchUIBlocks = true
		searchUIPages = true
	}()

	if err := searchUICmd.RunE(searchUICmd, []string{"query"}); err == nil {
		t.Fatalf("expected error when both search-blocks and search-pages are false")
	}
}
