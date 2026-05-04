package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

// TestPhase3_SaveRejectsUnregisteredCustomField pins the third Phase-3
// review finding: a caller that stages a CustomFields entry whose key is
// not a registered Custom field must fail the save — matching the
// pre-Phase-3 appendCustomFields contract ("unknown custom field %q").
// After the merged-Fields refactor it was silently persisted as if it were
// an unknown-key pass-through, masking programming errors and letting
// garbage land on disk under the CustomFields heading.
func TestPhase3_SaveRejectsUnregisteredCustomField(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "high"}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Seed a workflow task on disk so UpdateTask has something to target.
	seed := `---
id: BOGUS1
title: v1
status: ready
---
body
`
	if err := os.WriteFile(filepath.Join(tmp, "BOGUS1.md"), []byte(seed), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	loaded := s.GetTask("BOGUS1")
	if loaded == nil {
		t.Fatal("GetTask = nil")
	}

	// Stage a CustomFields entry for a key that is NOT a registered Custom
	// field. Pre-fix: the save succeeds and writes `not_registered:
	// garbage` to disk. Post-fix: UpdateTask errors.
	if loaded.CustomFields == nil {
		loaded.CustomFields = map[string]interface{}{}
	}
	loaded.CustomFields["not_registered"] = "garbage"

	err = s.UpdateTask(loaded)
	if err == nil {
		t.Fatal("UpdateTask should have rejected unregistered custom field, got nil error")
	}
	if !strings.Contains(err.Error(), "not_registered") {
		t.Errorf("error does not mention offending key: %v", err)
	}
}

// TestPhase3_SaveRejectsInvalidListValueInCustomFields pins the fourth
// Phase-3 review finding: a caller-staged CustomFields entry whose key IS
// a registered Custom list field but whose VALUE has a wrong shape (e.g.
// a scalar where a list is required) must fail the save. Matches the
// pre-Phase-3 appendCustomFields path which returned
// `invalid list value for %q: ...`.
//
// This is distinct from loaded UnknownFields — a stale value pulled off
// disk and demoted at load time must still round-trip verbatim (see
// TestPhase3_StaleListFieldRoundTrips). Validation applies only to caller-
// populated CustomFields.
func TestPhase3_SaveRejectsInvalidListValueInCustomFields(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "deps", Type: workflow.TypeListString},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	seed := `---
id: BADLST
title: v1
status: ready
---
body
`
	if err := os.WriteFile(filepath.Join(tmp, "BADLST.md"), []byte(seed), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	loaded := s.GetTask("BADLST")
	if loaded == nil {
		t.Fatal("GetTask = nil")
	}

	// Stage a value of the wrong shape for a registered list field.
	if loaded.CustomFields == nil {
		loaded.CustomFields = map[string]interface{}{}
	}
	loaded.CustomFields["deps"] = 42 // int, not a list

	err = s.UpdateTask(loaded)
	if err == nil {
		t.Fatal("UpdateTask should have rejected invalid list value, got nil error")
	}
	if !strings.Contains(err.Error(), "deps") {
		t.Errorf("error does not mention offending key: %v", err)
	}
}

// TestPhase3_StaleListFieldRoundTrips pins the first Phase-3 review finding:
// a stale value for a registered custom list field must survive a load →
// save → load cycle. Pre-fix: load demoted the value to UnknownFields,
// taskToTiki merged it back into Fields, and marshalTikiFrontmatter then
// re-validated it as a TypeListString and erased the bad entry. The old
// appendUnknownFields path wrote such values verbatim; the new bridge must
// preserve the same round-trip guarantee.
func TestPhase3_StaleListFieldRoundTrips(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	// Register `deps` as a list-of-string custom field. The file below
	// will contain `deps: "nope-not-a-list"` — a scalar value that fails
	// the TypeListString coercer but must survive because the user may
	// want to repair it by hand.
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "deps", Type: workflow.TypeListString},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	initial := `---
id: STALE3
title: Stale list
status: ready
deps: nope-not-a-list
---
body
`
	path := filepath.Join(tmp, "STALE3.md")
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	loaded := s.GetTask("STALE3")
	if loaded == nil {
		t.Fatal("GetTask after load = nil")
	}
	if _, inCustom := loaded.CustomFields["deps"]; inCustom {
		t.Errorf("stale list value leaked into CustomFields")
	}
	if got := loaded.UnknownFields["deps"]; got != "nope-not-a-list" {
		t.Errorf("UnknownFields[deps] = %v, want 'nope-not-a-list'", got)
	}

	// Trigger a save by editing an unrelated field (title).
	loaded.Title = "Stale list v2"
	if err := s.UpdateTask(loaded); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(saved), "deps: nope-not-a-list") {
		t.Errorf("stale deps value lost on round-trip; file:\n%s", saved)
	}

	// Reload a second time and confirm it still demotes to UnknownFields.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	after := s.GetTask("STALE3")
	if after == nil {
		t.Fatal("GetTask after second reload = nil")
	}
	if _, inCustom := after.CustomFields["deps"]; inCustom {
		t.Errorf("post-round-trip: stale list leaked into CustomFields")
	}
	if got := after.UnknownFields["deps"]; got != "nope-not-a-list" {
		t.Errorf("post-round-trip: UnknownFields[deps] = %v, want 'nope-not-a-list'", got)
	}
}

// TestPhase3_CreateTaskWithZeroValueWorkflowPreservesClassification pins the
// second review finding: CreateTask with IsWorkflow=true but no typed
// defaults must produce a file that reloads as workflow. derivedWorkflowPresence
// used to emit only non-zero fields, so an in-memory Task{IsWorkflow:true,
// Title: "x"} would serialize to just id+title and reload as plain.
//
// The fix: in-memory workflow tasks without a preserved presence set fall
// back to emitting every schema-known key so at least one workflow marker
// lands on disk and classification survives the round-trip.
func TestPhase3_CreateTaskWithZeroValueWorkflowPreservesClassification(t *testing.T) {
	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	incoming := &taskpkg.Task{
		ID:         "WFZERO",
		Title:      "workflow with no typed values",
		IsWorkflow: true,
	}
	if err := s.CreateTask(incoming); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	tk := s.GetTask("WFZERO")
	if tk == nil {
		t.Fatal("GetTask after reload = nil")
	}
	if !tk.IsWorkflow {
		t.Errorf("round-trip demoted IsWorkflow=true task to plain; would vanish from GetAllTasks()")
	}
}
