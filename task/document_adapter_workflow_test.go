package task

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/document"
)

// TestFromDocument_workflowFieldsRoundTripThroughFrontmatter verifies that
// a document whose frontmatter carries workflow fields is adapted into a
// Task with those fields populated — the Phase 4 mapping rule that typed
// Task fields are sourced from presence-bearing Frontmatter entries.
func TestFromDocument_workflowFieldsRoundTripThroughFrontmatter(t *testing.T) {
	due := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	doc := &document.Document{
		ID:    "ABC123",
		Title: "workflow doc",
		Body:  "body",
		Frontmatter: map[string]interface{}{
			"status":    "doing",
			"type":      "bug",
			"priority":  2,
			"points":    5,
			"tags":      []interface{}{"alpha", "beta"},
			"dependsOn": []interface{}{"XYZ999"},
			"due":       due,
			"assignee":  "alice",
		},
	}

	tk := FromDocument(doc)

	if !tk.IsWorkflow {
		t.Fatal("IsWorkflow = false, want true for doc with workflow frontmatter")
	}
	if string(tk.Status) != "doing" {
		t.Errorf("Status = %q, want doing", tk.Status)
	}
	if string(tk.Type) != "bug" {
		t.Errorf("Type = %q, want bug", tk.Type)
	}
	if tk.Priority != 2 {
		t.Errorf("Priority = %d, want 2", tk.Priority)
	}
	if tk.Points != 5 {
		t.Errorf("Points = %d, want 5", tk.Points)
	}
	if len(tk.Tags) != 2 || tk.Tags[0] != "alpha" || tk.Tags[1] != "beta" {
		t.Errorf("Tags = %v, want [alpha beta]", tk.Tags)
	}
	if len(tk.DependsOn) != 1 || tk.DependsOn[0] != "XYZ999" {
		t.Errorf("DependsOn = %v, want [XYZ999]", tk.DependsOn)
	}
	if !tk.Due.Equal(due) {
		t.Errorf("Due = %v, want %v", tk.Due, due)
	}
	if tk.Assignee != "alice" {
		t.Errorf("Assignee = %q, want alice", tk.Assignee)
	}
}

// TestToDocument_workflowFieldsMaterializeIntoFrontmatter verifies the
// reverse direction: a workflow Task produces a Document whose Frontmatter
// carries every non-zero workflow field. This is the invariant that makes
// ToDocument -> FromDocument lossless for workflow-capable tasks.
func TestToDocument_workflowFieldsMaterializeIntoFrontmatter(t *testing.T) {
	due := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	tk := &Task{
		ID:         "ABC123",
		Title:      "round trip",
		Status:     "ready",
		Type:       "story",
		Priority:   3,
		Points:     8,
		Tags:       []string{"k1", "k2"},
		DependsOn:  []string{"AAA111"},
		Due:        due,
		Assignee:   "bob",
		IsWorkflow: true,
	}

	doc := ToDocument(tk)
	fm := doc.Frontmatter

	if fm == nil {
		t.Fatal("Frontmatter = nil, want populated")
	}
	if fm["status"] != "ready" {
		t.Errorf("fm[status] = %v, want ready", fm["status"])
	}
	if fm["type"] != "story" {
		t.Errorf("fm[type] = %v, want story", fm["type"])
	}
	if fm["priority"] != 3 {
		t.Errorf("fm[priority] = %v, want 3", fm["priority"])
	}
	if fm["points"] != 8 {
		t.Errorf("fm[points] = %v, want 8", fm["points"])
	}
	if fm["assignee"] != "bob" {
		t.Errorf("fm[assignee] = %v, want bob", fm["assignee"])
	}
	if d, ok := fm["due"].(time.Time); !ok || !d.Equal(due) {
		t.Errorf("fm[due] = %v, want %v", fm["due"], due)
	}

	// Round-trip preserves every workflow field.
	back := FromDocument(doc)
	if back.Status != tk.Status || back.Type != tk.Type || back.Priority != tk.Priority ||
		back.Points != tk.Points || back.Assignee != tk.Assignee || !back.Due.Equal(tk.Due) {
		t.Errorf("round-trip lost workflow data: original=%+v back=%+v", tk, back)
	}
	if len(back.Tags) != 2 || back.Tags[0] != "k1" || back.Tags[1] != "k2" {
		t.Errorf("Tags round-trip failed: %v", back.Tags)
	}
	if len(back.DependsOn) != 1 || back.DependsOn[0] != "AAA111" {
		t.Errorf("DependsOn round-trip failed: %v", back.DependsOn)
	}
}

// TestToDocument_plainTaskProducesPlainDocument verifies the presence-aware
// guarantee on the Task -> Document direction: a Task with no workflow
// fields (zero-valued) produces a Document whose Frontmatter is nil, which
// classifies as a plain doc. This prevents the adapter from accidentally
// promoting plain documents when a caller round-trips them through Task.
func TestToDocument_plainTaskProducesPlainDocument(t *testing.T) {
	tk := &Task{ID: "PLAIN1", Title: "plain doc"}

	doc := ToDocument(tk)

	if doc.Frontmatter != nil {
		t.Errorf("Frontmatter = %v, want nil for plain task", doc.Frontmatter)
	}
	if document.IsWorkflowFrontmatter(doc.Frontmatter) {
		t.Error("plain task round-tripped as workflow document")
	}
}

// TestToDocument_PlainTaskIgnoresNonZeroTypedFields proves the stronger
// presence-aware guarantee: even if the store has populated typed workflow
// fields (Priority/Points/Status) on a plain Task — which used to happen
// via loadTaskFile defaulting — ToDocument must still emit a plain
// Document because IsWorkflow=false. This is the adapter-level half of
// review finding #1: the synthesis path cannot leak workflow frontmatter
// when the classification says "plain".
func TestToDocument_PlainTaskIgnoresNonZeroTypedFields(t *testing.T) {
	tk := &Task{
		ID:         "PLAIN1",
		Title:      "plain doc",
		Priority:   3,   // residual default — must NOT project
		Points:     5,   // residual default — must NOT project
		Status:     "x", // residual value — must NOT project
		IsWorkflow: false,
	}

	doc := ToDocument(tk)

	if doc.Frontmatter != nil {
		t.Errorf("plain task leaked Frontmatter from typed fields: %v", doc.Frontmatter)
	}
	if document.IsWorkflowFrontmatter(doc.Frontmatter) {
		t.Error("plain task with residual typed fields re-classified as workflow")
	}
}

// TestToDocument_WorkflowTaskPreservesSourceFrontmatter proves the
// preserved-presence path wins over value-derived synthesis. A workflow
// task loaded with only `status` in its preserved WorkflowFrontmatter must
// project back with only `status`, even if ancillary typed fields (like a
// defaulted Priority) are non-zero.
func TestToDocument_WorkflowTaskPreservesSourceFrontmatter(t *testing.T) {
	tk := &Task{
		ID:         "SPARSE",
		Title:      "sparse",
		Status:     "ready",
		Priority:   3, // would normally synthesize priority: 3 on output
		IsWorkflow: true,
		WorkflowFrontmatter: map[string]interface{}{
			"status": "ready",
		},
	}

	doc := ToDocument(tk)

	if _, has := doc.Frontmatter["status"]; !has {
		t.Error("preserved status did not project")
	}
	if _, leaked := doc.Frontmatter["priority"]; leaked {
		t.Errorf("priority leaked despite being absent from preserved frontmatter: %v",
			doc.Frontmatter["priority"])
	}
}

// TestToDocument_WorkflowTaskWithoutPreservedFallsBackToSynthesis
// verifies the fallback path: a workflow task with nil
// WorkflowFrontmatter (e.g. freshly built by NewTaskTemplate) emits
// frontmatter synthesized from typed values. Without this, creation
// defaults would never reach the Document.
func TestToDocument_WorkflowTaskWithoutPreservedFallsBackToSynthesis(t *testing.T) {
	tk := &Task{
		ID:         "NEW001",
		Title:      "fresh",
		Status:     "ready",
		Priority:   3,
		IsWorkflow: true,
		// WorkflowFrontmatter intentionally nil
	}

	doc := ToDocument(tk)

	if doc.Frontmatter["status"] != "ready" {
		t.Errorf("synthesis lost status: %v", doc.Frontmatter["status"])
	}
	if doc.Frontmatter["priority"] != 3 {
		t.Errorf("synthesis lost priority: %v", doc.Frontmatter["priority"])
	}
}

// TestRecurrenceRoundTrip proves review finding #3 is closed: recurrence
// is classified as a workflow key AND is read/written by the adapter
// helpers. A doc -> task -> doc round trip must preserve it.
func TestRecurrenceRoundTrip(t *testing.T) {
	doc := &document.Document{
		ID:    "RECUR1",
		Title: "weekly review",
		Frontmatter: map[string]interface{}{
			"status":     "ready",
			"recurrence": "0 0 * * MON",
		},
	}

	tk := FromDocument(doc)
	if string(tk.Recurrence) != "0 0 * * MON" {
		t.Errorf("FromDocument lost recurrence: got %q", tk.Recurrence)
	}

	back := ToDocument(tk)
	if back.Frontmatter["recurrence"] != "0 0 * * MON" {
		t.Errorf("ToDocument lost recurrence: got %v", back.Frontmatter["recurrence"])
	}
}
