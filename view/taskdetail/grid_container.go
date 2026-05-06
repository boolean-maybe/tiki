package taskdetail

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// gridContainer is a tview primitive that hosts a fixed-height grid of
// metadata fields. It rebuilds its inner Flex layout on width change so the
// grid algorithm runs against the live terminal width — tview primitives
// don't see their width at construction time, only on Draw via GetRect().
//
// Lineage: this mirrors the (deleted) responsiveMetadataRow draw-time
// rebuild pattern from the legacy view; the responsibilities split here
// because the new layout is always-grid rather than responsive-sections.
type gridContainer struct {
	*tview.Flex
	fields     []GridField
	primitives map[string]tview.Primitive
	lastWidth  int
}

// newGridContainer wires the input fields and their pre-rendered primitives
// into a horizontal Flex. The first Draw call computes the layout against
// the live width.
func newGridContainer(fields []GridField, primitives map[string]tview.Primitive) *gridContainer {
	g := &gridContainer{
		Flex:       tview.NewFlex().SetDirection(tview.FlexColumn),
		fields:     fields,
		primitives: primitives,
		lastWidth:  -1,
	}
	return g
}

// Draw rebuilds the inner layout when the width has changed, then delegates
// to Flex.Draw to render the children.
func (g *gridContainer) Draw(screen tcell.Screen) {
	_, _, width, _ := g.GetRect()
	if width != g.lastWidth {
		g.rebuild(width)
	}
	g.Flex.Draw(screen)
}

// rebuild recomputes the grid plan and repopulates the outer Flex with one
// inner FlexRow per column. Each inner column places its fields at their
// planned heights via AddItem(p, h, 0, false), with a residual stretch box
// absorbing leftover rows so the column always fills FixedHeight.
func (g *gridContainer) rebuild(width int) {
	g.lastWidth = width
	g.Clear()
	plan := CalculateGridLayout(width, g.fields)
	for i, col := range plan.Columns {
		colFlex := tview.NewFlex().SetDirection(tview.FlexRow)
		used := 0
		for _, f := range col.Fields {
			p := g.primitives[f.Name]
			if p == nil {
				p = tview.NewBox()
			}
			colFlex.AddItem(p, f.H, 0, false)
			used += f.H
		}
		if remaining := plan.FixedHeight - used; remaining > 0 {
			colFlex.AddItem(tview.NewBox(), remaining, 0, false)
		}
		g.AddItem(colFlex, col.Width, 0, false)
		if i < len(plan.Columns)-1 {
			g.AddItem(tview.NewBox(), interColumnGap, 0, false)
		}
	}
}
