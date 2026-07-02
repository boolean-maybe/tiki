package palettes

import "testing"

func TestGruvboxLightSpecValues(t *testing.T) {
	g := NewGruvboxLight()
	cases := map[string]struct {
		got  string
		want string
	}{
		"Bg0":           {g.Bg0, "#fbf1c7"},
		"Bg1":           {g.Bg1, "#ebdbb2"},
		"Fg0":           {g.Fg0, "#3c3836"},
		"NeutralYellow": {g.NeutralYellow, "#9d6104"},
		"NeutralRed":    {g.NeutralRed, "#9d0006"},
		"NeutralGreen":  {g.NeutralGreen, "#79740e"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestGruvboxLightAllFieldsExercised(t *testing.T) {
	g := NewGruvboxLight()
	_ = g.Bg0H
	_ = g.Bg0
	_ = g.Bg1
	_ = g.Bg2
	_ = g.Bg3
	_ = g.Fg0
	_ = g.Fg2
	_ = g.Fg3
	_ = g.Gray
	_ = g.NeutralRed
	_ = g.NeutralOrange
	_ = g.NeutralYellow
	_ = g.NeutralGreen
	_ = g.NeutralBlue
	_ = g.DarkAqua
	for _, c := range g.CaptionFg {
		_ = c
	}
	for _, c := range g.CaptionBg {
		_ = c
	}
}
