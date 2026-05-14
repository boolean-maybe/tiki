// Package gridlayout implements the metadata-grid layout DSL used by
// workflow.yaml's `views.detail.metadata`. The DSL describes a 2D grid of
// cells:
//
//	metadata:
//	  - [title,    --,        --,   --,      --        ]
//	  - [status,   assignee,  <->,  tags:30, depends:25]
//	  - [type,     createdBy, <->,  |,       |         ]
//	  - [priority, createdAt, <->,  _,       _         ]
//	  - [points,   updatedAt, <->,  _,       _         ]
//
// Cell vocabulary:
//
//	name      field, system-default width
//	name:N    field, preferred + minimum width of N chars
//	--        column span (continue the anchor to the left)
//	|         row span (continue the anchor above)
//	_         empty cell
//	<->       horizontal stretcher (absorbs remaining space)
//
// The package is parser + solver only — it has no tview or config
// dependencies, so plugin/ and view/ can both import it without cycles.
package gridlayout

// Cell is one parsed grid cell.
type Cell interface{ isCell() }

// FieldCell is a non-span field anchor cell. WantedWidth is 0 when the
// user did not write `:N`; otherwise it is the explicit preferred + minimum
// width in character cells.
type FieldCell struct {
	Name        string
	WantedWidth int
}

// ColSpanCell extends the anchor immediately to its left.
type ColSpanCell struct{}

// RowSpanCell extends the anchor immediately above.
type RowSpanCell struct{}

// EmptyCell renders nothing and contributes no width hint.
type EmptyCell struct{}

// StretcherCell marks a column whose width is computed from residual space
// after every fixed-width column has been satisfied.
type StretcherCell struct{}

func (FieldCell) isCell()     {}
func (ColSpanCell) isCell()   {}
func (RowSpanCell) isCell()   {}
func (EmptyCell) isCell()     {}
func (StretcherCell) isCell() {}

// Anchor is one placed field in the grid. Row/Col is the top-left corner;
// RowSpan/ColSpan describe its extent. WantedWidth carries the user's :N
// hint (0 when absent), applied across the spanned columns by the solver.
type Anchor struct {
	Name        string
	Row, Col    int
	RowSpan     int
	ColSpan     int
	WantedWidth int
}

// GridSpec is the parsed, validated grid. Anchors are emitted in
// declaration order (top-to-bottom, left-to-right) which is also the
// edit-mode traversal order.
type GridSpec struct {
	Rows, Cols int
	Anchors    []Anchor
	Stretcher  []bool // len == Cols; true for stretcher columns
	Cells      [][]Cell
}

// AnchorNames returns the anchor field names in declaration order. Useful
// for callers that need a flat field list (e.g. edit-traversal order).
func (s GridSpec) AnchorNames() []string {
	out := make([]string, len(s.Anchors))
	for i, a := range s.Anchors {
		out[i] = a.Name
	}
	return out
}

// IsEmpty reports whether the grid has zero anchors. Useful as a guard for
// callers falling back to default metadata.
func (s GridSpec) IsEmpty() bool { return len(s.Anchors) == 0 }
