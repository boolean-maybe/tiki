package tikistore

import (
	"os"
	"strings"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestCreateTiki_BareTikiWritesNoSchemaKeys verifies that a tiki created
// with only id/title/body — no entries in Fields — is serialized without
// any schema-known frontmatter keys. If they leaked into the file, a
// reload would inject schema-known fields that were never set.
func TestCreateTiki_BareTikiWritesNoSchemaKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tk := tikipkg.New()
	tk.ID = "BARE01"
	tk.Title = "bare doc"
	tk.Body = "hello"
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	data, err := os.ReadFile(tmp + "/BARE01.md")
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	for _, forbidden := range []string{"status:", "type:", "priority:", "points:"} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("bare doc file contains %q schema-known key; full contents:\n%s",
				forbidden, data)
		}
	}

	// Reloading must keep the same shape — no schema-known fields invented.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	loaded := s.GetTiki("BARE01")
	if loaded == nil {
		t.Fatal("GetTiki after Reload = nil")
	}
	if hasAnySchemaField(loaded) {
		t.Error("bare doc gained schema-known fields on reload — keys must not have been written to disk")
	}
}

// TestUpdateTiki_OmittingSchemaFieldsRemovesThem verifies that exact-presence
// semantics let a caller drop every schema-known field by passing a tiki
// whose Fields map omits them. After the update, the file must not carry
// the dropped keys, and a reload reflects the same shape.
func TestUpdateTiki_OmittingSchemaFieldsRemovesThem(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Create with schema-known fields.
	tk := tikipkg.New()
	tk.ID = "SHRINK"
	tk.Title = "starts with status"
	tk.Set(tikipkg.FieldStatus, "backlog")
	tk.Set(tikipkg.FieldPriority, 2)
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki with schema fields: %v", err)
	}

	// Confirm starting state.
	before := s.GetTiki("SHRINK")
	if before == nil || !hasAnySchemaField(before) {
		t.Fatalf("precondition: expected schema fields present, got %+v", before)
	}

	// Update with a tiki that has no schema-known fields. Exact-presence
	// removes them.
	bare := tikipkg.New()
	bare.ID = "SHRINK"
	bare.Title = "starts with status"
	bare.Path = before.Path
	bare.LoadedMtime = before.LoadedMtime
	if err := s.UpdateTiki(bare); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	after := s.GetTiki("SHRINK")
	if after == nil {
		t.Fatal("GetTiki after UpdateTiki = nil")
	}
	if hasAnySchemaField(after) {
		t.Error("UpdateTiki failed to drop schema-known fields under exact-presence")
	}

	// And it must persist across a reload — the keys are gone from disk too.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	reloaded := s.GetTiki("SHRINK")
	if reloaded == nil {
		t.Fatal("reloaded doc missing")
	}
	if hasAnySchemaField(reloaded) {
		t.Error("schema-known fields survived reload — they must have been written back to disk")
	}
}

// TestUpdateTiki_ClonedStoredTikiPreservesFields verifies the common
// caller pattern: load with GetTiki, Clone, modify, UpdateTiki. Schema-
// known fields are preserved because the caller carried them in the
// Fields map of the cloned value — not because the store synthesizes
// anything.
func TestUpdateTiki_ClonedStoredTikiPreservesFields(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tk := tikipkg.New()
	tk.ID = "CARRY1"
	tk.Title = "carries status"
	tk.Set(tikipkg.FieldStatus, "backlog")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	// Load, clone, and update — schema fields are carried because the
	// caller took them from the stored tiki.
	stored := s.GetTiki("CARRY1")
	if stored == nil {
		t.Fatal("GetTiki = nil")
	}
	updated := stored.Clone()
	updated.Title = "updated"
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	if got := s.GetTiki("CARRY1"); got == nil || !hasAnySchemaField(got) {
		t.Errorf("UpdateTiki should preserve schema-known fields when the caller cloned the stored tiki; got %+v", got)
	}
}
