package palettes

// GruvboxLight holds the Gruvbox Light palette.
// Ref: https://github.com/morhetz/gruvbox
type GruvboxLight struct {
	Bg0H, Bg0, Bg1, Bg2, Bg3 string
	Fg0, Fg2, Fg3            string
	Gray                     string
	NeutralRed               string
	NeutralOrange            string
	NeutralYellow            string
	NeutralGreen             string
	NeutralBlue              string
	DarkAqua                 string

	CaptionFg [6]string
	CaptionBg [6]string
}

func NewGruvboxLight() GruvboxLight {
	return GruvboxLight{
		Bg0H:          "#f9f5d7",
		Bg0:           "#fbf1c7",
		Bg1:           "#ebdbb2",
		Bg2:           "#d5c4a1",
		Bg3:           "#bdae93",
		Fg0:           "#3c3836",
		Fg2:           "#504945",
		Fg3:           "#504945",
		Gray:          "#928374",
		NeutralRed:    "#9d0006",
		NeutralOrange: "#af3a03",
		NeutralYellow: "#9d6104",
		NeutralGreen:  "#79740e",
		NeutralBlue:   "#076678",
		DarkAqua:      "#427b58",

		CaptionFg: [6]string{
			"#e2d6c3",
			"#dedcc3",
			"#ebcec0",
			"#d0ded5",
			"#dedbd8",
			"#e2d7d2",
		},
		CaptionBg: [6]string{
			"#8b5b0f",
			"#6f6a0d",
			"#c44103",
			"#3f7554",
			"#6a5f55",
			"#8b5e4b",
		},
	}
}
