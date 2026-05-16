package palettes

import "testing"

func TestMonokaiSpecValues(t *testing.T) {
	m := NewMonokai()
	cases := map[string]struct {
		got  string
		want string
	}{
		"Background": {m.Background, "#272822"},
		"Foreground": {m.Foreground, "#f8f8f2"},
		"Pink":       {m.Pink, "#f92672"},
		"Yellow":     {m.Yellow, "#e6db74"},
		"Green":      {m.Green, "#a6e22e"},
		"Cyan":       {m.Cyan, "#66d9ef"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestMonokaiAllFieldsExercised(t *testing.T) {
	m := NewMonokai()
	_ = m.Background
	_ = m.Foreground
	_ = m.Comment
	_ = m.Selection
	_ = m.SoftText
	_ = m.Pink
	_ = m.Orange
	_ = m.Yellow
	_ = m.Green
	_ = m.Cyan
	_ = m.StatuslineDarkBg
	_ = m.StatuslineBorderBg
	for _, c := range m.CaptionFg {
		_ = c
	}
	for _, c := range m.CaptionBg {
		_ = c
	}
	_ = m.CaptionFallbackStart
	_ = m.CaptionFallbackEnd
}
