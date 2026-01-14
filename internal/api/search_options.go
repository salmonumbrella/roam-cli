package api

// SearchOptions configures data.search for the Local API.
type SearchOptions struct {
	SearchBlocks   bool
	SearchPages    bool
	HideCodeBlocks bool
	Limit          int
	// Pull is an EDN pull pattern string that specifies which attributes to return.
	// Examples: "[*]" for all attributes, "[:block/string :block/uid]" for specific fields.
	// The Roam API technically accepts both string and EDN vector formats, but this CLI
	// uses string exclusively since EDN vectors are serialized as strings anyway.
	Pull string
}
