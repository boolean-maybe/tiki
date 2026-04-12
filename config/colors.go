package config

// Color and style definitions for the UI: gradients, unified Color values.

import (
	"github.com/gdamore/tcell/v2"
)

// Gradient defines a start and end RGB color for a gradient transition
type Gradient struct {
	Start [3]int // R, G, B (0-255)
	End   [3]int // R, G, B (0-255)
}

// ColorConfig holds all color and style definitions per view
type ColorConfig struct {
	// Caption colors
	CaptionFallbackGradient Gradient

	// Task box colors
	TaskBoxSelectedBackground   Color
	TaskBoxSelectedText         Color
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

	// Search box colors
	SearchBoxLabelColor      Color
	SearchBoxBackgroundColor Color
	SearchBoxTextColor       Color

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

	// Plugin-specific colors
	DepsEditorBackground Color // muted slate for dependency editor caption

	// Fallback solid colors for gradient scenarios (used when UseGradients = false)
	FallbackTaskIDColor   Color // Deep Sky Blue (end of task ID gradient)
	FallbackBurndownColor Color // Purple (start of burndown gradient)

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
	SoftTextColor    Color // #b4b4b4 — secondary readable text (task box titles, action labels)
	AccentColor      Color // #008000 — label text (green)
	ValueColor       Color // #8c92ac — field values (cool gray)
	TagFgColor       Color // #b4c8dc — tag chip foreground (light blue-gray)
	TagBgColor       Color // #1e3278 — tag chip background (dark blue)
	InfoLabelColor   Color // #ffa500 — orange, header view name

	// Selection
	SelectionBgColor Color // #3a5f8a — steel blue selection row background
	SelectionFgColor Color // ANSI 33 blue — selected task box background
	SelectionText    Color // ANSI 117 — selected task box text

	// Action key / accent blue
	AccentBlue Color // #5fafff — cyan-blue (action keys, points bar, chart bars)
	SlateColor Color // #5f6982 — muted blue-gray (tag values, unfilled bar segments)

	// Gradients
	CaptionFallbackGradient Gradient // Midnight Blue → Royal Blue
	DeepSkyBlue             Color    // #00bfff — task ID base color + gradient fallback
	DeepPurple              Color    // #865ad6 — fallback for burndown gradient

	// Statusline (Nord palette)
	NordPolarNight1 Color // #2e3440
	NordPolarNight2 Color // #3b4252
	NordPolarNight3 Color // #434c5e
	NordSnowStorm1  Color // #d8dee9
	NordFrostBlue   Color // #5e81ac
	NordAuroraGreen Color // #a3be8c
}

// DefaultPalette returns the default color palette.
func DefaultPalette() Palette {
	return Palette{
		HighlightColor:   NewColorHex("#ffff00"),
		TextColor:        NewColorHex("#ffffff"),
		TransparentColor: DefaultColor(),
		MutedColor:       NewColorHex("#686868"),
		SoftTextColor:    NewColorHex("#b4b4b4"),
		AccentColor:      NewColor(tcell.ColorGreen),
		ValueColor:       NewColorHex("#8c92ac"),
		TagFgColor:       NewColorRGB(180, 200, 220),
		TagBgColor:       NewColorRGB(30, 50, 120),
		InfoLabelColor:   NewColorHex("#ffa500"),

		SelectionBgColor: NewColorHex("#3a5f8a"),
		SelectionFgColor: NewColor(tcell.PaletteColor(33)),
		SelectionText:    NewColor(tcell.PaletteColor(117)),

		AccentBlue: NewColorHex("#5fafff"),
		SlateColor: NewColorHex("#5f6982"),

		CaptionFallbackGradient: Gradient{
			Start: [3]int{25, 25, 112},
			End:   [3]int{65, 105, 225},
		},
		DeepSkyBlue: NewColorRGB(0, 191, 255),
		DeepPurple:  NewColorRGB(134, 90, 214),

		NordPolarNight1: NewColorHex("#2e3440"),
		NordPolarNight2: NewColorHex("#3b4252"),
		NordPolarNight3: NewColorHex("#434c5e"),
		NordSnowStorm1:  NewColorHex("#d8dee9"),
		NordFrostBlue:   NewColorHex("#5e81ac"),
		NordAuroraGreen: NewColorHex("#a3be8c"),
	}
}

// DefaultColors returns the default color configuration built from the default palette.
func DefaultColors() *ColorConfig {
	return ColorsFromPalette(DefaultPalette())
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
		TaskBoxSelectedBackground:   p.SelectionFgColor,
		TaskBoxSelectedText:         p.SelectionText,
		TaskBoxSelectedBorder:       p.HighlightColor,
		TaskBoxUnselectedBorder:     p.MutedColor,
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
		TaskDetailTagForeground:     p.TagFgColor,
		TaskDetailTagBackground:     p.TagBgColor,
		TaskDetailPlaceholderColor:  p.MutedColor,

		// Content area
		ContentBackgroundColor: NewColor(tcell.ColorBlack),
		ContentTextColor:       p.TextColor,

		// Search box
		SearchBoxLabelColor:      p.TextColor,
		SearchBoxBackgroundColor: p.TransparentColor,
		SearchBoxTextColor:       p.TextColor,

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
		DepsEditorBackground: p.NordPolarNight3,

		// Fallback solid colors
		FallbackTaskIDColor:   p.DeepSkyBlue,
		FallbackBurndownColor: p.DeepPurple,

		// Statusline
		StatuslineBg:       p.NordPolarNight3,
		StatuslineFg:       p.NordSnowStorm1,
		StatuslineAccentBg: p.NordFrostBlue,
		StatuslineAccentFg: p.NordPolarNight1,
		StatuslineInfoFg:   p.NordAuroraGreen,
		StatuslineInfoBg:   p.NordPolarNight2,
		StatuslineErrorFg:  p.HighlightColor,
		StatuslineErrorBg:  p.NordPolarNight2,
		StatuslineFillBg:   p.NordPolarNight2,
	}
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

// GetColors returns the global color configuration with theme-aware overrides
func GetColors() *ColorConfig {
	if !colorsInitialized {
		globalColors = DefaultColors()
		// Apply theme-aware overrides for critical text colors
		if GetEffectiveTheme() == "light" {
			black := NewColor(tcell.ColorBlack)
			globalColors.ContentBackgroundColor = DefaultColor()
			globalColors.ContentTextColor = black
			globalColors.SearchBoxLabelColor = black
			globalColors.SearchBoxTextColor = black
			globalColors.InputFieldTextColor = black
			globalColors.TaskDetailEditFocusText = black
			globalColors.HeaderKeyText = black
		}
		colorsInitialized = true
	}
	return globalColors
}

// SetColors sets a custom color configuration
func SetColors(colors *ColorConfig) {
	globalColors = colors
}
