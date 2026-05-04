package view

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	"github.com/rivo/tview"
)

// PluginView renders a filtered/sorted list of tasks across lanes
type PluginView struct {
	root                *tview.Flex
	titleBar            tview.Primitive
	inputHelper         *InputHelper
	lanes               *tview.Flex
	laneBoxes           []*ScrollableList
	taskStore           store.Store
	pluginConfig        *model.PluginConfig
	pluginDef           *plugin.TikiPlugin
	registry            *controller.ActionRegistry
	showNavigation      bool
	storeListenerID     int
	selectionListenerID int
	getLaneTasks        func(lane int) []*tikipkg.Tiki // injected from controller
	ensureSelection     func() bool                    // injected from controller
	actionChangeHandler func()
}

// NewPluginView creates a plugin view
func NewPluginView(
	taskStore store.Store,
	pluginConfig *model.PluginConfig,
	pluginDef *plugin.TikiPlugin,
	getLaneTasks func(lane int) []*tikipkg.Tiki,
	ensureSelection func() bool,
	registry *controller.ActionRegistry,
	showNavigation bool,
) *PluginView {
	pv := &PluginView{
		taskStore:       taskStore,
		pluginConfig:    pluginConfig,
		pluginDef:       pluginDef,
		registry:        registry,
		showNavigation:  showNavigation,
		getLaneTasks:    getLaneTasks,
		ensureSelection: ensureSelection,
	}

	pv.build()

	return pv
}

func (pv *PluginView) build() {
	// title bar with gradient background using theme-derived caption colors
	colors := config.GetColors()
	pair := colors.CaptionColorForIndex(pv.pluginDef.ConfigIndex)
	bgColor := pair.Background
	textColor := pair.Foreground
	if pv.pluginDef.ConfigIndex < 0 {
		// code-only plugin (e.g. deps editor) — use explicit Background
		bgColor = pv.pluginDef.Background
		textColor = config.DefaultColor()
	}
	laneNames := make([]string, len(pv.pluginDef.Lanes))
	for i, lane := range pv.pluginDef.Lanes {
		laneNames[i] = lane.Name
	}
	laneWidths := make([]int, len(pv.pluginDef.Lanes))
	for i := range pv.pluginDef.Lanes {
		laneWidths[i] = pv.pluginConfig.GetWidthForLane(i)
	}
	pv.titleBar = NewGradientCaptionRow(laneNames, laneWidths, bgColor, textColor)

	// lanes container (rows)
	pv.lanes = tview.NewFlex().SetDirection(tview.FlexColumn)
	pv.laneBoxes = make([]*ScrollableList, 0, len(pv.pluginDef.Lanes))

	// input helper - focus returns to lanes container
	pv.inputHelper = NewInputHelper(pv.lanes)
	pv.inputHelper.SetCancelHandler(func() {
		pv.cancelCurrentInput()
	})
	pv.inputHelper.SetCloseHandler(func() {
		pv.removeInputBoxFromLayout()
	})
	pv.inputHelper.SetRestorePassiveHandler(func(_ string) {
		// layout already has the input box; no rebuild needed
	})

	// root layout
	pv.root = tview.NewFlex().SetDirection(tview.FlexRow)
	pv.rebuildLayout()

	pv.refresh()
}

// rebuildLayout rebuilds the root layout based on current state
func (pv *PluginView) rebuildLayout() {
	pv.root.Clear()
	pv.root.AddItem(pv.titleBar, 1, 0, false)

	if pv.inputHelper.IsVisible() {
		pv.root.AddItem(pv.inputHelper.GetInputBox(), config.InputBoxHeight, 0, false)
		pv.root.AddItem(pv.lanes, 0, 1, false)
	} else if pv.pluginConfig.IsSearchActive() {
		query := pv.pluginConfig.GetSearchQuery()
		pv.inputHelper.Show("> ", query, inputModeSearchPassive)
		pv.root.AddItem(pv.inputHelper.GetInputBox(), config.InputBoxHeight, 0, false)
		pv.root.AddItem(pv.lanes, 0, 1, false)
	} else {
		pv.root.AddItem(pv.lanes, 0, 1, true)
	}
}

func (pv *PluginView) refresh() {
	viewMode := pv.pluginConfig.GetViewMode()
	if pv.ensureSelection != nil {
		pv.ensureSelection()
	}

	// update item height based on view mode
	itemHeight := config.TaskBoxHeight
	if viewMode == model.ViewModeExpanded {
		itemHeight = config.TaskBoxHeightExpanded
	}
	selectedLane := pv.pluginConfig.GetSelectedLane()

	if len(pv.laneBoxes) != len(pv.pluginDef.Lanes) {
		pv.laneBoxes = make([]*ScrollableList, 0, len(pv.pluginDef.Lanes))
		for range pv.pluginDef.Lanes {
			pv.laneBoxes = append(pv.laneBoxes, NewScrollableList())
		}
	}

	pv.lanes.Clear()

	for laneIdx := range pv.pluginDef.Lanes {
		laneContainer := pv.laneBoxes[laneIdx]
		laneContainer.SetItemHeight(itemHeight)
		laneContainer.Clear()

		isSelectedLane := laneIdx == selectedLane
		pv.lanes.AddItem(laneContainer, 0, pv.pluginConfig.GetWidthForLane(laneIdx), isSelectedLane)

		tasks := pv.getLaneTasks(laneIdx)
		if isSelectedLane {
			pv.pluginConfig.ClampSelection(len(tasks))
		}
		if len(tasks) == 0 {
			laneContainer.SetSelection(-1)
			continue
		}

		columns := pv.pluginConfig.GetColumnsForLane(laneIdx)
		selectedIndex := pv.pluginConfig.GetSelectedIndexForLane(laneIdx)
		selectedRow := selectedIndex / columns

		numRows := (len(tasks) + columns - 1) / columns
		for row := 0; row < numRows; row++ {
			rowFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
			for col := 0; col < columns; col++ {
				idx := row*columns + col
				if idx < len(tasks) {
					task := tasks[idx]
					isSelected := isSelectedLane && idx == selectedIndex
					var taskBox *tview.Frame
					if viewMode == model.ViewModeCompact {
						taskBox = CreateCompactTaskBox(task, isSelected, config.GetColors())
					} else {
						taskBox = CreateExpandedTaskBox(task, isSelected, config.GetColors())
					}
					rowFlex.AddItem(taskBox, 0, 1, false)
				} else {
					spacer := tview.NewBox()
					rowFlex.AddItem(spacer, 0, 1, false)
				}
			}
			laneContainer.AddItem(rowFlex)
		}

		if isSelectedLane {
			laneContainer.SetSelection(selectedRow)
		} else {
			laneContainer.SetSelection(-1)
			laneContainer.ResetScrollOffset()
		}

		// Sync scroll offset from view to model for later lane navigation
		pv.pluginConfig.SetScrollOffsetForLane(laneIdx, laneContainer.GetScrollOffset())
	}

	if pv.actionChangeHandler != nil {
		pv.actionChangeHandler()
	}
}

func (pv *PluginView) GetSelectedID() string {
	lane := pv.pluginConfig.GetSelectedLane()
	tasks := pv.getLaneTasks(lane)
	idx := pv.pluginConfig.GetSelectedIndexForLane(lane)
	if idx < 0 || idx >= len(tasks) {
		return ""
	}
	return tasks[idx].ID
}

func (pv *PluginView) SetSelectedID(id string) {
	for lane := range pv.pluginDef.Lanes {
		for i, t := range pv.getLaneTasks(lane) {
			if t.ID == id {
				pv.pluginConfig.SetSelectedLane(lane)
				pv.pluginConfig.SetSelectedIndexForLane(lane, i)
				return
			}
		}
	}
}

func (pv *PluginView) SetActionChangeHandler(handler func()) {
	pv.actionChangeHandler = handler
}

// GetPrimitive returns the root tview primitive
func (pv *PluginView) GetPrimitive() tview.Primitive {
	return pv.root
}

// GetActionRegistry returns the view's action registry
func (pv *PluginView) GetActionRegistry() *controller.ActionRegistry {
	return pv.registry
}

// ShowNavigation returns whether plugin navigation keys should be shown in the header.
func (pv *PluginView) ShowNavigation() bool { return pv.showNavigation }

// GetViewName returns the plugin name for the header info section
func (pv *PluginView) GetViewName() string { return pv.pluginDef.GetName() }

// GetViewDescription returns the plugin description for the header info section
func (pv *PluginView) GetViewDescription() string { return pv.pluginDef.GetDescription() }

// GetViewID returns the view identifier
func (pv *PluginView) GetViewID() model.ViewID {
	return model.MakePluginViewID(pv.pluginDef.Name)
}

// OnFocus is called when the view becomes active
func (pv *PluginView) OnFocus() {
	pv.storeListenerID = pv.taskStore.AddListener(pv.refresh)
	pv.selectionListenerID = pv.pluginConfig.AddSelectionListener(pv.refresh)
	pv.refresh()
}

// OnBlur is called when the view becomes inactive
func (pv *PluginView) OnBlur() {
	pv.taskStore.RemoveListener(pv.storeListenerID)
	pv.pluginConfig.RemoveSelectionListener(pv.selectionListenerID)
}

// ShowInputBox displays the input box with the given prompt and initial text.
// If search is currently passive, action-input temporarily replaces it.
func (pv *PluginView) ShowInputBox(prompt, initial string) tview.Primitive {
	wasVisible := pv.inputHelper.IsVisible()

	inputBox := pv.inputHelper.Show(prompt, initial, inputModeActionInput)

	if !wasVisible {
		pv.root.Clear()
		pv.root.AddItem(pv.titleBar, 1, 0, false)
		pv.root.AddItem(pv.inputHelper.GetInputBox(), config.InputBoxHeight, 0, true)
		pv.root.AddItem(pv.lanes, 0, 1, false)
	}

	return inputBox
}

// ShowSearchBox opens the input box in search-editing mode.
func (pv *PluginView) ShowSearchBox() tview.Primitive {
	inputBox := pv.inputHelper.ShowSearch("")

	pv.root.Clear()
	pv.root.AddItem(pv.titleBar, 1, 0, false)
	pv.root.AddItem(pv.inputHelper.GetInputBox(), config.InputBoxHeight, 0, true)
	pv.root.AddItem(pv.lanes, 0, 1, false)

	return inputBox
}

// HideInputBox hides the input box without touching search state.
func (pv *PluginView) HideInputBox() {
	if !pv.inputHelper.IsVisible() {
		return
	}
	pv.inputHelper.Hide()
	pv.removeInputBoxFromLayout()
}

// removeInputBoxFromLayout rebuilds the layout without the input box and restores focus.
func (pv *PluginView) removeInputBoxFromLayout() {
	pv.root.Clear()
	pv.root.AddItem(pv.titleBar, 1, 0, false)
	pv.root.AddItem(pv.lanes, 0, 1, true)

	if pv.inputHelper.GetFocusSetter() != nil {
		pv.inputHelper.GetFocusSetter()(pv.lanes)
	}
}

// cancelCurrentInput handles Esc based on the current input mode.
func (pv *PluginView) cancelCurrentInput() {
	switch pv.inputHelper.Mode() {
	case inputModeSearchEditing, inputModeSearchPassive:
		pv.inputHelper.Hide()
		pv.pluginConfig.ClearSearchResults()
		pv.removeInputBoxFromLayout()
	case inputModeActionInput:
		// finishInput handles restore-passive-search vs full-close
		pv.inputHelper.finishInput()
	default:
		pv.inputHelper.Hide()
		pv.removeInputBoxFromLayout()
	}
}

// CancelInputBox triggers mode-aware cancel from the router
func (pv *PluginView) CancelInputBox() {
	pv.cancelCurrentInput()
}

// IsInputBoxVisible returns whether the input box is currently visible
func (pv *PluginView) IsInputBoxVisible() bool {
	return pv.inputHelper.IsVisible()
}

// IsInputBoxFocused returns whether the input box currently has focus
func (pv *PluginView) IsInputBoxFocused() bool {
	return pv.inputHelper.HasFocus()
}

// IsSearchPassive returns true if search is applied and the input box is passive
func (pv *PluginView) IsSearchPassive() bool {
	return pv.inputHelper.IsSearchPassive()
}

// SetInputSubmitHandler sets the callback for when input is submitted
func (pv *PluginView) SetInputSubmitHandler(handler func(text string) controller.InputSubmitResult) {
	pv.inputHelper.SetSubmitHandler(handler)
}

// SetInputCancelHandler sets the callback for when input is cancelled
func (pv *PluginView) SetInputCancelHandler(handler func()) {
	pv.inputHelper.SetCancelHandler(handler)
}

// SetFocusSetter sets the callback for requesting focus changes
func (pv *PluginView) SetFocusSetter(setter func(p tview.Primitive)) {
	pv.inputHelper.SetFocusSetter(setter)
}

// GetStats returns stats for the header and statusline (Total count of filtered tasks)
func (pv *PluginView) GetStats() []store.Stat {
	total := 0
	for lane := range pv.pluginDef.Lanes {
		tasks := pv.getLaneTasks(lane)
		total += len(tasks)
	}
	return []store.Stat{
		{Name: "Total", Value: fmt.Sprintf("%d", total), Order: 5},
	}
}
