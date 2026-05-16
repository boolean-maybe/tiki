package palettes

// GithubLight holds the GitHub Light (Primer) palette.
// Ref: https://github.com/primer/github-vscode-theme
type GithubLight struct {
	FgDefault     string // #1f2328
	FgMuted       string // #656d76
	FgSubtle      string // #424a53
	BorderDefault string // #d0d7de
	BorderMuted   string // #eaeef2
	CanvasDefault string // #ffffff
	CanvasSubtle  string // #f6f8fa

	BlueAccent     string // #0969da
	BlueAccentDark string // #0550ae
	Slate          string // #8c959f
	Selection      string // #ddf4ff

	Red    string // #cf222e
	Orange string // #953800
	Green  string // #116329

	CaptionFg [6]string
	CaptionBg [6]string

	CaptionFallbackStart [3]int
	CaptionFallbackEnd   [3]int
}

func NewGithubLight() GithubLight {
	return GithubLight{
		FgDefault:     "#1f2328",
		FgMuted:       "#656d76",
		FgSubtle:      "#424a53",
		BorderDefault: "#d0d7de",
		BorderMuted:   "#eaeef2",
		CanvasDefault: "#ffffff",
		CanvasSubtle:  "#f6f8fa",

		BlueAccent:     "#0969da",
		BlueAccentDark: "#0550ae",
		Slate:          "#8c959f",
		Selection:      "#ddf4ff",

		Red:    "#cf222e",
		Orange: "#953800",
		Green:  "#116329",

		CaptionFg: [6]string{
			"#c2daf6",
			"#c4d8ca",
			"#e5cdbf",
			"#e6d9bf",
			"#d9dbdd",
			"#e6d2d2",
		},
		CaptionBg: [6]string{
			"#0a72ed",
			"#188d3b",
			"#ba4600",
			"#9a6700",
			"#5b626a",
			"#9b4a4a",
		},

		CaptionFallbackStart: [3]int{255, 255, 255},
		CaptionFallbackEnd:   [3]int{246, 248, 250},
	}
}
