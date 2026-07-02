package palettes

// Nord holds the canonical Nord palette.
// Ref: https://www.nordtheme.com/docs/colors-and-palettes
type Nord struct {
	// Polar Night (4 dark shades)
	PolarNight0 string // nord0
	PolarNight1 string // nord1
	PolarNight2 string // nord2
	PolarNight3 string // nord3

	// Snow Storm (3 light shades)
	SnowStorm0 string // nord4
	SnowStorm1 string // nord5
	SnowStorm2 string // nord6

	// Frost (4 blues)
	Frost0 string // nord7 — frost teal
	Frost1 string // nord8 — frost cyan
	Frost2 string // nord9 — frost blue
	Frost3 string // nord10 — frost dark blue

	// Aurora (5 accents)
	Aurora0 string // nord11 — red
	Aurora1 string // nord12 — orange
	Aurora2 string // nord13 — yellow
	Aurora3 string // nord14 — green
	Aurora4 string // nord15 — purple

	// Caption pairs
	CaptionFg [6]string
	CaptionBg [6]string
}

func NewNord() Nord {
	return Nord{
		PolarNight0: "#2e3440",
		PolarNight1: "#3b4252",
		PolarNight2: "#434c5e",
		PolarNight3: "#4c566a",

		SnowStorm0: "#d8dee9",
		SnowStorm1: "#e5e9f0",
		SnowStorm2: "#eceff4",

		Frost0: "#8fbcbb",
		Frost1: "#88c0d0",
		Frost2: "#81a1c1",
		Frost3: "#5e81ac",

		Aurora0: "#bf616a",
		Aurora1: "#d08770",
		Aurora2: "#ebcb8b",
		Aurora3: "#a3be8c",
		Aurora4: "#b48ead",

		CaptionFg: [6]string{
			"#c9e3ea",
			"#d6e2cb",
			"#eac9bf",
			"#f4e3c3",
			"#aeb3bc",
			"#dca8ac",
		},
		CaptionBg: [6]string{
			"#293a3e",
			"#31392a",
			"#3e2922",
			"#473d2a",
			"#171a20",
			"#391d20",
		},
	}
}
