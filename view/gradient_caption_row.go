package view

import (
	"github.com/boolean-maybe/tiki/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// GradientCaptionRow is a tview primitive that renders multiple column captions
// with a continuous horizontal background gradient spanning the entire screen width
type GradientCaptionRow struct {
	*tview.Box
	columnNames []string
	gradient    config.Gradient
	textColor   tcell.Color
}

// NewGradientCaptionRow creates a new gradient caption row widget
func NewGradientCaptionRow(columnNames []string, gradient config.Gradient, textColor tcell.Color) *GradientCaptionRow {
	return &GradientCaptionRow{
		Box:         tview.NewBox(),
		columnNames: columnNames,
		gradient:    gradient,
		textColor:   textColor,
	}
}

// Draw renders all column captions with a screen-wide gradient background
func (gcr *GradientCaptionRow) Draw(screen tcell.Screen) {
	gcr.DrawForSubclass(screen, gcr)

	x, y, width, height := gcr.GetInnerRect()
	if width <= 0 || height <= 0 || len(gcr.columnNames) == 0 {
		return
	}

	// Calculate column width (equal distribution)
	numColumns := len(gcr.columnNames)
	columnWidth := width / numColumns

	// Convert all column names to runes for Unicode handling
	columnRunes := make([][]rune, numColumns)
	for i, name := range gcr.columnNames {
		columnRunes[i] = []rune(name)
	}

	// Render each column position across the screen
	for col := 0; col < width; col++ {
		// Calculate gradient color based on screen position (edges to center gradient)
		// Distance from center: 0.0 at center, 1.0 at edges
		centerPos := float64(width) / 2.0
		distanceFromCenter := 0.0
		if width > 1 {
			distanceFromCenter = (float64(col) - centerPos) / (centerPos)
			if distanceFromCenter < 0 {
				distanceFromCenter = -distanceFromCenter
			}
		}
		bgColor := interpolateColor(gcr.gradient, distanceFromCenter)

		// Determine which column this position belongs to
		columnIndex := col / columnWidth
		if columnIndex >= numColumns {
			columnIndex = numColumns - 1
		}

		// Calculate position within this column
		columnStartX := columnIndex * columnWidth
		columnEndX := columnStartX + columnWidth
		if columnIndex == numColumns-1 {
			columnEndX = width // Last column extends to screen edge
		}
		currentColumnWidth := columnEndX - columnStartX
		posInColumn := col - columnStartX

		// Get the text for this column
		textRunes := columnRunes[columnIndex]
		textWidth := len(textRunes)

		// Calculate centered text position within column
		textStartPos := 0
		if textWidth < currentColumnWidth {
			textStartPos = (currentColumnWidth - textWidth) / 2
		}

		// Determine if we should render a character at this position
		char := ' '
		textIndex := posInColumn - textStartPos
		if textIndex >= 0 && textIndex < textWidth {
			char = textRunes[textIndex]
		}

		// Render the cell with gradient background
		style := tcell.StyleDefault.Foreground(gcr.textColor).Background(bgColor)
		for row := 0; row < height; row++ {
			screen.SetContent(x+col, y+row, char, nil, style)
		}
	}
}

// interpolateColor performs linear RGB interpolation between gradient start and end
func interpolateColor(gradient config.Gradient, t float64) tcell.Color {
	// Clamp t to [0, 1]
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	// Linear interpolation for each RGB component
	r := int(float64(gradient.Start[0]) + t*float64(gradient.End[0]-gradient.Start[0]))
	g := int(float64(gradient.Start[1]) + t*float64(gradient.End[1]-gradient.Start[1]))
	b := int(float64(gradient.Start[2]) + t*float64(gradient.End[2]-gradient.Start[2]))

	//nolint:gosec // G115: RGB values are 0-255, safe to convert to int32
	return tcell.NewRGBColor(int32(r), int32(g), int32(b))
}
