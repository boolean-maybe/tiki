package task

import (
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/workflow"
)

// convenience constants matching the default workflow.yaml statuses.
const (
	StatusBacklog    = workflow.StatusBacklog
	StatusReady      = workflow.StatusReady
	StatusInProgress = workflow.StatusInProgress
	StatusReview     = workflow.StatusReview
	StatusDone       = workflow.StatusDone
)

// ParseStatus normalizes a raw status string and validates it against the registry.
// Empty input returns the configured default status.
// Unknown values return (DefaultStatus(), false).
func ParseStatus(status string) (string, bool) {
	normalized := workflow.NormalizeStatusKey(status)
	if normalized == "" {
		return DefaultStatus(), true
	}
	reg := config.GetStatusRegistry()
	if reg.IsValid(normalized) {
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
// registry default when the input is not recognized.
func StatusToString(status string) string {
	reg := config.GetStatusRegistry()
	if reg.IsValid(status) {
		return status
	}
	return reg.DefaultKey()
}

// StatusEmoji returns the emoji for a status from the registry.
func StatusEmoji(status string) string {
	reg := config.GetStatusRegistry()
	if def, ok := reg.Lookup(status); ok {
		return def.Emoji
	}
	return ""
}

// StatusLabel returns the display label for a status from the registry.
func StatusLabel(status string) string {
	reg := config.GetStatusRegistry()
	if def, ok := reg.Lookup(status); ok {
		return def.Label
	}
	return status
}

// StatusDisplay returns "Label Emoji" for a status.
func StatusDisplay(status string) string {
	label := StatusLabel(status)
	emoji := StatusEmoji(status)
	if emoji == "" {
		return label
	}
	return label + " " + emoji
}

// DefaultStatus returns the status configured as default in workflow.yaml.
func DefaultStatus() string {
	return config.GetStatusRegistry().DefaultKey()
}

// DoneStatus returns the status configured as done in workflow.yaml.
func DoneStatus() string {
	return config.GetStatusRegistry().DoneKey()
}

// AllStatuses returns the ordered list of all configured statuses.
func AllStatuses() []string {
	reg := config.GetStatusRegistry()
	defs := reg.All()
	statuses := make([]string, len(defs))
	for i, d := range defs {
		statuses[i] = d.Key
	}
	return statuses
}

// IsActiveStatus reports whether the status has the active flag set.
func IsActiveStatus(status string) bool {
	return config.GetStatusRegistry().IsActive(status)
}
