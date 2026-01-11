package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/salmonumbrella/roam-cli/internal/output"
)

func structuredOutputRequested() bool {
	return output.IsStructured(GetOutputFormat())
}

func printStructured(data interface{}) error {
	ctx := currentContext()
	printer := output.NewPrinter(stdoutFromContext(ctx), GetOutputFormat())
	return printer.Print(ctx, data)
}

func printRawStructured(raw []byte) error {
	var data interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	return printStructured(data)
}

func currentContext() context.Context {
	if rootCmd != nil && rootCmd.Context() != nil {
		return rootCmd.Context()
	}
	return context.Background()
}
