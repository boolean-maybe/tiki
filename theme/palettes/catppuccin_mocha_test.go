package palettes

import "testing"

func TestCatppuccinMochaSpecValues(t *testing.T) {
	c := NewCatppuccinMocha()
	cases := map[string]struct {
		got  string
		want string
	}{
		"Text":   {c.Text, "#cdd6f4"},
		"Base":   {c.Base, "#1e1e2e"},
		"Crust":  {c.Crust, "#11111b"},
		"Blue":   {c.Blue, "#89b4fa"},
		"Green":  {c.Green, "#a6e3a1"},
		"Red":    {c.Red, "#f38ba8"},
		"Yellow": {c.Yellow, "#f9e2af"},
	}
	for name, cc := range cases {
		if cc.got != cc.want {
			t.Errorf("%s = %q, want %q", name, cc.got, cc.want)
		}
	}
}

func TestCatppuccinMochaAllFieldsExercised(t *testing.T) {
	c := NewCatppuccinMocha()
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
	_ = c.Lavender
	_ = c.Mauve
	_ = c.Red
	_ = c.Peach
	for _, cc := range c.CaptionFg {
		_ = cc
	}
	for _, cc := range c.CaptionBg {
		_ = cc
	}
}
