package tikistore

import (
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

func TestReloadTask_CaseDuplicate(t *testing.T) {
	store, err := NewTikiStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create a tiki with ID 6EQDUE.
	tk := tikipkg.New()
	tk.ID = "6EQDUE"
	tk.Title = "Case Duplicate"
	tk.Set("type", "story")
	tk.Set("status", "backlog")
	tk.Set("priority", 3)
	tk.Set("points", 1)
	if err := store.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki failed: %v", err)
	}

	// Reload by ID; should not create a duplicate entry.
	if err := store.ReloadTask("6EQDUE"); err != nil {
		t.Fatalf("ReloadTask failed: %v", err)
	}

	tikis := store.GetAllTikis()
	if len(tikis) != 1 {
		t.Fatalf("expected 1 tiki after reload, got %d", len(tikis))
	}

	foundUpper := false
	for _, tik := range tikis {
		if tik.ID == "6EQDUE" {
			foundUpper = true
		}
	}

	if !foundUpper {
		t.Fatalf("expected uppercase ID variant, foundUpper=%v", foundUpper)
	}
}
