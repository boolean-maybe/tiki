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

// measureZero is a test helper: every anchor reports content width 1.
func measureZero(a Anchor) int { return 1 }

func TestSolveLayout_SingleAnchorGetsAllWidth(t *testing.T) {
	spec := mustParse(t, [][]string{{"status"}})
	plan := SolveLayout(spec, 40, 2, measureZero, nil)
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
	spec := mustParse(t, [][]string{{"status:10", "sp:fr", "tags:20"}})
	plan := SolveLayout(spec, 60, 2, measureZero, nil)
	// col 0 = 10, col 2 = 20, col 1 = grow with residual.
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
	// equal floors (all :10 fixed have floor 10) -> ascending-floor shedding
	// degenerates to a tie, broken right-to-left, so col 2 still drops first.
	spec := mustParse(t, [][]string{{"a:10", "b:10", "c:10"}})
	// Width 25: room for at most 2 cols (10+10+gap=22 OK; 10+10+10+2*2=34 too wide)
	plan := SolveLayout(spec, 25, 2, measureZero, nil)
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
	plan := SolveLayout(spec, 60, 2, measureZero, nil)
	if plan.ColumnWidths[0] != 30 {
		t.Errorf("col0 width = %d, want 30 (max of 20 and 30)", plan.ColumnWidths[0])
	}
}

func TestSolveLayout_AnchorSpanDistributesWantedWidth(t *testing.T) {
	spec := mustParse(t, [][]string{
		{"title:60", "--", "--"},
	})
	plan := SolveLayout(spec, 200, 2, measureZero, nil)
	sum := plan.ColumnWidths[0] + plan.ColumnWidths[1] + plan.ColumnWidths[2]
	if sum < 60 {
		t.Errorf("spanned width sum = %d, want >= 60", sum)
	}
}

func TestSolveLayout_TwoStretchersSplit(t *testing.T) {
	spec := mustParse(t, [][]string{{"l:fr", "status:10", "r:fr"}})
	plan := SolveLayout(spec, 50, 2, measureZero, nil)
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
	heightOf := func(a Anchor, w int) int {
		if a.Name == "tags" {
			return 3
		}
		return 1
	}
	plan := SolveLayout(spec, 40, 2, measureZero, heightOf)
	totalH := plan.RowHeights[0] + plan.RowHeights[1] + plan.RowHeights[2]
	if totalH < 3 {
		t.Errorf("total height = %d, want >= 3 (rows %v)", totalH, plan.RowHeights)
	}
}

func TestSolveLayout_DroppedAnchorSkippedInPlaced(t *testing.T) {
	spec := mustParse(t, [][]string{{"a:10", "b:10", "c:10"}})
	plan := SolveLayout(spec, 25, 2, measureZero, nil)
	for _, p := range plan.Placed {
		if p.Name == "c" {
			t.Errorf("dropped anchor 'c' should not appear in Placed")
		}
	}
}

func TestSolveLayout_EmptyAnchorsSafe(t *testing.T) {
	// All empty cells, no anchors.
	spec := mustParse(t, [][]string{{"_", "_"}})
	plan := SolveLayout(spec, 40, 2, measureZero, nil)
	if len(plan.Placed) != 0 {
		t.Errorf("placed = %+v, want empty", plan.Placed)
	}
}

func TestSolveLayout_FrSplitsByWeight(t *testing.T) {
	spec := mustParse(t, [][]string{{"a:2fr", "b:1fr", "c:4"}})
	// width 34, gap 2: fixed c=4, gaps=2*2=4, residual=34-4-4=26 split 2:1.
	plan := SolveLayout(spec, 34, 2, measureZero, nil)
	if plan.ColumnWidths[0] <= plan.ColumnWidths[1] {
		t.Errorf("a(%d) should exceed b(%d) at weight 2:1", plan.ColumnWidths[0], plan.ColumnWidths[1])
	}
	if plan.ColumnWidths[2] != 4 {
		t.Errorf("fixed c = %d, want 4", plan.ColumnWidths[2])
	}
}

func TestSolveLayout_AutoMeasuresContent(t *testing.T) {
	spec := mustParse(t, [][]string{{"status"}}) // auto, uncapped
	measure := func(a Anchor) int { return 11 }  // "In Progress"
	plan := SolveLayout(spec, 40, 2, measure, nil)
	if plan.ColumnWidths[0] != 11 {
		t.Errorf("auto width = %d, want 11 (content)", plan.ColumnWidths[0])
	}
}

func TestSolveLayout_AutoMaxClamps(t *testing.T) {
	spec := mustParse(t, [][]string{{"priority:auto..8"}})
	measure := func(a Anchor) int { return 11 } // longer than cap
	plan := SolveLayout(spec, 40, 2, measure, nil)
	if plan.ColumnWidths[0] != 8 {
		t.Errorf("clamped width = %d, want 8", plan.ColumnWidths[0])
	}
}

func TestSolveLayout_ShedByAscendingFloor(t *testing.T) {
	// floors: a=2 (lowest), b=6, c fixed 6. When the row can't fit, a drops first.
	spec := mustParse(t, [][]string{{"a:2..", "b:6..", "c:6"}})
	measure := func(a Anchor) int { return 6 }
	plan := SolveLayout(spec, 16, 2, measure, nil) // tight: forces one drop
	if !plan.Dropped[0] {
		t.Errorf("lowest-floor col a should drop first: %+v", plan.Dropped)
	}
	if plan.Dropped[1] || plan.Dropped[2] {
		t.Errorf("higher-floor cols wrongly dropped: %+v", plan.Dropped)
	}
}

// TestSolveLayout_CoShedCaptionWithValue verifies the hard rule that a field's
// `.caption` and its value shed together. Layout: a caption-beside-value pair
// (status.caption | status) next to a wide pinned column. When the value column
// must drop for width, its caption column drops too — no orphaned caption.
func TestSolveLayout_CoShedCaptionWithValue(t *testing.T) {
	spec := mustParse(t, [][]string{{"status.caption", "status", "keep:20"}})
	measure := func(a Anchor) int {
		if a.Name == "keep" {
			return 20
		}
		return 8
	}
	// Width fits only the pinned "keep" column plus gaps — the status pair must go.
	plan := SolveLayout(spec, 22, 2, measure, nil)
	if !plan.Dropped[0] || !plan.Dropped[1] {
		t.Fatalf("status caption(col0) and value(col1) must both drop: %+v", plan.Dropped)
	}
	if plan.Dropped[2] {
		t.Errorf("pinned keep column wrongly dropped: %+v", plan.Dropped)
	}
}

// TestSolveLayout_CoShedSuppressesOrphanInMixedColumn verifies per-anchor
// suppression: when a caption column survives (it holds a surviving field too)
// but one field's value dropped, that field's caption is individually
// suppressed rather than orphaned.
func TestSolveLayout_CoShedSuppressesOrphanInMixedColumn(t *testing.T) {
	// col0 holds two captions (a.caption, b.caption); col1 holds a's value;
	// col2 holds b's value. Drop only col2 (b's value): b.caption must be
	// suppressed though col0 survives (a.caption keeps it alive).
	spec := mustParse(t, [][]string{
		{"a.caption", "a", "b"},
		{"b.caption", "_", "_"},
	})
	// Only b's VALUE is wide (its caption is short). Size by display so the
	// caption cells don't inflate the shared caption column.
	measure := func(a Anchor) int {
		if a.Name == "b" && a.Display != DisplayCaption {
			return 30
		}
		return 4
	}
	// Width 16: a.caption(4)+a(4)+gap+gap ≈ 12 fits; b's value(30) cannot.
	// col0 (shared captions, floor 4) and col1 (a value, floor 1) survive;
	// col2 (b value) drops.
	plan := SolveLayout(spec, 16, 2, measure, nil)
	if !plan.Dropped[2] {
		t.Fatalf("expected b value column (col2) dropped: widths=%v dropped=%+v", plan.ColumnWidths, plan.Dropped)
	}
	if plan.Dropped[0] || plan.Dropped[1] {
		t.Fatalf("col0/col1 must survive (a is wholly present): %+v", plan.Dropped)
	}
	// b.caption is at (1,0) in the surviving shared column — suppress per-anchor.
	if !plan.SuppressedAnchorAt(spec, "b", 1, 0) {
		t.Errorf("b.caption at (1,0) must be suppressed (its value dropped)")
	}
	// a.caption at (0,0) must NOT be suppressed — a's value (col1) survives.
	if plan.SuppressedAnchorAt(spec, "a", 0, 0) {
		t.Errorf("a.caption wrongly suppressed while a's value survives")
	}
}

// TestSolveLayout_GrowFloorForcesNeighbourShed verifies the grow-floor rule:
// a grow column with an explicit floor (:MIN..fr) counts that floor toward
// required width, so when space is tight a droppable neighbour sheds to give
// the grow column its floor — rather than the grow column shrinking to a
// useless sliver. A plain :fr (no floor) does NOT force that shed; it just
// absorbs the residual.
func TestSolveLayout_GrowFloorForcesNeighbourShed(t *testing.T) {
	// keep(pinned 18) + opt(droppable, floor 1) + deps(grow, floor 16).
	// At width 38: required-with-floor = 18 + 6 + 16 + 2*gap = 44 > 38, so the
	// lowest-floor non-grow column (opt) sheds; afterward keep(18) + deps(16) +
	// 1 gap = 36 <= 38, so keep and deps both survive.
	spec := mustParse(t, [][]string{{"keep:18", "opt:1..", "deps:16..fr"}})
	measure := func(a Anchor) int {
		switch a.Name {
		case "keep":
			return 18
		case "opt":
			return 6
		}
		return 16
	}
	plan := SolveLayout(spec, 38, 2, measure, nil)
	if !plan.Dropped[1] {
		t.Errorf("droppable neighbour should shed to satisfy grow floor: widths=%v dropped=%+v", plan.ColumnWidths, plan.Dropped)
	}
	if plan.Dropped[0] || plan.Dropped[2] {
		t.Errorf("pinned keep and grow deps must survive: %+v", plan.Dropped)
	}
	if plan.ColumnWidths[2] < 16 {
		t.Errorf("grow deps width = %d, want >= its floor 16", plan.ColumnWidths[2])
	}

	// Plain :fr (no floor): the neighbour is NOT forced out; deps just shrinks.
	spec2 := mustParse(t, [][]string{{"keep:18", "opt:1..", "deps:fr"}})
	plan2 := SolveLayout(spec2, 30, 2, func(a Anchor) int {
		switch a.Name {
		case "keep":
			return 18
		case "opt":
			return 6
		}
		return 1
	}, nil)
	if plan2.Dropped[1] {
		t.Errorf("plain :fr must not force a neighbour shed: %+v", plan2.Dropped)
	}
}

// TestSolveLayout_CaptionAnchorHeightIsOne verifies a `.caption` field anchor is
// always height 1 even when the field's value height callback returns more —
// a caption must not inflate its row to the value's wrapped height.
func TestSolveLayout_CaptionAnchorHeightIsOne(t *testing.T) {
	spec := mustParse(t, [][]string{{"tags.caption"}})
	heightOf := func(a Anchor, w int) int { return 5 } // value would be 5 rows
	plan := SolveLayout(spec, 40, 2, measureZero, heightOf)
	if plan.RowHeights[0] != 1 {
		t.Errorf("caption row height = %d, want 1 (caption is always one line)", plan.RowHeights[0])
	}
}

func TestSolveLayout_CompositeColumnFloor(t *testing.T) {
	spec := mustParse(t, [][]string{
		{"<text.label>status.caption", `(status.label + " " + status.visual):16..`},
		{"<text.label>priority.caption", "priority"},
	})
	heightOf := func(a Anchor, w int) int { return 1 }
	// vary the composite's rendered width: Done (7), Ready (8), In Progress + emoji (14).
	for _, statusWidth := range []int{7, 8, 14} {
		measure := func(a Anchor) int {
			if a.Display == DisplayCaption {
				return len([]rune(a.Name)) + 1
			}
			if a.Kind == AnchorComposite {
				return statusWidth
			}
			return len("Medium")
		}
		plan := SolveLayout(spec, 200, 1, measure, heightOf)
		if plan.ColumnWidths[1] < 16 {
			t.Errorf("statusWidth=%d: value column = %d, want >=16 (no jump)", statusWidth, plan.ColumnWidths[1])
		}
	}
}
