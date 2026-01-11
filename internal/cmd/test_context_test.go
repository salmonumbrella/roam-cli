package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/output"
	"github.com/spf13/cobra"
)

func withTestContext(t *testing.T, format output.Format, yes bool) (*bytes.Buffer, *bytes.Buffer, func()) {
	t.Helper()
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	ctx := withIO(context.Background(), in, out, errBuf)
	ctx = output.WithFormat(ctx, format)
	ctx = output.WithYes(ctx, yes)
	ctx = output.WithQuiet(ctx, true)
	rootCmd.SetContext(ctx)

	prevType := outputType
	prevFmt := outputFmt
	outputType = format
	outputFmt = string(format)

	return out, errBuf, func() {
		outputType = prevType
		outputFmt = prevFmt
		rootCmd.SetContext(context.Background())
	}
}

func withTestClient(t *testing.T, apiClient api.RoamAPI) func() {
	t.Helper()
	prev := client
	client = apiClient
	return func() {
		client = prev
	}
}

func setCmdContext(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	cmd.SetContext(rootCmd.Context())
}
