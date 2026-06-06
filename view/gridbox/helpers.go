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
// content-measure callback wiring and the inter-column gap used as a visual
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

// MeasureAnchorText is the content-width measurer for non-field anchors
// (literals and composites). Field anchors are measured by a caller-supplied
// callback that reads the rendered field value; everything else is measured
// from its declared text.
//
// Single-row literals measure as their text length (+1 padding). Row-spanned
// literals and composites are wrapping prose blocks, so their minimum useful
// width is the longest word — not the full text — otherwise a long block would
// demand the full text length as a column width and get dropped on narrow
// terminals. Single-row composites take the widest single segment (sum
// overestimates "minimum useful" width for expression-like cells such as
// `visual + id`).
func MeasureAnchorText(a gridlayout.Anchor) int {
	switch a.Kind {
	case gridlayout.AnchorLiteral:
		if a.RowSpan > 1 {
			return longestWordWidth(a.Text)
		}
		return len(a.Text) + 1
	case gridlayout.AnchorComposite:
		return measureComposite(a)
	}
	return 1
}

func measureComposite(a gridlayout.Anchor) int {
	if a.RowSpan > 1 {
		max := 1
		for _, seg := range a.Segments {
			if seg.Kind != gridlayout.SegmentLiteral {
				continue
			}
			if w := longestWordWidth(seg.Text); w > max {
				max = w
			}
		}
		return max
	}
	max := 1
	for _, seg := range a.Segments {
		w := 1
		if seg.Kind == gridlayout.SegmentLiteral {
			w = len(seg.Text)
		}
		if w > max {
			max = w
		}
	}
	return max
}

// SolveGridLayout resolves the layout grid against the live terminal width.
// measure reports the content width of an anchor (field anchors read the
// rendered value; non-field anchors should defer to MeasureAnchorText).
// heightOf is the callback the solver uses to ask each field for its natural
// height at the resolved column width.
//
// As a single-column safety net (relevant to tiki cards on narrow lanes),
// when the spec has exactly one column it is promoted to grow regardless of
// how it was declared. Without this, a tiki card whose only column's content
// is wider than the available lane width would have its column shed by the
// solver — leaving an empty frame. Single-column layouts can't shed anything
// anyway, so promoting them to grow is always the right call.
//
// As a prose-block safety net, columns occupied by a row-spanned literal
// anchor (RowSpan > 1) are promoted to grow when the spec declares no grow
// column elsewhere. Row-spanned literals are wrapping prose blocks; without
// slack flowing to their columns, extra terminal width becomes whitespace
// instead of giving the prose more room to wrap. Promotion is gated on the
// spec having no other grow column so explicit author `:fr` choices win.
func SolveGridLayout(width int, spec gridlayout.GridSpec, measure func(a gridlayout.Anchor) int, heightOf func(a gridlayout.Anchor, width int) int) gridlayout.Plan {
	return gridlayout.SolveLayout(promoteForGrowth(spec), width, InterColumnGap, measure, heightOf)
}

// promoteForGrowth applies the single-column and prose-block grow safety nets,
// returning a (possibly copied) spec whose anchors carry the promoted SizeGrow
// mode. The original spec is never mutated.
func promoteForGrowth(spec gridlayout.GridSpec) gridlayout.GridSpec {
	if spec.Cols == 1 {
		return promoteSingleColumn(spec)
	}
	if !hasGrow(spec) {
		return promoteRowSpannedLiteralColumns(spec)
	}
	return spec
}

func hasGrow(spec gridlayout.GridSpec) bool {
	for _, g := range gridlayout.GrowColumns(spec) {
		if g {
			return true
		}
	}
	return false
}

// promoteSingleColumn makes the sole column grow by setting its single-column
// anchors to SizeGrow (only if no anchor already grows).
func promoteSingleColumn(spec gridlayout.GridSpec) gridlayout.GridSpec {
	if hasGrow(spec) {
		return spec
	}
	out := spec
	out.Anchors = append([]gridlayout.Anchor(nil), spec.Anchors...)
	for i := range out.Anchors {
		if out.Anchors[i].ColSpan == 1 {
			out.Anchors[i].Sizing = gridlayout.Sizing{Mode: gridlayout.SizeGrow, Weight: 1}
		}
	}
	return out
}

// promoteRowSpannedLiteralColumns sets row-spanned prose anchors to SizeGrow so
// column slack flows into them. Returns the original spec if nothing promoted.
func promoteRowSpannedLiteralColumns(spec gridlayout.GridSpec) gridlayout.GridSpec {
	out := spec
	out.Anchors = append([]gridlayout.Anchor(nil), spec.Anchors...)
	promoted := false
	for i := range out.Anchors {
		a := out.Anchors[i]
		// Only row-spanned single-column prose anchors are promoted: a grow
		// column comes from a single-column SizeGrow anchor (see gridlayout
		// GrowColumns), so promoting a multi-column anchor would have no effect.
		if a.RowSpan <= 1 || a.ColSpan != 1 || !isProseAnchor(a) {
			continue
		}
		out.Anchors[i].Sizing = gridlayout.Sizing{Mode: gridlayout.SizeGrow, Weight: 1}
		promoted = true
	}
	if !promoted {
		return spec
	}
	return out
}

func isMultiWord(s string) bool {
	return len(strings.Fields(s)) >= 2
}

// isProseAnchor reports whether a row-spanned anchor renders as a wrapping
// prose block — i.e. carries multi-word literal text. True for row-spanned
// literal anchors with multi-word text and for row-spanned composite anchors
// whose concatenated literal segments contain >=2 words. False for field
// anchors (no literal text to wrap) and short single-word captions.
func isProseAnchor(a gridlayout.Anchor) bool {
	switch a.Kind {
	case gridlayout.AnchorLiteral:
		return isMultiWord(a.Text)
	case gridlayout.AnchorComposite:
		var combined strings.Builder
		for _, seg := range a.Segments {
			if seg.Kind == gridlayout.SegmentLiteral {
				combined.WriteString(seg.Text)
				combined.WriteByte(' ')
			}
		}
		return isMultiWord(combined.String())
	}
	return false
}
