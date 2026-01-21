package view

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"

	"github.com/rivo/tview"
)

// BoardView renders the kanban board: panes arranged horizontally, each containing task boxes.
// BoardView renders the kanban board with panes
type BoardView struct {
	root                *tview.Flex
	searchHelper        *SearchHelper
	paneTitles          tview.Primitive // pane title row
	panes               *tview.Flex
	paneBoxes           []*ScrollableList // each pane's task container
	taskStore           store.Store
	boardConfig         *model.BoardConfig
	registry            *controller.ActionRegistry
	storeListenerID     int
	selectionListenerID int
}

// NewBoardView creates a board view
func NewBoardView(taskStore store.Store, boardConfig *model.BoardConfig) *BoardView {
	registry := controller.BoardViewActions()

	bv := &BoardView{
		taskStore:   taskStore,
		boardConfig: boardConfig,
		registry:    registry,
	}

	bv.build()

	// listeners are registered in OnFocus and removed in OnBlur

	return bv
}

// buildSearchMap creates a map of task IDs from search results for fast lookup.
// Returns nil if no search is active.
func buildSearchMap(searchResults []task.SearchResult) map[string]bool {
	if searchResults == nil {
		return nil
	}
	searchMap := make(map[string]bool, len(searchResults))
	for _, result := range searchResults {
		searchMap[result.Task.ID] = true
	}
	return searchMap
}

// filterTasksBySearch filters tasks based on search results.
// If searchMap is nil (no active search), returns all tasks.
// Otherwise returns only tasks present in the search map.
func filterTasksBySearch(tasks []*task.Task, searchMap map[string]bool) []*task.Task {
	if searchMap == nil {
		return tasks
	}
	filtered := make([]*task.Task, 0, len(tasks))
	for _, t := range tasks {
		if searchMap[t.ID] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func (bv *BoardView) build() {
	colors := config.GetColors()

	// Collect pane names for gradient caption row
	panes := bv.boardConfig.GetPanes()
	paneNames := make([]string, len(panes))
	for i, pane := range panes {
		paneNames[i] = pane.Name
	}

	// Create single gradient caption row for all panes
	bv.paneTitles = NewGradientCaptionRow(
		paneNames,
		colors.BoardPaneTitleGradient,
		colors.BoardPaneTitleText,
	)

	// panes container (just task lists, no titles)
	bv.panes = tview.NewFlex().SetDirection(tview.FlexColumn)
	bv.paneBoxes = make([]*ScrollableList, 0)

	// determine item height based on view mode
	itemHeight := config.TaskBoxHeight
	if bv.boardConfig.GetViewMode() == model.ViewModeExpanded {
		itemHeight = config.TaskBoxHeightExpanded
	}

	for _, pane := range panes {
		// task container for this pane
		taskContainer := NewScrollableList().SetItemHeight(itemHeight)
		bv.paneBoxes = append(bv.paneBoxes, taskContainer)

		// selected pane gets focus
		isSelected := pane.ID == bv.boardConfig.GetSelectedPaneID()
		bv.panes.AddItem(taskContainer, 0, 1, isSelected)
	}

	// search helper - focus returns to panes container
	bv.searchHelper = NewSearchHelper(bv.panes)
	bv.searchHelper.SetCancelHandler(func() {
		bv.HideSearch()
	})

	// root layout
	bv.root = tview.NewFlex().SetDirection(tview.FlexRow)
	bv.rebuildLayout()

	bv.refresh()
}

// rebuildLayout rebuilds the root layout based on current state (search visibility)
func (bv *BoardView) rebuildLayout() {
	bv.root.Clear()
	bv.root.AddItem(bv.paneTitles, 1, 0, false)

	// Restore search box if search is active (e.g., returning from task details)
	if bv.boardConfig.IsSearchActive() {
		query := bv.boardConfig.GetSearchQuery()
		bv.searchHelper.ShowSearch(query)
		bv.root.AddItem(bv.searchHelper.GetSearchBox(), config.SearchBoxHeight, 0, false)
		bv.root.AddItem(bv.panes, 0, 1, false)
	} else {
		bv.root.AddItem(bv.panes, 0, 1, true)
	}
}

func (bv *BoardView) refresh() {
	panes := bv.boardConfig.GetPanes()
	selectedPaneID := bv.boardConfig.GetSelectedPaneID()
	selectedRow := bv.boardConfig.GetSelectedRow()
	viewMode := bv.boardConfig.GetViewMode()

	// update item height based on view mode
	itemHeight := config.TaskBoxHeight
	if viewMode == model.ViewModeExpanded {
		itemHeight = config.TaskBoxHeightExpanded
	}

	// Check if search is active
	searchResults := bv.boardConfig.GetSearchResults()
	searchTaskMap := buildSearchMap(searchResults)

	for i, pane := range panes {
		if i >= len(bv.paneBoxes) {
			break
		}

		container := bv.paneBoxes[i]
		container.SetItemHeight(itemHeight)
		container.Clear()

		allTasks := bv.taskStore.GetTasksByStatus(task.Status(pane.Status))

		// Filter tasks by search results if search is active
		tasks := filterTasksBySearch(allTasks, searchTaskMap)

		if len(tasks) == 0 {
			continue
		}

		// clamp selectedRow to valid bounds for this pane
		effectiveRow := selectedRow
		if pane.ID == selectedPaneID {
			if effectiveRow >= len(tasks) {
				effectiveRow = len(tasks) - 1
				if effectiveRow < 0 {
					effectiveRow = 0
				}
				bv.boardConfig.SetSelectedRowSilent(effectiveRow)
			}
			// ensure selection is visible
			container.SetSelection(effectiveRow)
		} else {
			container.SetSelection(-1)
		}

		for j, task := range tasks {
			isSelected := pane.ID == selectedPaneID && j == effectiveRow
			var taskFrame *tview.Frame
			colors := config.GetColors()
			if viewMode == model.ViewModeCompact {
				taskFrame = CreateCompactTaskBox(task, isSelected, colors)
			} else {
				taskFrame = CreateExpandedTaskBox(task, isSelected, colors)
			}
			container.AddItem(taskFrame)
		}
	}

	// Smart pane selection: if current pane is empty, find nearest non-empty pane
	selectedStatus := bv.boardConfig.GetStatusForPane(selectedPaneID)
	allSelectedTasks := bv.taskStore.GetTasksByStatus(selectedStatus)

	// Filter by search if active
	selectedTasks := filterTasksBySearch(allSelectedTasks, searchTaskMap)

	if len(selectedTasks) == 0 {
		// Current pane is empty - find fallback pane
		currentIdx := -1
		for i, pane := range panes {
			if pane.ID == selectedPaneID {
				currentIdx = i
				break
			}
		}

		if currentIdx >= 0 {
			// Search LEFT first (preferred direction)
			for i := currentIdx - 1; i >= 0; i-- {
				status := bv.boardConfig.GetStatusForPane(panes[i].ID)
				candidateTasks := bv.taskStore.GetTasksByStatus(status)

				// Filter by search if active
				filteredCandidates := filterTasksBySearch(candidateTasks, searchTaskMap)

				if len(filteredCandidates) > 0 {
					bv.boardConfig.SetSelection(panes[i].ID, 0)
					return
				}
			}

			// Search RIGHT if no non-empty column found to the left
			for i := currentIdx + 1; i < len(panes); i++ {
				status := bv.boardConfig.GetStatusForPane(panes[i].ID)
				candidateTasks := bv.taskStore.GetTasksByStatus(status)

				// Filter by search if active
				filteredCandidates := filterTasksBySearch(candidateTasks, searchTaskMap)

				if len(filteredCandidates) > 0 {
					bv.boardConfig.SetSelection(panes[i].ID, 0)
					return
				}
			}
		}

		// All panes empty - selection remains but nothing renders
		// This is acceptable behavior per requirements
	}
}

// GetPrimitive returns the root tview primitive
func (bv *BoardView) GetPrimitive() tview.Primitive {
	return bv.root
}

// GetActionRegistry returns the view's action registry
func (bv *BoardView) GetActionRegistry() *controller.ActionRegistry {
	return bv.registry
}

// GetViewID returns the view identifier
func (bv *BoardView) GetViewID() model.ViewID {
	return model.BoardViewID
}

// OnFocus is called when the view becomes active
func (bv *BoardView) OnFocus() {
	// re-register listeners (they may have been removed in OnBlur)
	bv.storeListenerID = bv.taskStore.AddListener(bv.refresh)
	bv.selectionListenerID = bv.boardConfig.AddSelectionListener(bv.refresh)

	bv.ensureValidSelection()
	bv.refresh()
}

// ensureValidSelection ensures selection is on a valid task.
// selects first task in leftmost non-empty pane, or clears selection if all empty.
func (bv *BoardView) ensureValidSelection() {
	// check if current selection is valid
	currentPaneID := bv.boardConfig.GetSelectedPaneID()
	currentStatus := bv.boardConfig.GetStatusForPane(currentPaneID)
	currentTasks := bv.taskStore.GetTasksByStatus(currentStatus)
	currentRow := bv.boardConfig.GetSelectedRow()

	if len(currentTasks) > 0 && currentRow >= 0 && currentRow < len(currentTasks) {
		return // current selection is valid
	}

	// find first non-empty pane from left
	for _, pane := range bv.boardConfig.GetPanes() {
		status := bv.boardConfig.GetStatusForPane(pane.ID)
		tasks := bv.taskStore.GetTasksByStatus(status)
		if len(tasks) > 0 {
			bv.boardConfig.SetSelection(pane.ID, 0)
			return
		}
	}

	// all panes empty - reset to first pane, row 0 (nothing will be highlighted)
	panes := bv.boardConfig.GetPanes()
	if len(panes) > 0 {
		bv.boardConfig.SetSelection(panes[0].ID, 0)
	}
}

// OnBlur is called when the view becomes inactive
func (bv *BoardView) OnBlur() {
	// remove listeners to prevent accumulation
	bv.taskStore.RemoveListener(bv.storeListenerID)
	bv.boardConfig.RemoveSelectionListener(bv.selectionListenerID)
}

// GetSelectedID returns the selected task ID
func (bv *BoardView) GetSelectedID() string {
	paneID := bv.boardConfig.GetSelectedPaneID()
	status := bv.boardConfig.GetStatusForPane(paneID)
	tasks := bv.taskStore.GetTasksByStatus(status)

	row := bv.boardConfig.GetSelectedRow()
	if row >= 0 && row < len(tasks) {
		return tasks[row].ID
	}
	return ""
}

// SetSelectedID sets the selection to a task
func (bv *BoardView) SetSelectedID(id string) {
	// find task and select it
	task := bv.taskStore.GetTask(id)
	if task == nil {
		return
	}

	pane := bv.boardConfig.GetPaneByStatus(task.Status)
	if pane == nil {
		return
	}

	bv.boardConfig.SetSelectedPane(pane.ID)

	// find row index
	tasks := bv.taskStore.GetTasksByStatus(task.Status)
	for i, t := range tasks {
		if t.ID == id {
			bv.boardConfig.SetSelectedRow(i)
			break
		}
	}

	bv.refresh()
}

// ShowSearch displays the search box and returns the primitive to focus
func (bv *BoardView) ShowSearch() tview.Primitive {
	if bv.searchHelper.IsVisible() {
		return bv.searchHelper.GetSearchBox()
	}

	query := bv.boardConfig.GetSearchQuery()
	searchBox := bv.searchHelper.ShowSearch(query)

	// Rebuild layout with search box
	bv.root.Clear()
	bv.root.AddItem(bv.paneTitles, 1, 0, false)
	bv.root.AddItem(bv.searchHelper.GetSearchBox(), config.SearchBoxHeight, 0, true)
	bv.root.AddItem(bv.panes, 0, 1, false)

	return searchBox
}

// HideSearch hides the search box and clears search results
func (bv *BoardView) HideSearch() {
	if !bv.searchHelper.IsVisible() {
		return
	}

	bv.searchHelper.HideSearch()

	// Clear search results (restores pre-search selection)
	bv.boardConfig.ClearSearchResults()

	// Rebuild layout without search box
	bv.root.Clear()
	bv.root.AddItem(bv.paneTitles, 1, 0, false)
	bv.root.AddItem(bv.panes, 0, 1, true)
}

// IsSearchVisible returns whether the search box is currently visible
func (bv *BoardView) IsSearchVisible() bool {
	return bv.searchHelper.IsVisible()
}

// IsSearchBoxFocused returns whether the search box currently has focus
func (bv *BoardView) IsSearchBoxFocused() bool {
	return bv.searchHelper.HasFocus()
}

// SetSearchSubmitHandler sets the callback for when search is submitted
func (bv *BoardView) SetSearchSubmitHandler(handler func(text string)) {
	bv.searchHelper.SetSubmitHandler(handler)
}

// SetFocusSetter sets the callback for requesting focus changes
func (bv *BoardView) SetFocusSetter(setter func(p tview.Primitive)) {
	bv.searchHelper.SetFocusSetter(setter)
}

// GetStats returns stats for the header (Total count of board tasks)
func (bv *BoardView) GetStats() []store.Stat {
	// Count tasks in all board panes (non-backlog statuses)
	total := 0
	for _, pane := range bv.boardConfig.GetPanes() {
		tasks := bv.taskStore.GetTasksByStatus(task.Status(pane.Status))
		total += len(tasks)
	}

	return []store.Stat{
		{Name: "Total", Value: fmt.Sprintf("%d", total), Order: 5},
	}
}
