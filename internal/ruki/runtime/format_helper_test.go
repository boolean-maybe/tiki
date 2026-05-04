package runtime

import (
	"time"

	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/tiki"
)

// tikiFromLegacy builds a tiki from task-shaped fields for format tests.
// Mirrors the Phase 4 tikiFromTask convention: any non-zero schema field
// promotes to workflow so formatters render the expected values.
func tikiFromLegacy(t legacyFields) *tiki.Tiki {
	src := &task.Task{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Status:      task.Status(t.Status),
		Type:        task.Type(t.Type),
		Priority:    t.Priority,
		Points:      t.Points,
		Tags:        t.Tags,
		DependsOn:   t.DependsOn,
		Due:         t.Due,
		Recurrence:  task.Recurrence(t.Recurrence),
		Assignee:    t.Assignee,
		CreatedBy:   t.CreatedBy,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		FilePath:    t.FilePath,
	}
	workflow := src.Status != "" || src.Type != "" || src.Priority != 0 || src.Points != 0 ||
		src.Tags != nil || src.DependsOn != nil || !src.Due.IsZero() ||
		src.Recurrence != "" || src.Assignee != ""
	if workflow {
		src.IsWorkflow = true
	}
	out := tiki.FromTask(src)
	if out == nil {
		return nil
	}
	// Full-schema presence: initialize each schema key whose typed value
	// is present on the fixture, even when the Go zero is indistinguishable
	// from unset. This matches the Phase 4 test convention used across ruki
	// tests — formatters need present-empty strings to render as blank
	// cells rather than "<absent>".
	if workflow {
		if t.Tags != nil && !out.Has(tiki.FieldTags) {
			out.Set(tiki.FieldTags, append([]string(nil), t.Tags...))
		}
		if t.DependsOn != nil && !out.Has(tiki.FieldDependsOn) {
			out.Set(tiki.FieldDependsOn, append([]string(nil), t.DependsOn...))
		}
		if !out.Has(tiki.FieldAssignee) {
			out.Set(tiki.FieldAssignee, t.Assignee)
		}
		if !out.Has(tiki.FieldDue) {
			out.Set(tiki.FieldDue, t.Due)
		}
		if !out.Has(tiki.FieldRecurrence) {
			out.Set(tiki.FieldRecurrence, string(src.Recurrence))
		}
		if !out.Has(tiki.FieldPoints) {
			out.Set(tiki.FieldPoints, t.Points)
		}
		if !out.Has(tiki.FieldPriority) {
			out.Set(tiki.FieldPriority, t.Priority)
		}
	}
	for k, v := range t.CustomFields {
		out.Set(k, v)
	}
	return out
}

// legacyFields mirrors the task.Task fields that format tests care about,
// so migrating a fixture is a find-replace from "&task.Task{" to
// "tikiFromLegacy(legacyFields{" with a closing paren added.
type legacyFields struct {
	ID           string
	Title        string
	Description  string
	Status       string
	Type         string
	Priority     int
	Points       int
	Tags         []string
	DependsOn    []string
	Due          time.Time
	Recurrence   string
	Assignee     string
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	FilePath     string
	CustomFields map[string]interface{}
}
