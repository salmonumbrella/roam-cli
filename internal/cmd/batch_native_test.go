package cmd

import (
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/output"
)

func TestExecuteNativeBatch(t *testing.T) {
	fake := &fakeClient{
		ExecuteBatchFunc: func(batch *api.BatchBuilder) error { return nil },
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()

	actions := []BatchAction{
		{Action: "create-page", Page: map[string]interface{}{"title": "A", "uid": "temp1"}},
		{Action: "create-block", Location: map[string]interface{}{"parent-uid": "temp1", "order": "last"}, Block: map[string]interface{}{"string": "hi", "uid": "b1"}},
		{Action: "update-block", Block: map[string]interface{}{"uid": "b1", "string": "upd"}},
		{Action: "move-block", Block: map[string]interface{}{"uid": "b1"}, Location: map[string]interface{}{"parent-uid": "temp1", "order": "first"}},
		{Action: "delete-block", Block: map[string]interface{}{"uid": "b1"}},
		{Action: "update-page", Page: map[string]interface{}{"uid": "temp1", "title": "B"}},
		{Action: "delete-page", Page: map[string]interface{}{"uid": "temp1"}},
	}

	if err := executeNativeBatch(actions); err != nil {
		t.Fatalf("executeNativeBatch failed: %v", err)
	}
}
