package cmd

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/salmonumbrella/roam-cli/internal/output"
)

func TestGetOrCreatePageUID(t *testing.T) {
	// Existing page
	fake := &fakeClient{
		QueryFunc: func(query string, args ...interface{}) ([][]interface{}, error) {
			return [][]interface{}{{"uid-1"}}, nil
		},
	}
	uid, err := getOrCreatePageUID(fake, "Page")
	if err != nil || uid != "uid-1" {
		t.Fatalf("unexpected result: %v %v", uid, err)
	}

	// Create path
	calls := 0
	fake = &fakeClient{
		QueryFunc: func(query string, args ...interface{}) ([][]interface{}, error) {
			calls++
			if calls == 1 {
				return nil, nil
			}
			return [][]interface{}{{"uid-2"}}, nil
		},
		CreatePageFunc: func(title string) error { return nil },
	}
	uid, err = getOrCreatePageUID(fake, "New Page")
	if err != nil || uid != "uid-2" {
		t.Fatalf("unexpected create result: %v %v", uid, err)
	}
}

func TestFindOrCreateHeading(t *testing.T) {
	// Existing heading
	fake := &fakeClient{
		QueryFunc: func(query string, args ...interface{}) ([][]interface{}, error) {
			return [][]interface{}{{"heading-uid"}}, nil
		},
	}
	uid, err := findOrCreateHeading(fake, "page-uid", "TODO")
	if err != nil || uid != "heading-uid" {
		t.Fatalf("unexpected heading result: %v %v", uid, err)
	}

	// Create heading
	calls := 0
	fake = &fakeClient{
		QueryFunc: func(query string, args ...interface{}) ([][]interface{}, error) {
			calls++
			if calls == 1 {
				return nil, nil
			}
			return [][]interface{}{{"new-heading"}}, nil
		},
		CreateBlockFunc: func(parentUID, content string, order interface{}) error { return nil },
	}
	uid, err = findOrCreateHeading(fake, "page-uid", "TODO")
	if err != nil || uid != "new-heading" {
		t.Fatalf("unexpected heading create: %v %v", uid, err)
	}
}

func TestAddDailyBlock(t *testing.T) {
	fake := &fakeClient{
		QueryFunc: func(query string, args ...interface{}) ([][]interface{}, error) {
			if strings.Contains(query, ":node/title") {
				return [][]interface{}{{"page-uid"}}, nil
			}
			if strings.Contains(query, ":block/string") {
				return [][]interface{}{{"heading-uid"}}, nil
			}
			return nil, nil
		},
		CreateBlockFunc: func(parentUID, content string, order interface{}) error {
			if parentUID != "heading-uid" || content != "text" {
				return errors.New("unexpected block create")
			}
			return nil
		},
	}

	date := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)
	if _, err := addDailyBlock(fake, date, "text", "TODO"); err != nil {
		t.Fatalf("addDailyBlock failed: %v", err)
	}
}

func TestDailyContextStructured(t *testing.T) {
	fake := &fakeClient{
		GetPageByTitleFunc: func(title string) (json.RawMessage, error) {
			return json.RawMessage(`{"node/title":"Test","block/uid":"uid"}`), nil
		},
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	out, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(dailyContextCmd)

	if err := dailyContextCmd.Flags().Set("days", "1"); err != nil {
		t.Fatalf("set days failed: %v", err)
	}
	defer func() { _ = dailyContextCmd.Flags().Set("days", "3") }()

	if err := dailyContextCmd.RunE(dailyContextCmd, []string{}); err != nil {
		t.Fatalf("daily context failed: %v", err)
	}
	if len(out.Bytes()) == 0 {
		t.Fatal("expected output")
	}
}

func TestRememberStructured(t *testing.T) {
	fake := &fakeClient{
		QueryFunc: func(query string, args ...interface{}) ([][]interface{}, error) {
			return [][]interface{}{{"page-uid"}}, nil
		},
		CreateBlockFunc: func(parentUID, content string, order interface{}) error { return nil },
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(rememberCmd)

	if err := rememberCmd.RunE(rememberCmd, []string{"note"}); err != nil {
		t.Fatalf("remember failed: %v", err)
	}
}

func TestDailyAddStructured(t *testing.T) {
	fake := &fakeClient{
		QueryFunc: func(query string, args ...interface{}) ([][]interface{}, error) {
			return [][]interface{}{{"page-uid"}}, nil
		},
		CreateBlockFunc: func(parentUID, content string, order interface{}) error { return nil },
	}
	restoreClient := withTestClient(t, fake)
	defer restoreClient()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(dailyAddCmd)

	if err := dailyAddCmd.RunE(dailyAddCmd, []string{"text"}); err != nil {
		t.Fatalf("daily add failed: %v", err)
	}
}

func TestVerifyBlockCreated(t *testing.T) {
	fake := &fakeClient{
		QueryFunc: func(query string, args ...interface{}) ([][]interface{}, error) {
			return [][]interface{}{{"uid"}}, nil
		},
	}
	if !verifyBlockCreated(fake, "text") {
		t.Fatal("expected verification true")
	}
}
