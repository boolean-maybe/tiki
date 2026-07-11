// Package gridbox renders a parsed gridlayout spec into a width-adaptive
// tview primitive. Two surfaces use it: the detail view's layout grid
// (view-mode only — edit mode owns its own renderer in package tikidetail)
// and the tiki box on board/list views.
//
// What lives here vs. in tikidetail:
//
//   - This package owns the layout primitive (Container) and the
//     layout-solver adapter (SolveGridLayout, MeasureAnchorText, overhead
//     constants). Nothing in this package knows about field names, render
//     context, themes, or any per-field logic.
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
// column's content grew the row (for example, a multi-line list).
//
// Using nested Flexes (rather than tview.Grid) keeps the focus chain
// identical to the legacy renderer — editor widgets receive Tab/Down
// events without the Grid's internal focus traversal interfering.
type Container struct {
	*tview.Flex
	spec           gridlayout.GridSpec
	grow           []bool            // per-column: true for residual-absorbing (SizeGrow) columns
	primitives     []tview.Primitive // indexed by anchor position in spec.Anchors
	measure        func(a gridlayout.Anchor) int
	heightOf       func(a gridlayout.Anchor, width int) int
	lastWidth      int
	selectionBg    tcell.Color
	selectionBgSet bool
}

// NewContainer wires the parsed grid spec, the per-anchor primitives, a
// content-measure callback, and a height-resolver into a horizontal Flex
// wrapper. The first Draw call computes the layout against the live width.
//
// measure reports the content width of an anchor (field anchors read the
// rendered value; non-field anchors should defer to MeasureAnchorText). The
// grow safety nets (single-column, prose-block) are applied once here so the
// stored spec and its derived grow flags stay consistent with what the solver
// sees on every rebuild.
//
// primitives is indexed by anchor position (same order as spec.Anchors)
// so both field and literal anchors can be addressed uniformly.
func NewContainer(spec gridlayout.GridSpec, primitives []tview.Primitive, measure func(a gridlayout.Anchor) int, heightOf func(a gridlayout.Anchor, width int) int) *Container {
	promoted := promoteForGrowth(spec)
	g := &Container{
		Flex:       tview.NewFlex().SetDirection(tview.FlexRow),
		spec:       promoted,
		grow:       gridlayout.GrowColumns(promoted),
		primitives: primitives,
		measure:    measure,
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
//     get padded when a neighbour column grew the row. A residual flex-1 box
//     at the bottom of each column absorbs
//     leftover space.
func (g *Container) rebuild(width int) {
	g.lastWidth = width
	g.Flex.Clear()
	// g.spec is already promoted in NewContainer; solve directly to avoid
	// re-running the (idempotent) promotion on every width change.
	plan := gridlayout.SolveLayout(g.spec, width, InterColumnGap, g.measure, g.heightOf)
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

	// maxRowSpanAt[r] = max RowSpan of any anchor whose Row == r.
	// Used to grow a spanning-row into a spanning-band that consumes
	// max(RowSpan) rows, so a row-spanned literal in the same row as a
	// horizontal-span title has vertical room to render its full text.
	maxRowSpanAt := make([]int, plan.Rows)
	for _, a := range g.spec.Anchors {
		if a.RowSpan > maxRowSpanAt[a.Row] {
			maxRowSpanAt[a.Row] = a.RowSpan
		}
	}

	r := 0
	for r < plan.Rows {
		if hasHSpan[r] {
			rs := maxRowSpanAt[r]
			if rs < 1 {
				rs = 1
			}
			end := r + rs
			if end > plan.Rows {
				end = plan.Rows
			}
			g.addSpanningBand(r, end, plan, anchorAt)
			r = end
		} else {
			start := r
			for r < plan.Rows && !hasHSpan[r] {
				r++
			}
			g.addPackedBand(start, r, plan, anchorAt)
		}
	}
}

// addSpanningBand renders rows [start, end) where row `start` contains at
// least one horizontal-span anchor, AND the band may extend beyond a single
// row to accommodate row-spanned anchors that originate in row `start`.
//
// Layout shape: column-major (FlexColumn outer, FlexRow per column),
// matching addPackedBand. The wrinkle is horizontal spans: when the column
// loop reaches a column `c` whose row `start` contains an anchor with
// ColSpan>1, that anchor is rendered as a single combined-width cell and
// the column loop skips the spanned columns (they were already accounted
// for in the combined width). Cells in trailing rows under the spanning
// anchor's columns are also skipped because the spanning anchor's RowSpan
// determines its vertical extent.
//
// Why this exists: the original addSpanningRow was strictly per-row and
// could not honor RowSpan>1 — a row-spanned anchor in a row containing a
// horizontal span got clipped to 1 row of vertical space. This band-based
// rewrite fixes that by treating the row containing horizontal spans as
// the start of a band whose height is max(RowSpan) of anchors in row `start`.
func (g *Container) addSpanningBand(start, end int, plan gridlayout.Plan, anchorAt map[int]int) {
	bandHeight := 0
	for r := start; r < end; r++ {
		bandHeight += plan.RowHeights[r]
	}

	// Identify columns that are "owned" by a horizontal-span anchor
	// originating in row `start`. spanOwner[c] is the col index of the
	// anchor that owns col `c`; if spanOwner[c] != c, col `c` is a
	// continuation column that the column loop must skip.
	spanOwner := make([]int, plan.Cols)
	for c := range spanOwner {
		spanOwner[c] = c
	}
	for _, a := range g.spec.Anchors {
		if a.Row != start || a.ColSpan <= 1 {
			continue
		}
		for cc := a.Col + 1; cc < a.Col+a.ColSpan && cc < plan.Cols; cc++ {
			spanOwner[cc] = a.Col
		}
	}

	bandFlex := g.newRowFlex(tview.FlexColumn)
	c := 0
	for c < plan.Cols {
		if plan.Dropped[c] {
			c++
			continue
		}
		colFlex := g.newRowFlex(tview.FlexRow)
		// hSpanCols is the number of columns this colFlex consumes —
		// usually 1, but bumped when col `c` is an h-span owner.
		hSpanCols := 1
		colWidth := plan.ColumnWidths[c]
		hasStretcher := g.grow[c]
		if idx, ok := anchorAt[start*plan.Cols+c]; ok {
			a := g.spec.Anchors[idx]
			if a.ColSpan > 1 {
				hSpanCols = a.ColSpan
				colWidth, hasStretcher = g.spanWidth(a, plan)
			}
		}
		// Walk rows [start, end) within this column band, anchor by anchor.
		// For h-span owner columns the spanning anchor's row range is
		// already covered by RowSpan, but trailing rows in cols that are
		// NOT covered by the h-span may still have their own anchors.
		r := start
		for r < end {
			if idx, ok := anchorAt[r*plan.Cols+c]; ok {
				a := g.spec.Anchors[idx]
				prim := g.primitives[idx]
				if prim == nil || plan.SuppressedAnchorAt(g.spec, a.Name, a.Row, a.Col) {
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

		proportion := 0
		if hasStretcher {
			proportion = 1
			colWidth = 0
		}
		bandFlex.AddItem(colFlex, colWidth, proportion, false)
		// Inter-column gap: place after the LAST visible spanned column.
		lastSpanned := c + hSpanCols - 1
		if lastSpanned >= plan.Cols {
			lastSpanned = plan.Cols - 1
		}
		if lastSpanned < plan.Cols-1 && !plan.Dropped[lastSpanned+1] {
			bandFlex.AddItem(g.newSpacer(), InterColumnGap, 0, false)
		}
		c += hSpanCols
		// Skip continuation columns owned by h-span anchors. (Defensive:
		// hSpanCols already advances past them, but spanOwner is the
		// authoritative skip set in case future code adds more h-spans.)
		for c < plan.Cols && spanOwner[c] != c {
			c++
		}
	}
	g.Flex.AddItem(bandFlex, bandHeight, 0, false)
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
				if prim == nil || plan.SuppressedAnchorAt(g.spec, a.Name, a.Row, a.Col) {
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
		if g.grow[c] {
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
// whether any spanned column grows.
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
		if g.grow[cc] {
			hasStretcher = true
		}
	}
	if visible > 1 {
		totalWidth += (visible - 1) * InterColumnGap
	}
	return totalWidth, hasStretcher
}

// anchorPlacementHeight returns the height to allocate for an anchor's
// primitive within its column. For field anchors, this is the anchor's
// natural height (from heightOf); for single-row literals it is fixed at 1.
// Row-spanned literals (RowSpan > 1) get their declared row span so the
// wrapping prose-block renderer (renderLiteralAnchor) has vertical space
// to draw multiple wrapped lines. Critically it is NOT the solver's
// row-band sum — see rebuild() for the rationale.
func (g *Container) anchorPlacementHeight(a gridlayout.Anchor, plan gridlayout.Plan) int {
	if a.Kind == gridlayout.AnchorLiteral {
		if a.RowSpan > 1 {
			return a.RowSpan
		}
		return 1
	}
	// Row-spanned composites are prose blocks (renderCompositePrimitive enables
	// word-wrap when RowSpan>1). Allocate the declared row span so wrapped
	// lines have vertical space to draw, mirroring the row-spanned literal
	// path above. Single-row composites fall through to the field-style
	// natural-height computation below.
	if a.Kind == gridlayout.AnchorComposite && a.RowSpan > 1 {
		return a.RowSpan
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
	// A `.caption` or `.count` field anchor is a single line (label text / item
	// count), never the field's wrapped value — match the solver's anchorHeight
	// rule so the cell occupies exactly one row and sits flush above its value.
	if a.Display.IsSingleLineDisplay() {
		return 1
	}
	h := g.heightOf(a, totalWidth)
	if h < 1 {
		h = 1
	}
	return h
}
