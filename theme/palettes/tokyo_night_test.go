package palettes

import "testing"

func TestTokyoNightSpecValues(t *testing.T) {
	tn := NewTokyoNight()
	cases := map[string]struct {
		got  string
		want string
	}{
		"Foreground": {tn.Foreground, "#c0caf5"},
		"Background": {tn.Background, "#1a1b26"},
		"Blue":       {tn.Blue, "#7aa2f7"},
		"Cyan":       {tn.Cyan, "#7dcfff"},
		"Red":        {tn.Red, "#f7768e"},
		"Yellow":     {tn.Yellow, "#e0af68"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestTokyoNightAllFieldsExercised(t *testing.T) {
	tn := NewTokyoNight()
	_ = tn.Foreground
	_ = tn.Background
	_ = tn.Comment
	_ = tn.Selection
	_ = tn.Red
	_ = tn.Orange
	_ = tn.Yellow
	_ = tn.Green
	_ = tn.Cyan
	_ = tn.Blue
	_ = tn.Border
	_ = tn.SoftText
	_ = tn.StatuslineDarkBg
	_ = tn.StatuslineMidBg
	_ = tn.StatuslineBorderBg
	_ = tn.StatuslineAccent
	for _, c := range tn.CaptionFg {
		_ = c
	}
	for _, c := range tn.CaptionBg {
		_ = c
	}
}
