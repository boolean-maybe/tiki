package palettes

// Dracula holds the canonical Dracula color values.
// Ref: https://draculatheme.com/contribute
type Dracula struct {
	Background  string
	CurrentLine string
	Foreground  string
	Comment     string
	Cyan        string
	Green       string
	Orange      string
	Pink        string
	Purple      string
	Red         string
	Yellow      string

	// Additional values used by tiki that aren't in the base Dracula spec:
	SoftText           string
	StatuslineDarkBg   string
	StatuslineMidBg    string
	StatuslineBorderBg string
	StatuslineAccent   string

	// Caption color pairs (6 fg/bg per theme)
	CaptionFg [6]string
	CaptionBg [6]string

	// Caption fallback gradient
	CaptionFallbackStart [3]int
	CaptionFallbackEnd   [3]int
}

func NewDracula() Dracula {
	return Dracula{
		Background:  "#282a36",
		CurrentLine: "#44475a",
		Foreground:  "#f8f8f2",
		Comment:     "#6272a4",
		Cyan:        "#8be9fd",
		Green:       "#50fa7b",
		Orange:      "#ffb86c",
		Pink:        "#ff79c6",
		Purple:      "#bd93f9",
		Red:         "#ff5555",
		Yellow:      "#f1fa8c",

		SoftText:           "#bfbfbf",
		StatuslineDarkBg:   "#21222c",
		StatuslineMidBg:    "#282a36",
		StatuslineBorderBg: "#44475a",
		StatuslineAccent:   "#bd93f9",

		CaptionFg: [6]string{
			"#cbf5fe",
			"#b0fdc4",
			"#ffdfbd",
			"#f9fdcb",
			"#b8c0d6",
			"#ffc3e5",
		},
		CaptionBg: [6]string{
			"#2a464c",
			"#184b25",
			"#4d3720",
			"#484b2a",
			"#1d2231",
			"#4d243b",
		},

		CaptionFallbackStart: [3]int{40, 42, 54},
		CaptionFallbackEnd:   [3]int{68, 71, 90},
	}
}
