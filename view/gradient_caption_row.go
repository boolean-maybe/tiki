package view

import (
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/util/gradient"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// GradientCaptionRow is a tview primitive that renders multiple pane captions
// with a continuous horizontal background gradient spanning the entire screen width
type GradientCaptionRow struct {
	*tview.Box
	paneNames []string
	gradient  config.Gradient
	textColor tcell.Color
}

// NewGradientCaptionRow creates a new gradient caption row widget
func NewGradientCaptionRow(paneNames []string, gradient config.Gradient, textColor tcell.Color) *GradientCaptionRow {
	return &GradientCaptionRow{
		Box:       tview.NewBox(),
		paneNames: paneNames,
		gradient:  gradient,
		textColor: textColor,
	}
}

// Draw renders all pane captions with a screen-wide gradient background
func (gcr *GradientCaptionRow) Draw(screen tcell.Screen) {
	gcr.DrawForSubclass(screen, gcr)

	x, y, width, height := gcr.GetInnerRect()
	if width <= 0 || height <= 0 || len(gcr.paneNames) == 0 {
		return
	}

	// Calculate pane width (equal distribution)
	numPanes := len(gcr.paneNames)
	paneWidth := width / numPanes

	// Convert all pane names to runes for Unicode handling
	paneRunes := make([][]rune, numPanes)
	for i, name := range gcr.paneNames {
		paneRunes[i] = []rune(name)
	}

	// Render each pane position across the screen
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
		bgColor := gradient.InterpolateColor(gcr.gradient, distanceFromCenter)

		// Determine which pane this position belongs to
		paneIndex := col / paneWidth
		if paneIndex >= numPanes {
			paneIndex = numPanes - 1
		}

		// Calculate position within this pane
		paneStartX := paneIndex * paneWidth
		paneEndX := paneStartX + paneWidth
		if paneIndex == numPanes-1 {
			paneEndX = width // Last pane extends to screen edge
		}
		currentPaneWidth := paneEndX - paneStartX
		posInPane := col - paneStartX

		// Get the text for this pane
		textRunes := paneRunes[paneIndex]
		textWidth := len(textRunes)

		// Calculate centered text position within pane
		textStartPos := 0
		if textWidth < currentPaneWidth {
			textStartPos = (currentPaneWidth - textWidth) / 2
		}

		// Determine if we should render a character at this position
		char := ' '
		textIndex := posInPane - textStartPos
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

const (
	useVibrantPluginGradient = true
	// increase this to get vibrance boost
	vibrantBoost = 2.6
)

// pluginCaptionGradient selects the gradient derivation for plugin captions.
func pluginCaptionGradient(primary tcell.Color, fallback config.Gradient) config.Gradient {
	if useVibrantPluginGradient {
		return gradient.GradientFromColorVibrant(primary, vibrantBoost, fallback)
	}
	return gradient.GradientFromColor(primary, 0.35, fallback)
}
