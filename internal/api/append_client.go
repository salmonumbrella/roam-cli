package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// DefaultAppendBaseURL is the base URL for Roam Append API
	DefaultAppendBaseURL = "https://append-api.roamresearch.com"
	// AppendAPITimeout is the default timeout for Append API requests
	AppendAPITimeout = 30 * time.Second
)

// AppendBlock represents a block for the append API
type AppendBlock struct {
	String           string        `json:"string"`
	UID              string        `json:"uid,omitempty"`
	Children         []AppendBlock `json:"children,omitempty"`
	Heading          *int          `json:"heading,omitempty"`
	Open             *bool         `json:"open,omitempty"`
	TextAlign        string        `json:"text-align,omitempty"`
	ChildrenViewType string        `json:"children-view-type,omitempty"`
}

// AppendLocation specifies where to append blocks.
type AppendLocation struct {
	Page      *AppendPage        `json:"page,omitempty"`
	Block     *AppendBlockTarget `json:"block,omitempty"`
	NestUnder *AppendNestUnder   `json:"nest-under,omitempty"`
}

// AppendPage specifies a page target.
type AppendPage struct {
	// Title can be a string or {"daily-note-page": "MM-DD-YYYY"}
	Title interface{} `json:"title"`
}

// AppendBlockTarget specifies a block target.
type AppendBlockTarget struct {
	UID string `json:"uid"`
}

// AppendNestUnder specifies a "nest-under" target string.
type AppendNestUnder struct {
	String string `json:"string"`
}

// AppendClient implements the Roam Append API for encrypted graphs
type AppendClient struct {
	baseURL    string
	apiToken   string
	graphName  string
	httpClient *http.Client
}

// AppendClientOption is a function that configures an AppendClient
type AppendClientOption func(*AppendClient)

// WithAppendBaseURL sets a custom base URL for the client
func WithAppendBaseURL(url string) AppendClientOption {
	return func(c *AppendClient) {
		c.baseURL = url
	}
}

// WithAppendTimeout sets a custom timeout
func WithAppendTimeout(timeout time.Duration) AppendClientOption {
	return func(c *AppendClient) {
		c.httpClient.Timeout = timeout
	}
}

// NewAppendClient creates a new Append API client
func NewAppendClient(graphName, apiToken string, opts ...AppendClientOption) *AppendClient {
	c := &AppendClient{
		baseURL:   DefaultAppendBaseURL,
		apiToken:  apiToken,
		graphName: graphName,
		httpClient: &http.Client{
			Timeout: AppendAPITimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// GraphName returns the graph name
func (c *AppendClient) GraphName() string {
	return c.graphName
}

// appendRequest represents a request to the Append API
type appendRequest struct {
	Location   AppendLocation `json:"location"`
	AppendData []AppendBlock  `json:"append-data"`
}

// AppendBlocks appends blocks to a page by title
func (c *AppendClient) AppendBlocks(pageTitle string, blocks []AppendBlock) error {
	loc := AppendLocation{
		Page: &AppendPage{Title: pageTitle},
	}
	return c.doAppend(context.Background(), loc, blocks)
}

// AppendToDailyNote appends blocks to a daily note page
// dateStr should be in MM-DD-YYYY format
func (c *AppendClient) AppendToDailyNote(dateStr string, blocks []AppendBlock) error {
	loc := AppendLocation{
		Page: &AppendPage{
			Title: map[string]string{"daily-note-page": dateStr},
		},
	}
	return c.doAppend(context.Background(), loc, blocks)
}

// AppendToBlock appends blocks as children of an existing block UID.
func (c *AppendClient) AppendToBlock(uid string, blocks []AppendBlock) error {
	loc := AppendLocation{
		Block: &AppendBlockTarget{UID: uid},
	}
	return c.doAppend(context.Background(), loc, blocks)
}

// AppendWithLocation appends blocks with a fully specified location.
func (c *AppendClient) AppendWithLocation(loc AppendLocation, blocks []AppendBlock) error {
	return c.doAppend(context.Background(), loc, blocks)
}

// doAppend performs the actual append operation with context support
func (c *AppendClient) doAppend(ctx context.Context, loc AppendLocation, blocks []AppendBlock) error {
	url := fmt.Sprintf("%s/api/graph/%s/append-blocks", c.baseURL, c.graphName)

	reqBody := appendRequest{
		Location:   loc,
		AppendData: blocks,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("x-authorization", "Bearer "+c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return AuthenticationError{Message: "invalid API token"}
		case http.StatusTooManyRequests:
			return RateLimitError{Message: fmt.Sprintf("rate limit exceeded: %s", string(respBody))}
		default:
			return fmt.Errorf("append API error (status %d): %s", resp.StatusCode, string(respBody))
		}
	}

	return nil
}
