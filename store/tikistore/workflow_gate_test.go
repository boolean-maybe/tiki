package tikistore

import (
	"os"
	"path/filepath"
	"testing"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

func createValidTaskForGateTest() *taskpkg.Task {
	return &taskpkg.Task{
		ID:         "CREAT1",
		Title:      "created",
		Type:       taskpkg.TypeStory,
		Status:     "ready",
		Priority:   1,
		IsWorkflow: true,
	}
}

// TestGetAllTasks_IncludesPlainDocs verifies the Phase 5 contract: GetAllTasks
// returns all tikis projected to tasks, including plain docs. Workflow-only
// filtering belongs in the caller (e.g. ruki `select where has(status)` or
// hasAnyWorkflowField) not at the store boundary.
func TestGetAllTasks_IncludesPlainDocs(t *testing.T) {
	dir := t.TempDir()

	// plain doc: only id + title, no workflow fields.
	plain := filepath.Join(dir, "PLAIN1.md")
	if err := os.WriteFile(plain, []byte("---\nid: PLAIN1\ntitle: just a doc\n---\n\n# A plain markdown doc\n"), 0o644); err != nil {
		t.Fatalf("write plain: %v", err)
	}

	// workflow doc: has explicit status/type/priority.
	workflow := filepath.Join(dir, "WORK01.md")
	if err := os.WriteFile(workflow, []byte("---\nid: WORK01\ntitle: work item\ntype: story\nstatus: ready\npriority: 1\n---\nwork body\n"), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	all := s.GetAllTasks()
	if len(all) != 2 {
		t.Fatalf("GetAllTasks returned %d tasks, want 2 (plain + workflow)", len(all))
	}

	ids := map[string]bool{}
	for _, tk := range all {
		ids[tk.ID] = true
	}
	if !ids["PLAIN1"] {
		t.Error("plain doc should appear in GetAllTasks")
	}
	if !ids["WORK01"] {
		t.Error("workflow doc should appear in GetAllTasks")
	}

	// GetTask still finds both by id.
	if tk := s.GetTask("PLAIN1"); tk == nil {
		t.Error("GetTask should find plain docs by id")
	} else if tk.IsWorkflow {
		t.Error("plain doc should have IsWorkflow=false")
	}
	if tk := s.GetTask("WORK01"); tk == nil || !tk.IsWorkflow {
		t.Error("workflow doc should be marked IsWorkflow=true")
	}

	// GetAllTikis and GetAllTasks agree on count.
	if len(s.GetAllTikis()) != len(all) {
		t.Errorf("GetAllTikis count %d != GetAllTasks count %d", len(s.GetAllTikis()), len(all))
	}
}

// TestGetAllTasks_CreatedTasksAreWorkflow verifies that CreateTask marks the
// task as workflow-capable so programmatically-created tasks appear on boards.
func TestGetAllTasks_CreatedTasksAreWorkflow(t *testing.T) {
	dir := t.TempDir()
	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	if err := s.CreateTask(createValidTaskForGateTest()); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if got := len(s.GetAllTasks()); got != 1 {
		t.Errorf("GetAllTasks after CreateTask: got %d, want 1", got)
	}
}
