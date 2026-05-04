package tikistore

import (
	"os"
	"path/filepath"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestUpdateTiki_PreservesWorkflowClassificationForFreshValues is a regression
// guard: a caller that constructs a brand-new Tiki value (so Fields is empty /
// has no workflow keys) and passes it to UpdateTiki must not see the tiki
// disappear from GetAllTikis when workflow-field filters are applied. The store
// carries Path and LoadedMtime forward from the stored tiki; callers that set
// workflow fields in the fresh value get them emitted — exact-presence means
// whatever Fields the caller sets are what land on disk.
//
// Without exact-presence semantics a full reload would re-classify from disk,
// but in the in-memory index we check the live tiki, not the file.
func TestUpdateTiki_PreservesWorkflowClassificationForFreshValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "WF0001.md")
	content := "---\nid: WF0001\ntitle: workflow doc\ntype: story\nstatus: backlog\npriority: 3\npoints: 1\n---\nbody\n"
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
	if !hasAnyWorkflowField(loaded) {
		t.Fatal("precondition: loaded doc should be workflow-capable (status set)")
	}

	// Simulate a caller that constructs a fresh Tiki with workflow fields
	// explicitly set — no path, no LoadedMtime — and calls UpdateTiki.
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

	// Post-condition #1: the tiki is still classified as workflow — so it
	// keeps appearing in workflow-filtered views.
	after := store.GetTiki("WF0001")
	if after == nil {
		t.Fatal("WF0001 missing after update")
	}
	if !hasAnyWorkflowField(after) {
		t.Error("tiki lost workflow classification after UpdateTiki — would vanish from board views")
	}

	// Post-condition #2: GetAllTikis returns the tiki. This is the actual
	// user-visible regression, not just the flag.
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
