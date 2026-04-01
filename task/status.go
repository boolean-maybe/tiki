package task

import (
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/workflow"
)

// Status is a type alias for workflow.StatusKey.
// This preserves compatibility: task.Status and workflow.StatusKey are the same type.
type Status = workflow.StatusKey

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
func ParseStatus(status string) (Status, bool) {
	normalized := workflow.NormalizeStatusKey(status)
	if normalized == "" {
		return DefaultStatus(), true
	}
	reg := config.GetStatusRegistry()
	if reg.IsValid(string(normalized)) {
		return normalized, true
	}
	return DefaultStatus(), false
}

// NormalizeStatus standardizes a raw status string into a Status.
func NormalizeStatus(status string) Status {
	normalized, _ := ParseStatus(status)
	return normalized
}

// MapStatus maps a raw status string to a Status constant.
func MapStatus(status string) Status {
	return NormalizeStatus(status)
}

// StatusToString converts a Status to its string representation.
func StatusToString(status Status) string {
	reg := config.GetStatusRegistry()
	if reg.IsValid(string(status)) {
		return string(status)
	}
	return string(reg.DefaultKey())
}

// StatusEmoji returns the emoji for a status from the registry.
func StatusEmoji(status Status) string {
	reg := config.GetStatusRegistry()
	if def, ok := reg.Lookup(string(status)); ok {
		return def.Emoji
	}
	return ""
}

// StatusLabel returns the display label for a status from the registry.
func StatusLabel(status Status) string {
	reg := config.GetStatusRegistry()
	if def, ok := reg.Lookup(string(status)); ok {
		return def.Label
	}
	return string(status)
}

// StatusDisplay returns "Label Emoji" for a status.
func StatusDisplay(status Status) string {
	label := StatusLabel(status)
	emoji := StatusEmoji(status)
	if emoji == "" {
		return label
	}
	return label + " " + emoji
}

// DefaultStatus returns the status configured as default in workflow.yaml.
func DefaultStatus() Status {
	return config.GetStatusRegistry().DefaultKey()
}

// DoneStatus returns the status configured as done in workflow.yaml.
func DoneStatus() Status {
	return config.GetStatusRegistry().DoneKey()
}

// AllStatuses returns the ordered list of all configured statuses.
func AllStatuses() []Status {
	reg := config.GetStatusRegistry()
	defs := reg.All()
	statuses := make([]Status, len(defs))
	for i, d := range defs {
		statuses[i] = Status(d.Key)
	}
	return statuses
}

// IsActiveStatus reports whether the status has the active flag set.
func IsActiveStatus(status Status) bool {
	return config.GetStatusRegistry().IsActive(string(status))
}
