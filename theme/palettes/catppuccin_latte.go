package palettes

// CatppuccinLatte holds the Catppuccin Latte (light) palette.
// Ref: https://catppuccin.com/palette
type CatppuccinLatte struct {
	Text     string
	Subtext1 string
	Overlay0 string
	Surface0 string // #ccd0da
	Surface1 string // #bcc0cc
	Surface2 string // #acb0be
	Base     string
	Mantle   string
	Crust    string

	Yellow string
	Green  string
	Teal   string
	Sky    string
	Blue   string
	Red    string
	Peach  string

	CaptionFg [6]string
	CaptionBg [6]string
}

func NewCatppuccinLatte() CatppuccinLatte {
	return CatppuccinLatte{
		Text:     "#4c4f69",
		Subtext1: "#5c5f77",
		Overlay0: "#9ca0b0",
		Surface0: "#ccd0da",
		Surface1: "#bcc0cc",
		Surface2: "#acb0be",
		Base:     "#eff1f5",
		Mantle:   "#e6e9ef",
		Crust:    "#dce0e8",

		Yellow: "#df8e1d",
		Green:  "#40a02b",
		Teal:   "#179299",
		Sky:    "#04a5e5",
		Blue:   "#1e66f5",
		Red:    "#d20f39",
		Peach:  "#fe640b",

		CaptionFg: [6]string{
			"#c7d9fd",
			"#f7e3c7",
			"#ffd8c2",
			"#dedfe4",
			"#c5e4e6",
			"#c0e9f9",
		},
		CaptionBg: [6]string{
			"#1e66f5",
			"#8d5a12",
			"#b54708",
			"#5e606f",
			"#148187",
			"#0381b3",
		},
	}
}
