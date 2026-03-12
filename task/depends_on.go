package task

import (
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"
)

// DependsOnValue is a custom type for dependsOn that provides lenient YAML unmarshaling.
// It gracefully handles invalid YAML by defaulting to an empty slice instead of failing.
// Values are uppercased to ensure consistent TIKI-XXXXXX format.
type DependsOnValue []string

// UnmarshalYAML implements custom unmarshaling for dependsOn with lenient error handling.
// Valid YAML list formats are parsed normally. Invalid formats (scalars, objects, etc.)
// default to empty slice with a warning log instead of returning an error.
func (d *DependsOnValue) UnmarshalYAML(value *yaml.Node) error {
	// Try to decode as []string (normal case)
	var deps []string
	if err := value.Decode(&deps); err == nil {
		// Filter out empty strings, trim whitespace, and uppercase IDs
		filtered := make([]string, 0, len(deps))
		for _, dep := range deps {
			trimmed := strings.TrimSpace(dep)
			if trimmed != "" {
				filtered = append(filtered, strings.ToUpper(trimmed))
			}
		}
		*d = DependsOnValue(filtered)
		return nil
	}

	// If decoding fails, log warning and default to empty
	slog.Warn("invalid dependsOn field, defaulting to empty",
		"received_type", value.Kind,
		"line", value.Line,
		"column", value.Column)
	*d = DependsOnValue([]string{})
	return nil // Don't return error - use default instead
}

// MarshalYAML implements YAML marshaling for DependsOnValue.
// Returns the underlying slice as-is for standard YAML serialization.
func (d DependsOnValue) MarshalYAML() (any, error) {
	return []string(d), nil
}

// ToStringSlice converts DependsOnValue to []string for use with Task entity.
func (d DependsOnValue) ToStringSlice() []string {
	if d == nil {
		return []string{}
	}
	return []string(d)
}
