package task

import (
	"fmt"
	"strings"
)

// ValidationError represents a field-level validation failure with rich context
type ValidationError struct {
	Field   string    // Field name (e.g., "title", "priority")
	Value   any       // The invalid value
	Code    ErrorCode // Machine-readable error code
	Message string    // Human-readable message
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ErrorCode represents specific validation failure types
type ErrorCode string

const (
	ErrCodeRequired      ErrorCode = "required"
	ErrCodeTooLong       ErrorCode = "too_long"
	ErrCodeTooShort      ErrorCode = "too_short"
	ErrCodeOutOfRange    ErrorCode = "out_of_range"
	ErrCodeInvalidEnum   ErrorCode = "invalid_enum"
	ErrCodeInvalidFormat ErrorCode = "invalid_format"
)

// ValidationErrors is a collection of field-level errors
type ValidationErrors []*ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return "no validation errors"
	}
	var msgs []string
	for _, err := range ve {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// HasErrors returns true if there are any validation errors
func (ve ValidationErrors) HasErrors() bool {
	return len(ve) > 0
}

// ByField returns errors for a specific field
func (ve ValidationErrors) ByField(field string) []*ValidationError {
	var result []*ValidationError
	for _, err := range ve {
		if err.Field == field {
			result = append(result, err)
		}
	}
	return result
}

// HasField returns true if the field has any errors
func (ve ValidationErrors) HasField(field string) bool {
	return len(ve.ByField(field)) > 0
}
