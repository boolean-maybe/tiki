package runtime

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/task"
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

func nonEmptyLines(s string) []string {
	var result []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result
}
