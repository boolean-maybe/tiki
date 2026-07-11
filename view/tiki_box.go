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

// applyFrameStyle applies selected/unselected styling to a bordered frame.
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

// applyBorderlessStyle styles a borderless single-row tiki card. Selection
// is conveyed by painting SurfaceSelection as the row background; unselected
// rows fall through to the canvas (no background fill), matching the flat
// row-list look that single-row layouts opt into. Operates on the bare
// gridbox.Container (a tview.Flex) — no Frame is involved because Frame
// requires inner height >= 2 to draw and clips a single-row layout.
//
// Why SetSelectionBackground rather than SetBackgroundColor: each child
// primitive inside the container (TextView, Flex, spacer Box) clears its
// own area to its own background color in Box.DrawForSubclass, which
// would mask a row-level bg set only on the outer container. The
// container's SetSelectionBackground propagates the color to every cached
// anchor primitive and to spacer/flex primitives created during rebuild,
// so the selection band is continuous across the whole row.
func applyBorderlessStyle(container *gridbox.Container, selected bool, roles *theme.Theme) {
	if selected && !roles.SurfaceSelection().IsDefault() {
		container.SetSelectionBackground(roles.SurfaceSelection().TCell())
	}
}

// tikiBoxItemHeight returns the vertical cell count a tiki-box list item
// occupies for a given layout spec. Multi-row layouts add the framed-card
// overhead (top + bottom border via gridbox.TikiBoxOverhead). Single-row
// layouts render borderless and occupy exactly one cell — selection is
// conveyed by background color rather than a frame.
func tikiBoxItemHeight(spec gridlayout.GridSpec) int {
	if spec.Rows == 1 {
		return 1
	}
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
// pack cleanly into lanes). Multi-row list fields therefore render only their
// first row inside a card —
// they are intentionally clipped by tview's natural cell-width truncation
// in the same way long titles are. For multi-row rendering, declare the
// field on a `kind: detail` view instead.
func CreateTikiBox(tk *tikipkg.Tiki, spec gridlayout.GridSpec, selected bool, roles *theme.Theme) tview.Primitive {
	primitives := buildTikiBoxPrimitives(spec, tk, roles)
	heightOf := func(gridlayout.Anchor, int) int { return 1 }
	measure := tikiBoxMeasure(tk, roles)
	container := gridbox.NewContainer(spec, primitives, measure, heightOf)
	if spec.Rows == 1 {
		applyBorderlessStyle(container, selected, roles)
		return container
	}
	// right inset of 3 cells keeps truncated content (the `…`) and any flush
	// text from butting against the right border — a clear breathing gap,
	// matching the established card look. Left stays 0 so content is flush with
	// the left border as before. SetBorders args: top, bottom, header, footer,
	// left, right.
	frame := tview.NewFrame(container).SetBorders(0, 0, 0, 0, 0, 3)
	frame.SetBorder(true)
	applyFrameStyle(frame, selected, roles)
	return frame
}

// tikiBoxMeasure builds the content-measure callback for a card's grid solve,
// reusing the shared tikidetail.MeasureAnchor so cards and the detail box size
// columns by the same rule.
func tikiBoxMeasure(tk *tikipkg.Tiki, roles *theme.Theme) func(a gridlayout.Anchor) int {
	ctx := tikidetail.FieldRenderContext{Mode: tikidetail.RenderModeView, Roles: roles}
	return func(a gridlayout.Anchor) int {
		return tikidetail.MeasureAnchor(a, tk, ctx)
	}
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
