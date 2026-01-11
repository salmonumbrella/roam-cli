package api

import (
	"encoding/json"
	"testing"
)

func TestLocation_ToMap_ParentUID(t *testing.T) {
	loc := Location{
		ParentUID: "abc123",
		Order:     "last",
	}

	m := loc.ToMap()

	if m["parent-uid"] != "abc123" {
		t.Errorf("Expected parent-uid 'abc123', got %v", m["parent-uid"])
	}
	if m["order"] != "last" {
		t.Errorf("Expected order 'last', got %v", m["order"])
	}
	if _, exists := m["page-title"]; exists {
		t.Error("Expected no page-title when using parent-uid")
	}
}

func TestLocation_ToMap_PageTitle(t *testing.T) {
	loc := Location{
		PageTitle: "My Page",
		Order:     0,
	}

	m := loc.ToMap()

	if m["page-title"] != "My Page" {
		t.Errorf("Expected page-title 'My Page', got %v", m["page-title"])
	}
	if m["order"] != 0 {
		t.Errorf("Expected order 0, got %v", m["order"])
	}
}

func TestLocation_ToMap_DailyNote(t *testing.T) {
	loc := Location{
		DailyNoteDate: "01-15-2025",
		Order:         "first",
	}

	m := loc.ToMap()

	pageTitle, ok := m["page-title"].(map[string]string)
	if !ok {
		t.Fatalf("Expected page-title to be map, got %T", m["page-title"])
	}
	if pageTitle["daily-note-page"] != "01-15-2025" {
		t.Errorf("Expected daily-note-page '01-15-2025', got %v", pageTitle["daily-note-page"])
	}
}

func TestLocation_JSON_DailyNote(t *testing.T) {
	loc := Location{
		DailyNoteDate: "01-15-2025",
		Order:         "last",
	}

	m := loc.ToMap()
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	expected := `{"order":"last","page-title":{"daily-note-page":"01-15-2025"}}`
	if string(jsonBytes) != expected {
		t.Errorf("Expected JSON %s, got %s", expected, string(jsonBytes))
	}
}
