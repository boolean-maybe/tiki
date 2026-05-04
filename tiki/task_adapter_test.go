package tiki

import (
	"testing"

	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

// TestToTask_RoutesRegisteredCustomToCustomFields pins the fix for the
// Phase-4 review finding that ToTask had been sending every non-schema
// Fields key into Task.CustomFields. Registered Custom fields go to
// CustomFields; everything else goes to UnknownFields — laundering an
// unregistered key through CustomFields would fail saveTask's
// validateCustomFields check on persistence.
func TestToTask_RoutesRegisteredCustomToCustomFields(t *testing.T) {
	t.Cleanup(func() { workflow.ClearCustomFields() })
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "high"}, Custom: true},
	}); err != nil {
		t.Fatalf("register custom field: %v", err)
	}

	tk := &Tiki{
		ID:    "ABC123",
		Title: "T",
		Fields: map[string]interface{}{
			"severity": "high",       // registered custom → CustomFields
			"legacy":   "round-trip", // unregistered → UnknownFields
			"status":   "ready",      // schema-known → typed field
		},
	}

	out := ToTask(tk)

	if got := out.CustomFields["severity"]; got != "high" {
		t.Errorf("CustomFields[severity] = %v, want high", got)
	}
	if _, ok := out.CustomFields["legacy"]; ok {
		t.Error("CustomFields[legacy] present; unregistered keys must land in UnknownFields")
	}
	if got := out.UnknownFields["legacy"]; got != "round-trip" {
		t.Errorf("UnknownFields[legacy] = %v, want round-trip", got)
	}
	if _, ok := out.UnknownFields["severity"]; ok {
		t.Error("UnknownFields[severity] present; registered custom keys must land in CustomFields")
	}
	if out.Status != "ready" {
		t.Errorf("Status = %q, want ready", out.Status)
	}
}

// TestToTask_SkipsSyntheticRegisteredBuiltins mirrors persistence behavior:
// workflow.Field entries marked Custom=false are synthetic built-ins
// (filepath, etc.) that live outside frontmatter and must not round-trip
// into either CustomFields or UnknownFields on save.
func TestToTask_SkipsSyntheticRegisteredBuiltins(t *testing.T) {
	t.Cleanup(func() { workflow.ClearCustomFields() })

	tk := &Tiki{
		ID:    "ABC123",
		Title: "T",
		Fields: map[string]interface{}{
			"filepath": "/some/path.md",
		},
	}

	out := ToTask(tk)
	if _, ok := out.CustomFields["filepath"]; ok {
		t.Error("CustomFields[filepath] present; synthetic built-ins must be dropped")
	}
	if _, ok := out.UnknownFields["filepath"]; ok {
		t.Error("UnknownFields[filepath] present; synthetic built-ins must be dropped")
	}
}

// TestFromTask_ThenToTask_PreservesStaleProvenance pins the provenance
// rule: a registered Custom key loaded as Task.UnknownFields (because its
// on-disk value failed coercion) must round-trip back to UnknownFields
// after ruki/plugin/trigger execution, not get promoted to CustomFields
// where it would fail validateCustomFields on save.
func TestFromTask_ThenToTask_PreservesStaleProvenance(t *testing.T) {
	t.Cleanup(func() { workflow.ClearCustomFields() })
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "high"}, Custom: true},
	}); err != nil {
		t.Fatalf("register custom field: %v", err)
	}

	// Persistence-shaped task: a stale severity value the loader demoted
	// to UnknownFields so the original bytes survive round-trip for repair.
	src := &task.Task{
		ID:            "ABC123",
		Title:         "T",
		UnknownFields: map[string]interface{}{"severity": "ultra"},
	}

	tk := FromTask(src)
	round := ToTask(tk)

	if _, ok := round.CustomFields["severity"]; ok {
		t.Error("stale severity promoted to CustomFields; would fail validateCustomFields on save")
	}
	if got := round.UnknownFields["severity"]; got != "ultra" {
		t.Errorf("UnknownFields[severity] = %v, want ultra (original bytes preserved)", got)
	}
}

// TestFromTask_ThenSet_ClearsStaleProvenance pins the complement: an
// explicit ruki write (Set) on a stale key is treated as a repair — the
// stale marker clears, so the next ToTask routes the key to CustomFields.
func TestFromTask_ThenSet_ClearsStaleProvenance(t *testing.T) {
	t.Cleanup(func() { workflow.ClearCustomFields() })
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "high"}, Custom: true},
	}); err != nil {
		t.Fatalf("register custom field: %v", err)
	}

	src := &task.Task{
		ID:            "ABC123",
		Title:         "T",
		UnknownFields: map[string]interface{}{"severity": "ultra"},
	}

	tk := FromTask(src)
	tk.Set("severity", "high") // ruki repair
	round := ToTask(tk)

	if got := round.CustomFields["severity"]; got != "high" {
		t.Errorf("repaired severity should land in CustomFields; got CustomFields[severity]=%v", got)
	}
	if _, ok := round.UnknownFields["severity"]; ok {
		t.Error("after repair, severity still in UnknownFields")
	}
}
