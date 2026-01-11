package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/output"
)

func TestSetVersionInfo(t *testing.T) {
	// Save original values
	origVersion := version
	origCommit := commit
	origDate := date
	defer func() {
		version = origVersion
		commit = origCommit
		date = origDate
	}()

	SetVersionInfo("1.2.3", "abc123", "2025-01-01")

	if version != "1.2.3" {
		t.Errorf("version = %q, want '1.2.3'", version)
	}
	if commit != "abc123" {
		t.Errorf("commit = %q, want 'abc123'", commit)
	}
	if date != "2025-01-01" {
		t.Errorf("date = %q, want '2025-01-01'", date)
	}
}

func TestGetClient_ReturnsClient(t *testing.T) {
	// Save and restore
	prevClient := client
	defer func() { client = prevClient }()

	fc := &fakeClient{}
	client = fc

	got := GetClient()
	if got != fc {
		t.Error("GetClient() did not return the expected client")
	}
}

func TestGetClient_ReturnsNil(t *testing.T) {
	// Save and restore
	prevClient := client
	defer func() { client = prevClient }()

	client = nil

	got := GetClient()
	if got != nil {
		t.Errorf("GetClient() = %v, want nil", got)
	}
}

func TestGetOutputFormat_FromOutputType(t *testing.T) {
	// Save and restore
	prevType := outputType
	prevFmt := outputFmt
	defer func() {
		outputType = prevType
		outputFmt = prevFmt
	}()

	outputType = output.FormatJSON
	outputFmt = "text"

	got := GetOutputFormat()
	if got != output.FormatJSON {
		t.Errorf("GetOutputFormat() = %v, want %v", got, output.FormatJSON)
	}
}

func TestGetOutputFormat_FromOutputFmt(t *testing.T) {
	// Save and restore
	prevType := outputType
	prevFmt := outputFmt
	defer func() {
		outputType = prevType
		outputFmt = prevFmt
	}()

	outputType = ""
	outputFmt = "yaml"

	got := GetOutputFormat()
	if got != output.FormatYAML {
		t.Errorf("GetOutputFormat() = %v, want %v", got, output.FormatYAML)
	}
}

func TestGetOutputFormat_InvalidFmt(t *testing.T) {
	// Save and restore
	prevType := outputType
	prevFmt := outputFmt
	defer func() {
		outputType = prevType
		outputFmt = prevFmt
	}()

	outputType = ""
	outputFmt = "invalid"

	got := GetOutputFormat()
	if got != output.FormatText {
		t.Errorf("GetOutputFormat() = %v, want %v (default)", got, output.FormatText)
	}
}

func TestGetOutputFormatString_FromOutputType(t *testing.T) {
	// Save and restore
	prevType := outputType
	prevFmt := outputFmt
	defer func() {
		outputType = prevType
		outputFmt = prevFmt
	}()

	outputType = output.FormatNDJSON
	outputFmt = "text"

	got := GetOutputFormatString()
	if got != "ndjson" {
		t.Errorf("GetOutputFormatString() = %q, want 'ndjson'", got)
	}
}

func TestGetOutputFormatString_FromOutputFmt(t *testing.T) {
	// Save and restore
	prevType := outputType
	prevFmt := outputFmt
	defer func() {
		outputType = prevType
		outputFmt = prevFmt
	}()

	outputType = ""
	outputFmt = "table"

	got := GetOutputFormatString()
	if got != "table" {
		t.Errorf("GetOutputFormatString() = %q, want 'table'", got)
	}
}

func TestIsTerminal_Buffer(t *testing.T) {
	buf := &bytes.Buffer{}
	if isTerminal(buf) {
		t.Error("expected false for bytes.Buffer")
	}
}

func TestIsTerminal_File(t *testing.T) {
	// Create a temp file - not a terminal
	f, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if isTerminal(f) {
		t.Error("expected false for regular file")
	}
}

func TestIsTerminal_Nil(t *testing.T) {
	// nil writer is not an os.File
	if isTerminal(nil) {
		t.Error("expected false for nil")
	}
}
