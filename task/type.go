package task

import (
	"strings"

	"github.com/boolean-maybe/tiki/workflow"
)

// convenience constants matching the canonical types bundled in kanban.yaml.
// As with status, `type` is an ordinary enum field — these constants exist
// only as a convenience for tests and call sites that compare against
// well-known keys.
const (
	TypeStory = "story"
	TypeBug   = "bug"
	TypeSpike = "spike"
	TypeEpic  = "epic"
)

// typeField returns the loaded "type" workflow field, or (zero, false) when
// no type field is configured.
func typeField() (workflow.FieldDef, bool) {
	fd, ok := workflow.Field("type")
	if !ok || fd.Type != workflow.TypeEnum {
		return workflow.FieldDef{}, false
	}
	return fd, true
}

// NormalizeTypeKey lowercases, trims, and strips all separators. Generic —
// has no knowledge of the loaded type field's allowed values.
func NormalizeTypeKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// ParseType parses a raw string into a canonical type key with validation.
// Returns the canonical key and true if recognized, or ("", false) otherwise.
func ParseType(t string) (string, bool) {
	normalized := NormalizeTypeKey(t)
	fd, ok := typeField()
	if !ok {
		return "", false
	}
	if fd.IsValidEnum(normalized) {
		return normalized, true
	}
	return "", false
}

// TypeLabel returns a human-readable label for a task type.
func TypeLabel(taskType string) string {
	fd, ok := typeField()
	if !ok {
		return taskType
	}
	v, found := fd.LookupEnum(taskType)
	if !found {
		return taskType
	}
	if v.Label != "" {
		return v.Label
	}
	return v.Value
}

// TypeEmoji returns the emoji for a task type.
func TypeEmoji(taskType string) string {
	fd, ok := typeField()
	if !ok {
		return ""
	}
	v, found := fd.LookupEnum(taskType)
	if !found {
		return ""
	}
	return v.Emoji
}

// TypeDisplay returns a formatted display string with label and emoji.
func TypeDisplay(taskType string) string {
	fd, ok := typeField()
	if !ok {
		return taskType
	}
	return fd.EnumDisplay(taskType)
}

// ParseDisplay reverses a TypeDisplay() string back to a canonical key.
// Returns (key, true) on match, or ("", false) for unrecognized display strings.
func ParseDisplay(display string) (string, bool) {
	fd, ok := typeField()
	if !ok {
		return "", false
	}
	return fd.EnumParseDisplay(display)
}

// AllTypes returns the ordered list of all configured type keys.
func AllTypes() []string {
	fd, ok := typeField()
	if !ok {
		return nil
	}
	return fd.AllowedValues()
}

// DefaultType returns the creation-default type — the value with default:
// true, or "" when no default is configured.
func DefaultType() string {
	fd, ok := typeField()
	if !ok {
		return ""
	}
	return fd.EnumDefault()
}

// IsValidType reports whether t is a recognized type key.
func IsValidType(t string) bool {
	fd, ok := typeField()
	if !ok {
		return false
	}
	return fd.IsValidEnum(t)
}
