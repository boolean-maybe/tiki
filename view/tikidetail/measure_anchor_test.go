package tikidetail

import (
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/rivo/tview"
)

// TestMeasureAnchor_RowSpannedCompositeWrapsByLongestWord pins that a
// composite anchor spanning more than one row measures by its longest
// rendered word, not the full single-line width. Without this, a multi-row
// prose composite (e.g. the bundled Project view's blurb) reports a width
// equal to the whole paragraph and the solver sheds it, even though it would
// word-wrap to fit within its row span. Mirrors the row-span-aware behavior
// gridbox.measureComposite already provides for the gridbox draw path.
func TestMeasureAnchor_RowSpannedCompositeWrapsByLongestWord(t *testing.T) {
	tk := tikipkg.New()
	tk.SetID("PROSE1")
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}

	// a composite of three literal segments whose concatenation is a long
	// single line, but whose longest word is short.
	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorComposite,
		RowSpan: 3,
		ColSpan: 2,
		Segments: []gridlayout.Segment{
			{Kind: gridlayout.SegmentLiteral, Text: "Projects gather related tasks into a single unit of planning "},
			{Kind: gridlayout.SegmentLiteral, Text: "and move across Now Next and Later as priorities shift over time."},
		},
	}

	got := MeasureAnchor(a, tk, ctx)

	full := MeasureAnchor(
		gridlayout.Anchor{Kind: gridlayout.AnchorComposite, RowSpan: 1, ColSpan: 2, Segments: a.Segments},
		tk, ctx,
	)
	// longest word here is "priorities" (10) or "Projects"/"planning" (8) →
	// well under the full single-line width.
	if got >= full {
		t.Fatalf("row-spanned composite measured %d; want < full single-line width %d (should wrap by longest word)", got, full)
	}
	if got > 15 {
		t.Errorf("row-spanned composite measured %d; longest word is ~10 cells, expected a small wrap floor", got)
	}
}

// TestMeasureAnchor_SingleRowCompositeUsesFullWidth pins that a single-row
// composite still measures its full rendered width — only multi-row
// composites are allowed to wrap.
func TestMeasureAnchor_SingleRowCompositeUsesFullWidth(t *testing.T) {
	tk := tikipkg.New()
	tk.SetID("ONELN1")
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}

	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorComposite,
		RowSpan: 1,
		ColSpan: 1,
		Segments: []gridlayout.Segment{
			{Kind: gridlayout.SegmentLiteral, Text: "Done In Progress"},
		},
	}
	got := MeasureAnchor(a, tk, ctx)
	if got < len("Done In Progress") {
		t.Errorf("single-row composite measured %d; want full width >= %d", got, len("Done In Progress"))
	}
}

// TestMeasureAnchor_SingleRowCompositeReservesBreathingCell pins that a
// single-row composite reserves the same one-cell breathing gap every other
// truncator-backed cell does (scalarCellWidth, the literal +1). The composite
// draws through gridbox.NewTruncatingTextView, which truncates to width-1; if
// the measure returned only the content width the solver would size an N-cell
// column and the draw would clip to N-1 — the "In Progress ⚙️" → "In Progress …"
// bug. Measured width must therefore be content display width + 1.
func TestMeasureAnchor_SingleRowCompositeReservesBreathingCell(t *testing.T) {
	tk := tikipkg.New()
	tk.SetID("BREATH")
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}

	// "In Progress ⚙️": base gear U+2699 + variation selector U+FE0F is one
	// grapheme cluster of display width 2, so the content is 14 cells.
	text := "In Progress ⚙️"
	a := gridlayout.Anchor{
		Kind:     gridlayout.AnchorComposite,
		RowSpan:  1,
		ColSpan:  1,
		Segments: []gridlayout.Segment{{Kind: gridlayout.SegmentLiteral, Text: text}},
	}

	got := MeasureAnchor(a, tk, ctx)
	want := tview.TaggedStringWidth(text) + scalarBreathingCell
	if got != want {
		t.Errorf("single-row composite measured %d; want content+breathing cell = %d", got, want)
	}
}

// TestMeasureFieldValue_EnumReservesWidestLabel pins that an enum field's
// column reserves the widest declared label's width regardless of the stored
// value. The in-place enum editor cycles labels without the grid re-solving,
// so a column sized to a short stored value ("low") would clip when the user
// cycles to a longer one ("critical"). Sizing to the widest label makes the
// column stable across values and never clip — in view and edit mode alike.
func TestMeasureFieldValue_EnumReservesWidestLabel(t *testing.T) {
	cleanup := registerExtraWorkflowFieldForTest(t, "severity", []string{"low", "medium", "critical"})
	defer cleanup()

	tk := tikipkg.New()
	tk.SetID("ENUMWD")
	tk.Set("severity", "low") // stored value is the SHORTEST label
	ctx := FieldRenderContext{Mode: RenderModeView, FieldName: "severity", Roles: theme.Roles()}

	got := MeasureFieldValue("severity", tk, ctx)
	// "critical" is the widest label; the column must fit it plus the breathing
	// cell every scalar reserves, even though the stored value is "low".
	want := tview.TaggedStringWidth("critical") + scalarBreathingCell
	if got != want {
		t.Errorf("enum column measured %d for stored 'low'; want widest-label width %d", got, want)
	}
}
