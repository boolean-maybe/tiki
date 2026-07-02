package palettes

// SolarizedLight holds the Solarized Light palette.
// Ref: https://ethanschoonover.com/solarized/
type SolarizedLight struct {
	Base03 string // #002b36
	Base02 string // #073642
	Base01 string // #586e75
	Base00 string // #657b83 — default fg light
	Base0  string // #839496
	Base1  string // #93a1a1
	Base2  string // #eee8d5
	Base3  string // #fdf6e3 — lightest bg

	Yellow  string
	Orange  string
	Red     string
	Magenta string
	Violet  string
	Blue    string
	Cyan    string
	Green   string

	CaptionFg [6]string
	CaptionBg [6]string
}

func NewSolarizedLight() SolarizedLight {
	return SolarizedLight{
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
			"#c9e2f4",
			"#ede2bf",
			"#f2d2c5",
			"#d5dbdd",
			"#cae8e5",
			"#e1e6bf",
		},
		CaptionBg: [6]string{
			"#2073ae",
			"#826300",
			"#b74414",
			"#52666d",
			"#217d76",
			"#637200",
		},
	}
}
