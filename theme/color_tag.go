package theme

// ColorTag is a composable builder for tview [fg:bg:attr] color tags.
type ColorTag struct {
	fg   Color
	bg   *Color
	bold bool
}

func (t ColorTag) Bold() ColorTag {
	t.bold = true
	return t
}

func (t ColorTag) WithBg(c Color) ColorTag {
	t.bg = &c
	return t
}

// String renders the tview color tag string.
// Named colors are preserved so tview uses the terminal's ANSI palette.
func (t ColorTag) String() string {
	fg := t.fg.tagColor()

	hasBg := t.bg != nil
	if !hasBg && !t.bold {
		return "[" + fg + "]"
	}

	bg := "-"
	if hasBg {
		bg = t.bg.tagColor()
	}

	attr := ""
	if t.bold {
		attr = "b"
	}

	return "[" + fg + ":" + bg + ":" + attr + "]"
}
