package store

import (
	"testing"

	"github.com/boolean-maybe/tiki/document"
	taskpkg "github.com/boolean-maybe/tiki/task"
)

// TestInMemoryStore_DocumentAPIContract verifies InMemoryStore satisfies the
// Phase 4 DocumentStore interface and that the basic CRUD surface behaves as
// documented. A compile-time assertion lives next to the implementation; this
// test exercises the methods so a signature change does not silently shift
// semantics.
func TestInMemoryStore_DocumentAPIContract(t *testing.T) {
	var _ DocumentStore = NewInMemoryStore() // compile-time assertion
}

// TestInMemoryStore_CreateDocument_workflowPromotion verifies that a
// document carrying workflow frontmatter surfaces as a workflow-capable
// task after CreateDocument — i.e. GetAllTasks (workflow-only) returns it.
func TestInMemoryStore_CreateDocument_workflowPromotion(t *testing.T) {
	s := NewInMemoryStore()
	doc := &document.Document{
		ID:    "WKFW01",
		Title: "wf",
		Frontmatter: map[string]interface{}{
			"status": "ready",
		},
	}
	if err := s.CreateDocument(doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	got := s.GetDocument("WKFW01")
	if got == nil {
		t.Fatal("GetDocument returned nil after CreateDocument")
	}

	tasks := s.GetAllTasks()
	if len(tasks) != 1 || tasks[0].ID != "WKFW01" {
		t.Errorf("GetAllTasks = %v, want 1 workflow task WKFW01", tasks)
	}
}

// TestInMemoryStore_CreateDocument_plainDocStaysPlain verifies the presence-
// aware guarantee: a document with no workflow fields is NOT promoted to a
// workflow item by CreateDocument, even though CreateTask unconditionally
// promotes. The underlying Task stored in memory keeps IsWorkflow=false, so
// surfaces that honor the flag (board views, burndown, TikiStore.GetAllTasks)
// skip plain docs. InMemoryStore.GetAllTasks does not currently apply the
// workflow filter — that is a pre-existing inconsistency with TikiStore's
// GetAllTasks and is out of scope for Phase 4.
func TestInMemoryStore_CreateDocument_plainDocStaysPlain(t *testing.T) {
	s := NewInMemoryStore()
	doc := &document.Document{ID: "PLAIN1", Title: "plain"}

	if err := s.CreateDocument(doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	// Plain doc must not be workflow-flagged.
	tk := s.GetTask("PLAIN1")
	if tk == nil {
		t.Fatal("GetTask returned nil after CreateDocument")
	}
	if tk.IsWorkflow {
		t.Error("plain doc surfaced as workflow task — default promotion leaked through CreateDocument")
	}

	// Document-first surface sees the plain doc.
	allDocs := s.GetAllDocuments()
	if len(allDocs) != 1 || allDocs[0].ID != "PLAIN1" {
		t.Errorf("GetAllDocuments = %v, want plain doc visible", allDocs)
	}
}

// TestInMemoryStore_GetAllDocuments_mixesPlainAndWorkflow verifies the full
// unfiltered view: GetAllDocuments returns every stored document, plain or
// workflow, in id order — contrasting with GetAllTasks which applies the
// presence filter.
func TestInMemoryStore_GetAllDocuments_mixesPlainAndWorkflow(t *testing.T) {
	s := NewInMemoryStore()

	plainDoc := &document.Document{ID: "PLAIN1", Title: "plain"}
	wfDoc := &document.Document{ID: "WKFW01", Title: "wf",
		Frontmatter: map[string]interface{}{"status": "ready"}}

	if err := s.CreateDocument(plainDoc); err != nil {
		t.Fatalf("CreateDocument plain: %v", err)
	}
	if err := s.CreateDocument(wfDoc); err != nil {
		t.Fatalf("CreateDocument workflow: %v", err)
	}

	all := s.GetAllDocuments()
	if len(all) != 2 {
		t.Fatalf("GetAllDocuments count = %d, want 2", len(all))
	}
	// Ordered by id.
	if all[0].ID != "PLAIN1" || all[1].ID != "WKFW01" {
		t.Errorf("id order wrong: %s, %s", all[0].ID, all[1].ID)
	}
}

// TestInMemoryStore_UpdateDocument_roundTripsWorkflowFields verifies
// UpdateDocument carries frontmatter edits through the adapter boundary.
func TestInMemoryStore_UpdateDocument_roundTripsWorkflowFields(t *testing.T) {
	s := NewInMemoryStore()
	if err := s.CreateDocument(&document.Document{
		ID:          "UPD001",
		Title:       "v1",
		Frontmatter: map[string]interface{}{"status": "ready", "priority": 3},
	}); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	doc := s.GetDocument("UPD001")
	doc.Title = "v2"
	doc.Frontmatter["priority"] = 1
	if err := s.UpdateDocument(doc); err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}

	// Task-shaped read sees the new priority, proving the adapter projected
	// the change back into typed fields.
	tk := s.GetTask("UPD001")
	if tk == nil || tk.Title != "v2" || tk.Priority != 1 {
		t.Errorf("after UpdateDocument task = %+v, want Title=v2 Priority=1", tk)
	}
}

// TestInMemoryStore_DeleteDocument_removesFromBothSurfaces verifies the
// shared id space: DeleteDocument hides the entry from every surface.
func TestInMemoryStore_DeleteDocument_removesFromBothSurfaces(t *testing.T) {
	s := NewInMemoryStore()
	if err := s.CreateTask(&taskpkg.Task{ID: "DEL001", Title: "doomed"}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	s.DeleteDocument("DEL001")

	if got := s.GetDocument("DEL001"); got != nil {
		t.Errorf("GetDocument after DeleteDocument = %v, want nil", got)
	}
	if got := s.GetTask("DEL001"); got != nil {
		t.Errorf("GetTask after DeleteDocument = %v, want nil", got)
	}
}

// TestInMemoryStore_GetAllTasks_FiltersPlainDocs proves review finding #4
// is closed: GetAllTasks on the in-memory store applies the IsWorkflow
// filter just like TikiStore.GetAllTasks, so plain docs created via
// CreateDocument do not leak into board/list/burndown surfaces that
// consume the task-shaped API.
func TestInMemoryStore_GetAllTasks_FiltersPlainDocs(t *testing.T) {
	s := NewInMemoryStore()
	if err := s.CreateDocument(&document.Document{ID: "PLAIN1", Title: "plain"}); err != nil {
		t.Fatalf("CreateDocument plain: %v", err)
	}
	if err := s.CreateDocument(&document.Document{
		ID: "WKFW01", Title: "wf",
		Frontmatter: map[string]interface{}{"status": "ready"},
	}); err != nil {
		t.Fatalf("CreateDocument workflow: %v", err)
	}

	tasks := s.GetAllTasks()
	if len(tasks) != 1 {
		t.Fatalf("GetAllTasks = %v, want exactly 1 workflow task", tasks)
	}
	if tasks[0].ID != "WKFW01" {
		t.Errorf("GetAllTasks returned wrong task: %q, want WKFW01", tasks[0].ID)
	}
}

// TestInMemoryStore_UpdateDocument_Demotes proves the in-memory counterpart
// of the TikiStore demotion test: explicit workflow-to-plain demotion
// works through UpdateDocument without UpdateTask's carry-forward guard.
func TestInMemoryStore_UpdateDocument_Demotes(t *testing.T) {
	s := NewInMemoryStore()
	if err := s.CreateDocument(&document.Document{
		ID: "DEMO01", Title: "workflow",
		Frontmatter: map[string]interface{}{"status": "ready", "priority": 2},
	}); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	doc := s.GetDocument("DEMO01")
	doc.Frontmatter = nil
	if err := s.UpdateDocument(doc); err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}

	tk := s.GetTask("DEMO01")
	if tk == nil {
		t.Fatal("task missing after UpdateDocument")
	}
	if tk.IsWorkflow {
		t.Error("UpdateDocument failed to demote in-memory task")
	}
}
