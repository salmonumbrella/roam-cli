package cmd

import (
	"context"
	"io"
	"os"
)

type ioKey struct{}

type ioState struct {
	in  io.Reader
	out io.Writer
	err io.Writer
}

func withIO(ctx context.Context, in io.Reader, out, err io.Writer) context.Context {
	return context.WithValue(ctx, ioKey{}, ioState{in: in, out: out, err: err})
}

func stdinFromContext(ctx context.Context) io.Reader {
	if ctx != nil {
		if v, ok := ctx.Value(ioKey{}).(ioState); ok && v.in != nil {
			return v.in
		}
	}
	return os.Stdin
}

func stdoutFromContext(ctx context.Context) io.Writer {
	if ctx != nil {
		if v, ok := ctx.Value(ioKey{}).(ioState); ok && v.out != nil {
			return v.out
		}
	}
	return os.Stdout
}

func stderrFromContext(ctx context.Context) io.Writer {
	if ctx != nil {
		if v, ok := ctx.Value(ioKey{}).(ioState); ok && v.err != nil {
			return v.err
		}
	}
	return os.Stderr
}
