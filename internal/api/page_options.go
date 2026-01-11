package api

// PageOptions provides optional parameters for page creation/update.
type PageOptions struct {
	// Title is the page title (required for create)
	Title string
	// UID is an optional page UID to assign on create (empty = auto)
	UID string
	// ChildrenViewType: "bullet", "numbered", "document" (empty = default)
	ChildrenViewType string
}

// ToPage converts PageOptions to a Page struct.
func (o PageOptions) ToPage() Page {
	return Page(o)
}
