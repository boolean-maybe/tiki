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
	case "filepath":
		return t.FilePath
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

// FormatScalar renders a single scalar result as one line followed by a
// newline. Dates render as YYYY-MM-DD, timestamps as RFC3339, lists as JSON
// arrays, nil/unset as a blank line. This is the CLI output for top-level
// expression statements like `count(select where status = "done")`.
func FormatScalar(w io.Writer, res *ruki.ScalarResult) error {
	if res == nil {
		_, err := fmt.Fprintln(w, "")
		return err
	}
	_, err := fmt.Fprintln(w, renderScalarValue(res.Value, res.Type))
	return err
}

func renderScalarValue(val interface{}, typ ruki.ValueType) string {
	if val == nil {
		return ""
	}
	switch typ {
	case ruki.ValueDate:
		return renderDate(val)
	case ruki.ValueTimestamp:
		return renderTimestamp(val)
	case ruki.ValueListString, ruki.ValueListRef:
		return renderScalarList(val)
	case ruki.ValueBool:
		if b, ok := val.(bool); ok {
			if b {
				return "true"
			}
			return "false"
		}
		return fmt.Sprint(val)
	case ruki.ValueInt:
		return renderInt(val)
	default:
		return fmt.Sprint(val)
	}
}

// renderScalarList handles the []interface{} form produced by ruki's
// list-valued builtins, emitting JSON like ["a","b"] while also accepting
// []string for symmetry with the table formatter path.
func renderScalarList(val interface{}) string {
	switch v := val.(type) {
	case []string:
		b, err := json.Marshal(v)
		if err != nil {
			return "[]"
		}
		return string(b)
	case []interface{}:
		ss := make([]string, len(v))
		for i, elem := range v {
			ss[i] = fmt.Sprint(elem)
		}
		b, err := json.Marshal(ss)
		if err != nil {
			return "[]"
		}
		return string(b)
	default:
		return fmt.Sprint(val)
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

// JSONFormatter renders select results as a compact JSON array of row objects.
type JSONFormatter struct{}

// NewJSONFormatter returns a Formatter that emits compact JSON arrays.
func NewJSONFormatter() Formatter {
	return &JSONFormatter{}
}

func (f *JSONFormatter) Format(w io.Writer, proj *ruki.TaskProjection) error {
	fields := resolveFields(proj.Fields)

	rows := make([]map[string]interface{}, len(proj.Tasks))
	for i, t := range proj.Tasks {
		row := make(map[string]interface{}, len(fields))
		for _, fd := range fields {
			row[fd.Name] = jsonCellValue(t, fd)
		}
		rows[i] = row
	}

	b, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = fmt.Fprintln(w)
	return err
}

// jsonCellValue extracts a task field as a native JSON-marshalable value,
// using `null` for unset/zero date-like values and empty-string for unset
// scalar fields (matching how the executor/plugin treats "empty" elsewhere).
func jsonCellValue(t *task.Task, fd workflow.FieldDef) interface{} {
	val := extractFieldValue(t, fd.Name)
	return toJSONValue(val, fd.Type)
}

func toJSONValue(val interface{}, vt workflow.ValueType) interface{} {
	// list types always render as an array — including nil/unset, which emit
	// [] instead of null so scripts can always iterate without a null check
	if vt == workflow.TypeListString || vt == workflow.TypeListRef {
		return jsonList(val)
	}
	if val == nil {
		return nil
	}
	switch vt {
	case workflow.TypeDate:
		return jsonDate(val)
	case workflow.TypeTimestamp:
		return jsonTimestamp(val)
	case workflow.TypeInt:
		return jsonInt(val)
	case workflow.TypeBool:
		if b, ok := val.(bool); ok {
			return b
		}
		return val
	default:
		if s, ok := val.(string); ok {
			return s
		}
		return fmt.Sprint(val)
	}
}

func jsonDate(val interface{}) interface{} {
	t, ok := val.(time.Time)
	if !ok || t.IsZero() {
		return nil
	}
	return t.Format("2006-01-02")
}

func jsonTimestamp(val interface{}) interface{} {
	t, ok := val.(time.Time)
	if !ok || t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

// jsonList returns a non-nil []string so the JSON encoding always produces
// `[]` for list-typed fields, regardless of whether the backing slice is nil,
// empty, or populated. Downstream scripts can iterate without a null check.
func jsonList(val interface{}) interface{} {
	switch v := val.(type) {
	case nil:
		return []string{}
	case []string:
		if v == nil {
			return []string{}
		}
		return v
	case []interface{}:
		if v == nil {
			return []interface{}{}
		}
		return v
	default:
		return val
	}
}

func jsonInt(val interface{}) interface{} {
	if n, ok := val.(int); ok {
		return n
	}
	return val
}

// FormatScalarJSON writes a single scalar result as a bare JSON value plus
// newline, e.g. 42, true, "2026-04-26T...Z", or null.
func FormatScalarJSON(w io.Writer, res *ruki.ScalarResult) error {
	v := scalarJSONValue(res)
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = fmt.Fprintln(w)
	return err
}

func scalarJSONValue(res *ruki.ScalarResult) interface{} {
	if res == nil || res.Value == nil {
		return nil
	}
	switch res.Type {
	case ruki.ValueDate:
		return jsonDate(res.Value)
	case ruki.ValueTimestamp:
		return jsonTimestamp(res.Value)
	case ruki.ValueListString, ruki.ValueListRef:
		return scalarListJSON(res.Value)
	case ruki.ValueBool:
		if b, ok := res.Value.(bool); ok {
			return b
		}
		return res.Value
	case ruki.ValueInt:
		return jsonInt(res.Value)
	default:
		return res.Value
	}
}

func scalarListJSON(val interface{}) interface{} {
	switch v := val.(type) {
	case []string:
		if v == nil {
			return []string{}
		}
		return v
	case []interface{}:
		if v == nil {
			return []interface{}{}
		}
		return v
	default:
		return val
	}
}

// --- mutation / pipe / clipboard summaries ---

// formatUpdateSummary writes the summary for an UPDATE statement.
// Text form: "updated N tasks" or "updated N tasks (M failed)".
// JSON form: {"updated":N,"failed":M}.
func formatUpdateSummary(w io.Writer, succeeded, failed int, jsonOut bool) error {
	if jsonOut {
		return writeJSONLine(w, map[string]int{"updated": succeeded, "failed": failed})
	}
	if failed > 0 {
		_, err := fmt.Fprintf(w, "updated %d tasks (%d failed)\n", succeeded, failed)
		return err
	}
	_, err := fmt.Fprintf(w, "updated %d tasks\n", succeeded)
	return err
}

// formatCreateSummary writes the summary for a CREATE statement.
// Text form: "created TIKI-XXXXXX".
// JSON form: {"created":"TIKI-XXXXXX"}.
func formatCreateSummary(w io.Writer, id string, jsonOut bool) error {
	if jsonOut {
		return writeJSONLine(w, map[string]string{"created": id})
	}
	_, err := fmt.Fprintf(w, "created %s\n", id)
	return err
}

// formatDeleteSummary writes the summary for a DELETE statement.
func formatDeleteSummary(w io.Writer, succeeded, failed int, jsonOut bool) error {
	if jsonOut {
		return writeJSONLine(w, map[string]int{"deleted": succeeded, "failed": failed})
	}
	if failed > 0 {
		_, err := fmt.Fprintf(w, "deleted %d tasks (%d failed)\n", succeeded, failed)
		return err
	}
	_, err := fmt.Fprintf(w, "deleted %d tasks\n", succeeded)
	return err
}

// formatPipeSummary writes the summary after a pipe action completes.
func formatPipeSummary(w io.Writer, ran int, jsonOut bool) error {
	if jsonOut {
		return writeJSONLine(w, map[string]int{"ran": ran})
	}
	_, err := fmt.Fprintf(w, "ran command on %d rows\n", ran)
	return err
}

// formatClipboardSummary writes the summary after clipboard population.
func formatClipboardSummary(w io.Writer, copied int, jsonOut bool) error {
	if jsonOut {
		return writeJSONLine(w, map[string]int{"copied": copied})
	}
	_, err := fmt.Fprintf(w, "copied %d rows to clipboard\n", copied)
	return err
}

// writeJSONLine marshals v as compact JSON followed by a newline.
func writeJSONLine(w io.Writer, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = fmt.Fprintln(w)
	return err
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
