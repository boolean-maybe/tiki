package tikistore

import (
	"testing"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

func TestReloadTask_CaseDuplicate(t *testing.T) {
	store, err := NewTikiStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create a task with a lowercase suffix ID
	task := &taskpkg.Task{
		ID:       "6EQDUE",
		Title:    "Case Duplicate",
		Type:     taskpkg.TypeStory,
		Status:   taskpkg.StatusBacklog,
		Priority: 3,
		Points:   1,
	}
	if err := store.CreateTask(task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Reload by lowercase ID; should not create a duplicate entry.
	if err := store.ReloadTask("6EQDUE"); err != nil {
		t.Fatalf("ReloadTask failed: %v", err)
	}

	tasks := store.GetAllTasks()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after reload, got %d", len(tasks))
	}

	foundUpper := false
	for _, tsk := range tasks {
		switch tsk.ID {
		case "6EQDUE":
			foundUpper = true
		}
	}

	if !foundUpper {
		t.Fatalf("expected uppercase ID variant, foundUpper=%v", foundUpper)
	}
}
