package tikistore

import (
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestGetAllTikis_DeterministicOrder pins GetAllTikis to a stable, ID-sorted
// order. Without this guarantee, Go map iteration randomizes the slice on
// every call, which causes downstream stable sorts (e.g. ruki "order by"
// with tied keys) to render tikis in different positions on each redraw.
// The Roadmap epic view exhibited this: two epics tied on (priority, points)
// appeared to swap at random as the user navigated.
func TestGetAllTikis_DeterministicOrder(t *testing.T) {
	mk := func(id string) *tikipkg.Tiki {
		tk := tikipkg.New()
		tk.SetID(id)
		return tk
	}
	store := &TikiStore{
		tikis: makeTikiMap(
			mk("F8CGVY"),
			mk("T0CVS4"),
			mk("ABC123"),
			mk("ZZZ999"),
		),
	}

	want := []string{"ABC123", "F8CGVY", "T0CVS4", "ZZZ999"}

	// Many iterations: a single call would pass by coincidence. Repeating
	// makes Go's randomized map iteration extremely unlikely to land on
	// the same order all 50 times unless GetAllTikis is sorting.
	const trials = 50
	for i := 0; i < trials; i++ {
		got := store.GetAllTikis()
		if len(got) != len(want) {
			t.Fatalf("iteration %d: len = %d, want %d", i, len(got), len(want))
		}
		for j, tk := range got {
			if tk.ID() != want[j] {
				t.Fatalf("iteration %d: got[%d].ID = %q, want %q (full order: %v)",
					i, j, tk.ID(), want[j], idsOf(got))
			}
		}
	}
}

func idsOf(tikis []*tikipkg.Tiki) []string {
	out := make([]string, len(tikis))
	for i, tk := range tikis {
		out[i] = tk.ID()
	}
	return out
}
