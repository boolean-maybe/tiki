package palettes

// CatppuccinMocha holds the Catppuccin Mocha (dark) palette.
// Ref: https://catppuccin.com/palette
type CatppuccinMocha struct {
	Text     string
	Subtext1 string
	Overlay0 string
	Surface0 string // #313244
	Surface1 string // #45475a
	Surface2 string // #585b70
	Base     string
	Mantle   string
	Crust    string

	Yellow   string
	Green    string
	Teal     string
	Sky      string
	Blue     string
	Lavender string
	Mauve    string
	Red      string
	Peach    string

	CaptionFg [6]string
	CaptionBg [6]string

	CaptionFallbackStart [3]int
	CaptionFallbackEnd   [3]int
}

func NewCatppuccinMocha() CatppuccinMocha {
	return CatppuccinMocha{
		Text:     "#cdd6f4",
		Subtext1: "#bac2de",
		Overlay0: "#6c7086",
		Surface0: "#313244",
		Surface1: "#45475a",
		Surface2: "#585b70",
		Base:     "#1e1e2e",
		Mantle:   "#181825",
		Crust:    "#11111b",

		Yellow:   "#f9e2af",
		Green:    "#a6e3a1",
		Teal:     "#94e2d5",
		Sky:      "#89dceb",
		Blue:     "#89b4fa",
		Lavender: "#b4befe",
		Mauve:    "#cba6f7",
		Red:      "#f38ba8",
		Peach:    "#fab387",

		CaptionFg: [6]string{
			"#c9dcfd",
			"#d7f2d5",
			"#fdddc9",
			"#fcf0d9",
			"#f9eaea",
			"#cff2ec",
		},
		CaptionBg: [6]string{
			"#293648",
			"#324430",
			"#4b3629",
			"#4b4435",
			"#483d3d",
			"#2c4440",
		},

		CaptionFallbackStart: [3]int{30, 30, 46},
		CaptionFallbackEnd:   [3]int{69, 71, 90},
	}
}
