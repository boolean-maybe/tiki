package taskdetail

import (
	"testing"

	"github.com/rivo/tview"
)

func unitHeight(string, int) int { return 1 }

func TestGridContainer_RebuildOnWidthChange(t *testing.T) {
	spec := singleColumnSpec([]string{"a", "b"})
	primitives := map[string]tview.Primitive{
		"a": tview.NewTextView(),
		"b": tview.NewTextView(),
	}
	g := newGridContainer(spec, primitives, unitHeight)

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

func TestGridContainer_EmptySpec(t *testing.T) {
	g := newGridContainer(singleColumnSpec(nil), nil, unitHeight)
	g.rebuild(80)
	// No panic, lastWidth tracked.
	if g.lastWidth != 80 {
		t.Errorf("empty spec rebuild: lastWidth = %d, want 80", g.lastWidth)
	}
}
