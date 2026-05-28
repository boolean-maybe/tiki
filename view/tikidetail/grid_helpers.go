package tikidetail

import (
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/plugin"
)

// singleColumnSpec synthesizes a 1-column layout grid from a flat
// field name list — one anchor per row, in declaration order. Used by
// configurable_detail_*_test.go to build minimal specs; production
// callers always receive a parsed spec from the workflow loader.
func singleColumnSpec(names []string) gridlayout.GridSpec {
	if len(names) == 0 {
		return gridlayout.GridSpec{}
	}
	anchors := make([]gridlayout.Anchor, len(names))
	cells := make([][]gridlayout.Cell, len(names))
	for i, n := range names {
		anchors[i] = gridlayout.Anchor{
			Name: n, Row: i, Col: 0, RowSpan: 1, ColSpan: 1,
		}
		cells[i] = []gridlayout.Cell{gridlayout.FieldCell{Name: n}}
	}
	return gridlayout.GridSpec{
		Rows:      len(names),
		Cols:      1,
		Anchors:   anchors,
		Stretcher: []bool{false},
		Cells:     cells,
	}
}

// detailPluginFromFields builds a minimal *plugin.DetailPlugin suitable for
// constructor-driven tests. Production code receives a fully-populated
// DetailPlugin from the workflow parser; tests only need Name + Layout to
// drive the configurable detail view. The plugin name "Detail" matches the
// bundled kanban workflow's detail view.
func detailPluginFromFields(fields []string) *plugin.DetailPlugin {
	return &plugin.DetailPlugin{
		BasePlugin: plugin.BasePlugin{Name: "Detail", Kind: plugin.KindDetail},
		Layout:     singleColumnSpec(fields),
	}
}
