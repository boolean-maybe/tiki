package gridlayout

import (
	"strings"
	"testing"
)

// projectDetailSpec parses the bundled Project detail layout for diagnostics.
func projectDetailSpec(t *testing.T) GridSpec {
	t.Helper()
	raw := []string{
		`<highlight>title             | -- | -- | -- | -- | -- | -- | --`,
		`_ | _ | _ | _ | _ | _ | _ | _`,
		`<text.label>status.caption   | (status.label + " " + status.visual):16.. | <text.muted>createdBy.caption  | createdBy | <text.label>tags.caption | tags? | (<text.muted>"Projects gather related tasks — stories, bugs, and spikes — into a single unit of planning. Press " + <status.warn>"<L>" + <text.muted>" to see every task linked to this project. Move it across Now, Next, and Later as priorities shift; it auto-completes when all its tasks are done."):fr | _`,
		`<text.label>priority.caption | priority                           | <text.muted>createdAt.caption  | createdAt | ^                        | ^     | ^ | _`,
		`<text.label>points.caption   | points                             | <text.muted>updatedAt.caption  | updatedAt | ^                        | ^     | ^ | _`,
	}
	rows := make([][]string, len(raw))
	for i, line := range raw {
		parts := strings.Split(line, "|")
		for j := range parts {
			parts[j] = strings.TrimSpace(parts[j])
		}
		rows[i] = parts
	}
	spec, err := ParseGrid(rows)
	if err != nil {
		t.Fatalf("ParseGrid: %v", err)
	}
	return spec
}

// TestProjectDetail_DateColumnNotStarvedAt180 reproduces smoke-test defect #4:
// at the comfortable 180-col width the createdAt/updatedAt value column (col3)
// must be wide enough to render a full "2026-06-11 15:04" datetime (17 cells),
// not clipped to "2026-06-11…". The row-spanned prose composite (col6, :fr) is
// a grow column and must absorb residual width AFTER auto columns get their
// content width — it must not starve the date column.
func TestProjectDetail_DateColumnNotStarvedAt180(t *testing.T) {
	spec := projectDetailSpec(t)

	const datetimeWidth = 17 // "2026-06-11 15:04"
	measure := func(a Anchor) int {
		if a.Display == DisplayCaption {
			return len([]rune(a.Name))
		}
		switch a.Name {
		case "createdAt", "updatedAt":
			return datetimeWidth
		case "createdBy":
			return 12
		case "status", "priority", "points":
			return 10
		case "tags":
			return 16
		case "title":
			return 24
		}
		return len([]rune(a.Text))
	}

	plan := SolveLayout(spec, 180, 1, measure, nil)

	// col3 holds createdAt/updatedAt values.
	const dateValueCol = 3
	if plan.Dropped[dateValueCol] {
		t.Fatalf("date value column (col%d) was dropped at 180 cols", dateValueCol)
	}
	if got := plan.ColumnWidths[dateValueCol]; got < datetimeWidth {
		t.Errorf("date value column width = %d, want >= %d (full datetime, no clip); widths=%v",
			got, datetimeWidth, plan.ColumnWidths)
	}
}
