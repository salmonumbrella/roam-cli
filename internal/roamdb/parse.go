package roamdb

import (
	"encoding/json"
	"fmt"
)

// ParsePage parses a pull response into a Page and normalizes children order.
func ParsePage(raw json.RawMessage) (*Page, error) {
	var page Page
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("parse page: %w", err)
	}
	NormalizeBlocks(page.Children)
	return &page, nil
}

// ParseBlock parses a pull response into a Block and normalizes children order.
func ParseBlock(raw json.RawMessage) (*Block, error) {
	var block Block
	if err := json.Unmarshal(raw, &block); err != nil {
		return nil, fmt.Errorf("parse block: %w", err)
	}
	NormalizeBlocks(block.Children)
	return &block, nil
}
