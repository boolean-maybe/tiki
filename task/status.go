package task

import (
	"strings"
)

type Status string

const (
	StatusBacklog    Status = "backlog"
	StatusReady      Status = "ready"
	StatusInProgress Status = "in_progress"
	StatusReview     Status = "review"
	StatusDone       Status = "done"
)

type statusInfo struct {
	label string
	emoji string
	pane  Status
}

var statuses = map[Status]statusInfo{
	StatusBacklog:    {label: "Backlog", emoji: "📥", pane: StatusBacklog},
	StatusReady:      {label: "Ready", emoji: "📋", pane: StatusReady},
	StatusInProgress: {label: "In Progress", emoji: "⚙️", pane: StatusInProgress},
	StatusReview:     {label: "Review", emoji: "👀", pane: StatusReview},
	StatusDone:       {label: "Done", emoji: "✅", pane: StatusDone},
}

func normalizeStatusKey(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}

func ParseStatus(status string) (Status, bool) {
	normalized := normalizeStatusKey(status)
	switch normalized {
	case "", "backlog":
		return StatusBacklog, true
	case "ready", "todo", "to_do", "open":
		return StatusReady, true
	case "in_progress", "inprocess", "in_process", "inprogress":
		return StatusInProgress, true
	case "review", "in_review", "inreview":
		return StatusReview, true
	case "done", "closed", "completed":
		return StatusDone, true
	default:
		return StatusBacklog, false
	}
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
	if _, ok := statuses[status]; ok {
		return string(status)
	}
	return string(StatusBacklog)
}

func StatusPane(status Status) Status {
	if info, ok := statuses[status]; ok && info.pane != "" {
		return info.pane
	}
	return StatusBacklog
}

func StatusEmoji(status Status) string {
	if info, ok := statuses[status]; ok {
		return info.emoji
	}
	return ""
}

func StatusLabel(status Status) string {
	if info, ok := statuses[status]; ok {
		return info.label
	}
	// fall back to the raw string if unknown
	return string(status)
}

func StatusDisplay(status Status) string {
	label := StatusLabel(status)
	emoji := StatusEmoji(status)
	if emoji == "" {
		return label
	}
	return label + " " + emoji
}
