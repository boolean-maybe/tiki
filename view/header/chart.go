package header

import (
	"github.com/boolean-maybe/tiki/component/barchart"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store"

	"github.com/rivo/tview"
)

// ChartWidget displays the burndown chart
type ChartWidget struct {
	*barchart.BarChart
}

// NewChartWidgetSimple creates a new burndown chart widget without initial data
func NewChartWidgetSimple() *ChartWidget {
	colors := config.GetColors()
	chartTheme := barchart.DefaultTheme()
	chartTheme.AxisColor = colors.BurndownChartAxisColor
	chartTheme.BarGradientFrom = colors.BurndownHeaderGradientFrom.Start // Use header-specific gradient
	chartTheme.BarGradientTo = colors.BurndownHeaderGradientTo.Start     // Use header-specific gradient
	chartTheme.DotChar = '⣿'                                             // braille full cell for compact dots
	chartTheme.DotRowGap = 0
	chartTheme.DotColGap = 0

	chart := barchart.NewBarChart().
		UseBraille().
		SetBarWidth(2).
		SetGapWidth(1).
		SetTheme(chartTheme).
		SetVerticalOffset(0).
		ShowAxis(false).
		ShowLabels(false)

	return &ChartWidget{
		BarChart: chart,
	}
}

// UpdateBurndown updates the chart with new burndown data
func (cw *ChartWidget) UpdateBurndown(points []store.BurndownPoint) {
	applyBurndown(cw.BarChart, points)
}

// Primitive returns the underlying tview primitive
func (cw *ChartWidget) Primitive() tview.Primitive {
	return cw.BarChart
}

// applyBurndown applies burndown data to the chart
func applyBurndown(chart *barchart.BarChart, burndown []store.BurndownPoint) {
	if len(burndown) == 0 {
		chart.SetMaxValue(1).SetBars([]barchart.Bar{})
		return
	}

	bars := make([]barchart.Bar, 0, len(burndown))
	maxVal := 0.0
	for _, point := range burndown {
		value := float64(point.Remaining)
		if value > maxVal {
			maxVal = value
		}
		bars = append(bars, barchart.Bar{
			Label: "",
			Value: value,
		})
	}
	if maxVal <= 0 {
		maxVal = 1
	}
	chart.SetMaxValue(maxVal).SetBars(bars)
}
