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

// PluginView renders a filtered/sorted list of tasks in a 4-column grid
type PluginView struct {
	root                *tview.Flex
	titleBar            *tview.TextView
	searchHelper        *SearchHelper
	grid                *ScrollableList // rows container
	taskStore           store.Store
	pluginConfig        *model.PluginConfig
	pluginDef           *plugin.TikiPlugin
	registry            *controller.ActionRegistry
	storeListenerID     int
	selectionListenerID int
	getFilteredTasks    func() []*task.Task // injected from controller
}

// NewPluginView creates a plugin view
func NewPluginView(
	taskStore store.Store,
	pluginConfig *model.PluginConfig,
	pluginDef *plugin.TikiPlugin,
	getFilteredTasks func() []*task.Task,
) *PluginView {
	pv := &PluginView{
		taskStore:        taskStore,
		pluginConfig:     pluginConfig,
		pluginDef:        pluginDef,
		registry:         controller.PluginViewActions(),
		getFilteredTasks: getFilteredTasks,
	}

	pv.build()

	return pv
}

func (pv *PluginView) build() {
	// title bar with plugin colors
	pv.titleBar = tview.NewTextView().
		SetText(pv.pluginDef.Name).
		SetTextAlign(tview.AlignCenter)

	// Apply plugin colors
	if pv.pluginDef.Background != tcell.ColorDefault {
		pv.titleBar.SetBackgroundColor(pv.pluginDef.Background)
	}
	if pv.pluginDef.Foreground != tcell.ColorDefault {
		pv.titleBar.SetTextColor(pv.pluginDef.Foreground)
	}

	// determine item height based on view mode
	itemHeight := config.TaskBoxHeight
	if pv.pluginConfig.GetViewMode() == model.ViewModeExpanded {
		itemHeight = config.TaskBoxHeightExpanded
	}

	// grid container (rows)
	pv.grid = NewScrollableList().SetItemHeight(itemHeight)

	// search helper - focus returns to grid
	pv.searchHelper = NewSearchHelper(pv.grid)
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
		pv.root.AddItem(pv.grid, 0, 1, false)
	} else {
		pv.root.AddItem(pv.grid, 0, 1, true)
	}
}

func (pv *PluginView) refresh() {
	viewMode := pv.pluginConfig.GetViewMode()

	// update item height based on view mode
	itemHeight := config.TaskBoxHeight
	if viewMode == model.ViewModeExpanded {
		itemHeight = config.TaskBoxHeightExpanded
	}
	pv.grid.SetItemHeight(itemHeight)
	pv.grid.Clear()

	// Get filtered and sorted tasks from controller
	tasks := pv.getFilteredTasks()
	columns := pv.pluginConfig.GetColumns()

	if len(tasks) == 0 {
		// Show nothing when there are no tasks
		return
	}

	// clamp selection
	pv.pluginConfig.ClampSelection(len(tasks))
	selectedIndex := pv.pluginConfig.GetSelectedIndex()

	// set selection on grid (by row)
	selectedRow := selectedIndex / columns
	pv.grid.SetSelection(selectedRow)

	// build grid row by row
	numRows := (len(tasks) + columns - 1) / columns

	for row := 0; row < numRows; row++ {
		rowFlex := tview.NewFlex().SetDirection(tview.FlexColumn)

		for col := 0; col < columns; col++ {
			idx := row*columns + col

			if idx < len(tasks) {
				task := tasks[idx]
				isSelected := idx == selectedIndex
				var taskBox *tview.Frame
				if viewMode == model.ViewModeCompact {
					taskBox = CreateCompactTaskBox(task, isSelected, config.GetColors())
				} else {
					taskBox = CreateExpandedTaskBox(task, isSelected, config.GetColors())
				}
				rowFlex.AddItem(taskBox, 0, 1, false)
			} else {
				// empty placeholder for incomplete row
				spacer := tview.NewBox()
				rowFlex.AddItem(spacer, 0, 1, false)
			}
		}

		pv.grid.AddItem(rowFlex)
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
	tasks := pv.getFilteredTasks()
	idx := pv.pluginConfig.GetSelectedIndex()
	if idx >= 0 && idx < len(tasks) {
		return tasks[idx].ID
	}
	return ""
}

// SetSelectedID sets the selection to a task
func (pv *PluginView) SetSelectedID(id string) {
	tasks := pv.getFilteredTasks()
	for i, t := range tasks {
		if t.ID == id {
			pv.pluginConfig.SetSelectedIndex(i)
			break
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
	pv.root.AddItem(pv.grid, 0, 1, false)

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
	pv.root.AddItem(pv.grid, 0, 1, true)
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
	tasks := pv.getFilteredTasks()
	return []store.Stat{
		{Name: "Total", Value: fmt.Sprintf("%d", len(tasks)), Order: 5},
	}
}
