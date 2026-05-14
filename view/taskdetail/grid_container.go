package taskdetail

import (
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// gridContainer hosts the metadata grid for a configurable detail view.
// It rebuilds its inner Flex layout on width change so the layout
// algorithm runs against the live terminal width — tview primitives
// don't see their width at construction time, only on Draw via GetRect().
//
// The container is a FlexColumn (one inner Flex per grid column). Each
// inner Flex is a FlexRow of the anchors that start in that column at
// their resolved row height. Row-spanning anchors take the summed height
// of their spanned rows; cells covered by a horizontally-spanning anchor
// from the left receive an empty placeholder so vertical alignment is
// preserved.
//
// Using nested Flexes (rather than tview.Grid) keeps the focus chain
// identical to the legacy renderer — editor widgets receive Tab/Down
// events without the Grid's internal focus traversal interfering.
type gridContainer struct {
	*tview.Flex
	spec       gridlayout.GridSpec
	primitives map[string]tview.Primitive
	heightOf   func(name string, width int) int
	lastWidth  int
}

// newGridContainer wires the parsed grid spec, the per-anchor primitives,
// and a height-resolver into a horizontal Flex wrapper. The first Draw
// call computes the layout against the live width.
func newGridContainer(spec gridlayout.GridSpec, primitives map[string]tview.Primitive, heightOf func(name string, width int) int) *gridContainer {
	g := &gridContainer{
		Flex:       tview.NewFlex().SetDirection(tview.FlexColumn),
		spec:       spec,
		primitives: primitives,
		heightOf:   heightOf,
		lastWidth:  -1,
	}
	return g
}

// Draw rebuilds the inner layout when the width has changed, then
// delegates to Flex.Draw to render the children.
func (g *gridContainer) Draw(screen tcell.Screen) {
	_, _, width, _ := g.GetRect()
	if width != g.lastWidth {
		g.rebuild(width)
	}
	g.Flex.Draw(screen)
}

// rebuild recomputes the grid plan and repopulates the outer Flex. Each
// grid column becomes an inner FlexRow column; anchors anchored at that
// column place their primitive at the resolved height; cells covered by
// a horizontally-spanning anchor receive an empty placeholder so the
// column's vertical alignment matches its neighbours.
func (g *gridContainer) rebuild(width int) {
	g.lastWidth = width
	g.Flex.Clear()
	plan := SolveGridLayout(width, g.spec, g.heightOf)
	if plan.Cols == 0 || plan.Rows == 0 {
		return
	}

	anchorAt := make(map[int]gridlayout.PlacedAnchor, len(plan.Placed))
	for _, p := range plan.Placed {
		anchorAt[p.Row*plan.Cols+p.Col] = p
	}

	for c := 0; c < plan.Cols; c++ {
		if plan.Dropped[c] {
			continue
		}
		colFlex := tview.NewFlex().SetDirection(tview.FlexRow)
		r := 0
		for r < plan.Rows {
			if a, ok := anchorAt[r*plan.Cols+c]; ok {
				prim := g.primitives[a.Name]
				if prim == nil {
					prim = tview.NewBox()
				}
				colFlex.AddItem(prim, a.Height, 0, false)
				r += a.RowSpan
				continue
			}
			colFlex.AddItem(tview.NewBox(), plan.RowHeights[r], 0, false)
			r++
		}
		// Soak up any leftover vertical slack when the outer container is
		// taller than the sum of row heights (e.g. fixed-height frame
		// chrome). Without this, residual rows would be left blank but
		// would draw against the outer Flex's first AddItem.
		colFlex.AddItem(tview.NewBox(), 0, 1, false)

		colWidth := plan.ColumnWidths[c]
		proportion := 0
		if g.spec.Stretcher[c] {
			proportion = 1
			colWidth = 0
		}
		g.Flex.AddItem(colFlex, colWidth, proportion, false)
		if c < plan.Cols-1 && !plan.Dropped[c+1] {
			g.Flex.AddItem(tview.NewBox(), interColumnGap, 0, false)
		}
	}
}
