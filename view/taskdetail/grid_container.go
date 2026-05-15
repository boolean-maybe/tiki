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
	primitives []tview.Primitive // indexed by anchor position in spec.Anchors
	heightOf   func(name string, width int) int
	lastWidth  int
}

// newGridContainer wires the parsed grid spec, the per-anchor primitives,
// and a height-resolver into a horizontal Flex wrapper. The first Draw
// call computes the layout against the live width.
//
// primitives is indexed by anchor position (same order as spec.Anchors)
// so both field and literal anchors can be addressed uniformly.
func newGridContainer(spec gridlayout.GridSpec, primitives []tview.Primitive, heightOf func(name string, width int) int) *gridContainer {
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

// rebuild recomputes the grid plan and repopulates the outer Flex.
//
// Each grid column becomes an inner FlexRow. An anchored cell allocates
// only its anchor's natural height (heightOf result), not the grown row
// height: that prevents short single-line cells in a row from getting
// padded with blank space whenever a neighbour column grew the row to
// accommodate wrapped content (e.g. multi-line tags). Leftover slack at
// the bottom of each column is absorbed by a residual flex-1 box so the
// column ends flush with its neighbours.
//
// Cells covered by a horizontally-spanning anchor receive an empty
// placeholder so the column's vertical alignment matches its neighbours.
func (g *gridContainer) rebuild(width int) {
	g.lastWidth = width
	g.Flex.Clear()
	plan := SolveGridLayout(width, g.spec, g.heightOf)
	if plan.Cols == 0 || plan.Rows == 0 {
		return
	}

	// Map (row, col) → anchor index in spec.Anchors. Anchors are placed
	// at their top-left only; spanned cells are not separate entries.
	anchorAt := make(map[int]int, len(g.spec.Anchors))
	for i, a := range g.spec.Anchors {
		anchorAt[a.Row*plan.Cols+a.Col] = i
	}

	for c := 0; c < plan.Cols; c++ {
		if plan.Dropped[c] {
			continue
		}
		colFlex := tview.NewFlex().SetDirection(tview.FlexRow)
		r := 0
		for r < plan.Rows {
			if idx, ok := anchorAt[r*plan.Cols+c]; ok {
				a := g.spec.Anchors[idx]
				prim := g.primitives[idx]
				if prim == nil {
					prim = tview.NewBox()
				}
				h := g.anchorPlacementHeight(a, plan)
				colFlex.AddItem(prim, h, 0, false)
				r += a.RowSpan
				continue
			}
			colFlex.AddItem(tview.NewBox(), plan.RowHeights[r], 0, false)
			r++
		}
		// Residual flex-1 box absorbs any leftover vertical space in this
		// column. Without it, a column whose anchors collectively take less
		// height than the row-band sum would render with a draw artifact
		// at the bottom (the outer Flex's first AddItem would receive the
		// slack instead). This also delivers the row-packing fix: shorter
		// cells stop being padded mid-column when a neighbour column's
		// content grew a row band.
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

// anchorPlacementHeight returns the height to allocate for an anchor's
// primitive within its column. For field anchors, this is the anchor's
// natural height (from heightOf); for literals it is fixed at 1. Critically
// it is NOT the solver's row-band sum — see rebuild() for the rationale.
func (g *gridContainer) anchorPlacementHeight(a gridlayout.Anchor, plan gridlayout.Plan) int {
	if a.Kind == gridlayout.AnchorLiteral {
		return 1
	}
	totalWidth := 0
	visible := 0
	for cc := a.Col; cc < a.Col+a.ColSpan; cc++ {
		if plan.Dropped[cc] {
			continue
		}
		visible++
		totalWidth += plan.ColumnWidths[cc]
	}
	if visible > 1 {
		totalWidth += (visible - 1) * interColumnGap
	}
	h := g.heightOf(a.Name, totalWidth)
	if h < 1 {
		h = 1
	}
	return h
}
