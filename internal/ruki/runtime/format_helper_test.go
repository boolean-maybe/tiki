package runtime

import (
	"time"

	"github.com/boolean-maybe/tiki/tiki"
)

// tikiFromLegacy builds a tiki from legacy field values for format tests.
// Any non-zero schema field marks the tiki as workflow so formatters render
// the expected values. Full-schema presence is applied for fields that need
// "present empty" semantics (assignee, tags, etc.) to render as blank cells
// rather than "<absent>".
func tikiFromLegacy(f legacyFields) *tiki.Tiki {
	tk := tiki.New()
	tk.ID = f.ID
	tk.Title = f.Title
	tk.Body = f.Description
	tk.CreatedAt = f.CreatedAt
	tk.UpdatedAt = f.UpdatedAt
	tk.Path = f.FilePath
	if f.CreatedBy != "" {
		tk.Set("createdBy", f.CreatedBy)
	}

	workflow := f.Status != "" || f.Type != "" || f.Priority != 0 || f.Points != 0 ||
		f.Tags != nil || f.DependsOn != nil || !f.Due.IsZero() ||
		f.Recurrence != "" || f.Assignee != ""

	if workflow {
		tk.Set(tiki.FieldStatus, f.Status)
		tk.Set(tiki.FieldType, f.Type)
		tk.Set(tiki.FieldPriority, f.Priority)
		tk.Set(tiki.FieldPoints, f.Points)
		tk.Set(tiki.FieldAssignee, f.Assignee)
		tk.Set(tiki.FieldDue, f.Due)
		tk.Set(tiki.FieldRecurrence, f.Recurrence)
		if f.Tags != nil {
			tk.Set(tiki.FieldTags, append([]string(nil), f.Tags...))
		} else {
			tk.Set(tiki.FieldTags, []string{})
		}
		if f.DependsOn != nil {
			tk.Set(tiki.FieldDependsOn, append([]string(nil), f.DependsOn...))
		} else {
			tk.Set(tiki.FieldDependsOn, []string{})
		}
	}
	for k, v := range f.CustomFields {
		tk.Set(k, v)
	}
	return tk
}

// legacyFields mirrors the task.Task fields that format tests care about.
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
