package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

// TestRecursiveLoad_PicksUpNestedDocuments verifies the Phase 2 contract:
// the store scans the document root recursively and treats `.md` files in
// subdirectories as first-class documents.
//
// The previous flat `os.ReadDir` implementation would have returned only the
// top-level file and silently ignored anything beneath the root — a regression
// this test explicitly guards against.
func TestRecursiveLoad_PicksUpNestedDocuments(t *testing.T) {
	root := t.TempDir()

	writeDoc(t, filepath.Join(root, "AAAAAA.md"), "AAAAAA", "flat doc")
	writeDoc(t, filepath.Join(root, "tiki", "BBBBBB.md"), "BBBBBB", "legacy-layout doc")
	writeDoc(t, filepath.Join(root, "docs", "deep", "CCCCCC.md"), "CCCCCC", "deeply nested doc")

	store, err := NewTikiStore(root)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	for _, id := range []string{"AAAAAA", "BBBBBB", "CCCCCC"} {
		if task := store.GetTask(id); task == nil {
			t.Errorf("expected document %s to load, but it was missing", id)
		}
	}

	// Sanity check: the path index must reflect the actual on-disk layout,
	// not an id-derived default. A consumer of PathForID relies on this to
	// edit, delete, or git-stage the correct file.
	if got := store.PathForID("BBBBBB"); !pathEndsWith(got, filepath.Join("tiki", "BBBBBB.md")) {
		t.Errorf("PathForID(BBBBBB) = %q; want path ending in tiki/BBBBBB.md", got)
	}
	if got := store.PathForID("CCCCCC"); !pathEndsWith(got, filepath.Join("docs", "deep", "CCCCCC.md")) {
		t.Errorf("PathForID(CCCCCC) = %q; want path ending in docs/deep/CCCCCC.md", got)
	}
}

// TestRecursiveLoad_ExcludesProjectConfigFiles verifies that the reserved
// top-level config files are never misread as documents, even if a future
// version switches them to `.md`. `config.yaml` / `workflow.yaml` already
// fail the extension check; `config.md` / `workflow.md` are belt-and-braces.
func TestRecursiveLoad_ExcludesProjectConfigFiles(t *testing.T) {
	root := t.TempDir()

	writeDoc(t, filepath.Join(root, "AAAAAA.md"), "AAAAAA", "ok")
	// These four files must all be ignored by the walker.
	writeRaw(t, filepath.Join(root, "config.yaml"), "foo: bar\n")
	writeRaw(t, filepath.Join(root, "workflow.yaml"), "statuses: []\n")
	writeRaw(t, filepath.Join(root, "config.md"), "---\nid: ZZZZZZ\n---\nshould-be-ignored\n")
	writeRaw(t, filepath.Join(root, "workflow.md"), "---\nid: YYYYYY\n---\nshould-be-ignored\n")

	store, err := NewTikiStore(root)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Only AAAAAA should be loaded — the two bogus `.md` config files must be
	// skipped even though they carry valid-looking frontmatter. A failure
	// here would mean a user's `config.md` gets promoted to a workflow doc.
	if got := len(store.GetAllTasks()); got != 1 {
		t.Errorf("unexpected task count after load: got %d, want 1", got)
	}
	if store.GetTask("AAAAAA") == nil {
		t.Error("AAAAAA missing after load")
	}
	if store.GetTask("ZZZZZZ") != nil {
		t.Error("config.md incorrectly loaded as ZZZZZZ")
	}
	if store.GetTask("YYYYYY") != nil {
		t.Error("workflow.md incorrectly loaded as YYYYYY")
	}
}

// TestRecursiveLoad_SkipsHiddenDirectories verifies editor/VCS metadata
// directories (.git, .idea, .obsidian, etc.) don't get walked. A stray
// `.obsidian/cache/ABCDEF.md` must not end up in the document index.
func TestRecursiveLoad_SkipsHiddenDirectories(t *testing.T) {
	root := t.TempDir()

	writeDoc(t, filepath.Join(root, "AAAAAA.md"), "AAAAAA", "ok")
	writeDoc(t, filepath.Join(root, ".obsidian", "ZZZZZZ.md"), "ZZZZZZ", "hidden")
	writeDoc(t, filepath.Join(root, ".git", "YYYYYY.md"), "YYYYYY", "hidden")

	store, err := NewTikiStore(root)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	if store.GetTask("ZZZZZZ") != nil {
		t.Error("hidden .obsidian doc was loaded; expected directory to be skipped")
	}
	if store.GetTask("YYYYYY") != nil {
		t.Error("hidden .git doc was loaded; expected directory to be skipped")
	}
	if store.GetTask("AAAAAA") == nil {
		t.Error("visible doc unexpectedly missing")
	}
}

// TestRecursiveLoad_NewTaskLandsAtDocRoot verifies the Phase 2 save default:
// a brand-new document written via CreateTask ends up directly under the
// scan root (`<root>/<ID>.md`), not under a `tiki/` subdirectory. The
// recursive walker must then find it on reload.
func TestRecursiveLoad_NewTaskLandsAtDocRoot(t *testing.T) {
	root := t.TempDir()
	store, err := NewTikiStore(root)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	task := &taskpkg.Task{
		ID:       "EEEEEE",
		Title:    "new doc",
		Type:     taskpkg.TypeStory,
		Status:   "backlog",
		Priority: 3,
	}
	if err := store.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	want := filepath.Join(root, "EEEEEE.md")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected new doc at %s, stat error: %v", want, err)
	}

	// Reloading should rediscover the new file through the recursive walk.
	if err := store.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if store.GetTask("EEEEEE") == nil {
		t.Error("new doc missing after reload; recursive walker failed to find it")
	}
}

// TestRecursiveLoad_NestedSavePreservesPath guards the rename-is-identity
// invariant: once a document is loaded from a nested path, saves go back to
// that same path rather than the id-derived default. Without this, moving a
// file into a subfolder would silently create a duplicate at the root on
// the next edit.
func TestRecursiveLoad_NestedSavePreservesPath(t *testing.T) {
	root := t.TempDir()
	nestedPath := filepath.Join(root, "projects", "alpha", "FFFFFF.md")
	writeDoc(t, nestedPath, "FFFFFF", "original body")

	store, err := NewTikiStore(root)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	task := store.GetTask("FFFFFF")
	if task == nil {
		t.Fatal("FFFFFF missing after load")
	}
	task.Title = "updated"
	if err := store.UpdateTask(task); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	// The original nested file must still be the only copy; no duplicate
	// should have been created at <root>/FFFFFF.md.
	if _, err := os.Stat(nestedPath); err != nil {
		t.Errorf("nested file disappeared after save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "FFFFFF.md")); !os.IsNotExist(err) {
		t.Errorf("duplicate created at root; save ignored FilePath: err=%v", err)
	}
}

// TestPhase10_PathForIDReflectsFileMove proves the invariant the plan calls
// out: a `[[ID]]` link must survive a file move. The rewriter consults
// PathForID each time, so what really needs to work is the store tracking
// the current on-disk path after a file is moved and the store reloaded.
//
// Without this, a moved document would still resolve to its stale original
// path, and any consumer that reads or stages the file via PathForID would
// silently hit the wrong location.
func TestPhase10_PathForIDReflectsFileMove(t *testing.T) {
	root := t.TempDir()
	origDir := filepath.Join(root, "projects", "old")
	origPath := filepath.Join(origDir, "MOVEDD.md")
	writeDoc(t, origPath, "MOVEDD", "movable doc")

	store, err := NewTikiStore(root)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	if !pathEndsWith(store.PathForID("MOVEDD"), filepath.Join("projects", "old", "MOVEDD.md")) {
		t.Fatalf("initial PathForID wrong: %q", store.PathForID("MOVEDD"))
	}

	// Simulate the user moving the file on disk (e.g. via `git mv` or a
	// file manager) — same id, new location, no content change.
	newDir := filepath.Join(root, "projects", "new", "nested")
	newPath := filepath.Join(newDir, "MOVEDD.md")
	//nolint:gosec // G301: 0755 matches the rest of the test suite's temp-dir perms
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatalf("mkdir new: %v", err)
	}
	if err := os.Rename(origPath, newPath); err != nil {
		t.Fatalf("rename: %v", err)
	}

	if err := store.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	got := store.PathForID("MOVEDD")
	if !pathEndsWith(got, filepath.Join("projects", "new", "nested", "MOVEDD.md")) {
		t.Errorf("PathForID did not reflect move; got %q, want path ending in projects/new/nested/MOVEDD.md", got)
	}
	// Stale path must not still be returned — a consumer that trusts it
	// would edit/stage a file that no longer exists.
	if strings.Contains(got, filepath.Join("projects", "old")) {
		t.Errorf("PathForID still points to pre-move location: %q", got)
	}
}

// writeDoc writes a minimal workflow doc — id + title + status so it passes
// strict load and counts as workflow-capable.
func writeDoc(t *testing.T, path, id, title string) {
	t.Helper()
	//nolint:gosec // G301: 0755 matches the rest of the test suite's temp-dir perms
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	content := "---\nid: " + id + "\ntitle: " + title + "\ntype: story\nstatus: backlog\n---\nbody\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// writeRaw writes arbitrary content — for config-file exclusion tests where
// the frontmatter shape is intentionally varied.
func writeRaw(t *testing.T, path, content string) {
	t.Helper()
	//nolint:gosec // G301: 0755 matches the rest of the test suite's temp-dir perms
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func pathEndsWith(full, suffix string) bool {
	return len(full) >= len(suffix) && full[len(full)-len(suffix):] == suffix
}
