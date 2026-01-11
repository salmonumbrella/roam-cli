package cmd

import (
	"encoding/json"
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/output"
)

func TestParseEntityID(t *testing.T) {
	id, err := parseEntityID("123")
	if err != nil || id.(int64) != 123 {
		t.Fatalf("unexpected numeric parse: %v %v", id, err)
	}

	lookup, err := parseEntityID(`[:block/uid "abc"]`)
	if err != nil {
		t.Fatalf("unexpected lookup err: %v", err)
	}
	arr := lookup.([]interface{})
	if arr[0].(string) != ":block/uid" || arr[1].(string) != "abc" {
		t.Fatalf("unexpected lookup: %#v", arr)
	}

	if _, err := parseEntityID("bad"); err == nil {
		t.Fatal("expected error for invalid entity id")
	}
}

func TestRunQueryPullStructured(t *testing.T) {
	fake := &fakeClient{
		QueryFunc: func(query string, args ...interface{}) ([][]interface{}, error) {
			return [][]interface{}{{"x"}}, nil
		},
		PullFunc: func(eid interface{}, selector string) (json.RawMessage, error) {
			return json.RawMessage(`{"block/string":"hi"}`), nil
		},
		PullManyFunc: func(eids []interface{}, selector string) (json.RawMessage, error) {
			return json.RawMessage(`[{"block/string":"a"},{"block/string":"b"}]`), nil
		},
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	out, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(queryCmd)
	setCmdContext(pullCmd)
	setCmdContext(pullManyCmd)

	if err := runQuery(queryCmd, []string{"[:find ?e]"}); err != nil {
		t.Fatalf("runQuery failed: %v", err)
	}
	if err := runPull(pullCmd, []string{"123"}); err != nil {
		t.Fatalf("runPull failed: %v", err)
	}
	if err := runPullMany(pullManyCmd, []string{"123", "456"}); err != nil {
		t.Fatalf("runPullMany failed: %v", err)
	}

	if len(out.Bytes()) == 0 {
		t.Fatal("expected output")
	}
}
