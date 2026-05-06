package taskdetail

// grid_layout.go contains the pure column-packing algorithm for the metadata
// grid used by both ConfigurableDetailView and TaskEditView. It has no tview
// or config imports — integers in, plan out — so it can be tested and swapped
// independently. The plan describes a fixed-shape grid of `rowsPerColumn`
// rows; fields are greedily packed into columns in declaration order, where
// multi-row fields (tags, depends-on) consume more rows in their column.
//
// Geometry invariants the algorithm guarantees (asserted in tests):
//   - len(fields) == 0 → Columns == nil.
//   - For every PlannedField: Row ∈ [0, rowsPerColumn), H ∈ [1, rowsPerColumn],
//     Row + H ≤ rowsPerColumn.
//   - For every Column: Width ≥ 1.
//   - Field declaration order is preserved across the flattened columns.

const (
	// rowsPerColumn is the fixed grid-body height. Fields whose raw height
	// exceeds this value are clamped (the underlying renderer truncates).
	rowsPerColumn = 4
	// interColumnGap is the cell count between adjacent columns.
	interColumnGap = 2
)

// GridField is one input to the grid algorithm. HeightAt resolves the
// number of rows the field needs at a given inner column width. Single-row
// fields return 1 regardless of width; multi-row fields (tags, depends-on)
// return their wrapped row count.
type GridField struct {
	Name     string
	HeightAt func(width int) int
}

// PlannedField is the placement of one field within its column.
type PlannedField struct {
	Name string
	Row  int // 0..rowsPerColumn-1
	H    int // 1..rowsPerColumn
}

// Column is one vertical strip of placed fields with its allocated width.
type Column struct {
	Fields []PlannedField
	Width  int
}

// GridPlan is the full layout output. FixedHeight is always rowsPerColumn so
// callers can size their inner Flex deterministically.
type GridPlan struct {
	Columns     []Column
	FixedHeight int
}

// CalculateGridLayout packs fields into fixed-height columns and computes
// per-column widths. Returns an empty plan (Columns == nil) when no fields
// are supplied. See file-level doc for invariants.
func CalculateGridLayout(width int, fields []GridField) GridPlan {
	if len(fields) == 0 {
		return GridPlan{Columns: nil, FixedHeight: rowsPerColumn}
	}

	estimatedColW := width
	if estimatedColW < 1 {
		estimatedColW = 1
	}
	var columns []Column
	for pass := 0; pass < 2; pass++ {
		columns = packColumns(fields, estimatedColW)
		if len(columns) <= 1 {
			break
		}
		next := (width - (len(columns)-1)*interColumnGap) / len(columns)
		if next < 1 {
			next = 1
		}
		if next == estimatedColW {
			break
		}
		estimatedColW = next
	}

	assignWidths(columns, width)
	return GridPlan{Columns: columns, FixedHeight: rowsPerColumn}
}

// packColumns greedily places fields into columns of capacity rowsPerColumn,
// using estimatedColW to compute each field's height.
func packColumns(fields []GridField, estimatedColW int) []Column {
	var columns []Column
	used := 0
	for _, f := range fields {
		h := clampHeight(f.HeightAt(estimatedColW))
		if len(columns) == 0 || used+h > rowsPerColumn {
			columns = append(columns, Column{})
			used = 0
		}
		idx := len(columns) - 1
		columns[idx].Fields = append(columns[idx].Fields, PlannedField{Name: f.Name, Row: used, H: h})
		used += h
	}
	return columns
}

// assignWidths distributes the available width across columns with a hard
// floor of 1 per column. The last column absorbs the integer remainder.
func assignWidths(columns []Column, width int) {
	n := len(columns)
	if n == 0 {
		return
	}
	totalGap := (n - 1) * interColumnGap
	available := width - totalGap
	if available < n {
		// extreme narrow case: each column gets exactly 1 cell. Gaps and
		// content overflow horizontally; tview clips the overflow visually.
		for i := range columns {
			columns[i].Width = 1
		}
		return
	}
	base := available / n
	remainder := available - base*n
	for i := range columns {
		w := base
		if w < 1 {
			w = 1
		}
		columns[i].Width = w
	}
	columns[n-1].Width += remainder
}

// clampHeight enforces the [1, rowsPerColumn] range on a field's reported
// height. Heights ≤ 0 are bumped to 1 so empty list-fields still render
// their "(none)" placeholder; heights > rowsPerColumn are truncated so the
// grid body stays within its fixed-shape budget.
func clampHeight(h int) int {
	if h < 1 {
		return 1
	}
	if h > rowsPerColumn {
		return rowsPerColumn
	}
	return h
}
