package palettes

// TokyoNight holds the Tokyo Night theme palette.
// Ref: https://github.com/folke/tokyonight.nvim
type TokyoNight struct {
	Foreground string
	Background string
	Comment    string
	Selection  string

	Red    string
	Orange string
	Yellow string
	Green  string
	Cyan   string
	Blue   string

	// Tiki-extras
	Border             string
	SoftText           string
	StatuslineDarkBg   string
	StatuslineMidBg    string
	StatuslineBorderBg string
	StatuslineAccent   string

	CaptionFg [6]string
	CaptionBg [6]string
}

func NewTokyoNight() TokyoNight {
	return TokyoNight{
		Foreground: "#c0caf5",
		Background: "#1a1b26",
		Comment:    "#565f89",
		Selection:  "#283457",

		Red:    "#f7768e",
		Orange: "#ff9e64",
		Yellow: "#e0af68",
		Green:  "#9ece6a",
		Cyan:   "#7dcfff",
		Blue:   "#7aa2f7",

		Border:             "#3b4261",
		SoftText:           "#a9b1d6",
		StatuslineDarkBg:   "#16161e",
		StatuslineMidBg:    "#1a1b26",
		StatuslineBorderBg: "#24283b",
		StatuslineAccent:   "#7aa2f7",

		CaptionFg: [6]string{
			"#c5e9ff",
			"#d3e9bc",
			"#ffd3b9",
			"#efdbba",
			"#b3b7ca",
			"#fbc2cc",
		},
		CaptionBg: [6]string{
			"#263e4d",
			"#2f3e20",
			"#4d2f1e",
			"#44341f",
			"#1a1d29",
			"#4a232a",
		},
	}
}
