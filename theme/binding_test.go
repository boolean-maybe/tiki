package theme

import "testing"

func TestLoadByNameAllThemes(t *testing.T) {
	names := []string{
		"dark", "light", "dracula", "tokyo-night", "gruvbox-dark",
		"catppuccin-mocha", "solarized-dark", "nord", "monokai", "one-dark",
		"catppuccin-latte", "solarized-light", "gruvbox-light", "github-light",
	}
	for _, name := range names {
		th := LoadByName(name)
		if th == nil {
			t.Errorf("LoadByName(%q) returned nil", name)
			continue
		}
		if th.TextPrimary() == nil {
			t.Errorf("%s: TextPrimary is nil", name)
		}
		if th.BorderFocus() == nil {
			t.Errorf("%s: BorderFocus is nil", name)
		}
		if th.StatuslineMain() == nil {
			t.Errorf("%s: StatuslineMain is nil", name)
		}
		if th.PluginCaptions() == nil || th.PluginCaptions().Len() != 6 {
			t.Errorf("%s: PluginCaptions missing or wrong length", name)
		}
		if th.TikiIDGradient() == nil {
			t.Errorf("%s: TikiIDGradient is nil", name)
		}
	}
}

func TestBindingHexParity(t *testing.T) {
	cases := []struct {
		theme string
		role  string
		gotFn func(*Theme) string
		want  string
	}{
		// Dracula
		{"dracula", "TextPrimary", func(t *Theme) string { return t.TextPrimary().Hex() }, "#f8f8f2"},
		{"dracula", "BorderFocus", func(t *Theme) string { return t.BorderFocus().Hex() }, "#ff79c6"},
		{"dracula", "StatusDanger", func(t *Theme) string { return t.StatusDanger().Hex() }, "#ff5555"},
		{"dracula", "StatusOk", func(t *Theme) string { return t.StatusOk().Hex() }, "#50fa7b"},
		{"dracula", "LogoDot", func(t *Theme) string { return t.LogoDot().Hex() }, "#8be9fd"},
		// Dark (named-color preservation)
		{"dark", "TextLabel", func(t *Theme) string { return t.TextLabel().Hex() }, "#008000"}, // green
		// Nord
		{"nord", "TextPrimary", func(t *Theme) string { return t.TextPrimary().Hex() }, "#eceff4"},
		{"nord", "StatusDanger", func(t *Theme) string { return t.StatusDanger().Hex() }, "#bf616a"},
		// GruvboxDark
		{"gruvbox-dark", "TextPrimary", func(t *Theme) string { return t.TextPrimary().Hex() }, "#ebdbb2"},
		{"gruvbox-dark", "StatusDanger", func(t *Theme) string { return t.StatusDanger().Hex() }, "#fb4934"},
		// CatppuccinMocha
		{"catppuccin-mocha", "TextPrimary", func(t *Theme) string { return t.TextPrimary().Hex() }, "#cdd6f4"},
		{"catppuccin-mocha", "StatusDanger", func(t *Theme) string { return t.StatusDanger().Hex() }, "#f38ba8"},
		// GithubLight
		{"github-light", "TextPrimary", func(t *Theme) string { return t.TextPrimary().Hex() }, "#1f2328"},
	}
	for _, c := range cases {
		th := LoadByName(c.theme)
		if got := c.gotFn(th); got != c.want {
			t.Errorf("[%s] %s = %s, want %s", c.theme, c.role, got, c.want)
		}
	}
}

// TestDarkAccentNamedColor confirms Dark.Accent uses the tcell "green" name
// (not the hex), preserving ANSI palette resolution.
func TestDarkAccentNamedColor(t *testing.T) {
	th := bindDark()
	tag := th.TextLabel().Tag()
	if tag != "[green]" {
		t.Errorf("expected named-color tag [green], got %s", tag)
	}
}
