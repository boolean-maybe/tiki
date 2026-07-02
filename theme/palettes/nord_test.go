package palettes

import "testing"

func TestNordSpecValues(t *testing.T) {
	n := NewNord()
	cases := map[string]struct {
		got  string
		want string
	}{
		"PolarNight0": {n.PolarNight0, "#2e3440"},
		"PolarNight3": {n.PolarNight3, "#4c566a"},
		"SnowStorm2":  {n.SnowStorm2, "#eceff4"},
		"Frost1":      {n.Frost1, "#88c0d0"},
		"Aurora0":     {n.Aurora0, "#bf616a"},
		"Aurora3":     {n.Aurora3, "#a3be8c"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestNordAllFieldsExercised(t *testing.T) {
	n := NewNord()
	_ = n.PolarNight0
	_ = n.PolarNight1
	_ = n.PolarNight2
	_ = n.PolarNight3
	_ = n.SnowStorm0
	_ = n.SnowStorm1
	_ = n.SnowStorm2
	_ = n.Frost0
	_ = n.Frost1
	_ = n.Frost2
	_ = n.Frost3
	_ = n.Aurora0
	_ = n.Aurora1
	_ = n.Aurora2
	_ = n.Aurora3
	_ = n.Aurora4
	for _, c := range n.CaptionFg {
		_ = c
	}
	for _, c := range n.CaptionBg {
		_ = c
	}
}
