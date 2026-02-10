package view

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Note: tcell import is still used for pv.pluginDef.Background/Foreground checks

// PluginView renders a filtered/sorted list of tasks across lanes
type PluginView struct {
	root                *tview.Flex
	titleBar            tview.Primitive
	searchHelper        *SearchHelper
	lanes               *tview.Flex
	laneBoxes           []*ScrollableList
	taskStore           store.Store
	pluginConfig        *model.PluginConfig
	pluginDef           *plugin.TikiPlugin
	registry            *controller.ActionRegistry
	storeListenerID     int
	selectionListenerID int
	getLaneTasks        func(lane int) []*task.Task // injected from controller
	ensureSelection     func() bool                 // injected from controller
}

// NewPluginView creates a plugin view
func NewPluginView(
	taskStore store.Store,
	pluginConfig *model.PluginConfig,
	pluginDef *plugin.TikiPlugin,
	getLaneTasks func(lane int) []*task.Task,
	ensureSelection func() bool,
	registry *controller.ActionRegistry,
) *PluginView {
	pv := &PluginView{
		taskStore:       taskStore,
		pluginConfig:    pluginConfig,
		pluginDef:       pluginDef,
		registry:        registry,
		getLaneTasks:    getLaneTasks,
		ensureSelection: ensureSelection,
	}

	pv.build()

	return pv
}

func (pv *PluginView) build() {
	// title bar with gradient background using plugin color
	textColor := tcell.ColorDefault
	if pv.pluginDef.Foreground != tcell.ColorDefault {
		textColor = pv.pluginDef.Foreground
	}
	laneNames := make([]string, len(pv.pluginDef.Lanes))
	for i, lane := range pv.pluginDef.Lanes {
		laneNames[i] = lane.Name
	}
	pv.titleBar = NewGradientCaptionRow(laneNames, pv.pluginDef.Background, textColor)

	// lanes container (rows)
	pv.lanes = tview.NewFlex().SetDirection(tview.FlexColumn)
	pv.laneBoxes = make([]*ScrollableList, 0, len(pv.pluginDef.Lanes))

	// search helper - focus returns to lanes container
	pv.searchHelper = NewSearchHelper(pv.lanes)
	pv.searchHelper.SetCancelHandler(func() {
		pv.HideSearch()
	})

	// root layout
	pv.root = tview.NewFlex().SetDirection(tview.FlexRow)
	pv.rebuildLayout()

	pv.refresh()
}

// rebuildLayout rebuilds the root layout based on current state (search visibility)
func (pv *PluginView) rebuildLayout() {
	pv.root.Clear()
	pv.root.AddItem(pv.titleBar, 1, 0, false)

	// Restore search box if search is active (e.g., returning from task details)
	if pv.pluginConfig.IsSearchActive() {
		query := pv.pluginConfig.GetSearchQuery()
		pv.searchHelper.ShowSearch(query)
		pv.root.AddItem(pv.searchHelper.GetSearchBox(), config.SearchBoxHeight, 0, false)
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
		pv.lanes.AddItem(laneContainer, 0, 1, isSelectedLane)

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
		}

		// Sync scroll offset from view to model for later lane navigation
		pv.pluginConfig.SetScrollOffsetForLane(laneIdx, laneContainer.GetScrollOffset())
	}
}

// GetPrimitive returns the root tview primitive
func (pv *PluginView) GetPrimitive() tview.Primitive {
	return pv.root
}

// GetActionRegistry returns the view's action registry
func (pv *PluginView) GetActionRegistry() *controller.ActionRegistry {
	return pv.registry
}

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

// GetSelectedID returns the selected task ID
func (pv *PluginView) GetSelectedID() string {
	lane := pv.pluginConfig.GetSelectedLane()
	tasks := pv.getLaneTasks(lane)
	idx := pv.pluginConfig.GetSelectedIndexForLane(lane)
	if idx >= 0 && idx < len(tasks) {
		return tasks[idx].ID
	}
	return ""
}

// SetSelectedID sets the selection to a task
func (pv *PluginView) SetSelectedID(id string) {
	for lane := range pv.pluginDef.Lanes {
		tasks := pv.getLaneTasks(lane)
		for i, t := range tasks {
			if t.ID == id {
				pv.pluginConfig.SetSelectedLane(lane)
				pv.pluginConfig.SetSelectedIndexForLane(lane, i)
				return
			}
		}
	}
}

// ShowSearch displays the search box and returns the primitive to focus
func (pv *PluginView) ShowSearch() tview.Primitive {
	if pv.searchHelper.IsVisible() {
		return pv.searchHelper.GetSearchBox()
	}

	query := pv.pluginConfig.GetSearchQuery()
	searchBox := pv.searchHelper.ShowSearch(query)

	// Rebuild layout with search box
	pv.root.Clear()
	pv.root.AddItem(pv.titleBar, 1, 0, false)
	pv.root.AddItem(pv.searchHelper.GetSearchBox(), config.SearchBoxHeight, 0, true)
	pv.root.AddItem(pv.lanes, 0, 1, false)

	return searchBox
}

// HideSearch hides the search box and clears search results
func (pv *PluginView) HideSearch() {
	if !pv.searchHelper.IsVisible() {
		return
	}

	pv.searchHelper.HideSearch()

	// Clear search results (restores pre-search selection)
	pv.pluginConfig.ClearSearchResults()

	// Rebuild layout without search box
	pv.root.Clear()
	pv.root.AddItem(pv.titleBar, 1, 0, false)
	pv.root.AddItem(pv.lanes, 0, 1, true)
}

// IsSearchVisible returns whether the search box is currently visible
func (pv *PluginView) IsSearchVisible() bool {
	return pv.searchHelper.IsVisible()
}

// IsSearchBoxFocused returns whether the search box currently has focus
func (pv *PluginView) IsSearchBoxFocused() bool {
	return pv.searchHelper.HasFocus()
}

// SetSearchSubmitHandler sets the callback for when search is submitted
func (pv *PluginView) SetSearchSubmitHandler(handler func(text string)) {
	pv.searchHelper.SetSubmitHandler(handler)
}

// SetFocusSetter sets the callback for requesting focus changes
func (pv *PluginView) SetFocusSetter(setter func(p tview.Primitive)) {
	pv.searchHelper.SetFocusSetter(setter)
}

// GetStats returns stats for the header (Total count of filtered tasks)
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
