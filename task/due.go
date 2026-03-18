package task

import (
	"log/slog"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const DateFormat = "2006-01-02"

// DueValue is a custom type for due dates that provides lenient YAML unmarshaling.
// It gracefully handles invalid YAML by defaulting to zero time instead of failing.
// Embeds time.Time to inherit IsZero() method (required for yaml:",omitempty").
type DueValue struct {
	time.Time
}

// UnmarshalYAML implements custom unmarshaling for due dates with lenient error handling.
// Valid date strings in YYYY-MM-DD format are parsed normally. Invalid formats or types
// default to zero time with a warning log instead of returning an error.
func (d *DueValue) UnmarshalYAML(value *yaml.Node) error {
	// Try to decode as string (normal case)
	var dateStr string
	if err := value.Decode(&dateStr); err == nil {
		// Empty string means no due date (valid)
		trimmed := strings.TrimSpace(dateStr)
		if trimmed == "" {
			*d = DueValue{}
			return nil
		}

		// Parse as date
		parsed, err := time.Parse(DateFormat, trimmed)
		if err == nil {
			*d = DueValue{Time: parsed}
			return nil
		}

		// Invalid date format - log and default to zero
		slog.Warn("invalid due date format, defaulting to empty",
			"value", dateStr,
			"line", value.Line,
			"column", value.Column)
		*d = DueValue{}
		return nil
	}

	// If decoding fails, log warning and default to zero
	slog.Warn("invalid due field type, defaulting to empty",
		"received_type", value.Kind,
		"line", value.Line,
		"column", value.Column)
	*d = DueValue{}
	return nil // Don't return error - use default instead
}

// MarshalYAML implements YAML marshaling for DueValue.
// Returns empty string for zero time, otherwise formats as YYYY-MM-DD.
func (d DueValue) MarshalYAML() (any, error) {
	if d.IsZero() {
		return "", nil
	}
	return d.Format(DateFormat), nil
}

// ToTime converts DueValue to time.Time for use with Task entity.
func (d DueValue) ToTime() time.Time {
	return d.Time
}

// ParseDueDate parses a date string in YYYY-MM-DD format.
// Returns (time.Time, true) on success, (zero time, false) on failure.
// Empty string is treated as valid and returns zero time with true.
func ParseDueDate(s string) (time.Time, bool) {
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
