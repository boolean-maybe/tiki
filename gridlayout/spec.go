// Package gridlayout implements the grid layout DSL used by
// workflow.yaml's `layout:` field on board, list, and detail views. The
// DSL describes a 2D grid of cells written as a YAML block scalar (`|`)
// with one row per line and cells separated by `|`:
//
//	layout: |
//	  title                  | --       | title:fr | --      | --
//	  <text.label>"Status:"  | status   | tags:fr  | tags:30 | depends:25
//	  <text.label>"Type:"    | type     | _        | ^       | ^
//	  <text.label>"Priority:"| priority | _        | _       | _
//
// Cell vocabulary:
//
//	name                 field, value-only (no caption), sized to its content (auto, uncapped)
//	name:N               field, fixed width of exactly N character cells
//	name:auto            field, size to content; name:auto..N caps at N (then truncates with …)
//	name:fr              field, grows to absorb residual width; name:Wfr takes weight W (default 1)
//	name:MIN..MAX        bounds on any mode (e.g. :8.., :..20, :8..20)
//	name?                hide this field (and its .caption) when the tiki has no value for it
//	name.caption         field's caption text (label), rendered as a static label
//	name.count           item count of a list field (stringList/tikiIdList only), rendered as an integer
//	"any text"           literal text, default role text.primary
//	<role>"any text"     literal text painted with the given role
//	<role.mod>"any text" literal text with role + modifier (e.g. .accent)
//	--                   column span (continue the anchor to the left)
//	^                    row span (continue the anchor above)
//	_                    empty cell
//
// Fields are rendered value-only. Captions may be authored as literal cells,
// or — for fields that carry a caption: in workflow.yaml — referenced via
// `name.caption`, which renders the field's declared caption (falling back to
// the field name). A field may appear more than once in a layout (e.g. a
// `name.caption` cell plus a value cell); the parser does not reject duplicates.
//
// The package is parser + solver only — it has no tview or config
// dependencies, so plugin/ and view/ can both import it without cycles.
//
// HideFields produces a per-render copy of a parsed spec in which the named
// field anchors (and every cell they cover) become empty cells — the supported
// way to omit a field together with its caption for a specific document.
package gridlayout

import "sort"

// Cell is one parsed grid cell.
type Cell interface{ isCell() }

// DisplayMode controls how an enum field's value is rendered in the detail
// view grid. Label shows the human-readable label; Visual shows the compact
// visual indicator (emoji/icon). Label is the default when unspecified.
type DisplayMode int

const (
	DisplayLabel   DisplayMode = iota // default: show Label
	DisplayVisual                     // show Visual
	DisplayCaption                    // show the field's caption (label text, not value)
	DisplayCount                      // show the item count of a list field (valid on stringList/tikiIdList only)
)

// FieldCell is a non-span field anchor cell. Sizing carries the parsed
// `:[mode][min..max]` width spec (zero value = auto, uncapped). HideWhenEmpty
// is set by the `?` suffix. Role is the semantic color role from `<role>`
// prefix markup; empty when no role was declared. Display controls the
// enum rendering mode (`.label` or `.visual` suffix); zero value = label.
type FieldCell struct {
	Name          string
	Role          string
	Modifier      string // optional; one of theme.KnownModifierNames or ""
	Display       DisplayMode
	Sizing        Sizing
	HideWhenEmpty bool // `?` suffix: collapse this cell (and its .caption) when the field has no value
}

// LiteralCell is a static-text cell. Text is the literal string as it was
// written in the workflow (whitespace trimmed, surrounding quotes stripped).
// Role/Modifier carry the optional `<role>` / `<role.modifier>` prefix; when
// Role is empty, the renderer falls back to text.primary. When the cell is
// row-spanned (Anchor.RowSpan > 1), the renderer word-wraps the text across
// the spanned region — see view/tikidetail.renderLiteralAnchor.
type LiteralCell struct {
	Text     string
	Role     string
	Modifier string
}

// ColSpanCell extends the anchor immediately to its left.
type ColSpanCell struct{}

// RowSpanCell extends the anchor immediately above. Both `^` (bare-legal
// in YAML) and `|` (requires quoting) tokenize to this cell type.
type RowSpanCell struct{}

// EmptyCell renders nothing and contributes no width hint.
type EmptyCell struct{}

func (FieldCell) isCell()   {}
func (LiteralCell) isCell() {}
func (ColSpanCell) isCell() {}
func (RowSpanCell) isCell() {}
func (EmptyCell) isCell()   {}

// SegmentKind distinguishes field-reference segments from literal-text segments
// within a CompositeCell.
type SegmentKind int

const (
	SegmentField SegmentKind = iota
	SegmentLiteral
)

// Segment is one part of a CompositeCell — either a field reference or a
// literal string. Field segments carry optional Role and Display. Literal
// segments carry Text only. Composites do not hide, so there is no
// HideWhenEmpty here. Sizing belongs on the cell, not a segment — see
// CompositeCell.Sizing.
type Segment struct {
	Kind     SegmentKind
	Name     string
	Text     string
	Role     string
	Modifier string // optional; one of theme.KnownModifierNames or ""
	Display  DisplayMode
}

// CompositeCell concatenates multiple segments (field refs and/or literals)
// separated by ` + ` in the DSL. At least one segment must be a field ref;
// an all-literal composite falls through to LiteralCell for backwards compat.
// Sizing is the optional cell-level width spec, authored by wrapping the
// composite in parens with a suffix: `(a + b):16..`. Zero value is auto.
type CompositeCell struct {
	Segments []Segment
	Sizing   Sizing
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
// describe its extent. Sizing carries the parsed width spec, applied across
// the spanned columns by the solver. HideWhenEmpty propagates the `?` suffix.
//
// For Kind == AnchorField, Name holds the field name; Text is empty.
// For Kind == AnchorLiteral, Text holds the literal string; Name is empty
// (callers must use Kind to distinguish, not Name presence — a future
// feature could allow naming literals).
type Anchor struct {
	Kind          AnchorKind
	Name          string
	Text          string
	Role          string      // semantic color role propagated from FieldCell; empty = default
	Modifier      string      // optional; one of theme.KnownModifierNames or ""
	Display       DisplayMode // enum display mode propagated from FieldCell; zero = label
	Segments      []Segment   // populated only for AnchorComposite
	Row, Col      int
	RowSpan       int
	ColSpan       int
	Sizing        Sizing
	HideWhenEmpty bool
}

// GridSpec is the parsed, validated grid. Anchors are emitted in
// declaration order (top-to-bottom, left-to-right). Edit-mode Tab
// traversal does not use that order — it uses the column-major order
// produced by AnchorNamesColumnMajor / AnchorDisplaysColumnMajor.
type GridSpec struct {
	Rows, Cols int
	Anchors    []Anchor
	Cells      [][]Cell
}

// AnchorNames returns the field-anchor names in declaration order
// (top-to-bottom, left-to-right). Literal anchors are excluded — they are not
// edit targets and not field references. Useful for callers that need a flat
// field list. Edit-mode Tab traversal uses AnchorNamesColumnMajor instead.
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

// AnchorDisplays returns the DisplayMode of each anchor emitted by
// AnchorNames, in the same order and with the same filtering. The two slices
// are positionally aligned: AnchorDisplays()[i] is the DisplayMode of the
// anchor named AnchorNames()[i]. Callers use this to distinguish display-only
// caption anchors (DisplayCaption) from value anchors of the same field. The
// column-major equivalent is AnchorDisplaysColumnMajor.
func (s GridSpec) AnchorDisplays() []DisplayMode {
	out := make([]DisplayMode, 0, len(s.Anchors))
	for _, a := range s.Anchors {
		switch a.Kind {
		case AnchorField:
			out = append(out, a.Display)
		case AnchorComposite:
			if a.Name != "" {
				out = append(out, a.Display)
			}
		}
	}
	return out
}

// fieldAnchorsColumnMajor collects the field anchors (and named composite
// anchors) — the same filter AnchorNames applies — then stable-sorts them by
// top-left cell, column first then row. Two anchors cannot share a top-left
// cell, so (Col, Row) is a total order: a full-width title at (0,0) sorts
// first, and a row-spanned (`^`) anchor keys off its top-left corner. The
// result is the edit-mode Tab order: down a column, then the next column.
func (s GridSpec) fieldAnchorsColumnMajor() []Anchor {
	out := make([]Anchor, 0, len(s.Anchors))
	for _, a := range s.Anchors {
		switch a.Kind {
		case AnchorField:
			out = append(out, a)
		case AnchorComposite:
			if a.Name != "" {
				out = append(out, a)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Col != out[j].Col {
			return out[i].Col < out[j].Col
		}
		return out[i].Row < out[j].Row
	})
	return out
}

// AnchorNamesColumnMajor returns the field-anchor names in column-major order
// (down a column top-to-bottom, then the next column left-to-right). This is
// the edit-mode Tab traversal order. The declaration-order equivalent is
// AnchorNames.
func (s GridSpec) AnchorNamesColumnMajor() []string {
	anchors := s.fieldAnchorsColumnMajor()
	out := make([]string, len(anchors))
	for i, a := range anchors {
		out[i] = a.Name
	}
	return out
}

// AnchorDisplaysColumnMajor returns the DisplayMode of each anchor emitted by
// AnchorNamesColumnMajor, positionally aligned by construction (both derive
// from the same sorted slice): AnchorDisplaysColumnMajor()[i] is the
// DisplayMode of the anchor named AnchorNamesColumnMajor()[i].
func (s GridSpec) AnchorDisplaysColumnMajor() []DisplayMode {
	anchors := s.fieldAnchorsColumnMajor()
	out := make([]DisplayMode, len(anchors))
	for i, a := range anchors {
		out[i] = a.Display
	}
	return out
}

// IsEmpty reports whether the grid has zero anchors. Useful as a guard for
// callers falling back to default metadata.
func (s GridSpec) IsEmpty() bool { return len(s.Anchors) == 0 }

// IsSingleLineDisplay reports whether a display mode always renders exactly one
// line, independent of the field's value. Caption text and a list field's count
// are both single-line regardless of the underlying field type, so height
// computation must short-circuit on them rather than consult the field's
// (possibly multi-row) natural height.
func (d DisplayMode) IsSingleLineDisplay() bool {
	return d == DisplayCaption || d == DisplayCount
}
