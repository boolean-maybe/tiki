package task

import (
	"fmt"
	"slices"
	"strings"

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
	validStatuses := []Status{
		StatusBacklog,
		StatusReady,
		StatusInProgress,
		StatusReview,
		StatusDone,
	}

	if slices.Contains(validStatuses, task.Status) {
		return nil // Valid
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

// AssigneeValidator - no validation needed (any string is valid)
// DescriptionValidator - no validation needed (any string is valid)
