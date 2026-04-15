package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

// Formatter renders a TaskProjection to an io.Writer.
type Formatter interface {
	Format(w io.Writer, proj *ruki.TaskProjection) error
}

// TableFormatter renders results as an ASCII table.
type TableFormatter struct{}

// NewTableFormatter returns a Formatter that produces ASCII table output.
func NewTableFormatter() Formatter {
	return &TableFormatter{}
}

func (f *TableFormatter) Format(w io.Writer, proj *ruki.TaskProjection) error {
	fields := resolveFields(proj.Fields)

	// build cell grid
	rows := make([][]string, len(proj.Tasks))
	widths := make([]int, len(fields))
	for i, fd := range fields {
		if len(fd.Name) > widths[i] {
			widths[i] = len(fd.Name)
		}
	}
	for r, t := range proj.Tasks {
		row := make([]string, len(fields))
		for c, fd := range fields {
			row[c] = formatCell(t, fd)
			if len(row[c]) > widths[c] {
				widths[c] = len(row[c])
			}
		}
		rows[r] = row
	}

	// render
	sep := buildSeparator(widths)
	header := buildRow(fieldNames(fields), widths)

	if _, err := fmt.Fprintln(w, sep); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, header); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, sep); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(w, buildRow(row, widths)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, sep); err != nil {
		return err
	}
	return nil
}

// resolveFields returns the FieldDefs for the requested field names.
// If names is nil/empty (bare select), returns all fields in canonical order.
func resolveFields(names []string) []workflow.FieldDef {
	if len(names) == 0 {
		return workflow.Fields()
	}
	result := make([]workflow.FieldDef, 0, len(names))
	for _, name := range names {
		if fd, ok := workflow.Field(name); ok {
			result = append(result, fd)
		}
	}
	return result
}

func fieldNames(fields []workflow.FieldDef) []string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name
	}
	return names
}

// formatCell extracts and formats a single cell value from a task.
func formatCell(t *task.Task, fd workflow.FieldDef) string {
	val := extractFieldValue(t, fd.Name)
	return renderValue(val, fd.Type)
}

// extractFieldValue pulls a raw value from a task by field name.
func extractFieldValue(t *task.Task, name string) interface{} {
	switch name {
	case "id":
		return t.ID
	case "title":
		return t.Title
	case "description":
		return t.Description
	case "status":
		return string(t.Status)
	case "type":
		return string(t.Type)
	case "tags":
		return t.Tags
	case "dependsOn":
		return t.DependsOn
	case "due":
		return t.Due
	case "recurrence":
		return string(t.Recurrence)
	case "assignee":
		return t.Assignee
	case "priority":
		return t.Priority
	case "points":
		return t.Points
	case "createdBy":
		return t.CreatedBy
	case "createdAt":
		return t.CreatedAt
	case "updatedAt":
		return t.UpdatedAt
	default:
		fd, ok := workflow.Field(name)
		if !ok || !fd.Custom {
			return nil
		}
		if t.CustomFields != nil {
			if v, exists := t.CustomFields[name]; exists {
				return v
			}
		}
		// unset custom field — return nil to match executor semantics;
		// renderValue converts nil to "" so unset fields display as blank
		return nil
	}
}

// renderValue formats a value according to its workflow type.
func renderValue(val interface{}, vt workflow.ValueType) string {
	if val == nil {
		return ""
	}

	switch vt {
	case workflow.TypeDate:
		return renderDate(val)
	case workflow.TypeTimestamp:
		return renderTimestamp(val)
	case workflow.TypeListString, workflow.TypeListRef:
		return renderList(val)
	case workflow.TypeInt:
		return renderInt(val)
	case workflow.TypeEnum:
		return escapeScalar(fmt.Sprint(val))
	case workflow.TypeBool:
		return fmt.Sprint(val)
	default:
		return escapeScalar(fmt.Sprint(val))
	}
}

func renderDate(val interface{}) string {
	t, ok := val.(time.Time)
	if !ok || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func renderTimestamp(val interface{}) string {
	t, ok := val.(time.Time)
	if !ok || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func renderList(val interface{}) string {
	ss, ok := val.([]string)
	if !ok || ss == nil {
		return ""
	}
	// JSON array encoding — this is the final cell text, not passed through escapeScalar
	b, err := json.Marshal(ss)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func renderInt(val interface{}) string {
	n, ok := val.(int)
	if !ok {
		return ""
	}
	return fmt.Sprint(n)
}

// escapeScalar escapes backslash, newline, and tab in scalar text cells.
func escapeScalar(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// buildSeparator creates a "+---+---+" style separator line.
func buildSeparator(widths []int) string {
	var b strings.Builder
	b.WriteByte('+')
	for _, w := range widths {
		b.WriteByte('-')
		for range w {
			b.WriteByte('-')
		}
		b.WriteByte('-')
		b.WriteByte('+')
	}
	return b.String()
}

// buildRow creates a "| val | val |" style row.
func buildRow(cells []string, widths []int) string {
	var b strings.Builder
	b.WriteByte('|')
	for i, cell := range cells {
		b.WriteByte(' ')
		b.WriteString(cell)
		// pad
		for range widths[i] - len(cell) {
			b.WriteByte(' ')
		}
		b.WriteString(" |")
	}
	return b.String()
}
