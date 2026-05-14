package taskdetail

import "github.com/boolean-maybe/tiki/gridlayout"

// rowsPerPackedColumn is the per-column height used when packing a flat
// metadata field list into a multi-column grid. Matches the visible
// grid-body height (metadataGridHeight) so packed columns fill the box
// without clipping when each field is single-row.
const rowsPerPackedColumn = metadataGridHeight

// singleColumnSpec synthesizes a 1-column metadata grid from a flat
// field name list — one anchor per row, in declaration order. Useful for
// tests; production paths that take a flat list (TaskEditView) should
// use greedyPackedSpec instead so fields don't overflow the visible
// grid body.
func singleColumnSpec(names []string) gridlayout.GridSpec {
	if len(names) == 0 {
		return gridlayout.GridSpec{}
	}
	anchors := make([]gridlayout.Anchor, len(names))
	cells := make([][]gridlayout.Cell, len(names))
	for i, n := range names {
		anchors[i] = gridlayout.Anchor{
			Name: n, Row: i, Col: 0, RowSpan: 1, ColSpan: 1,
		}
		cells[i] = []gridlayout.Cell{gridlayout.FieldCell{Name: n}}
	}
	return gridlayout.GridSpec{
		Rows:      len(names),
		Cols:      1,
		Anchors:   anchors,
		Stretcher: []bool{false},
		Cells:     cells,
	}
}

// greedyPackedSpec packs a flat metadata field name list into a
// multi-column grid: each column holds up to rowsPerPackedColumn fields
// in declaration order, then a new column starts. Used by TaskEditView
// to preserve the legacy 4-rows-per-column visual layout when the
// caller hasn't supplied a parsed grid.
func greedyPackedSpec(names []string) gridlayout.GridSpec {
	if len(names) == 0 {
		return gridlayout.GridSpec{}
	}
	cols := (len(names) + rowsPerPackedColumn - 1) / rowsPerPackedColumn
	rows := rowsPerPackedColumn
	if len(names) < rows {
		rows = len(names)
	}
	anchors := make([]gridlayout.Anchor, 0, len(names))
	cells := make([][]gridlayout.Cell, rows)
	for r := 0; r < rows; r++ {
		cells[r] = make([]gridlayout.Cell, cols)
		for c := 0; c < cols; c++ {
			cells[r][c] = gridlayout.EmptyCell{}
		}
	}
	for i, n := range names {
		c := i / rowsPerPackedColumn
		r := i % rowsPerPackedColumn
		anchors = append(anchors, gridlayout.Anchor{
			Name: n, Row: r, Col: c, RowSpan: 1, ColSpan: 1,
		})
		cells[r][c] = gridlayout.FieldCell{Name: n}
	}
	stretcher := make([]bool, cols)
	return gridlayout.GridSpec{
		Rows:      rows,
		Cols:      cols,
		Anchors:   anchors,
		Stretcher: stretcher,
		Cells:     cells,
	}
}
