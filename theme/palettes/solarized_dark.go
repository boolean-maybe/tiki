package palettes

// SolarizedDark holds the Solarized Dark palette.
// Ref: https://ethanschoonover.com/solarized/
type SolarizedDark struct {
	Base03 string // #002b36 — darkest bg
	Base02 string // #073642
	Base01 string // #586e75
	Base00 string // #657b83
	Base0  string // #839496 — default fg dark
	Base1  string // #93a1a1
	Base2  string // #eee8d5
	Base3  string // #fdf6e3

	Yellow  string // #b58900
	Orange  string // #cb4b16
	Red     string // #dc322f
	Magenta string // #d33682
	Violet  string // #6c71c4
	Blue    string // #268bd2
	Cyan    string // #2aa198
	Green   string // #859900

	CaptionFg [6]string
	CaptionBg [6]string
}

func NewSolarizedDark() SolarizedDark {
	return SolarizedDark{
		Base03: "#002b36",
		Base02: "#073642",
		Base01: "#586e75",
		Base00: "#657b83",
		Base0:  "#839496",
		Base1:  "#93a1a1",
		Base2:  "#eee8d5",
		Base3:  "#fdf6e3",

		Yellow:  "#b58900",
		Orange:  "#cb4b16",
		Red:     "#dc322f",
		Magenta: "#d33682",
		Violet:  "#6c71c4",
		Blue:    "#268bd2",
		Cyan:    "#2aa198",
		Green:   "#859900",

		CaptionFg: [6]string{
			"#9dcbeb",
			"#c8d18c",
			"#e8ae96",
			"#9fd5d1",
			"#b4bec1",
			"#ec908f",
		},
		CaptionBg: [6]string{
			"#0b2a3f",
			"#282e00",
			"#3d1707",
			"#0d302e",
			"#1a2123",
			"#420f0e",
		},
	}
}
