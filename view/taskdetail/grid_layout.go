package taskdetail

import (
	"github.com/boolean-maybe/tiki/gridlayout"
)

// grid_layout.go is a thin adapter from the configurable detail view onto
// the pure gridlayout package. Width/height solving lives in
// gridlayout.SolveLayout; this file owns the per-field default-width hint
// and the height callback that resolves a field's natural height through
// FieldHeight.

// interColumnGap is the cell count between adjacent columns in the
// metadata box. Kept here (not in gridlayout) because it is a UI-layer
// constant tied to the visual style of the detail view.
const interColumnGap = 2

// defaultFieldWidth returns the wanted-width hint for a field that did
// not declare a `:N` width in the metadata grid. Kept conservative so
// stretcher columns absorb generous slack without crowding fixed columns.
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

// SolveGridLayout resolves the metadata grid against the live terminal
// width. heightOf is the callback the solver uses to ask each field for
// its natural height at the resolved column width.
func SolveGridLayout(width int, spec gridlayout.GridSpec, heightOf func(name string, width int) int) gridlayout.Plan {
	return gridlayout.SolveLayout(spec, width, interColumnGap, defaultFieldWidth, heightOf)
}
