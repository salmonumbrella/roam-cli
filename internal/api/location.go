package api

// Location specifies where to create or move a block.
// Use exactly one of: ParentUID, PageTitle, or DailyNoteDate.
type Location struct {
	// ParentUID is the UID of the parent block or page
	ParentUID string
	// PageTitle targets a page by title (creates if not exists)
	PageTitle string
	// DailyNoteDate targets a daily note page by date in MM-DD-YYYY format
	DailyNoteDate string
	// Order is the position: int (0-indexed), "first", or "last"
	Order interface{}
}

// ToMap converts Location to a map suitable for JSON serialization.
func (l Location) ToMap() map[string]interface{} {
	m := make(map[string]interface{})
	m["order"] = l.Order

	if l.ParentUID != "" {
		m["parent-uid"] = l.ParentUID
	} else if l.DailyNoteDate != "" {
		m["page-title"] = map[string]string{
			"daily-note-page": l.DailyNoteDate,
		}
	} else if l.PageTitle != "" {
		m["page-title"] = l.PageTitle
	}

	return m
}
