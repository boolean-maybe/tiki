package tikistore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMoveThenReload_PreservesFieldMapAndPath verifies the cross-cut
// invariant for Phase 7: a tiki whose file is moved on disk between two
// loads round-trips its full field map AND its new on-disk location.
// The store does not classify on load and does not synthesize fields, so
// the post-move tiki is byte-identical (id/title/Fields) to the pre-move
// tiki — and saving an edit later writes to the new location.
func TestMoveThenReload_PreservesFieldMapAndPath(t *testing.T) {
	dir := t.TempDir()

	original := filepath.Join(dir, "MV0001.md")
	src := "---\nid: MV0001\ntitle: travels\nstatus: backlog\npriority: 2\n---\nbody\n"
	if err := os.WriteFile(original, []byte(src), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	before := s.GetTiki("MV0001")
	if before == nil {
		t.Fatal("GetTiki MV0001 = nil pre-move")
	}
	beforeStatus, _, _ := before.StringField("status")
	beforePriority, _, _ := before.IntField("priority")

	// Move the file to a nested location.
	nested := filepath.Join(dir, "subdir")
	//nolint:gosec // G301: 0o755 matches the rest of the test suite
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	moved := filepath.Join(nested, "renamed.md")
	if err := os.Rename(original, moved); err != nil {
		t.Fatalf("rename: %v", err)
	}

	// Reload picks up the new location and the same field map.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	after := s.GetTiki("MV0001")
	if after == nil {
		t.Fatal("GetTiki MV0001 = nil post-move")
	}

	if afterStatus, _, _ := after.StringField("status"); afterStatus != beforeStatus {
		t.Errorf("status drift across move: %q → %q", beforeStatus, afterStatus)
	}
	if afterPriority, _, _ := after.IntField("priority"); afterPriority != beforePriority {
		t.Errorf("priority drift across move: %d → %d", beforePriority, afterPriority)
	}

	// And the loaded path now points at the moved file — a follow-up edit
	// must land there, not at the original location.
	if !strings.HasSuffix(after.Path, "subdir"+string(filepath.Separator)+"renamed.md") {
		t.Errorf("loaded Path did not track the move: %q", after.Path)
	}
	updated := after.Clone()
	updated.Title = "post-move title"
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki post-move: %v", err)
	}

	data, err := os.ReadFile(moved)
	if err != nil {
		t.Fatalf("read moved: %v", err)
	}
	if !strings.Contains(string(data), "title: post-move title") {
		t.Errorf("update did not target moved path; contents:\n%s", data)
	}
	if _, err := os.Stat(original); !os.IsNotExist(err) {
		t.Errorf("update wrote a duplicate at the pre-move path: %v", err)
	}
}

// TestUpdateTiki_DetectsExternalEditViaOptimisticLock verifies the
// optimistic locking contract end-to-end: an external write that bumps
// the file mtime between load and save must cause UpdateTiki to fail
// with ErrConflict, leaving the in-flight tiki value untouched on disk.
func TestUpdateTiki_DetectsExternalEditViaOptimisticLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "LOK001.md")
	src := "---\nid: LOK001\ntitle: original\nstatus: backlog\n---\nbody\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	loaded := s.GetTiki("LOK001")
	if loaded == nil {
		t.Fatal("GetTiki = nil")
	}

	// External edit: rewrite the file and bump mtime.
	externalSrc := "---\nid: LOK001\ntitle: clobbered externally\nstatus: backlog\n---\noutside body\n"
	if err := os.WriteFile(path, []byte(externalSrc), 0o644); err != nil {
		t.Fatalf("external rewrite: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	future := info.ModTime().Add(10_000_000_000) // +10s, well beyond fs granularity
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	updated := loaded.Clone()
	updated.Title = "in-flight overwrite attempt"
	err = s.UpdateTiki(updated)
	if err == nil {
		t.Fatal("UpdateTiki should have detected external edit and failed")
	}
	if !errors.Is(err, ErrConflict) {
		t.Errorf("UpdateTiki returned %v, want ErrConflict", err)
	}

	// The external content survived — our in-flight overwrite did not land.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after conflict: %v", err)
	}
	if !strings.Contains(string(data), "clobbered externally") {
		t.Errorf("external content was overwritten despite the conflict: %s", data)
	}
}
