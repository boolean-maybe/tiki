package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

// TestUpdateTask_FreshPartialTaskDoesNotEraseWorkflowKeys proves the
// current review finding is closed: a task-shaped compatibility caller
// that builds a fresh Task value carrying only the fields it cares about
// must NOT lose existing workflow keys whose typed values it left zero.
//
// Pre-fix failure mode: a sparse workflow file contained only
// `status: ready`. A caller updated only the title with
// Task{ID, Title}. updateTaskLocked carried IsWorkflow forward, seeded
// WorkflowFrontmatter={status}, then MergeTypedWorkflowDeltas saw
// incoming Status=="" differ from old Status=="ready" and DELETED the
// status key. saveTask then wrote a workflow task with no workflow keys,
// re-classifying the file as plain on the next load.
//
// The fix makes MergeTypedWorkflowDeltas grow-only: zeros on the
// incoming task mean "caller did not set this" and are ignored.
func TestUpdateTask_FreshPartialTaskDoesNotEraseWorkflowKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	sparsePath := filepath.Join(tmp, "SPARSE.md")
	sparse := "---\nid: SPARSE\ntitle: v1\nstatus: ready\n---\n\nbody\n"
	if err := os.WriteFile(sparsePath, []byte(sparse), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Task-shaped compatibility caller: builds a fresh Task with only the
	// fields it cares about (ID and new title). Every other typed field is
	// zero because the caller simply did not populate them. The store must
	// preserve everything else — path, mtime, IsWorkflow, AND the sparse
	// workflow presence set.
	if err := s.UpdateTask(&taskpkg.Task{ID: "SPARSE", Title: "v2"}); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	data, err := os.ReadFile(sparsePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	contents := string(data)

	if !strings.Contains(contents, "status: ready") {
		t.Errorf("status key erased by partial UpdateTask; contents:\n%s", contents)
	}
	if !strings.Contains(contents, "title: v2") {
		t.Errorf("title edit did not persist; contents:\n%s", contents)
	}

	// Reload must still classify as workflow — the pre-fix bug wrote the
	// file with no workflow keys so it reloaded as plain.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if tk := s.GetTask("SPARSE"); tk == nil {
		t.Fatal("GetTask after reload = nil")
	} else if !tk.IsWorkflow {
		t.Error("sparse workflow doc demoted to plain across partial UpdateTask + reload")
	}
}

// TestUpdateTask_FreshPartialTaskPreservesEveryWorkflowKey extends the
// case above to a fuller workflow doc: a file with status, type, and
// priority present. A partial UpdateTask that only changes the title must
// preserve all three.
func TestUpdateTask_FreshPartialTaskPreservesEveryWorkflowKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	filePath := filepath.Join(tmp, "FULL01.md")
	src := "---\nid: FULL01\ntitle: v1\nstatus: ready\ntype: story\npriority: 2\n---\n\nbody\n"
	if err := os.WriteFile(filePath, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	if err := s.UpdateTask(&taskpkg.Task{ID: "FULL01", Title: "v2"}); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	data, _ := os.ReadFile(filePath)
	contents := string(data)
	for _, required := range []string{"title: v2", "status: ready", "type: story", "priority: 2"} {
		if !strings.Contains(contents, required) {
			t.Errorf("expected %q in file after partial UpdateTask; contents:\n%s", required, contents)
		}
	}
}

// TestUpdateDocument_ExplicitKeyRemovalStillWorks verifies the complement:
// the document-first API must still be able to remove a workflow key by
// omitting it from doc.Frontmatter. Grow-only on the task API path does
// NOT apply to the document API path, because doc.Frontmatter IS the
// authoritative presence set.
func TestUpdateDocument_ExplicitKeyRemovalStillWorks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	filePath := filepath.Join(tmp, "TRIM01.md")
	src := "---\nid: TRIM01\ntitle: v1\nstatus: ready\npriority: 2\n---\n\nbody\n"
	if err := os.WriteFile(filePath, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	doc := s.GetDocument("TRIM01")
	if doc == nil {
		t.Fatal("GetDocument: nil")
	}
	// Remove priority explicitly — document API honors this.
	delete(doc.Frontmatter, "priority")
	if err := s.UpdateDocument(doc); err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}

	data, _ := os.ReadFile(filePath)
	contents := string(data)
	if !strings.Contains(contents, "status: ready") {
		t.Errorf("status should be preserved; contents:\n%s", contents)
	}
	if strings.Contains(contents, "priority:") {
		t.Errorf("priority should be removed via document API; contents:\n%s", contents)
	}
}
