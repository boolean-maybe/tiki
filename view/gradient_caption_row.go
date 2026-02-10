package view

import (
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/util/gradient"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// GradientCaptionRow is a tview primitive that renders multiple lane captions
// with a continuous horizontal background gradient spanning the entire screen width
type GradientCaptionRow struct {
	*tview.Box
	laneNames []string
	bgColor   tcell.Color     // original background color from plugin
	gradient  config.Gradient // computed gradient (for truecolor/256-color terminals)
	textColor tcell.Color
}

// NewGradientCaptionRow creates a new gradient caption row widget
func NewGradientCaptionRow(laneNames []string, bgColor tcell.Color, textColor tcell.Color) *GradientCaptionRow {
	return &GradientCaptionRow{
		Box:       tview.NewBox(),
		laneNames: laneNames,
		bgColor:   bgColor,
		gradient:  computeCaptionGradient(bgColor),
		textColor: textColor,
	}
}

// Draw renders all lane captions with a screen-wide gradient background
func (gcr *GradientCaptionRow) Draw(screen tcell.Screen) {
	gcr.DrawForSubclass(screen, gcr)

	x, y, width, height := gcr.GetInnerRect()
	if width <= 0 || height <= 0 || len(gcr.laneNames) == 0 {
		return
	}

	// Calculate lane width (equal distribution)
	numLanes := len(gcr.laneNames)
	laneWidth := width / numLanes

	// Convert all lane names to runes for Unicode handling
	laneRunes := make([][]rune, numLanes)
	for i, name := range gcr.laneNames {
		laneRunes[i] = []rune(name)
	}

	// Render each lane position across the screen
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

		// Use adaptive gradient based on terminal color capabilities
		var bgColor tcell.Color
		if config.UseWideGradients {
			// Truecolor: full gradient effect (dark center, bright edges)
			bgColor = gradient.InterpolateColor(gcr.gradient, distanceFromCenter)
		} else if config.UseGradients {
			// 256-color: solid color from gradient (use darker start for consistency)
			bgColor = gradient.InterpolateColor(gcr.gradient, 0.0)
		} else {
			// 8/16-color: use brighter fallback from gradient instead of original color
			// Original plugin colors (like #1e3a5f) map to black on basic terminals
			bgColor = gradient.InterpolateColor(gcr.gradient, 1.0)
		}

		// Determine which lane this position belongs to
		laneIndex := col / laneWidth
		if laneIndex >= numLanes {
			laneIndex = numLanes - 1
		}

		// Calculate position within this lane
		laneStartX := laneIndex * laneWidth
		laneEndX := laneStartX + laneWidth
		if laneIndex == numLanes-1 {
			laneEndX = width // Last lane extends to screen edge
		}
		currentLaneWidth := laneEndX - laneStartX
		posInLane := col - laneStartX

		// Get the text for this lane
		textRunes := laneRunes[laneIndex]
		textWidth := len(textRunes)

		// Calculate centered text position within lane
		textStartPos := 0
		if textWidth < currentLaneWidth {
			textStartPos = (currentLaneWidth - textWidth) / 2
		}

		// Determine if we should render a character at this position
		char := ' '
		textIndex := posInLane - textStartPos
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
	vibrantBoost = 1.6
)

// computeCaptionGradient computes the gradient for caption background from a base color.
func computeCaptionGradient(primary tcell.Color) config.Gradient {
	fallback := config.GetColors().CaptionFallbackGradient
	if useVibrantPluginGradient {
		return gradient.GradientFromColorVibrant(primary, vibrantBoost, fallback)
	}
	return gradient.GradientFromColor(primary, 0.35, fallback)
}
