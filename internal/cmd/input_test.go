package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadInputSource_Empty(t *testing.T) {
	_, err := readInputSource("", nil)
	if err == nil {
		t.Fatal("expected error for empty source")
	}
	if !strings.Contains(err.Error(), "empty input source") {
		t.Errorf("expected 'empty input source' error, got %v", err)
	}
}

func TestReadInputSource_Whitespace(t *testing.T) {
	_, err := readInputSource("   ", nil)
	if err == nil {
		t.Fatal("expected error for whitespace-only source")
	}
	if !strings.Contains(err.Error(), "empty input source") {
		t.Errorf("expected 'empty input source' error, got %v", err)
	}
}

func TestReadInputSource_File(t *testing.T) {
	// Create a temp file with content
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	content := "hello world\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	got, err := readInputSource(filePath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should be trimmed
	want := "hello world"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadInputSource_FileWithWhitespacePath(t *testing.T) {
	// Create a temp file
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	content := "test content"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Path with leading/trailing whitespace should be trimmed
	got, err := readInputSource("  "+filePath+"  ", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestReadInputSource_FileNotFound(t *testing.T) {
	_, err := readInputSource("/nonexistent/path/to/file.txt", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to read") {
		t.Errorf("expected 'failed to read' error, got %v", err)
	}
}

func TestReadInputSource_FileMultiLine(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "multiline.txt")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	got, err := readInputSource(filePath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Trailing newlines should be trimmed
	want := "line1\nline2\nline3"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadInputSource_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	got, err := readInputSource(filePath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestReadInputSource_FileWithOnlyWhitespace(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "whitespace.txt")
	if err := os.WriteFile(filePath, []byte("   \n\t\n  "), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	got, err := readInputSource(filePath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All whitespace should be trimmed
	if got != "" {
		t.Errorf("expected empty string after trim, got %q", got)
	}
}

func TestReadInputSource_Stdin(t *testing.T) {
	stdinContent := "content from stdin\n"
	stdin := strings.NewReader(stdinContent)

	got, err := readInputSource("-", stdin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "content from stdin"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadInputSource_StdinWithWhitespace(t *testing.T) {
	stdin := strings.NewReader("stdin content")

	// " - " should be trimmed to "-" which means stdin
	got, err := readInputSource(" - ", stdin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "stdin content"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInputHasData_StringReader(t *testing.T) {
	r := strings.NewReader("data")
	if !inputHasData(r) {
		t.Error("expected true for strings.Reader")
	}
}

func TestInputHasData_Pipe(t *testing.T) {
	// Create a pipe - this is a file but not a terminal
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer pr.Close()
	defer pw.Close()

	// Write some data to make it readable
	go func() {
		pw.Write([]byte("data"))
		pw.Close()
	}()

	// Pipe should be detected as having data (not a char device)
	if !inputHasData(pr) {
		t.Error("expected true for pipe (not a char device)")
	}
}

func TestInputHasData_BytesBuffer(t *testing.T) {
	// bytes.Buffer is not an *os.File, so should return true
	buf := &bytes.Buffer{}
	buf.WriteString("data")
	if !inputHasData(buf) {
		t.Error("expected true for bytes.Buffer (non-file reader)")
	}
}

func TestInputHasData_EmptyBytesBuffer(t *testing.T) {
	// Empty bytes.Buffer is still not an *os.File, so returns true.
	// The function checks reader TYPE, not content availability.
	buf := &bytes.Buffer{}
	if !inputHasData(buf) {
		t.Error("expected true for empty bytes.Buffer (non-file reader returns true regardless of content)")
	}
}
