package task

import (
	"strings"
	"testing"
)

func TestTitleValidator(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
		errCode ErrorCode
	}{
		{
			name:    "valid title",
			task:    &Task{Title: "Valid Task"},
			wantErr: false,
		},
		{
			name:    "empty title",
			task:    &Task{Title: ""},
			wantErr: true,
			errCode: ErrCodeRequired,
		},
		{
			name:    "whitespace title",
			task:    &Task{Title: "   "},
			wantErr: true,
			errCode: ErrCodeRequired,
		},
		{
			name:    "very long title",
			task:    &Task{Title: strings.Repeat("a", 201)},
			wantErr: true,
			errCode: ErrCodeTooLong,
		},
		{
			name:    "max length title",
			task:    &Task{Title: strings.Repeat("a", 200)},
			wantErr: false,
		},
	}

	validator := &TitleValidator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateField(tt.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("expected error: %v, got: %v", tt.wantErr, err)
			}
			if err != nil && err.Code != tt.errCode {
				t.Errorf("expected error code: %v, got: %v", tt.errCode, err.Code)
			}
		})
	}
}

func TestStatusValidator(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{"valid backlog", &Task{Status: StatusBacklog}, false},
		{"valid todo", &Task{Status: StatusReady}, false},
		{"valid in_progress", &Task{Status: StatusInProgress}, false},
		{"valid review", &Task{Status: StatusReview}, false},
		{"valid done", &Task{Status: StatusDone}, false},
		{"invalid status", &Task{Status: "invalid"}, true},
		{"empty status", &Task{Status: ""}, true},
	}

	validator := &StatusValidator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateField(tt.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("expected error: %v, got: %v", tt.wantErr, err)
			}
			if err != nil && err.Code != ErrCodeInvalidEnum {
				t.Errorf("expected error code: %v, got: %v", ErrCodeInvalidEnum, err.Code)
			}
		})
	}
}

func TestTypeValidator(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{"valid story", &Task{Type: TypeStory}, false},
		{"valid bug", &Task{Type: TypeBug}, false},
		{"valid spike", &Task{Type: TypeSpike}, false},
		{"valid epic", &Task{Type: TypeEpic}, false},
		{"invalid type", &Task{Type: "invalid"}, true},
		{"empty type", &Task{Type: ""}, true},
	}

	validator := &TypeValidator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateField(tt.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("expected error: %v, got: %v", tt.wantErr, err)
			}
			if err != nil && err.Code != ErrCodeInvalidEnum {
				t.Errorf("expected error code: %v, got: %v", ErrCodeInvalidEnum, err.Code)
			}
		})
	}
}

func TestPriorityValidator(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{"valid priority 1", &Task{Priority: 1}, false},
		{"valid priority 3", &Task{Priority: 3}, false},
		{"valid priority 5", &Task{Priority: 5}, false},
		{"invalid priority 0", &Task{Priority: 0}, true},
		{"invalid priority 6", &Task{Priority: 6}, true},
		{"invalid priority -1", &Task{Priority: -1}, true},
		{"invalid priority 10", &Task{Priority: 10}, true},
	}

	validator := &PriorityValidator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateField(tt.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("expected error: %v, got: %v", tt.wantErr, err)
			}
			if err != nil && err.Code != ErrCodeOutOfRange {
				t.Errorf("expected error code: %v, got: %v", ErrCodeOutOfRange, err.Code)
			}
		})
	}
}

func TestPointsValidator(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{"valid points 0 (unestimated)", &Task{Points: 0}, false},
		{"valid points 1", &Task{Points: 1}, false},
		{"valid points 5", &Task{Points: 5}, false},
		{"valid points 10", &Task{Points: 10}, false},
		{"invalid points -1", &Task{Points: -1}, true},
		{"invalid points 11", &Task{Points: 11}, true},
		{"invalid points 100", &Task{Points: 100}, true},
	}

	validator := &PointsValidator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateField(tt.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("expected error: %v, got: %v", tt.wantErr, err)
			}
			if err != nil && err.Code != ErrCodeOutOfRange {
				t.Errorf("expected error code: %v, got: %v", ErrCodeOutOfRange, err.Code)
			}
		})
	}
}

func TestTaskValidator_MultipleErrors(t *testing.T) {
	// Task with multiple validation errors
	task := &Task{
		Title:    "",        // Invalid: empty
		Status:   "invalid", // Invalid: not a valid enum
		Type:     "bad",     // Invalid: not a valid enum
		Priority: 10,        // Invalid: out of range
		Points:   -5,        // Invalid: negative
	}

	errors := task.Validate()

	if !errors.HasErrors() {
		t.Fatal("expected validation errors, got none")
	}

	expectedErrors := 5
	if len(errors) != expectedErrors {
		t.Errorf("expected %d errors, got %d", expectedErrors, len(errors))
	}

	// Check that each field has an error
	if !errors.HasField("title") {
		t.Error("expected title error")
	}
	if !errors.HasField("status") {
		t.Error("expected status error")
	}
	if !errors.HasField("type") {
		t.Error("expected type error")
	}
	if !errors.HasField("priority") {
		t.Error("expected priority error")
	}
	if !errors.HasField("points") {
		t.Error("expected points error")
	}
}

func TestTaskValidator_ValidTask(t *testing.T) {
	task := &Task{
		Title:    "Valid Task",
		Status:   StatusReady,
		Type:     TypeStory,
		Priority: 3,
		Points:   5,
	}

	errors := task.Validate()

	if errors.HasErrors() {
		t.Errorf("expected no errors, got: %v", errors)
	}

	if !task.IsValid() {
		t.Error("expected task to be valid")
	}
}

func TestTaskValidator_SingleFieldValidation(t *testing.T) {
	task := &Task{
		Priority: 10, // Invalid
	}

	err := task.ValidateField("priority")
	if err == nil {
		t.Fatal("expected validation error for priority field")
	}

	if err.Field != "priority" {
		t.Errorf("expected field 'priority', got '%s'", err.Field)
	}

	if err.Code != ErrCodeOutOfRange {
		t.Errorf("expected error code %v, got %v", ErrCodeOutOfRange, err.Code)
	}
}

func TestValidationErrors_ByField(t *testing.T) {
	errors := ValidationErrors{
		{Field: "title", Message: "title error"},
		{Field: "priority", Message: "priority error 1"},
		{Field: "priority", Message: "priority error 2"},
	}

	titleErrors := errors.ByField("title")
	if len(titleErrors) != 1 {
		t.Errorf("expected 1 title error, got %d", len(titleErrors))
	}

	priorityErrors := errors.ByField("priority")
	if len(priorityErrors) != 2 {
		t.Errorf("expected 2 priority errors, got %d", len(priorityErrors))
	}

	nonExistentErrors := errors.ByField("nonexistent")
	if len(nonExistentErrors) != 0 {
		t.Errorf("expected 0 errors for nonexistent field, got %d", len(nonExistentErrors))
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "title",
		Value:   "",
		Code:    ErrCodeRequired,
		Message: "title is required",
	}

	expected := "title: title is required"
	if err.Error() != expected {
		t.Errorf("expected error string '%s', got '%s'", expected, err.Error())
	}
}

func TestValidationErrors_Error(t *testing.T) {
	errors := ValidationErrors{
		{Field: "title", Message: "title is required"},
		{Field: "priority", Message: "priority must be between 1 and 5"},
	}

	errStr := errors.Error()
	if !strings.Contains(errStr, "title is required") {
		t.Error("error string should contain title message")
	}
	if !strings.Contains(errStr, "priority must be between 1 and 5") {
		t.Error("error string should contain priority message")
	}
}
