package config

import "testing"

func TestAllPalettesHaveNonDefaultCriticalFields(t *testing.T) {
	for name, info := range themeRegistry {
		p := info.Palette()
		critical := map[string]Color{
			"TextColor":      p.TextColor,
			"HighlightColor": p.HighlightColor,
			"AccentColor":    p.AccentColor,
			"MutedColor":     p.MutedColor,
			"AccentBlue":     p.AccentBlue,
			"InfoLabelColor": p.InfoLabelColor,
		}
		for field, c := range critical {
			if c.IsDefault() {
				t.Errorf("theme %q: %s is default/transparent", name, field)
			}
		}
	}
}

func TestLightPalettesHaveDarkText(t *testing.T) {
	for name, info := range themeRegistry {
		if !info.Light {
			continue
		}
		p := info.Palette()
		r, g, b := p.TextColor.RGB()
		luminance := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
		if luminance > 160 {
			t.Errorf("light theme %q: TextColor luminance %.0f is too bright (expected dark text)", name, luminance)
		}
	}
}

func TestDarkPalettesHaveLightText(t *testing.T) {
	for name, info := range themeRegistry {
		if info.Light {
			continue
		}
		p := info.Palette()
		r, g, b := p.TextColor.RGB()
		luminance := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
		if luminance < 128 {
			t.Errorf("dark theme %q: TextColor luminance %.0f is too dark (expected light text)", name, luminance)
		}
	}
}
