package barchart

import (
	"math"

	"github.com/gdamore/tcell/v2"
)

// Border character for axis line
const borderHorizontal = '─'

func drawAxis(screen tcell.Screen, x, y, width int, color, bgColor tcell.Color) {
	style := tcell.StyleDefault.Foreground(color).Background(bgColor)
	for col := 0; col < width; col++ {
		screen.SetContent(x+col, y, borderHorizontal, nil, style)
	}
}

func drawCenteredText(screen tcell.Screen, x, y, width int, text string, color, bgColor tcell.Color) {
	if width <= 0 {
		return
	}

	runes := []rune(text)
	if len(runes) > width {
		runes = runes[:width]
	}

	start := x + (width-len(runes))/2
	style := tcell.StyleDefault.Foreground(color).Background(bgColor)
	for i, r := range runes {
		screen.SetContent(start+i, y, r, nil, style)
	}
}

func valueToHeight(value, maxValue float64, chartHeight int) int {
	if chartHeight <= 0 || maxValue <= 0 {
		return 0
	}

	ratio := value / maxValue
	if ratio < 0 {
		ratio = 0
	}
	height := int(math.Round(ratio * float64(chartHeight)))

	if value > 0 && height == 0 {
		return 1
	}
	if height > chartHeight {
		return chartHeight
	}
	return height
}

func computeBarLayout(totalWidth, barCount, desiredBarWidth, desiredGap int) (int, int, int) {
	if totalWidth <= 0 || barCount <= 0 {
		return 0, 0, 0
	}

	barWidth := desiredBarWidth
	if barWidth < 1 {
		barWidth = 1
	}

	gapWidth := desiredGap
	if gapWidth < 0 {
		gapWidth = 0
	}

	contentWidth := barCount*barWidth + (barCount-1)*gapWidth
	if contentWidth <= totalWidth {
		return barWidth, gapWidth, contentWidth
	}

	barWidth = (totalWidth - (barCount-1)*gapWidth) / barCount
	if barWidth < 1 {
		barWidth = 1
	}

	contentWidth = barCount*barWidth + (barCount-1)*gapWidth
	if contentWidth <= totalWidth {
		return barWidth, gapWidth, contentWidth
	}

	if barCount > 1 {
		gapWidth = (totalWidth - barCount*barWidth) / (barCount - 1)
		if gapWidth < 0 {
			gapWidth = 0
		}
	}

	contentWidth = barCount*barWidth + (barCount-1)*gapWidth
	if contentWidth > totalWidth {
		contentWidth = totalWidth
	}

	return barWidth, gapWidth, contentWidth
}

func computeMaxVisibleBars(totalWidth, barWidth, gapWidth int) int {
	if totalWidth <= 0 || barWidth <= 0 {
		return 0
	}

	maxBars := (totalWidth + gapWidth) / (barWidth + gapWidth)
	if maxBars < 1 {
		return 1
	}
	return maxBars
}

func maxBarValue(bars []Bar) float64 {
	max := 0.0
	for _, bar := range bars {
		if bar.Value > max {
			max = bar.Value
		}
	}
	return max
}

func truncateRunes(text string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	return string(runes[:width])
}

func hasLabels(bars []Bar) bool {
	for _, b := range bars {
		if b.Label != "" {
			return true
		}
	}
	return false
}
