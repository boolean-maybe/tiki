package view

import (
	gradcore "github.com/boolean-maybe/tiki/internal/gradient"
	"github.com/boolean-maybe/tiki/theme"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// GradientCaptionRow is a tview primitive that renders multiple lane
// captions with a continuous horizontal background gradient spanning the
// entire screen width. The per-column color comes from a PositionPaint so
// the widget itself is unaware of gradient vs. solid — that decision lives
// inside the PositionPaint implementation.
type GradientCaptionRow struct {
	*tview.Box
	laneNames  []string
	laneWidths []int
	paint      theme.PositionPaint
	textColor  theme.Color
}

// NewGradientCaptionRow creates a new gradient caption row. The widget
// derives its per-column background color from bgRole with the `.lift`
// modifier: a vibrant gradient on capable terminals, a solid color when
// gradients are disabled.
func NewGradientCaptionRow(laneNames []string, laneWidths []int, bgRole theme.Role, textColor theme.Color) *GradientCaptionRow {
	pp, ok := theme.PaintForRolePosition(bgRole, "lift")
	if !ok {
		// defensive fallback — an unknown modifier from a refactor mistake
		// should not crash the UI. fall back to solid paint with the same role.
		pp, _ = theme.PaintForRolePosition(bgRole, "")
	}
	return &GradientCaptionRow{
		Box:        tview.NewBox(),
		laneNames:  laneNames,
		laneWidths: laneWidths,
		paint:      pp,
		textColor:  textColor,
	}
}

// Draw renders all lane captions with a screen-wide gradient background.
func (gcr *GradientCaptionRow) Draw(screen tcell.Screen) {
	gcr.DrawForSubclass(screen, gcr)

	x, y, width, height := gcr.GetInnerRect()
	if width <= 0 || height <= 0 || len(gcr.laneNames) == 0 {
		return
	}

	numLanes := len(gcr.laneNames)
	laneStarts, laneEnds := computeLaneBoundaries(gcr.laneWidths, numLanes, width)

	laneRunes := make([][]rune, numLanes)
	for i, name := range gcr.laneNames {
		laneRunes[i] = []rune(name)
	}

	laneIndex := 0
	for col := 0; col < width; col++ {
		for laneIndex < numLanes-1 && col >= laneEnds[laneIndex] {
			laneIndex++
		}

		// distance from center: 0.0 at center, 1.0 at edges. matches the
		// previous edge-to-center sweep math byte-for-byte.
		var t float64
		if width > 1 {
			centerPos := float64(width) / 2.0
			d := (float64(col) - centerPos) / centerPos
			if d < 0 {
				d = -d
			}
			t = d
		}

		bgColor := gcr.bgColorAt(t)

		currentLaneWidth := laneEnds[laneIndex] - laneStarts[laneIndex]
		posInLane := col - laneStarts[laneIndex]

		textRunes := laneRunes[laneIndex]
		textWidth := len(textRunes)
		textStartPos := 0
		if textWidth < currentLaneWidth {
			textStartPos = (currentLaneWidth - textWidth) / 2
		}

		char := ' '
		textIndex := posInLane - textStartPos
		if textIndex >= 0 && textIndex < textWidth {
			char = textRunes[textIndex]
		}

		style := tcell.StyleDefault.Foreground(gcr.textColor.TCell()).Background(bgColor)
		for row := 0; row < height; row++ {
			screen.SetContent(x+col, y+row, char, nil, style)
		}
	}
}

// bgColorAt picks the per-column background color from the widget's
// PositionPaint, branching on terminal capability:
//
//   - truecolor (UseWideGradients): full edge-to-center gradient — the
//     paint's natural ColorAt(t).
//   - 256-color (UseGradients but not wide): flat color from the gradient
//     start (t=0). Avoids visible banding when adjacent columns quantize
//     to the same palette index.
//   - 8/16-color (UseGradients=false): flat color from the gradient end
//     (t=1) — the base role color. End is preferred over start because
//     the start of the .lift gradient is the base color itself; on
//     8/16-color terminals where gradientPaint degrades to solid base,
//     t=1 also yields the base color, matching pre-migration behavior.
func (gcr *GradientCaptionRow) bgColorAt(t float64) tcell.Color {
	switch {
	case gradcore.UseWideGradients.Load():
		return gcr.paint.ColorAt(t)
	case gradcore.UseGradients.Load():
		return gcr.paint.ColorAt(0.0)
	default:
		return gcr.paint.ColorAt(1.0)
	}
}

// computeLaneBoundaries calculates pixel start/end positions for each lane
// based on proportional weights. Zero/missing entries default to weight 1.
func computeLaneBoundaries(weights []int, numLanes, totalWidth int) (starts []int, ends []int) {
	starts = make([]int, numLanes)
	ends = make([]int, numLanes)

	totalWeight := 0
	for i := 0; i < numLanes; i++ {
		if i < len(weights) && weights[i] > 0 {
			totalWeight += weights[i]
		} else {
			totalWeight++
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
			ends[i] = totalWidth
		} else {
			ends[i] = pos + (totalWidth*w)/totalWeight
		}
		pos = ends[i]
	}
	return starts, ends
}
