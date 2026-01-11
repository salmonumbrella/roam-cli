package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/output"
)

func TestValidateErrorFormat(t *testing.T) {
	tests := []struct {
		format  string
		wantErr bool
	}{
		{"", false},
		{"auto", false},
		{"text", false},
		{"json", false},
		{"yaml", false},
		{"AUTO", false},   // case insensitive
		{"TEXT", false},   // case insensitive
		{" json ", false}, // whitespace trimmed
		{"invalid", true},
		{"xml", true},
		{"ndjson", true},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			err := validateErrorFormat(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateErrorFormat(%q) error = %v, wantErr %v", tt.format, err, tt.wantErr)
			}
		})
	}
}

func TestEffectiveErrorFormat(t *testing.T) {
	tests := []struct {
		name         string
		errorFormat  string
		outputFormat output.Format
		want         string
	}{
		{
			name:         "empty defaults to text",
			errorFormat:  "",
			outputFormat: output.FormatText,
			want:         "text",
		},
		{
			name:         "auto with json output",
			errorFormat:  "auto",
			outputFormat: output.FormatJSON,
			want:         "json",
		},
		{
			name:         "auto with ndjson output",
			errorFormat:  "auto",
			outputFormat: output.FormatNDJSON,
			want:         "json",
		},
		{
			name:         "auto with yaml output",
			errorFormat:  "auto",
			outputFormat: output.FormatYAML,
			want:         "yaml",
		},
		{
			name:         "auto with text output",
			errorFormat:  "auto",
			outputFormat: output.FormatText,
			want:         "text",
		},
		{
			name:         "explicit json overrides",
			errorFormat:  "json",
			outputFormat: output.FormatText,
			want:         "json",
		},
		{
			name:         "explicit yaml overrides",
			errorFormat:  "yaml",
			outputFormat: output.FormatText,
			want:         "yaml",
		},
		{
			name:         "explicit text overrides",
			errorFormat:  "text",
			outputFormat: output.FormatJSON,
			want:         "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = WithErrorFormat(ctx, tt.errorFormat)
			ctx = output.WithFormat(ctx, tt.outputFormat)

			got := effectiveErrorFormat(ctx)
			if got != tt.want {
				t.Errorf("effectiveErrorFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildErrorEnvelope(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantType     string
		wantCategory string
		wantSubtype  string
	}{
		{
			name:         "generic error",
			err:          errors.New("something went wrong"),
			wantType:     "error",
			wantCategory: "system",
		},
		{
			name:         "auth error",
			err:          api.AuthenticationError{Message: "invalid token"},
			wantType:     "auth",
			wantCategory: "user",
		},
		{
			name:         "validation error",
			err:          api.ValidationError{Message: "invalid input"},
			wantType:     "validation",
			wantCategory: "user",
		},
		{
			name:         "not found error",
			err:          api.NotFoundError{Message: "page not found"},
			wantType:     "not_found",
			wantCategory: "user",
		},
		{
			name:         "rate limit error",
			err:          api.RateLimitError{Message: "too many requests"},
			wantType:     "rate_limit",
			wantCategory: "system",
		},
		{
			name:         "local api error",
			err:          api.LocalAPIError{Message: "local error"},
			wantType:     "local_api",
			wantCategory: "system",
		},
		{
			name:         "local api timeout",
			err:          api.LocalAPIError{Message: "Response timeout"},
			wantType:     "local_api",
			wantCategory: "system",
			wantSubtype:  "response_timeout",
		},
		{
			name:         "desktop not running error",
			err:          api.DesktopNotRunningError{Message: "desktop not running"},
			wantType:     "desktop_not_running",
			wantCategory: "user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildErrorEnvelope(tt.err)

			errMap, ok := result["error"].(map[string]interface{})
			if !ok {
				t.Fatal("expected 'error' map in result")
			}

			if errMap["message"] != tt.err.Error() {
				t.Errorf("message = %v, want %v", errMap["message"], tt.err.Error())
			}

			if errMap["type"] != tt.wantType {
				t.Errorf("type = %v, want %v", errMap["type"], tt.wantType)
			}

			if errMap["category"] != tt.wantCategory {
				t.Errorf("category = %v, want %v", errMap["category"], tt.wantCategory)
			}

			if tt.wantSubtype != "" {
				if errMap["subtype"] != tt.wantSubtype {
					t.Errorf("subtype = %v, want %v", errMap["subtype"], tt.wantSubtype)
				}
			}
		})
	}
}

func TestPrintCommandError_Nil(t *testing.T) {
	errBuf := &bytes.Buffer{}
	ctx := withIO(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, errBuf)

	printCommandError(ctx, nil)

	if errBuf.Len() != 0 {
		t.Errorf("expected no output for nil error, got %q", errBuf.String())
	}
}

func TestPrintCommandError_Text(t *testing.T) {
	errBuf := &bytes.Buffer{}
	ctx := context.Background()
	ctx = withIO(ctx, &bytes.Buffer{}, &bytes.Buffer{}, errBuf)
	ctx = WithErrorFormat(ctx, "text")
	ctx = output.WithFormat(ctx, output.FormatText)

	testErr := errors.New("test error message")
	printCommandError(ctx, testErr)

	got := strings.TrimSpace(errBuf.String())
	if got != "test error message" {
		t.Errorf("expected %q, got %q", "test error message", got)
	}
}

func TestPrintCommandError_JSON(t *testing.T) {
	errBuf := &bytes.Buffer{}
	ctx := context.Background()
	ctx = withIO(ctx, &bytes.Buffer{}, &bytes.Buffer{}, errBuf)
	ctx = WithErrorFormat(ctx, "json")
	ctx = output.WithFormat(ctx, output.FormatText)

	testErr := api.AuthenticationError{Message: "auth failed"}
	printCommandError(ctx, testErr)

	var result map[string]interface{}
	if err := json.Unmarshal(errBuf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	errMap, ok := result["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'error' map in output")
	}

	if errMap["message"] != "auth failed" {
		t.Errorf("message = %v, want 'auth failed'", errMap["message"])
	}
	if errMap["type"] != "auth" {
		t.Errorf("type = %v, want 'auth'", errMap["type"])
	}
}

func TestPrintCommandError_YAML(t *testing.T) {
	errBuf := &bytes.Buffer{}
	ctx := context.Background()
	ctx = withIO(ctx, &bytes.Buffer{}, &bytes.Buffer{}, errBuf)
	ctx = WithErrorFormat(ctx, "yaml")
	ctx = output.WithFormat(ctx, output.FormatText)

	testErr := api.ValidationError{Message: "validation failed"}
	printCommandError(ctx, testErr)

	var result map[string]interface{}
	if err := yaml.Unmarshal(errBuf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse YAML output: %v", err)
	}

	errMap, ok := result["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'error' map in output")
	}

	if errMap["message"] != "validation failed" {
		t.Errorf("message = %v, want 'validation failed'", errMap["message"])
	}
	if errMap["type"] != "validation" {
		t.Errorf("type = %v, want 'validation'", errMap["type"])
	}
}
