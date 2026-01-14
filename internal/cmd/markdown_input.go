package cmd

import (
	"fmt"
	"io"
	"strings"
)

func readMarkdownFromFlags(source, content string, stdin io.Reader) (string, error) {
	if strings.TrimSpace(source) != "" && strings.TrimSpace(content) != "" {
		return "", fmt.Errorf("use only one of --markdown or --markdown-file")
	}

	if content != "" {
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("markdown content required (use --markdown, --markdown-file, or stdin)")
		}
		return content, nil
	}

	if strings.TrimSpace(source) != "" {
		markdown, err := readInputSource(source, stdin)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(markdown) == "" {
			return "", fmt.Errorf("markdown content required (use --markdown, --markdown-file, or stdin)")
		}
		return markdown, nil
	}

	if inputHasData(stdin) {
		markdown, err := readInputSource("-", stdin)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(markdown) == "" {
			return "", fmt.Errorf("markdown content required (use --markdown, --markdown-file, or stdin)")
		}
		return markdown, nil
	}

	return "", fmt.Errorf("markdown content required (use --markdown, --markdown-file, or stdin)")
}
