package barchart

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RenderMode controls how bars are drawn.
type RenderMode int

const (
	RenderSolid RenderMode = iota
	RenderDotMatrix
	RenderBraille
)

// Bar represents a single bar in the chart.
// Color is optional; set UseColor to true to override the theme.
type Bar struct {
	Label    string
	Value    float64
	Color    tcell.Color
	UseColor bool
}

// Theme defines colors and glyphs used by the chart.
type Theme struct {
	AxisColor       tcell.Color
	LabelColor      tcell.Color
	ValueColor      tcell.Color
	BarColor        tcell.Color
	BackgroundColor tcell.Color // background color for chart area
	BarGradientFrom [3]int
	BarGradientTo   [3]int
	DotChar         rune
	BarChar         rune
	DotRowGap       int
	DotColGap       int
}

// BarChart renders vertical bars with optional dot-matrix styling.
type BarChart struct {
	*tview.Box

	bars       []Bar
	renderMode RenderMode
	barWidth   int
	gapWidth   int
	maxValue   float64

	showAxis       bool
	verticalOffset int
	showLabels     bool
	showValues     bool
	valueFormatter func(float64) string
	theme          Theme
}

// DefaultTheme returns a gradient that mirrors the purple-to-blue palette from the example screenshot.
func DefaultTheme() Theme {
	colors := config.GetColors()
	return Theme{
		AxisColor:       colors.BurndownChartAxisColor,
		LabelColor:      colors.BurndownChartLabelColor,
		ValueColor:      colors.BurndownChartValueColor,
		BarColor:        colors.BurndownChartBarColor,
		BackgroundColor: config.GetContentBackgroundColor(),
		BarGradientFrom: colors.BurndownChartGradientFrom.Start,
		BarGradientTo:   colors.BurndownChartGradientTo.Start,
		DotChar:         '⣿', // braille full cell for dense dot matrix
		BarChar:         '█',
		DotRowGap:       0,
		DotColGap:       0,
	}
}

// NewBarChart builds a chart with sensible defaults and solid bars.
func NewBarChart() *BarChart {
	return &BarChart{
		Box:            tview.NewBox(),
		renderMode:     RenderSolid,
		barWidth:       4,
		gapWidth:       2,
		valueFormatter: func(v float64) string { return fmt.Sprintf("%.0f", v) },
		theme:          DefaultTheme(),
		showAxis:       true,
		verticalOffset: 0,
		showLabels:     true,
		showValues:     false,
		maxValue:       0,
		bars:           make([]Bar, 0),
	}
}

// SetBars replaces the bars to render.
func (c *BarChart) SetBars(bars []Bar) *BarChart {
	c.bars = append([]Bar(nil), bars...)
	return c
}

// SetRenderMode switches between solid and dot-matrix modes.
func (c *BarChart) SetRenderMode(mode RenderMode) *BarChart {
	c.renderMode = mode
	return c
}

// UseDotMatrix is a convenience setter for the dot-matrix style.
func (c *BarChart) UseDotMatrix() *BarChart {
	c.renderMode = RenderDotMatrix
	return c
}

// UseBraille enables dense braille rendering (2x horizontal points, 4x vertical resolution).
func (c *BarChart) UseBraille() *BarChart {
	c.renderMode = RenderBraille
	return c
}

// UseSolidBars is a convenience setter for solid bars.
func (c *BarChart) UseSolidBars() *BarChart {
	c.renderMode = RenderSolid
	return c
}

// SetBarWidth sets the column width for each bar.
func (c *BarChart) SetBarWidth(width int) *BarChart {
	if width < 1 {
		width = 1
	}
	c.barWidth = width
	return c
}

// SetGapWidth sets the gap between bars.
func (c *BarChart) SetGapWidth(width int) *BarChart {
	if width < 0 {
		width = 0
	}
	c.gapWidth = width
	return c
}

// SetMaxValue overrides the computed max value; set to 0 to auto-compute.
func (c *BarChart) SetMaxValue(max float64) *BarChart {
	c.maxValue = max
	return c
}

// SetTheme overrides the chart theme.
func (c *BarChart) SetTheme(theme Theme) *BarChart {
	c.theme = theme
	return c
}

// ShowValues toggles per-bar value labels above the bars.
func (c *BarChart) ShowValues(show bool) *BarChart {
	c.showValues = show
	return c
}

// ShowLabels toggles rendering of labels beneath the axis.
func (c *BarChart) ShowLabels(show bool) *BarChart {
	c.showLabels = show
	return c
}

// ShowAxis toggles rendering of the horizontal axis line.
func (c *BarChart) ShowAxis(show bool) *BarChart {
	c.showAxis = show
	return c
}

// SetVerticalOffset shifts the chart drawing origin vertically.
// Negative values move the chart up; positive values move it down.
func (c *BarChart) SetVerticalOffset(offset int) *BarChart {
	c.verticalOffset = offset
	return c
}

// SetValueFormatter customizes how values are rendered; nil keeps the default.
func (c *BarChart) SetValueFormatter(formatter func(float64) string) *BarChart {
	if formatter != nil {
		c.valueFormatter = formatter
	}
	return c
}

// Draw renders the chart within its bounding box.
func (c *BarChart) Draw(screen tcell.Screen) {
	c.DrawForSubclass(screen, c)

	x, y, width, height := c.GetInnerRect()
	if width <= 0 || height <= 0 || len(c.bars) == 0 {
		return
	}

	y += c.verticalOffset

	labelHeight := 0
	if c.showLabels && hasLabels(c.bars) {
		labelHeight = 1
	}
	axisHeight := 0
	if c.showAxis {
		axisHeight = 1
	}
	valueHeight := 0
	if c.showValues {
		valueHeight = 1
	}

	chartHeight := height - labelHeight - axisHeight - valueHeight
	if chartHeight <= 0 {
		return
	}

	barCount := len(c.bars)

	maxValue := c.maxValue
	if maxValue <= 0 {
		maxValue = maxBarValue(c.bars)
	}
	if maxValue <= 0 {
		return
	}

	chartTop := y + valueHeight
	chartBottom := chartTop + chartHeight - 1
	axisY := chartBottom
	labelY := chartBottom + 1
	if c.showAxis {
		axisY = chartBottom + 1
		labelY = axisY + 1
	}

	if c.renderMode == RenderBraille {
		maxBars := width * 2
		if maxBars < barCount {
			barCount = maxBars
		}
		if barCount == 0 {
			return
		}

		bars := c.bars[:barCount]
		contentWidth := (barCount + 1) / 2
		if contentWidth > width {
			contentWidth = width
		}

		startX := x + (width-contentWidth)/2
		if startX < x {
			startX = x
		}

		if c.showAxis {
			drawAxis(screen, startX, axisY, contentWidth, c.theme.AxisColor, c.theme.BackgroundColor)
		}

		drawBrailleBars(screen, startX, chartBottom, chartHeight, bars, maxValue, c.theme)
		return
	}

	barWidth, gapWidth, _ := computeBarLayout(width, barCount, c.barWidth, c.gapWidth)
	if barWidth == 0 {
		return
	}

	maxBars := computeMaxVisibleBars(width, barWidth, gapWidth)
	if maxBars < barCount {
		barCount = maxBars
	}
	bars := c.bars[:barCount]
	contentWidth := barCount*barWidth + gapWidth*(barCount-1)
	if contentWidth > width {
		contentWidth = width
	}

	startX := x + (width-contentWidth)/2
	if startX < x {
		startX = x
	}

	if c.showAxis {
		drawAxis(screen, startX, axisY, contentWidth, c.theme.AxisColor, c.theme.BackgroundColor)
	}

	for i, bar := range bars {
		barX := startX + i*(barWidth+gapWidth)
		heightForBar := valueToHeight(bar.Value, maxValue, chartHeight)

		if c.showValues {
			valueText := c.valueFormatter(bar.Value)
			drawCenteredText(screen, barX, y, barWidth, valueText, c.theme.ValueColor, c.theme.BackgroundColor)
		}

		if heightForBar > 0 {
			if c.renderMode == RenderDotMatrix {
				drawBarDots(screen, barX, chartBottom, barWidth, heightForBar, bar, c.theme)
			} else {
				drawBarSolid(screen, barX, chartBottom, barWidth, heightForBar, bar, c.theme)
			}
		}

		if labelHeight > 0 {
			drawCenteredText(screen, barX, labelY, barWidth, truncateRunes(bar.Label, barWidth), c.theme.LabelColor, c.theme.BackgroundColor)
		}
	}
}
