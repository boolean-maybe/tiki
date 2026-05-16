package palettes

import "testing"

func TestLightSpecValues(t *testing.T) {
	l := NewLight()
	cases := map[string]struct {
		got  string
		want string
	}{
		"Highlight":  {l.Highlight, "#0055dd"},
		"Muted":      {l.Muted, "#808080"},
		"Accent":     {l.Accent, "#006400"},
		"AccentBlue": {l.AccentBlue, "#0060c0"},
		"LogoDot":    {l.LogoDot, "#20a090"},
		"Danger":     {l.Danger, "#cc0000"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestLightAllFieldsExercised(t *testing.T) {
	l := NewLight()
	_ = l.Highlight
	_ = l.Text
	_ = l.Muted
	_ = l.SoftBorder
	_ = l.SoftText
	_ = l.Accent
	_ = l.Value
	_ = l.Selection
	_ = l.AccentBlue
	_ = l.Slate
	_ = l.LogoDot
	_ = l.LogoShade
	_ = l.LogoBorder
	_ = l.DeepSkyBlue
	_ = l.StatuslineDarkBg
	_ = l.StatuslineMidBg
	_ = l.StatuslineBorderBg
	_ = l.StatuslineText
	_ = l.StatuslineAccent
	_ = l.Danger
	_ = l.Warn
	_ = l.Ok
	for _, c := range l.CaptionFg {
		_ = c
	}
	for _, c := range l.CaptionBg {
		_ = c
	}
}
