package barchart

import (
	"github.com/boolean-maybe/tiki/util/gradient"
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
	rgb := gradient.InterpolateRGB(theme.BarGradientFrom, theme.BarGradientTo, t)
	//nolint:gosec // G115: RGB values are 0-255, safe to convert to int32
	return tcell.NewRGBColor(int32(rgb[0]), int32(rgb[1]), int32(rgb[2]))
}
