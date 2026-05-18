package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/theme"
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
