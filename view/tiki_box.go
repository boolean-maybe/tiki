package view

import (
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/view/gridbox"
	"github.com/boolean-maybe/tiki/view/tikidetail"
)

// TikiBox provides a reusable tiki card widget used in board and list views.

// applyFrameStyle applies selected/unselected styling to a frame.
func applyFrameStyle(frame *tview.Frame, selected bool, roles *theme.Theme) {
	if selected {
		frame.SetBorderColor(roles.BorderFocus().TCell())
	} else {
		frame.SetBorderColor(roles.BorderIdle().TCell())
		if !roles.SurfaceTransparent().IsDefault() {
			frame.SetBackgroundColor(roles.SurfaceTransparent().TCell())
		}
	}
}

// tikiBoxItemHeight returns the vertical cell count a tiki-box list item
// occupies for a given layout spec. Borders are added on top of the
// spec's row count via gridbox.TikiBoxOverhead.
func tikiBoxItemHeight(spec gridlayout.GridSpec) int {
	return spec.Rows + gridbox.TikiBoxOverhead
}

// CreateTikiBox renders a tiki card from a layout spec. Box height is
// derived from spec.Rows + gridbox.TikiBoxOverhead at the caller. Each
// layout anchor produces one tview primitive via tikidetail's view-mode
// renderer; gridbox.Container lays them out in the live terminal width.
//
// Tiki cards are fixed-height: every anchor reports height 1 to the
// grid solver regardless of its natural multi-line size. This is the
// design contract (a board card has a fixed visual footprint so cards
// pack cleanly into lanes). Multi-row fields like `tags` or
// `dependsOn` therefore render only their first row inside a card —
// they are intentionally clipped by tview's natural cell-width truncation
// in the same way long titles are. For multi-row rendering, declare the
// field on a `kind: detail` view instead.
func CreateTikiBox(tk *tikipkg.Tiki, spec gridlayout.GridSpec, selected bool, roles *theme.Theme) *tview.Frame {
	primitives := buildTikiBoxPrimitives(spec, tk, roles)
	heightOf := func(string, int) int { return 1 }
	container := gridbox.NewContainer(spec, primitives, heightOf)
	frame := tview.NewFrame(container).SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true)
	applyFrameStyle(frame, selected, roles)
	return frame
}

// buildTikiBoxPrimitives walks the layout anchors and produces one
// tview primitive per anchor using the view-mode renderer.
func buildTikiBoxPrimitives(spec gridlayout.GridSpec, tk *tikipkg.Tiki, roles *theme.Theme) []tview.Primitive {
	ctx := tikidetail.FieldRenderContext{
		Mode:  tikidetail.RenderModeView,
		Roles: roles,
	}
	out := make([]tview.Primitive, len(spec.Anchors))
	for i, a := range spec.Anchors {
		out[i] = tikidetail.RenderViewModeAnchor(a, tk, ctx)
	}
	return out
}
