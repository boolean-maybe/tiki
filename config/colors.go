package config

// Color and style definitions for the UI: gradients, tcell colors, tview color tags.

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
	TaskBoxSelectedBackground   tcell.Color
	TaskBoxSelectedText         tcell.Color
	TaskBoxSelectedBorder       tcell.Color
	TaskBoxUnselectedBorder     tcell.Color
	TaskBoxUnselectedBackground tcell.Color
	TaskBoxIDColor              Gradient
	TaskBoxTitleColor           string // tview color string like "[#b8b8b8]"
	TaskBoxLabelColor           string // tview color string like "[#767676]"
	TaskBoxDescriptionColor     string // tview color string like "[#767676]"
	TaskBoxTagValueColor        string // tview color string like "[#5a6f8f]"
	TaskListSelectionColor      string // tview color string for selected row highlight, e.g. "[white:#3a5f8a]"
	TaskListStatusDoneColor     string // tview color string for done status indicator, e.g. "[#00ff7f]"
	TaskListStatusPendingColor  string // tview color string for pending status indicator, e.g. "[white]"

	// Task detail view colors
	TaskDetailIDColor           Gradient
	TaskDetailTitleText         string // tview color string like "[yellow]"
	TaskDetailLabelText         string // tview color string like "[green]"
	TaskDetailValueText         string // tview color string like "[white]"
	TaskDetailCommentAuthor     string // tview color string like "[yellow]"
	TaskDetailEditDimTextColor  string // tview color string like "[#808080]"
	TaskDetailEditDimLabelColor string // tview color string like "[#606060]"
	TaskDetailEditDimValueColor string // tview color string like "[#909090]"
	TaskDetailEditFocusMarker   string // tview color string like "[yellow]"
	TaskDetailEditFocusText     string // tview color string like "[white]"
	TaskDetailTagForeground     tcell.Color
	TaskDetailTagBackground     tcell.Color
	TaskDetailPlaceholderColor  tcell.Color

	// Content area colors (base canvas for editable/readable content)
	ContentBackgroundColor tcell.Color
	ContentTextColor       tcell.Color

	// Search box colors
	SearchBoxLabelColor      tcell.Color
	SearchBoxBackgroundColor tcell.Color
	SearchBoxTextColor       tcell.Color

	// Input field colors (used in task detail edit mode)
	InputFieldBackgroundColor tcell.Color
	InputFieldTextColor       tcell.Color

	// Completion prompt colors
	CompletionHintColor tcell.Color

	// Burndown chart colors
	BurndownChartAxisColor     tcell.Color
	BurndownChartLabelColor    tcell.Color
	BurndownChartValueColor    tcell.Color
	BurndownChartBarColor      tcell.Color
	BurndownChartGradientFrom  Gradient
	BurndownChartGradientTo    Gradient
	BurndownHeaderGradientFrom Gradient // Header-specific chart gradient
	BurndownHeaderGradientTo   Gradient

	// Header view colors
	HeaderInfoLabel     string // tview color string for view name (bold)
	HeaderInfoSeparator string // tview color string for horizontal rule below name
	HeaderInfoDesc      string // tview color string for view description
	HeaderKeyBinding    string // tview color string like "[yellow]"
	HeaderKeyText       string // tview color string like "[white]"

	// Points visual bar colors
	PointsFilledColor   string // tview color string for filled segments
	PointsUnfilledColor string // tview color string for unfilled segments

	// Header context help action colors
	HeaderActionGlobalKeyColor   string // tview color string for global action keys
	HeaderActionGlobalLabelColor string // tview color string for global action labels
	HeaderActionPluginKeyColor   string // tview color string for plugin action keys
	HeaderActionPluginLabelColor string // tview color string for plugin action labels
	HeaderActionViewKeyColor     string // tview color string for view action keys
	HeaderActionViewLabelColor   string // tview color string for view action labels

	// Plugin-specific colors
	DepsEditorBackground tcell.Color // muted slate for dependency editor caption

	// Fallback solid colors for gradient scenarios (used when UseGradients = false)
	FallbackTaskIDColor   tcell.Color // Deep Sky Blue (end of task ID gradient)
	FallbackBurndownColor tcell.Color // Purple (start of burndown gradient)

	// Statusline colors (bottom bar, powerline style)
	StatuslineBg       string // hex color for stat segment background, e.g. "#3a3a5c"
	StatuslineFg       string // hex color for stat segment text, e.g. "#cccccc"
	StatuslineAccentBg string // hex color for accent segment background (first segment), e.g. "#5f87af"
	StatuslineAccentFg string // hex color for accent segment text, e.g. "#1c1c2e"
	StatuslineInfoFg   string // hex color for info message text
	StatuslineInfoBg   string // hex color for info message background
	StatuslineErrorFg  string // hex color for error message text
	StatuslineErrorBg  string // hex color for error message background
	StatuslineFillBg   string // hex color for empty statusline area between segments
}

// DefaultColors returns the default color configuration
func DefaultColors() *ColorConfig {
	return &ColorConfig{
		// Caption fallback gradient
		CaptionFallbackGradient: Gradient{
			Start: [3]int{25, 25, 112},  // Midnight Blue (center)
			End:   [3]int{65, 105, 225}, // Royal Blue (edges)
		},

		// Task box
		TaskBoxSelectedBackground:   tcell.PaletteColor(33),  // Blue (ANSI 33)
		TaskBoxSelectedText:         tcell.PaletteColor(117), // Light Blue (ANSI 117)
		TaskBoxSelectedBorder:       tcell.ColorYellow,
		TaskBoxUnselectedBorder:     tcell.ColorGray,
		TaskBoxUnselectedBackground: tcell.ColorDefault, // transparent/no background
		TaskBoxIDColor: Gradient{
			Start: [3]int{30, 144, 255}, // Dodger Blue
			End:   [3]int{0, 191, 255},  // Deep Sky Blue
		},
		TaskBoxTitleColor:          "[#b8b8b8]",       // Light gray
		TaskBoxLabelColor:          "[#767676]",       // Darker gray for labels
		TaskBoxDescriptionColor:    "[#767676]",       // Darker gray for description
		TaskBoxTagValueColor:       "[#5a6f8f]",       // Blueish gray for tag values
		TaskListSelectionColor:     "[white:#3a5f8a]", // White text on steel blue background
		TaskListStatusDoneColor:    "[#00ff7f]",       // Spring green for done checkmark
		TaskListStatusPendingColor: "[white]",         // White for pending circle

		// Task detail
		TaskDetailIDColor: Gradient{
			Start: [3]int{30, 144, 255}, // Dodger Blue (same as task box)
			End:   [3]int{0, 191, 255},  // Deep Sky Blue
		},
		TaskDetailTitleText:         "[yellow]",
		TaskDetailLabelText:         "[green]",
		TaskDetailValueText:         "[#8c92ac]",
		TaskDetailCommentAuthor:     "[yellow]",
		TaskDetailEditDimTextColor:  "[#808080]",                      // Medium gray for dim text
		TaskDetailEditDimLabelColor: "[#606060]",                      // Darker gray for dim labels
		TaskDetailEditDimValueColor: "[#909090]",                      // Lighter gray for dim values
		TaskDetailEditFocusMarker:   "[yellow]",                       // Yellow arrow for focus
		TaskDetailEditFocusText:     "[white]",                        // White text after arrow
		TaskDetailTagForeground:     tcell.NewRGBColor(180, 200, 220), // Light blue-gray text
		TaskDetailTagBackground:     tcell.NewRGBColor(30, 50, 120),   // Dark blue background (more bluish)
		TaskDetailPlaceholderColor:  tcell.ColorGray,                  // Gray for placeholder text in edit fields

		// Content area (base canvas)
		ContentBackgroundColor: tcell.ColorBlack, // dark theme: explicit black
		ContentTextColor:       tcell.ColorWhite, // dark theme: white text

		// Search box
		SearchBoxLabelColor:      tcell.ColorWhite,
		SearchBoxBackgroundColor: tcell.ColorDefault, // Transparent
		SearchBoxTextColor:       tcell.ColorWhite,

		// Input field colors
		InputFieldBackgroundColor: tcell.ColorDefault, // Transparent
		InputFieldTextColor:       tcell.ColorWhite,

		// Completion prompt
		CompletionHintColor: tcell.NewRGBColor(128, 128, 128), // Medium gray for hint text

		// Burndown chart
		BurndownChartAxisColor:  tcell.NewRGBColor(80, 80, 80),    // Dark gray
		BurndownChartLabelColor: tcell.NewRGBColor(200, 200, 200), // Light gray
		BurndownChartValueColor: tcell.NewRGBColor(235, 235, 235), // Very light gray
		BurndownChartBarColor:   tcell.NewRGBColor(120, 170, 255), // Light blue
		BurndownChartGradientFrom: Gradient{
			Start: [3]int{134, 90, 214}, // Deep purple
			End:   [3]int{134, 90, 214}, // Deep purple (solid, not gradient)
		},
		BurndownChartGradientTo: Gradient{
			Start: [3]int{90, 170, 255}, // Blue/cyan
			End:   [3]int{90, 170, 255}, // Blue/cyan (solid, not gradient)
		},
		BurndownHeaderGradientFrom: Gradient{
			Start: [3]int{160, 120, 230}, // Purple base for header chart
			End:   [3]int{160, 120, 230}, // Purple base (solid)
		},
		BurndownHeaderGradientTo: Gradient{
			Start: [3]int{110, 190, 255}, // Cyan top for header chart
			End:   [3]int{110, 190, 255}, // Cyan top (solid)
		},

		// Points visual bar
		PointsFilledColor:   "[#508cff]", // Blue for filled segments
		PointsUnfilledColor: "[#5f6982]", // Gray for unfilled segments

		// Header
		HeaderInfoLabel:     "[orange]",
		HeaderInfoSeparator: "[#555555]",
		HeaderInfoDesc:      "[#888888]",
		HeaderKeyBinding:    "[yellow]",
		HeaderKeyText:       "[white]",

		// Header context help actions
		HeaderActionGlobalKeyColor:   "#ffff00", // yellow for global actions
		HeaderActionGlobalLabelColor: "#ffffff", // white for global action labels
		HeaderActionPluginKeyColor:   "#ff8c00", // orange for plugin actions
		HeaderActionPluginLabelColor: "#b0b0b0", // light gray for plugin labels
		HeaderActionViewKeyColor:     "#5fafff", // cyan for view-specific actions
		HeaderActionViewLabelColor:   "#808080", // gray for view-specific labels

		// Plugin-specific
		DepsEditorBackground: tcell.NewHexColor(0x4e5768), // Muted slate

		// Fallback solid colors (no-gradient terminals)
		FallbackTaskIDColor:   tcell.NewRGBColor(0, 191, 255),  // Deep Sky Blue
		FallbackBurndownColor: tcell.NewRGBColor(134, 90, 214), // Purple

		// Statusline (Nord theme)
		StatuslineBg:       "#434c5e", // Nord polar night 3
		StatuslineFg:       "#d8dee9", // Nord snow storm 1
		StatuslineAccentBg: "#5e81ac", // Nord frost blue
		StatuslineAccentFg: "#2e3440", // Nord polar night 1
		StatuslineInfoFg:   "#a3be8c", // Nord aurora green
		StatuslineInfoBg:   "#3b4252", // Nord polar night 2
		StatuslineErrorFg:  "#ffff00", // yellow, matches header global key color
		StatuslineErrorBg:  "#3b4252", // Nord polar night 2
		StatuslineFillBg:   "#3b4252", // Nord polar night 2
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
			globalColors.ContentBackgroundColor = tcell.ColorDefault
			globalColors.ContentTextColor = tcell.ColorBlack
			globalColors.SearchBoxLabelColor = tcell.ColorBlack
			globalColors.SearchBoxTextColor = tcell.ColorBlack
			globalColors.InputFieldTextColor = tcell.ColorBlack
			globalColors.TaskDetailEditFocusText = "[black]"
			globalColors.HeaderKeyText = "[black]"
		}
		colorsInitialized = true
	}
	return globalColors
}

// SetColors sets a custom color configuration
func SetColors(colors *ColorConfig) {
	globalColors = colors
}
