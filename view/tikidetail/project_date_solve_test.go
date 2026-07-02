package tikidetail

import (
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestProjectDetail_SolverGivesDateColumnFullWidth solves the real bundled
// Project layout with the real MeasureAnchor at a wide inner width and dumps
// the column widths. It isolates whether the date clip is a solver/measure
// problem (col3 < 16) or a draw-time problem (col3 >= 16 but rendered text
// clips anyway).
func TestProjectDetail_SolverGivesDateColumnFullWidth(t *testing.T) {
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
	spec, err := gridlayout.ParseGrid(rows)
	if err != nil {
		t.Fatalf("ParseGrid: %v", err)
	}

	tk := tikipkg.New()
	tk.Set("title", "Ground station UI rewrite with real-time collaboration")
	tk.Set("createdBy", "booleanmaybe")
	tk.SetCreatedAt(time.Date(2026, 6, 11, 20, 30, 0, 0, time.UTC))
	tk.SetUpdatedAt(time.Date(2026, 6, 11, 20, 30, 0, 0, time.UTC))

	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}
	measure := func(a gridlayout.Anchor) int { return MeasureAnchor(a, tk, ctx) }
	heightOf := func(a gridlayout.Anchor, w int) int { return FieldHeight(a.Name, tk, w) }

	plan := gridlayout.SolveLayout(spec, 190, 1, measure, heightOf)
	t.Logf("column widths @190: %v dropped=%v", plan.ColumnWidths, plan.Dropped)

	// the column must hold the 16-cell datetime AND the truncating view's 1-cell
	// breathing reservation (it draws to width-1), so >= 17.
	const dateCol = 3
	const datetimeFootprint = 16 + scalarBreathingCell // "2026-06-11 20:30" + breathing
	if plan.ColumnWidths[dateCol] < datetimeFootprint {
		t.Errorf("date column (col%d) width = %d, want >= %d (datetime renders un-clipped); widths=%v",
			dateCol, plan.ColumnWidths[dateCol], datetimeFootprint, plan.ColumnWidths)
	}
}
