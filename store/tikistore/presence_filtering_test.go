package tikistore

import (
	"os"
	"path/filepath"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

func tikiWithSchemaFields() *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.ID = "CREAT1"
	tk.Title = "created"
	tk.Set("type", "story")
	tk.Set("status", "backlog")
	tk.Set("priority", 1)
	return tk
}

// TestGetAllTikis_ReturnsEverything verifies the store contract: GetAllTikis
// returns every loaded tiki, regardless of which fields it carries. Filtering
// by presence of schema-known fields belongs to the caller (e.g.
// `select where has(status)` in ruki, or a hasAnySchemaField predicate).
func TestGetAllTikis_ReturnsEverything(t *testing.T) {
	dir := t.TempDir()

	// Tiki with no schema-known fields — only id + title.
	bare := filepath.Join(dir, "BARE01.md")
	if err := os.WriteFile(bare, []byte("---\nid: BARE01\ntitle: just a doc\n---\n\n# A bare markdown doc\n"), 0o644); err != nil {
		t.Fatalf("write bare: %v", err)
	}

	// Tiki with schema-known fields explicitly set.
	withSchema := filepath.Join(dir, "WITH01.md")
	if err := os.WriteFile(withSchema, []byte("---\nid: WITH01\ntitle: structured\ntype: story\nstatus: backlog\npriority: 1\n---\nbody\n"), 0o644); err != nil {
		t.Fatalf("write with-schema: %v", err)
	}

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	all := s.GetAllTikis()
	if len(all) != 2 {
		t.Fatalf("GetAllTikis returned %d tikis, want 2", len(all))
	}

	ids := map[string]bool{}
	for _, tk := range all {
		ids[tk.ID] = true
	}
	if !ids["BARE01"] {
		t.Error("bare tiki should appear in GetAllTikis — store does not classify")
	}
	if !ids["WITH01"] {
		t.Error("schema-bearing tiki should appear in GetAllTikis")
	}

	// GetTiki finds both by id, and presence checks reflect the loaded shape.
	if tk := s.GetTiki("BARE01"); tk == nil {
		t.Error("GetTiki should find the bare tiki by id")
	} else if hasAnySchemaField(tk) {
		t.Error("bare tiki should have no schema-known fields after load")
	}
	if tk := s.GetTiki("WITH01"); tk == nil || !hasAnySchemaField(tk) {
		t.Error("schema-bearing tiki should retain its schema-known fields after load")
	}
}

// TestCreateTiki_ProgrammaticTikiAppearsInListing verifies that a tiki
// created in-process through CreateTiki immediately appears in GetAllTikis
// without any classification step. The list is "everything the store knows
// about", not "everything the store decided was workflow-shaped".
func TestCreateTiki_ProgrammaticTikiAppearsInListing(t *testing.T) {
	dir := t.TempDir()
	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	if err := s.CreateTiki(tikiWithSchemaFields()); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	if got := len(s.GetAllTikis()); got != 1 {
		t.Errorf("GetAllTikis after CreateTiki: got %d, want 1", got)
	}
}
