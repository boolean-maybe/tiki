package task

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
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
	{RecurrenceMonthly, "Monthly on the 1st"},
}

// RecurrenceFrequency represents a high-level recurrence category for the UI editor.
type RecurrenceFrequency string

const (
	FrequencyNone    RecurrenceFrequency = "None"
	FrequencyDaily   RecurrenceFrequency = "Daily"
	FrequencyWeekly  RecurrenceFrequency = "Weekly"
	FrequencyMonthly RecurrenceFrequency = "Monthly"
)

// AllFrequencies returns the ordered list of frequencies for the UI.
func AllFrequencies() []string {
	return []string{
		string(FrequencyNone),
		string(FrequencyDaily),
		string(FrequencyWeekly),
		string(FrequencyMonthly),
	}
}

// monthlyPattern matches cron expressions like "0 0 15 * *"
var monthlyPattern = regexp.MustCompile(`^0 0 (\d{1,2}) \* \*$`)

// OrdinalSuffix returns the ordinal suffix for a number (st, nd, rd, th).
func OrdinalSuffix(n int) string {
	if n >= 11 && n <= 13 {
		return "th"
	}
	switch n % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
}

// MonthlyRecurrence creates a monthly cron expression for the given day (1-31).
func MonthlyRecurrence(day int) Recurrence {
	if day < 1 || day > 31 {
		return RecurrenceNone
	}
	return Recurrence(fmt.Sprintf("0 0 %d * *", day))
}

// IsMonthlyRecurrence checks if a recurrence is a monthly pattern.
// Returns the day of month and true if it matches "0 0 N * *".
func IsMonthlyRecurrence(r Recurrence) (int, bool) {
	m := monthlyPattern.FindStringSubmatch(string(r))
	if m == nil {
		return 0, false
	}
	day, err := strconv.Atoi(m[1])
	if err != nil || day < 1 || day > 31 {
		return 0, false
	}
	return day, true
}

// MonthlyDisplay returns a human-readable string like "Monthly on the 15th".
func MonthlyDisplay(day int) string {
	return fmt.Sprintf("Monthly on the %d%s", day, OrdinalSuffix(day))
}

// FrequencyFromRecurrence extracts the high-level frequency from a cron expression.
func FrequencyFromRecurrence(r Recurrence) RecurrenceFrequency {
	if r == RecurrenceNone {
		return FrequencyNone
	}
	if r == RecurrenceDaily {
		return FrequencyDaily
	}
	if _, ok := WeekdayFromRecurrence(r); ok {
		return FrequencyWeekly
	}
	if _, ok := IsMonthlyRecurrence(r); ok {
		return FrequencyMonthly
	}
	return FrequencyNone
}

// weekdayCronToName maps cron weekday abbreviations to full day names.
var weekdayCronToName = map[string]string{
	"MON": "Monday", "TUE": "Tuesday", "WED": "Wednesday",
	"THU": "Thursday", "FRI": "Friday", "SAT": "Saturday", "SUN": "Sunday",
}

// weekdayNameToCron maps full day names back to cron abbreviations.
var weekdayNameToCron = map[string]string{
	"Monday": "MON", "Tuesday": "TUE", "Wednesday": "WED",
	"Thursday": "THU", "Friday": "FRI", "Saturday": "SAT", "Sunday": "SUN",
}

// AllWeekdays returns weekday names in order for the UI.
func AllWeekdays() []string {
	return []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
}

// weeklyPattern matches cron expressions like "0 0 * * MON"
var weeklyPattern = regexp.MustCompile(`^0 0 \* \* ([A-Z]{3})$`)

// WeekdayFromRecurrence extracts the weekday name from a weekly cron expression.
func WeekdayFromRecurrence(r Recurrence) (string, bool) {
	m := weeklyPattern.FindStringSubmatch(string(r))
	if m == nil {
		return "", false
	}
	name, ok := weekdayCronToName[m[1]]
	return name, ok
}

// DayOfMonthFromRecurrence extracts the day (1-31) from a monthly cron expression.
func DayOfMonthFromRecurrence(r Recurrence) (int, bool) {
	return IsMonthlyRecurrence(r)
}

// WeeklyRecurrence creates a weekly cron expression for the given day name.
func WeeklyRecurrence(dayName string) Recurrence {
	abbrev, ok := weekdayNameToCron[dayName]
	if !ok {
		return RecurrenceNone
	}
	return Recurrence("0 0 * * " + abbrev)
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

// ParseRecurrence validates a cron string against known patterns or monthly pattern.
func ParseRecurrence(s string) (Recurrence, bool) {
	normalized := Recurrence(strings.ToLower(strings.TrimSpace(s)))
	// accept both lowercase and original casing
	for _, r := range knownRecurrences {
		if Recurrence(strings.ToLower(string(r.cron))) == normalized {
			return r.cron, true
		}
	}
	// try monthly pattern (e.g. "0 0 15 * *")
	candidate := Recurrence(strings.TrimSpace(s))
	if day, ok := IsMonthlyRecurrence(candidate); ok {
		return MonthlyRecurrence(day), true
	}
	return RecurrenceNone, false
}

// RecurrenceDisplay converts a cron expression to English display.
func RecurrenceDisplay(r Recurrence) string {
	if d, ok := cronToDisplay[r]; ok {
		return d
	}
	if day, ok := IsMonthlyRecurrence(r); ok {
		return MonthlyDisplay(day)
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

// IsValidRecurrence returns true if the recurrence is empty or matches a known or monthly pattern.
func IsValidRecurrence(r Recurrence) bool {
	if validCronSet[r] {
		return true
	}
	_, ok := IsMonthlyRecurrence(r)
	return ok
}
