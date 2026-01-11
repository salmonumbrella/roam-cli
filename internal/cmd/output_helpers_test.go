package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/output"
	"gopkg.in/yaml.v3"
)

func TestStructuredOutputRequested(t *testing.T) {
	tests := []struct {
		format output.Format
		want   bool
	}{
		{output.FormatText, false},
		{output.FormatJSON, true},
		{output.FormatNDJSON, true},
		{output.FormatYAML, true},
		{output.FormatTable, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			_, _, cleanup := withTestContext(t, tt.format, false)
			defer cleanup()

			got := structuredOutputRequested()
			if got != tt.want {
				t.Errorf("structuredOutputRequested() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrintStructured_JSON(t *testing.T) {
	out, _, cleanup := withTestContext(t, output.FormatJSON, false)
	defer cleanup()

	data := map[string]string{"key": "value"}
	if err := printStructured(data); err != nil {
		t.Fatalf("printStructured() error = %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput was: %s", err, out.String())
	}

	if result["key"] != "value" {
		t.Errorf("expected key='value', got key=%q", result["key"])
	}
}

func TestPrintStructured_YAML(t *testing.T) {
	out, _, cleanup := withTestContext(t, output.FormatYAML, false)
	defer cleanup()

	data := map[string]string{"key": "value"}
	if err := printStructured(data); err != nil {
		t.Fatalf("printStructured() error = %v", err)
	}

	var result map[string]string
	if err := yaml.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse YAML output: %v\nOutput was: %s", err, out.String())
	}

	if result["key"] != "value" {
		t.Errorf("expected key='value', got key=%q", result["key"])
	}
}

func TestPrintRawStructured_ValidJSON(t *testing.T) {
	out, _, cleanup := withTestContext(t, output.FormatJSON, false)
	defer cleanup()

	raw := []byte(`{"foo":"bar"}`)
	if err := printRawStructured(raw); err != nil {
		t.Fatalf("printRawStructured() error = %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput was: %s", err, out.String())
	}

	if result["foo"] != "bar" {
		t.Errorf("expected foo='bar', got foo=%q", result["foo"])
	}
}

func TestPrintRawStructured_ValidYAML(t *testing.T) {
	out, _, cleanup := withTestContext(t, output.FormatYAML, false)
	defer cleanup()

	raw := []byte(`{"foo":"bar"}`)
	if err := printRawStructured(raw); err != nil {
		t.Fatalf("printRawStructured() error = %v", err)
	}

	var result map[string]string
	if err := yaml.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse YAML output: %v\nOutput was: %s", err, out.String())
	}

	if result["foo"] != "bar" {
		t.Errorf("expected foo='bar', got foo=%q", result["foo"])
	}
}

func TestPrintRawStructured_InvalidJSON(t *testing.T) {
	_, _, cleanup := withTestContext(t, output.FormatJSON, false)
	defer cleanup()

	raw := []byte(`{invalid json}`)
	err := printRawStructured(raw)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse response") {
		t.Errorf("expected 'failed to parse response' error, got %v", err)
	}
}

func TestCurrentContext_WithRootCmd(t *testing.T) {
	_, _, cleanup := withTestContext(t, output.FormatText, false)
	defer cleanup()

	ctx := currentContext()
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	// Should have the format set
	format := output.FormatFromContext(ctx)
	if format != output.FormatText {
		t.Errorf("expected FormatText, got %v", format)
	}
}

func TestCurrentContext_NilRootCmd(t *testing.T) {
	// Save and restore rootCmd
	prevRootCmd := rootCmd
	rootCmd = nil
	defer func() { rootCmd = prevRootCmd }()

	ctx := currentContext()
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	// Should return context.Background()
	if ctx != context.Background() {
		t.Error("expected context.Background() when rootCmd is nil")
	}
}
