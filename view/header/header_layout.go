package header

// layout constants for the header widget. kept here alongside the pure layout
// function so the algorithm and its inputs are co-located.
const (
	InfoWidth       = 40 // fixed width for info section (view name + description)
	LogoWidth       = 25 // fixed width for logo
	MinContextWidth = 40 // minimum width for context help to remain readable
)

// HeaderLayout is the pure output of the header layout calculation.
// It contains no tview types — just the computed context-help width.
type HeaderLayout struct {
	ContextWidth int
}

// CalculateHeaderLayout computes header component widths from two integer inputs.
//
// Rules:
//  1. availableBetween = totalWidth - InfoWidth - LogoWidth (clamped to 0)
//  2. contextWidth is clamped to availableBetween
func CalculateHeaderLayout(totalWidth, contextHelpWidth int) HeaderLayout {
	availableBetween := max(totalWidth-InfoWidth-LogoWidth, 0)

	contextWidth := max(contextHelpWidth, 0)
	if contextWidth > availableBetween {
		contextWidth = availableBetween
	}

	return HeaderLayout{
		ContextWidth: contextWidth,
	}
}
