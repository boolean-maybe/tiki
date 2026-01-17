package header

import (
	"sort"

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
	HeaderColumnSpacing = 2  // spaces between action columns in ContextHelp
	StatsWidth          = 30 // fixed width for stats section
	ChartWidth          = 14 // fixed width for burndown chart
	LogoWidth           = 25 // fixed width for logo
	MinContextWidth     = 40 // minimum width for context help to remain readable
	ChartSpacing        = 10 // spacing between context help and chart when both visible
)

// HeaderWidget displays stats, available actions and burndown chart
type HeaderWidget struct {
	*tview.Flex

	// Components
	stats       *StatsWidget
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
	stats := NewStatsWidget()
	contextHelp := NewContextHelpWidget()
	chart := NewChartWidgetSimple() // No store dependency, data comes from HeaderConfig

	logo := tview.NewTextView()
	logo.SetDynamicColors(true)
	logo.SetTextAlign(tview.AlignLeft)
	logo.SetText(config.GetArtTView())

	flex := tview.NewFlex().SetDirection(tview.FlexColumn)

	hw := &HeaderWidget{
		Flex:         flex,
		stats:        stats,
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
	// Update stats from HeaderConfig
	stats := h.headerConfig.GetStats()
	h.rebuildStats(stats)

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

// rebuildStats updates the stats widget from HeaderConfig stats
func (h *HeaderWidget) rebuildStats(stats map[string]model.StatValue) {
	// Remove stats that are no longer in the config
	for _, key := range h.stats.GetKeys() {
		if _, exists := stats[key]; !exists {
			h.stats.RemoveStat(key)
		}
	}

	// Sort stats by priority for consistent ordering
	type statEntry struct {
		key      string
		value    string
		priority int
	}
	entries := make([]statEntry, 0, len(stats))
	for k, v := range stats {
		entries = append(entries, statEntry{key: k, value: v.Value, priority: v.Priority})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].priority < entries[j].priority
	})

	// Update stats widget
	for _, e := range entries {
		h.stats.AddStat(e.key, e.value, e.priority)
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

// rebuildLayout recalculates and rebuilds the flex layout based on terminal width
func (h *HeaderWidget) rebuildLayout(width int) {
	h.lastWidth = width

	availableBetween := width - StatsWidth - LogoWidth
	if availableBetween < 0 {
		availableBetween = 0
	}

	contextHelpWidth := h.contextHelp.GetWidth()
	requiredContext := contextHelpWidth
	if requiredContext < MinContextWidth && requiredContext > 0 {
		requiredContext = MinContextWidth
	}

	requiredForChart := requiredContext + ChartSpacing + ChartWidth
	chartVisible := availableBetween >= requiredForChart

	contextWidth := contextHelpWidth
	if contextWidth < 0 {
		contextWidth = 0
	}

	usedForChart := 0
	if chartVisible {
		usedForChart = ChartSpacing + ChartWidth
	}

	maxContextWidth := availableBetween - usedForChart
	if maxContextWidth < 0 {
		maxContextWidth = 0
	}
	if contextWidth > maxContextWidth {
		contextWidth = maxContextWidth
	}

	// rebuild flex to keep the middle group centered between stats and logo,
	// and to physically remove the chart when hidden.
	h.Clear()
	h.SetDirection(tview.FlexColumn)
	h.AddItem(h.stats.Primitive(), StatsWidth, 0, false)
	h.AddItem(h.leftSpacer, 0, 1, false)
	h.AddItem(h.contextHelp.Primitive(), contextWidth, 0, false)
	if chartVisible {
		h.AddItem(h.gap, ChartSpacing, 0, false)
		h.AddItem(h.chart.Primitive(), ChartWidth, 0, false)
	}
	h.AddItem(h.rightSpacer, 0, 1, false)
	h.AddItem(h.logo, LogoWidth, 0, false)

	h.chartVisible = chartVisible
}
