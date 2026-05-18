package gridbox

import (
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/rivo/tview"
)

func unitHeight(string, int) int { return 1 }

// singleColumnSpecForTest synthesizes a 1-column grid from a name list.
// Mirrors the test helper that previously lived in package tikidetail.
func singleColumnSpecForTest(names []string) gridlayout.GridSpec {
	if len(names) == 0 {
		return gridlayout.GridSpec{}
	}
	anchors := make([]gridlayout.Anchor, len(names))
	cells := make([][]gridlayout.Cell, len(names))
	for i, n := range names {
		anchors[i] = gridlayout.Anchor{
			Name: n, Row: i, Col: 0, RowSpan: 1, ColSpan: 1,
		}
		cells[i] = []gridlayout.Cell{gridlayout.FieldCell{Name: n}}
	}
	return gridlayout.GridSpec{
		Rows:      len(names),
		Cols:      1,
		Anchors:   anchors,
		Stretcher: []bool{false},
		Cells:     cells,
	}
}

func TestContainer_RebuildOnWidthChange(t *testing.T) {
	spec := singleColumnSpecForTest([]string{"a", "b"})
	primitives := []tview.Primitive{
		tview.NewTextView(),
		tview.NewTextView(),
	}
	g := NewContainer(spec, primitives, unitHeight)

	g.rebuild(120)
	if g.lastWidth != 120 {
		t.Errorf("after rebuild(120): lastWidth = %d, want 120", g.lastWidth)
	}
	g.rebuild(60)
	if g.lastWidth != 60 {
		t.Errorf("after rebuild(60): lastWidth = %d, want 60", g.lastWidth)
	}
	// Same-width rebuild remains valid (idempotent).
	g.rebuild(60)
	if g.lastWidth != 60 {
		t.Errorf("after idempotent rebuild: lastWidth = %d, want 60", g.lastWidth)
	}
}

func TestContainer_EmptySpec(t *testing.T) {
	g := NewContainer(singleColumnSpecForTest(nil), nil, unitHeight)
	g.rebuild(80)
	// No panic, lastWidth tracked.
	if g.lastWidth != 80 {
		t.Errorf("empty spec rebuild: lastWidth = %d, want 80", g.lastWidth)
	}
}

// TestContainer_AnchorPlacementHeight verifies that:
//   - Literal anchors always get height 1, regardless of heightOf.
//   - Field anchors get their natural heightOf result, NOT the solver's
//     grown row-band sum. This is the row-packing fix — short field
//     anchors must not get padded when a sibling column grows the row
//     band for a multi-line value.
func TestContainer_AnchorPlacementHeight(t *testing.T) {
	spec, err := gridlayout.ParseGrid([][]string{
		{"Status:", "status", "Tags:", "tags"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	heightOf := func(name string, w int) int {
		if name == "tags" {
			return 3
		}
		return 1
	}
	g := NewContainer(spec, make([]tview.Primitive, len(spec.Anchors)), heightOf)
	plan := SolveGridLayout(120, spec, heightOf)

	cases := []struct {
		idx        int
		wantHeight int
		why        string
	}{
		{0, 1, "literal caption Status: always 1 row"},
		{1, 1, "status is single-row even though tags grew the row band"},
		{2, 1, "literal caption Tags: always 1 row"},
		{3, 3, "tags has natural height 3"},
	}
	for _, c := range cases {
		a := spec.Anchors[c.idx]
		got := g.anchorPlacementHeight(a, plan)
		if got != c.wantHeight {
			t.Errorf("anchor[%d] (%s): got height %d, want %d (%s)",
				c.idx, a.Name+a.Text, got, c.wantHeight, c.why)
		}
	}
}
