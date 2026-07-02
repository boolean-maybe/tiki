package palettes

import "testing"

func TestSolarizedLightSpecValues(t *testing.T) {
	s := NewSolarizedLight()
	cases := map[string]struct {
		got  string
		want string
	}{
		"Base3":  {s.Base3, "#fdf6e3"},
		"Base2":  {s.Base2, "#eee8d5"},
		"Base00": {s.Base00, "#657b83"},
		"Yellow": {s.Yellow, "#b58900"},
		"Blue":   {s.Blue, "#268bd2"},
		"Green":  {s.Green, "#859900"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestSolarizedLightAllFieldsExercised(t *testing.T) {
	s := NewSolarizedLight()
	_ = s.Base03
	_ = s.Base02
	_ = s.Base01
	_ = s.Base00
	_ = s.Base0
	_ = s.Base1
	_ = s.Base2
	_ = s.Base3
	_ = s.Yellow
	_ = s.Orange
	_ = s.Red
	_ = s.Magenta
	_ = s.Violet
	_ = s.Blue
	_ = s.Cyan
	_ = s.Green
	for _, c := range s.CaptionFg {
		_ = c
	}
	for _, c := range s.CaptionBg {
		_ = c
	}
}
