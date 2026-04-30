package tikistore

import (
	"strings"
	"testing"

	taskpkg "github.com/boolean-maybe/tiki/task"
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
	target := &taskpkg.Task{ID: "AAAAAA", Title: "target", Status: "ready"}
	if err := s.CreateTask(target); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	dependent := &taskpkg.Task{
		ID: "BBBBBB", Title: "dependent", Status: "ready",
		DependsOn: []string{"TIKI-AAA"},
	}
	err = s.CreateTask(dependent)
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
	target := &taskpkg.Task{ID: "AAAAAA", Title: "target", Status: "ready"}
	if err := s.CreateTask(target); err != nil {
		t.Fatalf("seed: %v", err)
	}

	dependent := &taskpkg.Task{
		ID: "BBBBBB", Title: "dependent", Status: "ready",
		DependsOn: []string{"AAAAAA"},
	}
	if err := s.CreateTask(dependent); err != nil {
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
	orphan := &taskpkg.Task{
		ID: "BBBBBB", Title: "orphan", Status: "ready",
		DependsOn: []string{"ZZZZZZ"}, // well-formed but not seeded
	}
	err = s.CreateTask(orphan)
	if err == nil {
		t.Fatal("expected error for missing dependsOn target")
	}
	if !strings.Contains(err.Error(), "non-existent document") {
		t.Fatalf("expected missing-doc error, got: %v", err)
	}
}
