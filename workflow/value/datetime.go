package value

import (
	"strings"
	"time"
)

// DateTimeFormat is the canonical datetime layout used by timestamp-typed
// workflow fields (e.g. createdAt, updatedAt, and workflow-declared datetime
// fields). It carries a minute-resolution time component alongside the date.
const DateTimeFormat = "2006-01-02 15:04"

// ParseDateTime parses a datetime string in "YYYY-MM-DD HH:MM" format.
// Returns (time.Time, true) on success, (zero time, false) on failure.
// Empty or whitespace-only input is treated as valid and returns zero time.
func ParseDateTime(s string) (time.Time, bool) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return time.Time{}, true
	}
	parsed, err := time.Parse(DateTimeFormat, trimmed)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

// FormatDateTime renders a time.Time in the canonical datetime layout.
// Zero time formats to the empty string so an unset field round-trips to empty.
func FormatDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(DateTimeFormat)
}
