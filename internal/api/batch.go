package api

import "fmt"

// BatchBuilder constructs a batch of actions that can reference each other via tempids.
// Tempids are negative integers that act as placeholders for entity IDs.
type BatchBuilder struct {
	actions    []map[string]interface{}
	nextTempID int
}

// NewBatchBuilder creates a new batch builder.
func NewBatchBuilder() *BatchBuilder {
	return &BatchBuilder{
		actions:    make([]map[string]interface{}, 0),
		nextTempID: -1,
	}
}

// allocateTempID returns the next available tempid and decrements the counter.
func (b *BatchBuilder) allocateTempID() int {
	id := b.nextTempID
	b.nextTempID--
	return id
}

// CreatePage adds a create-page action and returns a tempid reference.
func (b *BatchBuilder) CreatePage(opts PageOptions) string {
	tempID := b.allocateTempID()
	uidValue := interface{}(tempID)
	uidRef := fmt.Sprintf("%d", tempID)
	if opts.UID != "" {
		uidValue = opts.UID
		uidRef = opts.UID
	}
	action := map[string]interface{}{
		"action": "create-page",
		"page": map[string]interface{}{
			"uid":   uidValue,
			"title": opts.Title,
		},
	}
	if opts.ChildrenViewType != "" {
		action["page"].(map[string]interface{})["children-view-type"] = opts.ChildrenViewType
	}
	b.actions = append(b.actions, action)
	return uidRef
}

// CreateBlock adds a create-block action and returns a tempid reference.
func (b *BatchBuilder) CreateBlock(loc Location, opts BlockOptions) string {
	tempID := b.allocateTempID()
	uidValue := interface{}(tempID)
	uidRef := fmt.Sprintf("%d", tempID)
	if opts.UID != "" {
		uidValue = opts.UID
		uidRef = opts.UID
	}

	blockMap := map[string]interface{}{
		"uid":    uidValue,
		"string": opts.Content,
	}
	opts.ApplyToMap(blockMap)

	action := map[string]interface{}{
		"action":   "create-block",
		"location": b.buildLocation(loc),
		"block":    blockMap,
	}
	b.actions = append(b.actions, action)
	return uidRef
}

// UpdateBlock adds an update-block action.
func (b *BatchBuilder) UpdateBlock(uid string, opts BlockOptions) {
	blockMap := map[string]interface{}{
		"uid": b.parseUID(uid),
	}
	if opts.Content != "" {
		blockMap["string"] = opts.Content
	}
	opts.ApplyToMap(blockMap)

	action := map[string]interface{}{
		"action": "update-block",
		"block":  blockMap,
	}
	b.actions = append(b.actions, action)
}

// UpdatePage adds an update-page action.
func (b *BatchBuilder) UpdatePage(uid string, opts PageOptions) {
	pageMap := map[string]interface{}{
		"uid": b.parseUID(uid),
	}
	if opts.Title != "" {
		pageMap["title"] = opts.Title
	}
	if opts.ChildrenViewType != "" {
		pageMap["children-view-type"] = opts.ChildrenViewType
	}
	action := map[string]interface{}{
		"action": "update-page",
		"page":   pageMap,
	}
	b.actions = append(b.actions, action)
}

// MoveBlock adds a move-block action.
func (b *BatchBuilder) MoveBlock(uid string, loc Location) {
	action := map[string]interface{}{
		"action":   "move-block",
		"location": b.buildLocation(loc),
		"block": map[string]interface{}{
			"uid": b.parseUID(uid),
		},
	}
	b.actions = append(b.actions, action)
}

// DeleteBlock adds a delete-block action.
func (b *BatchBuilder) DeleteBlock(uid string) {
	action := map[string]interface{}{
		"action": "delete-block",
		"block": map[string]interface{}{
			"uid": b.parseUID(uid),
		},
	}
	b.actions = append(b.actions, action)
}

// DeletePage adds a delete-page action.
func (b *BatchBuilder) DeletePage(uid string) {
	action := map[string]interface{}{
		"action": "delete-page",
		"page": map[string]interface{}{
			"uid": b.parseUID(uid),
		},
	}
	b.actions = append(b.actions, action)
}

// Build returns the actions as a slice ready for the batch-actions API.
func (b *BatchBuilder) Build() []map[string]interface{} {
	return b.actions
}

// buildLocation converts Location to map, handling tempid references.
func (b *BatchBuilder) buildLocation(loc Location) map[string]interface{} {
	m := make(map[string]interface{})
	m["order"] = loc.Order

	if loc.ParentUID != "" {
		m["parent-uid"] = b.parseUID(loc.ParentUID)
	} else if loc.DailyNoteDate != "" {
		m["page-title"] = map[string]string{
			"daily-note-page": loc.DailyNoteDate,
		}
	} else if loc.PageTitle != "" {
		m["page-title"] = loc.PageTitle
	}

	return m
}

// parseUID converts a string UID to the appropriate type.
// Tempid strings like "-1" become integers, real UIDs stay as strings.
func (b *BatchBuilder) parseUID(uid string) interface{} {
	// Check if it's a tempid (negative integer string)
	var tempID int
	if n, err := fmt.Sscanf(uid, "%d", &tempID); err == nil && n == 1 && tempID < 0 {
		return tempID
	}
	return uid
}
