package palettes

import "testing"

func TestDraculaSpecValues(t *testing.T) {
	d := NewDracula()
	cases := map[string]struct {
		got  string
		want string
	}{
		"Background":  {d.Background, "#282a36"},
		"CurrentLine": {d.CurrentLine, "#44475a"},
		"Foreground":  {d.Foreground, "#f8f8f2"},
		"Comment":     {d.Comment, "#6272a4"},
		"Cyan":        {d.Cyan, "#8be9fd"},
		"Green":       {d.Green, "#50fa7b"},
		"Orange":      {d.Orange, "#ffb86c"},
		"Pink":        {d.Pink, "#ff79c6"},
		"Purple":      {d.Purple, "#bd93f9"},
		"Red":         {d.Red, "#ff5555"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestDraculaAllFieldsExercised(t *testing.T) {
	d := NewDracula()
	// exercise every field so the unused linter doesn't trip before binding lands
	_ = d.Background
	_ = d.CurrentLine
	_ = d.Foreground
	_ = d.Comment
	_ = d.Cyan
	_ = d.Green
	_ = d.Orange
	_ = d.Pink
	_ = d.Purple
	_ = d.Red
	_ = d.Yellow
	_ = d.SoftText
	_ = d.StatuslineDarkBg
	_ = d.StatuslineMidBg
	_ = d.StatuslineBorderBg
	_ = d.StatuslineAccent
	for _, c := range d.CaptionFg {
		_ = c
	}
	for _, c := range d.CaptionBg {
		_ = c
	}
}
