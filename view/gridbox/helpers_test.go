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

// TestDefaultAnchorWidth_RowSpannedLiteralUsesLongestWord pins the width
// hint for row-spanned literals: instead of len(text)+1 (which would force
// the column to be as wide as the entire prose string and either get the
// column dropped on narrow terminals or rendered single-line on wide ones),
// the hint must be the length of the longest single word — i.e. the true
// minimum width at which word-wrapping can still produce content.
func TestDefaultAnchorWidth_RowSpannedLiteralUsesLongestWord(t *testing.T) {
	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorLiteral,
		Text:    "Lorem ipsum dolor sit amet consectetur adipiscing",
		RowSpan: 3,
		ColSpan: 1,
	}
	got := DefaultAnchorWidth(a)
	// "consectetur" = 11, "adipiscing" = 10; longest is 11.
	if got != 11 {
		t.Errorf("row-spanned literal width hint = %d, want 11 (longest word)", got)
	}
}

// TestDefaultAnchorWidth_SingleRowLiteralUsesFullText pins the existing
// behavior for single-row literals so the row-spanned override does not
// regress short captions like "Status:".
func TestDefaultAnchorWidth_SingleRowLiteralUsesFullText(t *testing.T) {
	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorLiteral,
		Text:    "Status:",
		RowSpan: 1,
		ColSpan: 1,
	}
	got := DefaultAnchorWidth(a)
	if got != len("Status:")+1 {
		t.Errorf("single-row literal width hint = %d, want %d", got, len("Status:")+1)
	}
}

// TestSolveGridLayout_RowSpannedLiteralPromotedToStretcher pins that when
// a layout contains a row-spanned literal anchor and has no other stretcher,
// the columns the literal occupies are promoted to stretcher. Without this,
// extra terminal width becomes whitespace instead of giving the literal
// more room to wrap. With promotion, the literal's columns grow with the
// terminal so more of the prose is visible.
func TestSolveGridLayout_RowSpannedLiteralPromotedToStretcher(t *testing.T) {
	// 3 cols. Col 0,1 are short fields; col 2 is a row-spanned literal.
	// No <-> in the spec; literal must drive the stretcher promotion.
	raw := [][]string{
		{`"Status:"`, "status", `"Some longer literal text"`},
		{`"Type:"`, "type", "^"},
	}
	spec, err := gridlayout.ParseGrid(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Sanity: confirm parser produced no stretcher.
	for i, s := range spec.Stretcher {
		if s {
			t.Fatalf("spec.Stretcher[%d] = true; test premise (no stretcher) violated", i)
		}
	}
	// Solve at a wide terminal. The literal's column must now grow well
	// beyond longestWordWidth("literal") = 7.
	plan := SolveGridLayout(120, spec, func(string, int) int { return 1 })
	if plan.ColumnWidths[2] < 30 {
		t.Errorf("literal col width at term=120 = %d, want >= 30 (promoted to stretcher)",
			plan.ColumnWidths[2])
	}
}

// TestSolveGridLayout_SingleWordCaptionWithRowSpanNotPromoted pins that a
// literal caption like "Tags:" whose RowSpan>1 (because `^` continuations
// appear in rows below it) is NOT promoted to stretcher. Only multi-word
// prose literals should drive the stretcher promotion. Without this guard,
// a row-spanned 1-word caption absorbs terminal slack and leaves a wide
// gap before its value column.
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
	plan := SolveGridLayout(120, spec, func(string, int) int { return 1 })
	// Col 2 is a single-word caption ("Tags:"); it must not be a
	// stretcher recipient. Width should remain at the caption's
	// default-width hint, not balloon to absorb terminal slack.
	if plan.ColumnWidths[2] > len("Tags:")+5 {
		t.Errorf("Tags: caption col grew to %d at term=120; expected ~%d (no stretcher promotion for single-word captions)",
			plan.ColumnWidths[2], len("Tags:")+1)
	}
}

// TestSolveGridLayout_RowSpannedLiteralAtNarrowKeepsBaseWidth pins that
// the stretcher promotion does NOT raise the minimum width — a narrow
// terminal must still get the longest-word minimum and not drop the
// column entirely.
func TestSolveGridLayout_RowSpannedLiteralAtNarrowKeepsBaseWidth(t *testing.T) {
	raw := [][]string{
		{`"X:"`, "status", `"foo bar baz qux"`},
		{`"Y:"`, "type", "^"},
	}
	spec, err := gridlayout.ParseGrid(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// At narrow width, literal col should still be present at its base
	// minimum (longestWordWidth = 3).
	plan := SolveGridLayout(40, spec, func(string, int) int { return 1 })
	if plan.Dropped[2] {
		t.Errorf("literal col dropped at term=40; promotion should not raise the min width")
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
