package roamdb

import (
	"strings"
	"testing"
	"time"
)

func TestEscapeString(t *testing.T) {
	got := EscapeString(`a"b"c`)
	if got != `a""b""c` {
		t.Fatalf("expected escaped string, got %q", got)
	}
}

func TestQueryPageByTitle(t *testing.T) {
	query := QueryPageByTitle(`My "Page"`)
	if !strings.Contains(query, `:node/title "My ""Page"""`) {
		t.Fatalf("unexpected query: %s", query)
	}
}

func TestQueryBlockByUID(t *testing.T) {
	query := QueryBlockByUID(`abc"123`)
	if !strings.Contains(query, `:block/uid "abc""123"`) {
		t.Fatalf("unexpected query: %s", query)
	}
}

func TestQueryListPages(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	query := QueryListPages(true, now)
	if !strings.Contains(query, ":edit/time") {
		t.Fatalf("expected edit time filter in query: %s", query)
	}
}
