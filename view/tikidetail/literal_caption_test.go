package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/theme"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TestRenderLiteralCaption pins the literal-caption primitive shape:
// a TextView whose unformatted text equals the literal string. With no
// role declared, the cell falls back to text.primary.
func TestRenderLiteralCaption(t *testing.T) {
	colors := theme.Roles()
	prim := renderLiteralCaption("Status:", "", "", colors)
	got := extractTextView(prim, true)
	if !strings.Contains(got, "Status:") {
		t.Errorf("literal text not found in rendered view: got %q, want substring %q", got, "Status:")
	}
}

// TestRenderLiteralCaption_PlainLiteralUsesTextPrimary verifies that an
// unmarked literal renders with the text.primary color tag (theme
// foreground), not the previously-hardcoded text.label.
func TestRenderLiteralCaption_PlainLiteralUsesTextPrimary(t *testing.T) {
	colors := theme.Roles()
	a := gridlayout.Anchor{Kind: gridlayout.AnchorLiteral, Text: "Status:", RowSpan: 1, ColSpan: 1}
	prim := renderLiteralAnchor(a, colors)
	got := extractTextView(prim, false)
	wantTag := colors.TextPrimary().Tag()
	if !strings.HasPrefix(got, wantTag) {
		t.Errorf("rendered text %q does not start with text.primary tag %q", got, wantTag)
	}
	labelTag := colors.TextLabel().Tag()
	if labelTag != wantTag && strings.HasPrefix(got, labelTag) {
		t.Errorf("rendered text %q unexpectedly starts with text.label tag (should be text.primary)", got)
	}
}

// TestRenderLiteralCaption_ExplicitRoleUsesThatRole verifies that an
// anchor with an explicit Role uses that role's tag.
func TestRenderLiteralCaption_ExplicitRoleUsesThatRole(t *testing.T) {
	colors := theme.Roles()
	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorLiteral,
		Text:    "Status:",
		Role:    "text.label",
		RowSpan: 1,
		ColSpan: 1,
	}
	prim := renderLiteralAnchor(a, colors)
	got := extractTextView(prim, false)
	wantTag := colors.TextLabel().Tag()
	if !strings.HasPrefix(got, wantTag) {
		t.Errorf("rendered text %q does not start with text.label tag %q", got, wantTag)
	}
}

// TestRenderLiteralAnchor_SingleRowDoesNotWrap pins that a literal anchor
// with RowSpan <= 1 falls through to the single-line text view, preserving
// the historical "Status:"-style caption behavior unchanged. The single-line
// path uses renderLiteralCaption which does not enable word-wrap.
func TestRenderLiteralAnchor_SingleRowDoesNotWrap(t *testing.T) {
	colors := theme.Roles()
	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorLiteral,
		Text:    "Status:",
		RowSpan: 1,
		ColSpan: 1,
	}
	prim := renderLiteralAnchor(a, colors)
	// single-row literals now render through the truncating single-line view
	// (renderLiteralCaption); multi-row literals are the plain wrapping
	// TextView. Assert it is NOT the wrapping primitive and carries the text.
	if _, isPlainWrap := prim.(*tview.TextView); isPlainWrap {
		t.Fatalf("single-row literal: got plain *tview.TextView (the wrapping multi-row shape), want single-line truncating view")
	}
	if got := extractTextView(prim, true); !strings.Contains(got, "Status:") {
		t.Errorf("single-row literal: missing original text in %q", got)
	}
}

// TestRenderLiteralAnchor_MultiRowWraps pins that a literal anchor with
// RowSpan > 1 returns a TextView with word-wrap enabled (the prose-block
// shape, same primitive used by header.InfoWidget for wrapping prose).
// Word-wrap is verified by exercising the Draw path with a long single-line
// string at narrow width and confirming that the rendered output spans
// multiple visual rows.
func TestRenderLiteralAnchor_MultiRowWraps(t *testing.T) {
	colors := theme.Roles()
	long := "Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod"
	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorLiteral,
		Text:    long,
		RowSpan: 3,
		ColSpan: 2,
	}
	prim := renderLiteralAnchor(a, colors)
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("multi-row literal: got %T, want *tview.TextView", prim)
	}
	// Place the view in a narrow rect (15 cols × 10 rows) and draw to a
	// simulation screen. With word-wrap enabled, the long single-line text
	// must produce content on multiple rows; without word-wrap it would
	// only paint row 0.
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init simulation screen: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(15, 10)
	tv.SetRect(0, 0, 15, 10)
	tv.Draw(screen)
	rowsWithContent := 0
	for y := 0; y < 10; y++ {
		hasContent := false
		for x := 0; x < 15; x++ {
			r, _, _, _ := screen.GetContent(x, y)
			if r != ' ' && r != 0 {
				hasContent = true
				break
			}
		}
		if hasContent {
			rowsWithContent++
		}
	}
	if rowsWithContent < 2 {
		t.Errorf("expected wrapped prose to span >=2 rows at width 15, got %d rows with content", rowsWithContent)
	}
}

// TestRenderLiteralAnchor_MultiRowEmptyTextFallsBackToSingleLine pins that
// a row-spanned literal whose text is whitespace-only falls back to the
// single-line caption path (defensive guard for malformed authors).
func TestRenderLiteralAnchor_MultiRowEmptyTextFallsBackToSingleLine(t *testing.T) {
	colors := theme.Roles()
	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorLiteral,
		Text:    "   ",
		RowSpan: 3,
		ColSpan: 2,
	}
	prim := renderLiteralAnchor(a, colors)
	// whitespace-only row-spanned literal falls back to the single-line caption
	// path (renderLiteralCaption), which is the truncating single-line view —
	// NOT the plain wrapping TextView used for real multi-row prose.
	if _, isPlainWrap := prim.(*tview.TextView); isPlainWrap {
		t.Fatalf("multi-row empty literal: got plain *tview.TextView, want single-line caption fallback")
	}
}
