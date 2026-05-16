package palettes

import "testing"

func TestGithubLightSpecValues(t *testing.T) {
	g := NewGithubLight()
	cases := map[string]struct {
		got  string
		want string
	}{
		"FgDefault":     {g.FgDefault, "#1f2328"},
		"BorderDefault": {g.BorderDefault, "#d0d7de"},
		"BlueAccent":    {g.BlueAccent, "#0969da"},
		"CanvasSubtle":  {g.CanvasSubtle, "#f6f8fa"},
		"Red":           {g.Red, "#cf222e"},
		"Green":         {g.Green, "#116329"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestGithubLightAllFieldsExercised(t *testing.T) {
	g := NewGithubLight()
	_ = g.FgDefault
	_ = g.FgMuted
	_ = g.FgSubtle
	_ = g.BorderDefault
	_ = g.BorderMuted
	_ = g.CanvasDefault
	_ = g.CanvasSubtle
	_ = g.BlueAccent
	_ = g.BlueAccentDark
	_ = g.Slate
	_ = g.Selection
	_ = g.Red
	_ = g.Orange
	_ = g.Green
	for _, c := range g.CaptionFg {
		_ = c
	}
	for _, c := range g.CaptionBg {
		_ = c
	}
}
