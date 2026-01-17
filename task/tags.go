package task

import (
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"
)

// TagsValue is a custom type for tags that provides lenient YAML unmarshaling.
// It gracefully handles invalid YAML by defaulting to an empty slice instead of failing.
type TagsValue []string

// UnmarshalYAML implements custom unmarshaling for tags with lenient error handling.
// Valid YAML list formats are parsed normally. Invalid formats (scalars, objects, etc.)
// default to empty slice with a warning log instead of returning an error.
func (t *TagsValue) UnmarshalYAML(value *yaml.Node) error {
	// Try to decode as []string (normal case)
	var tags []string
	if err := value.Decode(&tags); err == nil {
		// Filter out empty strings and whitespace-only strings
		filtered := make([]string, 0, len(tags))
		for _, tag := range tags {
			trimmed := strings.TrimSpace(tag)
			if trimmed != "" {
				filtered = append(filtered, trimmed)
			}
		}
		*t = TagsValue(filtered)
		return nil
	}

	// If decoding fails, log warning and default to empty
	slog.Warn("invalid tags field, defaulting to empty",
		"received_type", value.Kind,
		"line", value.Line,
		"column", value.Column)
	*t = TagsValue([]string{})
	return nil // Don't return error - use default instead
}

// MarshalYAML implements YAML marshaling for TagsValue.
// Returns the underlying slice as-is for standard YAML serialization.
func (t TagsValue) MarshalYAML() (interface{}, error) {
	return []string(t), nil
}

// ToStringSlice converts TagsValue to []string for use with Task entity.
func (t TagsValue) ToStringSlice() []string {
	if t == nil {
		return []string{}
	}
	return []string(t)
}
