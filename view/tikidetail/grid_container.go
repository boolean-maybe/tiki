package tikidetail

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
// The layout is hybrid row/column-major. Rows containing a horizontal
// span (ColSpan > 1) render as a FlexColumn where the spanning anchor
// occupies its combined width. Consecutive rows without horizontal spans
// render column-major (each column is a FlexRow) so per-column natural-
// height packing works — short cells don't get padded when a neighbour
// column's content grew the row (e.g. multi-line tags).
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
		Flex:       tview.NewFlex().SetDirection(tview.FlexRow),
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
// The outer Flex is FlexRow (vertical stacking). Rows are partitioned
// into bands:
//   - A row containing any anchor with ColSpan > 1 is a "spanning row"
//     rendered as a standalone FlexColumn. The spanning anchor receives
//     the combined width of its spanned columns + inter-column gaps.
//   - Consecutive rows without horizontal spans form a "packed band"
//     rendered column-major (each column is a FlexRow). This preserves
//     per-column natural-height packing so short cells in a row don't
//     get padded when a neighbour column grew the row (e.g. multi-line
//     tags). A residual flex-1 box at the bottom of each column absorbs
//     leftover space.
func (g *gridContainer) rebuild(width int) {
	g.lastWidth = width
	g.Flex.Clear()
	plan := SolveGridLayout(width, g.spec, g.heightOf)
	if plan.Cols == 0 || plan.Rows == 0 {
		return
	}

	anchorAt := make(map[int]int, len(g.spec.Anchors))
	for i, a := range g.spec.Anchors {
		anchorAt[a.Row*plan.Cols+a.Col] = i
	}

	hasHSpan := make([]bool, plan.Rows)
	for _, a := range g.spec.Anchors {
		if a.ColSpan > 1 {
			hasHSpan[a.Row] = true
		}
	}

	r := 0
	for r < plan.Rows {
		if hasHSpan[r] {
			g.addSpanningRow(r, plan, anchorAt)
			r++
		} else {
			start := r
			for r < plan.Rows && !hasHSpan[r] {
				r++
			}
			g.addPackedBand(start, r, plan, anchorAt)
		}
	}
}

// addSpanningRow renders a single row that contains at least one
// horizontal span as a FlexColumn. Each cell gets the combined width of
// its spanned columns (plus inter-column gaps between them).
func (g *gridContainer) addSpanningRow(row int, plan gridlayout.Plan, anchorAt map[int]int) {
	rowFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	c := 0
	for c < plan.Cols {
		if plan.Dropped[c] {
			c++
			continue
		}
		idx, ok := anchorAt[row*plan.Cols+c]
		if !ok {
			colWidth := plan.ColumnWidths[c]
			proportion := 0
			if g.spec.Stretcher[c] {
				proportion = 1
				colWidth = 0
			}
			rowFlex.AddItem(tview.NewBox(), colWidth, proportion, false)
			if c < plan.Cols-1 && !plan.Dropped[c+1] {
				rowFlex.AddItem(tview.NewBox(), interColumnGap, 0, false)
			}
			c++
			continue
		}
		a := g.spec.Anchors[idx]
		prim := g.primitives[idx]
		if prim == nil {
			prim = tview.NewBox()
		}
		cellWidth, hasStretcher := g.spanWidth(a, plan)
		proportion := 0
		if hasStretcher {
			proportion = 1
			cellWidth = 0
		}
		rowFlex.AddItem(prim, cellWidth, proportion, false)
		nextVisibleCol := g.lastVisibleSpannedCol(a, plan)
		if nextVisibleCol < plan.Cols-1 && !plan.Dropped[nextVisibleCol+1] {
			rowFlex.AddItem(tview.NewBox(), interColumnGap, 0, false)
		}
		c += a.ColSpan
	}
	h := plan.RowHeights[row]
	g.Flex.AddItem(rowFlex, h, 0, false)
}

// addPackedBand renders consecutive rows [start, end) that have no
// horizontal spans using column-major layout. Each column is a FlexRow
// containing only the cells for this band's rows. Per-column natural-
// height packing ensures short cells aren't padded by taller neighbours.
func (g *gridContainer) addPackedBand(start, end int, plan gridlayout.Plan, anchorAt map[int]int) {
	bandHeight := 0
	for r := start; r < end; r++ {
		bandHeight += plan.RowHeights[r]
	}

	bandFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	for c := 0; c < plan.Cols; c++ {
		if plan.Dropped[c] {
			continue
		}
		colFlex := tview.NewFlex().SetDirection(tview.FlexRow)
		r := start
		for r < end {
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
		colFlex.AddItem(tview.NewBox(), 0, 1, false)

		colWidth := plan.ColumnWidths[c]
		proportion := 0
		if g.spec.Stretcher[c] {
			proportion = 1
			colWidth = 0
		}
		bandFlex.AddItem(colFlex, colWidth, proportion, false)
		if c < plan.Cols-1 && !plan.Dropped[c+1] {
			bandFlex.AddItem(tview.NewBox(), interColumnGap, 0, false)
		}
	}
	g.Flex.AddItem(bandFlex, bandHeight, 0, false)
}

// spanWidth returns the total character width of an anchor's column span
// (including inter-column gaps between visible spanned columns) and
// whether any spanned column is a stretcher.
func (g *gridContainer) spanWidth(a gridlayout.Anchor, plan gridlayout.Plan) (int, bool) {
	totalWidth := 0
	visible := 0
	hasStretcher := false
	for cc := a.Col; cc < a.Col+a.ColSpan; cc++ {
		if plan.Dropped[cc] {
			continue
		}
		visible++
		totalWidth += plan.ColumnWidths[cc]
		if g.spec.Stretcher[cc] {
			hasStretcher = true
		}
	}
	if visible > 1 {
		totalWidth += (visible - 1) * interColumnGap
	}
	return totalWidth, hasStretcher
}

// lastVisibleSpannedCol returns the index of the last non-dropped column
// within an anchor's horizontal span.
func (g *gridContainer) lastVisibleSpannedCol(a gridlayout.Anchor, plan gridlayout.Plan) int {
	last := a.Col
	for cc := a.Col; cc < a.Col+a.ColSpan; cc++ {
		if !plan.Dropped[cc] {
			last = cc
		}
	}
	return last
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
