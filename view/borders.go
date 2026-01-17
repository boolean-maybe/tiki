package view

import (
	"github.com/boolean-maybe/tiki/config"

	"github.com/gdamore/tcell/v2"
)

// Single-line box drawing characters
const (
	BorderHorizontal  = '─'
	BorderVertical    = '│'
	BorderTopLeft     = '┌'
	BorderTopRight    = '┐'
	BorderBottomLeft  = '└'
	BorderBottomRight = '┘'
)

// DrawSingleLineBorder draws a single-line border around the given rectangle
// using the TaskBoxUnselectedBorder color from config.
// This is useful for primitives that should not use tview's double-line focus borders.
func DrawSingleLineBorder(screen tcell.Screen, x, y, width, height int) {
	if width <= 0 || height <= 0 {
		return
	}

	colors := config.GetColors()
	style := tcell.StyleDefault.Foreground(colors.TaskBoxUnselectedBorder).Background(config.GetContentBackgroundColor())

	DrawSingleLineBorderWithStyle(screen, x, y, width, height, style)
}

// DrawSingleLineBorderWithStyle draws a single-line border with a custom style
func DrawSingleLineBorderWithStyle(screen tcell.Screen, x, y, width, height int, style tcell.Style) {
	if width <= 0 || height <= 0 {
		return
	}

	// Draw horizontal lines
	for i := x + 1; i < x+width-1; i++ {
		screen.SetContent(i, y, BorderHorizontal, nil, style)
		screen.SetContent(i, y+height-1, BorderHorizontal, nil, style)
	}

	// Draw vertical lines
	for i := y + 1; i < y+height-1; i++ {
		screen.SetContent(x, i, BorderVertical, nil, style)
		screen.SetContent(x+width-1, i, BorderVertical, nil, style)
	}

	// Draw corners
	screen.SetContent(x, y, BorderTopLeft, nil, style)
	screen.SetContent(x+width-1, y, BorderTopRight, nil, style)
	screen.SetContent(x, y+height-1, BorderBottomLeft, nil, style)
	screen.SetContent(x+width-1, y+height-1, BorderBottomRight, nil, style)
}
