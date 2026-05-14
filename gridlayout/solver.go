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
// inter-column / inter-row gap, and a callback that reports each field's
// natural height at a given inner width.
//
// Width algorithm:
//  1. For each non-stretcher column, take the max of the WantedWidth (or
//     DefaultWidth) contributed by anchors whose columns include it. An
//     anchor spanning multiple columns distributes its wanted width
//     evenly across them (remainder to the rightmost spanned column).
//  2. If the sum of minimums plus gaps exceeds the available width, drop
//     the rightmost non-stretcher column. Repeat until it fits or only
//     stretcher columns remain.
//  3. Stretcher columns split the residual width equally; leftover units
//     go to the last stretcher.
//
// Height algorithm:
//  1. Initialize every row height to 1.
//  2. For each anchor, ask heightOf for its natural height at its total
//     allocated width. If that exceeds the sum of its spanned rows, grow
//     the last row in its span by the excess.
//  3. Cap each row at maxRowHeight to bound runaway content.
func SolveLayout(spec GridSpec, width, gap int, defaultWidth func(name string) int, heightOf func(name string, w int) int) Plan {
	if defaultWidth == nil {
		defaultWidth = func(string) int { return 12 }
	}
	if heightOf == nil {
		heightOf = func(string, int) int { return 1 }
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

	minWidth := make([]int, cols)
	for _, a := range spec.Anchors {
		wanted := a.WantedWidth
		if wanted == 0 {
			wanted = defaultWidth(a.Name)
		}
		if wanted < 1 {
			wanted = 1
		}
		per := wanted / a.ColSpan
		remainder := wanted - per*a.ColSpan
		for cc := a.Col; cc < a.Col+a.ColSpan; cc++ {
			if spec.Stretcher[cc] {
				continue
			}
			local := per
			if cc == a.Col+a.ColSpan-1 {
				local += remainder
			}
			if local > minWidth[cc] {
				minWidth[cc] = local
			}
		}
	}
	for c := 0; c < cols; c++ {
		if !spec.Stretcher[c] && minWidth[c] < 1 {
			minWidth[c] = 1
		}
	}

	computeFixed := func() int {
		s := 0
		visible := 0
		for c := 0; c < cols; c++ {
			if plan.Dropped[c] {
				continue
			}
			visible++
			if !spec.Stretcher[c] {
				s += minWidth[c]
			}
		}
		if visible > 1 {
			s += (visible - 1) * gap
		}
		return s
	}

	for computeFixed() > width {
		rightmost := -1
		for c := cols - 1; c >= 0; c-- {
			if !plan.Dropped[c] && !spec.Stretcher[c] {
				rightmost = c
				break
			}
		}
		if rightmost < 0 {
			break
		}
		plan.Dropped[rightmost] = true
	}

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
		if !spec.Stretcher[c] {
			plan.ColumnWidths[c] = minWidth[c]
		}
	}

	visibleCols := 0
	stretcherCols := 0
	fixedSum := 0
	for c := 0; c < cols; c++ {
		if plan.Dropped[c] {
			continue
		}
		visibleCols++
		if spec.Stretcher[c] {
			stretcherCols++
		} else {
			fixedSum += plan.ColumnWidths[c]
		}
	}
	residual := width - fixedSum
	if visibleCols > 1 {
		residual -= (visibleCols - 1) * gap
	}
	if residual < 0 {
		residual = 0
	}
	if stretcherCols > 0 {
		per := residual / stretcherCols
		remainder := residual - per*stretcherCols
		assigned := 0
		for c := 0; c < cols; c++ {
			if plan.Dropped[c] || !spec.Stretcher[c] {
				continue
			}
			w := per
			assigned++
			if assigned == stretcherCols {
				w += remainder
			}
			if w < 1 {
				w = 1
			}
			plan.ColumnWidths[c] = w
		}
	}

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
		h := heightOf(a.Name, totalWidth)
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
