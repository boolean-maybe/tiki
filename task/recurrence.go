package task

import (
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"
)

// Recurrence represents a task recurrence pattern as a cron expression.
// Empty string means no recurrence.
type Recurrence string

const (
	RecurrenceNone    Recurrence = ""
	RecurrenceDaily   Recurrence = "0 0 * * *"
	RecurrenceMonthly Recurrence = "0 0 1 * *"
)

type recurrenceInfo struct {
	cron    Recurrence
	display string
}

// known recurrence patterns — order matters for AllRecurrenceDisplayValues
var knownRecurrences = []recurrenceInfo{
	{RecurrenceNone, "None"},
	{RecurrenceDaily, "Daily"},
	{"0 0 * * MON", "Weekly on Monday"},
	{"0 0 * * TUE", "Weekly on Tuesday"},
	{"0 0 * * WED", "Weekly on Wednesday"},
	{"0 0 * * THU", "Weekly on Thursday"},
	{"0 0 * * FRI", "Weekly on Friday"},
	{"0 0 * * SAT", "Weekly on Saturday"},
	{"0 0 * * SUN", "Weekly on Sunday"},
	{RecurrenceMonthly, "Monthly"},
}

// built at init from knownRecurrences
var (
	cronToDisplay map[Recurrence]string
	displayToCron map[string]Recurrence
	validCronSet  map[Recurrence]bool
)

func init() {
	cronToDisplay = make(map[Recurrence]string, len(knownRecurrences))
	displayToCron = make(map[string]Recurrence, len(knownRecurrences))
	validCronSet = make(map[Recurrence]bool, len(knownRecurrences))
	for _, r := range knownRecurrences {
		cronToDisplay[r.cron] = r.display
		displayToCron[strings.ToLower(r.display)] = r.cron
		validCronSet[r.cron] = true
	}
}

// RecurrenceValue is a custom type for recurrence that provides lenient YAML unmarshaling.
type RecurrenceValue struct {
	Value Recurrence
}

// UnmarshalYAML implements custom unmarshaling for recurrence with lenient error handling.
func (r *RecurrenceValue) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err == nil {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			*r = RecurrenceValue{}
			return nil
		}
		if parsed, ok := ParseRecurrence(trimmed); ok {
			*r = RecurrenceValue{Value: parsed}
			return nil
		}
		slog.Warn("invalid recurrence format, defaulting to empty",
			"value", s,
			"line", value.Line,
			"column", value.Column)
		*r = RecurrenceValue{}
		return nil
	}

	slog.Warn("invalid recurrence field type, defaulting to empty",
		"received_type", value.Kind,
		"line", value.Line,
		"column", value.Column)
	*r = RecurrenceValue{}
	return nil
}

// MarshalYAML implements YAML marshaling for RecurrenceValue.
func (r RecurrenceValue) MarshalYAML() (any, error) {
	return string(r.Value), nil
}

// IsZero reports whether the recurrence is empty (needed for omitempty).
func (r RecurrenceValue) IsZero() bool {
	return r.Value == RecurrenceNone
}

// ToRecurrence converts RecurrenceValue to Recurrence.
func (r RecurrenceValue) ToRecurrence() Recurrence {
	return r.Value
}

// ParseRecurrence validates a cron string against known patterns.
func ParseRecurrence(s string) (Recurrence, bool) {
	normalized := Recurrence(strings.ToLower(strings.TrimSpace(s)))
	// accept both lowercase and original casing
	for _, r := range knownRecurrences {
		if Recurrence(strings.ToLower(string(r.cron))) == normalized {
			return r.cron, true
		}
	}
	return RecurrenceNone, false
}

// RecurrenceDisplay converts a cron expression to English display.
func RecurrenceDisplay(r Recurrence) string {
	if d, ok := cronToDisplay[r]; ok {
		return d
	}
	return "None"
}

// RecurrenceFromDisplay converts an English display string to a cron expression.
func RecurrenceFromDisplay(display string) Recurrence {
	if c, ok := displayToCron[strings.ToLower(strings.TrimSpace(display))]; ok {
		return c
	}
	return RecurrenceNone
}

// AllRecurrenceDisplayValues returns the ordered list of display values for UI selection.
func AllRecurrenceDisplayValues() []string {
	result := make([]string, len(knownRecurrences))
	for i, r := range knownRecurrences {
		result[i] = r.display
	}
	return result
}

// IsValidRecurrence returns true if the recurrence is empty or matches a known pattern.
func IsValidRecurrence(r Recurrence) bool {
	return validCronSet[r]
}
