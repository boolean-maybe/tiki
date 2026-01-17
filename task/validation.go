package task

// Validator is the main validation interface
type Validator interface {
	Validate(task *Task) ValidationErrors
}

// FieldValidator validates a single field
type FieldValidator interface {
	ValidateField(task *Task) *ValidationError
}

// TaskValidator orchestrates all field validators
type TaskValidator struct {
	validators []FieldValidator
}

// NewTaskValidator creates a validator with standard rules
func NewTaskValidator() *TaskValidator {
	return &TaskValidator{
		validators: []FieldValidator{
			&TitleValidator{},
			&StatusValidator{},
			&TypeValidator{},
			&PriorityValidator{},
			&PointsValidator{},
			// Assignee and Description have no constraints (always valid)
		},
	}
}

// Validate runs all validators and accumulates errors
func (tv *TaskValidator) Validate(task *Task) ValidationErrors {
	var errors ValidationErrors

	for _, validator := range tv.validators {
		if err := validator.ValidateField(task); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}

// ValidateField validates a single field by name
func (tv *TaskValidator) ValidateField(task *Task, fieldName string) *ValidationError {
	for _, validator := range tv.validators {
		if err := validator.ValidateField(task); err != nil && err.Field == fieldName {
			return err
		}
	}
	return nil
}

// QuickValidate is a convenience function for quick validation
func QuickValidate(task *Task) ValidationErrors {
	return NewTaskValidator().Validate(task)
}

// IsValid returns true if the task passes all validation rules
func IsValid(task *Task) bool {
	return !QuickValidate(task).HasErrors()
}
