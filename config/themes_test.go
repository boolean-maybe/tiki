package config

import (
	"testing"

	chromaStyles "github.com/alecthomas/chroma/v2/styles"
)

func TestThemeRegistryComplete(t *testing.T) {
	names := ThemeNames()
	if len(names) != 14 {
		t.Fatalf("expected 14 themes, got %d: %v", len(names), names)
	}
}

func TestThemeNamesAreSorted(t *testing.T) {
	names := ThemeNames()
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("ThemeNames() not sorted: %q before %q", names[i-1], names[i])
		}
	}
}

func TestAllPalettesResolve(t *testing.T) {
	for name, info := range themeRegistry {
		// calling Palette() must not panic
		p := info.Palette()
		if p.TextColor.IsDefault() {
			t.Errorf("theme %q: TextColor is default/transparent", name)
		}
		if p.HighlightColor.IsDefault() {
			t.Errorf("theme %q: HighlightColor is default/transparent", name)
		}
		if p.AccentColor.IsDefault() {
			t.Errorf("theme %q: AccentColor is default/transparent", name)
		}
	}
}

func TestIsLightThemeClassification(t *testing.T) {
	expectedLight := map[string]bool{
		"dark":             false,
		"light":            true,
		"dracula":          false,
		"tokyo-night":      false,
		"gruvbox-dark":     false,
		"catppuccin-mocha": false,
		"solarized-dark":   false,
		"nord":             false,
		"monokai":          false,
		"one-dark":         false,
		"catppuccin-latte": true,
		"solarized-light":  true,
		"gruvbox-light":    true,
		"github-light":     true,
	}
	for name, wantLight := range expectedLight {
		info, ok := themeRegistry[name]
		if !ok {
			t.Errorf("theme %q not in registry", name)
			continue
		}
		if info.Light != wantLight {
			t.Errorf("theme %q: Light = %v, want %v", name, info.Light, wantLight)
		}
	}
}

func TestUnknownThemeFallsToDark(t *testing.T) {
	// simulate unknown theme by looking up directly in registry
	_, ok := themeRegistry["nonexistent-theme"]
	if ok {
		t.Error("expected nonexistent-theme to not be in registry")
	}
	// lookupTheme() falls back to dark — verify via default
	if defaultTheme.Light {
		t.Error("default theme should be dark (Light=false)")
	}
	if defaultTheme.ChromaTheme != "nord" {
		t.Errorf("default chroma theme = %q, want nord", defaultTheme.ChromaTheme)
	}
}

func TestChromaThemesExist(t *testing.T) {
	for name, info := range themeRegistry {
		style := chromaStyles.Get(info.ChromaTheme)
		if style == nil {
			t.Errorf("theme %q: chroma theme %q not found in chroma registry", name, info.ChromaTheme)
		}
	}
}

func TestNavidownStylesValid(t *testing.T) {
	// navidown supports these style names; unknown names fall back to "dark"
	validNavidown := map[string]bool{
		"dark": true, "light": true,
		"dracula": true, "tokyo-night": true,
		"pink": true, "ascii": true, "notty": true,
		// additional named themes
		"gruvbox-dark": true, "catppuccin-mocha": true,
		"solarized-dark": true, "nord": true,
		"monokai": true, "one-dark": true,
		"catppuccin-latte": true, "solarized-light": true,
		"gruvbox-light": true, "github-light": true,
	}
	for name, info := range themeRegistry {
		if !validNavidown[info.NavidownStyle] {
			t.Errorf("theme %q: navidown style %q is not a known navidown style", name, info.NavidownStyle)
		}
	}
}

func TestChromaThemeForEffectiveNonEmpty(t *testing.T) {
	for name, info := range themeRegistry {
		if info.ChromaTheme == "" {
			t.Errorf("theme %q: ChromaTheme is empty", name)
		}
	}
}
