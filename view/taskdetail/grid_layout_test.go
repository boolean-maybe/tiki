package taskdetail

import (
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
)

func mustParse(t *testing.T, raw [][]string) gridlayout.GridSpec {
	t.Helper()
	spec, err := gridlayout.ParseGrid(raw)
	if err != nil {
		t.Fatalf("ParseGrid: %v", err)
	}
	return spec
}

func TestSolveGridLayout_DefaultFieldWidthHint(t *testing.T) {
	spec := mustParse(t, [][]string{{"tags"}})
	plan := SolveGridLayout(60, spec, func(string, int) int { return 1 })
	// tags default width is 24; column should fill 60.
	if plan.ColumnWidths[0] < 24 {
		t.Errorf("col width = %d, want >= 24", plan.ColumnWidths[0])
	}
}

func TestSolveGridLayout_CanonicalGridFits(t *testing.T) {
	spec := mustParse(t, [][]string{
		{"title", "--", "--", "--", "--"},
		{"status", "assignee", "<->", "tags:30", "depends:25"},
		{"type", "createdBy", "<->", "|", "|"},
		{"priority", "createdAt", "<->", "_", "_"},
		{"points", "updatedAt", "<->", "_", "_"},
	})
	plan := SolveGridLayout(120, spec, func(name string, w int) int {
		if name == "tags" || name == "depends" {
			return 2
		}
		return 1
	})
	if plan.ColumnWidths[3] != 30 {
		t.Errorf("tags col = %d, want 30", plan.ColumnWidths[3])
	}
	if plan.ColumnWidths[4] != 25 {
		t.Errorf("depends col = %d, want 25", plan.ColumnWidths[4])
	}
	if !plan.Dropped[3] && !plan.Dropped[4] && plan.ColumnWidths[2] < 1 {
		t.Errorf("stretcher col should be >= 1, got %d", plan.ColumnWidths[2])
	}
	// Title spans 5 columns; one of the placed anchors should be title.
	hasTitle := false
	for _, p := range plan.Placed {
		if p.Name == "title" {
			hasTitle = true
			if p.ColSpan != 5 {
				t.Errorf("title ColSpan = %d, want 5", p.ColSpan)
			}
		}
	}
	if !hasTitle {
		t.Errorf("title not placed in plan")
	}
}

func TestSolveGridLayout_NarrowShedsRight(t *testing.T) {
	spec := mustParse(t, [][]string{
		{"status", "assignee", "<->", "tags:30", "depends:25"},
	})
	plan := SolveGridLayout(40, spec, func(string, int) int { return 1 })
	// 40 chars can't fit status(12)+assignee(12)+stretcher(>=1)+tags(30)+depends(25)+4gaps(8) = 88.
	// Shed rightmost (depends), then tags. Expect both dropped.
	if !plan.Dropped[4] {
		t.Errorf("depends should be dropped at width 40, got %+v", plan.Dropped)
	}
}
