package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestUpdateTiki_FreshTikiPreservesLoadedPath verifies the L4 fix: a caller
// that passes a fresh Tiki value (empty Path) must not lose the loaded
// path. Update must target the on-disk location the tiki was loaded from,
// not the id-derived default.
func TestUpdateTiki_FreshTikiPreservesLoadedPath(t *testing.T) {
	dir := t.TempDir()
	renamed := filepath.Join(dir, "user-renamed.md")
	content := "---\nid: UPD001\ntitle: original\ntype: story\nstatus: ready\npriority: 1\n---\nbody\n"
	if err := os.WriteFile(renamed, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// caller constructs a fresh Tiki (empty Path, empty LoadedMtime) with
	// the same id and different content — simulating an API client that
	// didn't bother to preserve identity-bound state.
	fresh := tikipkg.New()
	fresh.ID = "UPD001"
	fresh.Title = "updated by fresh value"
	fresh.Set("type", "story")
	fresh.Set("status", "ready")
	fresh.Set("priority", 1)
	if err := s.UpdateTiki(fresh); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	// the update must land on the renamed file, not at <dir>/UPD001.md.
	updated, err := os.ReadFile(renamed)
	if err != nil {
		t.Fatalf("read renamed: %v", err)
	}
	if !strings.Contains(string(updated), "updated by fresh value") {
		t.Errorf("fresh-value update did not reach loaded path: %s", updated)
	}
	if _, err := os.Stat(filepath.Join(dir, "UPD001.md")); err == nil {
		t.Error("fresh-value update wrote a duplicate at id-derived path — Path was not carried over")
	}
}

// TestUpdateTiki_FreshTikiPreservesLoadedMtime verifies the companion L4 fix:
// LoadedMtime must also survive a fresh-Tiki update so the optimistic
// locking check still runs. Demonstrates this by racing a save against an
// external mtime change.
func TestUpdateTiki_FreshTikiPreservesLoadedMtime(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "LOCK01.md")
	content := "---\nid: LOCK01\ntitle: original\ntype: story\nstatus: ready\npriority: 1\n---\nbody\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// simulate external edit by setting mtime in the future; the loaded tiki
	// has the original mtime. A fresh-Tiki update must still detect the
	// conflict because LoadedMtime is carried from the stored tiki.
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	future := info.ModTime().Add(10 * 1000 * 1000 * 1000) // +10s
	if err := os.Chtimes(filePath, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	fresh := tikipkg.New()
	fresh.ID = "LOCK01"
	fresh.Title = "try to update"
	fresh.Set("type", "story")
	fresh.Set("status", "ready")
	fresh.Set("priority", 1)
	// Path + LoadedMtime deliberately left zero.
	err = s.UpdateTiki(fresh)
	if err == nil {
		t.Error("expected ErrConflict on fresh-Tiki update against externally-modified file")
	} else if err.Error() == "" {
		t.Error("conflict error had empty message")
	}
}
