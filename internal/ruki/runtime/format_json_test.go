package runtime

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

func TestJSONFormatterSelectRows(t *testing.T) {
	initTestRegistries()

	proj := &ruki.TaskProjection{
		Fields: []string{"id", "title", "status"},
		Tasks: []*task.Task{
			{ID: "TIKI-AAA001", Title: "Build API", Status: "ready"},
			{ID: "TIKI-BBB002", Title: "Write Docs", Status: "done"},
		},
	}

	var buf bytes.Buffer
	if err := NewJSONFormatter().Format(&buf, proj); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rows); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["id"] != "TIKI-AAA001" || rows[0]["title"] != "Build API" || rows[0]["status"] != "ready" {
		t.Errorf("row 0 mismatch: %#v", rows[0])
	}
	if rows[1]["id"] != "TIKI-BBB002" {
		t.Errorf("row 1 id mismatch: %#v", rows[1])
	}
}

func TestJSONFormatterEmptyResult(t *testing.T) {
	initTestRegistries()

	proj := &ruki.TaskProjection{Fields: []string{"id"}, Tasks: nil}

	var buf bytes.Buffer
	if err := NewJSONFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	// bare empty array + newline
	if got := strings.TrimSpace(buf.String()); got != "[]" {
		t.Errorf("expected `[]`, got %q", got)
	}
}

func TestJSONFormatterIntAndBool(t *testing.T) {
	initTestRegistries()
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "active", Type: workflow.TypeBool},
	}); err != nil {
		t.Fatalf("register custom fields: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	proj := &ruki.TaskProjection{
		Fields: []string{"priority", "points", "active"},
		Tasks: []*task.Task{{
			Priority: 3, Points: 8,
			CustomFields: map[string]interface{}{"active": true},
		}},
	}

	var buf bytes.Buffer
	if err := NewJSONFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// JSON numbers decode to float64 through generic map
	if n, _ := rows[0]["priority"].(float64); n != 3 {
		t.Errorf("priority = %v, want 3", rows[0]["priority"])
	}
	if n, _ := rows[0]["points"].(float64); n != 8 {
		t.Errorf("points = %v, want 8", rows[0]["points"])
	}
	if b, _ := rows[0]["active"].(bool); !b {
		t.Errorf("active = %v, want true", rows[0]["active"])
	}
}

func TestJSONFormatterDatesAndTimestamps(t *testing.T) {
	initTestRegistries()

	due := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	ts := time.Date(2025, 6, 1, 14, 30, 0, 0, time.FixedZone("EST", -5*3600))
	proj := &ruki.TaskProjection{
		Fields: []string{"due", "createdAt"},
		Tasks:  []*task.Task{{Due: due, CreatedAt: ts}},
	}

	var buf bytes.Buffer
	if err := NewJSONFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if rows[0]["due"] != "2026-03-15" {
		t.Errorf("due = %v, want 2026-03-15", rows[0]["due"])
	}
	if rows[0]["createdAt"] != "2025-06-01T19:30:00Z" {
		t.Errorf("createdAt = %v, want RFC3339 UTC", rows[0]["createdAt"])
	}
}

func TestJSONFormatterUnsetDateIsNull(t *testing.T) {
	initTestRegistries()

	proj := &ruki.TaskProjection{
		Fields: []string{"due", "createdAt"},
		Tasks:  []*task.Task{{}},
	}

	var buf bytes.Buffer
	if err := NewJSONFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if rows[0]["due"] != nil {
		t.Errorf("zero due should be null, got %v", rows[0]["due"])
	}
	if rows[0]["createdAt"] != nil {
		t.Errorf("zero createdAt should be null, got %v", rows[0]["createdAt"])
	}
}

func TestJSONFormatterListFields(t *testing.T) {
	initTestRegistries()

	proj := &ruki.TaskProjection{
		Fields: []string{"tags", "dependsOn"},
		Tasks: []*task.Task{
			{Tags: []string{"backend", "urgent"}, DependsOn: []string{"TIKI-AAA001"}},
			{Tags: []string{}, DependsOn: nil},
		},
	}

	var buf bytes.Buffer
	if err := NewJSONFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	tags, _ := rows[0]["tags"].([]interface{})
	if len(tags) != 2 || tags[0] != "backend" || tags[1] != "urgent" {
		t.Errorf("tags = %v", rows[0]["tags"])
	}
	deps, _ := rows[0]["dependsOn"].([]interface{})
	if len(deps) != 1 || deps[0] != "TIKI-AAA001" {
		t.Errorf("dependsOn = %v", rows[0]["dependsOn"])
	}
	// list fields always render as [] — set-empty and nil both flatten to an
	// empty array so consumers can iterate without null checks
	if emptyTags, ok := rows[1]["tags"].([]interface{}); !ok || len(emptyTags) != 0 {
		t.Errorf("empty tags should be [], got %v (%T)", rows[1]["tags"], rows[1]["tags"])
	}
	if emptyDeps, ok := rows[1]["dependsOn"].([]interface{}); !ok || len(emptyDeps) != 0 {
		t.Errorf("nil dependsOn should be [], got %v (%T)", rows[1]["dependsOn"], rows[1]["dependsOn"])
	}
}

func TestJSONFormatterCustomFields(t *testing.T) {
	initTestRegistries()
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "high"}},
		{Name: "score", Type: workflow.TypeInt},
	}); err != nil {
		t.Fatalf("register custom fields: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	proj := &ruki.TaskProjection{
		Fields: []string{"severity", "score"},
		Tasks: []*task.Task{
			{CustomFields: map[string]interface{}{"severity": "high", "score": 42}},
			{}, // unset
		},
	}

	var buf bytes.Buffer
	if err := NewJSONFormatter().Format(&buf, proj); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if rows[0]["severity"] != "high" {
		t.Errorf("severity = %v, want high", rows[0]["severity"])
	}
	if n, _ := rows[0]["score"].(float64); n != 42 {
		t.Errorf("score = %v, want 42", rows[0]["score"])
	}
	// unset custom fields render as null
	if rows[1]["severity"] != nil {
		t.Errorf("unset severity should be null, got %v", rows[1]["severity"])
	}
	if rows[1]["score"] != nil {
		t.Errorf("unset score should be null, got %v", rows[1]["score"])
	}
}

func TestJSONFormatterWriteError(t *testing.T) {
	proj := &ruki.TaskProjection{
		Fields: []string{"id"},
		Tasks:  []*task.Task{{ID: "TIKI-ABC123"}},
	}
	// fail on the first write (the JSON array itself)
	ew := &errorWriter{failAfter: 0}
	if err := NewJSONFormatter().Format(ew, proj); err == nil {
		t.Fatal("expected write error on JSON array")
	}
	// fail on the trailing newline
	ew2 := &errorWriter{failAfter: 1}
	if err := NewJSONFormatter().Format(ew2, proj); err == nil {
		t.Fatal("expected write error on trailing newline")
	}
}

func TestFormatScalarJSON_Values(t *testing.T) {
	tests := []struct {
		name string
		res  *ruki.ScalarResult
		want string
	}{
		{"int", &ruki.ScalarResult{Value: 42, Type: ruki.ValueInt}, "42"},
		{"bool true", &ruki.ScalarResult{Value: true, Type: ruki.ValueBool}, "true"},
		{"bool false", &ruki.ScalarResult{Value: false, Type: ruki.ValueBool}, "false"},
		{"string", &ruki.ScalarResult{Value: "hello", Type: ruki.ValueString}, `"hello"`},
		{"nil", nil, "null"},
		{"nil value", &ruki.ScalarResult{Value: nil, Type: ruki.ValueString}, "null"},
		{"date", &ruki.ScalarResult{Value: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC), Type: ruki.ValueDate}, `"2026-04-26"`},
		{"timestamp", &ruki.ScalarResult{
			Value: time.Date(2026, 4, 26, 14, 30, 0, 0, time.UTC),
			Type:  ruki.ValueTimestamp,
		}, `"2026-04-26T14:30:00Z"`},
		{"zero date → null", &ruki.ScalarResult{Value: time.Time{}, Type: ruki.ValueDate}, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := FormatScalarJSON(&buf, tt.res); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := strings.TrimSpace(buf.String())
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatScalarJSON_List(t *testing.T) {
	res := &ruki.ScalarResult{Value: []string{"a", "b"}, Type: ruki.ValueListString}
	var buf bytes.Buffer
	if err := FormatScalarJSON(&buf, res); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != `["a","b"]` {
		t.Errorf(`got %q, want ["a","b"]`, got)
	}
}

func TestFormatScalarJSON_WriteError(t *testing.T) {
	res := &ruki.ScalarResult{Value: 42, Type: ruki.ValueInt}
	ew := &errorWriter{failAfter: 0}
	if err := FormatScalarJSON(ew, res); err == nil {
		t.Fatal("expected write error")
	}
}

func TestFormatUpdateSummary(t *testing.T) {
	var buf bytes.Buffer
	if err := formatUpdateSummary(&buf, 3, 0, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != `{"failed":0,"updated":3}` {
		t.Errorf("json success: got %q", got)
	}

	buf.Reset()
	if err := formatUpdateSummary(&buf, 2, 1, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != `{"failed":1,"updated":2}` {
		t.Errorf("json partial: got %q", got)
	}

	// text success
	buf.Reset()
	if err := formatUpdateSummary(&buf, 3, 0, false); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != "updated 3 tasks" {
		t.Errorf("text success: got %q", got)
	}

	// text partial
	buf.Reset()
	if err := formatUpdateSummary(&buf, 2, 1, false); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != "updated 2 tasks (1 failed)" {
		t.Errorf("text partial: got %q", got)
	}
}

func TestFormatCreateSummary(t *testing.T) {
	var buf bytes.Buffer
	if err := formatCreateSummary(&buf, "TIKI-ABC123", true); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != `{"created":"TIKI-ABC123"}` {
		t.Errorf("json: got %q", got)
	}

	buf.Reset()
	if err := formatCreateSummary(&buf, "TIKI-ABC123", false); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != "created TIKI-ABC123" {
		t.Errorf("text: got %q", got)
	}
}

func TestFormatDeleteSummary(t *testing.T) {
	var buf bytes.Buffer
	if err := formatDeleteSummary(&buf, 2, 0, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != `{"deleted":2,"failed":0}` {
		t.Errorf("json: got %q", got)
	}
}

func TestFormatPipeSummary(t *testing.T) {
	var buf bytes.Buffer
	if err := formatPipeSummary(&buf, 5, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != `{"ran":5}` {
		t.Errorf("json: got %q", got)
	}
}

func TestFormatClipboardSummary(t *testing.T) {
	var buf bytes.Buffer
	if err := formatClipboardSummary(&buf, 7, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != `{"copied":7}` {
		t.Errorf("json: got %q", got)
	}
}
