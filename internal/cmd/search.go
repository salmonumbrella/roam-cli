package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/output"
	"github.com/salmonumbrella/roam-cli/internal/roamdb"
)

// SearchResult represents a search result
type SearchResult struct {
	UID       string `json:"uid"`
	Content   string `json:"content"`
	PageTitle string `json:"page_title,omitempty"`
	PageUID   string `json:"page_uid,omitempty"`
}

// SearchOutput represents the JSON output format
type SearchOutput struct {
	Query   string         `json:"query"`
	Count   int            `json:"count"`
	Results []SearchResult `json:"results"`
}

var (
	searchPage          int
	searchLimit         int
	searchCaseSensitive bool
	searchUIBlocks      bool
	searchUIPages       bool
	searchUIHideCode    bool
	searchUILimit       int
	searchUIPull        string
)

var searchCmd = &cobra.Command{
	Use:   "search <text>",
	Short: "Search blocks and pages",
	Long: `Search for content in your Roam graph.

Full-text search across all blocks and pages. Results include the block UID,
content, and the page title where the block appears.

Examples:
  # Search for text
  roam search "project ideas"
  
  # Search with pagination
  roam search "meeting notes" --page 2 --limit 20
  
  # Case-sensitive search
  roam search "API" --case-sensitive
  
  # Output as JSON
  roam search "todo" --output json`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

var searchTagsCmd = &cobra.Command{
	Use:   "tags <tag>",
	Short: "Search by tag",
	Long: `Search for blocks and pages with a specific tag.

Tags in Roam are denoted by #tag or [[tag]]. This command finds all blocks
that reference the given tag.

Examples:
  # Search for a tag (with or without #)
  roam search tags project
  roam search tags "#project"
  
  # Search with limit
  roam search tags meeting --limit 50`,
	Args: cobra.ExactArgs(1),
	RunE: runSearchTags,
}

var searchStatusCmd = &cobra.Command{
	Use:   "status <TODO|DONE>",
	Short: "Search by status",
	Long: `Search for blocks with a specific status.

Roam uses {{[[TODO]]}} and {{[[DONE]]}} markers for task status.
This command finds all blocks with the specified status.

Examples:
  # Find all TODO items
  roam search status TODO
  
  # Find all completed items
  roam search status DONE
  
  # With limit
  roam search status TODO --limit 100`,
	Args: cobra.ExactArgs(1),
	RunE: runSearchStatus,
}

var searchRefsCmd = &cobra.Command{
	Use:   "refs <uid>",
	Short: "Search block references",
	Long: `Find all blocks that reference a specific block.

Block references in Roam are created with ((uid)). This command finds
all blocks that contain a reference to the specified block UID.

Examples:
  # Find references to a block
  roam search refs abc123def
  
  # With pagination
  roam search refs abc123def --page 2 --limit 20`,
	Args: cobra.ExactArgs(1),
	RunE: runSearchRefs,
}

var searchUICmd = &cobra.Command{
	Use:   "ui <query>",
	Short: "UI-style search (local only)",
	Long: `Search pages and blocks using the Local API search algorithm.

This matches the Find or Create Page ranking and supports page results.`,
	Args: cobra.ExactArgs(1),
	RunE: runSearchUI,
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.AddCommand(searchTagsCmd)
	searchCmd.AddCommand(searchStatusCmd)
	searchCmd.AddCommand(searchRefsCmd)
	searchCmd.AddCommand(searchUICmd)

	// Flags for search command
	searchCmd.Flags().IntVar(&searchPage, "page", 1, "Page number for pagination")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 50, "Maximum number of results per page")
	searchCmd.Flags().BoolVar(&searchCaseSensitive, "case-sensitive", false, "Enable case-sensitive search")

	// Flags for subcommands
	searchTagsCmd.Flags().IntVar(&searchPage, "page", 1, "Page number for pagination")
	searchTagsCmd.Flags().IntVar(&searchLimit, "limit", 50, "Maximum number of results per page")

	searchStatusCmd.Flags().IntVar(&searchPage, "page", 1, "Page number for pagination")
	searchStatusCmd.Flags().IntVar(&searchLimit, "limit", 50, "Maximum number of results per page")

	searchRefsCmd.Flags().IntVar(&searchPage, "page", 1, "Page number for pagination")
	searchRefsCmd.Flags().IntVar(&searchLimit, "limit", 50, "Maximum number of results per page")

	searchUICmd.Flags().BoolVar(&searchUIBlocks, "search-blocks", true, "Include block results")
	searchUICmd.Flags().BoolVar(&searchUIPages, "search-pages", true, "Include page results")
	searchUICmd.Flags().BoolVar(&searchUIHideCode, "hide-code-blocks", false, "Exclude code blocks from results")
	searchUICmd.Flags().IntVar(&searchUILimit, "limit", 300, "Maximum number of results to return")
	searchUICmd.Flags().StringVar(&searchUIPull, "pull", "", "Custom pull pattern (EDN string)")
}

func runSearch(cmd *cobra.Command, args []string) error {
	client := GetClient()
	searchText := args[0]

	// Build the query based on case sensitivity
	// Roam API doesn't support clojure.string/lower-case, so always use case-sensitive search
	var query string
	escapedText := roamdb.EscapeString(searchText)

	// Check if using local API (from stored credentials or --local flag)
	_, isLocalClient := client.(*api.LocalClient)

	// Determine if we need case-sensitive search
	// Roam API (both cloud and local) doesn't support clojure.string/lower-case function
	useCaseSensitive := searchCaseSensitive || isLocalClient
	if isLocalClient && !searchCaseSensitive && !output.QuietFromContext(cmd.Context()) {
		fmt.Fprintln(cmd.ErrOrStderr(), "Note: Local API uses case-sensitive search")
	}

	if useCaseSensitive {
		query = fmt.Sprintf(`[:find ?uid ?string ?page-title ?page-uid
			:where
			[?b :block/uid ?uid]
			[?b :block/string ?string]
			[(clojure.string/includes? ?string "%s")]
			[?b :block/page ?page]
			[?page :node/title ?page-title]
			[?page :block/uid ?page-uid]]`, escapedText)
	} else {
		// Case-insensitive: convert both to lowercase
		query = fmt.Sprintf(`[:find ?uid ?string ?page-title ?page-uid
			:where
			[?b :block/uid ?uid]
			[?b :block/string ?string]
			[(clojure.string/lower-case ?string) ?lower-string]
			[(clojure.string/includes? ?lower-string "%s")]
			[?b :block/page ?page]
			[?page :node/title ?page-title]
			[?page :block/uid ?page-uid]]`, strings.ToLower(escapedText))
	}

	results, err := client.Query(query)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	results, totalResults, pageUsed := paginateResults(results, searchPage, searchLimit)

	// Convert to SearchResult
	searchResults := make([]SearchResult, 0, len(results))
	for _, row := range results {
		if len(row) >= 4 {
			searchResults = append(searchResults, SearchResult{
				UID:       fmt.Sprintf("%v", row[0]),
				Content:   fmt.Sprintf("%v", row[1]),
				PageTitle: fmt.Sprintf("%v", row[2]),
				PageUID:   fmt.Sprintf("%v", row[3]),
			})
		}
	}

	return outputSearchResults(searchText, searchResults, totalResults, pageUsed)
}

func runSearchTags(cmd *cobra.Command, args []string) error {
	client := GetClient()
	tag := args[0]

	// Remove # prefix if present
	tag = strings.TrimPrefix(tag, "#")
	escapedTag := roamdb.EscapeString(tag)

	// Search for both #tag and [[tag]] references
	query := fmt.Sprintf(`[:find ?uid ?string ?page-title ?page-uid
		:where
		[?b :block/uid ?uid]
		[?b :block/string ?string]
		(or
			[(clojure.string/includes? ?string "#%s")]
			[(clojure.string/includes? ?string "[[%s]]")])
		[?b :block/page ?page]
		[?page :node/title ?page-title]
		[?page :block/uid ?page-uid]]`, escapedTag, escapedTag)

	results, err := client.Query(query)
	if err != nil {
		return fmt.Errorf("tag search failed: %w", err)
	}

	results, totalResults, pageUsed := paginateResults(results, searchPage, searchLimit)

	// Convert to SearchResult
	searchResults := make([]SearchResult, 0, len(results))
	for _, row := range results {
		if len(row) >= 4 {
			searchResults = append(searchResults, SearchResult{
				UID:       fmt.Sprintf("%v", row[0]),
				Content:   fmt.Sprintf("%v", row[1]),
				PageTitle: fmt.Sprintf("%v", row[2]),
				PageUID:   fmt.Sprintf("%v", row[3]),
			})
		}
	}

	return outputSearchResults(fmt.Sprintf("tag:%s", tag), searchResults, totalResults, pageUsed)
}

func runSearchStatus(cmd *cobra.Command, args []string) error {
	client := GetClient()
	status := strings.ToUpper(args[0])

	if status != "TODO" && status != "DONE" {
		return fmt.Errorf("invalid status: %s (must be TODO or DONE)", status)
	}

	// Search for {{[[TODO]]}} or {{[[DONE]]}} markers
	statusMarker := fmt.Sprintf("{{[[%s]]}}", status)
	escapedMarker := roamdb.EscapeString(statusMarker)

	query := fmt.Sprintf(`[:find ?uid ?string ?page-title ?page-uid
		:where
		[?b :block/uid ?uid]
		[?b :block/string ?string]
		[(clojure.string/includes? ?string "%s")]
		[?b :block/page ?page]
		[?page :node/title ?page-title]
		[?page :block/uid ?page-uid]]`, escapedMarker)

	results, err := client.Query(query)
	if err != nil {
		return fmt.Errorf("status search failed: %w", err)
	}

	results, totalResults, pageUsed := paginateResults(results, searchPage, searchLimit)

	// Convert to SearchResult
	searchResults := make([]SearchResult, 0, len(results))
	for _, row := range results {
		if len(row) >= 4 {
			searchResults = append(searchResults, SearchResult{
				UID:       fmt.Sprintf("%v", row[0]),
				Content:   fmt.Sprintf("%v", row[1]),
				PageTitle: fmt.Sprintf("%v", row[2]),
				PageUID:   fmt.Sprintf("%v", row[3]),
			})
		}
	}

	return outputSearchResults(fmt.Sprintf("status:%s", status), searchResults, totalResults, pageUsed)
}

func runSearchRefs(cmd *cobra.Command, args []string) error {
	client := GetClient()
	uid := args[0]
	escapedUID := roamdb.EscapeString(uid)

	// Search for ((...)) block references
	refPattern := fmt.Sprintf("((%s))", escapedUID)

	query := fmt.Sprintf(`[:find ?uid ?string ?page-title ?page-uid
		:where
		[?b :block/uid ?uid]
		[?b :block/string ?string]
		[(clojure.string/includes? ?string "%s")]
		[?b :block/page ?page]
		[?page :node/title ?page-title]
		[?page :block/uid ?page-uid]]`, refPattern)

	results, err := client.Query(query)
	if err != nil {
		return fmt.Errorf("reference search failed: %w", err)
	}

	results, totalResults, pageUsed := paginateResults(results, searchPage, searchLimit)

	// Convert to SearchResult
	searchResults := make([]SearchResult, 0, len(results))
	for _, row := range results {
		if len(row) >= 4 {
			searchResults = append(searchResults, SearchResult{
				UID:       fmt.Sprintf("%v", row[0]),
				Content:   fmt.Sprintf("%v", row[1]),
				PageTitle: fmt.Sprintf("%v", row[2]),
				PageUID:   fmt.Sprintf("%v", row[3]),
			})
		}
	}

	return outputSearchResults(fmt.Sprintf("refs:%s", uid), searchResults, totalResults, pageUsed)
}

func runSearchUI(cmd *cobra.Command, args []string) error {
	query := args[0]

	if !searchUIBlocks && !searchUIPages {
		return fmt.Errorf("at least one of --search-blocks or --search-pages must be true")
	}

	localClient, ok := GetClient().(*api.LocalClient)
	if !ok {
		return fmt.Errorf("search ui requires the Local API (encrypted graph)")
	}

	opts := api.SearchOptions{
		SearchBlocks:   searchUIBlocks,
		SearchPages:    searchUIPages,
		HideCodeBlocks: searchUIHideCode,
		Limit:          searchUILimit,
	}
	if strings.TrimSpace(searchUIPull) != "" {
		opts.Pull = strings.TrimSpace(searchUIPull)
	}

	raw, err := localClient.Search(query, opts)
	if err != nil {
		return fmt.Errorf("search ui failed: %w", err)
	}

	if structuredOutputRequested() {
		return printRawStructured(raw)
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(raw, &results); err != nil {
		fmt.Println(string(raw))
		return nil
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	fmt.Printf("Found %d results for %q\n", len(results), query)
	for _, item := range results {
		fmt.Printf("- %s\n", summarizeSearchUIResult(item))
	}
	return nil
}

func summarizeSearchUIResult(item map[string]interface{}) string {
	uid := firstString(item, "block/uid", ":block/uid")
	title := firstString(item, "node/title", ":node/title")
	content := firstString(item, "block/string", ":block/string")

	if content != "" {
		if title != "" {
			if uid != "" {
				return fmt.Sprintf("%s (page: %s, uid: %s)", content, title, uid)
			}
			return fmt.Sprintf("%s (page: %s)", content, title)
		}
		if uid != "" {
			return fmt.Sprintf("%s (uid: %s)", content, uid)
		}
		return content
	}

	if title != "" {
		if uid != "" {
			return fmt.Sprintf("%s (uid: %s)", title, uid)
		}
		return title
	}

	if uid != "" {
		return fmt.Sprintf("uid: %s", uid)
	}

	return fmt.Sprintf("%v", item)
}

func firstString(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if raw, ok := item[key]; ok {
			if val, ok := raw.(string); ok {
				return val
			}
		}
	}
	return ""
}

func outputSearchResults(query string, results []SearchResult, totalCount int, page int) error {
	payload := SearchOutput{
		Query:   query,
		Count:   len(results),
		Results: results,
	}

	if structuredOutputRequested() {
		ctx := currentContext()
		printer := output.NewPrinter(stdoutFromContext(ctx), GetOutputFormat())
		return printer.Print(ctx, payload)
	}

	// Text output
	if len(results) == 0 {
		fmt.Printf("No results found for: %s\n", query)
		return nil
	}

	fmt.Printf("Search: %s\n", query)
	fmt.Printf("Showing %d of %d results (page %d)\n\n", len(results), totalCount, page)

	for i, r := range results {
		// Truncate long content for display
		content := r.Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		// Replace newlines with spaces for cleaner display
		content = strings.ReplaceAll(content, "\n", " ")

		fmt.Printf("%d. [%s] %s\n", i+1, r.UID, content)
		if r.PageTitle != "" {
			fmt.Printf("   Page: %s\n", r.PageTitle)
		}
		fmt.Println()
	}

	// Pagination info
	totalPages := (totalCount + searchLimit - 1) / searchLimit
	if totalPages > 1 {
		fmt.Printf("Page %d of %d (use --page N to navigate)\n", page, totalPages)
	}

	return nil
}
