package palettes

// Monokai holds the canonical Monokai palette.
// Ref: https://monokai.pro/
type Monokai struct {
	Background string
	Foreground string
	Comment    string
	Selection  string
	SoftText   string

	Pink   string
	Orange string
	Yellow string
	Green  string
	Cyan   string

	// Tiki-extras
	StatuslineDarkBg   string
	StatuslineBorderBg string

	CaptionFg [6]string
	CaptionBg [6]string
}

func NewMonokai() Monokai {
	return Monokai{
		Background: "#272822",
		Foreground: "#f8f8f2",
		Comment:    "#75715e",
		Selection:  "#49483e",
		SoftText:   "#cfcfc2",

		Pink:   "#f92672",
		Orange: "#fd971f",
		Yellow: "#e6db74",
		Green:  "#a6e22e",
		Cyan:   "#66d9ef",

		StatuslineDarkBg:   "#1e1f1c",
		StatuslineBorderBg: "#3e3d32",

		CaptionFg: [6]string{
			"#baeef8",
			"#d7f2a1",
			"#fed09a",
			"#f2eebc",
			"#c1bfb7",
			"#e08ea5",
		},
		CaptionBg: [6]string{
			"#1f4148",
			"#32440e",
			"#4c2d09",
			"#454223",
			"#23221c",
			"#4b0c22",
		},
	}
}
