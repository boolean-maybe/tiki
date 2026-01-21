package task

import (
	"strings"
)

// Type represents the type of work item
type Type string

const (
	TypeStory Type = "story"
	TypeBug   Type = "bug"
	TypeSpike Type = "spike"
	TypeEpic  Type = "epic"
)

type typeInfo struct {
	label string
	emoji string
}

var types = map[string]typeInfo{
	"story":   {label: "Story", emoji: "🌀"},
	"bug":     {label: "Bug", emoji: "💥"},
	"spike":   {label: "Spike", emoji: "🔍"},
	"epic":    {label: "Epic", emoji: "🗂️"},
	"feature": {label: "Story", emoji: "🌀"},
	"task":    {label: "Story", emoji: "🌀"},
}

// normalizeType standardizes a raw type string.
func normalizeType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func ParseType(t string) (Type, bool) {
	normalized := normalizeType(t)
	switch normalized {
	case "bug":
		return TypeBug, true
	case "spike":
		return TypeSpike, true
	case "epic":
		return TypeEpic, true
	case "story", "feature", "task":
		return TypeStory, true
	default:
		return TypeStory, false
	}
}

// NormalizeType standardizes a raw type string into a Type.
func NormalizeType(t string) Type {
	normalized, _ := ParseType(t)
	return normalized
}

// TypeLabel returns a human-readable label for a task type.
func TypeLabel(taskType Type) string {
	// Direct lookup using Type constant
	if info, ok := types[string(taskType)]; ok {
		return info.label
	}
	// Fallback to the raw string if unknown
	return string(taskType)
}

// TypeEmoji returns the emoji for a task type.
func TypeEmoji(taskType Type) string {
	// Direct lookup using Type constant
	if info, ok := types[string(taskType)]; ok {
		return info.emoji
	}
	return ""
}

// TypeDisplay returns a formatted display string with label and emoji.
func TypeDisplay(taskType Type) string {
	label := TypeLabel(taskType)
	emoji := TypeEmoji(taskType)
	if emoji == "" {
		return label
	}
	return label + " " + emoji
}
