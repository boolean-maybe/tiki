package tikistore

import (
	"os"
	"path/filepath"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestUpdateTiki_FreshValueWithSchemaFieldsPersists is a regression guard:
// a caller that builds a fresh *Tiki in memory (no Path, no LoadedMtime)
// and passes it to UpdateTiki should end up with all the schema-known
// fields the caller set on disk, and the tiki should still appear in
// GetAllTikis afterwards.
//
// In-memory exact-presence: whatever Fields the caller sets is what lands
// on disk; the store does not synthesize anything.
func TestUpdateTiki_FreshValueWithSchemaFieldsPersists(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "WF0001.md")
	content := "---\nid: WF0001\ntitle: structured doc\ntype: story\nstatus: backlog\npriority: 3\npoints: 1\n---\nbody\n"
	//nolint:gosec // G306: matches repo-wide test fixture perms
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	store, err := NewTikiStore(root)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	loaded := store.GetTiki("WF0001")
	if loaded == nil {
		t.Fatal("WF0001 missing after load")
	}
	if !hasAnySchemaField(loaded) {
		t.Fatal("precondition: loaded doc should carry schema-known fields (status set)")
	}

	// Construct a fresh Tiki with schema-known fields explicitly set —
	// no path, no LoadedMtime — and call UpdateTiki.
	fresh := tikipkg.New()
	fresh.ID = "WF0001"
	fresh.Title = "updated title"
	fresh.Set("type", "story")
	fresh.Set("status", "backlog")
	fresh.Set("priority", 3)
	fresh.Set("points", 1)
	if err := store.UpdateTiki(fresh); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	// Post-condition: the schema-known fields the caller set are still
	// present, so a downstream `select where has(status)` filter still
	// matches.
	after := store.GetTiki("WF0001")
	if after == nil {
		t.Fatal("WF0001 missing after update")
	}
	if !hasAnySchemaField(after) {
		t.Error("UpdateTiki dropped the caller's schema-known fields — would vanish from has(status) filters")
	}

	// And the tiki is still listed by GetAllTikis.
	all := store.GetAllTikis()
	found := false
	for _, tk := range all {
		if tk.ID == "WF0001" {
			found = true
			break
		}
	}
	if !found {
		t.Error("WF0001 missing from GetAllTikis after UpdateTiki with fresh Tiki value")
	}
}
