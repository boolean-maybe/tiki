package barchart

import (
	"math"

	"github.com/gdamore/tcell/v2"
)

func drawBrailleBars(screen tcell.Screen, startX, chartBottom, chartHeight int, bars []Bar, maxValue float64, theme Theme) {
	if chartHeight <= 0 || len(bars) == 0 || maxValue <= 0 {
		return
	}

	barHeights := make([]int, len(bars))
	for i, bar := range bars {
		barHeights[i] = valueToBrailleHeight(bar.Value, maxValue, chartHeight)
	}

	cellCount := (len(bars) + 1) / 2
	for cell := 0; cell < cellCount; cell++ {
		leftIndex := cell * 2
		rightIndex := leftIndex + 1

		leftUnits := 0
		rightUnits := 0
		if leftIndex < len(barHeights) {
			leftUnits = barHeights[leftIndex]
		}
		if rightIndex < len(barHeights) {
			rightUnits = barHeights[rightIndex]
		}

		for row := 0; row < chartHeight; row++ {
			leftCount := brailleUnitsForRow(leftUnits, row)
			rightCount := brailleUnitsForRow(rightUnits, row)
			if leftCount == 0 && rightCount == 0 {
				continue
			}

			r := brailleRuneForCounts(leftCount, rightCount)
			barIndex, rowIndex, total := dominantBarForCell(leftIndex, rightIndex, leftUnits, rightUnits, leftCount, rightCount, row, barHeights)
			color := theme.BarColor
			if barIndex >= 0 && total > 0 {
				if barIndex < len(bars) {
					color = barFillColor(bars[barIndex], rowIndex, total, theme)
				}
			}
			style := tcell.StyleDefault.Foreground(color).Background(theme.BackgroundColor)
			screen.SetContent(startX+cell, chartBottom-row, r, nil, style)
		}
	}
}

func dominantBarForCell(leftIndex, rightIndex, leftUnits, rightUnits, leftCount, rightCount, row int, heights []int) (int, int, int) {
	rowTopUnit := row*4 + leftCount - 1
	if rightCount > leftCount {
		rowTopUnit = row*4 + rightCount - 1
	}

	switch {
	case rightCount > leftCount && rightIndex < len(heights):
		return rightIndex, clampRowIndex(rowTopUnit, heights[rightIndex]), heights[rightIndex]
	case leftCount > rightCount && leftIndex < len(heights):
		return leftIndex, clampRowIndex(rowTopUnit, heights[leftIndex]), heights[leftIndex]
	case leftCount == 0 && rightCount == 0:
		return -1, 0, 0
	default:
		switch {
		case rightUnits > leftUnits && rightIndex < len(heights):
			return rightIndex, clampRowIndex(rowTopUnit, heights[rightIndex]), heights[rightIndex]
		case leftIndex < len(heights):
			return leftIndex, clampRowIndex(rowTopUnit, heights[leftIndex]), heights[leftIndex]
		case rightIndex < len(heights):
			return rightIndex, clampRowIndex(rowTopUnit, heights[rightIndex]), heights[rightIndex]
		}
	}
	return -1, 0, 0
}

func clampRowIndex(rowIndex, total int) int {
	if total <= 0 {
		return 0
	}
	if rowIndex < 0 {
		return 0
	}
	if rowIndex >= total {
		return total - 1
	}
	return rowIndex
}

func valueToBrailleHeight(value, maxValue float64, chartHeight int) int {
	totalUnits := chartHeight * 4
	if totalUnits <= 0 || maxValue <= 0 {
		return 0
	}

	ratio := value / maxValue
	if ratio < 0 {
		ratio = 0
	}

	units := int(math.Round(ratio * float64(totalUnits)))
	if value > 0 && units == 0 {
		return 1
	}
	if units > totalUnits {
		return totalUnits
	}
	return units
}

func brailleUnitsForRow(totalUnits, row int) int {
	if totalUnits <= 0 {
		return 0
	}
	start := row * 4
	if totalUnits <= start {
		return 0
	}
	remaining := totalUnits - start
	if remaining > 4 {
		return 4
	}
	return remaining
}

func brailleColumnMask(level int, rightColumn bool) uint8 {
	if level <= 0 {
		return 0
	}

	if level > 4 {
		level = 4
	}

	var dots [4]uint8
	if rightColumn {
		dots = [4]uint8{0x80, 0x20, 0x10, 0x08} // 8,6,5,4 from bottom to top
	} else {
		dots = [4]uint8{0x40, 0x04, 0x02, 0x01} // 7,3,2,1 from bottom to top
	}

	mask := uint8(0)
	for i := 0; i < level; i++ {
		mask |= dots[i]
	}
	return mask
}

func brailleRuneForCounts(leftCount, rightCount int) rune {
	mask := brailleColumnMask(leftCount, false) | brailleColumnMask(rightCount, true)
	return rune(0x2800 + int(mask))
}
