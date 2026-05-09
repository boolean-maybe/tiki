// Package value contains reusable workflow-field value types and helpers
// (dates, recurrence, points validation). These are workflow-field concerns —
// they are intentionally not part of the root tiki model package, which
// stays a generic document model.
package value

import (
	"log/slog"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DateFormat is the canonical date-only layout used by date-typed workflow
// fields (e.g. due).
const DateFormat = "2006-01-02"

// DateValue is a YAML date encoder/decoder for date-only (YYYY-MM-DD) values.
// Embeds time.Time to inherit IsZero() (required for yaml:",omitempty"). Used
// by the tikistore frontmatter encoder to emit date-only YAML for TypeDate
// workflow fields.
type DateValue struct {
	time.Time
}

// UnmarshalYAML implements custom unmarshaling for date values with lenient
// error handling. Valid date strings in YYYY-MM-DD format are parsed normally.
// Invalid formats or types default to zero time with a warning log instead of
// returning an error.
func (d *DateValue) UnmarshalYAML(value *yaml.Node) error {
	var dateStr string
	if err := value.Decode(&dateStr); err == nil {
		trimmed := strings.TrimSpace(dateStr)
		if trimmed == "" {
			*d = DateValue{}
			return nil
		}

		parsed, err := time.Parse(DateFormat, trimmed)
		if err == nil {
			*d = DateValue{Time: parsed}
			return nil
		}

		slog.Warn("invalid date format, defaulting to empty",
			"value", dateStr,
			"line", value.Line,
			"column", value.Column)
		*d = DateValue{}
		return nil
	}

	slog.Warn("invalid date field type, defaulting to empty",
		"received_type", value.Kind,
		"line", value.Line,
		"column", value.Column)
	*d = DateValue{}
	return nil
}

// MarshalYAML implements YAML marshaling for DateValue.
// Returns empty string for zero time, otherwise formats as YYYY-MM-DD.
func (d DateValue) MarshalYAML() (any, error) {
	if d.IsZero() {
		return "", nil
	}
	return d.Format(DateFormat), nil
}

// ToTime returns the embedded time.Time.
func (d DateValue) ToTime() time.Time {
	return d.Time
}

// ParseDate parses a date string in YYYY-MM-DD format.
// Returns (time.Time, true) on success, (zero time, false) on failure.
// Empty string is treated as valid and returns zero time with true.
func ParseDate(s string) (time.Time, bool) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return time.Time{}, true
	}

	parsed, err := time.Parse(DateFormat, trimmed)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
