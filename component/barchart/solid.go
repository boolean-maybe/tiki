package barchart

import (
	"math"

	"github.com/gdamore/tcell/v2"
)

func drawBarSolid(screen tcell.Screen, x, bottomY, width, height int, bar Bar, theme Theme) {
	for row := 0; row < height; row++ {
		color := barFillColor(bar, row, height, theme)
		style := tcell.StyleDefault.Foreground(color).Background(theme.BackgroundColor)
		y := bottomY - row
		for col := 0; col < width; col++ {
			screen.SetContent(x+col, y, theme.BarChar, nil, style)
		}
	}
}

func drawBarDots(screen tcell.Screen, x, bottomY, width, height int, bar Bar, theme Theme) {
	for row := 0; row < height; row++ {
		if theme.DotRowGap > 0 && row%(theme.DotRowGap+1) != 0 {
			continue
		}
		color := barFillColor(bar, row, height, theme)
		style := tcell.StyleDefault.Foreground(color).Background(theme.BackgroundColor)
		y := bottomY - row
		for col := 0; col < width; col++ {
			if theme.DotColGap > 0 && col%(theme.DotColGap+1) != 0 {
				continue
			}
			screen.SetContent(x+col, y, theme.DotChar, nil, style)
		}
	}
}

func barFillColor(bar Bar, row, total int, theme Theme) tcell.Color {
	if bar.UseColor {
		return bar.Color
	}
	if total <= 1 {
		return theme.BarColor
	}

	t := float64(row) / float64(total-1)
	rgb := interpolateRGB(theme.BarGradientFrom, theme.BarGradientTo, t)
	//nolint:gosec // G115: RGB values are 0-255, safe to convert to int32
	return tcell.NewRGBColor(int32(rgb[0]), int32(rgb[1]), int32(rgb[2]))
}

func interpolateRGB(from, to [3]int, t float64) [3]int {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	r := int(math.Round(float64(from[0]) + (float64(to[0])-float64(from[0]))*t))
	g := int(math.Round(float64(from[1]) + (float64(to[1])-float64(from[1]))*t))
	b := int(math.Round(float64(from[2]) + (float64(to[2])-float64(from[2]))*t))

	return [3]int{r, g, b}
}
