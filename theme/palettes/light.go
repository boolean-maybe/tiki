package palettes

// Light is the tiki built-in light palette. No external spec — fields mirror
// the descriptive names from the legacy config.Palette struct.
type Light struct {
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

	CaptionFallbackStart [3]int
	CaptionFallbackEnd   [3]int
}

func NewLight() Light {
	return Light{
		Highlight:   "#0055dd",
		Text:        "black",
		Muted:       "#808080",
		SoftBorder:  "#b0b8c8",
		SoftText:    "#404040",
		Accent:      "#006400",
		Value:       "#4a4e6a",
		Selection:   "#b8d4f0",
		AccentBlue:  "#0060c0",
		Slate:       "#7080a0",
		LogoDot:     "#20a090",
		LogoShade:   "#3060a0",
		LogoBorder:  "#6080a0",
		DeepSkyBlue: "#0064b4",

		StatuslineDarkBg:   "#eceff4",
		StatuslineMidBg:    "#e5e9f0",
		StatuslineBorderBg: "#d8dee9",
		StatuslineText:     "#2e3440",
		StatuslineAccent:   "#5e81ac",

		Danger: "#cc0000",
		Warn:   "#b85c00",
		Ok:     "#4c7a5a",

		CaptionFg: [6]string{
			"#e0f0ff",
			"#d2ded6",
			"#edd6bf",
			"#d2d3da",
			"#c7e7e3",
			"#dfdfdf",
		},
		CaptionBg: [6]string{
			"#3a6a90",
			"#467153",
			"#a45200",
			"#5a5e80",
			"#1a8174",
			"#616161",
		},

		CaptionFallbackStart: [3]int{100, 140, 200},
		CaptionFallbackEnd:   [3]int{60, 100, 180},
	}
}
