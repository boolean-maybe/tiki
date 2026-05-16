package theme

import "testing"

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

func TestRoleResolverClosure(t *testing.T) {
	th := LoadByName("dark")
	resolver := th.RoleResolver()

	tag, ok := resolver("status.danger")
	if !ok {
		t.Fatalf("RoleResolver returned ok=false for status.danger")
	}
	// dark theme's DangerColor is #ff4444; tag should be that hex.
	if want := "[#ff4444]"; tag != want {
		t.Errorf("RoleResolver(status.danger) = %q, want %q", tag, want)
	}

	if _, ok := resolver("unknown.thing"); ok {
		t.Errorf("RoleResolver should return ok=false for unknown names")
	}
}
