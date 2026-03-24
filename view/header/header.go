package header

import (
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SetActionsParams holds parameters for SetActionsWithPlugins
type SetActionsParams struct {
	ViewActions   []model.HeaderAction // view-specific actions (from HeaderConfig)
	PluginActions []model.HeaderAction // plugin navigation actions (from HeaderConfig)
}

// HeaderWidget renders the action bar at the top of each view.
//
// LAYOUT ALGORITHM:
// The header uses a responsive layout with manual centering:
//
// Components (left to right):
//   - Stats (30 chars, fixed left)
//   - LeftSpacer (flexible, pushes middle content to center)
//   - ContextHelp (calculated width based on actions)
//   - Gap (10 chars, only when chart visible)
//   - Chart (14 chars, hidden when terminal too narrow)
//   - RightSpacer (flexible, pushes middle content to center)
//   - Logo (25 chars, fixed right)
//
// The two flexible spacers center the middle content (ContextHelp + Gap + Chart)
// between Stats and Logo.
//
// When terminal width < 119 chars:
//   - Chart and Gap are hidden (width=0)
//   - Just ContextHelp is centered between Stats and Logo

const (
	HeaderHeight        = 6
	HeaderColumnSpacing = 2 // spaces between action columns in ContextHelp
)

// HeaderWidget displays view info, available actions and burndown chart
type HeaderWidget struct {
	*tview.Flex

	// Components
	info        *InfoWidget
	contextHelp *ContextHelpWidget
	chart       *ChartWidget

	// Layout elements
	leftSpacer  *tview.Box
	gap         *tview.Box
	rightSpacer *tview.Box
	logo        *tview.TextView

	// Model reference
	headerConfig *model.HeaderConfig
	listenerID   int

	// Layout state
	lastWidth    int
	chartVisible bool
}

// NewHeaderWidget creates a header widget that observes HeaderConfig for all state
func NewHeaderWidget(headerConfig *model.HeaderConfig) *HeaderWidget {
	info := NewInfoWidget()
	contextHelp := NewContextHelpWidget()
	chart := NewChartWidgetSimple() // No store dependency, data comes from HeaderConfig

	logo := tview.NewTextView()
	logo.SetDynamicColors(true)
	logo.SetTextAlign(tview.AlignLeft)
	logo.SetText(config.GetArtTView())

	flex := tview.NewFlex().SetDirection(tview.FlexColumn)

	hw := &HeaderWidget{
		Flex:         flex,
		info:         info,
		leftSpacer:   tview.NewBox(),
		contextHelp:  contextHelp,
		gap:          tview.NewBox(),
		logo:         logo,
		chart:        chart,
		rightSpacer:  tview.NewBox(),
		headerConfig: headerConfig,
	}

	// Subscribe to header config changes
	hw.listenerID = headerConfig.AddListener(hw.rebuild)

	hw.rebuild()
	hw.rebuildLayout(0)
	return hw
}

// rebuild reads all data from HeaderConfig and updates display
func (h *HeaderWidget) rebuild() {
	// Update view info from HeaderConfig
	h.info.SetViewInfo(h.headerConfig.GetViewName(), h.headerConfig.GetViewDescription())

	// Update burndown chart from HeaderConfig
	burndown := h.headerConfig.GetBurndown()
	h.chart.UpdateBurndown(burndown)

	// Update context help from HeaderConfig
	viewActions := h.headerConfig.GetViewActions()
	pluginActions := h.headerConfig.GetPluginActions()
	h.contextHelp.SetActionsFromModel(viewActions, pluginActions)

	if h.lastWidth > 0 {
		h.rebuildLayout(h.lastWidth)
	}
}

// Draw overrides to implement responsive layout
func (h *HeaderWidget) Draw(screen tcell.Screen) {
	_, _, width, _ := h.GetRect()
	if width != h.lastWidth {
		h.rebuildLayout(width)
	}
	h.Flex.Draw(screen)
}

// Cleanup removes the listener from HeaderConfig
func (h *HeaderWidget) Cleanup() {
	h.headerConfig.RemoveListener(h.listenerID)
}

// rebuildLayout recalculates and rebuilds the flex layout based on terminal width.
func (h *HeaderWidget) rebuildLayout(width int) {
	h.lastWidth = width

	layout := CalculateHeaderLayout(width, h.contextHelp.GetWidth())

	// rebuild flex to keep the middle group centered between stats and logo,
	// and to physically remove the chart when hidden.
	h.Clear()
	h.SetDirection(tview.FlexColumn)
	h.AddItem(h.info.Primitive(), InfoWidth, 0, false)
	h.AddItem(h.leftSpacer, 0, 1, false)
	h.AddItem(h.contextHelp.Primitive(), layout.ContextWidth, 0, false)
	if layout.ChartVisible {
		h.AddItem(h.gap, ChartSpacing, 0, false)
		h.AddItem(h.chart.Primitive(), ChartWidth, 0, false)
	}
	h.AddItem(h.rightSpacer, 0, 1, false)
	h.AddItem(h.logo, LogoWidth, 0, false)

	h.chartVisible = layout.ChartVisible
}
