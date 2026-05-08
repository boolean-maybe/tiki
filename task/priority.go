package task

import (
	"strings"

	"github.com/boolean-maybe/tiki/workflow"
)

// Convenience constants for the canonical priority keys bundled in
// kanban.yaml. Like the status constants, these are plain strings — priority
// is an ordinary enum field. Workflows are free to define their own priority
// values; runtime validation goes through the field catalog.
const (
	PriorityHigh       = "high"
	PriorityMediumHigh = "medium-high"
	PriorityMedium     = "medium"
	PriorityMediumLow  = "medium-low"
	PriorityLow        = "low"
)

// priorityField returns the loaded "priority" workflow field, or
// (zero, false) when no priority field is configured or it isn't an enum.
func priorityField() (workflow.FieldDef, bool) {
	fd, ok := workflow.Field("priority")
	if !ok || fd.Type != workflow.TypeEnum {
		return workflow.FieldDef{}, false
	}
	return fd, true
}

// NormalizePriority standardizes a raw priority string into a canonical key.
// Empty/whitespace input returns "". Unknown values also return "" so callers
// can distinguish "absent" / "invalid" from a valid result. Recognized
// aliases like "high-medium" → "medium-high" are folded by the underlying
// enum's case-insensitive match plus the alias map below.
func NormalizePriority(s string) string {
	raw := strings.TrimSpace(strings.ToLower(s))
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, "_", "-")
	raw = strings.ReplaceAll(raw, " ", "-")
	if alias, ok := priorityAliases[raw]; ok {
		raw = alias
	}
	fd, ok := priorityField()
	if !ok {
		return raw
	}
	if fd.IsValidEnum(raw) {
		return raw
	}
	return ""
}

// priorityAliases maps legacy synonyms to canonical keys. The current
// kanban.yaml uses hyphenated medium-high / medium-low, but historic content
// or hand-typed input may use the inverted form (high-medium / low-medium).
// We intentionally keep this list minimal — exotic spellings should be
// rejected so authoring mistakes surface rather than being silently mapped.
var priorityAliases = map[string]string{
	"high-medium": "medium-high",
	"low-medium":  "medium-low",
}

// PriorityDisplay returns "Label Emoji" for the given priority key. Falls
// back to the key itself when the priority field is not configured.
func PriorityDisplay(key string) string {
	fd, ok := priorityField()
	if !ok {
		return key
	}
	return fd.EnumDisplay(key)
}

// PriorityLabel returns the emoji string for a priority key, or "" when
// not found. Used by task_box.go for the priority badge in compact lists.
func PriorityLabel(key string) string {
	fd, ok := priorityField()
	if !ok {
		return ""
	}
	v, found := fd.LookupEnum(key)
	if !found {
		return ""
	}
	return v.Emoji
}

// PriorityFromDisplay reverses PriorityDisplay() back to a canonical key.
// Returns ("", false) for an unrecognized display string.
func PriorityFromDisplay(display string) (string, bool) {
	fd, ok := priorityField()
	if !ok {
		return "", false
	}
	return fd.EnumParseDisplay(display)
}

// AllPriorityDisplayValues returns the configured priority's display strings
// in declaration order. Returns nil when no priority field is configured.
func AllPriorityDisplayValues() []string {
	fd, ok := priorityField()
	if !ok {
		return nil
	}
	keys := fd.AllowedValues()
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = fd.EnumDisplay(k)
	}
	return out
}

// AllPriorities returns the ordered list of all configured priority keys.
func AllPriorities() []string {
	fd, ok := priorityField()
	if !ok {
		return nil
	}
	return fd.AllowedValues()
}

// DefaultPriority returns the priority configured as default in workflow.yaml,
// or "" when no priority field is defined or no value is marked default.
func DefaultPriority() string {
	fd, ok := priorityField()
	if !ok {
		return ""
	}
	return fd.EnumDefault()
}

// LegacyPriorityKeyFromInt maps a pre-Phase-3 numeric priority (1..N) onto
// the configured priority enum's keys by rank: 1 → first declared value,
// N → Nth. Returns ("", false) when no priority enum is loaded, when n is
// out of [1, len(allowed)], or when the field is not an enum.
//
// Used by the legacy upgrader to migrate `priority: 1` style frontmatter
// into the new `priority: <enum-key>` form on load. The mapping is rank-
// based rather than hardcoded so a workflow that customizes priority levels
// (e.g. four levels instead of five) still upgrades sensibly: rank 1 maps
// to the highest-urgency declared value, regardless of its key name.
func LegacyPriorityKeyFromInt(n int) (string, bool) {
	fd, ok := priorityField()
	if !ok {
		return "", false
	}
	allowed := fd.AllowedValues()
	if n < 1 || n > len(allowed) {
		return "", false
	}
	return allowed[n-1], true
}

// IsValidPriority reports whether key is a recognized priority value.
func IsValidPriority(key string) bool {
	fd, ok := priorityField()
	if !ok {
		return false
	}
	return fd.IsValidEnum(key)
}
