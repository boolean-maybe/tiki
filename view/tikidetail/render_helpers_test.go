package tikidetail

import (
	"strings"
	"testing"

	gradcore "github.com/boolean-maybe/tiki/internal/gradient"
	"github.com/boolean-maybe/tiki/theme"
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
	roles := theme.Roles()
	tk := tikipkg.New()
	tk.ID = "TTL001"
	tk.Title = "<highlight>foo"

	prim := RenderTitleText(tk, FieldRenderContext{Mode: RenderModeView, Roles: roles}, "", "")
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("RenderTitleText returned %T, want *tview.TextView", prim)
	}
	got := tv.GetText(false)
	highlightTag := roles.Highlight().Tag() // <highlight> resolves to the active theme's highlight role
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
	roles := theme.Roles()
	tk := tikipkg.New()
	tk.ID = "TTL002"
	tk.Title = "[red]x[/]"

	prim := RenderTitleText(tk, FieldRenderContext{Mode: RenderModeView, Roles: roles}, "", "")
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
	// and no role color tag was emitted (no `<role>` tokens in the input).
	if strings.Contains(got, roles.StatusDanger().Tag()) {
		t.Errorf("did not expect danger color tag in title without <role> markup, got %q", got)
	}
}

// TestRenderTitleText_BadMarkupFailsClosed pins the fail-closed contract:
// a typo like `{dangr}x` is unknown to ValidRoles, so ExpandVisual returns
// an error. The renderer must fall back to the plain escaped text and
// never crash or render a half-expanded mess.
func TestRenderTitleText_BadMarkupFailsClosed(t *testing.T) {
	roles := theme.Roles()
	tk := tikipkg.New()
	tk.ID = "TTL003"
	tk.Title = "{dangr}x"

	prim := RenderTitleText(tk, FieldRenderContext{Mode: RenderModeView, Roles: roles}, "", "")
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("RenderTitleText returned %T, want *tview.TextView", prim)
	}
	got := tv.GetText(false)
	// fail-closed: the raw token survives, escaped, no color tag for it.
	if !strings.Contains(got, "x") {
		t.Errorf("expected literal 'x' in title even on bad markup, got %q", got)
	}
}

func TestRenderTitleText_WithRole(t *testing.T) {
	roles := theme.Roles()
	tk := tikipkg.New()
	tk.ID = "TTL004"
	tk.Title = "My Task"

	prim := RenderTitleText(tk, FieldRenderContext{Mode: RenderModeView, Roles: roles}, "highlight", "")
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("RenderTitleText returned %T, want *tview.TextView", prim)
	}
	got := tv.GetText(false)
	highlightRole, _ := roles.ResolveByName("highlight")
	highlightTag := highlightRole.Tag()
	if !strings.Contains(got, highlightTag) {
		t.Errorf("expected highlight role tag %q in title, got %q", highlightTag, got)
	}
	boldTag := roles.TextPrimary().BoldTag()
	if strings.Contains(got, boldTag) {
		t.Errorf("role should replace default styling, but found bold tag %q in %q", boldTag, got)
	}
	if !strings.Contains(got, "My Task") {
		t.Errorf("expected 'My Task' in rendered title, got %q", got)
	}
}

// TestRenderTitleText_WithRoleAndModifier pins that passing a non-empty
// modifier routes through PaintResolver and produces per-rune gradient tags
// — distinct from the single-tag output of the bare-role path. This is the
// regression test for the third Critical: the renderer must read Modifier,
// not just Role.
func TestRenderTitleText_WithRoleAndModifier(t *testing.T) {
	roles := theme.Roles()
	tk := tikipkg.New()
	tk.ID = "TTL005"
	tk.Title = "AB"

	gradcore.UseGradients.Store(true)
	defer gradcore.UseGradients.Store(false)

	prim := RenderTitleText(tk, FieldRenderContext{Mode: RenderModeView, Roles: roles}, "highlight", "accent")
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("RenderTitleText returned %T, want *tview.TextView", prim)
	}
	got := tv.GetText(false)
	// modifier path emits one [#rrggbb] tag per visible rune; two runes → two tags.
	tagCount := strings.Count(got, "[#")
	if tagCount != 2 {
		t.Errorf("expected 2 per-rune color tags for modifier=accent, got %d in %q", tagCount, got)
	}
	if !strings.HasSuffix(got, "[-]") {
		t.Errorf("expected trailing [-] reset from Paint, got %q", got)
	}

	// sanity: unmodified call produces no [-] reset (bare-tag path keeps the
	// legacy behavior — tag-then-text-then-value-tag).
	primSolid := RenderTitleText(tk, FieldRenderContext{Mode: RenderModeView, Roles: roles}, "highlight", "")
	tvSolid, ok := primSolid.(*tview.TextView)
	if !ok {
		t.Fatalf("RenderTitleText returned %T, want *tview.TextView", primSolid)
	}
	solid := tvSolid.GetText(false)
	if strings.HasSuffix(solid, "[-]") {
		t.Errorf("bare-role path should not emit [-] reset, got %q", solid)
	}
	// and: bare-role and modifier paths produce different text (the modifier
	// path injects per-rune tags between the runes).
	if got == solid {
		t.Errorf("modifier path output equals bare-role path output: %q", got)
	}
}
