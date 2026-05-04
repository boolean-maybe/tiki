package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// RegisterFieldValidators registers standard field validators with the gate.
// Every validator runs on every create and update. Workflow-field validators
// treat an absent value as success, matching the presence-aware contract.
func RegisterFieldValidators(g *TaskMutationGate) {
	for _, fn := range AllTikiValidators() {
		wrapped := wrapTikiFieldValidator(fn)
		g.OnCreate(wrapped)
		g.OnUpdate(wrapped)
	}
}

func wrapTikiFieldValidator(fn func(*tikipkg.Tiki) string) MutationValidator {
	return func(old, new *tikipkg.Tiki, _ []*tikipkg.Tiki) *Rejection {
		// field validators only inspect the proposed tiki
		t := new
		if t == nil {
			t = old // delete case
		}
		if msg := fn(t); msg != "" {
			return &Rejection{Reason: msg}
		}
		return nil
	}
}

// AllTikiValidators returns the complete list of tiki field validation functions.
// The validators are presence-aware: absent fields pass validation so sparse
// workflow and plain docs are accepted.
func AllTikiValidators() []func(*tikipkg.Tiki) string {
	return []func(*tikipkg.Tiki) string{
		validateTikiTitle,
		validateTikiStatus,
		validateTikiType,
		validateTikiPriority,
		validateTikiPoints,
		validateTikiDependsOn,
		validateTikiDue,
		validateTikiRecurrence,
	}
}

func validateTikiTitle(tk *tikipkg.Tiki) string {
	title := strings.TrimSpace(tk.Title)
	if title == "" {
		return "title is required"
	}
	const maxTitleLength = 200
	if len(title) > maxTitleLength {
		return fmt.Sprintf("title exceeds maximum length of %d characters", maxTitleLength)
	}
	return ""
}

// validateTikiStatus returns an error message if the tiki status is invalid.
// An absent status field passes validation. A present field that cannot be
// coerced to a string (wrong stored type) is rejected.
func validateTikiStatus(tk *tikipkg.Tiki) string {
	s, ok, coerced := tk.StringField(tikipkg.FieldStatus)
	if !ok {
		return ""
	}
	if !coerced {
		return "status field has wrong type (expected string)"
	}
	if s == "" {
		return ""
	}
	if config.GetStatusRegistry().IsValid(s) {
		return ""
	}
	return fmt.Sprintf("invalid status value: %s", s)
}

// validateTikiType returns an error message if the tiki type is invalid.
// An absent type field passes validation. A present field that cannot be
// coerced to a string (wrong stored type) is rejected.
func validateTikiType(tk *tikipkg.Tiki) string {
	s, ok, coerced := tk.StringField(tikipkg.FieldType)
	if !ok {
		return ""
	}
	if !coerced {
		return "type field has wrong type (expected string)"
	}
	if s == "" {
		return ""
	}
	if config.GetTypeRegistry().IsValid(task.Type(s)) {
		return ""
	}
	return fmt.Sprintf("invalid type value: %s", s)
}

// validateTikiPriority returns an error message if the tiki priority is out of range.
// An absent or zero priority passes validation. A present field that cannot be
// coerced to an int (wrong stored type) is rejected.
func validateTikiPriority(tk *tikipkg.Tiki) string {
	n, ok, coerced := tk.IntField(tikipkg.FieldPriority)
	if !ok {
		return ""
	}
	if !coerced {
		return "priority field has wrong type (expected integer)"
	}
	if n == 0 {
		return ""
	}
	if n < task.MinPriority || n > task.MaxPriority {
		return fmt.Sprintf("priority must be between %d and %d", task.MinPriority, task.MaxPriority)
	}
	return ""
}

// validateTikiPoints returns an error message if tiki story points are out of range.
// A present field that cannot be coerced to an int (wrong stored type) is rejected.
func validateTikiPoints(tk *tikipkg.Tiki) string {
	n, ok, coerced := tk.IntField(tikipkg.FieldPoints)
	if !ok {
		return ""
	}
	if !coerced {
		return "points field has wrong type (expected integer)"
	}
	if n == 0 {
		return ""
	}
	const minPoints = 1
	maxPoints := config.GetMaxPoints()
	if n < minPoints || n > maxPoints {
		return fmt.Sprintf("story points must be between %d and %d", minPoints, maxPoints)
	}
	return ""
}

// validateTikiDependsOn returns an error message if any dependency ID is malformed.
func validateTikiDependsOn(tk *tikipkg.Tiki) string {
	deps, ok, coerced := tk.StringSliceField(tikipkg.FieldDependsOn)
	if !ok {
		return ""
	}
	if !coerced {
		return "dependsOn field has wrong type (expected list of strings)"
	}
	for _, dep := range deps {
		if !document.IsValidID(dep) {
			return fmt.Sprintf("invalid document ID format: %s (expected %d uppercase alphanumeric chars)", dep, document.IDLength)
		}
	}
	return ""
}

// validateTikiDue returns an error message if the tiki due date is not normalized to midnight UTC.
// A present field that cannot be coerced to a time.Time (wrong stored type) is rejected.
func validateTikiDue(tk *tikipkg.Tiki) string {
	tv, ok, coerced := tk.TimeField(tikipkg.FieldDue)
	if !ok {
		return ""
	}
	if !coerced {
		return "due field has wrong type (expected date)"
	}
	if tv.IsZero() {
		return ""
	}
	if tv.Hour() != 0 || tv.Minute() != 0 || tv.Second() != 0 || tv.Nanosecond() != 0 || tv.Location() != time.UTC {
		return "due date must be normalized to midnight UTC (use date-only format)"
	}
	return ""
}

// validateTikiRecurrence returns an error message if the tiki recurrence pattern is invalid.
// A present field that cannot be coerced to a string (wrong stored type) is rejected.
func validateTikiRecurrence(tk *tikipkg.Tiki) string {
	s, ok, coerced := tk.StringField(tikipkg.FieldRecurrence)
	if !ok {
		return ""
	}
	if !coerced {
		return "recurrence field has wrong type (expected string)"
	}
	if s == "" {
		return ""
	}
	r := task.Recurrence(s)
	if r == task.RecurrenceNone {
		return ""
	}
	if !task.IsValidRecurrence(r) {
		return fmt.Sprintf("invalid recurrence pattern: %s", s)
	}
	return ""
}
