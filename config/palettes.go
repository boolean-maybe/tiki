package config

// Palette constructors for all built-in and named themes.
// Each function returns a Palette with canonical hex values from the theme's specification.

import (
	"github.com/gdamore/tcell/v2"
)

// DarkPalette returns the color palette for dark backgrounds.
func DarkPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#ffff00"),
		TextColor:        NewColorHex("#ffffff"),
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#686868"),
		SoftBorderColor:  NewColorHex("#686868"),
		SoftTextColor:    NewColorHex("#b4b4b4"),
		AccentColor:      NewColor(tcell.ColorGreen),
		ValueColor:       NewColorHex("#8c92ac"),
		InfoLabelColor:   NewColorHex("#ffa500"),

		SelectionBgColor: NewColorHex("#3a5f8a"),

		AccentBlue: NewColorHex("#5fafff"),
		SlateColor: NewColorHex("#5f6982"),

		LogoDotColor:    NewColorHex("#40e0d0"),
		LogoShadeColor:  NewColorHex("#4682b4"),
		LogoBorderColor: NewColorHex("#324664"),

		CaptionFallbackGradient: Gradient{
			Start: [3]int{25, 25, 112},
			End:   [3]int{65, 105, 225},
		},
		DeepSkyBlue: NewColorRGB(0, 191, 255),
		DeepPurple:  NewColorRGB(134, 90, 214),

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#2e3440"),
		StatuslineMidBg:    NewColorHex("#3b4252"),
		StatuslineBorderBg: NewColorHex("#434c5e"),
		StatuslineText:     NewColorHex("#d8dee9"),
		StatuslineAccent:   NewColorHex("#5e81ac"),
		StatuslineOk:       NewColorHex("#a3be8c"),
	}
}

// LightPalette returns the color palette for light backgrounds.
func LightPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#0055dd"),
		TextColor:        NewColor(tcell.ColorBlack),
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#808080"),
		SoftBorderColor:  NewColorHex("#b0b8c8"),
		SoftTextColor:    NewColorHex("#404040"),
		AccentColor:      NewColorHex("#006400"),
		ValueColor:       NewColorHex("#4a4e6a"),
		InfoLabelColor:   NewColorHex("#b85c00"),

		SelectionBgColor: NewColorHex("#b8d4f0"),

		AccentBlue: NewColorHex("#0060c0"),
		SlateColor: NewColorHex("#7080a0"),

		LogoDotColor:    NewColorHex("#20a090"),
		LogoShadeColor:  NewColorHex("#3060a0"),
		LogoBorderColor: NewColorHex("#6080a0"),

		CaptionFallbackGradient: Gradient{
			Start: [3]int{100, 140, 200},
			End:   [3]int{60, 100, 180},
		},
		DeepSkyBlue: NewColorRGB(0, 100, 180),
		DeepPurple:  NewColorRGB(90, 50, 160),

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#eceff4"),
		StatuslineMidBg:    NewColorHex("#e5e9f0"),
		StatuslineBorderBg: NewColorHex("#d8dee9"),
		StatuslineText:     NewColorHex("#2e3440"),
		StatuslineAccent:   NewColorHex("#5e81ac"),
		StatuslineOk:       NewColorHex("#4c7a5a"),
	}
}

// DraculaPalette returns the Dracula theme palette.
// Ref: https://draculatheme.com/contribute
func DraculaPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#ff79c6"), // pink
		TextColor:        NewColorHex("#f8f8f2"), // foreground
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#6272a4"), // comment
		SoftBorderColor:  NewColorHex("#44475a"), // current line
		SoftTextColor:    NewColorHex("#bfbfbf"),
		AccentColor:      NewColorHex("#50fa7b"), // green
		ValueColor:       NewColorHex("#bd93f9"), // purple
		InfoLabelColor:   NewColorHex("#ffb86c"), // orange

		SelectionBgColor: NewColorHex("#44475a"),

		AccentBlue: NewColorHex("#8be9fd"), // cyan
		SlateColor: NewColorHex("#6272a4"), // comment

		LogoDotColor:    NewColorHex("#8be9fd"),
		LogoShadeColor:  NewColorHex("#bd93f9"),
		LogoBorderColor: NewColorHex("#44475a"),

		CaptionFallbackGradient: Gradient{
			Start: [3]int{40, 42, 54},
			End:   [3]int{68, 71, 90},
		},
		DeepSkyBlue: NewColorHex("#8be9fd"),
		DeepPurple:  NewColorHex("#bd93f9"),

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#21222c"),
		StatuslineMidBg:    NewColorHex("#282a36"),
		StatuslineBorderBg: NewColorHex("#44475a"),
		StatuslineText:     NewColorHex("#f8f8f2"),
		StatuslineAccent:   NewColorHex("#bd93f9"),
		StatuslineOk:       NewColorHex("#50fa7b"),
	}
}

// TokyoNightPalette returns the Tokyo Night theme palette.
// Ref: https://github.com/folke/tokyonight.nvim
func TokyoNightPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#e0af68"), // yellow
		TextColor:        NewColorHex("#c0caf5"), // foreground
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#565f89"), // comment
		SoftBorderColor:  NewColorHex("#3b4261"),
		SoftTextColor:    NewColorHex("#a9b1d6"),
		AccentColor:      NewColorHex("#9ece6a"), // green
		ValueColor:       NewColorHex("#7aa2f7"), // blue
		InfoLabelColor:   NewColorHex("#ff9e64"), // orange

		SelectionBgColor: NewColorHex("#283457"),

		AccentBlue: NewColorHex("#7aa2f7"),
		SlateColor: NewColorHex("#565f89"),

		LogoDotColor:    NewColorHex("#7dcfff"),
		LogoShadeColor:  NewColorHex("#7aa2f7"),
		LogoBorderColor: NewColorHex("#3b4261"),

		CaptionFallbackGradient: Gradient{
			Start: [3]int{26, 27, 38},
			End:   [3]int{59, 66, 97},
		},
		DeepSkyBlue: NewColorHex("#7dcfff"),
		DeepPurple:  NewColorHex("#bb9af7"),

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#16161e"),
		StatuslineMidBg:    NewColorHex("#1a1b26"),
		StatuslineBorderBg: NewColorHex("#24283b"),
		StatuslineText:     NewColorHex("#c0caf5"),
		StatuslineAccent:   NewColorHex("#7aa2f7"),
		StatuslineOk:       NewColorHex("#9ece6a"),
	}
}

// GruvboxDarkPalette returns the Gruvbox Dark theme palette.
// Ref: https://github.com/morhetz/gruvbox
func GruvboxDarkPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#fabd2f"), // yellow
		TextColor:        NewColorHex("#ebdbb2"), // fg
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#928374"), // gray
		SoftBorderColor:  NewColorHex("#504945"), // bg2
		SoftTextColor:    NewColorHex("#bdae93"), // fg3
		AccentColor:      NewColorHex("#b8bb26"), // green
		ValueColor:       NewColorHex("#83a598"), // blue
		InfoLabelColor:   NewColorHex("#fe8019"), // orange

		SelectionBgColor: NewColorHex("#504945"),

		AccentBlue: NewColorHex("#83a598"),
		SlateColor: NewColorHex("#665c54"), // bg3

		LogoDotColor:    NewColorHex("#8ec07c"), // aqua
		LogoShadeColor:  NewColorHex("#83a598"),
		LogoBorderColor: NewColorHex("#3c3836"), // bg1

		CaptionFallbackGradient: Gradient{
			Start: [3]int{40, 40, 40},
			End:   [3]int{80, 73, 69},
		},
		DeepSkyBlue: NewColorHex("#83a598"),
		DeepPurple:  NewColorHex("#d3869b"), // purple

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#1d2021"), // bg0_h
		StatuslineMidBg:    NewColorHex("#282828"), // bg0
		StatuslineBorderBg: NewColorHex("#3c3836"), // bg1
		StatuslineText:     NewColorHex("#ebdbb2"),
		StatuslineAccent:   NewColorHex("#689d6a"), // dark aqua
		StatuslineOk:       NewColorHex("#b8bb26"),
	}
}

// CatppuccinMochaPalette returns the Catppuccin Mocha theme palette.
// Ref: https://catppuccin.com/palette
func CatppuccinMochaPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#f9e2af"), // yellow
		TextColor:        NewColorHex("#cdd6f4"), // text
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#6c7086"), // overlay0
		SoftBorderColor:  NewColorHex("#45475a"), // surface0
		SoftTextColor:    NewColorHex("#bac2de"), // subtext1
		AccentColor:      NewColorHex("#a6e3a1"), // green
		ValueColor:       NewColorHex("#89b4fa"), // blue
		InfoLabelColor:   NewColorHex("#fab387"), // peach

		SelectionBgColor: NewColorHex("#45475a"),

		AccentBlue: NewColorHex("#89b4fa"),
		SlateColor: NewColorHex("#585b70"), // surface2

		LogoDotColor:    NewColorHex("#94e2d5"), // teal
		LogoShadeColor:  NewColorHex("#89b4fa"),
		LogoBorderColor: NewColorHex("#313244"), // surface0

		CaptionFallbackGradient: Gradient{
			Start: [3]int{30, 30, 46},
			End:   [3]int{69, 71, 90},
		},
		DeepSkyBlue: NewColorHex("#89dceb"), // sky
		DeepPurple:  NewColorHex("#cba6f7"), // mauve

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#11111b"), // crust
		StatuslineMidBg:    NewColorHex("#1e1e2e"), // base
		StatuslineBorderBg: NewColorHex("#313244"), // surface0
		StatuslineText:     NewColorHex("#cdd6f4"),
		StatuslineAccent:   NewColorHex("#89b4fa"),
		StatuslineOk:       NewColorHex("#a6e3a1"),
	}
}

// SolarizedDarkPalette returns the Solarized Dark theme palette.
// Ref: https://ethanschoonover.com/solarized/
func SolarizedDarkPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#b58900"), // yellow
		TextColor:        NewColorHex("#839496"), // base0
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#586e75"), // base01
		SoftBorderColor:  NewColorHex("#073642"), // base02
		SoftTextColor:    NewColorHex("#93a1a1"), // base1
		AccentColor:      NewColorHex("#859900"), // green
		ValueColor:       NewColorHex("#268bd2"), // blue
		InfoLabelColor:   NewColorHex("#cb4b16"), // orange

		SelectionBgColor: NewColorHex("#073642"),

		AccentBlue: NewColorHex("#268bd2"),
		SlateColor: NewColorHex("#586e75"),

		LogoDotColor:    NewColorHex("#2aa198"), // cyan
		LogoShadeColor:  NewColorHex("#268bd2"),
		LogoBorderColor: NewColorHex("#073642"),

		CaptionFallbackGradient: Gradient{
			Start: [3]int{0, 43, 54},
			End:   [3]int{7, 54, 66},
		},
		DeepSkyBlue: NewColorHex("#268bd2"),
		DeepPurple:  NewColorHex("#6c71c4"), // violet

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#002b36"), // base03
		StatuslineMidBg:    NewColorHex("#073642"), // base02
		StatuslineBorderBg: NewColorHex("#073642"),
		StatuslineText:     NewColorHex("#839496"),
		StatuslineAccent:   NewColorHex("#268bd2"),
		StatuslineOk:       NewColorHex("#859900"),
	}
}

// NordPalette returns the Nord theme palette.
// Ref: https://www.nordtheme.com/docs/colors-and-palettes
func NordPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#ebcb8b"), // nord13 — yellow
		TextColor:        NewColorHex("#eceff4"), // nord6 — snow storm
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#4c566a"), // nord3
		SoftBorderColor:  NewColorHex("#434c5e"), // nord2
		SoftTextColor:    NewColorHex("#d8dee9"), // nord4
		AccentColor:      NewColorHex("#a3be8c"), // nord14 — green
		ValueColor:       NewColorHex("#81a1c1"), // nord9 — blue
		InfoLabelColor:   NewColorHex("#d08770"), // nord12 — orange

		SelectionBgColor: NewColorHex("#434c5e"),

		AccentBlue: NewColorHex("#88c0d0"), // nord8 — frost cyan
		SlateColor: NewColorHex("#4c566a"),

		LogoDotColor:    NewColorHex("#8fbcbb"), // nord7 — frost teal
		LogoShadeColor:  NewColorHex("#81a1c1"),
		LogoBorderColor: NewColorHex("#3b4252"), // nord1

		CaptionFallbackGradient: Gradient{
			Start: [3]int{46, 52, 64},
			End:   [3]int{59, 66, 82},
		},
		DeepSkyBlue: NewColorHex("#88c0d0"),
		DeepPurple:  NewColorHex("#b48ead"), // nord15 — purple

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#2e3440"), // nord0
		StatuslineMidBg:    NewColorHex("#3b4252"), // nord1
		StatuslineBorderBg: NewColorHex("#434c5e"), // nord2
		StatuslineText:     NewColorHex("#d8dee9"), // nord4
		StatuslineAccent:   NewColorHex("#5e81ac"), // nord10
		StatuslineOk:       NewColorHex("#a3be8c"),
	}
}

// MonokaiPalette returns the Monokai theme palette.
// Ref: https://monokai.pro/
func MonokaiPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#e6db74"), // yellow
		TextColor:        NewColorHex("#f8f8f2"), // foreground
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#75715e"), // comment
		SoftBorderColor:  NewColorHex("#49483e"),
		SoftTextColor:    NewColorHex("#cfcfc2"),
		AccentColor:      NewColorHex("#a6e22e"), // green
		ValueColor:       NewColorHex("#66d9ef"), // cyan
		InfoLabelColor:   NewColorHex("#fd971f"), // orange

		SelectionBgColor: NewColorHex("#49483e"),

		AccentBlue: NewColorHex("#66d9ef"),
		SlateColor: NewColorHex("#75715e"),

		LogoDotColor:    NewColorHex("#a6e22e"),
		LogoShadeColor:  NewColorHex("#66d9ef"),
		LogoBorderColor: NewColorHex("#3e3d32"),

		CaptionFallbackGradient: Gradient{
			Start: [3]int{39, 40, 34},
			End:   [3]int{73, 72, 62},
		},
		DeepSkyBlue: NewColorHex("#66d9ef"),
		DeepPurple:  NewColorHex("#ae81ff"), // purple

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#1e1f1c"),
		StatuslineMidBg:    NewColorHex("#272822"), // bg
		StatuslineBorderBg: NewColorHex("#3e3d32"),
		StatuslineText:     NewColorHex("#f8f8f2"),
		StatuslineAccent:   NewColorHex("#66d9ef"),
		StatuslineOk:       NewColorHex("#a6e22e"),
	}
}

// OneDarkPalette returns the Atom One Dark theme palette.
// Ref: https://github.com/Binaryify/OneDark-Pro
func OneDarkPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#e5c07b"), // yellow
		TextColor:        NewColorHex("#abb2bf"), // foreground
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#5c6370"), // comment
		SoftBorderColor:  NewColorHex("#3e4452"),
		SoftTextColor:    NewColorHex("#9da5b4"),
		AccentColor:      NewColorHex("#98c379"), // green
		ValueColor:       NewColorHex("#61afef"), // blue
		InfoLabelColor:   NewColorHex("#d19a66"), // orange

		SelectionBgColor: NewColorHex("#3e4452"),

		AccentBlue: NewColorHex("#61afef"),
		SlateColor: NewColorHex("#5c6370"),

		LogoDotColor:    NewColorHex("#56b6c2"), // cyan
		LogoShadeColor:  NewColorHex("#61afef"),
		LogoBorderColor: NewColorHex("#3b4048"),

		CaptionFallbackGradient: Gradient{
			Start: [3]int{40, 44, 52},
			End:   [3]int{62, 68, 82},
		},
		DeepSkyBlue: NewColorHex("#61afef"),
		DeepPurple:  NewColorHex("#c678dd"), // purple

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#21252b"),
		StatuslineMidBg:    NewColorHex("#282c34"), // bg
		StatuslineBorderBg: NewColorHex("#3b4048"),
		StatuslineText:     NewColorHex("#abb2bf"),
		StatuslineAccent:   NewColorHex("#61afef"),
		StatuslineOk:       NewColorHex("#98c379"),
	}
}

// --- Light themes ---

// CatppuccinLattePalette returns the Catppuccin Latte (light) theme palette.
// Ref: https://catppuccin.com/palette
func CatppuccinLattePalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#df8e1d"), // yellow
		TextColor:        NewColorHex("#4c4f69"), // text
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#9ca0b0"), // overlay0
		SoftBorderColor:  NewColorHex("#ccd0da"), // surface0
		SoftTextColor:    NewColorHex("#5c5f77"), // subtext1
		AccentColor:      NewColorHex("#40a02b"), // green
		ValueColor:       NewColorHex("#1e66f5"), // blue
		InfoLabelColor:   NewColorHex("#fe640b"), // peach

		SelectionBgColor: NewColorHex("#ccd0da"),

		AccentBlue: NewColorHex("#1e66f5"),
		SlateColor: NewColorHex("#acb0be"), // surface2

		LogoDotColor:    NewColorHex("#179299"), // teal
		LogoShadeColor:  NewColorHex("#1e66f5"),
		LogoBorderColor: NewColorHex("#bcc0cc"), // surface1

		CaptionFallbackGradient: Gradient{
			Start: [3]int{239, 241, 245},
			End:   [3]int{204, 208, 218},
		},
		DeepSkyBlue: NewColorHex("#04a5e5"), // sky
		DeepPurple:  NewColorHex("#8839ef"), // mauve

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#eff1f5"), // base
		StatuslineMidBg:    NewColorHex("#e6e9ef"), // mantle
		StatuslineBorderBg: NewColorHex("#dce0e8"), // crust
		StatuslineText:     NewColorHex("#4c4f69"),
		StatuslineAccent:   NewColorHex("#1e66f5"),
		StatuslineOk:       NewColorHex("#40a02b"),
	}
}

// SolarizedLightPalette returns the Solarized Light theme palette.
// Ref: https://ethanschoonover.com/solarized/
func SolarizedLightPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#b58900"), // yellow (same accent colors as dark)
		TextColor:        NewColorHex("#657b83"), // base00
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#93a1a1"), // base1
		SoftBorderColor:  NewColorHex("#eee8d5"), // base2
		SoftTextColor:    NewColorHex("#586e75"), // base01
		AccentColor:      NewColorHex("#859900"), // green
		ValueColor:       NewColorHex("#268bd2"), // blue
		InfoLabelColor:   NewColorHex("#cb4b16"), // orange

		SelectionBgColor: NewColorHex("#eee8d5"),

		AccentBlue: NewColorHex("#268bd2"),
		SlateColor: NewColorHex("#93a1a1"),

		LogoDotColor:    NewColorHex("#2aa198"), // cyan
		LogoShadeColor:  NewColorHex("#268bd2"),
		LogoBorderColor: NewColorHex("#eee8d5"),

		CaptionFallbackGradient: Gradient{
			Start: [3]int{253, 246, 227},
			End:   [3]int{238, 232, 213},
		},
		DeepSkyBlue: NewColorHex("#268bd2"),
		DeepPurple:  NewColorHex("#6c71c4"), // violet

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#fdf6e3"), // base3
		StatuslineMidBg:    NewColorHex("#eee8d5"), // base2
		StatuslineBorderBg: NewColorHex("#eee8d5"),
		StatuslineText:     NewColorHex("#657b83"),
		StatuslineAccent:   NewColorHex("#268bd2"),
		StatuslineOk:       NewColorHex("#859900"),
	}
}

// GruvboxLightPalette returns the Gruvbox Light theme palette.
// Ref: https://github.com/morhetz/gruvbox
func GruvboxLightPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#b57614"), // dark yellow
		TextColor:        NewColorHex("#3c3836"), // fg (dark0_hard)
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#928374"), // gray
		SoftBorderColor:  NewColorHex("#d5c4a1"), // bg2
		SoftTextColor:    NewColorHex("#504945"), // fg3 (dark2)
		AccentColor:      NewColorHex("#79740e"), // dark green
		ValueColor:       NewColorHex("#076678"), // dark blue
		InfoLabelColor:   NewColorHex("#af3a03"), // dark orange

		SelectionBgColor: NewColorHex("#d5c4a1"),

		AccentBlue: NewColorHex("#076678"),
		SlateColor: NewColorHex("#bdae93"), // bg3

		LogoDotColor:    NewColorHex("#427b58"), // dark aqua
		LogoShadeColor:  NewColorHex("#076678"),
		LogoBorderColor: NewColorHex("#ebdbb2"), // bg1

		CaptionFallbackGradient: Gradient{
			Start: [3]int{251, 241, 199},
			End:   [3]int{235, 219, 178},
		},
		DeepSkyBlue: NewColorHex("#076678"),
		DeepPurple:  NewColorHex("#8f3f71"), // dark purple

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#fbf1c7"), // bg0
		StatuslineMidBg:    NewColorHex("#ebdbb2"), // bg1
		StatuslineBorderBg: NewColorHex("#d5c4a1"), // bg2
		StatuslineText:     NewColorHex("#3c3836"),
		StatuslineAccent:   NewColorHex("#427b58"),
		StatuslineOk:       NewColorHex("#79740e"),
	}
}

// GithubLightPalette returns the GitHub Light theme palette.
// Ref: https://github.com/primer/github-vscode-theme
func GithubLightPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#0550ae"), // blue accent
		TextColor:        NewColorHex("#1f2328"), // fg.default
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#656d76"), // fg.muted
		SoftBorderColor:  NewColorHex("#d0d7de"), // border.default
		SoftTextColor:    NewColorHex("#424a53"),
		AccentColor:      NewColorHex("#116329"), // green
		ValueColor:       NewColorHex("#0969da"), // blue
		InfoLabelColor:   NewColorHex("#953800"), // orange

		SelectionBgColor: NewColorHex("#ddf4ff"),

		AccentBlue: NewColorHex("#0969da"),
		SlateColor: NewColorHex("#8c959f"),

		LogoDotColor:    NewColorHex("#0969da"),
		LogoShadeColor:  NewColorHex("#0550ae"),
		LogoBorderColor: NewColorHex("#d0d7de"),

		CaptionFallbackGradient: Gradient{
			Start: [3]int{255, 255, 255},
			End:   [3]int{246, 248, 250},
		},
		DeepSkyBlue: NewColorHex("#0969da"),
		DeepPurple:  NewColorHex("#8250df"), // purple

		ContentBackgroundColor: DefaultColor(),

		StatuslineDarkBg:   NewColorHex("#ffffff"),
		StatuslineMidBg:    NewColorHex("#f6f8fa"), // canvas.subtle
		StatuslineBorderBg: NewColorHex("#eaeef2"),
		StatuslineText:     NewColorHex("#1f2328"),
		StatuslineAccent:   NewColorHex("#0969da"),
		StatuslineOk:       NewColorHex("#116329"),
	}
}
