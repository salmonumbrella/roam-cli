package outfmt

import (
	"strings"
	"testing"
)

func TestPrintJSON(t *testing.T) {
	var sb strings.Builder
	p := &Printer{Format: FormatJSON, Writer: &sb}

	if err := p.Print(map[string]int{"a": 1}); err != nil {
		t.Fatalf("Print JSON failed: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "\"a\": 1") {
		t.Fatalf("unexpected json output: %s", out)
	}
}

func TestPrintYAML(t *testing.T) {
	var sb strings.Builder
	p := &Printer{Format: FormatYAML, Writer: &sb}

	if err := p.Print(map[string]int{"a": 1}); err != nil {
		t.Fatalf("Print YAML failed: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "a: 1") {
		t.Fatalf("unexpected yaml output: %s", out)
	}
}

func TestPrintTable(t *testing.T) {
	var sb strings.Builder
	p := &Printer{Format: FormatTable, Writer: &sb}

	headers := []string{"A", "B"}
	rows := [][]string{{"1", "2"}, {"3", "4"}}
	p.PrintTable(headers, rows)

	out := sb.String()
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Fatalf("unexpected table headers: %s", out)
	}
	if !strings.Contains(out, "1") || !strings.Contains(out, "4") {
		t.Fatalf("unexpected table rows: %s", out)
	}
}
