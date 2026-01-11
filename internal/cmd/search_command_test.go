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
