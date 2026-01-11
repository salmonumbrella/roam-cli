package roamdb

import (
	"fmt"
	"strings"
	"time"
)

// EscapeString escapes quotes for safe embedding in Datalog strings.
func EscapeString(s string) string {
	return strings.ReplaceAll(s, `"`, `""`)
}

// QueryPageByTitle builds a query that finds a page entity by title.
func QueryPageByTitle(title string) string {
	escaped := EscapeString(title)
	return fmt.Sprintf(`[:find ?e :where [?e :node/title "%s"]]`, escaped)
}

// QueryBlockByUID builds a query that finds a block entity by UID.
func QueryBlockByUID(uid string) string {
	escaped := EscapeString(uid)
	return fmt.Sprintf(`[:find ?e :where [?e :block/uid "%s"]]`, escaped)
}

// QuerySearchBlocksContains builds a query to find blocks containing text.
func QuerySearchBlocksContains(text string) string {
	escaped := EscapeString(text)
	return fmt.Sprintf(`[:find ?uid ?string ?page-title
		:where
		[?b :block/uid ?uid]
		[?b :block/string ?string]
		[(clojure.string/includes? ?string "%s")]
		[?b :block/page ?page]
		[?page :node/title ?page-title]]`, escaped)
}

// QueryListPages builds a query for listing pages, optionally filtered by today.
func QueryListPages(modifiedToday bool, now time.Time) string {
	if modifiedToday {
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		timestamp := startOfDay.UnixMilli()

		return fmt.Sprintf(`[:find ?title ?uid ?edit-time
			:where
			[?p :node/title ?title]
			[?p :block/uid ?uid]
			[?p :edit/time ?edit-time]
			[(> ?edit-time %d)]]`, timestamp)
	}

	return `[:find ?title ?uid
		:where
		[?p :node/title ?title]
		[?p :block/uid ?uid]]`
}
