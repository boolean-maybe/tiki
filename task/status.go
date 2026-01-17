package task

import (
	"strings"
)

type Status string

const (
	StatusBacklog    Status = "backlog"
	StatusTodo       Status = "todo"
	StatusReady      Status = "ready"
	StatusInProgress Status = "in_progress"
	StatusWaiting    Status = "waiting"
	StatusBlocked    Status = "blocked"
	StatusReview     Status = "review"
	StatusDone       Status = "done"
)

type statusInfo struct {
	label  string
	emoji  string
	column Status
}

var statuses = map[Status]statusInfo{
	StatusBacklog:    {label: "Backlog", emoji: "📥", column: StatusBacklog},
	StatusTodo:       {label: "To Do", emoji: "📋", column: StatusTodo},
	StatusReady:      {label: "Ready", emoji: "📋", column: StatusTodo},
	StatusInProgress: {label: "In Progress", emoji: "⚙️", column: StatusInProgress},
	StatusWaiting:    {label: "Waiting", emoji: "⏳", column: StatusReview},
	StatusBlocked:    {label: "Blocked", emoji: "⛔", column: StatusInProgress},
	StatusReview:     {label: "Review", emoji: "👀", column: StatusReview},
	StatusDone:       {label: "Done", emoji: "✅", column: StatusDone},
}

// NormalizeStatus standardizes a raw status string into a Status.
func NormalizeStatus(status string) Status {
	normalized := strings.ToLower(strings.TrimSpace(status))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")

	switch normalized {
	case "", "backlog":
		return StatusBacklog
	case "todo", "to_do":
		return StatusTodo
	case "ready":
		return StatusReady
	case "open":
		return StatusTodo
	case "in_progress", "inprocess", "in_process", "inprogress":
		return StatusInProgress
	case "waiting", "on_hold", "hold":
		return StatusWaiting
	case "blocked", "blocker":
		return StatusBlocked
	case "review", "in_review", "inreview":
		return StatusReview
	case "done", "closed", "completed":
		return StatusDone
	default:
		return StatusBacklog
	}
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

func StatusColumn(status Status) Status {
	if info, ok := statuses[status]; ok && info.column != "" {
		return info.column
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
