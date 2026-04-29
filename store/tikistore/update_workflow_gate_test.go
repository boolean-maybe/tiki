package tikistore

import (
	"os"
	"path/filepath"
	"testing"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

// TestUpdateTask_PreservesIsWorkflowForFreshTaskValues is a regression
// guard for the subtle demotion bug: a caller that constructs a brand-new
// Task value (so IsWorkflow is the zero value, false) and passes it to
// UpdateTask must not see the task disappear from GetAllTasks. The store
// has to carry IsWorkflow forward from the loaded task when the caller
// left it zero-valued, matching how FilePath and LoadedMtime are handled.
//
// Without this safety net, the task stays in the store map (so GetTask
// still finds it by id) but vanishes from board/list views that filter
// on IsWorkflow, until a full reload reclassifies it from frontmatter.
func TestUpdateTask_PreservesIsWorkflowForFreshTaskValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "WF0001.md")
	content := "---\nid: WF0001\ntitle: workflow doc\ntype: story\nstatus: backlog\npriority: 3\npoints: 1\n---\nbody\n"
	//nolint:gosec // G306: matches repo-wide test fixture perms
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	store, err := NewTikiStore(root)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	loaded := store.GetTask("WF0001")
	if loaded == nil {
		t.Fatal("WF0001 missing after load")
	}
	if !loaded.IsWorkflow {
		t.Fatal("precondition: loaded doc should be workflow-capable (status set)")
	}

	// Simulate a caller that constructs a fresh Task value from fields
	// they remember — no IsWorkflow, no FilePath, no LoadedMtime — and
	// calls UpdateTask. This is the pattern that used to silently demote
	// the task.
	fresh := &taskpkg.Task{
		ID:       "WF0001",
		Title:    "updated title",
		Type:     taskpkg.TypeStory,
		Status:   "backlog",
		Priority: 3,
		Points:   1,
	}
	if err := store.UpdateTask(fresh); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	// Post-condition #1: the task is still classified as workflow — so it
	// keeps appearing in GetAllTasks / board / search results.
	after := store.GetTask("WF0001")
	if after == nil {
		t.Fatal("WF0001 missing after update")
	}
	if !after.IsWorkflow {
		t.Error("IsWorkflow was demoted to false after UpdateTask — task would vanish from board views")
	}

	// Post-condition #2: GetAllTasks returns the task. This is the actual
	// user-visible regression, not just the flag.
	all := store.GetAllTasks()
	found := false
	for _, tk := range all {
		if tk.ID == "WF0001" {
			found = true
			break
		}
	}
	if !found {
		t.Error("WF0001 missing from GetAllTasks after UpdateTask with fresh Task value")
	}
}
