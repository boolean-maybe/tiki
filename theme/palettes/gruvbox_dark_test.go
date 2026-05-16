package palettes

import "testing"

func TestGruvboxDarkSpecValues(t *testing.T) {
	g := NewGruvboxDark()
	cases := map[string]struct {
		got  string
		want string
	}{
		"Bg0H":          {g.Bg0H, "#1d2021"},
		"Bg0":           {g.Bg0, "#282828"},
		"Fg0":           {g.Fg0, "#ebdbb2"},
		"NeutralYellow": {g.NeutralYellow, "#fabd2f"},
		"NeutralRed":    {g.NeutralRed, "#fb4934"},
		"NeutralGreen":  {g.NeutralGreen, "#b8bb26"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestGruvboxDarkAllFieldsExercised(t *testing.T) {
	g := NewGruvboxDark()
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
	_ = g.NeutralAqua
	_ = g.NeutralBlue
	_ = g.DarkAqua
	for _, c := range g.CaptionFg {
		_ = c
	}
	for _, c := range g.CaptionBg {
		_ = c
	}
	_ = g.CaptionFallbackStart
	_ = g.CaptionFallbackEnd
}
