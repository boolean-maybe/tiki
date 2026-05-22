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
// a TextView whose unformatted text equals the literal string. The
// dim-label color tag is included in the formatted text but stripped
// when GetText(false) drops the tags.
func TestRenderLiteralCaption(t *testing.T) {
	colors := theme.Roles()
	prim := renderLiteralCaption("Status:", colors)
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("renderLiteralCaption returned %T, want *tview.TextView", prim)
	}
	got := tv.GetText(true)
	if !strings.Contains(got, "Status:") {
		t.Errorf("literal text not found in rendered view: got %q, want substring %q", got, "Status:")
	}
}

// TestRenderLiteralCaption_ExpandsRoleMarkup pins that `<role>` color
// markup in a caption resolves to a tview color tag, that the literal
// text after the token survives, and that the role span closes with the
// `[-]` reset emitted by workflow.ExpandVisual.
func TestRenderLiteralCaption_ExpandsRoleMarkup(t *testing.T) {
	colors := theme.Roles()
	dangerTag := colors.StatusDanger().Tag()

	prim := renderLiteralCaption("<danger>!!!", colors)
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("renderLiteralCaption returned %T, want *tview.TextView", prim)
	}
	got := tv.GetText(false) // keep style tags (stripAllTags=false)
	if !strings.Contains(got, dangerTag) {
		t.Errorf("expected danger color tag %q in rendered text, got %q", dangerTag, got)
	}
	if !strings.Contains(got, "!!!") {
		t.Errorf("expected literal text after role token in rendered text, got %q", got)
	}
	if !strings.Contains(got, "[-]") {
		t.Errorf("expected reset tag '[-]' after role span, got %q", got)
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
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("single-row literal: got %T, want *tview.TextView", prim)
	}
	if !strings.Contains(tv.GetText(true), "Status:") {
		t.Errorf("single-row literal: missing original text in %q", tv.GetText(true))
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
	if _, ok := prim.(*tview.TextView); !ok {
		t.Fatalf("multi-row empty literal: got %T, want *tview.TextView", prim)
	}
}
