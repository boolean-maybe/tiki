package task

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/config"
)

// TitleValidator validates task title
type TitleValidator struct{}

func (v *TitleValidator) ValidateField(task *Task) *ValidationError {
	title := strings.TrimSpace(task.Title)

	if title == "" {
		return &ValidationError{
			Field:   "title",
			Value:   task.Title,
			Code:    ErrCodeRequired,
			Message: "title is required",
		}
	}

	// Optional: max length check (reasonable limit for UI display)
	const maxTitleLength = 200
	if len(title) > maxTitleLength {
		return &ValidationError{
			Field:   "title",
			Value:   task.Title,
			Code:    ErrCodeTooLong,
			Message: fmt.Sprintf("title exceeds maximum length of %d characters", maxTitleLength),
		}
	}

	return nil
}

// StatusValidator validates task status enum
type StatusValidator struct{}

func (v *StatusValidator) ValidateField(task *Task) *ValidationError {
	if config.GetStatusRegistry().IsValid(string(task.Status)) {
		return nil
	}

	return &ValidationError{
		Field:   "status",
		Value:   task.Status,
		Code:    ErrCodeInvalidEnum,
		Message: fmt.Sprintf("invalid status value: %s", task.Status),
	}
}

// TypeValidator validates task type enum
type TypeValidator struct{}

func (v *TypeValidator) ValidateField(task *Task) *ValidationError {
	validTypes := []Type{
		TypeStory,
		TypeBug,
		TypeSpike,
		TypeEpic,
	}

	if slices.Contains(validTypes, task.Type) {
		return nil // Valid
	}

	return &ValidationError{
		Field:   "type",
		Value:   task.Type,
		Code:    ErrCodeInvalidEnum,
		Message: fmt.Sprintf("invalid type value: %s", task.Type),
	}
}

// Priority validation constants
const (
	MinPriority     = 1
	MaxPriority     = 5
	DefaultPriority = 3 // Medium
)

func IsValidPriority(priority int) bool {
	return priority >= MinPriority && priority <= MaxPriority
}

func IsValidPoints(points int) bool {
	if points == 0 {
		return true
	}
	if points < 0 {
		return false
	}
	return points <= config.GetMaxPoints()
}

// PriorityValidator validates priority range (1-5)
type PriorityValidator struct{}

func (v *PriorityValidator) ValidateField(task *Task) *ValidationError {
	if task.Priority < MinPriority || task.Priority > MaxPriority {
		return &ValidationError{
			Field:   "priority",
			Value:   task.Priority,
			Code:    ErrCodeOutOfRange,
			Message: fmt.Sprintf("priority must be between %d and %d", MinPriority, MaxPriority),
		}
	}

	return nil
}

// PointsValidator validates story points range (1-maxPoints from config)
type PointsValidator struct{}

func (v *PointsValidator) ValidateField(task *Task) *ValidationError {
	const minPoints = 1
	maxPoints := config.GetMaxPoints()

	// Points of 0 are valid (means not estimated yet)
	if task.Points == 0 {
		return nil
	}

	if task.Points < minPoints || task.Points > maxPoints {
		return &ValidationError{
			Field:   "points",
			Value:   task.Points,
			Code:    ErrCodeOutOfRange,
			Message: fmt.Sprintf("story points must be between %d and %d", minPoints, maxPoints),
		}
	}

	return nil
}

// DependsOnValidator validates dependsOn tiki ID format
type DependsOnValidator struct{}

func (v *DependsOnValidator) ValidateField(task *Task) *ValidationError {
	for _, dep := range task.DependsOn {
		if !isValidTikiIDFormat(dep) {
			return &ValidationError{
				Field:   "dependsOn",
				Value:   dep,
				Code:    ErrCodeInvalidFormat,
				Message: fmt.Sprintf("invalid tiki ID format: %s (expected TIKI-XXXXXX)", dep),
			}
		}
	}
	return nil
}

// isValidTikiIDFormat checks if a string matches the TIKI-XXXXXX format
// where X is an uppercase alphanumeric character.
func isValidTikiIDFormat(id string) bool {
	if len(id) != 11 || id[:5] != "TIKI-" {
		return false
	}
	for _, c := range id[5:] {
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// DueValidator validates due date is normalized to midnight UTC
type DueValidator struct{}

func (v *DueValidator) ValidateField(task *Task) *ValidationError {
	// Zero time (no due date) is valid
	if task.Due.IsZero() {
		return nil
	}

	// Validate date is normalized to midnight UTC
	if task.Due.Hour() != 0 || task.Due.Minute() != 0 || task.Due.Second() != 0 || task.Due.Nanosecond() != 0 || task.Due.Location() != time.UTC {
		return &ValidationError{
			Field:   "due",
			Value:   task.Due,
			Code:    ErrCodeInvalidFormat,
			Message: "due date must be normalized to midnight UTC (use date-only format)",
		}
	}

	return nil
}

// RecurrenceValidator validates recurrence pattern
type RecurrenceValidator struct{}

func (v *RecurrenceValidator) ValidateField(task *Task) *ValidationError {
	if task.Recurrence == RecurrenceNone {
		return nil
	}
	if !IsValidRecurrence(task.Recurrence) {
		return &ValidationError{
			Field:   "recurrence",
			Value:   task.Recurrence,
			Code:    ErrCodeInvalidFormat,
			Message: fmt.Sprintf("invalid recurrence pattern: %s", task.Recurrence),
		}
	}
	return nil
}

// AssigneeValidator - no validation needed (any string is valid)
// DescriptionValidator - no validation needed (any string is valid)
