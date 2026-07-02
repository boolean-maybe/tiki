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
//   - RightSpacer (flexible, pushes middle content to center)
//   - Logo (25 chars, fixed right)
//
// The two flexible spacers center ContextHelp between Stats and Logo.

const (
	HeaderHeight        = 6
	HeaderColumnSpacing = 2 // spaces between action columns in ContextHelp
)

// HeaderWidget displays view info and available actions.
type HeaderWidget struct {
	*tview.Flex

	// Components
	info        *InfoWidget
	contextHelp *ContextHelpWidget

	// Layout elements
	leftSpacer  *tview.Box
	rightSpacer *tview.Box
	logo        *tview.TextView

	// Model references
	headerConfig          *model.HeaderConfig
	viewContext           *model.ViewContext
	headerListenerID      int
	viewContextListenerID int

	// Layout state
	lastWidth int
}

// NewHeaderWidget creates a header widget that observes HeaderConfig (visibility)
// and ViewContext (view info + actions) for state.
func NewHeaderWidget(headerConfig *model.HeaderConfig, viewContext *model.ViewContext) *HeaderWidget {
	info := NewInfoWidget()
	contextHelp := NewContextHelpWidget()

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
		logo:         logo,
		rightSpacer:  tview.NewBox(),
		headerConfig: headerConfig,
		viewContext:  viewContext,
	}

	hw.headerListenerID = headerConfig.AddListener(hw.rebuild)
	if viewContext != nil {
		hw.viewContextListenerID = viewContext.AddListener(hw.rebuild)
	}

	hw.rebuild()
	hw.rebuildLayout(0)
	return hw
}

// rebuild reads data from ViewContext (view info + actions).
func (h *HeaderWidget) rebuild() {
	if h.viewContext != nil {
		h.info.SetViewInfo(h.viewContext.GetViewName(), h.viewContext.GetViewDescription())
		h.contextHelp.SetActionsFromModel(h.viewContext.GetGlobalActions(), h.viewContext.GetViewActions(), h.viewContext.GetPluginActions())
	}

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

// Cleanup removes all listeners
func (h *HeaderWidget) Cleanup() {
	h.headerConfig.RemoveListener(h.headerListenerID)
	if h.viewContext != nil {
		h.viewContext.RemoveListener(h.viewContextListenerID)
	}
}

// rebuildLayout recalculates and rebuilds the flex layout based on terminal width.
func (h *HeaderWidget) rebuildLayout(width int) {
	h.lastWidth = width

	layout := CalculateHeaderLayout(width, h.contextHelp.GetWidth())

	h.Clear()
	h.SetDirection(tview.FlexColumn)
	h.AddItem(h.info.Primitive(), InfoWidth, 0, false)
	h.AddItem(h.leftSpacer, 0, 1, false)
	h.AddItem(h.contextHelp.Primitive(), layout.ContextWidth, 0, false)
	h.AddItem(h.rightSpacer, 0, 1, false)
	h.AddItem(h.logo, LogoWidth, 0, false)
}
