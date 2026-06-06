package gridbox

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

// measureOne reports content width 1 for every anchor — the neutral measurer
// used by tests that don't exercise content-driven sizing.
func measureOne(a gridlayout.Anchor) int { return 1 }

func TestSolveGridLayout_SingleColumnAbsorbsWidth(t *testing.T) {
	// A bare single-column field is promoted to grow, so it fills the lane.
	spec := mustParse(t, [][]string{{"tags"}})
	plan := SolveGridLayout(60, spec, measureOne, func(gridlayout.Anchor, int) int { return 1 })
	if plan.ColumnWidths[0] < 50 {
		t.Errorf("col width = %d, want it to grow toward 60", plan.ColumnWidths[0])
	}
}

func TestSolveGridLayout_CanonicalGridFits(t *testing.T) {
	spec := mustParse(t, [][]string{
		{"title", "--", "--", "--", "--"},
		{"status", "assignee", "sp:fr", "tags:30", "depends:25"},
		{"type", "createdBy", "_", "|", "|"},
		{"priority", "createdAt", "_", "_", "_"},
		{"points", "updatedAt", "_", "_", "_"},
	})
	plan := SolveGridLayout(120, spec, measureOne, func(a gridlayout.Anchor, w int) int {
		if a.Name == "tags" || a.Name == "depends" {
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
		t.Errorf("grow col should be >= 1, got %d", plan.ColumnWidths[2])
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

// TestMeasureAnchorText_RowSpannedLiteralUsesLongestWord pins the width hint
// for row-spanned literals: instead of len(text)+1 (which would force the
// column to be as wide as the entire prose string and either get the column
// dropped on narrow terminals or rendered single-line on wide ones), the hint
// must be the length of the longest single word — i.e. the true minimum width
// at which word-wrapping can still produce content.
func TestMeasureAnchorText_RowSpannedLiteralUsesLongestWord(t *testing.T) {
	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorLiteral,
		Text:    "Lorem ipsum dolor sit amet consectetur adipiscing",
		RowSpan: 3,
		ColSpan: 1,
	}
	got := MeasureAnchorText(a)
	// "consectetur" = 11, "adipiscing" = 10; longest is 11.
	if got != 11 {
		t.Errorf("row-spanned literal width hint = %d, want 11 (longest word)", got)
	}
}

// TestMeasureAnchorText_SingleRowLiteralUsesFullText pins the existing behavior
// for single-row literals so the row-spanned override does not regress short
// captions like "Status:".
func TestMeasureAnchorText_SingleRowLiteralUsesFullText(t *testing.T) {
	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorLiteral,
		Text:    "Status:",
		RowSpan: 1,
		ColSpan: 1,
	}
	got := MeasureAnchorText(a)
	if got != len("Status:")+1 {
		t.Errorf("single-row literal width hint = %d, want %d", got, len("Status:")+1)
	}
}

// TestSolveGridLayout_RowSpannedLiteralPromotedToGrow pins that when a layout
// contains a row-spanned prose literal and no other grow column, the literal's
// column is promoted to grow. Without this, extra terminal width becomes
// whitespace instead of giving the literal more room to wrap.
func TestSolveGridLayout_RowSpannedLiteralPromotedToGrow(t *testing.T) {
	// 3 cols. Col 0,1 are short fields; col 2 is a row-spanned prose literal.
	// No :fr in the spec; the literal must drive the grow promotion.
	raw := [][]string{
		{`"Status:"`, "status", `"Some longer literal text"`},
		{`"Type:"`, "type", "^"},
	}
	spec, err := gridlayout.ParseGrid(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Sanity: confirm the parsed spec has no grow column yet.
	for i, g := range gridlayout.GrowColumns(spec) {
		if g {
			t.Fatalf("GrowColumns[%d] = true; test premise (no grow) violated", i)
		}
	}
	// Solve at a wide terminal. The literal's column must now grow well
	// beyond longestWordWidth("literal") = 7.
	plan := SolveGridLayout(120, spec, MeasureAnchorText, func(gridlayout.Anchor, int) int { return 1 })
	if plan.ColumnWidths[2] < 30 {
		t.Errorf("literal col width at term=120 = %d, want >= 30 (promoted to grow)",
			plan.ColumnWidths[2])
	}
}

// TestSolveGridLayout_SingleWordCaptionWithRowSpanNotPromoted pins that a
// literal caption like "Tags:" whose RowSpan>1 is NOT promoted to grow. Only
// multi-word prose literals should drive the promotion. Without this guard, a
// row-spanned 1-word caption absorbs terminal slack and leaves a wide gap.
func TestSolveGridLayout_SingleWordCaptionWithRowSpanNotPromoted(t *testing.T) {
	// 4 cols. Col 2 is "Tags:" caption with `^` below (RowSpan=3).
	// Col 3 is the tags value, also row-spanned.
	raw := [][]string{
		{`"Status:"`, "status", `"Tags:"`, "tags"},
		{`"Type:"`, "type", "^", "^"},
		{`"Author:"`, "createdBy", "^", "^"},
	}
	spec, err := gridlayout.ParseGrid(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	plan := SolveGridLayout(120, spec, MeasureAnchorText, func(gridlayout.Anchor, int) int { return 1 })
	// Col 2 is a single-word caption ("Tags:"); it must not be a grow
	// recipient. Width should remain at the caption's text-width hint.
	if plan.ColumnWidths[2] > len("Tags:")+5 {
		t.Errorf("Tags: caption col grew to %d at term=120; expected ~%d (no grow promotion for single-word captions)",
			plan.ColumnWidths[2], len("Tags:")+1)
	}
}

// TestSolveGridLayout_RowSpannedLiteralAtNarrowKeepsBaseWidth pins that the
// grow promotion does NOT raise the minimum width — a narrow terminal must
// still get the longest-word minimum and not drop the column entirely.
func TestSolveGridLayout_RowSpannedLiteralAtNarrowKeepsBaseWidth(t *testing.T) {
	raw := [][]string{
		{`"X:"`, "status", `"foo bar baz qux"`},
		{`"Y:"`, "type", "^"},
	}
	spec, err := gridlayout.ParseGrid(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	plan := SolveGridLayout(40, spec, MeasureAnchorText, func(gridlayout.Anchor, int) int { return 1 })
	if plan.Dropped[2] {
		t.Errorf("literal col dropped at term=40; promotion should not raise the min width")
	}
}

func TestSolveGridLayout_NarrowSheds(t *testing.T) {
	spec := mustParse(t, [][]string{
		{"status:12", "assignee:12", "sp:fr", "tags:30", "depends:25"},
	})
	plan := SolveGridLayout(40, spec, measureOne, func(gridlayout.Anchor, int) int { return 1 })
	// 40 chars can't fit status+assignee+grow+tags(30)+depends(25)+gaps.
	// Equal-floor fixed cols shed right-to-left, so depends drops.
	if !plan.Dropped[4] {
		t.Errorf("depends should be dropped at width 40, got %+v", plan.Dropped)
	}
}

func TestSolveGridLayout_SingleColumnGrows(t *testing.T) {
	spec, err := gridlayout.ParseGrid([][]string{{"title"}})
	if err != nil {
		t.Fatal(err)
	}
	measure := func(a gridlayout.Anchor) int { return 5 }
	plan := SolveGridLayout(80, spec, measure, func(gridlayout.Anchor, int) int { return 1 })
	// single column should absorb full width, not sit at content 5.
	if plan.ColumnWidths[0] < 70 {
		t.Errorf("single column width = %d, want it to grow toward 80", plan.ColumnWidths[0])
	}
}
