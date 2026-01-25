package view

import (
	"github.com/boolean-maybe/tiki/config"

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
		bgColor := interpolateColor(gcr.gradient, distanceFromCenter)

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
	// Add 0.5 before truncating to int for proper rounding, ensuring smoother gradients
	r := int(float64(gradient.Start[0]) + t*float64(gradient.End[0]-gradient.Start[0]) + 0.5)
	g := int(float64(gradient.Start[1]) + t*float64(gradient.End[1]-gradient.Start[1]) + 0.5)
	b := int(float64(gradient.Start[2]) + t*float64(gradient.End[2]-gradient.Start[2]) + 0.5)

	//nolint:gosec // G115: RGB values are 0-255, safe to convert to int32
	return tcell.NewRGBColor(int32(r), int32(g), int32(b))
}

func gradientFromPrimaryColor(primary tcell.Color, fallback config.Gradient) config.Gradient {
	if primary == tcell.ColorDefault || !primary.Valid() {
		return fallback
	}

	r, g, b := primary.TrueColor().RGB()
	base := [3]int{int(r), int(g), int(b)}
	edge := lightenRGB(base, 0.35)

	return config.Gradient{
		Start: base,
		End:   edge,
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
		return gradientFromPrimaryColorVibrant(primary, fallback)
	}
	return gradientFromPrimaryColor(primary, fallback)
}

// gradientFromPrimaryColorVibrant derives a brighter gradient without desaturating.
func gradientFromPrimaryColorVibrant(primary tcell.Color, fallback config.Gradient) config.Gradient {
	if primary == tcell.ColorDefault || !primary.Valid() {
		return fallback
	}

	r, g, b := primary.TrueColor().RGB()
	base := [3]int{int(r), int(g), int(b)}
	edge := [3]int{
		clampRGB(int(float64(base[0]) * vibrantBoost)),
		clampRGB(int(float64(base[1]) * vibrantBoost)),
		clampRGB(int(float64(base[2]) * vibrantBoost)),
	}

	return config.Gradient{
		Start: base,
		End:   edge,
	}
}

func lightenRGB(rgb [3]int, ratio float64) [3]int {
	return [3]int{
		clampRGB(int(float64(rgb[0]) + (255.0-float64(rgb[0]))*ratio)),
		clampRGB(int(float64(rgb[1]) + (255.0-float64(rgb[1]))*ratio)),
		clampRGB(int(float64(rgb[2]) + (255.0-float64(rgb[2]))*ratio)),
	}
}

func clampRGB(value int) int {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return value
}
