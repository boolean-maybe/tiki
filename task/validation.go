package task

import (
	"fmt"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/document"
)

// Priority validation constants
const (
	MinPriority     = 1
	MaxPriority     = 5
	DefaultPriority = 3 // Medium
)

// ValidateTitle returns an error message if the task title is invalid.
func ValidateTitle(t *Task) string {
	title := strings.TrimSpace(t.Title)
	if title == "" {
		return "title is required"
	}
	const maxTitleLength = 200
	if len(title) > maxTitleLength {
		return fmt.Sprintf("title exceeds maximum length of %d characters", maxTitleLength)
	}
	return ""
}

// ValidateStatus returns an error message if the task status is invalid.
// An empty status is treated as absent (Phase 1 presence-aware contract) and
// passes validation — only set values are checked against the registry.
func ValidateStatus(t *Task) string {
	if t.Status == "" {
		return ""
	}
	if config.GetStatusRegistry().IsValid(string(t.Status)) {
		return ""
	}
	return fmt.Sprintf("invalid status value: %s", t.Status)
}

// ValidateType returns an error message if the task type is invalid. An
// empty type is treated as absent and passes validation; only set values
// are checked against the registry.
func ValidateType(t *Task) string {
	if t.Type == "" {
		return ""
	}
	if requireTypeRegistry().IsValid(t.Type) {
		return ""
	}
	return fmt.Sprintf("invalid type value: %s", t.Type)
}

// ValidatePriority returns an error message if the task priority is out of range.
// A zero priority is treated as absent (Phase 1 presence-aware contract) and
// passes validation; only set values are range-checked.
func ValidatePriority(t *Task) string {
	if t.Priority == 0 {
		return ""
	}
	if t.Priority < MinPriority || t.Priority > MaxPriority {
		return fmt.Sprintf("priority must be between %d and %d", MinPriority, MaxPriority)
	}
	return ""
}

// ValidatePoints returns an error message if story points are out of range.
func ValidatePoints(t *Task) string {
	if t.Points == 0 {
		return ""
	}
	const minPoints = 1
	maxPoints := config.GetMaxPoints()
	if t.Points < minPoints || t.Points > maxPoints {
		return fmt.Sprintf("story points must be between %d and %d", minPoints, maxPoints)
	}
	return ""
}

// ValidateDependsOn returns an error message if any dependency ID is malformed.
// Only bare document IDs are accepted — legacy TIKI-XXXXXX values are not
// valid references. Run `tiki repair ids --fix` to migrate legacy IDs.
func ValidateDependsOn(t *Task) string {
	for _, dep := range t.DependsOn {
		if !document.IsValidID(dep) {
			return fmt.Sprintf("invalid document ID format: %s (expected %d uppercase alphanumeric chars)", dep, document.IDLength)
		}
	}
	return ""
}

// ValidateDue returns an error message if the due date is not normalized to midnight UTC.
func ValidateDue(t *Task) string {
	if t.Due.IsZero() {
		return ""
	}
	if t.Due.Hour() != 0 || t.Due.Minute() != 0 || t.Due.Second() != 0 || t.Due.Nanosecond() != 0 || t.Due.Location() != time.UTC {
		return "due date must be normalized to midnight UTC (use date-only format)"
	}
	return ""
}

// ValidateRecurrence returns an error message if the recurrence pattern is invalid.
func ValidateRecurrence(t *Task) string {
	if t.Recurrence == RecurrenceNone {
		return ""
	}
	if !IsValidRecurrence(t.Recurrence) {
		return fmt.Sprintf("invalid recurrence pattern: %s", t.Recurrence)
	}
	return ""
}

// IsValidPriority checks if a priority value is within the valid range.
func IsValidPriority(priority int) bool {
	return priority >= MinPriority && priority <= MaxPriority
}

// IsValidPoints checks if a points value is within the valid range.
func IsValidPoints(points int) bool {
	if points == 0 {
		return true
	}
	if points < 0 {
		return false
	}
	return points <= config.GetMaxPoints()
}

// AllValidators returns the complete list of field validation functions.
// Each returns an error message (empty string = valid). Workflow-field
// validators treat a zero value as absent and return success — a plain
// document (id+title only) or a sparse workflow document (a subset of
// workflow fields set) therefore passes validation without any IsWorkflow-
// based gating at the caller. Title is always required.
func AllValidators() []func(*Task) string {
	return []func(*Task) string{
		ValidateTitle,
		ValidateStatus,
		ValidateType,
		ValidatePriority,
		ValidatePoints,
		ValidateDependsOn,
		ValidateDue,
		ValidateRecurrence,
	}
}
