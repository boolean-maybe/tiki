package gridlayout

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/boolean-maybe/tiki/theme"
)

var cellNameRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*)(?:\.(label|visual))?(?::(\d+))?$`)
var cellRolePrefixRe = regexp.MustCompile(`^<([a-z][a-z.]*)>(.+)$`)
var literalSegmentRe = regexp.MustCompile(`^"(.*)"$`)

func parseSegment(raw string) (Segment, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Segment{}, fmt.Errorf("empty segment in composite cell")
	}
	if m := literalSegmentRe.FindStringSubmatch(s); m != nil {
		return Segment{Kind: SegmentLiteral, Text: m[1]}, nil
	}
	remainder := s
	var role, modifier string
	if rm := cellRolePrefixRe.FindStringSubmatch(s); rm != nil {
		role, modifier = theme.SplitRoleModifier(rm[1])
		remainder = rm[2]
	}
	m := cellNameRe.FindStringSubmatch(remainder)
	if m == nil {
		return Segment{}, fmt.Errorf("invalid segment %q in composite cell (not a field ref or \"quoted\" literal)", raw)
	}
	seg := Segment{
		Kind:     SegmentField,
		Name:     m[1],
		Role:     role,
		Modifier: modifier,
		Display:  parseDisplayMode(m[2]),
	}
	if m[3] != "" {
		w, err := strconv.Atoi(m[3])
		if err != nil || w <= 0 {
			return Segment{}, fmt.Errorf("invalid width in composite segment %q", raw)
		}
		seg.WantedWidth = w
	}
	return seg, nil
}

func tryParseComposite(s string) (Cell, bool, error) {
	if !strings.Contains(s, " + ") {
		return nil, false, nil
	}
	parts := strings.Split(s, " + ")
	segments := make([]Segment, 0, len(parts))
	hasField := false
	for _, p := range parts {
		seg, err := parseSegment(p)
		if err != nil {
			return nil, true, err
		}
		if seg.Kind == SegmentField {
			hasField = true
		}
		segments = append(segments, seg)
	}
	if !hasField {
		return nil, false, nil
	}
	return CompositeCell{Segments: segments}, true, nil
}

// TokenizeCell parses a single grid cell string. Whitespace is trimmed.
//
// The classification is content-based, in this order:
//  1. Empty → error.
//  2. Bare marker (--, ^, |, _, <->) → corresponding span/empty/stretcher cell.
//  3. Identifier shape (letter, then letters/digits/underscores/hyphens, optional :N) → FieldCell.
//  4. Anything else → LiteralCell.
//
// `|` is accepted as a synonym for `^` (row-span) so existing workflows
// that quote `|` continue to parse.
//
// Authoring caveat: identifier-shaped typos (e.g. `staus` instead of `status`)
// reach FieldCell and trip schema validation at workflow-load time, surfacing
// a clear error. Non-identifier-shaped typos (e.g. `status:` with stray
// colon, `stat us` with embedded space, or any quoted form that includes
// punctuation) bypass schema validation and become on-screen literal text.
// Reviewing the rendered detail view is the safety net for that class.
func TokenizeCell(s string) (Cell, error) {
	t := strings.TrimSpace(s)
	switch t {
	case "":
		return nil, fmt.Errorf("empty cell")
	case "--":
		return ColSpanCell{}, nil
	case "^", "|":
		return RowSpanCell{}, nil
	case "_":
		return EmptyCell{}, nil
	case "<->":
		return StretcherCell{}, nil
	}

	// composite detection — must come before single-field parsing since
	// composites may contain valid field-ref substrings
	if cell, isComposite, err := tryParseComposite(t); isComposite {
		if err != nil {
			return nil, fmt.Errorf("composite cell %q: %w", t, err)
		}
		return cell, nil
	}

	if rm := cellRolePrefixRe.FindStringSubmatch(t); rm != nil {
		role, modifier := theme.SplitRoleModifier(rm[1])
		remainder := rm[2]
		if m := cellNameRe.FindStringSubmatch(remainder); m != nil {
			fc := FieldCell{Name: m[1], Role: role, Modifier: modifier, Display: parseDisplayMode(m[2])}
			if m[3] != "" {
				w, err := strconv.Atoi(m[3])
				if err != nil || w <= 0 {
					return nil, fmt.Errorf("invalid width in cell %q (want positive integer)", t)
				}
				fc.WantedWidth = w
			}
			return fc, nil
		}
	}
	m := cellNameRe.FindStringSubmatch(t)
	if m == nil {
		return LiteralCell{Text: t}, nil
	}
	fc := FieldCell{Name: m[1], Display: parseDisplayMode(m[2])}
	if m[3] != "" {
		w, err := strconv.Atoi(m[3])
		if err != nil || w <= 0 {
			return nil, fmt.Errorf("invalid width in cell %q (want positive integer)", t)
		}
		fc.WantedWidth = w
	}
	return fc, nil
}

func parseDisplayMode(s string) DisplayMode {
	if s == "visual" {
		return DisplayVisual
	}
	return DisplayLabel
}

// ParseGrid parses a 2D string array (typically straight from YAML) into a
// validated GridSpec. Errors carry the row/col coordinates of the offending
// cell so workflow authors get an actionable diagnostic.
func ParseGrid(raw [][]string) (GridSpec, error) {
	if len(raw) == 0 {
		return GridSpec{}, fmt.Errorf("grid is empty")
	}
	cols := len(raw[0])
	if cols == 0 {
		return GridSpec{}, fmt.Errorf("grid first row is empty")
	}
	rows := len(raw)

	cells := make([][]Cell, rows)
	for r, row := range raw {
		if len(row) != cols {
			return GridSpec{}, fmt.Errorf("row %d has %d cells, expected %d (every row must have the same number of cells)", r, len(row), cols)
		}
		cells[r] = make([]Cell, cols)
		for c, s := range row {
			cell, err := TokenizeCell(s)
			if err != nil {
				return GridSpec{}, fmt.Errorf("row %d, col %d: %w", r, c, err)
			}
			cells[r][c] = cell
		}
	}

	occupancy := make([][]int, rows)
	for r := range occupancy {
		occupancy[r] = make([]int, cols)
		for c := range occupancy[r] {
			occupancy[r][c] = -1
		}
	}

	var anchors []Anchor
	seen := make(map[string]struct{})

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if occupancy[r][c] >= 0 {
				continue
			}
			switch cell := cells[r][c].(type) {
			case FieldCell:
				if _, dup := seen[cell.Name]; dup {
					return GridSpec{}, fmt.Errorf("row %d, col %d: field %q appears more than once", r, c, cell.Name)
				}
				seen[cell.Name] = struct{}{}

				colSpan, rowSpan := computeSpans(cells, r, c, rows, cols)
				idx := len(anchors)
				anchors = append(anchors, Anchor{
					Kind:        AnchorField,
					Name:        cell.Name,
					Role:        cell.Role,
					Modifier:    cell.Modifier,
					Display:     cell.Display,
					Row:         r,
					Col:         c,
					RowSpan:     rowSpan,
					ColSpan:     colSpan,
					WantedWidth: cell.WantedWidth,
				})
				markOccupancy(occupancy, idx, r, c, rowSpan, colSpan)

			case LiteralCell:
				colSpan, rowSpan := computeSpans(cells, r, c, rows, cols)
				idx := len(anchors)
				anchors = append(anchors, Anchor{
					Kind:    AnchorLiteral,
					Text:    cell.Text,
					Row:     r,
					Col:     c,
					RowSpan: rowSpan,
					ColSpan: colSpan,
				})
				markOccupancy(occupancy, idx, r, c, rowSpan, colSpan)

			case CompositeCell:
				fieldNames := make(map[string]struct{})
				var totalWidth int
				for _, seg := range cell.Segments {
					if seg.Kind == SegmentField {
						fieldNames[seg.Name] = struct{}{}
					}
					totalWidth += seg.WantedWidth
				}
				for name := range fieldNames {
					if _, dup := seen[name]; dup {
						return GridSpec{}, fmt.Errorf("row %d, col %d: field %q appears more than once", r, c, name)
					}
					seen[name] = struct{}{}
				}
				anchorName := ""
				if len(fieldNames) == 1 {
					for name := range fieldNames {
						anchorName = name
					}
				}
				colSpan, rowSpan := computeSpans(cells, r, c, rows, cols)
				idx := len(anchors)
				anchors = append(anchors, Anchor{
					Kind:        AnchorComposite,
					Name:        anchorName,
					Segments:    cell.Segments,
					Row:         r,
					Col:         c,
					RowSpan:     rowSpan,
					ColSpan:     colSpan,
					WantedWidth: totalWidth,
				})
				markOccupancy(occupancy, idx, r, c, rowSpan, colSpan)

			case ColSpanCell:
				return GridSpec{}, fmt.Errorf("row %d, col %d: orphan '--' (no anchor to the left to extend)", r, c)
			case RowSpanCell:
				return GridSpec{}, fmt.Errorf("row %d, col %d: orphan row-span (no anchor above to extend); written as '^' or '|'", r, c)
			case EmptyCell, StretcherCell:
				// occupancy stays -1; stretcher consistency checked below
			}
		}
	}

	stretcher := make([]bool, cols)
	for c := 0; c < cols; c++ {
		hasStretcher := false
		hasIncompatible := false
		for r := 0; r < rows; r++ {
			switch cells[r][c].(type) {
			case StretcherCell:
				hasStretcher = true
			case EmptyCell, ColSpanCell:
				// neutral. ColSpanCell here is a pass-through from an
				// anchor in a column to the left, so a column may be
				// stretcher even if a wider anchor (e.g. a title that
				// spans the whole row) crosses it.
			case FieldCell, LiteralCell, RowSpanCell, CompositeCell:
				hasIncompatible = true
			}
		}
		if hasStretcher && hasIncompatible {
			return GridSpec{}, fmt.Errorf("col %d: '<->' must not be mixed with anchored or row-spanned cells in the same column", c)
		}
		stretcher[c] = hasStretcher
	}

	return GridSpec{
		Rows:      rows,
		Cols:      cols,
		Anchors:   anchors,
		Stretcher: stretcher,
		Cells:     cells,
	}, nil
}

// computeSpans walks right and down from (r, c) and returns the colSpan
// and rowSpan of the anchor at that position. Spans only extend through
// `--` (column) and `^`/`|` (row) markers.
func computeSpans(cells [][]Cell, r, c, rows, cols int) (colSpan, rowSpan int) {
	colSpan = 1
	for cc := c + 1; cc < cols; cc++ {
		if _, ok := cells[r][cc].(ColSpanCell); !ok {
			break
		}
		colSpan++
	}

	rowSpan = 1
	for rr := r + 1; rr < rows; rr++ {
		ok := true
		for cc := c; cc < c+colSpan; cc++ {
			if _, isRow := cells[rr][cc].(RowSpanCell); !isRow {
				ok = false
				break
			}
		}
		if !ok {
			break
		}
		rowSpan++
	}
	return colSpan, rowSpan
}

// markOccupancy stamps the anchor index into every cell covered by the
// anchor's span so subsequent iteration skips covered cells.
func markOccupancy(occupancy [][]int, idx, r, c, rowSpan, colSpan int) {
	for rr := r; rr < r+rowSpan; rr++ {
		for cc := c; cc < c+colSpan; cc++ {
			occupancy[rr][cc] = idx
		}
	}
}
