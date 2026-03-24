package header

// layout constants for the header widget. kept here alongside the pure layout
// function so the algorithm and its inputs are co-located.
const (
	InfoWidth       = 40 // fixed width for info section (view name + description)
	ChartWidth      = 14 // fixed width for burndown chart
	LogoWidth       = 25 // fixed width for logo
	MinContextWidth = 40 // minimum width for context help to remain readable
	ChartSpacing    = 10 // spacing between context help and chart when both visible
)

// HeaderLayout is the pure output of the header layout calculation.
// It contains no tview types — just integers and a boolean.
type HeaderLayout struct {
	ContextWidth int
	ChartVisible bool
}

// CalculateHeaderLayout computes header component widths from two integer inputs.
//
// Rules:
//  1. availableBetween = totalWidth - InfoWidth - LogoWidth (clamped to 0)
//  2. requiredContext = max(contextHelpWidth, MinContextWidth) when contextHelpWidth > 0
//  3. chart is visible when availableBetween >= requiredContext + ChartSpacing + ChartWidth
//  4. contextWidth is clamped to availableBetween minus chart reservation when chart visible
func CalculateHeaderLayout(totalWidth, contextHelpWidth int) HeaderLayout {
	availableBetween := max(totalWidth-InfoWidth-LogoWidth, 0)

	requiredContext := contextHelpWidth
	if requiredContext < MinContextWidth && requiredContext > 0 {
		requiredContext = MinContextWidth
	}

	chartVisible := availableBetween >= requiredContext+ChartSpacing+ChartWidth

	contextWidth := max(contextHelpWidth, 0)

	usedForChart := 0
	if chartVisible {
		usedForChart = ChartSpacing + ChartWidth
	}

	maxContextWidth := max(availableBetween-usedForChart, 0)
	if contextWidth > maxContextWidth {
		contextWidth = maxContextWidth
	}

	return HeaderLayout{
		ContextWidth: contextWidth,
		ChartVisible: chartVisible,
	}
}
