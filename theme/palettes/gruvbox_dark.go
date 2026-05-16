package palettes

// GruvboxDark holds the Gruvbox Dark palette.
// Ref: https://github.com/morhetz/gruvbox
type GruvboxDark struct {
	Bg0H, Bg0, Bg1, Bg2, Bg3 string
	Fg0, Fg2, Fg3            string
	Gray                     string
	NeutralRed               string
	NeutralOrange            string
	NeutralYellow            string
	NeutralGreen             string
	NeutralAqua              string
	NeutralBlue              string
	DarkAqua                 string

	CaptionFg [6]string
	CaptionBg [6]string

	CaptionFallbackStart [3]int
	CaptionFallbackEnd   [3]int
}

func NewGruvboxDark() GruvboxDark {
	return GruvboxDark{
		Bg0H:          "#1d2021",
		Bg0:           "#282828",
		Bg1:           "#3c3836",
		Bg2:           "#504945",
		Bg3:           "#665c54",
		Fg0:           "#ebdbb2",
		Fg2:           "#bdae93",
		Fg3:           "#bdae93",
		Gray:          "#928374",
		NeutralRed:    "#fb4934",
		NeutralOrange: "#fe8019",
		NeutralYellow: "#fabd2f",
		NeutralGreen:  "#b8bb26",
		NeutralAqua:   "#8ec07c",
		NeutralBlue:   "#83a598",
		DarkAqua:      "#689d6a",

		CaptionFg: [6]string{
			"#c7d7d1",
			"#dfe09d",
			"#ffc698",
			"#fcdfaa",
			"#bab6b2",
			"#fd9a90",
		},
		CaptionBg: [6]string{
			"#27322e",
			"#37380b",
			"#4c2608",
			"#4b390e",
			"#1f1c19",
			"#4b1610",
		},

		CaptionFallbackStart: [3]int{40, 40, 40},
		CaptionFallbackEnd:   [3]int{80, 73, 69},
	}
}
