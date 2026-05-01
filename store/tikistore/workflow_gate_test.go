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

// TestGetAllTasks_FiltersPlainDocs verifies the presence-aware contract from
// the plan: a markdown file with `id` and `title` only must not surface on
// board/burndown/list views that consume GetAllTasks.
func TestGetAllTasks_FiltersPlainDocs(t *testing.T) {
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
	if len(all) != 1 {
		t.Fatalf("GetAllTasks returned %d tasks, want 1 (the workflow doc)", len(all))
	}
	if all[0].ID != "WORK01" {
		t.Errorf("GetAllTasks returned %q, want WORK01 — plain doc leaked through gate", all[0].ID)
	}

	// Both docs are still in the internal map; only GetAllTasks filters.
	if tk := s.GetTask("PLAIN1"); tk == nil {
		t.Error("GetTask should still find plain docs by id")
	} else if tk.IsWorkflow {
		t.Error("plain doc should have IsWorkflow=false")
	}
	if tk := s.GetTask("WORK01"); tk == nil || !tk.IsWorkflow {
		t.Error("workflow doc should be marked IsWorkflow=true")
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
