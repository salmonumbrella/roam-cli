package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/salmonumbrella/roam-cli/internal/roamdb"
)

const (
	// DefaultBaseURL is the base URL for Roam Research API
	DefaultBaseURL = "https://api.roamresearch.com"
	// DefaultTimeout is the default HTTP client timeout
	DefaultTimeout = 30 * time.Second
	// MaxRetries for rate limit errors
	MaxRetries = 3
	// InitialBackoff for rate limit retries
	InitialBackoff = 10 * time.Second
)

// Error types for specific API errors
type (
	// AuthenticationError indicates an authentication failure
	AuthenticationError struct{ Message string }
	// RateLimitError indicates rate limit exceeded
	RateLimitError struct{ Message string }
	// NotFoundError indicates a resource was not found
	NotFoundError struct{ Message string }
	// ValidationError indicates invalid input
	ValidationError struct{ Message string }
)

func (e AuthenticationError) Error() string { return e.Message }
func (e RateLimitError) Error() string      { return e.Message }
func (e NotFoundError) Error() string       { return e.Message }
func (e ValidationError) Error() string     { return e.Message }

// Client represents a Roam Research API client
type Client struct {
	baseURL       string
	apiToken      string
	graphName     string
	httpClient    *http.Client
	redirectCache map[string]string
	debug         bool
}

// ClientOption is a function that configures a Client
type ClientOption func(*Client)

// WithBaseURL sets a custom base URL for the client
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithTimeout sets a custom timeout for the HTTP client
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithDebug enables debug logging
func WithDebug(debug bool) ClientOption {
	return func(c *Client) {
		c.debug = debug
	}
}

// NewClient creates a new Roam Research API client
func NewClient(graphName, apiToken string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:       DefaultBaseURL,
		apiToken:      apiToken,
		graphName:     graphName,
		redirectCache: make(map[string]string),
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects automatically - we handle them manually
				return http.ErrUseLastResponse
			},
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// GraphName returns the graph name
func (c *Client) GraphName() string {
	return c.graphName
}

// SetDebug enables or disables debug logging
func (c *Client) SetDebug(debug bool) {
	c.debug = debug
}

// call makes a single API call to the specified path using background context
func (c *Client) call(path string, body interface{}) ([]byte, error) {
	return c.callCtx(context.Background(), path, body)
}

// callCtx makes a single API call to the specified path with context support
func (c *Client) callCtx(ctx context.Context, path string, body interface{}) ([]byte, error) {
	// Check for cached redirect URL
	baseURL := c.baseURL
	if cached, ok := c.redirectCache[c.graphName]; ok {
		baseURL = cached
	}

	url := baseURL + path

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Roam requires both Authorization and x-authorization headers
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("x-authorization", "Bearer "+c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Handle redirects (Roam uses 307 redirects to peer nodes)
	if resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusPermanentRedirect {
		location := resp.Header.Get("Location")
		if location == "" {
			return nil, fmt.Errorf("redirect without Location header")
		}

		// Parse peer redirect URL: https://peer-N...:PORT
		re := regexp.MustCompile(`https://(peer-\d+).*?:(\d+)`)
		matches := re.FindStringSubmatch(location)
		if matches == nil {
			return nil, fmt.Errorf("could not parse redirect URL: %s", location)
		}

		peer, port := matches[1], matches[2]
		redirectURL := fmt.Sprintf("https://%s.api.roamresearch.com:%s", peer, port)
		c.redirectCache[c.graphName] = redirectURL

		// Retry with new URL
		return c.callCtx(ctx, path, body)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return nil, AuthenticationError{Message: "invalid API token"}
		case http.StatusTooManyRequests:
			return nil, RateLimitError{Message: fmt.Sprintf("rate limit exceeded: %s", string(respBody))}
		case http.StatusBadRequest:
			return nil, ValidationError{Message: fmt.Sprintf("invalid request: %s", string(respBody))}
		case http.StatusInternalServerError:
			return nil, fmt.Errorf("server error: %s", string(respBody))
		default:
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}
	}

	return respBody, nil
}

// callWithRetry calls the API with retry logic for rate limits
func (c *Client) callWithRetry(path string, body interface{}) ([]byte, error) {
	backoff := InitialBackoff

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		resp, err := c.call(path, body)
		if err == nil {
			return resp, nil
		}

		// Only retry on rate limit errors
		if _, ok := err.(RateLimitError); !ok {
			return nil, err
		}

		if attempt < MaxRetries {
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	return nil, RateLimitError{Message: "rate limit exceeded after retries"}
}

// QueryResult represents the result of a Datalog query
type QueryResult struct {
	Result [][]interface{} `json:"result"`
}

// Query executes a Datalog query against the graph
func (c *Client) Query(query string, args ...interface{}) ([][]interface{}, error) {
	path := fmt.Sprintf("/api/graph/%s/q", c.graphName)

	body := map[string]interface{}{
		"query": query,
	}
	if len(args) > 0 {
		body["args"] = args
	}

	resp, err := c.callWithRetry(path, body)
	if err != nil {
		return nil, err
	}

	var result QueryResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse query result: %w", err)
	}

	return result.Result, nil
}

// PullResult represents the result of a pull operation
type PullResult struct {
	Result json.RawMessage `json:"result"`
}

// Pull retrieves an entity by ID with the given selector pattern
func (c *Client) Pull(eid interface{}, selector string) (json.RawMessage, error) {
	path := fmt.Sprintf("/api/graph/%s/pull", c.graphName)

	body := map[string]interface{}{
		"eid":      eid,
		"selector": selector,
	}

	resp, err := c.callWithRetry(path, body)
	if err != nil {
		return nil, err
	}

	var result PullResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse pull result: %w", err)
	}

	return result.Result, nil
}

// PullMany retrieves multiple entities by IDs
func (c *Client) PullMany(eids []interface{}, selector string) (json.RawMessage, error) {
	path := fmt.Sprintf("/api/graph/%s/pull-many", c.graphName)

	body := map[string]interface{}{
		"eids":     eids,
		"selector": selector,
	}

	resp, err := c.callWithRetry(path, body)
	if err != nil {
		return nil, err
	}

	var result PullResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse pull-many result: %w", err)
	}

	return result.Result, nil
}

// WriteAction represents a write operation type
type WriteAction string

const (
	ActionCreateBlock  WriteAction = "create-block"
	ActionUpdateBlock  WriteAction = "update-block"
	ActionMoveBlock    WriteAction = "move-block"
	ActionDeleteBlock  WriteAction = "delete-block"
	ActionCreatePage   WriteAction = "create-page"
	ActionUpdatePage   WriteAction = "update-page"
	ActionDeletePage   WriteAction = "delete-page"
	ActionBatchActions WriteAction = "batch-actions"
)

// BlockLocation specifies where to create/move a block
type BlockLocation struct {
	ParentUID string      `json:"parent-uid"`
	Order     interface{} `json:"order"` // int or "first"/"last"
}

// Block represents a Roam block
type Block struct {
	String           string `json:"string,omitempty"`
	UID              string `json:"uid,omitempty"`
	Open             *bool  `json:"open,omitempty"`
	Heading          *int   `json:"heading,omitempty"`
	TextAlign        string `json:"text-align,omitempty"`
	ChildrenViewType string `json:"children-view-type,omitempty"`
	BlockViewType    string `json:"block-view-type,omitempty"`
	Props            map[string]interface{} `json:"props,omitempty"`
}

// Page represents a Roam page
type Page struct {
	Title            string `json:"title,omitempty"`
	UID              string `json:"uid,omitempty"`
	ChildrenViewType string `json:"children-view-type,omitempty"`
}

// Write performs a write operation on the graph
func (c *Client) Write(action WriteAction, data map[string]interface{}) error {
	path := fmt.Sprintf("/api/graph/%s/write", c.graphName)

	body := map[string]interface{}{
		"action": string(action),
	}
	for k, v := range data {
		body[k] = v
	}

	_, err := c.callWithRetry(path, body)
	return err
}

// CreateBlock creates a new block
func (c *Client) CreateBlock(parentUID string, content string, order interface{}) error {
	return c.Write(ActionCreateBlock, map[string]interface{}{
		"location": BlockLocation{
			ParentUID: parentUID,
			Order:     order,
		},
		"block": Block{
			String: content,
		},
	})
}

// CreateBlockWithOptions creates a new block with extended properties
func (c *Client) CreateBlockWithOptions(parentUID string, opts BlockOptions, order interface{}) error {
	block := opts.ToBlock()
	return c.Write(ActionCreateBlock, map[string]interface{}{
		"location": BlockLocation{
			ParentUID: parentUID,
			Order:     order,
		},
		"block": block,
	})
}

// CreateBlockAtLocation creates a block at a flexible location
func (c *Client) CreateBlockAtLocation(loc Location, opts BlockOptions) error {
	block := opts.ToBlock()
	return c.Write(ActionCreateBlock, map[string]interface{}{
		"location": loc.ToMap(),
		"block":    block,
	})
}

// UpdateBlock updates an existing block
func (c *Client) UpdateBlock(uid string, content string) error {
	return c.Write(ActionUpdateBlock, map[string]interface{}{
		"block": Block{
			UID:    uid,
			String: content,
		},
	})
}

// UpdateBlockWithOptions updates a block with extended properties
func (c *Client) UpdateBlockWithOptions(uid string, opts BlockOptions) error {
	block := opts.ToBlock()
	block.UID = uid
	return c.Write(ActionUpdateBlock, map[string]interface{}{
		"block": block,
	})
}

// MoveBlock moves a block to a new location
func (c *Client) MoveBlock(uid string, parentUID string, order interface{}) error {
	return c.MoveBlockToLocation(uid, Location{
		ParentUID: parentUID,
		Order:     order,
	})
}

// MoveBlockToLocation moves a block to a flexible location
func (c *Client) MoveBlockToLocation(uid string, loc Location) error {
	return c.Write(ActionMoveBlock, map[string]interface{}{
		"location": loc.ToMap(),
		"block": Block{
			UID: uid,
		},
	})
}

// DeleteBlock deletes a block
func (c *Client) DeleteBlock(uid string) error {
	return c.Write(ActionDeleteBlock, map[string]interface{}{
		"block": Block{
			UID: uid,
		},
	})
}

// CreatePage creates a new page
func (c *Client) CreatePage(title string) error {
	return c.Write(ActionCreatePage, map[string]interface{}{
		"page": Page{
			Title: title,
		},
	})
}

// CreatePageWithOptions creates a new page with extended properties
func (c *Client) CreatePageWithOptions(opts PageOptions) error {
	page := opts.ToPage()
	return c.Write(ActionCreatePage, map[string]interface{}{
		"page": page,
	})
}

// UpdatePage updates a page
func (c *Client) UpdatePage(uid string, title string) error {
	return c.UpdatePageWithOptions(uid, PageOptions{Title: title})
}

// UpdatePageWithOptions updates a page with extended properties
func (c *Client) UpdatePageWithOptions(uid string, opts PageOptions) error {
	page := opts.ToPage()
	page.UID = uid
	return c.Write(ActionUpdatePage, map[string]interface{}{
		"page": page,
	})
}

// DeletePage deletes a page
func (c *Client) DeletePage(uid string) error {
	return c.Write(ActionDeletePage, map[string]interface{}{
		"page": Page{
			UID: uid,
		},
	})
}

// ExecuteBatch executes a batch of actions atomically
func (c *Client) ExecuteBatch(batch *BatchBuilder) error {
	return c.Write(ActionBatchActions, map[string]interface{}{
		"actions": batch.Build(),
	})
}

// GetPageByTitle retrieves a page by its title
func (c *Client) GetPageByTitle(title string) (json.RawMessage, error) {
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
func (c *Client) GetBlockByUID(uid string) (json.RawMessage, error) {
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
func (c *Client) SearchBlocks(text string, limit int) ([][]interface{}, error) {
	results, err := c.Query(roamdb.QuerySearchBlocksContains(text))
	if err != nil {
		return nil, err
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Ensure Client implements RoamAPI at compile time
var _ RoamAPI = (*Client)(nil)

// ListPages returns pages modified today
func (c *Client) ListPages(modifiedToday bool, limit int) ([][]interface{}, error) {
	results, err := c.Query(roamdb.QueryListPages(modifiedToday, time.Now()))
	if err != nil {
		return nil, err
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}
