package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

// TestPhase3_SaveRejectsUnregisteredCustomField pins the third Phase-3
// review finding: a caller that stages a tiki Field entry whose key is
// a registered Custom field but with an invalid value must fail the save.
// This test now operates at the tiki level via UpdateTiki.
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

	// Seed a tiki on disk so UpdateTiki has something to target.
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
	loaded := s.GetTiki("BOGUS1")
	if loaded == nil {
		t.Fatal("GetTiki = nil")
	}

	// Stage an unregistered field. At the tiki level, unregistered fields
	// pass through as UnknownFields at save time — this is intentional for
	// round-trip fidelity. The test now verifies that a registered Custom
	// field with an INVALID value for its type is rejected.
	updated := loaded.Clone()
	updated.Set("severity", "not-a-valid-enum") // valid key, invalid value

	// UpdateTiki invokes marshalTikiFrontmatter which validates custom fields.
	err = s.UpdateTiki(updated)
	if err == nil {
		t.Fatal("UpdateTiki should have rejected invalid enum value for registered custom field, got nil error")
	}
	if !strings.Contains(err.Error(), "severity") {
		t.Errorf("error does not mention offending key: %v", err)
	}
}

// TestPhase3_SaveRejectsInvalidListValueInCustomFields pins the fourth
// Phase-3 review finding: a tiki Field entry whose key IS a registered
// Custom list field but whose VALUE has a wrong shape (e.g. a scalar where
// a list is required) must fail the save.
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
	loaded := s.GetTiki("BADLST")
	if loaded == nil {
		t.Fatal("GetTiki = nil")
	}

	// Stage a value of the wrong shape for a registered list field.
	updated := loaded.Clone()
	updated.Set("deps", 42) // int, not a list

	err = s.UpdateTiki(updated)
	if err == nil {
		t.Fatal("UpdateTiki should have rejected invalid list value, got nil error")
	}
	if !strings.Contains(err.Error(), "deps") {
		t.Errorf("error does not mention offending key: %v", err)
	}
}

// TestPhase3_StaleListFieldRoundTrips pins the first Phase-3 review finding:
// a stale value for a registered custom list field must survive a load →
// save → load cycle. The stale value is demoted to UnknownFields on load
// and must be preserved as-is through a tiki-native UpdateTiki save.
func TestPhase3_StaleListFieldRoundTrips(t *testing.T) {
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

	loaded := s.GetTiki("STALE3")
	if loaded == nil {
		t.Fatal("GetTiki after load = nil")
	}
	// The stale value should NOT be in Fields as a proper list.
	if v, ok := loaded.Fields["deps"]; ok {
		if _, isList := v.([]string); isList {
			t.Errorf("stale list value was coerced into Fields as list — expected UnknownFields demote")
		}
	}

	// Trigger a save by editing an unrelated field (title) via tiki clone.
	updated := loaded.Clone()
	updated.Title = "Stale list v2"
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(saved), "deps: nope-not-a-list") {
		t.Errorf("stale deps value lost on round-trip; file:\n%s", saved)
	}

	// Reload a second time and confirm the stale value survives.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	after := s.GetTiki("STALE3")
	if after == nil {
		t.Fatal("GetTiki after second reload = nil")
	}
	if v, ok := after.Fields["deps"]; ok {
		if _, isList := v.([]string); isList {
			t.Errorf("post-round-trip: stale list leaked into Fields as proper list")
		}
	}
}

// TestPhase3_CreateTikiPreservesSchemaFieldsAcrossReload verifies that a
// schema-known field set on creation survives a save → load round-trip.
// Callers that want presence of a schema-known field must set it explicitly;
// the store does not synthesize one.
func TestPhase3_CreateTikiPreservesSchemaFieldsAcrossReload(t *testing.T) {
	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	wf := tikipkg.New()
	wf.ID = "WFZERO"
	wf.Title = "carries status"
	wf.Set("status", "ready")
	if err := s.CreateTiki(wf); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	tk := s.GetTiki("WFZERO")
	if tk == nil {
		t.Fatal("GetTiki after reload = nil")
	}
	if !hasAnySchemaField(tk) {
		t.Errorf("round-trip dropped the schema-known field set on create — file would no longer match has(status) filters")
	}
}
