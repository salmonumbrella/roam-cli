package api

// BlockOptions provides optional parameters for block creation/update.
// All fields except Content are optional.
type BlockOptions struct {
	// Content is the text content of the block (required for create)
	Content string
	// UID is an optional block UID to assign on create (empty = auto)
	UID string
	// Open controls whether the block is expanded (nil = don't set)
	Open *bool
	// Heading level: 1, 2, or 3 (nil = not a heading)
	Heading *int
	// TextAlign: "left", "center", "right", "justify" (empty = default)
	TextAlign string
	// ChildrenViewType: "bullet", "numbered", "document" (empty = default)
	ChildrenViewType string
	// BlockViewType: "bullet", "numbered", "document" (empty = default)
	BlockViewType string
	// Props is a map of custom properties (nil = don't set)
	Props map[string]interface{}
}

// ToBlock converts BlockOptions to a Block struct for API calls.
func (o BlockOptions) ToBlock() Block {
	return Block{
		String:           o.Content,
		UID:              o.UID,
		Open:             o.Open,
		Heading:          o.Heading,
		TextAlign:        o.TextAlign,
		ChildrenViewType: o.ChildrenViewType,
		BlockViewType:    o.BlockViewType,
		Props:            o.Props,
	}
}

// ApplyToMap adds optional block properties to an existing map.
// This is a helper for building API request payloads.
func (o BlockOptions) ApplyToMap(m map[string]interface{}) {
	if o.UID != "" {
		if _, exists := m["uid"]; !exists {
			m["uid"] = o.UID
		}
	}
	if o.Open != nil {
		m["open"] = *o.Open
	}
	if o.Heading != nil {
		m["heading"] = *o.Heading
	}
	if o.TextAlign != "" {
		m["text-align"] = o.TextAlign
	}
	if o.ChildrenViewType != "" {
		m["children-view-type"] = o.ChildrenViewType
	}
	if o.BlockViewType != "" {
		m["block-view-type"] = o.BlockViewType
	}
	if o.Props != nil {
		m["props"] = o.Props
	}
}
