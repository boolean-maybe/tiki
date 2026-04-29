package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

// TestSaveTask_UsesLoadedPath verifies the high-finding fix: after a task is
// loaded from a file with any path, subsequent save/delete/reload operations
// must target the loaded path — not an id-derived default path.
func TestSaveTask_UsesLoadedPath(t *testing.T) {
	tmpDir := t.TempDir()

	// write a file at a non-default path shape: filename doesn't match the id.
	custom := filepath.Join(tmpDir, "renamed-by-user.md")
	content := "---\nid: PATH01\ntitle: Original\ntype: story\nstatus: ready\npriority: 1\n---\nbody\n"
	if err := os.WriteFile(custom, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	tk := s.GetTask("PATH01")
	if tk == nil {
		t.Fatal("task not loaded")
	}

	// update — this should NOT create a new file at <id>.md.
	tk.Title = "Updated"
	if err := s.UpdateTask(tk); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	// the original file must be the one updated.
	updated, err := os.ReadFile(custom)
	if err != nil {
		t.Fatalf("read custom: %v", err)
	}
	if !strings.Contains(string(updated), "title: Updated") {
		t.Errorf("update did not write to loaded path: %s", updated)
	}

	// there must NOT be a duplicate file at the id-derived default path.
	if _, err := os.Stat(filepath.Join(tmpDir, "PATH01.md")); err == nil {
		t.Error("save created a duplicate file at id-derived path instead of updating the loaded path")
	}
}

// TestDeleteTask_UsesLoadedPath verifies delete targets the loaded path so a
// moved/renamed file still gets cleaned up.
func TestDeleteTask_UsesLoadedPath(t *testing.T) {
	tmpDir := t.TempDir()
	custom := filepath.Join(tmpDir, "moved.md")
	content := "---\nid: DEL001\ntitle: go away\ntype: story\nstatus: ready\npriority: 1\n---\nbody\n"
	if err := os.WriteFile(custom, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	s.DeleteTask("DEL001")

	if _, err := os.Stat(custom); !os.IsNotExist(err) {
		t.Errorf("delete did not remove the loaded file: err=%v", err)
	}
}

// TestCreateTask_NewFileUsesIDDerivedPath verifies the fallback: a brand-new
// task (FilePath empty) lands at <id>.md under the task dir.
func TestCreateTask_NewFileUsesIDDerivedPath(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	task := &taskpkg.Task{ID: "NEW001", Title: "fresh", Type: taskpkg.TypeStory, Status: "ready", Priority: 1}
	if err := s.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "NEW001.md")); err != nil {
		t.Errorf("expected new task file at NEW001.md, stat err: %v", err)
	}
}
