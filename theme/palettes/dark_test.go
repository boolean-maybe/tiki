package palettes

import "testing"

func TestDarkSpecValues(t *testing.T) {
	d := NewDark()
	cases := map[string]struct {
		got  string
		want string
	}{
		"Highlight":  {d.Highlight, "#ffff00"},
		"Text":       {d.Text, "#ffffff"},
		"Muted":      {d.Muted, "#686868"},
		"AccentBlue": {d.AccentBlue, "#5fafff"},
		"LogoDot":    {d.LogoDot, "#40e0d0"},
		"Danger":     {d.Danger, "#ff4444"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestDarkAllFieldsExercised(t *testing.T) {
	d := NewDark()
	_ = d.Highlight
	_ = d.Text
	_ = d.Muted
	_ = d.SoftBorder
	_ = d.SoftText
	_ = d.Accent
	_ = d.Value
	_ = d.Selection
	_ = d.AccentBlue
	_ = d.Slate
	_ = d.LogoDot
	_ = d.LogoShade
	_ = d.LogoBorder
	_ = d.DeepSkyBlue
	_ = d.StatuslineDarkBg
	_ = d.StatuslineMidBg
	_ = d.StatuslineBorderBg
	_ = d.StatuslineText
	_ = d.StatuslineAccent
	_ = d.Danger
	_ = d.Warn
	_ = d.Ok
	for _, c := range d.CaptionFg {
		_ = c
	}
	for _, c := range d.CaptionBg {
		_ = c
	}
}
