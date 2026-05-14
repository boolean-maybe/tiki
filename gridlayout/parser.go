package gridlayout

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var cellNameRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*)(?::(\d+))?$`)

// TokenizeCell parses a single grid cell string. Whitespace is trimmed.
// Returns an error for empty cells, unrecognized tokens, or `:0` widths.
func TokenizeCell(s string) (Cell, error) {
	t := strings.TrimSpace(s)
	switch t {
	case "":
		return nil, fmt.Errorf("empty cell")
	case "--":
		return ColSpanCell{}, nil
	case "|":
		return RowSpanCell{}, nil
	case "_":
		return EmptyCell{}, nil
	case "<->":
		return StretcherCell{}, nil
	}
	m := cellNameRe.FindStringSubmatch(t)
	if m == nil {
		return nil, fmt.Errorf("invalid cell %q", t)
	}
	fc := FieldCell{Name: m[1]}
	if m[2] != "" {
		w, err := strconv.Atoi(m[2])
		if err != nil || w <= 0 {
			return nil, fmt.Errorf("invalid width in cell %q (want positive integer)", t)
		}
		fc.WantedWidth = w
	}
	return fc, nil
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

				colSpan := 1
				for cc := c + 1; cc < cols; cc++ {
					if _, ok := cells[r][cc].(ColSpanCell); !ok {
						break
					}
					colSpan++
				}

				rowSpan := 1
				for rr := r + 1; rr < rows; rr++ {
					canSpan := true
					for cc := c; cc < c+colSpan; cc++ {
						if _, ok := cells[rr][cc].(RowSpanCell); !ok {
							canSpan = false
							break
						}
					}
					if !canSpan {
						break
					}
					rowSpan++
				}

				idx := len(anchors)
				anchors = append(anchors, Anchor{
					Name:        cell.Name,
					Row:         r,
					Col:         c,
					RowSpan:     rowSpan,
					ColSpan:     colSpan,
					WantedWidth: cell.WantedWidth,
				})
				for rr := r; rr < r+rowSpan; rr++ {
					for cc := c; cc < c+colSpan; cc++ {
						occupancy[rr][cc] = idx
					}
				}

			case ColSpanCell:
				return GridSpec{}, fmt.Errorf("row %d, col %d: orphan '--' (no anchor to the left to extend)", r, c)
			case RowSpanCell:
				return GridSpec{}, fmt.Errorf("row %d, col %d: orphan '|' (no anchor above to extend)", r, c)
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
			case FieldCell, RowSpanCell:
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
