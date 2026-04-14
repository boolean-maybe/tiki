package config

import "testing"

func TestCaptionColorForIndex_Valid(t *testing.T) {
	cc := ColorsFromPalette(DarkPalette())
	for i := 0; i < 6; i++ {
		pair := cc.CaptionColorForIndex(i)
		if pair.Foreground.IsDefault() {
			t.Errorf("index %d: foreground is default", i)
		}
		if pair.Background.IsDefault() {
			t.Errorf("index %d: background is default", i)
		}
	}
}

func TestCaptionColorForIndex_Wraps(t *testing.T) {
	cc := ColorsFromPalette(DarkPalette())
	first := cc.CaptionColorForIndex(0)
	wrapped := cc.CaptionColorForIndex(6)
	if first.Foreground.Hex() != wrapped.Foreground.Hex() {
		t.Errorf("expected index 6 to wrap to index 0: got fg %s vs %s", wrapped.Foreground.Hex(), first.Foreground.Hex())
	}
	if first.Background.Hex() != wrapped.Background.Hex() {
		t.Errorf("expected index 6 to wrap to index 0: got bg %s vs %s", wrapped.Background.Hex(), first.Background.Hex())
	}
}

func TestCaptionColorForIndex_Negative(t *testing.T) {
	cc := ColorsFromPalette(DarkPalette())
	pair := cc.CaptionColorForIndex(-1)
	if !pair.Foreground.IsDefault() {
		t.Errorf("expected default foreground for negative index, got %s", pair.Foreground.Hex())
	}
	if !pair.Background.IsDefault() {
		t.Errorf("expected default background for negative index, got %s", pair.Background.Hex())
	}
}

func TestAllThemesHaveCaptionColors(t *testing.T) {
	for name, info := range themeRegistry {
		p := info.Palette()
		if len(p.CaptionColors) < 6 {
			t.Errorf("theme %q: has %d caption colors, want at least 6", name, len(p.CaptionColors))
		}
	}
}
