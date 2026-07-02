package palettes

import "testing"

func TestCatppuccinLatteSpecValues(t *testing.T) {
	c := NewCatppuccinLatte()
	cases := map[string]struct {
		got  string
		want string
	}{
		"Text":   {c.Text, "#4c4f69"},
		"Base":   {c.Base, "#eff1f5"},
		"Blue":   {c.Blue, "#1e66f5"},
		"Yellow": {c.Yellow, "#df8e1d"},
		"Red":    {c.Red, "#d20f39"},
		"Peach":  {c.Peach, "#fe640b"},
	}
	for name, cc := range cases {
		if cc.got != cc.want {
			t.Errorf("%s = %q, want %q", name, cc.got, cc.want)
		}
	}
}

func TestCatppuccinLatteAllFieldsExercised(t *testing.T) {
	c := NewCatppuccinLatte()
	_ = c.Text
	_ = c.Subtext1
	_ = c.Overlay0
	_ = c.Surface0
	_ = c.Surface1
	_ = c.Surface2
	_ = c.Base
	_ = c.Mantle
	_ = c.Crust
	_ = c.Yellow
	_ = c.Green
	_ = c.Teal
	_ = c.Sky
	_ = c.Blue
	_ = c.Red
	_ = c.Peach
	for _, cc := range c.CaptionFg {
		_ = cc
	}
	for _, cc := range c.CaptionBg {
		_ = cc
	}
}
