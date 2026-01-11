package api

import "encoding/json"

// RoamAPI defines the interface for interacting with Roam Research.
// Both cloud API (Client) and local encrypted graph API (LocalClient)
// implement this interface, allowing commands to work with either backend.
type RoamAPI interface {
	// Query executes a Datalog query against the graph.
	// The query should be a valid Datalog query string.
	// Optional args can be passed for parameterized queries.
	Query(query string, args ...interface{}) ([][]interface{}, error)

	// Pull retrieves an entity by ID with the given selector pattern.
	// The eid can be an entity ID (int) or a lookup ref like [:block/uid "xxx"].
	// The selector is an EDN pattern like "[*]" or "[* {:block/children ...}]".
	Pull(eid interface{}, selector string) (json.RawMessage, error)

	// PullMany retrieves multiple entities by their IDs.
	// Each eid can be an entity ID or lookup ref.
	PullMany(eids []interface{}, selector string) (json.RawMessage, error)

	// Block operations

	// CreateBlock creates a new block under the specified parent.
	// The order can be an int (0-indexed position) or "first"/"last".
	CreateBlock(parentUID, content string, order interface{}) error

	// CreateBlockWithOptions creates a new block with extended properties.
	CreateBlockWithOptions(parentUID string, opts BlockOptions, order interface{}) error

	// CreateBlockAtLocation creates a block at a flexible location (page title or daily note).
	CreateBlockAtLocation(loc Location, opts BlockOptions) error

	// UpdateBlock updates the content of an existing block.
	UpdateBlock(uid, content string) error

	// UpdateBlockWithOptions updates a block with extended properties.
	UpdateBlockWithOptions(uid string, opts BlockOptions) error

	// MoveBlock moves a block to a new parent at the specified order.
	MoveBlock(uid, parentUID string, order interface{}) error

	// MoveBlockToLocation moves a block to a flexible location (page title or daily note).
	MoveBlockToLocation(uid string, loc Location) error

	// DeleteBlock removes a block from the graph.
	DeleteBlock(uid string) error

	// Page operations

	// CreatePage creates a new page with the given title.
	CreatePage(title string) error

	// CreatePageWithOptions creates a page with extended properties.
	CreatePageWithOptions(opts PageOptions) error

	// UpdatePage updates the title of an existing page.
	UpdatePage(uid, title string) error

	// UpdatePageWithOptions updates a page with extended properties.
	UpdatePageWithOptions(uid string, opts PageOptions) error

	// DeletePage removes a page from the graph.
	DeletePage(uid string) error

	// Batch operations

	// ExecuteBatch executes a batch of actions atomically with tempid support.
	ExecuteBatch(batch *BatchBuilder) error

	// GraphName returns the name of the graph this API is connected to.
	GraphName() string

	// High-level helper methods

	// GetPageByTitle retrieves a page by its title, returning NotFoundError if not found.
	GetPageByTitle(title string) (json.RawMessage, error)

	// GetBlockByUID retrieves a block by its UID, returning NotFoundError if not found.
	GetBlockByUID(uid string) (json.RawMessage, error)

	// SearchBlocks searches for blocks containing the given text.
	// Returns results as [uid, string, page-title] tuples.
	SearchBlocks(text string, limit int) ([][]interface{}, error)

	// ListPages returns pages, optionally filtered by modification date.
	// If modifiedToday is true, only returns pages modified today.
	// Limit of 0 means no limit.
	ListPages(modifiedToday bool, limit int) ([][]interface{}, error)
}
