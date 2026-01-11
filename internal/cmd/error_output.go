package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/output"
)

func validateErrorFormat(format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "auto", "text", "json", "yaml":
		return nil
	default:
		return fmt.Errorf("invalid --error-format %q (expected auto|text|json|yaml)", format)
	}
}

func effectiveErrorFormat(ctx context.Context) string {
	format := strings.ToLower(strings.TrimSpace(ErrorFormatFromContext(ctx)))
	if format == "" || format == "auto" {
		switch output.FormatFromContext(ctx) {
		case output.FormatJSON, output.FormatNDJSON:
			return "json"
		case output.FormatYAML:
			return "yaml"
		default:
			return "text"
		}
	}
	return format
}

func printCommandError(ctx context.Context, err error) {
	if err == nil {
		return
	}

	switch effectiveErrorFormat(ctx) {
	case "json":
		enc := json.NewEncoder(stderrFromContext(ctx))
		enc.SetEscapeHTML(false)
		_ = enc.Encode(buildErrorEnvelope(err))
		return
	case "yaml":
		enc := yaml.NewEncoder(stderrFromContext(ctx))
		enc.SetIndent(2)
		_ = enc.Encode(buildErrorEnvelope(err))
		_ = enc.Close()
		return
	}

	_, _ = fmt.Fprintln(stderrFromContext(ctx), err)
}

func buildErrorEnvelope(err error) map[string]interface{} {
	payload := map[string]interface{}{
		"error": map[string]interface{}{
			"message": err.Error(),
		},
	}

	errMap := payload["error"].(map[string]interface{})
	errMap["category"] = "system"
	errMap["type"] = "error"

	var authErr api.AuthenticationError
	if errors.As(err, &authErr) {
		errMap["type"] = "auth"
		errMap["category"] = "user"
	}

	var validationErr api.ValidationError
	if errors.As(err, &validationErr) {
		errMap["type"] = "validation"
		errMap["category"] = "user"
	}

	var notFoundErr api.NotFoundError
	if errors.As(err, &notFoundErr) {
		errMap["type"] = "not_found"
		errMap["category"] = "user"
	}

	var rateErr api.RateLimitError
	if errors.As(err, &rateErr) {
		errMap["type"] = "rate_limit"
		errMap["category"] = "system"
	}

	var localErr api.LocalAPIError
	if errors.As(err, &localErr) {
		errMap["type"] = "local_api"
		if localErr.IsResponseTimeout() {
			errMap["subtype"] = "response_timeout"
		}
	}

	var desktopErr api.DesktopNotRunningError
	if errors.As(err, &desktopErr) {
		errMap["type"] = "desktop_not_running"
		errMap["category"] = "user"
	}

	return payload
}
