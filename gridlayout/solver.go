package gridlayout

// PlacedAnchor is the resolved placement of one anchor: grid coordinates
// plus its final width and height in character cells.
type PlacedAnchor struct {
	Name             string
	Row, Col         int
	RowSpan, ColSpan int
	Width, Height    int
}

// Plan is the resolved layout for a GridSpec at a particular terminal
// width. ColumnWidths and RowHeights have the original GridSpec.Cols /
// GridSpec.Rows length; columns dropped by right-to-left shedding have a
// width of 0 and are listed in DroppedAnchors.
type Plan struct {
	Cols, Rows     int
	ColumnWidths   []int
	RowHeights     []int
	Gap            int
	Placed         []PlacedAnchor
	Dropped        []bool // len == Cols; true if column was shed
	DroppedAnchors []string
}

// SolveLayout computes column widths, row heights, and per-anchor
// placements given a parsed spec, the total available width, the
// inter-column / inter-row gap, a content-measure callback, and a callback
// that reports each field's natural height at a given inner width.
//
// measure(a) returns the content width of an anchor (literals measure their
// text; fields measure their rendered value — supplied by the caller). It
// sizes `auto` columns and supplies the implicit min-content floor of an
// unbounded `fr` column.
//
// Width algorithm:
//  1. Each column has a base width. A column's mode is `grow` iff a
//     single-column SizeGrow anchor sits in it. Non-grow columns are sized by
//     mode: fixed = Min; auto = clamp(measure, Min/1, Max/∞). A spanning anchor
//     contributes only a minimum split across its non-grow columns; it never
//     sets a column's mode.
//  2. If required width (non-grow base widths + grow columns' floors + gaps)
//     exceeds the available width, shed by position priority: drop the
//     rightmost visible column — grow or not — and retry. Leftmost columns
//     (the core identity/status fields) survive longest; rightmost (optional)
//     content sheds first. A floored grow column at the right end sheds before
//     any interior column rather than clinging on as a useless sliver.
//  3. Grow columns split the residual width left after fixed/content columns
//     in proportion to their weights; leftover units go to the last grow.
//
// Height algorithm:
//  1. Initialize every row height to 1.
//  2. For each anchor, ask heightOf for its natural height at its total
//     allocated width. If that exceeds the sum of its spanned rows, grow
//     the last row in its span by the excess.
//  3. Cap each row at maxRowHeight to bound runaway content.
func SolveLayout(spec GridSpec, width, gap int, measure func(a Anchor) int, heightOf func(a Anchor, w int) int) Plan {
	if measure == nil {
		measure = func(Anchor) int { return 1 }
	}
	if heightOf == nil {
		heightOf = func(Anchor, int) int { return 1 }
	}

	cols := spec.Cols
	rows := spec.Rows

	plan := Plan{
		Cols:         cols,
		Rows:         rows,
		ColumnWidths: make([]int, cols),
		RowHeights:   make([]int, rows),
		Gap:          gap,
		Dropped:      make([]bool, cols),
	}
	if cols == 0 || rows == 0 {
		return plan
	}

	grow, weight := computeGrowColumns(spec)
	base := computeColumnWidths(spec, grow, measure)
	growFloors := computeGrowFloors(spec, grow)

	// required width = non-grow base widths + grow columns' floors. Counting a
	// grow column's floor (its explicit `:MIN..fr` minimum) means a grow column
	// that cannot be given at least its floor triggers further shedding instead
	// of silently shrinking to a useless sliver — e.g. a `dependsOn:8..fr`
	// column is shed rather than clipped to "1E" when the box runs out of room.
	// A floorless grow column (plain `:fr`) contributes 0 and keeps its old
	// behavior (absorb whatever residual remains).
	computeFixed := func() int {
		s := 0
		visible := 0
		for c := 0; c < cols; c++ {
			if plan.Dropped[c] {
				continue
			}
			visible++
			if grow[c] {
				s += growFloors[c]
			} else {
				s += base[c]
			}
		}
		if visible > 1 {
			s += (visible - 1) * gap
		}
		return s
	}

	for computeFixed() > width {
		victim := rightmostVisibleColumn(cols, plan.Dropped)
		if victim < 0 {
			break
		}
		plan.Dropped[victim] = true
	}

	// Hard rule: a field's value and its `.caption` shed together. The two may
	// live in different columns (caption-beside-value layouts), so dropping a
	// value column would otherwise orphan its caption in the surviving caption
	// column. dropCoShedColumns drops any further column whose every anchor
	// belongs to a field already partially shed — collapsing a caption-only or
	// value-only remnant so no half of a field ever survives alone.
	dropCoShedColumns(spec, plan.Dropped)

	for _, a := range spec.Anchors {
		allDropped := true
		for cc := a.Col; cc < a.Col+a.ColSpan; cc++ {
			if !plan.Dropped[cc] {
				allDropped = false
				break
			}
		}
		if allDropped {
			plan.DroppedAnchors = append(plan.DroppedAnchors, a.Name)
		}
	}

	for c := 0; c < cols; c++ {
		if plan.Dropped[c] {
			plan.ColumnWidths[c] = 0
			continue
		}
		if !grow[c] {
			plan.ColumnWidths[c] = base[c]
		}
	}

	assignGrowWidths(&plan, width, gap, grow, weight)

	for i := range plan.RowHeights {
		plan.RowHeights[i] = 1
	}

	for _, a := range spec.Anchors {
		allDropped := true
		visible := 0
		totalWidth := 0
		for cc := a.Col; cc < a.Col+a.ColSpan; cc++ {
			if plan.Dropped[cc] {
				continue
			}
			allDropped = false
			visible++
			totalWidth += plan.ColumnWidths[cc]
		}
		if allDropped {
			continue
		}
		if visible > 1 {
			totalWidth += (visible - 1) * gap
		}
		h := anchorHeight(a, totalWidth, heightOf)
		if h < 1 {
			h = 1
		}
		currentTotal := 0
		for rr := a.Row; rr < a.Row+a.RowSpan; rr++ {
			currentTotal += plan.RowHeights[rr]
		}
		if h > currentTotal {
			plan.RowHeights[a.Row+a.RowSpan-1] += h - currentTotal
		}
	}

	const maxRowHeight = 6
	for i, h := range plan.RowHeights {
		if h > maxRowHeight {
			plan.RowHeights[i] = maxRowHeight
		}
	}

	for _, a := range spec.Anchors {
		allDropped := true
		visible := 0
		totalWidth := 0
		for cc := a.Col; cc < a.Col+a.ColSpan; cc++ {
			if plan.Dropped[cc] {
				continue
			}
			allDropped = false
			visible++
			totalWidth += plan.ColumnWidths[cc]
		}
		if allDropped {
			continue
		}
		if visible > 1 {
			totalWidth += (visible - 1) * gap
		}
		totalHeight := 0
		for rr := a.Row; rr < a.Row+a.RowSpan; rr++ {
			totalHeight += plan.RowHeights[rr]
		}
		if a.RowSpan > 1 {
			totalHeight += (a.RowSpan - 1) * gap
		}
		plan.Placed = append(plan.Placed, PlacedAnchor{
			Name:    a.Name,
			Row:     a.Row,
			Col:     a.Col,
			RowSpan: a.RowSpan,
			ColSpan: a.ColSpan,
			Width:   totalWidth,
			Height:  totalHeight,
		})
	}

	return plan
}

// anchorHeight returns the natural height (in rows) of an anchor at the given
// total width. A `.caption` or `.count` field anchor renders exactly one line
// (label text / item count) — never the field's wrapped value — so it is always
// height 1; without this short-circuit such a cell next to a multi-row list
// value would inflate its row to the value's height and orphan it above a gap.
// All other anchors defer to the caller-supplied height callback.
func anchorHeight(a Anchor, width int, heightOf func(a Anchor, w int) int) int {
	if a.Kind == AnchorField && a.Display.IsSingleLineDisplay() {
		return 1
	}
	return heightOf(a, width)
}

// GrowColumns reports, per column, whether that column grows to absorb residual
// width (its mode is SizeGrow). View code uses this to give grow columns flex
// proportion at draw time. See computeGrowColumns for the rule.
func GrowColumns(spec GridSpec) []bool {
	grow, _ := computeGrowColumns(spec)
	return grow
}

// computeGrowColumns builds the per-column grow flag and weight. A column is
// grow iff a single-column (ColSpan==1) SizeGrow anchor declares it. A spanning
// SizeGrow anchor never sets a column's mode; it is silently ignored here and
// contributes only a minimum in computeColumnWidths.
func computeGrowColumns(spec GridSpec) (grow []bool, weight []int) {
	grow = make([]bool, spec.Cols)
	weight = make([]int, spec.Cols)
	for _, a := range spec.Anchors {
		if a.ColSpan != 1 || a.Sizing.Mode != SizeGrow {
			continue
		}
		grow[a.Col] = true
		w := a.Sizing.Weight
		if w < 1 {
			w = 1
		}
		if w > weight[a.Col] {
			weight[a.Col] = w
		}
	}
	return grow, weight
}

// computeGrowFloors returns, per grow column, the minimum width it must be
// granted to remain visible. A grow column with an explicit `:MIN..fr` floor
// (Sizing.MinSet) contributes that MIN; a plain `:fr` column contributes 0,
// preserving its "absorb whatever residual remains, never force a shed"
// behavior. Non-grow columns are 0 here (they are accounted via `base`).
func computeGrowFloors(spec GridSpec, grow []bool) []int {
	floors := make([]int, spec.Cols)
	for _, a := range spec.Anchors {
		if a.ColSpan != 1 || a.Sizing.Mode != SizeGrow || !a.Sizing.MinSet {
			continue
		}
		if a.Col < len(grow) && grow[a.Col] && a.Sizing.Min > floors[a.Col] {
			floors[a.Col] = a.Sizing.Min
		}
	}
	return floors
}

// computeColumnWidths returns per-column base (preferred) widths. Non-grow
// columns are sized by mode; grow columns get base 0 (their width is assigned
// from residual). A spanning anchor distributes its preferred width across its
// non-grow columns (remainder to the rightmost), contributing only a minimum.
func computeColumnWidths(spec GridSpec, grow []bool, measure func(a Anchor) int) (base []int) {
	base = make([]int, spec.Cols)
	for _, a := range spec.Anchors {
		want := anchorBaseWidth(a, measure)
		distributeAcrossColumns(a, grow, want, base)
	}
	for c := 0; c < spec.Cols; c++ {
		if grow[c] {
			continue
		}
		if base[c] < 1 {
			base[c] = 1
		}
	}
	return base
}

// anchorBaseWidth is the preferred width an anchor wants for its whole span.
func anchorBaseWidth(a Anchor, measure func(a Anchor) int) int {
	switch a.Sizing.Mode {
	case SizeFixed:
		return a.Sizing.Min
	case SizeGrow:
		return growFloor(a, measure)
	default: // SizeAuto
		return clampWidth(measure(a), a.Sizing)
	}
}

// growFloor is the min-content floor of a grow anchor: its explicit Min when
// set, otherwise its measured content (so a plain :fr never shrinks below its
// content unless an explicit :0.. floor was given).
func growFloor(a Anchor, measure func(a Anchor) int) int {
	if a.Sizing.MinSet {
		return a.Sizing.Min
	}
	return measure(a)
}

// clampWidth clamps a measured content width to the sizing's bounds. Max==0
// means unbounded; MinSet==false means floor 1.
func clampWidth(w int, sz Sizing) int {
	lo := 1
	if sz.MinSet {
		lo = sz.Min
	}
	if w < lo {
		w = lo
	}
	if sz.Max > 0 && w > sz.Max {
		w = sz.Max
	}
	if w < 1 {
		w = 1
	}
	return w
}

// distributeAcrossColumns splits an anchor's value evenly across its non-grow
// spanned columns (remainder to the rightmost), taking the per-column max.
func distributeAcrossColumns(a Anchor, grow []bool, value int, target []int) {
	if value < 1 {
		value = 1
	}
	per := value / a.ColSpan
	remainder := value - per*a.ColSpan
	for cc := a.Col; cc < a.Col+a.ColSpan; cc++ {
		if cc >= len(grow) || grow[cc] {
			continue
		}
		local := per
		if cc == a.Col+a.ColSpan-1 {
			local += remainder
		}
		if local > target[cc] {
			target[cc] = local
		}
	}
}

// dropCoShedColumns enforces the caption↔value co-shedding rule at column
// granularity. A field is "partially shed" when at least one of its anchors
// (its value or its `.caption`) sits in an already-dropped column. Any visible
// column whose every field anchor belongs to a partially-shed field is then
// dropped too — so a caption-beside-value pair never survives as a lone caption
// column (or lone value column). Iterates to a fixed point because dropping one
// column can newly partially-shed a field whose other anchor anchors another
// column. Columns holding at least one wholly-surviving field are kept; lone
// orphaned cells in such mixed columns are handled per-anchor by
// SuppressedAnchorAt at render time.
func dropCoShedColumns(spec GridSpec, dropped []bool) {
	for {
		shedFields := partiallyShedFields(spec, dropped)
		if len(shedFields) == 0 {
			return
		}
		changed := false
		for c := 0; c < len(dropped); c++ {
			if dropped[c] || !columnAllShed(spec, c, shedFields) {
				continue
			}
			dropped[c] = true
			changed = true
		}
		if !changed {
			return
		}
	}
}

// fieldNamed reports whether an anchor carries a field name to pair on — a
// plain field anchor (value or `.caption`) or a single-field composite (e.g.
// `status.label + " " + status.visual`, Name=="status"). Literal anchors carry
// no field identity and never participate in co-shedding.
func fieldNamed(a Anchor) bool {
	switch a.Kind {
	case AnchorField:
		return a.Name != ""
	case AnchorComposite:
		return a.Name != ""
	}
	return false
}

// partiallyShedFields returns the set of field names that have at least one
// name-bearing anchor (value, `.caption`, or single-field composite) in a
// dropped column.
func partiallyShedFields(spec GridSpec, dropped []bool) map[string]struct{} {
	shed := make(map[string]struct{})
	for _, a := range spec.Anchors {
		if !fieldNamed(a) {
			continue
		}
		for cc := a.Col; cc < a.Col+a.ColSpan && cc < len(dropped); cc++ {
			if dropped[cc] {
				shed[a.Name] = struct{}{}
				break
			}
		}
	}
	return shed
}

// columnAllShed reports whether column c holds at least one name-bearing anchor
// and every such anchor belongs to a partially-shed field — i.e. nothing
// wholly-surviving would be lost by dropping the column.
func columnAllShed(spec GridSpec, c int, shedFields map[string]struct{}) bool {
	hasField := false
	for _, a := range spec.Anchors {
		if !fieldNamed(a) {
			continue
		}
		if c < a.Col || c >= a.Col+a.ColSpan {
			continue
		}
		hasField = true
		if _, shed := shedFields[a.Name]; !shed {
			return false
		}
	}
	return hasField
}

// SuppressedAnchorAt reports whether the field anchor at (row,col) must be
// blanked at render time to honor the caption↔value co-shedding rule: its
// column survived, but the field's other half (value or caption) lives in a
// dropped column, so rendering this half alone would orphan it. Mixed columns
// (holding both surviving and shed fields) can't be dropped wholesale by
// dropCoShedColumns, so the renderer consults this per anchor.
func (p Plan) SuppressedAnchorAt(spec GridSpec, name string, row, col int) bool {
	if name == "" {
		return false
	}
	// is this anchor's own column dropped? then it's already not rendered.
	if col < len(p.Dropped) && p.Dropped[col] {
		return false
	}
	// suppress when any OTHER name-bearing anchor of the same field (value,
	// `.caption`, or single-field composite) is in a dropped column.
	for _, a := range spec.Anchors {
		if !fieldNamed(a) || a.Name != name {
			continue
		}
		if a.Row == row && a.Col == col {
			continue
		}
		for cc := a.Col; cc < a.Col+a.ColSpan && cc < len(p.Dropped); cc++ {
			if p.Dropped[cc] {
				return true
			}
		}
	}
	return false
}

// rightmostVisibleColumn returns the highest-index column not yet dropped, or -1
// when every column is gone. This is the position-priority shedding victim: the
// layout sheds from the right, so leftmost (core identity/status) columns survive
// longest and rightmost (optional) content sheds first — grow and non-grow alike.
// A floored grow column at the right end is therefore shed before any interior
// column, instead of clinging on as a useless sliver while a core column dies.
func rightmostVisibleColumn(cols int, dropped []bool) int {
	for c := cols - 1; c >= 0; c-- {
		if !dropped[c] {
			return c
		}
	}
	return -1
}

// assignGrowWidths splits the residual width among visible grow columns in
// proportion to their weights; leftover units go to the last grow column.
func assignGrowWidths(plan *Plan, width, gap int, grow []bool, weight []int) {
	visibleCols, totalWeight, fixedSum := 0, 0, 0
	for c := 0; c < plan.Cols; c++ {
		if plan.Dropped[c] {
			continue
		}
		visibleCols++
		if grow[c] {
			totalWeight += weight[c]
		} else {
			fixedSum += plan.ColumnWidths[c]
		}
	}
	if totalWeight == 0 {
		return
	}
	residual := width - fixedSum
	if visibleCols > 1 {
		residual -= (visibleCols - 1) * gap
	}
	if residual < 0 {
		residual = 0
	}
	per := residual / totalWeight
	assignedWeight := 0
	for c := 0; c < plan.Cols; c++ {
		if plan.Dropped[c] || !grow[c] {
			continue
		}
		w := per * weight[c]
		assignedWeight += weight[c]
		if assignedWeight == totalWeight {
			w += residual - per*totalWeight // leftover units to the last grow
		}
		if w < 1 {
			w = 1
		}
		plan.ColumnWidths[c] = w
	}
}

// TotalHeight returns the sum of row heights plus inter-row gaps. Useful
// for callers sizing the outer container.
func (p Plan) TotalHeight() int {
	s := 0
	for _, h := range p.RowHeights {
		s += h
	}
	if p.Rows > 1 {
		s += (p.Rows - 1) * p.Gap
	}
	return s
}
