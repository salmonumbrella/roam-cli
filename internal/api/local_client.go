package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/salmonumbrella/roam-cli/internal/roamdb"
)

const (
	// LocalAPITimeout is the default timeout for Local API requests
	LocalAPITimeout = 30 * time.Second
	// PortFilePath is the file containing the Local API port
	PortFilePath = ".roam-api-port"
)

// LocalAPIError indicates an error from the Local API
type LocalAPIError struct {
	Message string
}

func (e LocalAPIError) Error() string { return e.Message }

// IsResponseTimeout returns true if this is a "Response timeout" error.
// Note: On encrypted graphs, writes may succeed despite this error.
func (e LocalAPIError) IsResponseTimeout() bool {
	return e.Message == "Response timeout"
}

// DesktopNotRunningError indicates the Roam desktop app is not running
type DesktopNotRunningError struct {
	Message string
}

func (e DesktopNotRunningError) Error() string { return e.Message }

// LocalClient implements RoamAPI for encrypted graphs via Local API
type LocalClient struct {
	graphName  string
	httpClient *http.Client
}

// LocalClientOption is a function that configures a LocalClient
type LocalClientOption func(*LocalClient)

// WithLocalTimeout sets a custom timeout for the HTTP client
func WithLocalTimeout(timeout time.Duration) LocalClientOption {
	return func(c *LocalClient) {
		c.httpClient.Timeout = timeout
	}
}

// NewLocalClient creates a client for encrypted graphs using the Local API.
// The Local API requires the Roam desktop app to be running.
func NewLocalClient(graphName string, opts ...LocalClientOption) (*LocalClient, error) {
	c := &LocalClient{
		graphName: graphName,
		httpClient: &http.Client{
			Timeout: LocalAPITimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// GraphName returns the graph name
func (c *LocalClient) GraphName() string {
	return c.graphName
}

// discoverPort reads the port from ~/.roam-api-port
func discoverPort() (int, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return 0, fmt.Errorf("failed to get home directory: %w", err)
	}

	portFile := filepath.Join(homeDir, PortFilePath)
	data, err := os.ReadFile(portFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, DesktopNotRunningError{
				Message: fmt.Sprintf("Roam desktop app not running: port file %s not found. Start Roam and enable 'Encrypted local API' in settings.", portFile),
			}
		}
		return 0, fmt.Errorf("failed to read port file %s: %w", portFile, err)
	}

	portStr := strings.TrimSpace(string(data))
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port in %s: %q", portFile, portStr)
	}

	return port, nil
}

// localRequest represents a request to the Local API
type localRequest struct {
	Action string        `json:"action"`
	Args   []interface{} `json:"args"`
}

// localResponse represents a response from the Local API
type localResponse struct {
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// call makes a request to the Local API using background context
func (c *LocalClient) call(action string, args ...interface{}) (json.RawMessage, error) {
	return c.callCtx(context.Background(), action, args...)
}

// Call invokes a Local API action with arbitrary args.
// This is a public wrapper for advanced/unsupported actions.
func (c *LocalClient) Call(action string, args ...interface{}) (json.RawMessage, error) {
	return c.call(action, args...)
}

// callCtx makes a request to the Local API with context support
func (c *LocalClient) callCtx(ctx context.Context, action string, args ...interface{}) (json.RawMessage, error) {
	port, err := discoverPort()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("http://localhost:%d/api/%s", port, c.graphName)

	reqBody := localRequest{
		Action: action,
		Args:   args,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to Local API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result localResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		// Handle HTTP-level errors when response isn't valid JSON
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("local API error (status %d): %s", resp.StatusCode, string(respBody))
		}
		return nil, fmt.Errorf("failed to parse Local API response: %w", err)
	}

	if !result.Success {
		return nil, LocalAPIError{Message: result.Error}
	}

	// Handle HTTP-level errors even if JSON parsed
	if resp.StatusCode != http.StatusOK {
		if result.Error != "" {
			return nil, LocalAPIError{Message: result.Error}
		}
		return nil, fmt.Errorf("local API error (status %d)", resp.StatusCode)
	}

	return result.Result, nil
}

// Query executes a Datalog query against the graph
func (c *LocalClient) Query(query string, args ...interface{}) ([][]interface{}, error) {
	// Build args for the Local API call
	// Local API expects: data.q with query string as first arg
	callArgs := make([]interface{}, 0, 1+len(args))
	callArgs = append(callArgs, query)
	callArgs = append(callArgs, args...)

	rawResult, err := c.call("data.q", callArgs...)
	if err != nil {
		return nil, err
	}

	var result [][]interface{}
	if err := json.Unmarshal(rawResult, &result); err != nil {
		return nil, fmt.Errorf("failed to parse query result: %w", err)
	}

	return result, nil
}

// Pull retrieves an entity by ID with the given selector pattern
func (c *LocalClient) Pull(eid interface{}, selector string) (json.RawMessage, error) {
	// Local API expects: data.pull with selector and eid as args
	return c.call("data.pull", selector, eid)
}

// PullMany retrieves multiple entities by IDs
func (c *LocalClient) PullMany(eids []interface{}, selector string) (json.RawMessage, error) {
	// Local API expects: data.pull-many with selector and eids as args
	return c.call("data.pull-many", selector, eids)
}

// CreateBlock creates a new block under the specified parent
func (c *LocalClient) CreateBlock(parentUID, content string, order interface{}) error {
	// Local API expects: data.block.create with a map containing location and block
	args := map[string]interface{}{
		"location": map[string]interface{}{
			"parent-uid": parentUID,
			"order":      order,
		},
		"block": map[string]interface{}{
			"string": content,
		},
	}
	_, err := c.call("data.block.create", args)
	return err
}

// CreateBlockAndGetUID creates a block and returns its UID if available.
func (c *LocalClient) CreateBlockAndGetUID(parentUID, content string, order interface{}) (string, error) {
	args := map[string]interface{}{
		"location": map[string]interface{}{
			"parent-uid": parentUID,
			"order":      order,
		},
		"block": map[string]interface{}{
			"string": content,
		},
	}

	result, err := c.call("data.block.create", args)
	if err != nil {
		return "", err
	}

	uid, ok := parseLocalCreateUID(result)
	if !ok {
		return "", fmt.Errorf("local API did not return created block UID")
	}
	return uid, nil
}

// CreateBlockWithOptions creates a new block with extended properties
func (c *LocalClient) CreateBlockWithOptions(parentUID string, opts BlockOptions, order interface{}) error {
	blockMap := map[string]interface{}{
		"string": opts.Content,
	}
	opts.ApplyToMap(blockMap)

	args := map[string]interface{}{
		"location": map[string]interface{}{
			"parent-uid": parentUID,
			"order":      order,
		},
		"block": blockMap,
	}
	_, err := c.call("data.block.create", args)
	return err
}

// CreateBlockWithOptionsAndGetUID creates a block with options and returns its UID if available.
func (c *LocalClient) CreateBlockWithOptionsAndGetUID(parentUID string, opts BlockOptions, order interface{}) (string, error) {
	blockMap := map[string]interface{}{
		"string": opts.Content,
	}
	opts.ApplyToMap(blockMap)

	args := map[string]interface{}{
		"location": map[string]interface{}{
			"parent-uid": parentUID,
			"order":      order,
		},
		"block": blockMap,
	}

	result, err := c.call("data.block.create", args)
	if err != nil {
		return "", err
	}

	uid, ok := parseLocalCreateUID(result)
	if !ok {
		return "", fmt.Errorf("local API did not return created block UID")
	}
	return uid, nil
}

// resolveLocationToParentUID resolves a Location to a parent-uid.
// The local API doesn't support page-title or daily-note-page in locations,
// so we need to resolve these to actual page UIDs first.
func (c *LocalClient) resolveLocationToParentUID(loc Location) (string, error) {
	if loc.ParentUID != "" {
		return loc.ParentUID, nil
	}

	var pageTitle string
	if loc.PageTitle != "" {
		pageTitle = loc.PageTitle
	} else if loc.DailyNoteDate != "" {
		// Convert MM-DD-YYYY to Roam's daily note format (e.g., "January 2nd, 2025")
		t, err := time.Parse("01-02-2006", loc.DailyNoteDate)
		if err != nil {
			return "", fmt.Errorf("invalid daily note date format (expected MM-DD-YYYY): %w", err)
		}
		pageTitle = formatRoamDailyNoteTitle(t)
	} else {
		return "", fmt.Errorf("location must specify parent-uid, page-title, or daily-note-date")
	}

	// Look up page UID by title, create if not exists
	return c.getOrCreatePageUID(pageTitle)
}

// formatRoamDailyNoteTitle formats a date into Roam's daily note page title format
// e.g., "January 2nd, 2025"
func formatRoamDailyNoteTitle(t time.Time) string {
	day := t.Day()
	suffix := "th"
	if day < 11 || day > 13 {
		switch day % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%s %d%s, %d", t.Month().String(), day, suffix, t.Year())
}

// getOrCreatePageUID gets the UID of a page by title, creating it if it doesn't exist
func (c *LocalClient) getOrCreatePageUID(title string) (string, error) {
	escapedTitle := roamdb.EscapeString(title)
	query := fmt.Sprintf(`[:find ?uid :where [?p :node/title "%s"] [?p :block/uid ?uid]]`, escapedTitle)

	results, err := c.Query(query)
	if err != nil {
		return "", err
	}

	if len(results) > 0 {
		if uid, ok := results[0][0].(string); ok {
			return uid, nil
		}
	}

	// Page doesn't exist, create it
	if err := c.CreatePage(title); err != nil {
		return "", fmt.Errorf("failed to create page: %w", err)
	}

	// Query again to get the UID
	results, err = c.Query(query)
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

// CreateBlockAtLocation creates a block at a flexible location
func (c *LocalClient) CreateBlockAtLocation(loc Location, opts BlockOptions) error {
	// Local API doesn't support page-title/daily-note in locations,
	// so resolve to parent-uid first
	parentUID, err := c.resolveLocationToParentUID(loc)
	if err != nil {
		return err
	}

	blockMap := map[string]interface{}{
		"string": opts.Content,
	}
	opts.ApplyToMap(blockMap)

	args := map[string]interface{}{
		"location": map[string]interface{}{
			"parent-uid": parentUID,
			"order":      loc.Order,
		},
		"block": blockMap,
	}
	_, err = c.call("data.block.create", args)
	return err
}

// CreateBlockAtLocationAndGetUID creates a block at a location and returns its UID if available.
func (c *LocalClient) CreateBlockAtLocationAndGetUID(loc Location, opts BlockOptions) (string, error) {
	// Local API doesn't support page-title/daily-note in locations,
	// so resolve to parent-uid first
	parentUID, err := c.resolveLocationToParentUID(loc)
	if err != nil {
		return "", err
	}

	blockMap := map[string]interface{}{
		"string": opts.Content,
	}
	opts.ApplyToMap(blockMap)

	args := map[string]interface{}{
		"location": map[string]interface{}{
			"parent-uid": parentUID,
			"order":      loc.Order,
		},
		"block": blockMap,
	}

	result, err := c.call("data.block.create", args)
	if err != nil {
		return "", err
	}

	uid, ok := parseLocalCreateUID(result)
	if !ok {
		return "", fmt.Errorf("local API did not return created block UID")
	}
	return uid, nil
}

// UpdateBlock updates the content of an existing block
func (c *LocalClient) UpdateBlock(uid, content string) error {
	// Local API expects: data.block.update with a map containing block info
	args := map[string]interface{}{
		"block": map[string]interface{}{
			"uid":    uid,
			"string": content,
		},
	}
	_, err := c.call("data.block.update", args)
	return err
}

// UpdateBlockWithOptions updates a block with extended properties
func (c *LocalClient) UpdateBlockWithOptions(uid string, opts BlockOptions) error {
	blockMap := map[string]interface{}{
		"uid": uid,
	}
	if opts.Content != "" {
		blockMap["string"] = opts.Content
	}
	opts.ApplyToMap(blockMap)

	args := map[string]interface{}{
		"block": blockMap,
	}
	_, err := c.call("data.block.update", args)
	return err
}

// MoveBlock moves a block to a new parent at the specified order
func (c *LocalClient) MoveBlock(uid, parentUID string, order interface{}) error {
	return c.MoveBlockToLocation(uid, Location{
		ParentUID: parentUID,
		Order:     order,
	})
}

// MoveBlockToLocation moves a block to a flexible location
func (c *LocalClient) MoveBlockToLocation(uid string, loc Location) error {
	// Local API expects: data.block.move with location and block uid
	args := map[string]interface{}{
		"location": loc.ToMap(),
		"block": map[string]interface{}{
			"uid": uid,
		},
	}
	_, err := c.call("data.block.move", args)
	return err
}

// DeleteBlock removes a block from the graph
func (c *LocalClient) DeleteBlock(uid string) error {
	// Local API expects: data.block.delete with block uid
	args := map[string]interface{}{
		"block": map[string]interface{}{
			"uid": uid,
		},
	}
	_, err := c.call("data.block.delete", args)
	return err
}

// CreatePage creates a new page with the given title
func (c *LocalClient) CreatePage(title string) error {
	// Local API expects: data.page.create with page info
	args := map[string]interface{}{
		"page": map[string]interface{}{
			"title": title,
		},
	}
	_, err := c.call("data.page.create", args)
	return err
}

// CreatePageWithOptions creates a new page with extended properties
func (c *LocalClient) CreatePageWithOptions(opts PageOptions) error {
	pageMap := map[string]interface{}{
		"title": opts.Title,
	}
	if opts.UID != "" {
		pageMap["uid"] = opts.UID
	}
	if opts.ChildrenViewType != "" {
		pageMap["children-view-type"] = opts.ChildrenViewType
	}
	args := map[string]interface{}{
		"page": pageMap,
	}
	_, err := c.call("data.page.create", args)
	return err
}

// UpdatePage updates the title of an existing page
func (c *LocalClient) UpdatePage(uid, title string) error {
	return c.UpdatePageWithOptions(uid, PageOptions{Title: title})
}

// UpdatePageWithOptions updates a page with extended properties
func (c *LocalClient) UpdatePageWithOptions(uid string, opts PageOptions) error {
	// Local API expects: data.page.update with page uid and updated fields
	pageMap := map[string]interface{}{
		"uid": uid,
	}
	if opts.Title != "" {
		pageMap["title"] = opts.Title
	}
	if opts.ChildrenViewType != "" {
		pageMap["children-view-type"] = opts.ChildrenViewType
	}
	args := map[string]interface{}{
		"page": pageMap,
	}
	_, err := c.call("data.page.update", args)
	return err
}

// DeletePage removes a page from the graph
func (c *LocalClient) DeletePage(uid string) error {
	// Local API expects: data.page.delete with page uid
	args := map[string]interface{}{
		"page": map[string]interface{}{
			"uid": uid,
		},
	}
	_, err := c.call("data.page.delete", args)
	return err
}

// AddPageShortcut adds a page to the left sidebar shortcuts.
// If index is nil, appends to the end; otherwise inserts at index.
func (c *LocalClient) AddPageShortcut(uid string, index *int) error {
	if index == nil {
		_, err := c.call("data.page.addShortcut", uid)
		return err
	}
	_, err := c.call("data.page.addShortcut", uid, *index)
	return err
}

// RemovePageShortcut removes a page from the left sidebar shortcuts.
func (c *LocalClient) RemovePageShortcut(uid string) error {
	_, err := c.call("data.page.removeShortcut", uid)
	return err
}

// UpsertUser creates or updates a user entity.
func (c *LocalClient) UpsertUser(userUID, displayName string) error {
	args := map[string]interface{}{
		"user-uid": userUID,
	}
	if displayName != "" {
		args["display-name"] = displayName
	}
	_, err := c.call("data.user.upsert", args)
	return err
}

// ExecuteBatch executes a batch of actions atomically
// Note: Local API may not support batch-actions natively, falls back to sequential execution
func (c *LocalClient) ExecuteBatch(batch *BatchBuilder) error {
	// Local API doesn't have batch-actions, execute sequentially
	for i, action := range batch.Build() {
		actionType, ok := action["action"].(string)
		if !ok {
			return fmt.Errorf("batch action %d: missing or invalid action type", i)
		}
		switch actionType {
		case "create-page":
			page, ok := action["page"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("batch action %d: invalid page data", i)
			}
			title, ok := page["title"].(string)
			if !ok {
				return fmt.Errorf("batch action %d: missing page title", i)
			}
			if err := c.CreatePage(title); err != nil {
				return err
			}
		case "create-block":
			block, ok := action["block"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("batch action %d: invalid block data", i)
			}
			location, ok := action["location"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("batch action %d: invalid location data", i)
			}
			content := ""
			if s, ok := block["string"].(string); ok {
				content = s
			}
			parentUID := ""
			if p, ok := location["parent-uid"].(string); ok {
				parentUID = p
			}
			order := location["order"]
			if err := c.CreateBlock(parentUID, content, order); err != nil {
				return err
			}
		case "update-block":
			block, ok := action["block"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("batch action %d: invalid block data", i)
			}
			uid, ok := block["uid"].(string)
			if !ok {
				return fmt.Errorf("batch action %d: missing block uid", i)
			}
			content, _ := block["string"].(string)
			if err := c.UpdateBlock(uid, content); err != nil {
				return err
			}
		case "move-block":
			block, ok := action["block"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("batch action %d: invalid block data", i)
			}
			location, ok := action["location"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("batch action %d: invalid location data", i)
			}
			uid, ok := block["uid"].(string)
			if !ok {
				return fmt.Errorf("batch action %d: missing block uid", i)
			}
			parentUID, ok := location["parent-uid"].(string)
			if !ok {
				return fmt.Errorf("batch action %d: missing parent-uid", i)
			}
			order := location["order"]
			if err := c.MoveBlock(uid, parentUID, order); err != nil {
				return err
			}
		case "delete-block":
			block, ok := action["block"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("batch action %d: invalid block data", i)
			}
			uid, ok := block["uid"].(string)
			if !ok {
				return fmt.Errorf("batch action %d: missing block uid", i)
			}
			if err := c.DeleteBlock(uid); err != nil {
				return err
			}
		case "delete-page":
			page, ok := action["page"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("batch action %d: invalid page data", i)
			}
			uid, ok := page["uid"].(string)
			if !ok {
				return fmt.Errorf("batch action %d: missing page uid", i)
			}
			if err := c.DeletePage(uid); err != nil {
				return err
			}
		default:
			return fmt.Errorf("batch action %d: unknown action type %q", i, actionType)
		}
	}
	return nil
}

// GetPageByTitle retrieves a page by its title
func (c *LocalClient) GetPageByTitle(title string) (json.RawMessage, error) {
	query := roamdb.QueryPageByTitle(title)
	results, err := c.Query(query)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, NotFoundError{Message: fmt.Sprintf("page not found: %s", title)}
	}

	// Get the entity ID from the query result
	eid := results[0][0]

	// Pull the full page data with children
	return c.Pull(eid, "[* {:block/children ...}]")
}

// GetBlockByUID retrieves a block by its UID
func (c *LocalClient) GetBlockByUID(uid string) (json.RawMessage, error) {
	query := roamdb.QueryBlockByUID(uid)
	results, err := c.Query(query)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, NotFoundError{Message: fmt.Sprintf("block not found: %s", uid)}
	}

	eid := results[0][0]
	return c.Pull(eid, "[* {:block/children ...}]")
}

// SearchBlocks searches for blocks containing the given text
func (c *LocalClient) SearchBlocks(text string, limit int) ([][]interface{}, error) {
	results, err := c.Query(roamdb.QuerySearchBlocksContains(text))
	if err != nil {
		return nil, err
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// ListPages returns pages, optionally filtered by modification date
func (c *LocalClient) ListPages(modifiedToday bool, limit int) ([][]interface{}, error) {
	results, err := c.Query(roamdb.QueryListPages(modifiedToday, time.Now()))
	if err != nil {
		return nil, err
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Undo reverses the last action in the graph
func (c *LocalClient) Undo() error {
	_, err := c.call("data.undo")
	return err
}

// Redo reapplies the last undone action
func (c *LocalClient) Redo() error {
	_, err := c.call("data.redo")
	return err
}

// ReorderBlocks sets the order of child blocks under a parent
// blockUIDs should be in the desired order
func (c *LocalClient) ReorderBlocks(parentUID string, blockUIDs []string) error {
	args := map[string]interface{}{
		"parent-uid": parentUID,
		"block-uids": blockUIDs,
	}
	_, err := c.call("data.block.reorderBlocks", args)
	if err != nil {
		// Fallback for older local API versions
		_, fallbackErr := c.call("data.block.reorder", args)
		if fallbackErr == nil {
			return nil
		}
		return err
	}
	return nil
}

// UploadFile uploads a file to the graph and returns the URL
func (c *LocalClient) UploadFile(filename string, data []byte) (string, error) {
	args := map[string]interface{}{
		"filename": filename,
		"data":     base64.StdEncoding.EncodeToString(data),
	}

	result, err := c.call("file.upload", args)
	if err != nil {
		return "", err
	}

	var response struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return "", fmt.Errorf("failed to parse upload response: %w", err)
	}

	return response.URL, nil
}

// DeleteFile deletes a file hosted in the graph.
func (c *LocalClient) DeleteFile(url string) error {
	args := map[string]interface{}{
		"url": url,
	}
	_, err := c.call("file.delete", args)
	return err
}

// GetFile retrieves a file from the given URL.
// Returns file bytes plus optional name/type metadata if available.
func (c *LocalClient) GetFile(url string) ([]byte, string, string, error) {
	args := map[string]interface{}{
		"url": url,
	}

	result, err := c.call("file.get", args)
	if err != nil {
		return nil, "", "", err
	}

	data, name, fileType, err := parseLocalFileResult(result)
	if err != nil {
		return nil, "", "", err
	}
	return data, name, fileType, nil
}

// DownloadFile downloads a file from the given URL.
// This keeps backward compatibility with older Local API action names.
func (c *LocalClient) DownloadFile(url string) ([]byte, error) {
	data, _, _, err := c.GetFile(url)
	if err == nil {
		return data, nil
	}

	// Fallback to older action name used by some builds.
	args := map[string]interface{}{
		"url": url,
	}

	result, fallbackErr := c.call("file.download", args)
	if fallbackErr != nil {
		return nil, err
	}

	// Result is base64 encoded
	var encoded string
	if decodeErr := json.Unmarshal(result, &encoded); decodeErr != nil {
		return nil, fmt.Errorf("failed to parse download response: %w", decodeErr)
	}

	data, decodeErr := base64.StdEncoding.DecodeString(encoded)
	if decodeErr != nil {
		return nil, fmt.Errorf("failed to decode file data: %w", decodeErr)
	}

	return data, nil
}

func parseLocalCreateUID(raw json.RawMessage) (string, bool) {
	var parsed interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", false
	}

	switch value := parsed.(type) {
	case string:
		if value != "" {
			return value, true
		}
	case map[string]interface{}:
		if uid, ok := value["uid"].(string); ok && uid != "" {
			return uid, true
		}
		if uid, ok := value["block/uid"].(string); ok && uid != "" {
			return uid, true
		}
		if block, ok := value["block"].(map[string]interface{}); ok {
			if uid, ok := block["uid"].(string); ok && uid != "" {
				return uid, true
			}
			if uid, ok := block["block/uid"].(string); ok && uid != "" {
				return uid, true
			}
		}
	case map[string]string:
		if uid := value["uid"]; uid != "" {
			return uid, true
		}
		if uid := value["block/uid"]; uid != "" {
			return uid, true
		}
	}

	return "", false
}

func parseLocalFileResult(raw json.RawMessage) ([]byte, string, string, error) {
	var parsed interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, "", "", fmt.Errorf("failed to parse file response: %w", err)
	}

	decode := func(encoded string) ([]byte, error) {
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("failed to decode file data: %w", err)
		}
		return data, nil
	}

	switch value := parsed.(type) {
	case string:
		data, err := decode(value)
		return data, "", "", err
	case map[string]interface{}:
		// Some implementations may nest data under "file"
		if nested, ok := value["file"].(map[string]interface{}); ok {
			value = nested
		}

		name, _ := value["name"].(string)
		if name == "" {
			name, _ = value["filename"].(string)
		}
		fileType, _ := value["type"].(string)
		if fileType == "" {
			fileType, _ = value["mime"].(string)
		}

		for _, key := range []string{"data", "base64", "content"} {
			if encoded, ok := value[key].(string); ok && encoded != "" {
				data, err := decode(encoded)
				return data, name, fileType, err
			}
		}
	}

	return nil, "", "", fmt.Errorf("unsupported file response format")
}

// Ensure LocalClient implements RoamAPI at compile time
var _ RoamAPI = (*LocalClient)(nil)
