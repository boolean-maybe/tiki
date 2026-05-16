package theme

import (
	"strings"
	"testing"

	gradcore "github.com/boolean-maybe/tiki/internal/gradient"
)

func TestResolveByNameCanonical(t *testing.T) {
	th := LoadByName("dark")

	names := []string{
		"text.primary", "text.secondary", "text.muted", "text.label",
		"text.value", "text.hint",
		"border.focus", "border.idle",
		"surface.transparent", "surface.selection", "surface.canvas",
		"highlight", "accent.action", "accent.tag",
		"status.danger", "status.warn", "status.ok",
		"statusline.main.fg", "statusline.main.bg",
		"statusline.accent.fg", "statusline.accent.bg",
		"statusline.info.fg", "statusline.info.bg",
		"statusline.error.fg", "statusline.error.bg",
		"statusline.fill",
		"deps-editor.surface",
		"logo.dot", "logo.shade", "logo.border",
	}
	for _, name := range names {
		role, ok := th.ResolveByName(name)
		if !ok {
			t.Errorf("ResolveByName(%q) = _, false; want true", name)
			continue
		}
		if role == nil {
			t.Errorf("ResolveByName(%q) returned (nil, true)", name)
		}
	}
	// 30 canonical names — matches the spec vocabulary.
	if got, want := len(names), 30; got != want {
		t.Errorf("canonical name count: got %d, want %d", got, want)
	}
}

func TestResolveByNameAliases(t *testing.T) {
	th := LoadByName("dark")
	cases := map[string]string{
		"muted":     "text.muted",
		"accent":    "text.label",
		"highlight": "highlight",
		"info":      "status.warn",
		"action":    "accent.action",
		"text":      "text.primary",
		"danger":    "status.danger",
		"warn":      "status.warn",
		"ok":        "status.ok",
	}
	for alias, canonical := range cases {
		aliasRole, aliasOk := th.ResolveByName(alias)
		canonRole, canonOk := th.ResolveByName(canonical)
		if !aliasOk {
			t.Errorf("alias %q not resolvable", alias)
			continue
		}
		if !canonOk {
			t.Errorf("canonical %q not resolvable", canonical)
			continue
		}
		if aliasRole.Hex() != canonRole.Hex() {
			t.Errorf("alias %q = %s, canonical %q = %s; mismatch",
				alias, aliasRole.Hex(), canonical, canonRole.Hex())
		}
	}
}

func TestResolveByNameUnknown(t *testing.T) {
	th := LoadByName("dark")
	if _, ok := th.ResolveByName("definitely.not.a.role"); ok {
		t.Errorf("expected ok=false for unknown role name")
	}
	if _, ok := th.ResolveByName(""); ok {
		t.Errorf("expected ok=false for empty name")
	}
}

// TestResolverEveryThemeAllNames sanity-checks that every canonical name
// resolves cleanly under every theme — catches accidental nil roles in any
// bind function.
func TestResolverEveryThemeAllNames(t *testing.T) {
	themes := []string{
		"dark", "light", "dracula", "tokyo-night", "gruvbox-dark",
		"catppuccin-mocha", "solarized-dark", "nord", "monokai", "one-dark",
		"catppuccin-latte", "solarized-light", "gruvbox-light", "github-light",
	}
	// Derive the test vocabulary from KnownRoleNames so adding a name to the
	// resolver also adds it to this coverage check.
	names := KnownRoleNames()
	for _, themeName := range themes {
		th := LoadByName(themeName)
		for _, name := range names {
			role, ok := th.ResolveByName(name)
			if !ok || role == nil {
				t.Errorf("[%s] ResolveByName(%q) failed (ok=%v role=%v)", themeName, name, ok, role)
			}
		}
	}
}

func TestPaintResolver_SolidRole(t *testing.T) {
	th := LoadByName("dark")
	resolver := th.PaintResolver()
	p, ok := resolver("status.danger", "")
	if !ok || p == nil {
		t.Fatalf("PaintResolver(status.danger, \"\") = (%v, %v); want non-nil + true", p, ok)
	}
	want := th.StatusDanger().Tag() + "x[-]"
	if got := p.PaintString("x"); got != want {
		t.Errorf("PaintString(\"x\") = %q, want %q", got, want)
	}
}

func TestPaintResolver_GradientModifier(t *testing.T) {
	gradcore.UseGradients.Store(true)
	defer gradcore.UseGradients.Store(false)
	th := LoadByName("dark")
	resolver := th.PaintResolver()
	for _, modifier := range []string{"accent", "lift"} {
		p, ok := resolver("tiki.id", modifier)
		if !ok || p == nil {
			t.Errorf("PaintResolver(tiki.id, %q) = (%v, %v); want non-nil + true", modifier, p, ok)
			continue
		}
		got := p.PaintString("AB")
		if !strings.Contains(got, "[#") {
			t.Errorf("modifier %q: expected per-rune color tags in %q", modifier, got)
		}
	}
}

func TestPaintResolver_UnknownRole(t *testing.T) {
	th := LoadByName("dark")
	resolver := th.PaintResolver()
	if _, ok := resolver("nosuchrole", ""); ok {
		t.Error("PaintResolver returned ok=true for unknown role")
	}
}

func TestPaintResolver_UnknownModifier(t *testing.T) {
	th := LoadByName("dark")
	resolver := th.PaintResolver()
	if _, ok := resolver("status.danger", "nosuchmod"); ok {
		t.Error("PaintResolver returned ok=true for unknown modifier")
	}
}
