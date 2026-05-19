package gridlayout

import (
	"testing"
)

func mustParse(t *testing.T, raw [][]string) GridSpec {
	t.Helper()
	spec, err := ParseGrid(raw)
	if err != nil {
		t.Fatalf("ParseGrid: %v", err)
	}
	return spec
}

func TestSolveLayout_SingleAnchorGetsAllWidth(t *testing.T) {
	spec := mustParse(t, [][]string{{"status"}})
	plan := SolveLayout(spec, 40, 2, nil, nil)
	if plan.ColumnWidths[0] < 1 {
		t.Errorf("width = %d, want >=1", plan.ColumnWidths[0])
	}
	if plan.RowHeights[0] != 1 {
		t.Errorf("height = %d, want 1", plan.RowHeights[0])
	}
	if len(plan.Placed) != 1 || plan.Placed[0].Name != "status" {
		t.Fatalf("placed = %+v, want one status anchor", plan.Placed)
	}
}

func TestSolveLayout_StretcherAbsorbsResidual(t *testing.T) {
	spec := mustParse(t, [][]string{{"status:10", "<->", "tags:20"}})
	plan := SolveLayout(spec, 60, 2, nil, nil)
	// col 0 = 10, col 2 = 20, col 1 = stretcher with residual.
	if plan.ColumnWidths[0] != 10 {
		t.Errorf("col0 = %d, want 10", plan.ColumnWidths[0])
	}
	if plan.ColumnWidths[2] != 20 {
		t.Errorf("col2 = %d, want 20", plan.ColumnWidths[2])
	}
	// residual = 60 - 10 - 20 - 2*2 (gaps) = 26
	if plan.ColumnWidths[1] != 26 {
		t.Errorf("col1 (stretcher) = %d, want 26", plan.ColumnWidths[1])
	}
}

func TestSolveLayout_ShedRightToLeft(t *testing.T) {
	spec := mustParse(t, [][]string{{"a:10", "b:10", "c:10"}})
	// Width 25: room for at most 2 cols (10+10+gap=22 OK; 10+10+10+2*2=34 too wide)
	plan := SolveLayout(spec, 25, 2, nil, nil)
	if !plan.Dropped[2] {
		t.Errorf("col 2 should be dropped: %+v", plan.Dropped)
	}
	if plan.Dropped[0] || plan.Dropped[1] {
		t.Errorf("col 0 or 1 wrongly dropped: %+v", plan.Dropped)
	}
	gotDropped := map[string]bool{}
	for _, n := range plan.DroppedAnchors {
		gotDropped[n] = true
	}
	if !gotDropped["c"] || gotDropped["a"] || gotDropped["b"] {
		t.Errorf("dropped anchors = %v, want only [c]", plan.DroppedAnchors)
	}
}

func TestSolveLayout_MaxWidthAcrossColumn(t *testing.T) {
	spec := mustParse(t, [][]string{
		{"tags:20"},
		{"depends:30"},
	})
	plan := SolveLayout(spec, 60, 2, nil, nil)
	if plan.ColumnWidths[0] != 30 {
		t.Errorf("col0 width = %d, want 30 (max of 20 and 30)", plan.ColumnWidths[0])
	}
}

func TestSolveLayout_AnchorSpanDistributesWantedWidth(t *testing.T) {
	spec := mustParse(t, [][]string{
		{"title:60", "--", "--"},
	})
	plan := SolveLayout(spec, 200, 2, nil, nil)
	sum := plan.ColumnWidths[0] + plan.ColumnWidths[1] + plan.ColumnWidths[2]
	if sum < 60 {
		t.Errorf("spanned width sum = %d, want >= 60", sum)
	}
}

func TestSolveLayout_TwoStretchersSplit(t *testing.T) {
	spec := mustParse(t, [][]string{{"<->", "status:10", "<->"}})
	plan := SolveLayout(spec, 50, 2, nil, nil)
	// gaps = 2*2 = 4; fixed = 10; residual = 50 - 10 - 4 = 36; split as 18+18.
	if plan.ColumnWidths[0] != 18 || plan.ColumnWidths[2] != 18 {
		t.Errorf("stretchers = %d/%d, want 18/18", plan.ColumnWidths[0], plan.ColumnWidths[2])
	}
}

func TestSolveLayout_MultiRowAnchorGrowsLastRow(t *testing.T) {
	spec := mustParse(t, [][]string{
		{"tags:30"},
		{"^"},
		{"^"},
	})
	heightOf := func(name string, w int) int {
		if name == "tags" {
			return 3
		}
		return 1
	}
	plan := SolveLayout(spec, 40, 2, nil, heightOf)
	totalH := plan.RowHeights[0] + plan.RowHeights[1] + plan.RowHeights[2]
	if totalH < 3 {
		t.Errorf("total height = %d, want >= 3 (rows %v)", totalH, plan.RowHeights)
	}
}

func TestSolveLayout_DroppedAnchorSkippedInPlaced(t *testing.T) {
	spec := mustParse(t, [][]string{{"a:10", "b:10", "c:10"}})
	plan := SolveLayout(spec, 25, 2, nil, nil)
	for _, p := range plan.Placed {
		if p.Name == "c" {
			t.Errorf("dropped anchor 'c' should not appear in Placed")
		}
	}
}

func TestSolveLayout_EmptyAnchorsSafe(t *testing.T) {
	// All empty / stretcher cells, no anchors.
	spec := mustParse(t, [][]string{{"<->", "_"}})
	plan := SolveLayout(spec, 40, 2, nil, nil)
	if len(plan.Placed) != 0 {
		t.Errorf("placed = %+v, want empty", plan.Placed)
	}
}
