package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/output"
	"github.com/salmonumbrella/roam-cli/internal/roamdb"
	"github.com/spf13/cobra"
)

var blockCmd = &cobra.Command{
	Use:   "block",
	Short: "Manage blocks in your Roam graph",
	Long: `Commands for managing blocks in your Roam Research graph.

Blocks are the fundamental unit of content in Roam. Each block has a unique
identifier (UID) and can contain text, references, and child blocks.

Examples:
  roam block get abc123def             # Get block by UID
  roam block get abc123def --depth 2   # Get block with 2 levels of children
  roam block create --parent abc123 --content "New block"
  roam block update abc123 --content "Updated content"
  roam block move abc123 --parent def456 --order first
  roam block delete abc123 --yes`,
}

// Get block command
var blockGetCmd = &cobra.Command{
	Use:   "get <uid>",
	Short: "Get a block by its UID",
	Long: `Retrieve a block and its contents by its unique identifier (UID).

The UID is the 9-character alphanumeric identifier that Roam assigns to each block.
You can find a block's UID by right-clicking on it and selecting "Copy block ref".

Use --depth to control how many levels of child blocks to retrieve.`,
	Example: `  roam block get abc123def
  roam block get abc123def --depth 3
  roam block get abc123def --output json`,
	Args: cobra.ExactArgs(1),
	RunE: runBlockGet,
}

var blockGetDepth int

func runBlockGet(cmd *cobra.Command, args []string) error {
	uid := args[0]
	client := GetClient()

	selector := buildBlockSelector(blockGetDepth)

	data, err := client.Pull([]interface{}{":block/uid", uid}, selector)
	if err != nil {
		return fmt.Errorf("failed to get block: %w", err)
	}

	if structuredOutputRequested() {
		if err := printRawStructured(data); err != nil {
			fmt.Println(string(data))
		}
		return nil
	}

	block, err := roamdb.ParseBlock(data)
	if err != nil {
		return err
	}

	printBlockText(*block, 0)
	return nil
}

func buildBlockSelector(depth int) string {
	if depth <= 0 {
		return "[*]"
	}
	return fmt.Sprintf("[* {:block/children %s}]", buildBlockSelector(depth-1))
}

func printBlockText(block roamdb.Block, indent int) {
	prefix := ""
	for i := 0; i < indent; i++ {
		prefix += "  "
	}

	// Print block content
	if block.String != "" {
		fmt.Printf("%s- %s\n", prefix, block.String)
	}

	// Print UID
	if block.UID != "" {
		fmt.Printf("%s  UID: %s\n", prefix, block.UID)
	}

	// Print children if present
	for _, child := range block.Children {
		printBlockText(child, indent+1)
	}
}

// Create block command
var blockCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new block",
	Long: `Create a new block under a parent block or page.

The parent UID can be either a block UID or a page UID.
Order can be a number (0-indexed position) or "first"/"last".`,
	Example: `  roam block create --parent abc123 --content "New block"
  roam block create --parent abc123 --content "First child" --order first
  roam block create --parent abc123 --content "Third child" --order 2`,
	RunE: runBlockCreate,
}

var (
	blockCreateParent       string
	blockCreateUID          string
	blockCreatePageTitle    string
	blockCreateDailyNote    string
	blockCreateContent      string
	blockCreateOrder        string
	blockCreateOpen         string // "true", "false", or "" (unset)
	blockCreateHeading      int    // 0 = not a heading, 1-3 = heading level
	blockCreateTextAlign    string
	blockCreateViewType     string
	blockCreateChildrenView string
	blockCreateProps        string
)

func runBlockCreate(cmd *cobra.Command, args []string) error {
	if blockCreateContent == "" {
		return fmt.Errorf("--content flag is required")
	}

	// Validate location - exactly one must be set
	locationCount := 0
	if blockCreateParent != "" {
		locationCount++
	}
	if blockCreatePageTitle != "" {
		locationCount++
	}
	if blockCreateDailyNote != "" {
		locationCount++
	}
	if locationCount == 0 {
		return fmt.Errorf("one of --parent, --page-title, or --daily-note is required")
	}
	if locationCount > 1 {
		return fmt.Errorf("only one of --parent, --page-title, or --daily-note can be specified")
	}

	client := GetClient()

	// Parse order - can be int or "first"/"last"
	var order interface{}
	if blockCreateOrder == "first" || blockCreateOrder == "last" {
		order = blockCreateOrder
	} else {
		orderInt, err := strconv.Atoi(blockCreateOrder)
		if err != nil {
			return fmt.Errorf("invalid order value: %s (must be a number, 'first', or 'last')", blockCreateOrder)
		}
		order = orderInt
	}

	// Build block options
	props, err := parsePropsJSON(blockCreateProps)
	if err != nil {
		return err
	}
	opts := api.BlockOptions{
		Content:          blockCreateContent,
		UID:              blockCreateUID,
		TextAlign:        blockCreateTextAlign,
		BlockViewType:    blockCreateViewType,
		ChildrenViewType: blockCreateChildrenView,
		Props:            props,
	}

	// Parse open flag if set
	if blockCreateOpen != "" {
		open := blockCreateOpen == "true"
		opts.Open = &open
	}

	// Parse heading if set
	if blockCreateHeading > 0 && blockCreateHeading <= 3 {
		opts.Heading = &blockCreateHeading
	}

	// Build location and create block
	var locationDesc string

	if blockCreateParent != "" {
		err = client.CreateBlockWithOptions(blockCreateParent, opts, order)
		locationDesc = fmt.Sprintf("parent %s", blockCreateParent)
	} else {
		loc := api.Location{Order: order}
		if blockCreatePageTitle != "" {
			loc.PageTitle = blockCreatePageTitle
			locationDesc = fmt.Sprintf("page '%s'", blockCreatePageTitle)
		} else if blockCreateDailyNote != "" {
			loc.DailyNoteDate = blockCreateDailyNote
			locationDesc = fmt.Sprintf("daily note %s", blockCreateDailyNote)
		}
		err = client.CreateBlockAtLocation(loc, opts)
	}

	if err != nil {
		// Check for Local API timeout - write may have succeeded
		var localErr api.LocalAPIError
		if errors.As(err, &localErr) && localErr.IsResponseTimeout() {
			// Verify if block was created
			if verifyBlockCreatedByContent(client, blockCreateContent) {
				// Block was created despite timeout, continue
				goto blockCreated
			}
		}
		return fmt.Errorf("failed to create block: %w", err)
	}
blockCreated:

	if structuredOutputRequested() {
		result := map[string]interface{}{
			"success": true,
			"content": blockCreateContent,
			"order":   order,
		}
		return printStructured(result)
	} else {
		fmt.Printf("Block created successfully under %s\n", locationDesc)
	}

	return nil
}

// Update block command
var blockUpdateCmd = &cobra.Command{
	Use:   "update <uid>",
	Short: "Update a block's content",
	Long: `Update the content of an existing block.

This replaces the entire content of the block with the new content.`,
	Example: `  roam block update abc123 --content "Updated content"
  roam block update abc123 --content "New [[reference]]"`,
	Args: cobra.ExactArgs(1),
	RunE: runBlockUpdate,
}

var (
	blockUpdateContent      string
	blockUpdateOpen         string
	blockUpdateHeading      int
	blockUpdateTextAlign    string
	blockUpdateViewType     string
	blockUpdateChildrenView string
	blockUpdateProps        string
)

func runBlockUpdate(cmd *cobra.Command, args []string) error {
	uid := args[0]

	// At least one property must be set
	if blockUpdateContent == "" && blockUpdateOpen == "" && blockUpdateHeading == 0 &&
		blockUpdateTextAlign == "" && blockUpdateViewType == "" && blockUpdateChildrenView == "" && blockUpdateProps == "" {
		return fmt.Errorf("at least one property flag is required (--content, --open, --heading, etc.)")
	}

	client := GetClient()

	props, err := parsePropsJSON(blockUpdateProps)
	if err != nil {
		return err
	}
	opts := api.BlockOptions{
		Content:          blockUpdateContent,
		TextAlign:        blockUpdateTextAlign,
		BlockViewType:    blockUpdateViewType,
		ChildrenViewType: blockUpdateChildrenView,
		Props:            props,
	}

	if blockUpdateOpen != "" {
		open := blockUpdateOpen == "true"
		opts.Open = &open
	}

	if blockUpdateHeading > 0 && blockUpdateHeading <= 3 {
		opts.Heading = &blockUpdateHeading
	}

	if err := client.UpdateBlockWithOptions(uid, opts); err != nil {
		// Check for Local API timeout - write may have succeeded
		var localErr api.LocalAPIError
		if errors.As(err, &localErr) && localErr.IsResponseTimeout() {
			// Verify if block content was updated
			if blockUpdateContent != "" && verifyBlockUpdated(client, uid, blockUpdateContent) {
				// Block was updated despite timeout, continue
				goto blockUpdated
			}
		}
		return fmt.Errorf("failed to update block: %w", err)
	}
blockUpdated:

	if structuredOutputRequested() {
		result := map[string]interface{}{
			"success": true,
			"uid":     uid,
		}
		return printStructured(result)
	} else {
		fmt.Printf("Block %s updated successfully\n", uid)
	}

	return nil
}

// Move block command
var blockMoveCmd = &cobra.Command{
	Use:   "move <uid>",
	Short: "Move a block to a new location",
	Long: `Move a block to a new parent at a specified position.

The parent UID can be either a block UID or a page UID.
Order can be a number (0-indexed position) or "first"/"last".`,
	Example: `  roam block move abc123 --parent def456
  roam block move abc123 --parent def456 --order first
  roam block move abc123 --parent def456 --order 0`,
	Args: cobra.ExactArgs(1),
	RunE: runBlockMove,
}

var (
	blockMoveParent string
	blockMovePage   string
	blockMoveDaily  string
	blockMoveOrder  string
)

func runBlockMove(cmd *cobra.Command, args []string) error {
	uid := args[0]

	targets := 0
	if blockMoveParent != "" {
		targets++
	}
	if blockMovePage != "" {
		targets++
	}
	if blockMoveDaily != "" {
		targets++
	}
	if targets == 0 {
		return fmt.Errorf("one target is required (--parent, --page-title, or --daily-note)")
	}
	if targets > 1 {
		return fmt.Errorf("only one target is allowed (--parent, --page-title, or --daily-note)")
	}

	client := GetClient()

	// Parse order - can be int or "first"/"last"
	var order interface{}
	if blockMoveOrder == "first" || blockMoveOrder == "last" {
		order = blockMoveOrder
	} else {
		orderInt, err := strconv.Atoi(blockMoveOrder)
		if err != nil {
			return fmt.Errorf("invalid order value: %s (must be a number, 'first', or 'last')", blockMoveOrder)
		}
		order = orderInt
	}

	loc := api.Location{Order: order}
	if blockMoveParent != "" {
		loc.ParentUID = blockMoveParent
	}
	if blockMovePage != "" {
		loc.PageTitle = blockMovePage
	}
	if blockMoveDaily != "" {
		loc.DailyNoteDate = blockMoveDaily
	}

	if err := client.MoveBlockToLocation(uid, loc); err != nil {
		// Check for Local API timeout - write may have succeeded
		var localErr api.LocalAPIError
		if errors.As(err, &localErr) && localErr.IsResponseTimeout() {
			// Verify if block was moved to new parent
			if blockMoveParent != "" && verifyBlockMoved(client, uid, blockMoveParent) {
				// Block was moved despite timeout, continue
				goto blockMoved
			}
		}
		return fmt.Errorf("failed to move block: %w", err)
	}
blockMoved:

	if structuredOutputRequested() {
		result := map[string]interface{}{
			"success": true,
			"uid":     uid,
			"target":  loc.ToMap(),
			"order":   order,
		}
		return printStructured(result)
	} else {
		targetDesc := blockMoveParent
		if blockMovePage != "" {
			targetDesc = blockMovePage
		}
		if blockMoveDaily != "" {
			targetDesc = blockMoveDaily
		}
		fmt.Printf("Block %s moved to %s\n", uid, targetDesc)
	}

	return nil
}

// Delete block command
var blockDeleteCmd = &cobra.Command{
	Use:   "delete <uid>",
	Short: "Delete a block",
	Long: `Delete a block and all its children.

This operation is irreversible. Use --yes to skip the confirmation prompt.`,
	Example: `  roam block delete abc123
  roam block delete abc123 --yes`,
	Args: cobra.ExactArgs(1),
	RunE: runBlockDelete,
}

func runBlockDelete(cmd *cobra.Command, args []string) error {
	uid := args[0]
	client := GetClient()

	if !output.YesFromContext(cmd.Context()) {
		// Try to get block info first to show what will be deleted
		data, err := client.GetBlockByUID(uid)
		if err != nil {
			return fmt.Errorf("failed to get block: %w", err)
		}

		block, err := roamdb.ParseBlock(data)
		if err != nil {
			return fmt.Errorf("failed to parse block data: %w", err)
		}

		content := block.String
		if len(content) > 50 {
			content = content[:50] + "..."
		}

		errOut := cmd.ErrOrStderr()
		fmt.Fprintf(errOut, "Are you sure you want to delete this block?\n")
		fmt.Fprintf(errOut, "  UID: %s\n", uid)
		if content != "" {
			fmt.Fprintf(errOut, "  Content: %s\n", content)
		}
		fmt.Fprintf(errOut, "\nThis action cannot be undone. Use --yes to confirm.\n")
		return nil
	}

	if err := client.DeleteBlock(uid); err != nil {
		return fmt.Errorf("failed to delete block: %w", err)
	}

	if structuredOutputRequested() {
		result := map[string]interface{}{
			"success": true,
			"uid":     uid,
			"deleted": true,
		}
		return printStructured(result)
	} else {
		fmt.Printf("Block %s deleted successfully\n", uid)
	}

	return nil
}

// verifyBlockCreatedByContent checks if a block with the given content was recently created
func verifyBlockCreatedByContent(client api.RoamAPI, content string) bool {
	// Wait briefly for async write to complete
	time.Sleep(500 * time.Millisecond)

	// Query for the block by content
	escapedContent := roamdb.EscapeString(content)
	query := fmt.Sprintf(`[:find ?uid :where [?b :block/string "%s"] [?b :block/uid ?uid]]`, escapedContent)

	results, err := client.Query(query)
	if err != nil {
		return false
	}
	return len(results) > 0
}

// verifyBlockUpdated checks if a block's content matches the expected value
func verifyBlockUpdated(client api.RoamAPI, uid, expectedContent string) bool {
	// Wait briefly for async write to complete
	time.Sleep(500 * time.Millisecond)

	// Query to check the block's current content
	escapedUID := roamdb.EscapeString(uid)
	escapedContent := roamdb.EscapeString(expectedContent)

	query := fmt.Sprintf(`[:find ?b
		:where
		[?b :block/uid "%s"]
		[?b :block/string "%s"]]`, escapedUID, escapedContent)

	results, err := client.Query(query)
	if err != nil {
		return false
	}
	return len(results) > 0
}

// verifyBlockMoved checks if a block is now under the expected parent
func verifyBlockMoved(client api.RoamAPI, blockUID, parentUID string) bool {
	// Wait briefly for async write to complete
	time.Sleep(500 * time.Millisecond)

	// Query to check if block is a child of the parent
	escapedBlockUID := roamdb.EscapeString(blockUID)
	escapedParentUID := roamdb.EscapeString(parentUID)

	query := fmt.Sprintf(`[:find ?b
		:where
		[?parent :block/uid "%s"]
		[?parent :block/children ?b]
		[?b :block/uid "%s"]]`, escapedParentUID, escapedBlockUID)

	results, err := client.Query(query)
	if err != nil {
		return false
	}
	return len(results) > 0
}

func parsePropsJSON(raw string) (map[string]interface{}, error) {
	if raw == "" {
		return nil, nil
	}
	var props map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &props); err != nil {
		return nil, fmt.Errorf("invalid props JSON: %w", err)
	}
	return props, nil
}

func init() {
	rootCmd.AddCommand(blockCmd)

	// Get command
	blockCmd.AddCommand(blockGetCmd)
	blockGetCmd.Flags().IntVar(&blockGetDepth, "depth", 0, "Number of child levels to retrieve (0 = no children)")

	// Create command
	blockCmd.AddCommand(blockCreateCmd)
	blockCreateCmd.Flags().StringVar(&blockCreateParent, "parent", "", "Parent block or page UID")
	blockCreateCmd.Flags().StringVar(&blockCreateUID, "uid", "", "Custom block UID (optional)")
	blockCreateCmd.Flags().StringVar(&blockCreatePageTitle, "page-title", "", "Target page by title (creates if not exists)")
	blockCreateCmd.Flags().StringVar(&blockCreateDailyNote, "daily-note", "", "Target daily note by date (MM-DD-YYYY)")
	blockCreateCmd.Flags().StringVar(&blockCreateContent, "content", "", "Block content (required)")
	blockCreateCmd.Flags().StringVar(&blockCreateOrder, "order", "last", "Position: number, 'first', or 'last'")
	blockCreateCmd.Flags().StringVar(&blockCreateOpen, "open", "", "Expand/collapse block: true, false")
	blockCreateCmd.Flags().IntVar(&blockCreateHeading, "heading", 0, "Heading level: 1, 2, or 3")
	blockCreateCmd.Flags().StringVar(&blockCreateTextAlign, "text-align", "", "Text alignment: left, center, right, justify")
	blockCreateCmd.Flags().StringVar(&blockCreateViewType, "view-type", "", "Block view: bullet, numbered, document")
	blockCreateCmd.Flags().StringVar(&blockCreateChildrenView, "children-view", "", "Children view: bullet, numbered, document")
	blockCreateCmd.Flags().StringVar(&blockCreateProps, "props", "", "Block props as JSON object")
	blockCreateCmd.MarkFlagRequired("content")

	// Update command
	blockCmd.AddCommand(blockUpdateCmd)
	blockUpdateCmd.Flags().StringVar(&blockUpdateContent, "content", "", "New block content")
	blockUpdateCmd.Flags().StringVar(&blockUpdateOpen, "open", "", "Expand/collapse: true, false")
	blockUpdateCmd.Flags().IntVar(&blockUpdateHeading, "heading", 0, "Heading level: 1, 2, or 3 (0 to remove)")
	blockUpdateCmd.Flags().StringVar(&blockUpdateTextAlign, "text-align", "", "Text alignment")
	blockUpdateCmd.Flags().StringVar(&blockUpdateViewType, "view-type", "", "Block view type")
	blockUpdateCmd.Flags().StringVar(&blockUpdateChildrenView, "children-view", "", "Children view type")
	blockUpdateCmd.Flags().StringVar(&blockUpdateProps, "props", "", "Block props as JSON object")

	// Move command
	blockCmd.AddCommand(blockMoveCmd)
	blockMoveCmd.Flags().StringVar(&blockMoveParent, "parent", "", "New parent block or page UID")
	blockMoveCmd.Flags().StringVar(&blockMovePage, "page-title", "", "Move to page title (creates if not exists)")
	blockMoveCmd.Flags().StringVar(&blockMoveDaily, "daily-note", "", "Move to daily note by date (MM-DD-YYYY)")
	blockMoveCmd.Flags().StringVar(&blockMoveOrder, "order", "last", "Position: number, 'first', or 'last'")

	// Delete command
	blockCmd.AddCommand(blockDeleteCmd)
}
