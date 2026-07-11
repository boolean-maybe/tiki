package runtime

import (
	"time"

	"github.com/boolean-maybe/ruki"
	"github.com/boolean-maybe/tiki/tiki"
)

// tikiFromLegacy builds a tiki from legacy field values for format tests,
// wrapped as a ruki.Document for the projection types the formatters consume.
// Any non-zero schema field marks the tiki as workflow so formatters render
// the expected values. Full-schema presence is applied for fields that need
// "present empty" semantics (assignee, tags, etc.) to render as blank cells
// rather than "<absent>".
func tikiFromLegacy(f legacyFields) ruki.Document {
	tk := tiki.New()
	tk.SetID(f.ID)
	tk.SetTitle(f.Title)
	tk.SetBody(f.Description)
	tk.SetCreatedAt(f.CreatedAt)
	tk.SetUpdatedAt(f.UpdatedAt)
	tk.SetPath(f.FilePath)
	if f.CreatedBy != "" {
		tk.Set("createdBy", f.CreatedBy)
	}

	workflow := f.Status != "" || f.Type != "" || f.Priority != "" || f.Points != "" ||
		f.Tags != nil || f.DependsOn != nil || !f.Due.IsZero() ||
		f.Recurrence != "" || f.Assignee != ""

	if workflow {
		tk.Set("status", f.Status)
		tk.Set("type", f.Type)
		if f.Priority != "" {
			tk.Set("priority", f.Priority)
		}
		if f.Points != "" {
			tk.Set("points", f.Points)
		}
		tk.Set("assignee", f.Assignee)
		tk.Set("due", f.Due)
		tk.Set("recurrence", f.Recurrence)
		if f.Tags != nil {
			tk.Set("tags", append([]string(nil), f.Tags...))
		} else {
			tk.Set("tags", []string{})
		}
		if f.DependsOn != nil {
			tk.Set("dependsOn", append([]string(nil), f.DependsOn...))
		} else {
			tk.Set("dependsOn", []string{})
		}
	}
	for k, v := range f.CustomFields {
		tk.Set(k, v)
	}
	return tiki.WrapDoc(tk)
}

// legacyFields mirrors the workflow-field shape format tests care about.
// The runtime model (tiki.Tiki) stores these as ordinary entries in a generic
// Fields map; tests build fixtures here for ergonomic literal style.
type legacyFields struct {
	ID           string
	Title        string
	Description  string
	Status       string
	Type         string
	Priority     string
	Points       string
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
