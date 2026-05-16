package theme

import "github.com/boolean-maybe/tiki/theme/palettes"

// LoadByName returns a fully populated *Theme for the requested theme name.
// Unknown names fall back to the dark theme.
func LoadByName(name string) *Theme {
	switch name {
	case "dark":
		return bindDark()
	case "light":
		return bindLight()
	case "dracula":
		return bindDracula()
	case "tokyo-night":
		return bindTokyoNight()
	case "gruvbox-dark":
		return bindGruvboxDark()
	case "catppuccin-mocha":
		return bindCatppuccinMocha()
	case "solarized-dark":
		return bindSolarizedDark()
	case "nord":
		return bindNord()
	case "monokai":
		return bindMonokai()
	case "one-dark":
		return bindOneDark()
	case "catppuccin-latte":
		return bindCatppuccinLatte()
	case "solarized-light":
		return bindSolarizedLight()
	case "gruvbox-light":
		return bindGruvboxLight()
	case "github-light":
		return bindGithubLight()
	default:
		return bindDark()
	}
}

// roleOf wraps a palette color string as a single-color Role.
func roleOf(s string) Role { return newColorRole(NewColorHex(s)) }

// pairOf builds a fg/bg PairRole from two palette color strings.
func pairOf(fg, bg string) PairRole { return newPairRole(roleOf(fg), roleOf(bg)) }

// captionPairs builds the 6-entry caption PairListRole from two parallel
// fg/bg arrays.
func captionPairs(fg, bg [6]string) PairListRole {
	pairs := make([]PairRole, 6)
	for i := 0; i < 6; i++ {
		pairs[i] = pairOf(fg[i], bg[i])
	}
	return newPairListRole(pairs)
}

func bindDark() *Theme {
	p := palettes.NewDark()
	return &Theme{
		textPrimary:   roleOf(p.Text),
		textSecondary: roleOf(p.SoftText),
		textMuted:     roleOf(p.Muted),
		textLabel:     roleOf(p.Accent),
		textValue:     roleOf(p.Value),
		textHint:      roleOf(p.Muted),

		borderFocus: roleOf(p.Highlight),
		borderIdle:  roleOf(p.SoftBorder),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.Selection),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.Highlight),
		accentAction: roleOf(p.AccentBlue),
		accentTag:    roleOf(p.Slate),

		statusDanger: roleOf(p.Danger),
		statusWarn:   roleOf(p.Warn),
		statusOk:     roleOf(p.Ok),

		statuslineMain:   pairOf(p.StatuslineText, p.StatuslineBorderBg),
		statuslineAccent: pairOf(p.StatuslineDarkBg, p.StatuslineAccent),
		statuslineInfo:   pairOf(p.Ok, p.StatuslineMidBg),
		statuslineError:  pairOf(p.Highlight, p.StatuslineMidBg),
		statuslineFill:   roleOf(p.StatuslineMidBg),

		depsEditorSurface: roleOf(p.StatuslineBorderBg),

		logoDot:    roleOf(p.LogoDot),
		logoShade:  roleOf(p.LogoShade),
		logoBorder: roleOf(p.LogoBorder),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.DeepSkyBlue), 0.2),
			roleOf(p.DeepSkyBlue),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.SoftBorder),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}

func bindLight() *Theme {
	p := palettes.NewLight()
	return &Theme{
		textPrimary:   roleOf(p.Text),
		textSecondary: roleOf(p.SoftText),
		textMuted:     roleOf(p.Muted),
		textLabel:     roleOf(p.Accent),
		textValue:     roleOf(p.Value),
		textHint:      roleOf(p.Muted),

		borderFocus: roleOf(p.Highlight),
		borderIdle:  roleOf(p.SoftBorder),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.Selection),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.Highlight),
		accentAction: roleOf(p.AccentBlue),
		accentTag:    roleOf(p.Slate),

		statusDanger: roleOf(p.Danger),
		statusWarn:   roleOf(p.Warn),
		statusOk:     roleOf(p.Ok),

		statuslineMain:   pairOf(p.StatuslineText, p.StatuslineBorderBg),
		statuslineAccent: pairOf(p.StatuslineDarkBg, p.StatuslineAccent),
		statuslineInfo:   pairOf(p.Ok, p.StatuslineMidBg),
		statuslineError:  pairOf(p.Highlight, p.StatuslineMidBg),
		statuslineFill:   roleOf(p.StatuslineMidBg),

		depsEditorSurface: roleOf(p.StatuslineBorderBg),

		logoDot:    roleOf(p.LogoDot),
		logoShade:  roleOf(p.LogoShade),
		logoBorder: roleOf(p.LogoBorder),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.DeepSkyBlue), 0.2),
			roleOf(p.DeepSkyBlue),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.SoftBorder),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}

func bindDracula() *Theme {
	p := palettes.NewDracula()
	return &Theme{
		textPrimary:   roleOf(p.Foreground),
		textSecondary: roleOf(p.SoftText),
		textMuted:     roleOf(p.Comment),
		textLabel:     roleOf(p.Green),
		textValue:     roleOf(p.Purple),
		textHint:      roleOf(p.Comment),

		borderFocus: roleOf(p.Pink),
		borderIdle:  roleOf(p.CurrentLine),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.CurrentLine),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.Pink),
		accentAction: roleOf(p.Cyan),
		accentTag:    roleOf(p.Comment),

		statusDanger: roleOf(p.Red),
		statusWarn:   roleOf(p.Orange),
		statusOk:     roleOf(p.Green),

		statuslineMain:   pairOf(p.Foreground, p.StatuslineBorderBg),
		statuslineAccent: pairOf(p.StatuslineDarkBg, p.StatuslineAccent),
		statuslineInfo:   pairOf(p.Green, p.StatuslineMidBg),
		statuslineError:  pairOf(p.Pink, p.StatuslineMidBg),
		statuslineFill:   roleOf(p.StatuslineMidBg),

		depsEditorSurface: roleOf(p.StatuslineBorderBg),

		logoDot:    roleOf(p.Cyan),
		logoShade:  roleOf(p.Purple),
		logoBorder: roleOf(p.CurrentLine),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.Cyan), 0.2),
			roleOf(p.Cyan),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.CurrentLine),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}

func bindNord() *Theme {
	p := palettes.NewNord()
	return &Theme{
		textPrimary:   roleOf(p.SnowStorm2),
		textSecondary: roleOf(p.SnowStorm0),
		textMuted:     roleOf(p.PolarNight3),
		textLabel:     roleOf(p.Aurora3),
		textValue:     roleOf(p.Frost2),
		textHint:      roleOf(p.PolarNight3),

		borderFocus: roleOf(p.Aurora2),
		borderIdle:  roleOf(p.PolarNight2),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.PolarNight2),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.Aurora2),
		accentAction: roleOf(p.Frost1),
		accentTag:    roleOf(p.PolarNight3),

		statusDanger: roleOf(p.Aurora0),
		statusWarn:   roleOf(p.Aurora1),
		statusOk:     roleOf(p.Aurora3),

		statuslineMain:   pairOf(p.SnowStorm0, p.PolarNight2),
		statuslineAccent: pairOf(p.PolarNight0, p.Frost3),
		statuslineInfo:   pairOf(p.Aurora3, p.PolarNight1),
		statuslineError:  pairOf(p.Aurora2, p.PolarNight1),
		statuslineFill:   roleOf(p.PolarNight1),

		depsEditorSurface: roleOf(p.PolarNight2),

		logoDot:    roleOf(p.Frost0),
		logoShade:  roleOf(p.Frost2),
		logoBorder: roleOf(p.PolarNight1),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.Frost1), 0.2),
			roleOf(p.Frost1),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.PolarNight2),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}

func bindGruvboxDark() *Theme {
	p := palettes.NewGruvboxDark()
	return &Theme{
		textPrimary:   roleOf(p.Fg0),
		textSecondary: roleOf(p.Fg2),
		textMuted:     roleOf(p.Gray),
		textLabel:     roleOf(p.NeutralGreen),
		textValue:     roleOf(p.NeutralBlue),
		textHint:      roleOf(p.Gray),

		borderFocus: roleOf(p.NeutralYellow),
		borderIdle:  roleOf(p.Bg2),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.Bg2),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.NeutralYellow),
		accentAction: roleOf(p.NeutralBlue),
		accentTag:    roleOf(p.Bg3),

		statusDanger: roleOf(p.NeutralRed),
		statusWarn:   roleOf(p.NeutralOrange),
		statusOk:     roleOf(p.NeutralGreen),

		statuslineMain:   pairOf(p.Fg0, p.Bg1),
		statuslineAccent: pairOf(p.Bg0H, p.DarkAqua),
		statuslineInfo:   pairOf(p.NeutralGreen, p.Bg0),
		statuslineError:  pairOf(p.NeutralYellow, p.Bg0),
		statuslineFill:   roleOf(p.Bg0),

		depsEditorSurface: roleOf(p.Bg1),

		logoDot:    roleOf(p.NeutralAqua),
		logoShade:  roleOf(p.NeutralBlue),
		logoBorder: roleOf(p.Bg1),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.NeutralBlue), 0.2),
			roleOf(p.NeutralBlue),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.Bg2),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}

func bindGruvboxLight() *Theme {
	p := palettes.NewGruvboxLight()
	return &Theme{
		textPrimary:   roleOf(p.Fg0),
		textSecondary: roleOf(p.Fg2),
		textMuted:     roleOf(p.Gray),
		textLabel:     roleOf(p.NeutralGreen),
		textValue:     roleOf(p.NeutralBlue),
		textHint:      roleOf(p.Gray),

		borderFocus: roleOf(p.NeutralYellow),
		borderIdle:  roleOf(p.Bg2),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.Bg2),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.NeutralYellow),
		accentAction: roleOf(p.NeutralBlue),
		accentTag:    roleOf(p.Bg3),

		statusDanger: roleOf(p.NeutralRed),
		statusWarn:   roleOf(p.NeutralOrange),
		statusOk:     roleOf(p.NeutralGreen),

		statuslineMain:   pairOf(p.Fg0, p.Bg2),
		statuslineAccent: pairOf(p.Bg0, p.DarkAqua),
		statuslineInfo:   pairOf(p.NeutralGreen, p.Bg1),
		statuslineError:  pairOf(p.NeutralYellow, p.Bg1),
		statuslineFill:   roleOf(p.Bg1),

		depsEditorSurface: roleOf(p.Bg2),

		logoDot:    roleOf(p.DarkAqua),
		logoShade:  roleOf(p.NeutralBlue),
		logoBorder: roleOf(p.Bg1),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.NeutralBlue), 0.2),
			roleOf(p.NeutralBlue),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.Bg2),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}

func bindTokyoNight() *Theme {
	p := palettes.NewTokyoNight()
	return &Theme{
		textPrimary:   roleOf(p.Foreground),
		textSecondary: roleOf(p.SoftText),
		textMuted:     roleOf(p.Comment),
		textLabel:     roleOf(p.Green),
		textValue:     roleOf(p.Blue),
		textHint:      roleOf(p.Comment),

		borderFocus: roleOf(p.Yellow),
		borderIdle:  roleOf(p.Border),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.Selection),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.Yellow),
		accentAction: roleOf(p.Blue),
		accentTag:    roleOf(p.Comment),

		statusDanger: roleOf(p.Red),
		statusWarn:   roleOf(p.Orange),
		statusOk:     roleOf(p.Green),

		statuslineMain:   pairOf(p.Foreground, p.StatuslineBorderBg),
		statuslineAccent: pairOf(p.StatuslineDarkBg, p.StatuslineAccent),
		statuslineInfo:   pairOf(p.Green, p.StatuslineMidBg),
		statuslineError:  pairOf(p.Yellow, p.StatuslineMidBg),
		statuslineFill:   roleOf(p.StatuslineMidBg),

		depsEditorSurface: roleOf(p.StatuslineBorderBg),

		logoDot:    roleOf(p.Cyan),
		logoShade:  roleOf(p.Blue),
		logoBorder: roleOf(p.Border),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.Cyan), 0.2),
			roleOf(p.Cyan),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.Border),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}

func bindMonokai() *Theme {
	p := palettes.NewMonokai()
	return &Theme{
		textPrimary:   roleOf(p.Foreground),
		textSecondary: roleOf(p.SoftText),
		textMuted:     roleOf(p.Comment),
		textLabel:     roleOf(p.Green),
		textValue:     roleOf(p.Cyan),
		textHint:      roleOf(p.Comment),

		borderFocus: roleOf(p.Yellow),
		borderIdle:  roleOf(p.Selection),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.Selection),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.Yellow),
		accentAction: roleOf(p.Cyan),
		accentTag:    roleOf(p.Comment),

		statusDanger: roleOf(p.Pink),
		statusWarn:   roleOf(p.Orange),
		statusOk:     roleOf(p.Green),

		statuslineMain:   pairOf(p.Foreground, p.StatuslineBorderBg),
		statuslineAccent: pairOf(p.StatuslineDarkBg, p.Cyan),
		statuslineInfo:   pairOf(p.Green, p.Background),
		statuslineError:  pairOf(p.Yellow, p.Background),
		statuslineFill:   roleOf(p.Background),

		depsEditorSurface: roleOf(p.StatuslineBorderBg),

		logoDot:    roleOf(p.Green),
		logoShade:  roleOf(p.Cyan),
		logoBorder: roleOf(p.StatuslineBorderBg),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.Cyan), 0.2),
			roleOf(p.Cyan),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.Selection),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}

func bindOneDark() *Theme {
	p := palettes.NewOneDark()
	return &Theme{
		textPrimary:   roleOf(p.Foreground),
		textSecondary: roleOf(p.SoftText),
		textMuted:     roleOf(p.Comment),
		textLabel:     roleOf(p.Green),
		textValue:     roleOf(p.Blue),
		textHint:      roleOf(p.Comment),

		borderFocus: roleOf(p.Yellow),
		borderIdle:  roleOf(p.Selection),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.Selection),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.Yellow),
		accentAction: roleOf(p.Blue),
		accentTag:    roleOf(p.Comment),

		statusDanger: roleOf(p.Red),
		statusWarn:   roleOf(p.Orange),
		statusOk:     roleOf(p.Green),

		statuslineMain:   pairOf(p.Foreground, p.StatuslineBorderBg),
		statuslineAccent: pairOf(p.StatuslineDarkBg, p.Blue),
		statuslineInfo:   pairOf(p.Green, p.Background),
		statuslineError:  pairOf(p.Yellow, p.Background),
		statuslineFill:   roleOf(p.Background),

		depsEditorSurface: roleOf(p.StatuslineBorderBg),

		logoDot:    roleOf(p.Cyan),
		logoShade:  roleOf(p.Blue),
		logoBorder: roleOf(p.StatuslineBorderBg),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.Blue), 0.2),
			roleOf(p.Blue),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.Selection),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}

func bindCatppuccinMocha() *Theme {
	p := palettes.NewCatppuccinMocha()
	return &Theme{
		textPrimary:   roleOf(p.Text),
		textSecondary: roleOf(p.Subtext1),
		textMuted:     roleOf(p.Overlay0),
		textLabel:     roleOf(p.Green),
		textValue:     roleOf(p.Blue),
		textHint:      roleOf(p.Overlay0),

		borderFocus: roleOf(p.Yellow),
		borderIdle:  roleOf(p.Surface1),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.Surface1),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.Yellow),
		accentAction: roleOf(p.Blue),
		accentTag:    roleOf(p.Surface2),

		statusDanger: roleOf(p.Red),
		statusWarn:   roleOf(p.Peach),
		statusOk:     roleOf(p.Green),

		statuslineMain:   pairOf(p.Text, p.Surface0),
		statuslineAccent: pairOf(p.Crust, p.Blue),
		statuslineInfo:   pairOf(p.Green, p.Base),
		statuslineError:  pairOf(p.Yellow, p.Base),
		statuslineFill:   roleOf(p.Base),

		depsEditorSurface: roleOf(p.Surface0),

		logoDot:    roleOf(p.Teal),
		logoShade:  roleOf(p.Blue),
		logoBorder: roleOf(p.Surface0),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.Sky), 0.2),
			roleOf(p.Sky),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.Surface1),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}

func bindCatppuccinLatte() *Theme {
	p := palettes.NewCatppuccinLatte()
	return &Theme{
		textPrimary:   roleOf(p.Text),
		textSecondary: roleOf(p.Subtext1),
		textMuted:     roleOf(p.Overlay0),
		textLabel:     roleOf(p.Green),
		textValue:     roleOf(p.Blue),
		textHint:      roleOf(p.Overlay0),

		borderFocus: roleOf(p.Yellow),
		borderIdle:  roleOf(p.Surface0),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.Surface0),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.Yellow),
		accentAction: roleOf(p.Blue),
		accentTag:    roleOf(p.Surface2),

		statusDanger: roleOf(p.Red),
		statusWarn:   roleOf(p.Peach),
		statusOk:     roleOf(p.Green),

		statuslineMain:   pairOf(p.Text, p.Crust),
		statuslineAccent: pairOf(p.Base, p.Blue),
		statuslineInfo:   pairOf(p.Green, p.Mantle),
		statuslineError:  pairOf(p.Yellow, p.Mantle),
		statuslineFill:   roleOf(p.Mantle),

		depsEditorSurface: roleOf(p.Crust),

		logoDot:    roleOf(p.Teal),
		logoShade:  roleOf(p.Blue),
		logoBorder: roleOf(p.Surface1),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.Sky), 0.2),
			roleOf(p.Sky),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.Surface0),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}

func bindSolarizedDark() *Theme {
	p := palettes.NewSolarizedDark()
	return &Theme{
		textPrimary:   roleOf(p.Base0),
		textSecondary: roleOf(p.Base1),
		textMuted:     roleOf(p.Base01),
		textLabel:     roleOf(p.Green),
		textValue:     roleOf(p.Blue),
		textHint:      roleOf(p.Base01),

		borderFocus: roleOf(p.Yellow),
		borderIdle:  roleOf(p.Base02),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.Base02),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.Yellow),
		accentAction: roleOf(p.Blue),
		accentTag:    roleOf(p.Base01),

		statusDanger: roleOf(p.Red),
		statusWarn:   roleOf(p.Orange),
		statusOk:     roleOf(p.Green),

		statuslineMain:   pairOf(p.Base0, p.Base02),
		statuslineAccent: pairOf(p.Base03, p.Blue),
		statuslineInfo:   pairOf(p.Green, p.Base02),
		statuslineError:  pairOf(p.Yellow, p.Base02),
		statuslineFill:   roleOf(p.Base02),

		depsEditorSurface: roleOf(p.Base02),

		logoDot:    roleOf(p.Cyan),
		logoShade:  roleOf(p.Blue),
		logoBorder: roleOf(p.Base02),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.Blue), 0.2),
			roleOf(p.Blue),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.Base02),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}

func bindSolarizedLight() *Theme {
	p := palettes.NewSolarizedLight()
	return &Theme{
		textPrimary:   roleOf(p.Base00),
		textSecondary: roleOf(p.Base01),
		textMuted:     roleOf(p.Base1),
		textLabel:     roleOf(p.Green),
		textValue:     roleOf(p.Blue),
		textHint:      roleOf(p.Base1),

		borderFocus: roleOf(p.Yellow),
		borderIdle:  roleOf(p.Base2),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.Base2),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.Yellow),
		accentAction: roleOf(p.Blue),
		accentTag:    roleOf(p.Base1),

		statusDanger: roleOf(p.Red),
		statusWarn:   roleOf(p.Orange),
		statusOk:     roleOf(p.Green),

		statuslineMain:   pairOf(p.Base00, p.Base2),
		statuslineAccent: pairOf(p.Base3, p.Blue),
		statuslineInfo:   pairOf(p.Green, p.Base2),
		statuslineError:  pairOf(p.Yellow, p.Base2),
		statuslineFill:   roleOf(p.Base2),

		depsEditorSurface: roleOf(p.Base2),

		logoDot:    roleOf(p.Cyan),
		logoShade:  roleOf(p.Blue),
		logoBorder: roleOf(p.Base2),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.Blue), 0.2),
			roleOf(p.Blue),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.Base2),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}

func bindGithubLight() *Theme {
	p := palettes.NewGithubLight()
	return &Theme{
		textPrimary:   roleOf(p.FgDefault),
		textSecondary: roleOf(p.FgSubtle),
		textMuted:     roleOf(p.FgMuted),
		textLabel:     roleOf(p.Green),
		textValue:     roleOf(p.BlueAccent),
		textHint:      roleOf(p.FgMuted),

		borderFocus: roleOf(p.BlueAccentDark),
		borderIdle:  roleOf(p.BorderDefault),

		surfaceTransparent: newColorRole(DefaultColor()),
		surfaceSelection:   roleOf(p.Selection),
		surfaceCanvas:      newColorRole(DefaultColor()),

		highlight:    roleOf(p.BlueAccentDark),
		accentAction: roleOf(p.BlueAccent),
		accentTag:    roleOf(p.Slate),

		statusDanger: roleOf(p.Red),
		statusWarn:   roleOf(p.Orange),
		statusOk:     roleOf(p.Green),

		statuslineMain:   pairOf(p.FgDefault, p.BorderMuted),
		statuslineAccent: pairOf(p.CanvasDefault, p.BlueAccent),
		statuslineInfo:   pairOf(p.Green, p.CanvasSubtle),
		statuslineError:  pairOf(p.BlueAccentDark, p.CanvasSubtle),
		statuslineFill:   roleOf(p.CanvasSubtle),

		depsEditorSurface: roleOf(p.BorderMuted),

		logoDot:    roleOf(p.BlueAccent),
		logoShade:  roleOf(p.BlueAccentDark),
		logoBorder: roleOf(p.BorderDefault),

		tikiIDGradient: newGradientRole(
			gradientFromColor(NewColorHex(p.BlueAccent), 0.2),
			roleOf(p.BlueAccent),
		),
		captionFallbackGradient: newGradientRole(
			Gradient{Start: p.CaptionFallbackStart, End: p.CaptionFallbackEnd},
			roleOf(p.BorderDefault),
		),

		pluginCaptions: captionPairs(p.CaptionFg, p.CaptionBg),
	}
}
