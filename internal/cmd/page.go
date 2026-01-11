package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/output"
	"github.com/salmonumbrella/roam-cli/internal/roamdb"
)

var pageCmd = &cobra.Command{
	Use:   "page",
	Short: "Manage pages in your Roam graph",
	Long: `Manage pages in your Roam Research graph.

This command provides subcommands for creating, reading, updating,
deleting, and listing pages in your graph.`,
}

// renderMarkdown converts page data to markdown format
func renderMarkdown(page *roamdb.Page) string {
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(page.Title)
	sb.WriteString("\n\n")

	renderBlocksMarkdown(&sb, page.Children, 0)
	return sb.String()
}

// renderBlocksMarkdown recursively renders blocks as markdown
func renderBlocksMarkdown(sb *strings.Builder, blocks []roamdb.Block, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, block := range blocks {
		sb.WriteString(indent)
		sb.WriteString("- ")
		sb.WriteString(block.String)
		sb.WriteString("\n")

		if len(block.Children) > 0 {
			renderBlocksMarkdown(sb, block.Children, depth+1)
		}
	}
}

// Page get command
var pageGetCmd = &cobra.Command{
	Use:   "get <title>",
	Short: "Get a page by title",
	Long: `Get a page by its title from your Roam graph.

The page content can be displayed in different formats:
- raw: Shows the raw JSON response from the API
- markdown: Renders the page as markdown with nested bullets

Examples:
  roam page get "Daily Notes"
  roam page get "Project Ideas" --render markdown
  roam page get "Meeting Notes" --output json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]
		renderFormat, _ := cmd.Flags().GetString("render")

		client := GetClient()
		raw, err := client.GetPageByTitle(title)
		if err != nil {
			if _, ok := err.(api.NotFoundError); ok {
				return fmt.Errorf("page not found: %s", title)
			}
			return fmt.Errorf("failed to get page: %w", err)
		}

		// Handle output format
		if structuredOutputRequested() {
			if err := printRawStructured(raw); err != nil {
				return err
			}
			return nil
		}

		// Parse and display based on render flag
		out := stdoutFromContext(cmd.Context())
		switch renderFormat {
		case "raw":
			var prettyJSON map[string]interface{}
			if err := json.Unmarshal(raw, &prettyJSON); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			encoder := json.NewEncoder(out)
			encoder.SetIndent("", "  ")
			return encoder.Encode(prettyJSON)
		case "markdown":
			page, err := roamdb.ParsePage(raw)
			if err != nil {
				return err
			}
			fmt.Fprint(out, renderMarkdown(page))
		default:
			page, err := roamdb.ParsePage(raw)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "Title: %s\n", page.Title)
			fmt.Fprintf(out, "UID: %s\n", page.UID)
			if len(page.Children) > 0 {
				fmt.Fprintf(out, "Blocks: %d\n", countBlocks(page.Children))
			}
		}

		return nil
	},
}

// countBlocks recursively counts all blocks
func countBlocks(blocks []roamdb.Block) int {
	count := len(blocks)
	for _, block := range blocks {
		count += countBlocks(block.Children)
	}
	return count
}

// Page create command
var pageCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new page",
	Long: `Create a new page in your Roam graph.

If the page already exists, this command will fail.
Optionally, you can add initial content to the page.

Examples:
  roam page create "New Project"
  roam page create "Meeting Notes" --content "Attendees:"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]
		content, _ := cmd.Flags().GetString("content")
		childrenView, _ := cmd.Flags().GetString("children-view")
		uid, _ := cmd.Flags().GetString("uid")

		client := GetClient()

		// Check if page already exists
		_, err := client.GetPageByTitle(title)
		if err == nil {
			return fmt.Errorf("page already exists: %s", title)
		}
		if _, ok := err.(api.NotFoundError); !ok {
			return fmt.Errorf("failed to check existing page: %w", err)
		}

		// Create the page with options
		opts := api.PageOptions{
			Title:            title,
			UID:              uid,
			ChildrenViewType: childrenView,
		}
		if err := client.CreatePageWithOptions(opts); err != nil {
			// Check for Local API timeout - write may have succeeded
			var localErr api.LocalAPIError
			if errors.As(err, &localErr) && localErr.IsResponseTimeout() {
				// Verify if page was created
				if verifyPageCreated(client, title) {
					// Page was created despite timeout, continue
					goto pageCreated
				}
			}
			return fmt.Errorf("failed to create page: %w", err)
		}
	pageCreated:

		// Add initial content if provided
		if content != "" {
			// Get the newly created page to find its UID
			raw, err := client.GetPageByTitle(title)
			if err != nil {
				return fmt.Errorf("page created but failed to add content: %w", err)
			}

			page, err := roamdb.ParsePage(raw)
			if err != nil {
				return fmt.Errorf("page created but failed to parse: %w", err)
			}

			if err := client.CreateBlock(page.UID, content, "last"); err != nil {
				return fmt.Errorf("page created but failed to add content: %w", err)
			}
		}

		// Output result
		if structuredOutputRequested() {
			result := map[string]string{
				"status": "created",
				"title":  title,
			}
			return printStructured(result)
		}

		fmt.Printf("Created page: %s\n", title)
		return nil
	},
}

// Page update command
var pageUpdateCmd = &cobra.Command{
	Use:   "update <uid>",
	Short: "Update a page title",
	Long: `Update the title of an existing page by its UID.

You can find the UID of a page by using the 'page get' or 'page list' commands.

Examples:
  roam page update "abc123" --title "New Title"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uid := args[0]
		title, _ := cmd.Flags().GetString("title")
		childrenView, _ := cmd.Flags().GetString("children-view")

		if title == "" && childrenView == "" {
			return fmt.Errorf("at least one of --title or --children-view is required")
		}

		client := GetClient()
		opts := api.PageOptions{
			Title:            title,
			ChildrenViewType: childrenView,
		}
		if err := client.UpdatePageWithOptions(uid, opts); err != nil {
			return fmt.Errorf("failed to update page: %w", err)
		}

		if structuredOutputRequested() {
			result := map[string]string{
				"status": "updated",
				"uid":    uid,
			}
			if title != "" {
				result["title"] = title
			}
			if childrenView != "" {
				result["children_view"] = childrenView
			}
			return printStructured(result)
		}

		fmt.Printf("Updated page %s\n", uid)
		return nil
	},
}

// Page delete command
var pageDeleteCmd = &cobra.Command{
	Use:   "delete <uid>",
	Short: "Delete a page",
	Long: `Delete a page from your Roam graph by its UID.

This action is destructive and cannot be undone. Use the --yes flag
to skip the confirmation prompt.

Examples:
  roam page delete "abc123"
  roam page delete "abc123" --yes`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uid := args[0]

		if !output.YesFromContext(cmd.Context()) {
			errOut := cmd.ErrOrStderr()
			fmt.Fprintf(errOut, "Are you sure you want to delete page %s? This cannot be undone.\n", uid)
			fmt.Fprint(errOut, "Type 'yes' to confirm: ")
			reader := bufio.NewReader(stdinFromContext(cmd.Context()))
			confirm, _ := reader.ReadString('\n')
			if strings.TrimSpace(confirm) != "yes" {
				fmt.Fprintln(errOut, "Aborted.")
				return nil
			}
		}

		client := GetClient()
		if err := client.DeletePage(uid); err != nil {
			return fmt.Errorf("failed to delete page: %w", err)
		}

		if structuredOutputRequested() {
			result := map[string]string{
				"status": "deleted",
				"uid":    uid,
			}
			return printStructured(result)
		}

		fmt.Printf("Deleted page: %s\n", uid)
		return nil
	},
}

// Page list command
var pageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pages in your graph",
	Long: `List pages in your Roam graph.

By default, lists all pages. Use --modified-today to filter to pages
modified today. Use --limit to restrict the number of results.

Examples:
  roam page list
  roam page list --modified-today
  roam page list --limit 10 --sort title
  roam page list --output json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		modifiedToday, _ := cmd.Flags().GetBool("modified-today")
		limit, _ := cmd.Flags().GetInt("limit")
		sortBy, _ := cmd.Flags().GetString("sort")

		client := GetClient()
		results, err := client.ListPages(modifiedToday, 0) // Get all, sort/limit locally
		if err != nil {
			return fmt.Errorf("failed to list pages: %w", err)
		}

		// Build page list
		type pageInfo struct {
			Title    string `json:"title"`
			UID      string `json:"uid"`
			EditTime int64  `json:"edit_time,omitempty"`
		}

		var pages []pageInfo
		for _, row := range results {
			p := pageInfo{}
			if len(row) > 0 {
				if title, ok := row[0].(string); ok {
					p.Title = title
				}
			}
			if len(row) > 1 {
				if uid, ok := row[1].(string); ok {
					p.UID = uid
				}
			}
			if len(row) > 2 {
				if editTime, ok := row[2].(float64); ok {
					p.EditTime = int64(editTime)
				}
			}
			pages = append(pages, p)
		}

		// Sort results
		switch sortBy {
		case "title":
			sort.Slice(pages, func(i, j int) bool {
				return strings.ToLower(pages[i].Title) < strings.ToLower(pages[j].Title)
			})
		case "modified":
			sort.Slice(pages, func(i, j int) bool {
				return pages[i].EditTime > pages[j].EditTime // Most recent first
			})
		case "uid":
			sort.Slice(pages, func(i, j int) bool {
				return pages[i].UID < pages[j].UID
			})
		}

		// Apply limit
		if limit > 0 && len(pages) > limit {
			pages = pages[:limit]
		}

		// Output results
		if structuredOutputRequested() {
			return printStructured(pages)
		}

		// Table output
		if len(pages) == 0 {
			fmt.Println("No pages found.")
			return nil
		}

		ctx := currentContext()
		printer := output.NewPrinter(stdoutFromContext(ctx), output.FormatTable)
		var headers []string
		var rows [][]string

		if modifiedToday {
			headers = []string{"TITLE", "UID", "MODIFIED"}
			for _, p := range pages {
				rows = append(rows, []string{
					truncateString(p.Title, 50),
					p.UID,
					formatTimestamp(p.EditTime),
				})
			}
		} else {
			headers = []string{"TITLE", "UID"}
			for _, p := range pages {
				rows = append(rows, []string{
					truncateString(p.Title, 50),
					p.UID,
				})
			}
		}

		return printer.Print(ctx, output.Table{Headers: headers, Rows: rows})
	},
}

// Page rename command
var pageRenameCmd = &cobra.Command{
	Use:   "rename <old-title> <new-title>",
	Short: "Rename a page",
	Long: `Rename a page from one title to another.

This command finds the page by its current title and updates it
to the new title.

Examples:
  roam page rename "Old Title" "New Title"
  roam page rename "Draft Notes" "Final Notes"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldTitle := args[0]
		newTitle := args[1]

		client := GetClient()

		// Get the page by old title
		raw, err := client.GetPageByTitle(oldTitle)
		if err != nil {
			if _, ok := err.(api.NotFoundError); ok {
				return fmt.Errorf("page not found: %s", oldTitle)
			}
			return fmt.Errorf("failed to find page: %w", err)
		}

		page, err := roamdb.ParsePage(raw)
		if err != nil {
			return fmt.Errorf("failed to parse page: %w", err)
		}

		// Check if new title already exists
		_, err = client.GetPageByTitle(newTitle)
		if err == nil {
			return fmt.Errorf("a page with title '%s' already exists", newTitle)
		}
		if _, ok := err.(api.NotFoundError); !ok {
			return fmt.Errorf("failed to check new title: %w", err)
		}

		// Update the page
		if err := client.UpdatePage(page.UID, newTitle); err != nil {
			return fmt.Errorf("failed to rename page: %w", err)
		}

		if structuredOutputRequested() {
			result := map[string]string{
				"status":    "renamed",
				"uid":       page.UID,
				"old_title": oldTitle,
				"new_title": newTitle,
			}
			return printStructured(result)
		}

		fmt.Printf("Renamed page from '%s' to '%s'\n", oldTitle, newTitle)
		return nil
	},
}

// Helper functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatTimestamp(ts int64) string {
	if ts == 0 {
		return "-"
	}
	// Convert milliseconds to readable format
	// For simplicity, just show relative time or date
	return fmt.Sprintf("%d", ts)
}

// verifyPageCreated checks if a page with the given title was recently created
func verifyPageCreated(client api.RoamAPI, title string) bool {
	// Wait briefly for async write to complete
	time.Sleep(500 * time.Millisecond)

	// Query for the page by title
	escapedTitle := roamdb.EscapeString(title)
	query := fmt.Sprintf(`[:find ?uid :where [?p :node/title "%s"] [?p :block/uid ?uid]]`, escapedTitle)

	results, err := client.Query(query)
	if err != nil {
		return false
	}
	return len(results) > 0
}

func init() {
	// Add page command to root
	rootCmd.AddCommand(pageCmd)

	// Add subcommands to page
	pageCmd.AddCommand(pageGetCmd)
	pageCmd.AddCommand(pageCreateCmd)
	pageCmd.AddCommand(pageUpdateCmd)
	pageCmd.AddCommand(pageDeleteCmd)
	pageCmd.AddCommand(pageListCmd)
	pageCmd.AddCommand(pageRenameCmd)

	// Flags for get command
	pageGetCmd.Flags().StringP("render", "f", "text", "Render format: text, raw, markdown")

	// Flags for create command
	pageCreateCmd.Flags().StringP("content", "c", "", "Initial content for the page")
	pageCreateCmd.Flags().String("uid", "", "Custom page UID (optional)")
	pageCreateCmd.Flags().String("children-view", "", "Children view: bullet, numbered, document")

	// Flags for update command
	pageUpdateCmd.Flags().StringP("title", "t", "", "New title for the page")
	pageUpdateCmd.Flags().String("children-view", "", "Children view: bullet, numbered, document")

	// Flags for delete command

	// Flags for list command
	pageListCmd.Flags().Bool("modified-today", false, "Only show pages modified today")
	pageListCmd.Flags().IntP("limit", "l", 0, "Maximum number of pages to return")
	pageListCmd.Flags().StringP("sort", "s", "", "Sort by: title, modified, uid")
}
