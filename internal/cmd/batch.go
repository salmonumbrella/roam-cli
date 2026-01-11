package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/roamdb"
	"github.com/spf13/cobra"
)

// BatchAction represents a single action in a batch operation
type BatchAction struct {
	Action   string                 `json:"action"`
	Block    map[string]interface{} `json:"block,omitempty"`
	Page     map[string]interface{} `json:"page,omitempty"`
	Location map[string]interface{} `json:"location,omitempty"`
}

// BatchResult represents the result of a single batch action
type BatchResult struct {
	Index   int         `json:"index"`
	Action  string      `json:"action"`
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
	Result  interface{} `json:"result,omitempty"`
}

// BatchSummary represents the summary of batch execution
type BatchSummary struct {
	Total     int           `json:"total"`
	Succeeded int           `json:"succeeded"`
	Failed    int           `json:"failed"`
	Results   []BatchResult `json:"results"`
}

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Execute batch operations on your Roam graph",
	Long: `Execute multiple actions in batch from a JSON file or stdin.

Each action in the batch is a JSON object with the following structure:
  - action: The action type (create-block, update-block, move-block, delete-block,
            create-page, update-page, delete-page)
  - block: Block data (uid, string) - for block operations
  - page: Page data (uid, title) - for page operations
  - location: Location data (parent-uid, order) - for create/move operations

Actions are executed sequentially in the order they appear.

Examples:
  # Execute batch from file
  roam batch --file actions.json

  # Execute batch from stdin
  echo '[{"action":"create-page","page":{"title":"Test"}}]' | roam batch

  # Preview without executing
  roam batch --file actions.json --dry-run

  # Example actions.json:
  [
    {"action": "create-page", "page": {"title": "New Page"}},
    {"action": "create-block", "location": {"parent-uid": "abc123", "order": "last"}, "block": {"string": "Hello"}},
    {"action": "update-block", "block": {"uid": "def456", "string": "Updated content"}},
    {"action": "move-block", "block": {"uid": "ghi789"}, "location": {"parent-uid": "jkl012", "order": "first"}},
    {"action": "delete-block", "block": {"uid": "mno345"}}
  ]`,
	RunE: runBatch,
}

var (
	batchFile   string
	batchDryRun bool
	batchNative bool
)

func runBatch(cmd *cobra.Command, args []string) error {
	var input io.Reader

	if batchFile != "" {
		file, err := os.Open(batchFile)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()
		input = file
	} else {
		in := cmd.InOrStdin()
		if !inputHasData(in) {
			return fmt.Errorf("no input provided. Use --file flag or pipe JSON to stdin")
		}
		input = in
	}

	// Read all input
	data, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Parse actions
	var actions []BatchAction
	if err := json.Unmarshal(data, &actions); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(actions) == 0 {
		return fmt.Errorf("no actions provided")
	}

	// Dry run mode
	if batchDryRun {
		return dryRunBatch(actions)
	}

	// Native mode uses batch-actions endpoint
	if batchNative {
		return executeNativeBatch(actions)
	}

	// Execute batch
	client := GetClient()
	summary := executeBatch(client, actions)

	// Output results
	if structuredOutputRequested() {
		return printStructured(summary)
	}

	// Text output
	fmt.Printf("\nBatch execution complete:\n")
	fmt.Printf("  Total:     %d\n", summary.Total)
	fmt.Printf("  Succeeded: %d\n", summary.Succeeded)
	fmt.Printf("  Failed:    %d\n", summary.Failed)

	if summary.Failed > 0 {
		fmt.Printf("\nFailed actions:\n")
		for _, result := range summary.Results {
			if !result.Success {
				fmt.Printf("  [%d] %s: %s\n", result.Index, result.Action, result.Error)
			}
		}
	}

	return nil
}

func dryRunBatch(actions []BatchAction) error {
	fmt.Printf("Dry run - %d action(s) would be executed:\n\n", len(actions))

	for i, action := range actions {
		fmt.Printf("[%d] %s\n", i, action.Action)

		switch action.Action {
		case "create-block":
			if action.Location != nil {
				parentUID, _ := action.Location["parent-uid"].(string)
				order := action.Location["order"]
				fmt.Printf("    Location: parent=%s, order=%v\n", parentUID, order)
			}
			if action.Block != nil {
				content, _ := action.Block["string"].(string)
				fmt.Printf("    Content: %s\n", truncateString(content, 60))
			}

		case "update-block":
			if action.Block != nil {
				uid, _ := action.Block["uid"].(string)
				content, _ := action.Block["string"].(string)
				fmt.Printf("    UID: %s\n", uid)
				fmt.Printf("    Content: %s\n", truncateString(content, 60))
			}

		case "move-block":
			if action.Block != nil {
				uid, _ := action.Block["uid"].(string)
				fmt.Printf("    UID: %s\n", uid)
			}
			if action.Location != nil {
				parentUID, _ := action.Location["parent-uid"].(string)
				order := action.Location["order"]
				fmt.Printf("    New location: parent=%s, order=%v\n", parentUID, order)
			}

		case "delete-block":
			if action.Block != nil {
				uid, _ := action.Block["uid"].(string)
				fmt.Printf("    UID: %s\n", uid)
			}

		case "create-page":
			if action.Page != nil {
				title, _ := action.Page["title"].(string)
				fmt.Printf("    Title: %s\n", title)
			}

		case "update-page":
			if action.Page != nil {
				uid, _ := action.Page["uid"].(string)
				title, _ := action.Page["title"].(string)
				fmt.Printf("    UID: %s\n", uid)
				fmt.Printf("    New title: %s\n", title)
			}

		case "delete-page":
			if action.Page != nil {
				uid, _ := action.Page["uid"].(string)
				fmt.Printf("    UID: %s\n", uid)
			}

		default:
			fmt.Printf("    (unknown action type)\n")
		}
		fmt.Println()
	}

	return nil
}

func executeNativeBatch(actions []BatchAction) error {
	client := GetClient()
	batch := api.NewBatchBuilder()

	// Map old UIDs to tempid refs for chaining
	uidMap := make(map[string]string)

	for _, action := range actions {
		switch action.Action {
		case "create-page":
			opts := pageOptionsFromMap(action.Page)
			ref := batch.CreatePage(opts)
			// Store for potential future reference
			if action.Page != nil {
				if uid := uidFromAny(action.Page["uid"]); uid != "" {
					uidMap[uid] = ref
				}
			}

		case "create-block":
			loc, err := locationFromMap(action.Location, uidMap)
			if err != nil {
				return err
			}
			opts := blockOptionsFromMap(action.Block)
			ref := batch.CreateBlock(loc, opts)
			if action.Block != nil {
				if uid := uidFromAny(action.Block["uid"]); uid != "" {
					uidMap[uid] = ref
				}
			}

		case "update-block":
			uid := uidFromAny(getMapValue(action.Block, "uid"))
			if mapped, exists := uidMap[uid]; exists {
				uid = mapped
			}
			batch.UpdateBlock(uid, blockOptionsFromMap(action.Block))

		case "move-block":
			uid := uidFromAny(getMapValue(action.Block, "uid"))
			if mapped, exists := uidMap[uid]; exists {
				uid = mapped
			}
			loc, err := locationFromMap(action.Location, uidMap)
			if err != nil {
				return err
			}
			batch.MoveBlock(uid, loc)

		case "delete-block":
			uid := uidFromAny(getMapValue(action.Block, "uid"))
			if mapped, exists := uidMap[uid]; exists {
				uid = mapped
			}
			batch.DeleteBlock(uid)

		case "update-page":
			uid := uidFromAny(getMapValue(action.Page, "uid"))
			if mapped, exists := uidMap[uid]; exists {
				uid = mapped
			}
			batch.UpdatePage(uid, pageOptionsFromMap(action.Page))

		case "delete-page":
			uid := uidFromAny(getMapValue(action.Page, "uid"))
			if mapped, exists := uidMap[uid]; exists {
				uid = mapped
			}
			batch.DeletePage(uid)
		}
	}

	if err := client.ExecuteBatch(batch); err != nil {
		return fmt.Errorf("batch execution failed: %w", err)
	}

	if structuredOutputRequested() {
		return printStructured(map[string]interface{}{
			"success": true,
			"count":   len(actions),
		})
	}
	fmt.Printf("Batch executed successfully (%d actions)\n", len(actions))
	return nil
}

func executeBatch(client api.RoamAPI, actions []BatchAction) BatchSummary {
	summary := BatchSummary{
		Total:   len(actions),
		Results: make([]BatchResult, 0, len(actions)),
	}

	for i, action := range actions {
		result := BatchResult{
			Index:  i,
			Action: action.Action,
		}

		err := executeAction(client, action)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			summary.Failed++
		} else {
			result.Success = true
			summary.Succeeded++
		}

		summary.Results = append(summary.Results, result)
	}

	return summary
}

func executeAction(client api.RoamAPI, action BatchAction) error {
	switch action.Action {
	case "create-block":
		if action.Location == nil {
			return fmt.Errorf("location required for create-block")
		}
		if action.Block == nil {
			return fmt.Errorf("block required for create-block")
		}
		loc, err := locationFromMap(action.Location, nil)
		if err != nil {
			return err
		}
		opts := blockOptionsFromMap(action.Block)
		return client.CreateBlockAtLocation(loc, opts)

	case "update-block":
		if action.Block == nil {
			return fmt.Errorf("block required for update-block")
		}
		uid := uidFromAny(getMapValue(action.Block, "uid"))
		if uid == "" {
			return fmt.Errorf("uid required in block")
		}
		return client.UpdateBlockWithOptions(uid, blockOptionsFromMap(action.Block))

	case "move-block":
		if action.Block == nil {
			return fmt.Errorf("block required for move-block")
		}
		if action.Location == nil {
			return fmt.Errorf("location required for move-block")
		}
		uid := uidFromAny(getMapValue(action.Block, "uid"))
		if uid == "" {
			return fmt.Errorf("uid required in block")
		}
		loc, err := locationFromMap(action.Location, nil)
		if err != nil {
			return err
		}
		return client.MoveBlockToLocation(uid, loc)

	case "delete-block":
		if action.Block == nil {
			return fmt.Errorf("block required for delete-block")
		}
		uid := uidFromAny(getMapValue(action.Block, "uid"))
		if uid == "" {
			return fmt.Errorf("uid required in block")
		}
		return client.DeleteBlock(uid)

	case "create-page":
		if action.Page == nil {
			return fmt.Errorf("page required for create-page")
		}
		opts := pageOptionsFromMap(action.Page)
		if opts.Title == "" {
			return fmt.Errorf("title required in page")
		}
		return client.CreatePageWithOptions(opts)

	case "update-page":
		if action.Page == nil {
			return fmt.Errorf("page required for update-page")
		}
		uid := uidFromAny(getMapValue(action.Page, "uid"))
		if uid == "" {
			return fmt.Errorf("uid required in page")
		}
		return client.UpdatePageWithOptions(uid, pageOptionsFromMap(action.Page))

	case "delete-page":
		if action.Page == nil {
			return fmt.Errorf("page required for delete-page")
		}
		uid := uidFromAny(getMapValue(action.Page, "uid"))
		if uid == "" {
			return fmt.Errorf("uid required in page")
		}
		return client.DeletePage(uid)

	default:
		return fmt.Errorf("unknown action: %s", action.Action)
	}
}

func getMapValue(m map[string]interface{}, key string) interface{} {
	if m == nil {
		return nil
	}
	return m[key]
}

func uidFromAny(v interface{}) string {
	switch value := v.(type) {
	case string:
		return value
	case float64:
		if value == float64(int(value)) {
			return strconv.Itoa(int(value))
		}
		return fmt.Sprintf("%v", value)
	case json.Number:
		if i, err := value.Int64(); err == nil {
			return strconv.FormatInt(i, 10)
		}
		return value.String()
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	}
	return ""
}

func intFromAny(v interface{}) (int, bool) {
	switch value := v.(type) {
	case float64:
		if value == float64(int(value)) {
			return int(value), true
		}
	case json.Number:
		if i, err := value.Int64(); err == nil {
			return int(i), true
		}
	case int:
		return value, true
	case int64:
		return int(value), true
	}
	return 0, false
}

func normalizeOrder(v interface{}) interface{} {
	if v == nil {
		return "last"
	}
	if i, ok := intFromAny(v); ok {
		return i
	}
	return v
}

func locationFromMap(m map[string]interface{}, uidMap map[string]string) (api.Location, error) {
	if m == nil {
		return api.Location{}, fmt.Errorf("location required")
	}
	loc := api.Location{Order: normalizeOrder(m["order"])}

	if parentRaw, ok := m["parent-uid"]; ok {
		parent := uidFromAny(parentRaw)
		if parent == "" {
			return api.Location{}, fmt.Errorf("parent-uid required in location")
		}
		if uidMap != nil {
			if mapped, exists := uidMap[parent]; exists {
				parent = mapped
			}
		}
		loc.ParentUID = parent
		return loc, nil
	}

	if pageTitleRaw, ok := m["page-title"]; ok {
		switch v := pageTitleRaw.(type) {
		case string:
			loc.PageTitle = v
		case map[string]interface{}:
			if dnp, ok := v["daily-note-page"].(string); ok {
				loc.DailyNoteDate = dnp
			}
		}
		if loc.PageTitle == "" && loc.DailyNoteDate == "" {
			return api.Location{}, fmt.Errorf("page-title required in location")
		}
		return loc, nil
	}

	return api.Location{}, fmt.Errorf("location requires parent-uid or page-title")
}

func blockOptionsFromMap(m map[string]interface{}) api.BlockOptions {
	opts := api.BlockOptions{}
	if m == nil {
		return opts
	}
	if content, ok := m["string"].(string); ok {
		opts.Content = content
	}
	opts.UID = uidFromAny(m["uid"])
	if open, ok := m["open"].(bool); ok {
		opts.Open = &open
	}
	if heading, ok := intFromAny(m["heading"]); ok {
		opts.Heading = &heading
	}
	if align, ok := m["text-align"].(string); ok {
		opts.TextAlign = align
	}
	if cvt, ok := m["children-view-type"].(string); ok {
		opts.ChildrenViewType = cvt
	}
	if bvt, ok := m["block-view-type"].(string); ok {
		opts.BlockViewType = bvt
	}
	if props, ok := m["props"].(map[string]interface{}); ok {
		opts.Props = props
	}
	return opts
}

func pageOptionsFromMap(m map[string]interface{}) api.PageOptions {
	opts := api.PageOptions{}
	if m == nil {
		return opts
	}
	if title, ok := m["title"].(string); ok {
		opts.Title = title
	}
	opts.UID = uidFromAny(m["uid"])
	if cvt, ok := m["children-view-type"].(string); ok {
		opts.ChildrenViewType = cvt
	}
	return opts
}

// Import command
var importCmd = &cobra.Command{
	Use:   "import <file.md>",
	Short: "Import a markdown file to Roam",
	Long: `Import a markdown file into your Roam graph.

The markdown file is parsed and converted into Roam's block hierarchy:
- Headings become top-level blocks
- Nested bullet lists become nested blocks
- Paragraphs become individual blocks

You can specify a target page (created if it doesn't exist) or a parent block.

Examples:
  # Import to a new page
  roam import notes.md --page "Imported Notes"

  # Import under an existing block
  roam import notes.md --parent abc123def

  # Import at the beginning of a page
  roam import notes.md --page "My Page" --order first

  # Preview without importing
  roam import notes.md --page "Test" --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

var (
	importPage   string
	importParent string
	importOrder  string
	importDryRun bool
)

// MarkdownBlock represents a parsed markdown block
type MarkdownBlock struct {
	Content  string
	Children []*MarkdownBlock
	Level    int
}

func runImport(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	// Read markdown file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse markdown into block hierarchy
	blocks := parseMarkdown(string(content))

	if len(blocks) == 0 {
		return fmt.Errorf("no content found in markdown file")
	}

	// Validate flags
	if importPage == "" && importParent == "" {
		return fmt.Errorf("either --page or --parent flag is required")
	}

	if importPage != "" && importParent != "" {
		return fmt.Errorf("cannot use both --page and --parent flags")
	}

	// Dry run mode
	if importDryRun {
		return dryRunImport(blocks, importPage, importParent)
	}

	client := GetClient()

	// Parse order
	order, err := parseImportOrder(importOrder)
	if err != nil {
		return err
	}

	var (
		count       int
		parentUID   string
		pageCreated bool
	)

	if _, ok := client.(*api.Client); ok {
		count, parentUID, pageCreated, err = importWithBatch(client, blocks, order)
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}
	} else if localClient, ok := client.(*api.LocalClient); ok {
		parentUID, pageCreated, err = resolveImportParent(client, false)
		if err != nil {
			return err
		}
		count, err = importBlocksLocal(localClient, parentUID, blocks, order)
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}
	} else {
		parentUID, pageCreated, err = resolveImportParent(client, false)
		if err != nil {
			return err
		}
		count, err = importBlocks(client, parentUID, blocks, order)
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}
	}

	// Output result
	if structuredOutputRequested() {
		result := map[string]interface{}{
			"success":        true,
			"blocks_created": count,
		}
		if importPage != "" {
			result["page"] = importPage
		}
		if parentUID != "" && !pageCreated {
			result["parent"] = parentUID
		}
		return printStructured(result)
	}

	fmt.Printf("Successfully imported %d blocks\n", count)
	if importPage != "" {
		fmt.Printf("Target page: %s\n", importPage)
	}
	if parentUID != "" && !pageCreated {
		fmt.Printf("Parent UID: %s\n", parentUID)
	}

	return nil
}

func parseMarkdown(content string) []*MarkdownBlock {
	var entries []markdownEntry
	scanner := bufio.NewScanner(strings.NewReader(content))

	// Regex patterns
	bulletPattern := regexp.MustCompile(`^(\s*)[-*+]\s+(.*)`)
	headingPattern := regexp.MustCompile(`^(#{1,6})\s+(.*)`)
	numberedPattern := regexp.MustCompile(`^(\s*)\d+\.\s+(.*)`)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry markdownEntry
		var level int

		// Check for headings
		if matches := headingPattern.FindStringSubmatch(line); matches != nil {
			level = 0 // Headings are top-level
			entry = markdownEntry{level: level, content: matches[2]}
		} else if matches := bulletPattern.FindStringSubmatch(line); matches != nil {
			// Calculate level from indentation (2 spaces per level)
			indent := len(matches[1])
			level = indent / 2
			entry = markdownEntry{level: level, content: matches[2]}
		} else if matches := numberedPattern.FindStringSubmatch(line); matches != nil {
			// Handle numbered lists
			indent := len(matches[1])
			level = indent / 2
			entry = markdownEntry{level: level, content: matches[2]}
		} else {
			// Plain paragraph - treat as top-level block
			level = 0
			entry = markdownEntry{level: level, content: strings.TrimSpace(line)}
		}

		if entry.content == "" {
			continue
		}
		entries = append(entries, entry)
	}

	return buildMarkdownTree(entries)
}

func dryRunImport(blocks []*MarkdownBlock, page, parent string) error {
	total := countMarkdownBlocks(blocks)

	fmt.Printf("Dry run - would import %d block(s)\n\n", total)
	if page != "" {
		fmt.Printf("Target page: %s (will be created if not exists)\n", page)
	} else {
		fmt.Printf("Target parent UID: %s\n", parent)
	}
	fmt.Printf("\nBlock hierarchy:\n")

	printMarkdownBlocks(blocks, 0)
	return nil
}

func countMarkdownBlocks(blocks []*MarkdownBlock) int {
	count := len(blocks)
	for _, block := range blocks {
		count += countMarkdownBlocks(block.Children)
	}
	return count
}

func printMarkdownBlocks(blocks []*MarkdownBlock, indent int) {
	prefix := strings.Repeat("  ", indent)
	for _, block := range blocks {
		fmt.Printf("%s- %s\n", prefix, truncateString(block.Content, 60))
		if len(block.Children) > 0 {
			printMarkdownBlocks(block.Children, indent+1)
		}
	}
}

func importBlocks(client api.RoamAPI, parentUID string, blocks []*MarkdownBlock, startOrder interface{}) (int, error) {
	count := 0

	for i, block := range blocks {
		// Determine order for this block
		var order interface{}
		if i == 0 && startOrder != nil {
			order = startOrder
		} else {
			order = "last"
		}

		// Create the block
		if err := client.CreateBlock(parentUID, block.Content, order); err != nil {
			return count, fmt.Errorf("failed to create block: %w", err)
		}
		count++

		// If there are children, we need to find the UID of the block we just created
		// to use as the parent for children
		if len(block.Children) > 0 {
			// Query for the block we just created by searching for content under parent
			// This is a limitation - we need to find the newly created block's UID
			// For now, search for the block by content
			results, err := client.SearchBlocks(block.Content, 10)
			if err != nil {
				return count, fmt.Errorf("failed to find created block: %w", err)
			}

			var childParentUID string
			for _, row := range results {
				if len(row) >= 1 {
					if uid, ok := row[0].(string); ok {
						// Use the first matching block as parent for children
						childParentUID = uid
						break
					}
				}
			}

			if childParentUID == "" {
				// Fallback: skip children if we can't find the parent
				fmt.Fprintf(os.Stderr, "Warning: could not find UID for block, skipping children: %s\n", truncateString(block.Content, 40))
				continue
			}

			// Recursively import children
			childCount, err := importBlocks(client, childParentUID, block.Children, "last")
			if err != nil {
				return count + childCount, err
			}
			count += childCount
		}
	}

	return count, nil
}

func parseImportOrder(order string) (interface{}, error) {
	if order == "first" || order == "last" || order == "" {
		if order == "" {
			return "last", nil
		}
		return order, nil
	}

	orderInt, err := strconv.Atoi(order)
	if err != nil {
		return nil, fmt.Errorf("invalid order value: %s", order)
	}
	return orderInt, nil
}

func resolveImportParent(client api.RoamAPI, createInBatch bool) (string, bool, error) {
	if importPage == "" {
		return importParent, false, nil
	}

	raw, err := client.GetPageByTitle(importPage)
	if err != nil {
		if _, ok := err.(api.NotFoundError); ok {
			if createInBatch {
				return "", true, nil
			}
			if err := client.CreatePage(importPage); err != nil {
				return "", false, fmt.Errorf("failed to create page: %w", err)
			}
			raw, err = client.GetPageByTitle(importPage)
			if err != nil {
				return "", false, fmt.Errorf("failed to get created page: %w", err)
			}
		} else {
			return "", false, fmt.Errorf("failed to check page: %w", err)
		}
	}

	page, err := roamdb.ParsePage(raw)
	if err != nil {
		return "", false, fmt.Errorf("failed to parse page: %w", err)
	}
	return page.UID, false, nil
}

func importWithBatch(client api.RoamAPI, blocks []*MarkdownBlock, order interface{}) (int, string, bool, error) {
	parentUID, createdPage, err := resolveImportParent(client, true)
	if err != nil {
		return 0, "", false, err
	}

	batch := api.NewBatchBuilder()
	if createdPage {
		parentUID = batch.CreatePage(api.PageOptions{Title: importPage})
	}

	count := buildBatchBlocks(batch, parentUID, blocks, order)
	if err := client.ExecuteBatch(batch); err != nil {
		return 0, "", createdPage, err
	}

	return count, parentUID, createdPage, nil
}

func buildBatchBlocks(batch *api.BatchBuilder, parentUID string, blocks []*MarkdownBlock, startOrder interface{}) int {
	count := 0

	for i, block := range blocks {
		order := interface{}("last")
		if i == 0 && startOrder != nil {
			order = startOrder
		}

		ref := batch.CreateBlock(api.Location{ParentUID: parentUID, Order: order}, api.BlockOptions{Content: block.Content})
		count++

		if len(block.Children) > 0 {
			count += buildBatchBlocks(batch, ref, block.Children, "last")
		}
	}

	return count
}

func importBlocksLocal(client *api.LocalClient, parentUID string, blocks []*MarkdownBlock, startOrder interface{}) (int, error) {
	count := 0

	for i, block := range blocks {
		order := interface{}("last")
		if i == 0 && startOrder != nil {
			order = startOrder
		}

		uid, err := client.CreateBlockAndGetUID(parentUID, block.Content, order)
		if err != nil {
			return count, fmt.Errorf("failed to create block: %w", err)
		}
		count++

		if len(block.Children) > 0 {
			childCount, err := importBlocksLocal(client, uid, block.Children, "last")
			if err != nil {
				return count + childCount, err
			}
			count += childCount
		}
	}

	return count, nil
}

func init() {
	// Add batch command to root
	rootCmd.AddCommand(batchCmd)
	batchCmd.Flags().StringVarP(&batchFile, "file", "f", "", "JSON file containing batch actions")
	batchCmd.Flags().BoolVar(&batchDryRun, "dry-run", false, "Preview actions without executing")
	batchCmd.Flags().BoolVar(&batchNative, "native", false, "Use native batch-actions endpoint (atomic execution)")

	// Add import command to root
	rootCmd.AddCommand(importCmd)
	importCmd.Flags().StringVarP(&importPage, "page", "p", "", "Target page title (creates if not exists)")
	importCmd.Flags().StringVar(&importParent, "parent", "", "Parent block UID")
	importCmd.Flags().StringVar(&importOrder, "order", "last", "Position: number, 'first', or 'last'")
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "Preview import without executing")
}
