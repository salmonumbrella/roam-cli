package cmd

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/config"
)

var appendCmd = &cobra.Command{
	Use:   "append",
	Short: "Append blocks to a page (Append API for encrypted graphs)",
	Long: `Append blocks to a page using the Roam Append API.

This command is designed for encrypted graphs and does not require
the Roam desktop app to be running.

Blocks can be provided via --content flag or piped from stdin.
Use indentation (2 spaces) to create nested blocks.
Use --json or --file to send a raw Append API payload with full control.

Examples:
  # Append a single block
  roam append --page "My Page" --content "New block"

  # Append to today's daily note
  roam append --daily-note --content "Quick thought"

  # Append nested blocks from stdin
  echo -e "Parent block\n  Child block" | roam append --page "My Page"

  # Append from file
  cat notes.txt | roam append --page "Meeting Notes"

  # Append under a specific block
  roam append --block-uid abc123 --content "Child block"

  # Raw JSON payload
  roam append --file payload.json`,
	RunE: runAppend,
}

var (
	appendPage      string
	appendDailyNote bool
	appendDate      string
	appendContent   string
)

func runAppend(cmd *cobra.Command, args []string) error {
	// Validate target
	if appendPage == "" && !appendDailyNote {
		return fmt.Errorf("either --page or --daily-note is required")
	}
	if appendPage != "" && appendDailyNote {
		return fmt.Errorf("cannot use both --page and --daily-note")
	}

	// Get content from flag or stdin
	var content string
	if appendContent != "" {
		content = appendContent
	} else {
		input := cmd.InOrStdin()
		if inputHasData(input) {
			scanner := bufio.NewScanner(input)
			var lines []string
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			content = strings.Join(lines, "\n")
		}
	}

	if content == "" {
		return fmt.Errorf("no content provided (use --content or pipe to stdin)")
	}

	// Parse content into blocks
	blocks := parseIndentedBlocks(content)

	// Get credentials
	cfg, err := loadConfigFromFlag()
	if err != nil {
		return formatConfigLoadError(err)
	}
	graph := getGraphNameForAppend(cfg)
	token := getAPITokenForAppend(cfg)
	if token == "" {
		return fmt.Errorf("API token required (set ROAM_API_TOKEN or run 'roam auth login')")
	}
	if graph == "" {
		return fmt.Errorf("Graph name required (set ROAM_GRAPH_NAME or use --graph flag)")
	}

	client := api.NewAppendClient(graph, token)

	// Execute append
	var appendErr error
	if appendDailyNote {
		dateStr := appendDate
		if dateStr == "" {
			dateStr = time.Now().Format("01-02-2006")
		}
		appendErr = client.AppendToDailyNote(dateStr, blocks)
	} else {
		appendErr = client.AppendBlocks(appendPage, blocks)
	}

	if appendErr != nil {
		return fmt.Errorf("append failed: %w", appendErr)
	}

	// Output result
	if structuredOutputRequested() {
		result := map[string]interface{}{
			"success":      true,
			"blocks_count": countAppendBlocks(blocks),
		}
		if appendPage != "" {
			result["page"] = appendPage
		}
		return printStructured(result)
	} else {
		target := appendPage
		if appendDailyNote {
			target = "daily note"
		}
		fmt.Printf("Appended %d block(s) to %s\n", countAppendBlocks(blocks), target)
	}

	return nil
}

// getGraphNameForAppend gets the graph name from flags, environment, or keyring
func getGraphNameForAppend(cfg *config.Config) string {
	if graphName != "" {
		return graphName
	}
	if g := envGet("ROAM_GRAPH_NAME"); g != "" {
		return g
	}
	if cfg != nil && strings.TrimSpace(cfg.GraphName) != "" {
		return strings.TrimSpace(cfg.GraphName)
	}
	// Try to get from keyring
	store, err := openSecretsStore()
	if err == nil {
		if tok, err := store.GetToken(graphKeyPrefix + defaultProfile); err == nil {
			return tok.RefreshToken
		}
	}
	return ""
}

// getAPITokenForAppend gets the API token from flags, environment, or keyring
func getAPITokenForAppend(cfg *config.Config) string {
	if apiToken != "" {
		return apiToken
	}
	if t := envGet("ROAM_API_TOKEN"); t != "" {
		return t
	}
	if cfg != nil && strings.TrimSpace(cfg.Token) != "" {
		return strings.TrimSpace(cfg.Token)
	}
	// Try to get from keyring
	store, err := openSecretsStore()
	if err == nil {
		if tok, err := store.GetToken(defaultProfile); err == nil {
			return tok.RefreshToken
		}
	}
	return ""
}

// parseIndentedBlocks parses indented text into nested AppendBlocks
func parseIndentedBlocks(content string) []api.AppendBlock {
	lines := strings.Split(content, "\n")
	entries := make([]markdownEntry, 0, len(lines))

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Count leading spaces (2 spaces = 1 level)
		indent := 0
		for _, ch := range line {
			if ch == ' ' {
				indent++
			} else if ch == '\t' {
				indent += 2
			} else {
				break
			}
		}
		level := indent / 2
		entries = append(entries, markdownEntry{level: level, content: strings.TrimSpace(line)})
	}

	blocks := buildMarkdownTree(entries)
	return markdownToAppendBlocks(blocks)
}

func markdownToAppendBlocks(blocks []*MarkdownBlock) []api.AppendBlock {
	result := make([]api.AppendBlock, 0, len(blocks))
	for _, block := range blocks {
		result = append(result, api.AppendBlock{
			String:   block.Content,
			Children: markdownToAppendBlocks(block.Children),
		})
	}
	return result
}

// countAppendBlocks recursively counts blocks
func countAppendBlocks(blocks []api.AppendBlock) int {
	count := len(blocks)
	for _, b := range blocks {
		count += countAppendBlocks(b.Children)
	}
	return count
}

func init() {
	rootCmd.AddCommand(appendCmd)

	appendCmd.Flags().StringVar(&appendPage, "page", "", "Target page title")
	appendCmd.Flags().BoolVar(&appendDailyNote, "daily-note", false, "Append to daily note")
	appendCmd.Flags().StringVar(&appendDate, "date", "", "Daily note date (MM-DD-YYYY, default: today)")
	appendCmd.Flags().StringVarP(&appendContent, "content", "c", "", "Content to append")
}
