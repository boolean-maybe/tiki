package taskdetail

import (
	"testing"

	"github.com/rivo/tview"
)

func TestGridContainer_RebuildOnWidthChange(t *testing.T) {
	fields := []GridField{
		fixedHeight("a", 1),
		fixedHeight("b", 1),
	}
	primitives := map[string]tview.Primitive{
		"a": tview.NewTextView(),
		"b": tview.NewTextView(),
	}
	g := newGridContainer(fields, primitives)

	g.rebuild(120)
	if g.lastWidth != 120 {
		t.Errorf("after rebuild(120): lastWidth = %d, want 120", g.lastWidth)
	}
	firstChildren := g.GetItemCount()
	g.rebuild(60)
	if g.lastWidth != 60 {
		t.Errorf("after rebuild(60): lastWidth = %d, want 60", g.lastWidth)
	}
	if g.GetItemCount() == 0 {
		t.Error("after rebuild: no children")
	}
	// Same-width rebuild remains valid (idempotent).
	g.rebuild(60)
	if g.GetItemCount() == 0 {
		t.Error("after idempotent rebuild: no children")
	}
	_ = firstChildren
}

func TestGridContainer_EmptyFields(t *testing.T) {
	g := newGridContainer(nil, nil)
	g.rebuild(80)
	if g.GetItemCount() != 0 {
		t.Errorf("empty fields rebuild: GetItemCount = %d, want 0", g.GetItemCount())
	}
}
