package gridlayout

import "testing"

// TestKanbanProjectFrReachesFrame is a regression test for smoke-test finding
// P/P1: the bundled kanban "Project" detail layout has a long `:fr` prose cell
// followed by a trailing decorative column that only receives the title's
// colspan passthrough plus empty cells. Before the fix the trailing column
// reserved width + a gap, stranding a dead horizontal gap between the prose and
// the box's right frame (and collapsing the prose to a sliver at narrow
// widths). The prose column must now absorb all residual up to the frame.
//
// The layout string is a verbatim copy of config/workflows/kanban.yaml's
// Project view `layout:` block; keep them in sync.
func TestKanbanProjectFrReachesFrame(t *testing.T) {
	const layout = `<highlight>title             | --                                 | --                    | --        | --                       | --    | --                                                                                                                                                                                                                                                                                    | --
_                            | _                                  | _                     | _         | _                        | _     | _                                                                                                                                                                                                                                                                                     | _
<text.label>status.caption   | (status.label + " " + status.visual):16.. | <text.muted>createdBy.caption  | createdBy | <text.label>tags.caption | tags?:18.. | (<text.muted>"Projects gather related tasks — stories, bugs, and spikes — into a single unit of planning. Press " + <status.warn>"<L>" + <text.muted>" to see every task linked to this project. Move it across Now, Next, and Later as priorities shift; it auto-completes when all its tasks are done."):fr | _
<text.label>priority.caption | priority                           | <text.muted>createdAt.caption  | createdAt | ^                        | ^     | ^                                                                                                                                                                                                                                                                                     | _
<text.label>points.caption   | points                             | <text.muted>updatedAt.caption  | updatedAt | ^                        | ^     | ^                                                                                                                                                                                                                                                                                     | _`

	spec, err := ParseLayout(layout)
	if err != nil {
		t.Fatalf("ParseLayout: %v", err)
	}

	// the prose composite is the only :fr; give it a large min-content so it
	// wants far more than the available width. Other cells measure modestly.
	measure := func(a Anchor) int {
		if a.Kind == AnchorComposite && a.Sizing.Mode == SizeGrow {
			return 240
		}
		if a.Display == DisplayCaption {
			return len([]rune(a.Name)) + 1
		}
		return 6
	}
	heightOf := func(a Anchor, w int) int { return 1 }

	proseCol := spec.Cols - 2 // the :fr column; last column is the trailing `_`
	for _, width := range []int{180, 120} {
		plan := SolveLayout(spec, width, 1, measure, heightOf)

		used, visible := 0, 0
		for c := 0; c < spec.Cols; c++ {
			if plan.Dropped[c] {
				continue
			}
			visible++
			used += plan.ColumnWidths[c]
		}
		if visible > 1 {
			used += (visible - 1) * 1
		}

		if !plan.Dropped[spec.Cols-1] {
			t.Errorf("width=%d: trailing span-only column not dropped: %v", width, plan.ColumnWidths)
		}
		if plan.Dropped[proseCol] || plan.ColumnWidths[proseCol] < 1 {
			t.Errorf("width=%d: prose :fr column wrongly starved/dropped: %v", width, plan.ColumnWidths)
		}
		if used != width {
			t.Errorf("width=%d: dead gap of %d before frame; :fr prose should fill to frame: %v",
				width, width-used, plan.ColumnWidths)
		}
	}
}
