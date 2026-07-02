package store

import (
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestInMemoryStore_GetAllTikis_DeterministicOrder pins GetAllTikis to a
// stable, ID-sorted order so downstream stable sorts on tied keys do not
// flicker between renders. See store/tikistore for the production-store
// equivalent test and the user-visible Roadmap-epic symptom.
func TestInMemoryStore_GetAllTikis_DeterministicOrder(t *testing.T) {
	store := NewInMemoryStore()
	for _, id := range []string{"F8CGVY", "T0CVS4", "ABC123", "ZZZ999"} {
		tk := tikipkg.New()
		tk.SetID(id)
		if err := store.CreateTiki(tk); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}

	want := []string{"ABC123", "F8CGVY", "T0CVS4", "ZZZ999"}

	const trials = 50
	for i := 0; i < trials; i++ {
		got := store.GetAllTikis()
		if len(got) != len(want) {
			t.Fatalf("iteration %d: len = %d, want %d", i, len(got), len(want))
		}
		for j, tk := range got {
			if tk.ID() != want[j] {
				ids := make([]string, len(got))
				for k, t2 := range got {
					ids[k] = t2.ID()
				}
				t.Fatalf("iteration %d: got[%d].ID = %q, want %q (full order: %v)",
					i, j, tk.ID(), want[j], ids)
			}
		}
	}
}
