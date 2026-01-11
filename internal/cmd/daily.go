package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/roamdb"
)

// formatDailyNoteTitle formats a date into Roam's daily note page title format
// e.g., "January 2nd, 2025"
func formatDailyNoteTitle(t time.Time) string {
	day := t.Day()
	suffix := ordinalSuffix(day)
	return fmt.Sprintf("%s %d%s, %d", t.Month().String(), day, suffix, t.Year())
}

// ordinalSuffix returns the ordinal suffix for a day number (st, nd, rd, th)
func ordinalSuffix(day int) string {
	if day >= 11 && day <= 13 {
		return "th"
	}
	switch day % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
}

// parseDate parses a date string in various formats and returns a time.Time
func parseDate(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"01/02/2006",
		"January 2, 2006",
		"Jan 2, 2006",
		"2 January 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s (try YYYY-MM-DD format)", dateStr)
}

var dailyCmd = &cobra.Command{
	Use:   "daily",
	Short: "Manage daily notes",
	Long: `Commands for working with Roam daily notes pages.

Daily notes in Roam use the format "January 2nd, 2025" for page titles.`,
}

var dailyGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a daily note",
	Long: `Get the contents of a daily note page.

By default, retrieves today's daily note. Use --date to specify a different date.`,
	Example: `  # Get today's daily note
  roam daily get

  # Get daily note for a specific date
  roam daily get --date 2025-01-15
  roam daily get --date "January 15, 2025"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := GetClient()
		dateStr, _ := cmd.Flags().GetString("date")

		var targetDate time.Time
		if dateStr == "" {
			targetDate = time.Now()
		} else {
			var err error
			targetDate, err = parseDate(dateStr)
			if err != nil {
				return err
			}
		}

		pageTitle := formatDailyNoteTitle(targetDate)

		pageData, err := client.GetPageByTitle(pageTitle)
		if err != nil {
			return fmt.Errorf("failed to get daily note '%s': %w", pageTitle, err)
		}

		if structuredOutputRequested() {
			if err := printRawStructured(pageData); err != nil {
				return err
			}
			return nil
		}

		page, err := roamdb.ParsePage(pageData)
		if err != nil {
			return fmt.Errorf("failed to parse page data: %w", err)
		}

		fmt.Printf("Daily Note: %s\n", pageTitle)
		fmt.Println(strings.Repeat("-", 40))
		printBlockTree(page.Children, 0)

		return nil
	},
}

// printBlockTree recursively prints blocks with indentation
func printBlockTree(blocks []roamdb.Block, indent int) {
	prefix := strings.Repeat("  ", indent)
	for _, block := range blocks {
		if block.String != "" {
			fmt.Printf("%s- %s\n", prefix, block.String)
		}
		if len(block.Children) > 0 {
			printBlockTree(block.Children, indent+1)
		}
	}
}

var dailyAddCmd = &cobra.Command{
	Use:   "add <text>",
	Short: "Add content to a daily note",
	Long: `Add a new block to a daily note page.

By default, adds to today's daily note at the end of the page.
Use --heading to nest the content under a specific heading.
Use --date to add to a different daily note.`,
	Example: `  # Add a block to today's daily note
  roam daily add "Meeting notes from standup"

  # Add under a specific heading
  roam daily add "Fix login bug" --heading "TODO"

  # Add to a specific date's note
  roam daily add "Remember this" --date 2025-01-10`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := GetClient()
		text := args[0]
		dateStr, _ := cmd.Flags().GetString("date")
		heading, _ := cmd.Flags().GetString("heading")

		var targetDate time.Time
		if dateStr == "" {
			targetDate = time.Now()
		} else {
			var err error
			targetDate, err = parseDate(dateStr)
			if err != nil {
				return err
			}
		}

		pageTitle, err := addDailyBlock(client, targetDate, text, heading)
		if err != nil {
			return err
		}

		if structuredOutputRequested() {
			result := map[string]string{
				"status":    "success",
				"pageTitle": pageTitle,
				"text":      text,
			}
			if heading != "" {
				result["heading"] = heading
			}
			return printStructured(result)
		}

		if heading != "" {
			fmt.Printf("Added to %s under '%s': %s\n", pageTitle, heading, text)
		} else {
			fmt.Printf("Added to %s: %s\n", pageTitle, text)
		}

		return nil
	},
}

// getOrCreatePageUID gets the UID of a page by title, creating it if it doesn't exist
func getOrCreatePageUID(client interface {
	Query(query string, args ...interface{}) ([][]interface{}, error)
	CreatePage(title string) error
}, title string,
) (string, error) {
	// Query for the page UID
	escapedTitle := roamdb.EscapeString(title)
	query := fmt.Sprintf(`[:find ?uid :where [?p :node/title "%s"] [?p :block/uid ?uid]]`, escapedTitle)

	results, err := client.Query(query)
	if err != nil {
		return "", err
	}

	if len(results) > 0 {
		if uid, ok := results[0][0].(string); ok {
			return uid, nil
		}
	}

	// Page doesn't exist, create it
	if err := client.CreatePage(title); err != nil {
		return "", fmt.Errorf("failed to create page: %w", err)
	}

	// Query again to get the UID
	results, err = client.Query(query)
	if err != nil {
		return "", err
	}

	if len(results) > 0 {
		if uid, ok := results[0][0].(string); ok {
			return uid, nil
		}
	}

	return "", fmt.Errorf("could not get page UID after creation")
}

// findOrCreateHeading finds or creates a heading block under a page
func findOrCreateHeading(client interface {
	Query(query string, args ...interface{}) ([][]interface{}, error)
	CreateBlock(parentUID string, content string, order interface{}) error
}, pageUID, heading string,
) (string, error) {
	// Query for an existing heading block
	escapedPageUID := roamdb.EscapeString(pageUID)
	escapedHeading := roamdb.EscapeString(heading)

	query := fmt.Sprintf(`[:find ?uid
		:where
		[?page :block/uid "%s"]
		[?page :block/children ?block]
		[?block :block/uid ?uid]
		[?block :block/string ?string]
		[(= ?string "%s")]]`, escapedPageUID, escapedHeading)

	results, err := client.Query(query)
	if err != nil {
		return "", err
	}

	if len(results) > 0 {
		if uid, ok := results[0][0].(string); ok {
			return uid, nil
		}
	}

	// Heading doesn't exist, create it
	// We need to create the block and then query for its UID
	if err := client.CreateBlock(pageUID, heading, "first"); err != nil {
		return "", fmt.Errorf("failed to create heading: %w", err)
	}

	// Query again to get the UID
	results, err = client.Query(query)
	if err != nil {
		return "", err
	}

	if len(results) > 0 {
		if uid, ok := results[0][0].(string); ok {
			return uid, nil
		}
	}

	return "", fmt.Errorf("could not get heading UID after creation")
}

var dailyContextCmd = &cobra.Command{
	Use:   "context",
	Short: "Get daily notes context",
	Long: `Get recent daily notes content for context.

This command retrieves the content of recent daily notes, which can be useful
for providing context to AI assistants or reviewing recent activity.`,
	Example: `  # Get last 3 days of daily notes (default)
  roam daily context

  # Get last 7 days of daily notes
  roam daily context --days 7`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := GetClient()
		days, _ := cmd.Flags().GetInt("days")

		if days < 1 {
			days = 3
		}

		type dailyNote struct {
			Date    string       `json:"date"`
			Title   string       `json:"title"`
			Content *roamdb.Page `json:"content"`
		}

		var notes []dailyNote

		for i := 0; i < days; i++ {
			targetDate := time.Now().AddDate(0, 0, -i)
			pageTitle := formatDailyNoteTitle(targetDate)

			pageData, err := client.GetPageByTitle(pageTitle)
			if err != nil {
				// Skip days without notes
				continue
			}

			page, err := roamdb.ParsePage(pageData)
			if err != nil {
				continue
			}

			note := dailyNote{
				Date:    targetDate.Format("2006-01-02"),
				Title:   pageTitle,
				Content: page,
			}
			notes = append(notes, note)
		}

		if structuredOutputRequested() {
			return printStructured(notes)
		}

		if len(notes) == 0 {
			fmt.Printf("No daily notes found for the last %d days\n", days)
			return nil
		}

		for _, note := range notes {
			fmt.Printf("\n=== %s (%s) ===\n", note.Title, note.Date)
			if note.Content != nil {
				printBlockTree(note.Content.Children, 0)
			}
		}

		return nil
	},
}

// rememberCmd is a top-level command for quick memory capture
var rememberCmd = &cobra.Command{
	Use:   "remember <text>",
	Short: "Quick memory capture to daily note",
	Long: `Quickly capture a thought, task, or memory to your daily note.

This is a shortcut for 'roam daily add' that adds content to today's daily note.
Use --categories to add tags/categories for better organization.`,
	Example: `  # Quick capture
  roam remember "Call dentist tomorrow"

  # With categories
  roam remember "Review PR #123" --categories "work,todo"

  # With multiple categories
  roam remember "Buy groceries" --categories "personal,errands"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := GetClient()
		text := args[0]
		categories, _ := cmd.Flags().GetString("categories")

		finalText := formatCategories(text, categories)
		pageTitle, err := addDailyBlock(client, time.Now(), finalText, "")
		if err != nil {
			return err
		}

		if structuredOutputRequested() {
			result := map[string]string{
				"status":    "success",
				"pageTitle": pageTitle,
				"text":      finalText,
			}
			return printStructured(result)
		}

		fmt.Printf("Remembered: %s\n", finalText)

		return nil
	},
}

func addDailyBlock(client api.RoamAPI, date time.Time, text, heading string) (string, error) {
	pageTitle := formatDailyNoteTitle(date)

	pageUID, err := getOrCreatePageUID(client, pageTitle)
	if err != nil {
		return "", fmt.Errorf("failed to get or create daily note '%s': %w", pageTitle, err)
	}

	parentUID := pageUID
	if heading != "" {
		headingUID, err := findOrCreateHeading(client, pageUID, heading)
		if err != nil {
			return "", fmt.Errorf("failed to find or create heading '%s': %w", heading, err)
		}
		parentUID = headingUID
	}

	if err := client.CreateBlock(parentUID, text, "last"); err != nil {
		// Check for Local API timeout - write may have succeeded
		var localErr api.LocalAPIError
		if errors.As(err, &localErr) && localErr.IsResponseTimeout() {
			// Verify if write succeeded by checking for the block
			if verifyBlockCreated(client, text) {
				return pageTitle, nil
			}
		}
		return "", fmt.Errorf("failed to add block: %w", err)
	}

	return pageTitle, nil
}

// verifyBlockCreated checks if a block with the given text was recently created
func verifyBlockCreated(client api.RoamAPI, text string) bool {
	// Wait briefly for async write to complete
	time.Sleep(500 * time.Millisecond)

	// Query for the block - escape special characters for Datalog
	escapedText := roamdb.EscapeString(text)
	query := fmt.Sprintf(`[:find ?uid :where [?b :block/string "%s"] [?b :block/uid ?uid]]`, escapedText)

	results, err := client.Query(query)
	if err != nil {
		return false
	}
	return len(results) > 0
}

func formatCategories(text, categories string) string {
	categories = strings.TrimSpace(categories)
	if categories == "" {
		return text
	}

	categoryList := strings.Split(categories, ",")
	var tags []string
	for _, cat := range categoryList {
		cat = strings.TrimSpace(cat)
		if cat != "" {
			tags = append(tags, fmt.Sprintf("#[[%s]]", cat))
		}
	}
	if len(tags) == 0 {
		return text
	}
	return fmt.Sprintf("%s %s", text, strings.Join(tags, " "))
}

func init() {
	// Daily command flags
	dailyGetCmd.Flags().String("date", "", "Date for the daily note (default: today)")
	dailyAddCmd.Flags().String("date", "", "Date for the daily note (default: today)")
	dailyAddCmd.Flags().String("heading", "", "Heading to nest content under")
	dailyContextCmd.Flags().Int("days", 3, "Number of days to include in context")

	// Remember command flags
	rememberCmd.Flags().String("categories", "", "Comma-separated categories/tags to add")

	// Add subcommands
	dailyCmd.AddCommand(dailyGetCmd)
	dailyCmd.AddCommand(dailyAddCmd)
	dailyCmd.AddCommand(dailyContextCmd)

	// Add commands to root
	rootCmd.AddCommand(dailyCmd)
	rootCmd.AddCommand(rememberCmd)
}
