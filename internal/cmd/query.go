package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/roam-cli/internal/api"
)

var pullPattern string

var queryCmd = &cobra.Command{
	Use:   "query <datalog>",
	Short: "Execute a raw Datalog query",
	Long: `Execute a raw Datalog query against the Roam graph.

Datalog is a declarative query language used by Roam's database (Datomic).
Queries find entities matching specified patterns and conditions.

Query Structure:
  [:find <vars>     - What to return (variables start with ?)
   :where           - Conditions that must match
   [<entity> <attribute> <value>]]

Common Attributes:
  :node/title       - Page title
  :block/string     - Block content text
  :block/uid        - Block/page unique identifier
  :block/page       - Reference to parent page
  :block/children   - Child blocks
  :block/refs       - Page/block references
  :edit/time        - Last edit timestamp (milliseconds)
  :create/time      - Creation timestamp (milliseconds)

Examples:
  # Find all page titles
  roam query '[:find ?title :where [?e :node/title ?title]]'

  # Find blocks containing specific text
  roam query '[:find ?uid ?string :where [?b :block/uid ?uid] [?b :block/string ?string] [(clojure.string/includes? ?string "TODO")]]'

  # Find pages with a specific title
  roam query '[:find ?e :where [?e :node/title "My Page"]]'

  # Find all blocks on a page
  roam query '[:find ?uid ?string :where [?p :node/title "My Page"] [?b :block/page ?p] [?b :block/uid ?uid] [?b :block/string ?string]]'

  # Find all page references in blocks
  roam query '[:find ?title :where [?b :block/refs ?ref] [?ref :node/title ?title]]'

  # Find blocks edited today (timestamp is milliseconds since epoch)
  roam query '[:find ?uid ?string :where [?b :block/uid ?uid] [?b :block/string ?string] [?b :edit/time ?t] [(> ?t 1704067200000)]]'

Output:
  Results are returned as a list of tuples matching the :find clause.
  Use --output json for machine-readable output.`,
	Args: cobra.ExactArgs(1),
	RunE: runQuery,
}

var pullCmd = &cobra.Command{
	Use:   "pull <eid>",
	Short: "Pull entity data by ID",
	Long: `Pull entity data by entity ID using a pull pattern.

The pull API retrieves attribute values for an entity. The pattern specifies
which attributes to include and how to traverse relationships.

Entity ID Formats:
  - Numeric ID: 12345 (internal Datomic entity ID)
  - Lookup ref: [:block/uid "abc123"] (find by unique attribute)
  - Lookup ref: [:node/title "Page Name"] (find by page title)

Pattern Syntax:
  [*]                     - All attributes (default)
  [:block/string]         - Specific attribute
  [:block/string :block/uid]  - Multiple attributes
  [{:block/children ...}] - Recursive pull of children
  [* {:block/children ...}] - All attributes plus recursive children

Examples:
  # Pull all attributes for entity 12345
  roam pull 12345

  # Pull block by UID lookup ref
  roam pull '[:block/uid "abc123"]'

  # Pull page by title
  roam pull '[:node/title "My Page"]'

  # Pull with specific pattern
  roam pull 12345 --pattern '[:block/string :block/uid]'

  # Pull with children recursively
  roam pull '[:node/title "My Page"]' --pattern '[* {:block/children ...}]'`,
	Args: cobra.ExactArgs(1),
	RunE: runPull,
}

var pullManyCmd = &cobra.Command{
	Use:   "pull-many <eid> [<eid>...]",
	Short: "Pull multiple entities by ID",
	Long: `Pull multiple entities by their IDs using a pull pattern.

Similar to 'pull' but retrieves multiple entities in a single request.
More efficient than multiple individual pulls.

Entity ID Formats:
  - Numeric IDs: 12345 67890
  - Lookup refs: '[:block/uid "abc"]' '[:block/uid "def"]'
  - Mixed: 12345 '[:block/uid "abc"]'

Examples:
  # Pull multiple entities by numeric ID
  roam pull-many 12345 67890 11111

  # Pull multiple blocks by UID
  roam pull-many '[:block/uid "abc"]' '[:block/uid "def"]'

  # Pull with custom pattern
  roam pull-many 12345 67890 --pattern '[:block/string :block/uid]'`,
	Args: cobra.MinimumNArgs(1),
	RunE: runPullMany,
}

func init() {
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(pullManyCmd)

	pullCmd.Flags().StringVarP(&pullPattern, "pattern", "p", "[*]", "Pull pattern (Datomic pull syntax)")
	pullManyCmd.Flags().StringVarP(&pullPattern, "pattern", "p", "[*]", "Pull pattern (Datomic pull syntax)")
}

func runQuery(cmd *cobra.Command, args []string) error {
	client := GetClient()
	if client == nil {
		return fmt.Errorf("API client not initialized")
	}

	query := args[0]

	results, err := client.Query(query)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if structuredOutputRequested() {
		return printStructured(results)
	}

	// Text output
	if len(results) == 0 {
		fmt.Println("No results found")
		return nil
	}

	fmt.Printf("Found %d result(s):\n\n", len(results))
	for i, row := range results {
		fmt.Printf("[%d] ", i+1)
		for j, col := range row {
			if j > 0 {
				fmt.Print(" | ")
			}
			fmt.Printf("%v", col)
		}
		fmt.Println()
	}

	return nil
}

func runPull(cmd *cobra.Command, args []string) error {
	client := GetClient()
	if client == nil {
		return fmt.Errorf("API client not initialized")
	}

	eid, err := parseEntityID(args[0])
	if err != nil {
		return fmt.Errorf("invalid entity ID: %w", err)
	}

	result, err := client.Pull(eid, pullPattern)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	if structuredOutputRequested() {
		if err := printRawStructured(result); err != nil {
			fmt.Println(string(result))
		}
		return nil
	}

	// For text output, still use JSON but formatted
	var parsed interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		fmt.Println(string(result))
		return nil
	}
	output, _ := json.MarshalIndent(parsed, "", "  ")
	fmt.Println(string(output))

	return nil
}

func runPullMany(cmd *cobra.Command, args []string) error {
	client := GetClient()
	if client == nil {
		return fmt.Errorf("API client not initialized")
	}

	eids := make([]interface{}, len(args))
	for i, arg := range args {
		eid, err := parseEntityID(arg)
		if err != nil {
			return fmt.Errorf("invalid entity ID at position %d: %w", i+1, err)
		}
		eids[i] = eid
	}

	var result json.RawMessage
	var err error

	// Check if using LocalClient - it doesn't support pull-many natively,
	// so we fall back to multiple individual pull calls
	if localClient, ok := client.(*api.LocalClient); ok {
		result, err = pullManyFallback(localClient, eids, pullPattern)
	} else {
		result, err = client.PullMany(eids, pullPattern)
	}

	if err != nil {
		return fmt.Errorf("pull-many failed: %w", err)
	}

	if structuredOutputRequested() {
		if err := printRawStructured(result); err != nil {
			fmt.Println(string(result))
		}
		return nil
	}

	// For text output, still use JSON but formatted
	var parsed interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		fmt.Println(string(result))
		return nil
	}
	output, _ := json.MarshalIndent(parsed, "", "  ")
	fmt.Println(string(output))

	return nil
}

// pullManyFallback implements pull-many for LocalClient by calling Pull
// individually for each entity and combining the results into an array.
func pullManyFallback(client *api.LocalClient, eids []interface{}, selector string) (json.RawMessage, error) {
	results := make([]json.RawMessage, 0, len(eids))

	for i, eid := range eids {
		result, err := client.Pull(eid, selector)
		if err != nil {
			return nil, fmt.Errorf("pull failed for entity %d (%v): %w", i+1, eid, err)
		}
		results = append(results, result)
	}

	// Combine results into a JSON array
	combined, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("failed to combine results: %w", err)
	}

	return combined, nil
}

// parseEntityID parses an entity ID from string input.
// Supports:
//   - Numeric IDs: "12345" -> int
//   - Lookup refs: "[:block/uid \"abc\"]" -> []interface{}{":block/uid", "abc"}
func parseEntityID(input string) (interface{}, error) {
	input = strings.TrimSpace(input)

	// Try parsing as numeric ID first
	if id, err := strconv.ParseInt(input, 10, 64); err == nil {
		return id, nil
	}

	// Check if it looks like a lookup ref
	if strings.HasPrefix(input, "[") && strings.HasSuffix(input, "]") {
		// Parse as lookup ref: [:keyword "value"]
		inner := strings.TrimPrefix(strings.TrimSuffix(input, "]"), "[")
		inner = strings.TrimSpace(inner)

		// Find the keyword (starts with :)
		parts := strings.SplitN(inner, " ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid lookup ref format: expected [:keyword \"value\"]")
		}

		keyword := strings.TrimSpace(parts[0])
		if !strings.HasPrefix(keyword, ":") {
			return nil, fmt.Errorf("lookup ref keyword must start with :")
		}

		// Extract the value (remove quotes if present)
		value := strings.TrimSpace(parts[1])
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			value = strings.TrimPrefix(strings.TrimSuffix(value, "\""), "\"")
		}

		// Return as a two-element array (Datomic lookup ref format)
		return []interface{}{keyword, value}, nil
	}

	return nil, fmt.Errorf("entity ID must be a number or lookup ref like [:block/uid \"abc\"]")
}
