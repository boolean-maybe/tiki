package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReloadTask_IDChangeRemovesStaleEntry locks in the invariant that a
// ReloadTask whose file's frontmatter id has changed does not leave the
// old id pointing at the reloaded task. Without the fix, the map ends up
// with two entries — the stale one under the original id and the new one
// under the updated id — both referencing the same file.
func TestReloadTask_IDChangeRemovesStaleEntry(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ORIG01.md")
	writeWorkflowDoc(t, path, "ORIG01", "original")

	store, err := NewTikiStore(root)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	if store.GetTask("ORIG01") == nil {
		t.Fatal("precondition: ORIG01 must be loaded")
	}

	// Simulate an external edit that rewrites the id in frontmatter but
	// keeps the file at the same path (e.g. user ran a rename tool on
	// the id string).
	writeWorkflowDoc(t, path, "NEWID1", "renamed")
	if err := store.ReloadTask("ORIG01"); err != nil {
		t.Fatalf("ReloadTask: %v", err)
	}

	if got := store.GetTask("ORIG01"); got != nil {
		t.Error("stale entry under old id ORIG01 should have been removed")
	}
	if got := store.GetTask("NEWID1"); got == nil {
		t.Error("expected NEWID1 to be registered after reload")
	}
}

// TestReloadTask_IDChangeRefusesCollisionWithPeer guards the memory-map
// against a silent overwrite: if a file's frontmatter id is edited to an
// id already owned by a different task, ReloadTask must refuse to apply
// the change. Post-conditions verified:
//
//  1. an error is returned naming the collision (caller can surface it);
//  2. the peer entry (AAAAAA → AAAAAA.md) is preserved — no silent overwrite;
//  3. the stale old-id entry (BBBBBB) is gone — the file no longer claims
//     that id on disk, so keeping it in the map would be a lie. The next
//     full Reload will surface the underlying duplicate through diagnostics.
func TestReloadTask_IDChangeRefusesCollisionWithPeer(t *testing.T) {
	root := t.TempDir()
	aPath := filepath.Join(root, "AAAAAA.md")
	bPath := filepath.Join(root, "BBBBBB.md")
	writeWorkflowDoc(t, aPath, "AAAAAA", "a")
	writeWorkflowDoc(t, bPath, "BBBBBB", "b")

	store, err := NewTikiStore(root)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Rewrite B's id to A's id — a collision.
	writeWorkflowDoc(t, bPath, "AAAAAA", "b-collides")

	err = store.ReloadTask("BBBBBB")
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), "collides") {
		t.Errorf("error message should name the collision, got: %v", err)
	}

	// Peer preserved: AAAAAA still maps to AAAAAA.md, not to BBBBBB.md.
	if got := store.GetTask("AAAAAA"); got == nil {
		t.Fatal("peer AAAAAA must not be evicted by the rejected reload")
	} else if !strings.HasSuffix(got.FilePath, "AAAAAA.md") {
		t.Errorf("peer AAAAAA now points at wrong file: %s", got.FilePath)
	}

	// Stale old id gone: BBBBBB.md no longer has id BBBBBB on disk, so the
	// store must not keep reporting a task at that key. Otherwise a board
	// would render a ghost entry whose file has silently moved identity.
	if got := store.GetTask("BBBBBB"); got != nil {
		t.Errorf("stale BBBBBB entry should have been dropped on collision, got %+v", got)
	}
}

// TestReloadTask_SameIDUpdatesInPlace is the happy path: when the file's
// id is unchanged, ReloadTask updates the existing entry in place. This
// used to be the only behavior and must keep working.
func TestReloadTask_SameIDUpdatesInPlace(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "SAME01.md")
	writeWorkflowDoc(t, path, "SAME01", "original title")

	store, err := NewTikiStore(root)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// External edit: new title, same id.
	writeWorkflowDoc(t, path, "SAME01", "new title")
	if err := store.ReloadTask("SAME01"); err != nil {
		t.Fatalf("ReloadTask: %v", err)
	}

	task := store.GetTask("SAME01")
	if task == nil {
		t.Fatal("SAME01 missing after reload")
	}
	if task.Title != "new title" {
		t.Errorf("title not updated: got %q", task.Title)
	}
}

// writeWorkflowDoc overwrites path with a minimal workflow-capable doc
// carrying the given id and title.
func writeWorkflowDoc(t *testing.T, path, id, title string) {
	t.Helper()
	content := "---\nid: " + id + "\ntitle: " + title + "\ntype: story\nstatus: backlog\n---\nbody\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
