package cmd

import "testing"

func TestPaginateResults(t *testing.T) {
	results := [][]interface{}{{1}, {2}, {3}, {4}, {5}}

	page1, total, pageUsed := paginateResults(results, 1, 2)
	if total != 5 || pageUsed != 1 || len(page1) != 2 {
		t.Fatalf("unexpected page1 results: total=%d page=%d len=%d", total, pageUsed, len(page1))
	}

	page3, total, pageUsed := paginateResults(results, 3, 2)
	if total != 5 || pageUsed != 3 || len(page3) != 1 {
		t.Fatalf("unexpected page3 results: total=%d page=%d len=%d", total, pageUsed, len(page3))
	}

	page4, total, pageUsed := paginateResults(results, 4, 2)
	if total != 5 || pageUsed != 4 || len(page4) != 0 {
		t.Fatalf("unexpected page4 results: total=%d page=%d len=%d", total, pageUsed, len(page4))
	}
}
