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
	// Board view colors
	BoardColumnTitleBackground tcell.Color
	BoardColumnTitleText       tcell.Color
	BoardColumnBorder          tcell.Color
	BoardColumnTitleGradient   Gradient
	BoardPaneTitleBackground   tcell.Color
	BoardPaneTitleText         tcell.Color
	BoardPaneBorder            tcell.Color
	BoardPaneTitleGradient     Gradient

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

	// Search box colors
	SearchBoxLabelColor      tcell.Color
	SearchBoxBackgroundColor tcell.Color
	SearchBoxTextColor       tcell.Color

	// Input field colors (used in task detail edit mode)
	InputFieldBackgroundColor tcell.Color
	InputFieldTextColor       tcell.Color

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
	HeaderInfoLabel  string // tview color string like "[orange]"
	HeaderInfoValue  string // tview color string like "[white]"
	HeaderKeyBinding string // tview color string like "[yellow]"
	HeaderKeyText    string // tview color string like "[white]"
}

// DefaultColors returns the default color configuration
func DefaultColors() *ColorConfig {
	return &ColorConfig{
		// Board view
		BoardColumnTitleBackground: tcell.ColorNavy,
		BoardColumnTitleText:       tcell.PaletteColor(153), // Sky Blue (ANSI 153)
		BoardColumnBorder:          tcell.ColorDefault,      // transparent/no border
		BoardColumnTitleGradient: Gradient{
			Start: [3]int{25, 25, 112},  // Midnight Blue (center)
			End:   [3]int{65, 105, 225}, // Royal Blue (edges)
		},
		BoardPaneTitleBackground: tcell.ColorNavy,
		BoardPaneTitleText:       tcell.PaletteColor(153), // Sky Blue (ANSI 153)
		BoardPaneBorder:          tcell.ColorDefault,      // transparent/no border
		BoardPaneTitleGradient: Gradient{
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
		TaskBoxTitleColor:       "[#b8b8b8]", // Light gray
		TaskBoxLabelColor:       "[#767676]", // Darker gray for labels
		TaskBoxDescriptionColor: "[#767676]", // Darker gray for description
		TaskBoxTagValueColor:    "[#5a6f8f]", // Blueish gray for tag values

		// Task detail
		TaskDetailIDColor: Gradient{
			Start: [3]int{30, 144, 255}, // Dodger Blue (same as task box)
			End:   [3]int{0, 191, 255},  // Deep Sky Blue
		},
		TaskDetailTitleText:         "[yellow]",
		TaskDetailLabelText:         "[green]",
		TaskDetailValueText:         "[#8c92ac]",
		TaskDetailCommentAuthor:     "[yellow]",
		TaskDetailEditDimTextColor:  "[#808080]", // Medium gray for dim text
		TaskDetailEditDimLabelColor: "[#606060]", // Darker gray for dim labels
		TaskDetailEditDimValueColor: "[#909090]", // Lighter gray for dim values
		TaskDetailEditFocusMarker:   "[yellow]",  // Yellow arrow for focus
		TaskDetailEditFocusText:     "[white]",   // White text after arrow

		// Search box
		SearchBoxLabelColor:      tcell.ColorWhite,
		SearchBoxBackgroundColor: tcell.ColorDefault, // Transparent
		SearchBoxTextColor:       tcell.ColorWhite,

		// Input field colors
		InputFieldBackgroundColor: tcell.ColorDefault, // Transparent
		InputFieldTextColor:       tcell.ColorWhite,

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

		// Header
		HeaderInfoLabel:  "[orange]",
		HeaderInfoValue:  "[#cccccc]",
		HeaderKeyBinding: "[yellow]",
		HeaderKeyText:    "[white]",
	}
}

// Global color config instance
var globalColors *ColorConfig
var colorsInitialized bool

// GetColors returns the global color configuration with theme-aware overrides
func GetColors() *ColorConfig {
	if !colorsInitialized {
		globalColors = DefaultColors()
		// Apply theme-aware overrides for critical text colors
		if GetEffectiveTheme() == "light" {
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
