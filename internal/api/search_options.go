package api

// SearchOptions configures data.search for the Local API.
type SearchOptions struct {
	SearchBlocks   bool
	SearchPages    bool
	HideCodeBlocks bool
	Limit          int
	Pull           interface{}
}
