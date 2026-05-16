package palettes

// OneDark holds the Atom One Dark palette.
// Ref: https://github.com/Binaryify/OneDark-Pro
type OneDark struct {
	Background string
	Foreground string
	Comment    string
	Selection  string
	SoftText   string

	Red    string
	Orange string
	Yellow string
	Green  string
	Cyan   string
	Blue   string

	// Tiki-extras
	StatuslineDarkBg   string
	StatuslineBorderBg string

	CaptionFg [6]string
	CaptionBg [6]string
}

func NewOneDark() OneDark {
	return OneDark{
		Background: "#282c34",
		Foreground: "#abb2bf",
		Comment:    "#5c6370",
		Selection:  "#3e4452",
		SoftText:   "#9da5b4",

		Red:    "#e06c75",
		Orange: "#d19a66",
		Yellow: "#e5c07b",
		Green:  "#98c379",
		Cyan:   "#56b6c2",
		Blue:   "#61afef",

		StatuslineDarkBg:   "#21252b",
		StatuslineBorderBg: "#3b4048",

		CaptionFg: [6]string{
			"#b8dbf8",
			"#d1e4c3",
			"#ead2ba",
			"#f1deba",
			"#b6b9bf",
			"#eeb2b6",
		},
		CaptionBg: [6]string{
			"#1d3548",
			"#2e3b24",
			"#3f2e1f",
			"#453a25",
			"#1c1e22",
			"#432123",
		},
	}
}
