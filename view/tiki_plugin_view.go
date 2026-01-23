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

// PluginView renders a filtered/sorted list of tasks across panes
type PluginView struct {
	root                *tview.Flex
	titleBar            tview.Primitive
	searchHelper        *SearchHelper
	panes               *tview.Flex
	paneBoxes           []*ScrollableList
	taskStore           store.Store
	pluginConfig        *model.PluginConfig
	pluginDef           *plugin.TikiPlugin
	registry            *controller.ActionRegistry
	storeListenerID     int
	selectionListenerID int
	getPaneTasks        func(pane int) []*task.Task // injected from controller
	ensureSelection     func() bool                 // injected from controller
}

// NewPluginView creates a plugin view
func NewPluginView(
	taskStore store.Store,
	pluginConfig *model.PluginConfig,
	pluginDef *plugin.TikiPlugin,
	getPaneTasks func(pane int) []*task.Task,
	ensureSelection func() bool,
) *PluginView {
	pv := &PluginView{
		taskStore:       taskStore,
		pluginConfig:    pluginConfig,
		pluginDef:       pluginDef,
		registry:        controller.PluginViewActions(),
		getPaneTasks:    getPaneTasks,
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
	titleGradient := pluginCaptionGradient(pv.pluginDef.Background, config.GetColors().BoardPaneTitleGradient)
	paneNames := make([]string, len(pv.pluginDef.Panes))
	for i, pane := range pv.pluginDef.Panes {
		paneNames[i] = pane.Name
	}
	pv.titleBar = NewGradientCaptionRow(paneNames, titleGradient, textColor)

	// panes container (rows)
	pv.panes = tview.NewFlex().SetDirection(tview.FlexColumn)
	pv.paneBoxes = make([]*ScrollableList, 0, len(pv.pluginDef.Panes))

	// search helper - focus returns to panes container
	pv.searchHelper = NewSearchHelper(pv.panes)
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
		pv.root.AddItem(pv.panes, 0, 1, false)
	} else {
		pv.root.AddItem(pv.panes, 0, 1, true)
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
	selectedPane := pv.pluginConfig.GetSelectedPane()

	if len(pv.paneBoxes) != len(pv.pluginDef.Panes) {
		pv.paneBoxes = make([]*ScrollableList, 0, len(pv.pluginDef.Panes))
		for range pv.pluginDef.Panes {
			pv.paneBoxes = append(pv.paneBoxes, NewScrollableList())
		}
	}

	pv.panes.Clear()

	for paneIdx := range pv.pluginDef.Panes {
		paneContainer := pv.paneBoxes[paneIdx]
		paneContainer.SetItemHeight(itemHeight)
		paneContainer.Clear()

		isSelectedPane := paneIdx == selectedPane
		pv.panes.AddItem(paneContainer, 0, 1, isSelectedPane)

		tasks := pv.getPaneTasks(paneIdx)
		if isSelectedPane {
			pv.pluginConfig.ClampSelection(len(tasks))
		}
		if len(tasks) == 0 {
			paneContainer.SetSelection(-1)
			continue
		}

		columns := pv.pluginConfig.GetColumnsForPane(paneIdx)
		selectedIndex := pv.pluginConfig.GetSelectedIndexForPane(paneIdx)
		selectedRow := selectedIndex / columns

		numRows := (len(tasks) + columns - 1) / columns
		for row := 0; row < numRows; row++ {
			rowFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
			for col := 0; col < columns; col++ {
				idx := row*columns + col
				if idx < len(tasks) {
					task := tasks[idx]
					isSelected := isSelectedPane && idx == selectedIndex
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
			paneContainer.AddItem(rowFlex)
		}

		if isSelectedPane {
			paneContainer.SetSelection(selectedRow)
		} else {
			paneContainer.SetSelection(-1)
		}
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
	pane := pv.pluginConfig.GetSelectedPane()
	tasks := pv.getPaneTasks(pane)
	idx := pv.pluginConfig.GetSelectedIndexForPane(pane)
	if idx >= 0 && idx < len(tasks) {
		return tasks[idx].ID
	}
	return ""
}

// SetSelectedID sets the selection to a task
func (pv *PluginView) SetSelectedID(id string) {
	for pane := range pv.pluginDef.Panes {
		tasks := pv.getPaneTasks(pane)
		for i, t := range tasks {
			if t.ID == id {
				pv.pluginConfig.SetSelectedPane(pane)
				pv.pluginConfig.SetSelectedIndexForPane(pane, i)
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
	pv.root.AddItem(pv.panes, 0, 1, false)

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
	pv.root.AddItem(pv.panes, 0, 1, true)
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
	for pane := range pv.pluginDef.Panes {
		tasks := pv.getPaneTasks(pane)
		total += len(tasks)
	}
	return []store.Stat{
		{Name: "Total", Value: fmt.Sprintf("%d", total), Order: 5},
	}
}
