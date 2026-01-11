package cmd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestWithIO(t *testing.T) {
	in := strings.NewReader("input")
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}

	ctx := withIO(context.Background(), in, out, err)

	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	// Verify the readers/writers are stored in context
	gotIn := stdinFromContext(ctx)
	gotOut := stdoutFromContext(ctx)
	gotErr := stderrFromContext(ctx)

	if gotIn != in {
		t.Errorf("expected stdin to be the provided reader")
	}
	if gotOut != out {
		t.Errorf("expected stdout to be the provided buffer")
	}
	if gotErr != err {
		t.Errorf("expected stderr to be the provided buffer")
	}
}

func TestStdinFromContext_NilContext(t *testing.T) {
	got := stdinFromContext(nil) //nolint:staticcheck // testing nil context behavior
	if got != os.Stdin {
		t.Errorf("expected os.Stdin for nil context, got %v", got)
	}
}

func TestStdinFromContext_EmptyContext(t *testing.T) {
	got := stdinFromContext(context.Background())
	if got != os.Stdin {
		t.Errorf("expected os.Stdin for empty context, got %v", got)
	}
}

func TestStdinFromContext_WithIO(t *testing.T) {
	in := strings.NewReader("test input")
	ctx := withIO(context.Background(), in, &bytes.Buffer{}, &bytes.Buffer{})

	got := stdinFromContext(ctx)
	if got != in {
		t.Errorf("expected provided reader")
	}
}

func TestStdoutFromContext_NilContext(t *testing.T) {
	got := stdoutFromContext(nil) //nolint:staticcheck // testing nil context behavior
	if got != os.Stdout {
		t.Errorf("expected os.Stdout for nil context, got %v", got)
	}
}

func TestStdoutFromContext_EmptyContext(t *testing.T) {
	got := stdoutFromContext(context.Background())
	if got != os.Stdout {
		t.Errorf("expected os.Stdout for empty context, got %v", got)
	}
}

func TestStdoutFromContext_WithIO(t *testing.T) {
	buf := &bytes.Buffer{}
	ctx := withIO(context.Background(), nil, buf, &bytes.Buffer{})

	got := stdoutFromContext(ctx)
	if got != buf {
		t.Errorf("expected provided buffer")
	}
}

func TestStderrFromContext_NilContext(t *testing.T) {
	got := stderrFromContext(nil) //nolint:staticcheck // testing nil context behavior
	if got != os.Stderr {
		t.Errorf("expected os.Stderr for nil context, got %v", got)
	}
}

func TestStderrFromContext_EmptyContext(t *testing.T) {
	got := stderrFromContext(context.Background())
	if got != os.Stderr {
		t.Errorf("expected os.Stderr for empty context, got %v", got)
	}
}

func TestStderrFromContext_WithIO(t *testing.T) {
	buf := &bytes.Buffer{}
	ctx := withIO(context.Background(), nil, &bytes.Buffer{}, buf)

	got := stderrFromContext(ctx)
	if got != buf {
		t.Errorf("expected provided buffer")
	}
}

func TestWithIO_NilAll(t *testing.T) {
	ctx := withIO(context.Background(), nil, nil, nil)

	// When all are nil in context, should fall back to os defaults
	gotIn := stdinFromContext(ctx)
	gotOut := stdoutFromContext(ctx)
	gotErr := stderrFromContext(ctx)

	if gotIn != os.Stdin {
		t.Errorf("expected os.Stdin for nil stdin in context")
	}
	if gotOut != os.Stdout {
		t.Errorf("expected os.Stdout for nil stdout in context")
	}
	if gotErr != os.Stderr {
		t.Errorf("expected os.Stderr for nil stderr in context")
	}
}
