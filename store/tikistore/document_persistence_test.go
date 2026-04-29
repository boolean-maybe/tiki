package tikistore

import (
	"os"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/document"
)

// TestCreateDocument_PlainDocFileHasNoWorkflowKeys proves the review's
// High finding #1 is closed: a plain document created via CreateDocument
// must be serialized WITHOUT workflow keys (status/type/priority/points)
// in the YAML frontmatter. If workflow keys leak into the file, a reload
// would re-promote the doc through document.IsWorkflowFrontmatter.
func TestCreateDocument_PlainDocFileHasNoWorkflowKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	if err := s.CreateDocument(&document.Document{
		ID:    "PLAIN1",
		Title: "plain doc",
		Body:  "hello",
	}); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	data, err := os.ReadFile(tmp + "/PLAIN1.md")
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	for _, forbidden := range []string{"status:", "type:", "priority:", "points:"} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("plain doc file contains %q workflow key; full contents:\n%s",
				forbidden, data)
		}
	}

	// Reloading must keep the doc plain — the classification survives disk.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	tk := s.GetTask("PLAIN1")
	if tk == nil {
		t.Fatal("GetTask after Reload = nil")
	}
	if tk.IsWorkflow {
		t.Error("plain doc was promoted to workflow on reload — workflow keys persisted to disk")
	}
}

// TestUpdateDocument_DemotesWorkflowDocWhenFrontmatterStripped proves the
// review's High finding #2 is closed: a caller can demote a workflow doc
// back to plain by stripping every workflow key from the frontmatter and
// calling UpdateDocument. UpdateTask's protective carry-forward must not
// fire on the document-first path.
func TestUpdateDocument_DemotesWorkflowDocWhenFrontmatterStripped(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Create as workflow.
	if err := s.CreateDocument(&document.Document{
		ID:    "DEMO01",
		Title: "will be demoted",
		Frontmatter: map[string]interface{}{
			"status":   "ready",
			"priority": 2,
		},
	}); err != nil {
		t.Fatalf("CreateDocument workflow: %v", err)
	}

	// Confirm starting state.
	before := s.GetTask("DEMO01")
	if before == nil || !before.IsWorkflow {
		t.Fatalf("precondition: expected workflow task, got %+v", before)
	}

	// Now update with no workflow frontmatter — explicit demotion.
	doc := s.GetDocument("DEMO01")
	doc.Frontmatter = nil
	if err := s.UpdateDocument(doc); err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}

	after := s.GetTask("DEMO01")
	if after == nil {
		t.Fatal("GetTask after UpdateDocument = nil")
	}
	if after.IsWorkflow {
		t.Error("UpdateDocument failed to demote: IsWorkflow still true")
	}

	// And it must persist across a reload.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	reloaded := s.GetTask("DEMO01")
	if reloaded == nil {
		t.Fatal("reloaded doc missing")
	}
	if reloaded.IsWorkflow {
		t.Error("demotion did not survive reload — workflow keys still on disk")
	}
}

// TestUpdateTask_StillCarriesWorkflowForwardForTaskCallers proves the
// demotion change does not regress the protective carry-forward on the
// task-shaped API: a caller that passes a fresh Task with IsWorkflow=false
// over a workflow-flagged stored task still has the flag carried forward,
// matching the pre-existing semantics (and the reason the guard exists).
func TestUpdateTask_StillCarriesWorkflowForwardForTaskCallers(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	if err := s.CreateDocument(&document.Document{
		ID:          "CARRY1",
		Title:       "workflow",
		Frontmatter: map[string]interface{}{"status": "ready"},
	}); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	// Simulate a ruki/UI caller that rebuilt a fresh Task and forgot IsWorkflow.
	fresh := s.GetTask("CARRY1").Clone()
	fresh.IsWorkflow = false
	if err := s.UpdateTask(fresh); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	if got := s.GetTask("CARRY1"); got == nil || !got.IsWorkflow {
		t.Errorf("UpdateTask should carry workflow forward; got %+v", got)
	}
}
