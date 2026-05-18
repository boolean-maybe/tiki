package gridbox

import (
	"github.com/boolean-maybe/tiki/gridlayout"
)

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
// Used by ConfigurableDetailView and TikiEditView.
const DetailBoxOverhead = 4

// DefaultAnchorWidth returns the wanted-width hint for an anchor that did
// not declare a `:N` width in the layout grid. Kept conservative so
// stretcher columns absorb generous slack without crowding fixed columns.
//
// Field anchors get a per-name table lookup; literal anchors get a width
// based on their text length so short captions like "Status:" don't crowd
// out adjacent value columns.
func DefaultAnchorWidth(a gridlayout.Anchor) int {
	if a.Kind == gridlayout.AnchorLiteral {
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
func SolveGridLayout(width int, spec gridlayout.GridSpec, heightOf func(name string, width int) int) gridlayout.Plan {
	if spec.Cols == 1 && len(spec.Stretcher) > 0 && !spec.Stretcher[0] {
		stretched := spec
		stretched.Stretcher = []bool{true}
		spec = stretched
	}
	return gridlayout.SolveLayout(spec, width, InterColumnGap, DefaultAnchorWidth, heightOf)
}
