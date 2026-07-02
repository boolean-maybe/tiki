package palettes

// Dark is the tiki built-in dark palette. No external spec — fields mirror the
// descriptive names from the legacy config.Palette struct.
type Dark struct {
	Highlight   string
	Text        string
	Muted       string
	SoftBorder  string
	SoftText    string
	Accent      string
	Value       string
	Selection   string
	AccentBlue  string
	Slate       string
	LogoDot     string
	LogoShade   string
	LogoBorder  string
	DeepSkyBlue string

	StatuslineDarkBg   string
	StatuslineMidBg    string
	StatuslineBorderBg string
	StatuslineText     string
	StatuslineAccent   string

	Danger string
	Warn   string
	Ok     string

	CaptionFg [6]string
	CaptionBg [6]string
}

func NewDark() Dark {
	return Dark{
		Highlight:   "#ffff00",
		Text:        "#ffffff",
		Muted:       "#686868",
		SoftBorder:  "#686868",
		SoftText:    "#b4b4b4",
		Accent:      "green",
		Value:       "#8c92ac",
		Selection:   "#3a5f8a",
		AccentBlue:  "#5fafff",
		Slate:       "#5f6982",
		LogoDot:     "#40e0d0",
		LogoShade:   "#4682b4",
		LogoBorder:  "#324664",
		DeepSkyBlue: "#00bfff",

		StatuslineDarkBg:   "#2e3440",
		StatuslineMidBg:    "#3b4252",
		StatuslineBorderBg: "#434c5e",
		StatuslineText:     "#d8dee9",
		StatuslineAccent:   "#5e81ac",

		Danger: "#ff4444",
		Warn:   "#ffa500",
		Ok:     "#a3be8c",

		CaptionFg: [6]string{
			"#87ceeb",
			"#8cd98c",
			"#ffd78c",
			"#a9f1ea",
			"#b7bcc7",
			"#b0c4d4",
		},
		CaptionBg: [6]string{
			"#25496a",
			"#003300",
			"#4d3200",
			"#13433e",
			"#1d2027",
			"#1e2d3a",
		},
	}
}
