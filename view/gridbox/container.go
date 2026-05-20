// Package gridbox renders a parsed gridlayout spec into a width-adaptive
// tview primitive. Two surfaces use it: the detail view's layout grid
// (view-mode only — edit mode owns its own renderer in package tikidetail)
// and the tiki box on board/list views.
//
// What lives here vs. in tikidetail:
//
//   - This package owns the layout primitive (Container) and the
//     layout-solver adapter (SolveGridLayout, DefaultAnchorWidth,
//     overhead constants). Nothing in this package knows about field
//     names, render context, themes, or any per-field logic.
//   - Package view/tikidetail owns the view-mode field renderers
//     (RenderViewModeAnchor and the underlying renderConfiguredField,
//     renderCompositePrimitive, renderLiteralCaption, etc.). The tiki
//     box reuses those renderers via the tikidetail package's exported
//     API rather than this package owning a parallel copy.
//
// The plan that introduced this package envisioned a
// view/gridbox/field_render.go that would own the view-mode renderers.
// Implementation discovered the renderers depend on a substantial
// amount of tikidetail-internal state (FieldRenderContext, the field
// registry, theme paint resolvers, workflow.ExpandVisual integration);
// duplicating them in gridbox or moving the entire dependency surface
// into gridbox would have been a much larger refactor. The chosen
// alternative — gridbox owns layout primitives only, tikidetail owns
// field rendering — keeps the package boundary clean and the dependency
// direction one-way (view/tiki_box.go imports both).
package gridbox

import (
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Container hosts a layout grid for a configurable detail view or a tiki
// box. It rebuilds its inner Flex layout on width change so the layout
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
type Container struct {
	*tview.Flex
	spec           gridlayout.GridSpec
	primitives     []tview.Primitive // indexed by anchor position in spec.Anchors
	heightOf       func(name string, width int) int
	lastWidth      int
	selectionBg    tcell.Color
	selectionBgSet bool
}

// NewContainer wires the parsed grid spec, the per-anchor primitives,
// and a height-resolver into a horizontal Flex wrapper. The first Draw
// call computes the layout against the live width.
//
// primitives is indexed by anchor position (same order as spec.Anchors)
// so both field and literal anchors can be addressed uniformly.
func NewContainer(spec gridlayout.GridSpec, primitives []tview.Primitive, heightOf func(name string, width int) int) *Container {
	g := &Container{
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
func (g *Container) Draw(screen tcell.Screen) {
	_, _, width, _ := g.GetRect()
	if width != g.lastWidth {
		g.rebuild(width)
	}
	g.Flex.Draw(screen)
}

// SetSelectionBackground configures a background color that will be
// applied to every child primitive (cached anchor primitives and the
// spacer Boxes added during rebuild). Required for the borderless
// single-row tiki box: setting only the outer container's background
// is invisible because each child Box.DrawForSubclass paints its own
// (default) background, masking the row color. Propagating the color
// to children makes the selection band visible end-to-end.
func (g *Container) SetSelectionBackground(color tcell.Color) {
	g.selectionBg = color
	g.selectionBgSet = true
	g.SetBackgroundColor(color)
	for _, p := range g.primitives {
		applyBackground(p, color)
	}
	g.lastWidth = -1
}

// applyBackground sets the background color on any tview primitive
// whose concrete type embeds *tview.Box (the common case for everything
// gridbox builds: TextView, Flex, Box). The interface assertion keeps
// the call total — primitives that don't embed Box are silently skipped.
func applyBackground(p tview.Primitive, color tcell.Color) {
	if p == nil {
		return
	}
	type bgSetter interface {
		SetBackgroundColor(tcell.Color) *tview.Box
	}
	if s, ok := p.(bgSetter); ok {
		s.SetBackgroundColor(color)
	}
}

// newSpacer creates a filler Box and tints it with the selection bg if
// one is configured. Used everywhere rebuild() previously called
// tview.NewBox() so spacer cells participate in the row's selection
// band instead of punching default-bg holes through it.
func (g *Container) newSpacer() *tview.Box {
	b := tview.NewBox()
	if g.selectionBgSet {
		b.SetBackgroundColor(g.selectionBg)
	}
	return b
}

// newRowFlex creates an inner Flex used to assemble a row or column band
// during rebuild. Mirrors newSpacer for the same reason: tview.Flex's
// embedded Box paints its own bg, so it must be tinted to keep the
// selection band continuous beneath the children.
func (g *Container) newRowFlex(direction int) *tview.Flex {
	f := tview.NewFlex().SetDirection(direction)
	if g.selectionBgSet {
		f.SetBackgroundColor(g.selectionBg)
	}
	return f
}

// HasFocus reports whether any cached primitive holds focus.
//
// We override Flex's implementation because Container's inner Flex is
// populated lazily — rebuild() runs only on first Draw. tview.Application
// dispatches input by walking the primitive tree top-down, asking each
// container whether any descendant has focus; if Container's embedded
// Flex has no items yet, Flex.HasFocus iterates an empty slice and
// returns false, causing the entire dispatch to short-circuit and
// silently drop the keystroke. By consulting g.primitives directly we
// report focus correctly even before the first Draw — i.e. between
// view creation and the first redraw cycle.
func (g *Container) HasFocus() bool {
	for _, p := range g.primitives {
		if p != nil && p.HasFocus() {
			return true
		}
	}
	return g.Flex.HasFocus()
}

// InputHandler forwards keystrokes to whichever cached primitive holds
// focus. Mirrors HasFocus: Flex.InputHandler iterates g.Flex.items, but
// items are populated lazily so a keystroke arriving before the first
// Draw would otherwise be dropped. By walking g.primitives we forward
// correctly in the cold-Container case too.
func (g *Container) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return g.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		for _, p := range g.primitives {
			if p == nil || !p.HasFocus() {
				continue
			}
			if handler := p.InputHandler(); handler != nil {
				handler(event, setFocus)
				return
			}
		}
		// Fallback: if no cached primitive claims focus, defer to the
		// embedded Flex (which may have non-primitive filler items).
		if handler := g.Flex.InputHandler(); handler != nil {
			handler(event, setFocus)
		}
	})
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
func (g *Container) rebuild(width int) {
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
func (g *Container) addSpanningRow(row int, plan gridlayout.Plan, anchorAt map[int]int) {
	rowFlex := g.newRowFlex(tview.FlexColumn)
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
			rowFlex.AddItem(g.newSpacer(), colWidth, proportion, false)
			if c < plan.Cols-1 && !plan.Dropped[c+1] {
				rowFlex.AddItem(g.newSpacer(), InterColumnGap, 0, false)
			}
			c++
			continue
		}
		a := g.spec.Anchors[idx]
		prim := g.primitives[idx]
		if prim == nil {
			prim = g.newSpacer()
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
			rowFlex.AddItem(g.newSpacer(), InterColumnGap, 0, false)
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
func (g *Container) addPackedBand(start, end int, plan gridlayout.Plan, anchorAt map[int]int) {
	bandHeight := 0
	for r := start; r < end; r++ {
		bandHeight += plan.RowHeights[r]
	}

	bandFlex := g.newRowFlex(tview.FlexColumn)
	for c := 0; c < plan.Cols; c++ {
		if plan.Dropped[c] {
			continue
		}
		colFlex := g.newRowFlex(tview.FlexRow)
		r := start
		for r < end {
			if idx, ok := anchorAt[r*plan.Cols+c]; ok {
				a := g.spec.Anchors[idx]
				prim := g.primitives[idx]
				if prim == nil {
					prim = g.newSpacer()
				}
				h := g.anchorPlacementHeight(a, plan)
				colFlex.AddItem(prim, h, 0, false)
				r += a.RowSpan
				continue
			}
			colFlex.AddItem(g.newSpacer(), plan.RowHeights[r], 0, false)
			r++
		}
		colFlex.AddItem(g.newSpacer(), 0, 1, false)

		colWidth := plan.ColumnWidths[c]
		proportion := 0
		if g.spec.Stretcher[c] {
			proportion = 1
			colWidth = 0
		}
		bandFlex.AddItem(colFlex, colWidth, proportion, false)
		if c < plan.Cols-1 && !plan.Dropped[c+1] {
			bandFlex.AddItem(g.newSpacer(), InterColumnGap, 0, false)
		}
	}
	g.Flex.AddItem(bandFlex, bandHeight, 0, false)
}

// spanWidth returns the total character width of an anchor's column span
// (including inter-column gaps between visible spanned columns) and
// whether any spanned column is a stretcher.
func (g *Container) spanWidth(a gridlayout.Anchor, plan gridlayout.Plan) (int, bool) {
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
		totalWidth += (visible - 1) * InterColumnGap
	}
	return totalWidth, hasStretcher
}

// lastVisibleSpannedCol returns the index of the last non-dropped column
// within an anchor's horizontal span.
func (g *Container) lastVisibleSpannedCol(a gridlayout.Anchor, plan gridlayout.Plan) int {
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
func (g *Container) anchorPlacementHeight(a gridlayout.Anchor, plan gridlayout.Plan) int {
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
		totalWidth += (visible - 1) * InterColumnGap
	}
	h := g.heightOf(a.Name, totalWidth)
	if h < 1 {
		h = 1
	}
	return h
}
