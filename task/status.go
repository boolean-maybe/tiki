package task

import (
	"strings"

	"github.com/boolean-maybe/tiki/workflow"
)

// convenience constants matching the canonical statuses bundled in
// kanban.yaml. They are plain strings — `status` is an ordinary enum field
// and these constants exist only as a convenience for tests and call sites
// that compare against well-known keys. Workflows are free to define their
// own status sets; runtime validation goes through the field catalog.
const (
	StatusBacklog    = "backlog"
	StatusReady      = "ready"
	StatusInProgress = "inProgress"
	StatusReview     = "review"
	StatusDone       = "done"
)

// statusField returns the loaded "status" workflow field, or (zero, false)
// when no status field is configured (plain-document workflow).
func statusField() (workflow.FieldDef, bool) {
	fd, ok := workflow.Field("status")
	if !ok || fd.Type != workflow.TypeEnum {
		return workflow.FieldDef{}, false
	}
	return fd, true
}

// NormalizeStatusKey converts a raw status key to canonical camelCase. Splits
// on "_", "-", " ", and camelCase boundaries, then reassembles. Examples:
// "in_progress" → "inProgress", "In Progress" → "inProgress",
// "inProgress" → "inProgress", "IN_PROGRESS" → "inProgress".
//
// Note: this is a generic camelCase normalizer — it has no knowledge of the
// loaded status field's allowed values. Callers should validate against the
// field catalog after normalization.
func NormalizeStatusKey(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}
	words := splitCamelOrSeparators(trimmed)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	for i, w := range words {
		if i == 0 {
			b.WriteString(strings.ToLower(w))
			continue
		}
		b.WriteString(strings.ToUpper(w[:1]))
		b.WriteString(strings.ToLower(w[1:]))
	}
	return b.String()
}

func splitCamelOrSeparators(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	var words []string
	for _, p := range parts {
		words = append(words, splitCamelCase(p)...)
	}
	return words
}

func splitCamelCase(s string) []string {
	if s == "" {
		return nil
	}
	var words []string
	start := 0
	for i := 1; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' && s[i-1] >= 'a' && s[i-1] <= 'z' {
			words = append(words, s[start:i])
			start = i
		}
	}
	words = append(words, s[start:])
	return words
}

// ParseStatus normalizes a raw status string and validates it against the
// loaded status field's enum values. Empty input returns the configured
// default. Unknown values return (DefaultStatus(), false).
func ParseStatus(status string) (string, bool) {
	normalized := NormalizeStatusKey(status)
	if normalized == "" {
		return DefaultStatus(), true
	}
	fd, ok := statusField()
	if !ok {
		return normalized, true
	}
	if fd.IsValidEnum(normalized) {
		return normalized, true
	}
	return DefaultStatus(), false
}

// NormalizeStatus standardizes a raw status string into a canonical status key.
func NormalizeStatus(status string) string {
	normalized, _ := ParseStatus(status)
	return normalized
}

// MapStatus maps a raw status string to a canonical status key.
func MapStatus(status string) string {
	return NormalizeStatus(status)
}

// StatusToString returns the canonical key string, falling back to the
// configured default when the input is not recognized.
func StatusToString(status string) string {
	fd, ok := statusField()
	if !ok {
		return status
	}
	if fd.IsValidEnum(status) {
		return status
	}
	return fd.EnumDefault()
}

// StatusEmoji returns the emoji for a status from the field catalog.
func StatusEmoji(status string) string {
	fd, ok := statusField()
	if !ok {
		return ""
	}
	v, found := fd.LookupEnum(status)
	if !found {
		return ""
	}
	return v.Emoji
}

// StatusLabel returns the display label for a status from the field catalog.
func StatusLabel(status string) string {
	fd, ok := statusField()
	if !ok {
		return status
	}
	v, found := fd.LookupEnum(status)
	if !found {
		return status
	}
	if v.Label != "" {
		return v.Label
	}
	return v.Value
}

// StatusDisplay returns "Label Emoji" for a status.
func StatusDisplay(status string) string {
	fd, ok := statusField()
	if !ok {
		return status
	}
	return fd.EnumDisplay(status)
}

// ParseStatusDisplay reverses a StatusDisplay() string back to a canonical
// key. Returns ("", false) for an unrecognized display.
func ParseStatusDisplay(display string) (string, bool) {
	fd, ok := statusField()
	if !ok {
		return "", false
	}
	return fd.EnumParseDisplay(display)
}

// DefaultStatus returns the status configured as default in workflow.yaml,
// or "" when no status field is defined or no value is marked default.
func DefaultStatus() string {
	fd, ok := statusField()
	if !ok {
		return ""
	}
	return fd.EnumDefault()
}

// AllStatuses returns the ordered list of all configured status keys.
func AllStatuses() []string {
	fd, ok := statusField()
	if !ok {
		return nil
	}
	return fd.AllowedValues()
}

// IsValidStatus reports whether key is a recognized status value.
func IsValidStatus(key string) bool {
	fd, ok := statusField()
	if !ok {
		return false
	}
	return fd.IsValidEnum(key)
}
