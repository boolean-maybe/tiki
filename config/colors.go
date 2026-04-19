package config

// Color and style definitions for the UI: gradients, unified Color values.

// Gradient defines a start and end RGB color for a gradient transition
type Gradient struct {
	Start [3]int // R, G, B (0-255)
	End   [3]int // R, G, B (0-255)
}

// CaptionColorPair holds the foreground and background colors for a plugin caption row.
type CaptionColorPair struct {
	Foreground Color
	Background Color
}

// ColorConfig holds all color and style definitions per view
type ColorConfig struct {
	// Caption colors
	CaptionFallbackGradient Gradient

	// Task box colors
	TaskBoxSelectedBorder       Color
	TaskBoxUnselectedBorder     Color
	TaskBoxUnselectedBackground Color
	TaskBoxIDColor              Gradient
	TaskBoxTitleColor           Color
	TaskBoxLabelColor           Color
	TaskBoxDescriptionColor     Color
	TaskBoxTagValueColor        Color
	TaskListSelectionFg         Color // selected row foreground
	TaskListSelectionBg         Color // selected row background
	TaskListStatusDoneColor     Color
	TaskListStatusPendingColor  Color

	// Task detail view colors
	TaskDetailIDColor           Gradient
	TaskDetailTitleText         Color
	TaskDetailLabelText         Color
	TaskDetailValueText         Color
	TaskDetailCommentAuthor     Color
	TaskDetailEditDimTextColor  Color
	TaskDetailEditDimLabelColor Color
	TaskDetailEditDimValueColor Color
	TaskDetailEditFocusMarker   Color
	TaskDetailEditFocusText     Color
	TaskDetailTagForeground     Color
	TaskDetailTagBackground     Color
	TaskDetailPlaceholderColor  Color

	// Content area colors (base canvas for editable/readable content)
	ContentBackgroundColor Color
	ContentTextColor       Color

	// Input box colors
	InputBoxLabelColor      Color
	InputBoxBackgroundColor Color
	InputBoxTextColor       Color

	// Input field colors (used in task detail edit mode)
	InputFieldBackgroundColor Color
	InputFieldTextColor       Color

	// Completion prompt colors
	CompletionHintColor Color

	// Burndown chart colors
	BurndownChartAxisColor     Color
	BurndownChartLabelColor    Color
	BurndownChartValueColor    Color
	BurndownChartBarColor      Color
	BurndownChartGradientFrom  Gradient
	BurndownChartGradientTo    Gradient
	BurndownHeaderGradientFrom Gradient // Header-specific chart gradient
	BurndownHeaderGradientTo   Gradient

	// Header view colors
	HeaderInfoLabel     Color
	HeaderInfoSeparator Color
	HeaderInfoDesc      Color
	HeaderKeyBinding    Color
	HeaderKeyText       Color

	// Points visual bar colors
	PointsFilledColor   Color
	PointsUnfilledColor Color

	// Header context help action colors
	HeaderActionGlobalKeyColor   Color
	HeaderActionGlobalLabelColor Color
	HeaderActionPluginKeyColor   Color
	HeaderActionPluginLabelColor Color
	HeaderActionViewKeyColor     Color
	HeaderActionViewLabelColor   Color

	// Plugin caption colors (auto-generated per theme)
	CaptionColors []CaptionColorPair

	// Plugin-specific colors
	DepsEditorBackground Color // muted slate for dependency editor caption

	// Fallback solid colors for gradient scenarios (used when UseGradients = false)
	FallbackTaskIDColor   Color // Deep Sky Blue (end of task ID gradient)
	FallbackBurndownColor Color // Purple (start of burndown gradient)

	// Logo colors (header art)
	LogoDotColor    Color // bright turquoise (● dots)
	LogoShadeColor  Color // medium blue (▓ shade)
	LogoBorderColor Color // dark blue (▒ border)

	// Statusline colors (bottom bar, powerline style)
	StatuslineBg       Color
	StatuslineFg       Color
	StatuslineAccentBg Color
	StatuslineAccentFg Color
	StatuslineInfoFg   Color
	StatuslineInfoBg   Color
	StatuslineErrorFg  Color
	StatuslineErrorBg  Color
	StatuslineFillBg   Color
}

// Palette defines the base color values used throughout the UI.
// Each entry is a semantic name for a unique color; ColorConfig fields reference these.
// To change a color everywhere it appears, change it here.
type Palette struct {
	HighlightColor   Color // yellow — accents, focus markers, key bindings, borders
	TextColor        Color // white — primary text on dark background
	TransparentColor Color // default/transparent — inherit background
	MutedColor       Color // #686868 — de-emphasized text, placeholders, hints, borders, dim values/labels, descriptions
	SoftBorderColor  Color // subtle border for unselected task boxes (dark: matches MutedColor, light: recedes)
	SoftTextColor    Color // #b4b4b4 — secondary readable text (task box titles, action labels)
	AccentColor      Color // #008000 — label text (green)
	ValueColor       Color // #8c92ac — field values (cool gray)
	InfoLabelColor   Color // #ffa500 — orange, header view name

	// Selection
	SelectionBgColor Color // #3a5f8a — steel blue selection row background

	// Action key / accent blue
	AccentBlue Color // #5fafff — cyan-blue (action keys, points bar, chart bars)
	SlateColor Color // #5f6982 — muted blue-gray (tag values, unfilled bar segments)

	// Logo
	LogoDotColor    Color // #40e0d0 — bright turquoise (● in header art)
	LogoShadeColor  Color // #4682b4 — steel blue (▓ in header art)
	LogoBorderColor Color // #324664 — dark navy (▒ in header art)

	// Gradients
	CaptionFallbackGradient Gradient // Midnight Blue → Royal Blue
	DeepSkyBlue             Color    // #00bfff — task ID base color + gradient fallback
	DeepPurple              Color    // #865ad6 — fallback for burndown gradient

	// Content area
	ContentBackgroundColor Color // canvas background (transparent/default — inherits terminal bg)

	// Statusline
	StatuslineDarkBg   Color // darkest statusline background (accent foreground)
	StatuslineMidBg    Color // mid statusline background (info/error/fill)
	StatuslineBorderBg Color // statusline main background + deps editor background
	StatuslineText     Color // statusline primary text
	StatuslineAccent   Color // statusline accent background
	StatuslineOk       Color // statusline info/success foreground

	// Plugin caption colors (6 curated fg/bg pairs per theme)
	CaptionColors []CaptionColorPair
}

// darkenRGB returns a darkened version of an RGB triple. ratio 0 = no change, 1 = black.
func darkenRGB(rgb [3]int, ratio float64) [3]int {
	return [3]int{
		int(float64(rgb[0]) * (1 - ratio)),
		int(float64(rgb[1]) * (1 - ratio)),
		int(float64(rgb[2]) * (1 - ratio)),
	}
}

// gradientFromColor derives a gradient from a single Color by darkening for the start.
func gradientFromColor(c Color, darkenRatio float64) Gradient {
	r, g, b := c.RGB()
	end := [3]int{int(r), int(g), int(b)}
	return Gradient{Start: darkenRGB(end, darkenRatio), End: end}
}

// ColorsFromPalette builds a ColorConfig from a Palette.
func ColorsFromPalette(p Palette) *ColorConfig {
	idGradient := gradientFromColor(p.DeepSkyBlue, 0.2)
	deepPurpleSolid := Gradient{Start: [3]int{134, 90, 214}, End: [3]int{134, 90, 214}}
	blueCyanSolid := Gradient{Start: [3]int{90, 170, 255}, End: [3]int{90, 170, 255}}
	headerPurpleSolid := Gradient{Start: [3]int{160, 120, 230}, End: [3]int{160, 120, 230}}
	headerCyanSolid := Gradient{Start: [3]int{110, 190, 255}, End: [3]int{110, 190, 255}}

	return &ColorConfig{
		CaptionFallbackGradient: p.CaptionFallbackGradient,

		// Task box
		TaskBoxSelectedBorder:       p.HighlightColor,
		TaskBoxUnselectedBorder:     p.SoftBorderColor,
		TaskBoxUnselectedBackground: p.TransparentColor,
		TaskBoxIDColor:              idGradient,
		TaskBoxTitleColor:           p.SoftTextColor,
		TaskBoxLabelColor:           p.MutedColor,
		TaskBoxDescriptionColor:     p.MutedColor,
		TaskBoxTagValueColor:        p.SlateColor,
		TaskListSelectionFg:         p.TextColor,
		TaskListSelectionBg:         p.SelectionBgColor,
		TaskListStatusDoneColor:     p.AccentColor,
		TaskListStatusPendingColor:  p.TextColor,

		// Task detail
		TaskDetailIDColor:           idGradient,
		TaskDetailTitleText:         p.HighlightColor,
		TaskDetailLabelText:         p.AccentColor,
		TaskDetailValueText:         p.ValueColor,
		TaskDetailCommentAuthor:     p.HighlightColor,
		TaskDetailEditDimTextColor:  p.MutedColor,
		TaskDetailEditDimLabelColor: p.MutedColor,
		TaskDetailEditDimValueColor: p.SoftTextColor,
		TaskDetailEditFocusMarker:   p.HighlightColor,
		TaskDetailEditFocusText:     p.TextColor,
		TaskDetailTagForeground:     p.SoftTextColor,
		TaskDetailTagBackground:     p.SelectionBgColor,
		TaskDetailPlaceholderColor:  p.MutedColor,

		// Content area
		ContentBackgroundColor: p.ContentBackgroundColor,
		ContentTextColor:       p.TextColor,

		// Input box
		InputBoxLabelColor:      p.TextColor,
		InputBoxBackgroundColor: p.TransparentColor,
		InputBoxTextColor:       p.TextColor,

		// Input field
		InputFieldBackgroundColor: p.TransparentColor,
		InputFieldTextColor:       p.TextColor,

		// Completion prompt
		CompletionHintColor: p.MutedColor,

		// Burndown chart
		BurndownChartAxisColor:     p.MutedColor,
		BurndownChartLabelColor:    p.MutedColor,
		BurndownChartValueColor:    p.MutedColor,
		BurndownChartBarColor:      p.AccentBlue,
		BurndownChartGradientFrom:  deepPurpleSolid,
		BurndownChartGradientTo:    blueCyanSolid,
		BurndownHeaderGradientFrom: headerPurpleSolid,
		BurndownHeaderGradientTo:   headerCyanSolid,

		// Points bar
		PointsFilledColor:   p.AccentBlue,
		PointsUnfilledColor: p.SlateColor,

		// Header
		HeaderInfoLabel:     p.InfoLabelColor,
		HeaderInfoSeparator: p.MutedColor,
		HeaderInfoDesc:      p.MutedColor,
		HeaderKeyBinding:    p.HighlightColor,
		HeaderKeyText:       p.TextColor,

		// Header context help actions
		HeaderActionGlobalKeyColor:   p.HighlightColor,
		HeaderActionGlobalLabelColor: p.TextColor,
		HeaderActionPluginKeyColor:   p.InfoLabelColor,
		HeaderActionPluginLabelColor: p.SoftTextColor,
		HeaderActionViewKeyColor:     p.AccentBlue,
		HeaderActionViewLabelColor:   p.MutedColor,

		// Plugin-specific
		DepsEditorBackground: p.StatuslineBorderBg,

		// Fallback solid colors
		FallbackTaskIDColor:   p.DeepSkyBlue,
		FallbackBurndownColor: p.DeepPurple,

		// Logo
		LogoDotColor:    p.LogoDotColor,
		LogoShadeColor:  p.LogoShadeColor,
		LogoBorderColor: p.LogoBorderColor,

		// Statusline
		StatuslineBg:       p.StatuslineBorderBg,
		StatuslineFg:       p.StatuslineText,
		StatuslineAccentBg: p.StatuslineAccent,
		StatuslineAccentFg: p.StatuslineDarkBg,
		StatuslineInfoFg:   p.StatuslineOk,
		StatuslineInfoBg:   p.StatuslineMidBg,
		StatuslineErrorFg:  p.HighlightColor,
		StatuslineErrorBg:  p.StatuslineMidBg,
		StatuslineFillBg:   p.StatuslineMidBg,

		// Plugin caption colors
		CaptionColors: p.CaptionColors,
	}
}

// CaptionColorForIndex returns the caption color pair for a plugin at the given config index.
// Wraps modulo slice length. Returns zero-value for negative index or empty slice.
func (cc *ColorConfig) CaptionColorForIndex(index int) CaptionColorPair {
	if index < 0 || len(cc.CaptionColors) == 0 {
		return CaptionColorPair{}
	}
	return cc.CaptionColors[index%len(cc.CaptionColors)]
}

// Global color config instance
var globalColors *ColorConfig
var colorsInitialized bool

// UseGradients controls whether gradients are rendered or solid colors are used
// Set during bootstrap based on terminal color count vs gradientThreshold
var UseGradients bool

// UseWideGradients controls whether screen-wide gradients (like caption rows) are rendered
// Screen-wide gradients show more banding on 256-color terminals, so require truecolor
var UseWideGradients bool

// GetColors returns the global color configuration for the effective theme
func GetColors() *ColorConfig {
	if !colorsInitialized {
		globalColors = ColorsFromPalette(PaletteForTheme())
		colorsInitialized = true
	}
	return globalColors
}

// SetColors sets a custom color configuration
func SetColors(colors *ColorConfig) {
	globalColors = colors
}
