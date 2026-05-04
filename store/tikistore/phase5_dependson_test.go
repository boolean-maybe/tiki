package tikistore

import (
	"strings"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestPhase5_DependsOnRejectsNonBareID proves that validateDependsOnLocked
// rejects legacy TIKI-prefixed IDs even when a document of that id happens
// to exist in the index. Phase 5 makes bare IDs the only valid form; a
// dependency on `TIKI-AAA` must fail at the store boundary so stale docs
// or hand-edited frontmatter cannot silently re-introduce the legacy format.
func TestPhase5_DependsOnRejectsNonBareID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	// Seed a valid target so the only possible failure is the format check.
	target := tikipkg.New()
	target.ID = "AAAAAA"
	target.Title = "target"
	target.Set("status", "ready")
	if err := s.CreateTiki(target); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	dependent := tikipkg.New()
	dependent.ID = "BBBBBB"
	dependent.Title = "dependent"
	dependent.Set("status", "ready")
	dependent.Set("dependsOn", []string{"TIKI-AAA"})
	err = s.CreateTiki(dependent)
	if err == nil {
		t.Fatal("expected error for non-bare dependsOn id")
	}
	if !strings.Contains(err.Error(), "bare document id") {
		t.Fatalf("expected bare-id error, got: %v", err)
	}
}

// TestPhase5_DependsOnAcceptsBareID proves a bare-ID dependency resolves
// to an existing document regardless of workflow status. Any loaded doc
// is a valid target per Phase 5.
func TestPhase5_DependsOnAcceptsBareID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	target := tikipkg.New()
	target.ID = "AAAAAA"
	target.Title = "target"
	target.Set("status", "ready")
	if err := s.CreateTiki(target); err != nil {
		t.Fatalf("seed: %v", err)
	}

	dependent := tikipkg.New()
	dependent.ID = "BBBBBB"
	dependent.Title = "dependent"
	dependent.Set("status", "ready")
	dependent.Set("dependsOn", []string{"AAAAAA"})
	if err := s.CreateTiki(dependent); err != nil {
		t.Fatalf("create dependent: %v", err)
	}
}

// TestPhase5_DependsOnMissingTargetRejected makes sure the non-existent-
// document error still fires for well-formed bare IDs that have no match
// in the store index.
func TestPhase5_DependsOnMissingTargetRejected(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	orphan := tikipkg.New()
	orphan.ID = "BBBBBB"
	orphan.Title = "orphan"
	orphan.Set("status", "ready")
	orphan.Set("dependsOn", []string{"ZZZZZZ"}) // well-formed but not seeded
	err = s.CreateTiki(orphan)
	if err == nil {
		t.Fatal("expected error for missing dependsOn target")
	}
	if !strings.Contains(err.Error(), "non-existent document") {
		t.Fatalf("expected missing-doc error, got: %v", err)
	}
}
