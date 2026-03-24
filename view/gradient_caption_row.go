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
	laneNames  []string
	laneWidths []int           // proportional widths (same values used in tview.Flex)
	gradient   config.Gradient // computed gradient (for truecolor/256-color terminals)
	textColor  tcell.Color
}

// NewGradientCaptionRow creates a new gradient caption row widget.
// laneWidths should match the flex proportions used for lane layout (nil = equal).
func NewGradientCaptionRow(laneNames []string, laneWidths []int, bgColor tcell.Color, textColor tcell.Color) *GradientCaptionRow {
	return &GradientCaptionRow{
		Box:        tview.NewBox(),
		laneNames:  laneNames,
		laneWidths: laneWidths,
		gradient:   computeCaptionGradient(bgColor),
		textColor:  textColor,
	}
}

// Draw renders all lane captions with a screen-wide gradient background
func (gcr *GradientCaptionRow) Draw(screen tcell.Screen) {
	gcr.DrawForSubclass(screen, gcr)

	x, y, width, height := gcr.GetInnerRect()
	if width <= 0 || height <= 0 || len(gcr.laneNames) == 0 {
		return
	}

	numLanes := len(gcr.laneNames)

	// Calculate proportional lane boundaries
	laneStarts, laneEnds := computeLaneBoundaries(gcr.laneWidths, numLanes, width)

	// Convert all lane names to runes for Unicode handling
	laneRunes := make([][]rune, numLanes)
	for i, name := range gcr.laneNames {
		laneRunes[i] = []rune(name)
	}

	// Build a column→lane lookup for efficient rendering
	laneIndex := 0

	// Render each lane position across the screen
	for col := 0; col < width; col++ {
		// Advance lane index when we pass the current lane's end
		for laneIndex < numLanes-1 && col >= laneEnds[laneIndex] {
			laneIndex++
		}

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

		currentLaneWidth := laneEnds[laneIndex] - laneStarts[laneIndex]
		posInLane := col - laneStarts[laneIndex]

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

// computeLaneBoundaries calculates pixel start/end positions for each lane
// based on proportional weights. Weights should be pre-normalized via normalizeLaneWidths;
// zero/missing entries default to weight 1 as a safety fallback.
// Returns parallel slices of start and end positions.
func computeLaneBoundaries(weights []int, numLanes, totalWidth int) (starts []int, ends []int) {
	starts = make([]int, numLanes)
	ends = make([]int, numLanes)

	totalWeight := 0
	for i := 0; i < numLanes; i++ {
		if i < len(weights) && weights[i] > 0 {
			totalWeight += weights[i]
		} else {
			totalWeight += 1
		}
	}

	pos := 0
	for i := 0; i < numLanes; i++ {
		w := 1
		if i < len(weights) && weights[i] > 0 {
			w = weights[i]
		}
		starts[i] = pos
		if i == numLanes-1 {
			ends[i] = totalWidth // last lane absorbs rounding remainder
		} else {
			ends[i] = pos + (totalWidth*w)/totalWeight
		}
		pos = ends[i]
	}
	return starts, ends
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
