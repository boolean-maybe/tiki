package gridlayout

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/boolean-maybe/tiki/theme"
)

var cellNameRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*)(\?)?(?:\.(label|visual|caption|count))?(?::([A-Za-z0-9.]+))?$`)
var cellRolePrefixRe = regexp.MustCompile(`^<([a-z][a-z.]*)>(.+)$`)
var literalSegmentRe = regexp.MustCompile(`^"(.*)"$`)

func parseSegment(raw string) (Segment, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Segment{}, fmt.Errorf("empty segment in composite cell")
	}

	// peel off optional <role> / <role.mod> prefix once, up front
	var role, modifier string
	remainder := s
	if rm := cellRolePrefixRe.FindStringSubmatch(s); rm != nil {
		role, modifier = theme.SplitRoleModifier(rm[1])
		remainder = rm[2]
	}

	// quoted literal segment
	if lm := literalSegmentRe.FindStringSubmatch(remainder); lm != nil {
		return Segment{
			Kind:     SegmentLiteral,
			Text:     lm[1],
			Role:     role,
			Modifier: modifier,
		}, nil
	}

	// field reference segment
	m := cellNameRe.FindStringSubmatch(remainder)
	if m == nil {
		return Segment{}, fmt.Errorf("invalid segment %q in composite cell (not a field ref or \"quoted\" literal)", raw)
	}
	if m[4] != "" {
		return Segment{}, fmt.Errorf("sizing not allowed on composite segment %q; put it on the cell: (a + b):N", raw)
	}
	// composites ignore the `?` hide suffix (group 2): they do not hide.
	return Segment{
		Kind:     SegmentField,
		Name:     m[1],
		Role:     role,
		Modifier: modifier,
		Display:  parseDisplayMode(m[3]),
	}, nil
}

// peelCompositeSizing extracts an optional cell-level sizing suffix from a
// parenthesized composite. Input forms:
//
//	"(a + b):16.."  → body "a + b", sizing "16..", true, nil
//	"(a + b)"       → body "a + b", sizing "",      true, nil
//	"a + b"         → body "a + b", sizing "",      false, nil  (no parens)
//	"(a + b"        → error (unbalanced)
//
// The scan is quote-aware: a '(' or ')' inside a "..." region is literal text
// and does not open/close the wrapper. Returns hadParens=false (not an error)
// when the cell does not start with '(' — that is the legacy bare composite.
func peelCompositeSizing(s string) (body, sizing string, hadParens bool, err error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "(") {
		return s, "", false, nil
	}
	inQuote := false
	depth := 0
	for i, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == '(' && !inQuote:
			depth++
		case r == ')' && !inQuote:
			depth--
			if depth == 0 {
				body = strings.TrimSpace(s[1:i])
				rest := strings.TrimSpace(s[i+1:])
				if rest == "" {
					return body, "", true, nil
				}
				if !strings.HasPrefix(rest, ":") {
					return "", "", true, fmt.Errorf("unexpected %q after composite ')'; sizing must follow as \":N\"", rest)
				}
				return body, rest[1:], true, nil
			}
		}
	}
	return "", "", true, fmt.Errorf("unbalanced '(' in composite cell %q", s)
}

func tryParseComposite(s string) (Cell, bool, error) {
	body, sizingStr, _, err := peelCompositeSizing(s)
	if err != nil {
		return nil, true, err
	}
	if !strings.Contains(body, " + ") {
		// not a composite. A leading '(' with no " + " inside is not our cell;
		// let the caller's later branches classify it (likely a literal).
		return nil, false, nil
	}
	var sizing Sizing
	if sizingStr != "" {
		sizing, err = ParseSizing(sizingStr)
		if err != nil {
			return nil, true, fmt.Errorf("invalid composite sizing %q: %w", sizingStr, err)
		}
	}
	parts := strings.Split(body, " + ")
	segments := make([]Segment, 0, len(parts))
	for _, p := range parts {
		seg, err := parseSegment(p)
		if err != nil {
			return nil, true, err
		}
		segments = append(segments, seg)
	}
	return CompositeCell{Segments: segments, Sizing: sizing}, true, nil
}

// TokenizeCell parses a single grid cell string. Whitespace is trimmed.
//
// The classification is content-based, in this order:
//  1. Empty → error.
//  2. Bare marker (--, ^, _) → corresponding span/empty cell.
//  3. Identifier shape (letter, then letters/digits/underscores/hyphens,
//     optional `?` hide suffix, optional `.label`/`.visual`/`.caption`/`.count`
//     display suffix, optional `:[mode][min..max]` sizing suffix) → FieldCell.
//  4. Anything else → LiteralCell.
//
// Sizing grammar (after the `:`): "auto" (default, content-sized) | "<int>"
// (fixed) | "[<weight>]fr" (grows by weight), with optional "min..max" bounds
// on any mode. A trailing `?` (before any display/sizing suffix) marks the
// field hide-when-empty.
//
// Authoring caveat: identifier-shaped typos (e.g. `categry` instead of `category`)
// reach FieldCell and trip schema validation at workflow-load time, surfacing
// a clear error. Non-identifier-shaped typos (e.g. `status:` with stray
// colon, `stat us` with embedded space, or any quoted form that includes
// punctuation) bypass schema validation and become on-screen literal text.
// Reviewing the rendered detail view is the safety net for that class.
func TokenizeCell(s string) (Cell, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return nil, fmt.Errorf("empty cell")
	}

	// composite detection runs first on the raw string. A composite has its
	// own per-segment `<role>` grammar, and the cell-level peel below would
	// strip a leading per-segment role and confuse the segmenter.
	if cell, isComposite, err := tryParseComposite(t); isComposite {
		if err != nil {
			return nil, fmt.Errorf("composite cell %q: %w", t, err)
		}
		return cell, nil
	}

	// peel off optional <role> / <role.mod> prefix once
	var role, modifier string
	remainder := t
	if rm := cellRolePrefixRe.FindStringSubmatch(t); rm != nil {
		role, modifier = theme.SplitRoleModifier(rm[1])
		remainder = rm[2]
	}

	// bare markers may not carry a role prefix
	switch remainder {
	case "--", "^", "_":
		if role != "" {
			return nil, fmt.Errorf("role prefix not allowed on bare marker %q", remainder)
		}
		switch remainder {
		case "--":
			return ColSpanCell{}, nil
		case "^":
			return RowSpanCell{}, nil
		case "_":
			return EmptyCell{}, nil
		}
	}

	// quoted literal
	if lm := literalSegmentRe.FindStringSubmatch(remainder); lm != nil {
		return LiteralCell{Text: lm[1], Role: role, Modifier: modifier}, nil
	}

	// field reference
	if m := cellNameRe.FindStringSubmatch(remainder); m != nil {
		sz, err := ParseSizing(m[4])
		if err != nil {
			return nil, fmt.Errorf("invalid sizing in cell %q: %w", t, err)
		}
		return FieldCell{
			Name:          m[1],
			Role:          role,
			Modifier:      modifier,
			Display:       parseDisplayMode(m[3]),
			Sizing:        sz,
			HideWhenEmpty: m[2] == "?",
		}, nil
	}

	// last-resort fallback: treat the (possibly role-stripped) remainder as
	// literal text. Preserves the historical "anything that isn't an
	// identifier or marker becomes a literal" escape hatch for unquoted
	// captions in legacy workflows.
	return LiteralCell{Text: remainder, Role: role, Modifier: modifier}, nil
}

func parseDisplayMode(s string) DisplayMode {
	switch s {
	case "visual":
		return DisplayVisual
	case "caption":
		return DisplayCaption
	case "count":
		return DisplayCount
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

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if occupancy[r][c] >= 0 {
				continue
			}
			switch cell := cells[r][c].(type) {
			case FieldCell:
				colSpan, rowSpan := computeSpans(cells, r, c, rows, cols)
				idx := len(anchors)
				anchors = append(anchors, Anchor{
					Kind:          AnchorField,
					Name:          cell.Name,
					Role:          cell.Role,
					Modifier:      cell.Modifier,
					Display:       cell.Display,
					Row:           r,
					Col:           c,
					RowSpan:       rowSpan,
					ColSpan:       colSpan,
					Sizing:        cell.Sizing,
					HideWhenEmpty: cell.HideWhenEmpty,
				})
				markOccupancy(occupancy, idx, r, c, rowSpan, colSpan)

			case LiteralCell:
				colSpan, rowSpan := computeSpans(cells, r, c, rows, cols)
				idx := len(anchors)
				anchors = append(anchors, Anchor{
					Kind:     AnchorLiteral,
					Text:     cell.Text,
					Role:     cell.Role,
					Modifier: cell.Modifier,
					Row:      r,
					Col:      c,
					RowSpan:  rowSpan,
					ColSpan:  colSpan,
				})
				markOccupancy(occupancy, idx, r, c, rowSpan, colSpan)

			case CompositeCell:
				fieldNames := make(map[string]struct{})
				for _, seg := range cell.Segments {
					if seg.Kind == SegmentField {
						fieldNames[seg.Name] = struct{}{}
					}
				}
				anchorName := ""
				if len(fieldNames) == 1 {
					for name := range fieldNames {
						anchorName = name
					}
				}
				colSpan, rowSpan := computeSpans(cells, r, c, rows, cols)
				idx := len(anchors)
				// a composite sizes to its rendered content (auto) unless the
				// author attached a cell-level suffix via parens: (a + b):N.
				sizing := cell.Sizing
				if sizing == (Sizing{}) {
					sizing = Sizing{Mode: SizeAuto}
				}
				anchors = append(anchors, Anchor{
					Kind:     AnchorComposite,
					Name:     anchorName,
					Segments: cell.Segments,
					Row:      r,
					Col:      c,
					RowSpan:  rowSpan,
					ColSpan:  colSpan,
					Sizing:   sizing,
				})
				markOccupancy(occupancy, idx, r, c, rowSpan, colSpan)

			case ColSpanCell:
				return GridSpec{}, fmt.Errorf("row %d, col %d: orphan '--' (no anchor to the left to extend)", r, c)
			case RowSpanCell:
				return GridSpec{}, fmt.Errorf("row %d, col %d: orphan row-span (no anchor above to extend); written as '^'", r, c)
			case EmptyCell:
				// occupancy stays -1
			}
		}
	}

	return GridSpec{
		Rows:    rows,
		Cols:    cols,
		Anchors: anchors,
		Cells:   cells,
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
