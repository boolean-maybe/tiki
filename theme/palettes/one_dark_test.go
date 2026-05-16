package palettes

import "testing"

func TestOneDarkSpecValues(t *testing.T) {
	o := NewOneDark()
	cases := map[string]struct {
		got  string
		want string
	}{
		"Background": {o.Background, "#282c34"},
		"Foreground": {o.Foreground, "#abb2bf"},
		"Blue":       {o.Blue, "#61afef"},
		"Red":        {o.Red, "#e06c75"},
		"Green":      {o.Green, "#98c379"},
		"Yellow":     {o.Yellow, "#e5c07b"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestOneDarkAllFieldsExercised(t *testing.T) {
	o := NewOneDark()
	_ = o.Background
	_ = o.Foreground
	_ = o.Comment
	_ = o.Selection
	_ = o.SoftText
	_ = o.Red
	_ = o.Orange
	_ = o.Yellow
	_ = o.Green
	_ = o.Cyan
	_ = o.Blue
	_ = o.StatuslineDarkBg
	_ = o.StatuslineBorderBg
	for _, c := range o.CaptionFg {
		_ = c
	}
	for _, c := range o.CaptionBg {
		_ = c
	}
	_ = o.CaptionFallbackStart
	_ = o.CaptionFallbackEnd
}
