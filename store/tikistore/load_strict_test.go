package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
)

func init() {
	// every test in this file assumes workflow registries have been touched;
	// MarkRegistriesLoadedForTest is the existing helper used elsewhere.
	config.MarkRegistriesLoadedForTest()
}

// TestLoadTaskFile_MissingIDIsHardError verifies the new strict-load contract:
// a file without frontmatter id: must refuse to load. The remedy is
// `tiki repair ids --fix`, which our error message points at.
func TestLoadTaskFile_MissingIDIsHardError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-id.md")
	if err := os.WriteFile(path, []byte("---\ntitle: plain\n---\nbody\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	if tasks := s.GetAllTasks(); len(tasks) != 0 {
		t.Errorf("expected 0 loaded tasks, got %d — file without id must not load", len(tasks))
	}
	// file must remain untouched: no migration-on-load.
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "id:") {
		t.Errorf("load mutated the file (added id): %s", got)
	}
}

// TestLoadTaskFile_LegacyIDIsHardError verifies that TIKI-XXXXXX ids are
// rejected: the compatibility layer is gone, only the repair command knows
// how to migrate them.
func TestLoadTaskFile_LegacyIDIsHardError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.md")
	if err := os.WriteFile(path, []byte("---\nid: TIKI-ABC123\ntitle: legacy\ntype: story\nstatus: ready\npriority: 1\n---\nbody\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	// legacy id must not load under any lookup shape.
	if tk := s.GetTask("TIKI-ABC123"); tk != nil {
		t.Error("legacy id should have been rejected at load")
	}
	if tk := s.GetTask("ABC123"); tk != nil {
		t.Error("legacy id should not strip to a bare id at load")
	}
	// file must remain byte-for-byte unchanged.
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "id: TIKI-ABC123") {
		t.Errorf("load mutated legacy file: %s", got)
	}
}

// TestLoadTaskFile_DuplicateIDSkipped verifies that two files with the same
// id don't silently overwrite each other. Exactly one wins (the first
// encountered in directory iteration order).
func TestLoadTaskFile_DuplicateIDSkipped(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.md")
	second := filepath.Join(dir, "b.md")
	content := func(title string) string {
		return "---\nid: DUPLIC\ntitle: " + title + "\ntype: story\nstatus: ready\npriority: 1\n---\nbody\n"
	}
	if err := os.WriteFile(first, []byte(content("first")), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(second, []byte(content("second")), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	// exactly one task should be registered under DUPLIC.
	tk := s.GetTask("DUPLIC")
	if tk == nil {
		t.Fatal("expected one task under duplicate id, got none")
	}
	if len(s.GetAllTasks()) != 1 {
		t.Errorf("expected exactly 1 task loaded, got %d", len(s.GetAllTasks()))
	}
	// both files are still on disk (we do not delete).
	if _, err := os.Stat(first); err != nil {
		t.Errorf("first file removed: %v", err)
	}
	if _, err := os.Stat(second); err != nil {
		t.Errorf("second file removed: %v", err)
	}
}
