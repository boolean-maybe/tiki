package runtime

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

func TestTableFormatterProjectedFields(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"id", "title", "status"},
		Tasks: []*task.Task{
			{ID: "TIKI-AAA001", Title: "First", Status: "ready"},
			{ID: "TIKI-BBB002", Title: "Second", Status: "done"},
		},
	}

	var buf bytes.Buffer
	f := NewTableFormatter()
	if err := f.Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// header should contain the field names
	if !strings.Contains(out, "id") || !strings.Contains(out, "title") || !strings.Contains(out, "status") {
		t.Errorf("header missing field names:\n%s", out)
	}
	// data rows
	if !strings.Contains(out, "TIKI-AAA001") || !strings.Contains(out, "First") {
		t.Errorf("missing first row data:\n%s", out)
	}
	if !strings.Contains(out, "TIKI-BBB002") || !strings.Contains(out, "Second") {
		t.Errorf("missing second row data:\n%s", out)
	}
}

func TestTableFormatterFieldOrder(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"status", "id"},
		Tasks:  []*task.Task{{ID: "TIKI-A00001", Status: "ready"}},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(buf.String(), "\n")
	// header row is lines[1] (after separator)
	header := lines[1]
	statusIdx := strings.Index(header, "status")
	idIdx := strings.Index(header, "id")
	if statusIdx < 0 || idIdx < 0 {
		t.Fatalf("header missing fields: %q", header)
	}
	if statusIdx >= idIdx {
		t.Errorf("status should appear before id in header, got status@%d id@%d", statusIdx, idIdx)
	}
}

func TestTableFormatterEmptyResult(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"id", "title"},
		Tasks:  nil,
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// should have header but no data rows
	lines := nonEmptyLines(out)
	// top sep + header + sep + bottom sep = 4 lines
	if len(lines) != 4 {
		t.Errorf("empty result should produce 4 lines (sep+header+sep+sep), got %d:\n%s", len(lines), out)
	}
	if !strings.Contains(out, "id") || !strings.Contains(out, "title") {
		t.Errorf("header-only table missing field names:\n%s", out)
	}
}

func TestTableFormatterDateFormatting(t *testing.T) {
	due := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	proj := &ruki.TaskProjection{
		Fields: []string{"due"},
		Tasks:  []*task.Task{{Due: due}},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "2025-03-15") {
		t.Errorf("date should be YYYY-MM-DD format:\n%s", buf.String())
	}
}

func TestTableFormatterTimestampFormatting(t *testing.T) {
	ts := time.Date(2025, 6, 1, 14, 30, 0, 0, time.FixedZone("EST", -5*3600))
	proj := &ruki.TaskProjection{
		Fields: []string{"createdAt"},
		Tasks:  []*task.Task{{CreatedAt: ts}},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}

	// should be RFC3339 in UTC
	if !strings.Contains(buf.String(), "2025-06-01T19:30:00Z") {
		t.Errorf("timestamp should be RFC3339 UTC:\n%s", buf.String())
	}
}

func TestTableFormatterZeroDate(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"due"},
		Tasks:  []*task.Task{{Due: time.Time{}}},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}

	// zero date cell should be empty (just spaces between pipes)
	lines := strings.Split(buf.String(), "\n")
	dataRow := lines[3] // sep, header, sep, data
	// extract cell content between pipes
	parts := strings.Split(dataRow, "|")
	if len(parts) < 3 {
		t.Fatalf("unexpected row format: %q", dataRow)
	}
	cell := strings.TrimSpace(parts[1])
	if cell != "" {
		t.Errorf("zero date should render as empty cell, got %q", cell)
	}
}

func TestTableFormatterZeroTimestamp(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"createdAt"},
		Tasks:  []*task.Task{{CreatedAt: time.Time{}}},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(buf.String(), "\n")
	dataRow := lines[3]
	parts := strings.Split(dataRow, "|")
	cell := strings.TrimSpace(parts[1])
	if cell != "" {
		t.Errorf("zero timestamp should render as empty cell, got %q", cell)
	}
}

func TestTableFormatterListJSON(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"tags"},
		Tasks: []*task.Task{
			{Tags: []string{"backend", "urgent"}},
			{Tags: []string{}},
			{Tags: nil},
		},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, `["backend","urgent"]`) {
		t.Errorf("list should be JSON array:\n%s", out)
	}
	// empty slice renders as empty cell
	// nil slice also renders as empty cell
}

func TestTableFormatterListEscaping(t *testing.T) {
	// list cells with special characters should use standard JSON escaping
	proj := &ruki.TaskProjection{
		Fields: []string{"tags"},
		Tasks:  []*task.Task{{Tags: []string{`back\slash`, "new\nline", `"quoted"`}}},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// JSON encoding handles escaping
	if !strings.Contains(out, `"back\\slash"`) {
		t.Errorf("backslash should be JSON-escaped:\n%s", out)
	}
	if !strings.Contains(out, `"new\nline"`) {
		t.Errorf("newline should be JSON-escaped:\n%s", out)
	}
	if !strings.Contains(out, `"\"quoted\""`) {
		t.Errorf("quotes should be JSON-escaped:\n%s", out)
	}
}

func TestTableFormatterScalarEscaping(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"title"},
		Tasks:  []*task.Task{{Title: "line1\nline2\ttab\\slash"}},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, `line1\nline2\ttab\\slash`) {
		t.Errorf("scalar should have escaped \\n, \\t, \\\\:\n%s", out)
	}
}

func TestTableFormatterNoRowFooter(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"id"},
		Tasks:  []*task.Task{{ID: "TIKI-A00001"}, {ID: "TIKI-B00002"}},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// should not contain "2 rows" or similar
	lower := strings.ToLower(out)
	if strings.Contains(lower, "row") {
		t.Errorf("output should not contain row count footer:\n%s", out)
	}
}

func TestTableFormatterBorders(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"id"},
		Tasks:  []*task.Task{{ID: "TIKI-A00001"}},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}

	lines := nonEmptyLines(buf.String())
	// expect: separator, header, separator, data, separator = 5 lines
	if len(lines) != 5 {
		t.Errorf("expected 5 lines (3 separators + header + 1 data), got %d:\n%s", len(lines), buf.String())
	}
	for _, i := range []int{0, 2, 4} {
		if !strings.HasPrefix(lines[i], "+") || !strings.HasSuffix(lines[i], "+") {
			t.Errorf("separator line %d should start and end with +: %q", i, lines[i])
		}
	}
	for _, i := range []int{1, 3} {
		if !strings.HasPrefix(lines[i], "|") || !strings.HasSuffix(lines[i], "|") {
			t.Errorf("content line %d should start and end with |: %q", i, lines[i])
		}
	}
}

func TestTableFormatterEmptyString(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"assignee"},
		Tasks:  []*task.Task{{Assignee: ""}},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(buf.String(), "\n")
	dataRow := lines[3]
	parts := strings.Split(dataRow, "|")
	cell := strings.TrimSpace(parts[1])
	if cell != "" {
		t.Errorf("empty string should render as empty cell, got %q", cell)
	}
}

func TestTableFormatterAllFieldsDefault(t *testing.T) {
	// bare select with nil fields resolves to all canonical fields
	proj := &ruki.TaskProjection{
		Fields: nil,
		Tasks:  []*task.Task{{ID: "TIKI-A00001", Title: "Test", Status: "ready", Priority: 3}},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// should include all canonical fields in the header
	if !strings.Contains(out, "id") || !strings.Contains(out, "title") || !strings.Contains(out, "priority") {
		t.Errorf("all-fields table should contain canonical fields:\n%s", out)
	}
}

func TestRenderIntEdgeCases(t *testing.T) {
	// renderInt with non-int value
	if got := renderInt("not-an-int"); got != "" {
		t.Errorf("renderInt(string) = %q, want empty", got)
	}

	// renderInt with 0
	if got := renderInt(0); got != "0" {
		t.Errorf("renderInt(0) = %q, want %q", got, "0")
	}

	// renderInt with valid int
	if got := renderInt(42); got != "42" {
		t.Errorf("renderInt(42) = %q, want %q", got, "42")
	}
}

func TestRenderValueNil(t *testing.T) {
	if got := renderValue(nil, 0); got != "" {
		t.Errorf("renderValue(nil) = %q, want empty", got)
	}
}

func TestRenderDateEdgeCases(t *testing.T) {
	// renderDate with non-time value
	if got := renderDate("not-a-time"); got != "" {
		t.Errorf("renderDate(string) = %q, want empty", got)
	}
}

func TestRenderTimestampEdgeCases(t *testing.T) {
	// renderTimestamp with non-time value
	if got := renderTimestamp("not-a-time"); got != "" {
		t.Errorf("renderTimestamp(string) = %q, want empty", got)
	}
}

func TestRenderListEdgeCases(t *testing.T) {
	// renderList with non-slice value
	if got := renderList(42); got != "" {
		t.Errorf("renderList(int) = %q, want empty", got)
	}

	// renderList with nil slice
	if got := renderList([]string(nil)); got != "" {
		t.Errorf("renderList(nil) = %q, want empty", got)
	}
}

func TestExtractFieldValueUnknown(t *testing.T) {
	tk := &task.Task{ID: "TIKI-A00001"}
	if got := extractFieldValue(tk, "nonexistent"); got != nil {
		t.Errorf("extractFieldValue(unknown) = %v, want nil", got)
	}
}

func TestTableFormatterIntField(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"priority", "points"},
		Tasks:  []*task.Task{{Priority: 3, Points: 8}},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "3") {
		t.Errorf("missing priority value:\n%s", out)
	}
	if !strings.Contains(out, "8") {
		t.Errorf("missing points value:\n%s", out)
	}
}

func TestTableFormatterRecurrenceField(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"recurrence"},
		Tasks:  []*task.Task{{Recurrence: "0 0 * * MON"}},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "0 0 * * MON") {
		t.Errorf("missing recurrence value:\n%s", out)
	}
}

func nonEmptyLines(s string) []string {
	var result []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result
}

func TestTableFormatterWriteError(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"id"},
		Tasks:  []*task.Task{{ID: "TIKI-ABC123"}},
	}

	ew := &errorWriter{failAfter: 0}
	err := NewTableFormatter().Format(ew, proj)
	if err == nil {
		t.Fatal("expected write error")
	}
}

type errorWriter struct {
	writes    int
	failAfter int
}

func (w *errorWriter) Write(p []byte) (int, error) {
	if w.writes >= w.failAfter {
		return 0, fmt.Errorf("write error")
	}
	w.writes++
	return len(p), nil
}

func TestRenderListNilSlice(t *testing.T) {
	got := renderList(nil)
	if got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestRenderListNonStringSlice(t *testing.T) {
	got := renderList(42)
	if got != "" {
		t.Errorf("expected empty for non-slice, got %q", got)
	}
}

func TestRenderIntNonInt(t *testing.T) {
	got := renderInt("not an int")
	if got != "" {
		t.Errorf("expected empty for non-int, got %q", got)
	}
}

func TestRenderDateNonTime(t *testing.T) {
	got := renderDate("not a time")
	if got != "" {
		t.Errorf("expected empty for non-time, got %q", got)
	}
}

func TestRenderTimestampNonTime(t *testing.T) {
	got := renderTimestamp("not a time")
	if got != "" {
		t.Errorf("expected empty for non-time, got %q", got)
	}
}

// --- write-error tests at each stage of Format ---

func TestTableFormatterWriteErrorAtHeader(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"id"},
		Tasks:  []*task.Task{{ID: "TIKI-ABC123"}},
	}

	// fail on second write (header row)
	ew := &errorWriter{failAfter: 1}
	err := NewTableFormatter().Format(ew, proj)
	if err == nil {
		t.Fatal("expected write error at header")
	}
}

func TestTableFormatterWriteErrorAtSecondSep(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"id"},
		Tasks:  []*task.Task{{ID: "TIKI-ABC123"}},
	}

	// fail on third write (second separator after header)
	ew := &errorWriter{failAfter: 2}
	err := NewTableFormatter().Format(ew, proj)
	if err == nil {
		t.Fatal("expected write error at second separator")
	}
}

func TestTableFormatterWriteErrorAtDataRow(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"id"},
		Tasks:  []*task.Task{{ID: "TIKI-ABC123"}},
	}

	// fail on fourth write (data row)
	ew := &errorWriter{failAfter: 3}
	err := NewTableFormatter().Format(ew, proj)
	if err == nil {
		t.Fatal("expected write error at data row")
	}
}

func TestTableFormatterWriteErrorAtClosingSep(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"id"},
		Tasks:  []*task.Task{{ID: "TIKI-ABC123"}},
	}

	// fail on fifth write (closing separator)
	ew := &errorWriter{failAfter: 4}
	err := NewTableFormatter().Format(ew, proj)
	if err == nil {
		t.Fatal("expected write error at closing separator")
	}
}

func TestEscapeScalar(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello", "hello"},
		{"line\nnewline", `line\nnewline`},
		{"tab\there", `tab\there`},
		{`back\slash`, `back\\slash`},
	}
	for _, tt := range tests {
		got := escapeScalar(tt.input)
		if got != tt.want {
			t.Errorf("escapeScalar(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatCustomFields(t *testing.T) {
	// register custom fields so extractFieldValue and resolveFields find them
	initTestRegistries()
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "medium", "high"}},
		{Name: "score", Type: workflow.TypeInt},
		{Name: "active", Type: workflow.TypeBool},
		{Name: "notes", Type: workflow.TypeString},
	}); err != nil {
		t.Fatalf("register custom fields: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	proj := &ruki.TaskProjection{
		Fields: []string{"severity", "score", "active", "notes"},
		Tasks: []*task.Task{
			{
				ID: "TIKI-CF0001", Title: "Custom", Status: "ready",
				CustomFields: map[string]interface{}{
					"severity": "high",
					"score":    42,
					"active":   true,
					"notes":    "important",
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "high") {
		t.Errorf("missing severity value:\n%s", out)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("missing score value:\n%s", out)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("missing active value:\n%s", out)
	}
	if !strings.Contains(out, "important") {
		t.Errorf("missing notes value:\n%s", out)
	}
}

func TestFormatMissingCustomFields(t *testing.T) {
	initTestRegistries()
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "medium", "high"}},
		{Name: "score", Type: workflow.TypeInt},
		{Name: "active", Type: workflow.TypeBool},
		{Name: "notes", Type: workflow.TypeString},
	}); err != nil {
		t.Fatalf("register custom fields: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	// task with no custom fields set — should render as empty (nil → "")
	proj := &ruki.TaskProjection{
		Fields: []string{"score", "active"},
		Tasks: []*task.Task{
			{ID: "TIKI-CF0002", Title: "Empty", Status: "ready"},
		},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	lines := strings.Split(out, "\n")
	// data row is lines[3]
	dataRow := lines[3]
	parts := strings.Split(dataRow, "|")

	// score (int, unset) → ""
	scoreCell := strings.TrimSpace(parts[1])
	if scoreCell != "" {
		t.Errorf("unset int custom field should render as empty, got %q", scoreCell)
	}

	// active (bool, unset) → ""
	activeCell := strings.TrimSpace(parts[2])
	if activeCell != "" {
		t.Errorf("unset bool custom field should render as empty, got %q", activeCell)
	}
}

func TestFormatSetToZeroVsUnset(t *testing.T) {
	initTestRegistries()
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "score", Type: workflow.TypeInt},
		{Name: "active", Type: workflow.TypeBool},
	}); err != nil {
		t.Fatalf("register custom fields: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	proj := &ruki.TaskProjection{
		Fields: []string{"score", "active"},
		Tasks: []*task.Task{
			{ID: "TIKI-Z00001", Title: "Explicit zero", Status: "ready",
				CustomFields: map[string]interface{}{"score": 0, "active": false}},
			{ID: "TIKI-Z00002", Title: "Unset", Status: "ready"},
		},
	}

	var buf bytes.Buffer
	if err := NewTableFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	lines := strings.Split(out, "\n")

	// first data row (explicit zero): score=0, active=false
	row1 := strings.Split(lines[3], "|")
	if s := strings.TrimSpace(row1[1]); s != "0" {
		t.Errorf("explicit zero int should render as '0', got %q", s)
	}
	if s := strings.TrimSpace(row1[2]); s != "false" {
		t.Errorf("explicit false bool should render as 'false', got %q", s)
	}

	// second data row (unset): both empty
	row2 := strings.Split(lines[4], "|")
	if s := strings.TrimSpace(row2[1]); s != "" {
		t.Errorf("unset int should render as empty, got %q", s)
	}
	if s := strings.TrimSpace(row2[2]); s != "" {
		t.Errorf("unset bool should render as empty, got %q", s)
	}
}
