package roamdb

import (
	"encoding/json"
	"sort"
)

// Page represents a Roam page as returned by pull.
type Page struct {
	Title    string  `json:"node/title"`
	UID      string  `json:"block/uid"`
	EditTime int64   `json:"edit/time,omitempty"`
	Children []Block `json:"block/children,omitempty"`
}

// UnmarshalJSON handles both standard API keys (node/title) and Local API keys (:node/title).
func (p *Page) UnmarshalJSON(data []byte) error {
	// Try with colon-prefixed keys first (Local API format)
	type colonPage struct {
		Title    string  `json:":node/title"`
		UID      string  `json:":block/uid"`
		EditTime int64   `json:":edit/time,omitempty"`
		Children []Block `json:":block/children,omitempty"`
	}

	var cp colonPage
	if err := json.Unmarshal(data, &cp); err == nil && (cp.Title != "" || cp.UID != "") {
		p.Title = cp.Title
		p.UID = cp.UID
		p.EditTime = cp.EditTime
		p.Children = cp.Children
		return nil
	}

	// Fall back to standard API format (no colon prefix)
	type standardPage struct {
		Title    string  `json:"node/title"`
		UID      string  `json:"block/uid"`
		EditTime int64   `json:"edit/time,omitempty"`
		Children []Block `json:"block/children,omitempty"`
	}

	var sp standardPage
	if err := json.Unmarshal(data, &sp); err != nil {
		return err
	}

	p.Title = sp.Title
	p.UID = sp.UID
	p.EditTime = sp.EditTime
	p.Children = sp.Children
	return nil
}

// Block represents a Roam block as returned by pull.
type Block struct {
	String   string  `json:"block/string"`
	UID      string  `json:"block/uid"`
	Order    int     `json:"block/order,omitempty"`
	Children []Block `json:"block/children,omitempty"`
}

// UnmarshalJSON handles both standard API keys (block/string) and Local API keys (:block/string).
func (b *Block) UnmarshalJSON(data []byte) error {
	// Try with colon-prefixed keys first (Local API format)
	type colonBlock struct {
		String   string  `json:":block/string"`
		UID      string  `json:":block/uid"`
		Order    int     `json:":block/order,omitempty"`
		Children []Block `json:":block/children,omitempty"`
	}

	var cb colonBlock
	if err := json.Unmarshal(data, &cb); err == nil && (cb.String != "" || cb.UID != "") {
		b.String = cb.String
		b.UID = cb.UID
		b.Order = cb.Order
		b.Children = cb.Children
		return nil
	}

	// Fall back to standard API format (no colon prefix)
	type standardBlock struct {
		String   string  `json:"block/string"`
		UID      string  `json:"block/uid"`
		Order    int     `json:"block/order,omitempty"`
		Children []Block `json:"block/children,omitempty"`
	}

	var sb standardBlock
	if err := json.Unmarshal(data, &sb); err != nil {
		return err
	}

	b.String = sb.String
	b.UID = sb.UID
	b.Order = sb.Order
	b.Children = sb.Children
	return nil
}

// NormalizeBlocks sorts blocks by order and recurses into children.
func NormalizeBlocks(blocks []Block) {
	if len(blocks) == 0 {
		return
	}

	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Order < blocks[j].Order
	})

	for i := range blocks {
		NormalizeBlocks(blocks[i].Children)
	}
}
