package api

import "testing"

func TestPageOptions_ToPage(t *testing.T) {
	opts := PageOptions{
		Title:            "Test Page",
		ChildrenViewType: "numbered",
	}

	page := opts.ToPage()

	if page.Title != "Test Page" {
		t.Errorf("Expected Title 'Test Page', got '%s'", page.Title)
	}
	if page.ChildrenViewType != "numbered" {
		t.Errorf("Expected ChildrenViewType 'numbered', got '%s'", page.ChildrenViewType)
	}
}
