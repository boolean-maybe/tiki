// Package gridlayout implements the grid layout DSL used by
// workflow.yaml's `layout:` field on board, list, and detail views. The
// DSL describes a 2D grid of cells written as a YAML block scalar (`|`)
// with one row per line and cells separated by `|`:
//
//	layout: |
//	  title       | --       | --  | --      | --
//	  "Status:"   | status   | <-> | tags:30 | depends:25
//	  "Type:"     | type     | <-> | ^       | ^
//	  "Priority:" | priority | <-> | _       | _
//
// Cell vocabulary:
//
//	name        field, value-only (no caption), system-default width
//	name:N      field, preferred + minimum width of N chars
//	"any text"  literal caption (any double-quoted string that is not a
//	            bare identifier or marker); used to label adjacent fields
//	--          column span (continue the anchor to the left)
//	^           row span (continue the anchor above)
//	_           empty cell
//	<->         horizontal stretcher (absorbs remaining space)
//
// Fields are rendered value-only. Captions are placed by the layout
// author as literal cells.
//
// The package is parser + solver only — it has no tview or config
// dependencies, so plugin/ and view/ can both import it without cycles.
package gridlayout

// Cell is one parsed grid cell.
type Cell interface{ isCell() }

// DisplayMode controls how an enum field's value is rendered in the detail
// view grid. Label shows the human-readable label; Visual shows the compact
// visual indicator (emoji/icon). Label is the default when unspecified.
type DisplayMode int

const (
	DisplayLabel  DisplayMode = iota // default: show Label
	DisplayVisual                    // show Visual
)

// FieldCell is a non-span field anchor cell. WantedWidth is 0 when the
// user did not write `:N`; otherwise it is the explicit preferred + minimum
// width in character cells. Role is the semantic color role from `<role>`
// prefix markup; empty when no role was declared. Display controls the
// enum rendering mode (`.label` or `.visual` suffix); zero value = label.
type FieldCell struct {
	Name        string
	Role        string
	Modifier    string // optional; one of theme.KnownModifierNames or ""
	Display     DisplayMode
	WantedWidth int
}

// LiteralCell is a static-text cell. Text is the literal string as it was
// written in the workflow (whitespace trimmed). It contributes a width
// hint based on Text length and renders as a static text primitive. When
// the cell is row-spanned (Anchor.RowSpan > 1), the renderer word-wraps
// the text across the spanned region — see view/tikidetail.renderLiteralAnchor.
type LiteralCell struct {
	Text string
}

// ColSpanCell extends the anchor immediately to its left.
type ColSpanCell struct{}

// RowSpanCell extends the anchor immediately above. Both `^` (bare-legal
// in YAML) and `|` (requires quoting) tokenize to this cell type.
type RowSpanCell struct{}

// EmptyCell renders nothing and contributes no width hint.
type EmptyCell struct{}

// StretcherCell marks a column whose width is computed from residual space
// after every fixed-width column has been satisfied.
type StretcherCell struct{}

func (FieldCell) isCell()     {}
func (LiteralCell) isCell()   {}
func (ColSpanCell) isCell()   {}
func (RowSpanCell) isCell()   {}
func (EmptyCell) isCell()     {}
func (StretcherCell) isCell() {}

// SegmentKind distinguishes field-reference segments from literal-text segments
// within a CompositeCell.
type SegmentKind int

const (
	SegmentField SegmentKind = iota
	SegmentLiteral
)

// Segment is one part of a CompositeCell — either a field reference or a
// literal string. Field segments carry optional Role, Display, and WantedWidth
// (same semantics as FieldCell). Literal segments carry Text only.
type Segment struct {
	Kind        SegmentKind
	Name        string
	Text        string
	Role        string
	Modifier    string // optional; one of theme.KnownModifierNames or ""
	Display     DisplayMode
	WantedWidth int
}

// CompositeCell concatenates multiple segments (field refs and/or literals)
// separated by ` + ` in the DSL. At least one segment must be a field ref;
// an all-literal composite falls through to LiteralCell for backwards compat.
type CompositeCell struct {
	Segments []Segment
}

func (CompositeCell) isCell() {}

// AnchorKind distinguishes a field anchor (renders a tiki field's value)
// from a literal anchor (renders fixed text declared in the layout).
type AnchorKind int

const (
	AnchorField AnchorKind = iota
	AnchorLiteral
	AnchorComposite
)

// Anchor is one placed cell-with-content in the grid: a field anchor or a
// literal text anchor. Row/Col is the top-left corner; RowSpan/ColSpan
// describe its extent. WantedWidth carries the user's :N hint (0 when
// absent), applied across the spanned columns by the solver.
//
// For Kind == AnchorField, Name holds the field name; Text is empty.
// For Kind == AnchorLiteral, Text holds the literal string; Name is empty
// (callers must use Kind to distinguish, not Name presence — a future
// feature could allow naming literals).
type Anchor struct {
	Kind        AnchorKind
	Name        string
	Text        string
	Role        string      // semantic color role propagated from FieldCell; empty = default
	Modifier    string      // optional; one of theme.KnownModifierNames or ""
	Display     DisplayMode // enum display mode propagated from FieldCell; zero = label
	Segments    []Segment   // populated only for AnchorComposite
	Row, Col    int
	RowSpan     int
	ColSpan     int
	WantedWidth int
}

// GridSpec is the parsed, validated grid. Anchors are emitted in
// declaration order (top-to-bottom, left-to-right) which is also the
// edit-mode traversal order for field anchors.
type GridSpec struct {
	Rows, Cols int
	Anchors    []Anchor
	Stretcher  []bool // len == Cols; true for stretcher columns
	Cells      [][]Cell
}

// AnchorNames returns the field-anchor names in declaration order. Literal
// anchors are excluded — they are not edit targets and not field references.
// Useful for callers that need a flat field list (e.g. edit-traversal order).
func (s GridSpec) AnchorNames() []string {
	out := make([]string, 0, len(s.Anchors))
	for _, a := range s.Anchors {
		switch a.Kind {
		case AnchorField:
			out = append(out, a.Name)
		case AnchorComposite:
			if a.Name != "" {
				out = append(out, a.Name)
			}
		}
	}
	return out
}

// IsEmpty reports whether the grid has zero anchors. Useful as a guard for
// callers falling back to default metadata.
func (s GridSpec) IsEmpty() bool { return len(s.Anchors) == 0 }
