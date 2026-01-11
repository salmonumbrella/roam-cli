package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// readInputSource reads content from a file path or stdin when source is "-".
func readInputSource(source string, stdin io.Reader) (string, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return "", fmt.Errorf("empty input source")
	}

	var r io.Reader
	if trimmed == "-" {
		if stdin != nil {
			r = stdin
		} else {
			r = os.Stdin
		}
	} else {
		file, err := os.Open(trimmed)
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", trimmed, err)
		}
		defer file.Close()
		r = file
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

func inputHasData(r io.Reader) bool {
	if r == nil {
		r = os.Stdin
	}
	if file, ok := r.(*os.File); ok {
		stat, err := file.Stat()
		if err != nil {
			return false
		}
		return (stat.Mode() & os.ModeCharDevice) == 0
	}
	return true
}
