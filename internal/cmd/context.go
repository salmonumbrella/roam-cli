package cmd

import "context"

type errorFormatKey struct{}

// WithErrorFormat stores the error format in the context.
func WithErrorFormat(ctx context.Context, format string) context.Context {
	return context.WithValue(ctx, errorFormatKey{}, format)
}

// ErrorFormatFromContext retrieves the error format from context.
func ErrorFormatFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(errorFormatKey{}).(string); ok {
		return v
	}
	return ""
}
