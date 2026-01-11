package cmd

import (
	"testing"
	"time"
)

func TestFormatCategories(t *testing.T) {
	got := formatCategories("hello", "work,  ,personal")
	if got != "hello #[[work]] #[[personal]]" {
		t.Fatalf("unexpected categories: %q", got)
	}

	got = formatCategories("hello", "")
	if got != "hello" {
		t.Fatalf("expected unchanged text, got %q", got)
	}
}

func TestDailyHelpers(t *testing.T) {
	if ordinalSuffix(1) != "st" || ordinalSuffix(2) != "nd" || ordinalSuffix(3) != "rd" || ordinalSuffix(4) != "th" {
		t.Fatal("unexpected ordinal suffix")
	}
	if ordinalSuffix(11) != "th" || ordinalSuffix(12) != "th" || ordinalSuffix(13) != "th" {
		t.Fatal("unexpected teen ordinal suffix")
	}

	d := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)
	if got := formatDailyNoteTitle(d); got != "January 2nd, 2026" {
		t.Fatalf("unexpected daily title: %s", got)
	}

	parsed, err := parseDate("2026-01-02")
	if err != nil || parsed.Year() != 2026 || parsed.Month() != time.January || parsed.Day() != 2 {
		t.Fatalf("unexpected parseDate: %v %v", parsed, err)
	}

	if _, err := parseDate("not-a-date"); err == nil {
		t.Fatal("expected parse error")
	}
}
