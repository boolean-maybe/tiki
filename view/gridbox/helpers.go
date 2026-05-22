package gridbox

import (
	"strings"

	"github.com/boolean-maybe/tiki/gridlayout"
)

// longestWordWidth returns the length of the longest whitespace-separated
// word in s, or 1 if s contains no words. Used as the column-width hint
// for row-spanned literals so they wrap rather than demand full text width.
func longestWordWidth(s string) int {
	max := 1
	for _, w := range strings.Fields(s) {
		if len(w) > max {
			max = len(w)
		}
	}
	return max
}

// helpers.go is the layout-solver adapter shared by all gridbox callers.
// Width/height solving lives in gridlayout.SolveLayout; this file owns the
// per-field default-width hint and the inter-column gap used as a visual
// separator.

// InterColumnGap is the cell count between adjacent columns in a layout
// box. Kept here (not in gridlayout) because it is a UI-layer constant
// tied to the visual style of the box.
const InterColumnGap = 2

// TikiBoxOverhead is the fixed vertical cost a tiki-card frame adds
// beyond the grid body: 1 top border + 1 bottom border. Tiki cards have
// no inner padding rows, so a layout with N rows renders as N+2 cells
// tall. Used by board/list views.
const TikiBoxOverhead = 2

// DetailBoxOverhead is the fixed vertical cost the detail view's
// metadata box adds beyond the grid body: 1 top border + 1 top padding
// row + 1 spacer row appended after the grid + 1 bottom border = 4.
// Detail views also call SetBorderPadding(1, 0, 2, 2) on the frame; the
// top padding inside that call is what the "+1 top padding" reflects.
// Used by ConfigurableDetailView.
const DetailBoxOverhead = 4

// DefaultAnchorWidth returns the wanted-width hint for an anchor that did
// not declare a `:N` width in the layout grid. Kept conservative so
// stretcher columns absorb generous slack without crowding fixed columns.
//
// Field anchors get a per-name table lookup; literal anchors get a width
// based on their text length so short captions like "Status:" don't crowd
// out adjacent value columns. Row-spanned literals (RowSpan > 1) are
// rendered as wrapping prose blocks by renderLiteralAnchor, so their
// minimum useful width is the longest word — not the full text. Otherwise
// a long row-spanned literal would demand the full text length as a column
// width and get dropped on narrower terminals.
func DefaultAnchorWidth(a gridlayout.Anchor) int {
	if a.Kind == gridlayout.AnchorLiteral {
		if a.RowSpan > 1 {
			return longestWordWidth(a.Text)
		}
		return len(a.Text) + 1
	}
	if a.Kind == gridlayout.AnchorComposite {
		// Composite default width is a hint, not a hard requirement. We
		// take the maximum *single segment*'s default width rather than
		// the sum — composites in tiki cards are expression-like (visual
		// + id, label + value) and the sum vastly overestimates the
		// "minimum useful" width. With a sum, narrow lanes drop the only
		// column entirely because no column ever fits.
		max := 1
		for _, seg := range a.Segments {
			w := 0
			if seg.Kind == gridlayout.SegmentLiteral {
				w = len(seg.Text)
			} else {
				w = defaultFieldWidth(seg.Name)
			}
			if w > max {
				max = w
			}
		}
		return max
	}
	return defaultFieldWidth(a.Name)
}

func defaultFieldWidth(name string) int {
	switch name {
	case "tags", "dependsOn", "depends":
		return 24
	case "createdAt", "updatedAt", "due":
		return 18
	case "title":
		return 30
	}
	return 12
}

// SolveGridLayout resolves the layout grid against the live terminal
// width. heightOf is the callback the solver uses to ask each field for
// its natural height at the resolved column width.
//
// As a single-column safety net (relevant to tiki cards on narrow lanes),
// when the spec has exactly one column it is treated as a stretcher
// regardless of how it was declared. Without this, a tiki card whose only
// column's content has a default width wider than the available lane
// width would have its column shed by the solver — leaving an empty
// frame. Single-column layouts can't shed anything anyway, so promoting
// them to stretcher is always the right call.
//
// As a prose-block safety net, columns occupied by a row-spanned literal
// anchor (RowSpan > 1) are promoted to stretcher when the spec declares
// no stretcher elsewhere. Row-spanned literals are wrapping prose blocks
// (rendered by view/tikidetail.renderLiteralAnchor); without slack flowing
// to their columns, extra terminal width becomes whitespace instead of
// giving the prose more room to wrap. Promotion is gated on the spec
// having no other stretcher so explicit author choices win — if a layout
// already has `<->` somewhere, the author already directed where slack
// should go and we do not override that.
func SolveGridLayout(width int, spec gridlayout.GridSpec, heightOf func(name string, width int) int) gridlayout.Plan {
	if spec.Cols == 1 && len(spec.Stretcher) > 0 && !spec.Stretcher[0] {
		stretched := spec
		stretched.Stretcher = []bool{true}
		spec = stretched
	}
	if !hasStretcher(spec) {
		spec = promoteRowSpannedLiteralColumns(spec)
	}
	return gridlayout.SolveLayout(spec, width, InterColumnGap, DefaultAnchorWidth, heightOf)
}

func hasStretcher(spec gridlayout.GridSpec) bool {
	for _, s := range spec.Stretcher {
		if s {
			return true
		}
	}
	return false
}

func promoteRowSpannedLiteralColumns(spec gridlayout.GridSpec) gridlayout.GridSpec {
	promoted := false
	stretcher := append([]bool(nil), spec.Stretcher...)
	for _, a := range spec.Anchors {
		if a.Kind != gridlayout.AnchorLiteral || a.RowSpan <= 1 {
			continue
		}
		// Only promote literals whose text is prose (multiple words),
		// not short captions like "Tags:" that happen to have RowSpan>1
		// because a `^` row-continuation appears below them. Captions are
		// 1-word labels; promoting them stretches a label column to absorb
		// terminal slack, leaving a wide gap before the value column.
		if !isMultiWord(a.Text) {
			continue
		}
		for cc := a.Col; cc < a.Col+a.ColSpan && cc < len(stretcher); cc++ {
			if !stretcher[cc] {
				stretcher[cc] = true
				promoted = true
			}
		}
	}
	if !promoted {
		return spec
	}
	out := spec
	out.Stretcher = stretcher
	return out
}

func isMultiWord(s string) bool {
	return len(strings.Fields(s)) >= 2
}
