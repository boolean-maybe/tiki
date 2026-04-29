package tikistore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/config"
)

// TestCreateTask_IDUniquenessChecksMap verifies the H1 fix: the
// id-generation loop consults s.tasks (the authoritative identity index), not
// the filesystem. A task loaded from a renamed file occupies an id without
// occupying <taskdir>/<id>.md, so a filesystem-only check would let the
// generator collide and overwrite the in-memory entry.
func TestCreateTask_IDUniquenessChecksMap(t *testing.T) {
	dir := t.TempDir()

	// seed a task loaded from a non-default path: id ABC123 occupies the
	// identity slot, but ABC123.md does NOT exist under taskdir.
	renamed := filepath.Join(dir, "renamed-file.md")
	content := "---\nid: ABC123\ntitle: Loaded\ntype: story\nstatus: ready\npriority: 1\n---\nbody\n"
	if err := os.WriteFile(renamed, []byte(content), 0o644); err != nil {
		t.Fatalf("write renamed: %v", err)
	}

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// force the id generator to return ABC123 repeatedly, then a unique one.
	call := 0
	prev := config.GenerateRandomIDForTest
	config.GenerateRandomIDForTest = func() string {
		call++
		if call <= 3 {
			return "ABC123"
		}
		return "ZZZZZZ"
	}
	t.Cleanup(func() { config.GenerateRandomIDForTest = prev })

	newTask, err := s.NewTaskTemplate()
	if err != nil {
		t.Fatalf("NewTaskTemplate: %v", err)
	}
	if newTask.ID == "ABC123" {
		t.Fatal("generator returned the id of an existing (renamed-file) task — map-based uniqueness check failed")
	}
	if newTask.ID != "ZZZZZZ" {
		t.Errorf("expected fallback id ZZZZZZ, got %q", newTask.ID)
	}
	if call < 4 {
		t.Errorf("generator should have been called until a unique id was produced, got %d calls", call)
	}

	// the original loaded task must still exist under its original path.
	if tk := s.GetTask("ABC123"); tk == nil || tk.FilePath != renamed {
		t.Errorf("loaded task overwritten; FilePath=%q", tk.FilePath)
	}
}
