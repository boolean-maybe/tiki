package taskdetail

import (
	"time"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

// task_edit_fields.go contains small typed-conversion helpers used by
// TaskEditView when it bridges registry editor strings into typed save
// callbacks. The previous typed-widget builders (ensureStatusSelectList,
// ensurePointsInput, ...) were retired when both views switched to the
// shared field registry — only the plain conversion helpers remain.

// prioritySaveFromDisplay reverses a priority display label (e.g. "P1",
// "P2", "—") back to its numeric value (1..5; 0 = absent).
func prioritySaveFromDisplay(display string) int {
	return taskpkg.PriorityFromDisplay(display)
}

// parseDueDateText forwards to the task package's date parser. Returns the
// parsed time and a validity flag; an empty input returns (zero, true) so
// the caller can treat "no due date" as a valid state distinct from
// malformed input.
func parseDueDateText(text string) (time.Time, bool) {
	return taskpkg.ParseDueDate(text)
}
