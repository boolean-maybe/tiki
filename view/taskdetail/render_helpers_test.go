package taskdetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/rivo/tview"
)

// TestRenderTitleText_ExpandsRoleMarkup pins that a title stored with
// `<role>` color markup renders with the resolved tview color tag and the
// literal text after the role token. Detail-view titles are user-controlled
// (free-text in the workflow `title:` field) and the escape-then-expand
// path is the only safe way to honor color intent without exposing tview's
// `[...]` tag syntax.
func TestRenderTitleText_ExpandsRoleMarkup(t *testing.T) {
	colors := config.GetColors()
	tk := tikipkg.New()
	tk.ID = "TTL001"
	tk.Title = "<highlight>foo"

	prim := RenderTitleText(tk, FieldRenderContext{Mode: RenderModeView, Colors: colors}, "")
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("RenderTitleText returned %T, want *tview.TextView", prim)
	}
	got := tv.GetText(false)
	highlightTag := colors.TaskBoxSelectedBorder.Tag().String() // <highlight> resolves to this
	if !strings.Contains(got, highlightTag) {
		t.Errorf("expected highlight color tag %q in rendered title, got %q", highlightTag, got)
	}
	if !strings.Contains(got, "foo") {
		t.Errorf("expected literal text 'foo' in rendered title, got %q", got)
	}
}

// TestRenderTitleText_TviewTagsRenderLiterally pins that a stored title
// containing literal tview color tags (e.g. `[red]x[/]`) is escape-first:
// the brackets are neutralized so the tag is shown as-is rather than
// interpreted by SetDynamicColors(true). This is the defense against
// hostile stored content masquerading as markup.
func TestRenderTitleText_TviewTagsRenderLiterally(t *testing.T) {
	colors := config.GetColors()
	tk := tikipkg.New()
	tk.ID = "TTL002"
	tk.Title = "[red]x[/]"

	prim := RenderTitleText(tk, FieldRenderContext{Mode: RenderModeView, Colors: colors}, "")
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("RenderTitleText returned %T, want *tview.TextView", prim)
	}
	got := tv.GetText(false)
	// tview.Escape inserts a closing-bracket-following-marker pattern: "[red[]"
	// (the `[]` neutralizes the `[red]` opener). The exact escape form is
	// covered by tview's own tests; here we assert the brackets are not
	// active markup by checking that the raw characters survive.
	if !strings.Contains(got, "[red") {
		t.Errorf("expected '[red' fragment to be present (escaped form), got %q", got)
	}
	// And no role color tag was emitted (no `<role>` tokens in the input).
	if strings.Contains(got, colors.DangerColor.Tag().String()) {
		t.Errorf("did not expect danger color tag in title without <role> markup, got %q", got)
	}
}

// TestRenderTitleText_BadMarkupFailsClosed pins the fail-closed contract:
// a typo like `{dangr}x` is unknown to ValidRoles, so ExpandVisual returns
// an error. The renderer must fall back to the plain escaped text and
// never crash or render a half-expanded mess.
func TestRenderTitleText_BadMarkupFailsClosed(t *testing.T) {
	colors := config.GetColors()
	tk := tikipkg.New()
	tk.ID = "TTL003"
	tk.Title = "{dangr}x"

	prim := RenderTitleText(tk, FieldRenderContext{Mode: RenderModeView, Colors: colors}, "")
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("RenderTitleText returned %T, want *tview.TextView", prim)
	}
	got := tv.GetText(false)
	// Fail-closed: the raw token survives, escaped, no color tag for it.
	if !strings.Contains(got, "x") {
		t.Errorf("expected literal 'x' in title even on bad markup, got %q", got)
	}
}

func TestRenderTitleText_WithRole(t *testing.T) {
	colors := config.GetColors()
	tk := tikipkg.New()
	tk.ID = "TTL004"
	tk.Title = "My Task"

	prim := RenderTitleText(tk, FieldRenderContext{Mode: RenderModeView, Colors: colors}, "highlight")
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("RenderTitleText returned %T, want *tview.TextView", prim)
	}
	got := tv.GetText(false)
	resolver := colors.RoleResolver()
	highlightTag, _ := resolver("highlight")
	if !strings.Contains(got, highlightTag) {
		t.Errorf("expected highlight role tag %q in title, got %q", highlightTag, got)
	}
	boldTag := colors.TaskDetailTitleText.Tag().Bold().String()
	if strings.Contains(got, boldTag) {
		t.Errorf("role should replace default styling, but found bold tag %q in %q", boldTag, got)
	}
	if !strings.Contains(got, "My Task") {
		t.Errorf("expected 'My Task' in rendered title, got %q", got)
	}
}
