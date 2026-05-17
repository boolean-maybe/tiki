package tikidetail

import (
	"time"

	"github.com/boolean-maybe/tiki/workflow/value"
)

// tiki_edit_fields.go contains small typed-conversion helpers used by
// TikiEditView when it bridges registry editor strings into typed save
// callbacks. The previous typed-widget builders (ensureStatusSelectList,
// ensurePointsInput, ...) were retired when both views switched to the
// shared field registry — only the plain conversion helpers remain.

// parseDueDateText forwards to the workflow/value date parser. Returns the
// parsed time and a validity flag; an empty input returns (zero, true) so
// the caller can treat "no due date" as a valid state distinct from
// malformed input.
func parseDueDateText(text string) (time.Time, bool) {
	return value.ParseDate(text)
}
