// Package render holds small string-building helpers shared between view
// subpackages, kept here to avoid duplication.
package render

import "github.com/boolean-maybe/tiki/theme"

// RenderTikiIDPaint renders a tiki ID using the theme's tiki.id role with
// the .accent modifier — a subtle gradient on capable terminals, a solid
// color on lesser ones. Returns empty when id is empty. Panics if the
// resolver fails: tiki.id + .accent is a canonical pair; a miss means the
// theme system is structurally broken and silent uncolored rendering would
// mask the bug.
func RenderTikiIDPaint(id string, roles *theme.Theme) string {
	if id == "" {
		return ""
	}
	paint, ok := roles.PaintResolver()("tiki.id", "accent")
	if !ok {
		// canonical role + canonical modifier should always resolve. Failing
		// here means the theme system itself is broken — refuse to silently
		// render uncolored text.
		panic("render: tiki.id.accent resolver miss — theme system invariant violated")
	}
	return paint.PaintString(id)
}
