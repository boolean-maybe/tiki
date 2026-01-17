package task

import (
	"log/slog"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Priority scale: lower = higher priority
// 1 = highest, 5 = lowest
const (
	PriorityHigh       = 1
	PriorityMediumHigh = 2
	PriorityMedium     = 3
	PriorityMediumLow  = 4
	PriorityLow        = 5
)

type priorityInfo struct {
	label string
	value int
}

var priorities = map[string]priorityInfo{
	"high":        {label: "High", value: PriorityHigh},
	"medium-high": {label: "Medium High", value: PriorityMediumHigh},
	"high-medium": {label: "Medium High", value: PriorityMediumHigh},
	"medium":      {label: "Medium", value: PriorityMedium},
	"medium-low":  {label: "Medium Low", value: PriorityMediumLow},
	"low-medium":  {label: "Medium Low", value: PriorityMediumLow},
	"low":         {label: "Low", value: PriorityLow},
}

// PriorityValue unmarshals priority from int or string (e.g., "high", "medium-high").
// high=1, low=5. Values outside 1-5 range are clamped to boundaries.
type PriorityValue int

func (p *PriorityValue) UnmarshalYAML(value *yaml.Node) error {
	// first try integer
	var intVal int
	if err := value.Decode(&intVal); err == nil {
		// Clamp to valid range 1-5
		*p = PriorityValue(clampPriority(intVal))
		return nil
	}

	// try string
	var strVal string
	if err := value.Decode(&strVal); err != nil {
		// If it's not an int or string (e.g., boolean, object), default to medium
		slog.Warn("invalid priority field, defaulting to medium",
			"received_type", value.Kind,
			"line", value.Line,
			"column", value.Column)
		*p = PriorityValue(PriorityMedium)
		return nil
	}

	normalized := normalizePriority(strVal)
	if normalized == "" {
		// Empty or whitespace-only string, default to medium
		slog.Warn("invalid priority field, defaulting to medium",
			"received_value", strVal,
			"line", value.Line,
			"column", value.Column)
		*p = PriorityValue(PriorityMedium)
		return nil
	}

	mapped, ok := priorityWordToInt(normalized)
	if !ok {
		// attempt parsing numeric string
		if num, err := strconv.Atoi(strVal); err == nil {
			// Clamp to valid range 1-5
			*p = PriorityValue(clampPriority(num))
			return nil
		}
		// Unrecognized priority word or invalid numeric string, default to medium
		slog.Warn("invalid priority field, defaulting to medium",
			"received_value", strVal,
			"line", value.Line,
			"column", value.Column)
		*p = PriorityValue(PriorityMedium)
		return nil
	}

	*p = PriorityValue(mapped)
	return nil
}

// clampPriority ensures priority is within valid range 1-5
func clampPriority(val int) int {
	if val < PriorityHigh {
		return PriorityHigh
	}
	if val > PriorityLow {
		return PriorityLow
	}
	return val
}

func (p PriorityValue) MarshalYAML() (interface{}, error) {
	return int(p), nil
}

// normalizePriority standardizes a raw priority string.
func normalizePriority(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// priorityWordToInt converts priority words to numeric values.
func priorityWordToInt(s string) (int, bool) {
	if info, ok := priorities[s]; ok {
		return info.value, true
	}
	return 0, false
}

// PriorityLabel returns an emoji-based label for a priority value.
func PriorityLabel(priority int) string {
	switch priority {
	case PriorityHigh: // 1
		return "🔴"
	case PriorityMediumHigh: // 2
		return "🟠"
	case PriorityMedium: // 3
		return "🟡"
	case PriorityMediumLow: // 4
		return "🔵"
	case PriorityLow: // 5
		return "🟢"
	default:
		// For out-of-range values, map to boundaries
		if priority < PriorityHigh {
			return "🔴"
		}
		return "🟢"
	}
}

// NormalizePriority standardizes a raw priority string or number into an integer.
// Returns the normalized priority value (clamped to 1-5) or -1 if invalid.
func NormalizePriority(priority string) int {
	normalized := normalizePriority(priority)
	if normalized == "" {
		return -1
	}

	if mapped, ok := priorityWordToInt(normalized); ok {
		return mapped
	}

	// Try parsing as number
	if num, err := strconv.Atoi(priority); err == nil {
		return clampPriority(num)
	}

	return -1
}
