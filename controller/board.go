package controller

import (
	"log/slog"
	"strings"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// BoardController handles board view actions: pane/task navigation, moving tasks, create/delete.
// BoardController handles board-specific actions
type BoardController struct {
	taskStore     store.Store
	boardConfig   *model.BoardConfig
	navController *NavigationController
	registry      *ActionRegistry
}

// NewBoardController creates a board controller
func NewBoardController(
	taskStore store.Store,
	boardConfig *model.BoardConfig,
	navController *NavigationController,
) *BoardController {
	return &BoardController{
		taskStore:     taskStore,
		boardConfig:   boardConfig,
		navController: navController,
		registry:      BoardViewActions(),
	}
}

// GetActionRegistry returns the actions for the board view
func (bc *BoardController) GetActionRegistry() *ActionRegistry {
	return bc.registry
}

// HandleAction processes board-specific actions such as navigation, task movement, and view switching.
// Returns true if the action was handled, false otherwise.
func (bc *BoardController) HandleAction(actionID ActionID) bool {
	switch actionID {
	case ActionNavLeft:
		return bc.handleNavLeft()
	case ActionNavRight:
		return bc.handleNavRight()
	case ActionNavUp:
		return bc.handleNavUp()
	case ActionNavDown:
		return bc.handleNavDown()
	case ActionOpenTask:
		return bc.handleOpenTask()
	case ActionMoveTaskLeft:
		return bc.handleMoveTaskLeft()
	case ActionMoveTaskRight:
		return bc.handleMoveTaskRight()
	case ActionNewTask:
		return bc.handleNewTask()
	case ActionDeleteTask:
		return bc.handleDeleteTask()
	case ActionToggleViewMode:
		return bc.handleToggleViewMode()
	default:
		return false
	}
}

func (bc *BoardController) handleNavLeft() bool {
	panes := bc.boardConfig.GetPanes()
	currentIdx := -1
	currentPaneID := bc.boardConfig.GetSelectedPaneID()

	for i, pane := range panes {
		if pane.ID == currentPaneID {
			currentIdx = i
			break
		}
	}

	if currentIdx < 0 {
		return false
	}

	// find first non-empty pane to the left
	for i := currentIdx - 1; i >= 0; i-- {
		status := bc.boardConfig.GetStatusForPane(panes[i].ID)
		tasks := bc.taskStore.GetTasksByStatus(status)
		if len(tasks) > 0 {
			bc.boardConfig.SetSelection(panes[i].ID, 0)
			return true
		}
	}
	return false
}

func (bc *BoardController) handleNavRight() bool {
	panes := bc.boardConfig.GetPanes()
	currentIdx := -1
	currentPaneID := bc.boardConfig.GetSelectedPaneID()

	for i, pane := range panes {
		if pane.ID == currentPaneID {
			currentIdx = i
			break
		}
	}

	if currentIdx < 0 {
		return false
	}

	// find first non-empty pane to the right
	for i := currentIdx + 1; i < len(panes); i++ {
		status := bc.boardConfig.GetStatusForPane(panes[i].ID)
		tasks := bc.taskStore.GetTasksByStatus(status)
		if len(tasks) > 0 {
			bc.boardConfig.SetSelection(panes[i].ID, 0)
			return true
		}
	}
	return false
}

func (bc *BoardController) handleNavUp() bool {
	row := bc.boardConfig.GetSelectedRow()
	if row > 0 {
		bc.boardConfig.SetSelectedRow(row - 1)
		return true
	}
	return false
}

func (bc *BoardController) handleNavDown() bool {
	// get task count for current pane to validate
	paneID := bc.boardConfig.GetSelectedPaneID()
	status := bc.boardConfig.GetStatusForPane(paneID)
	tasks := bc.taskStore.GetTasksByStatus(status)

	row := bc.boardConfig.GetSelectedRow()
	if row < len(tasks)-1 {
		bc.boardConfig.SetSelectedRow(row + 1)
		return true
	}
	return false
}

func (bc *BoardController) handleOpenTask() bool {
	taskID := bc.getSelectedTaskID()
	if taskID == "" {
		return false
	}

	// push task detail view with task ID parameter
	bc.navController.PushView(model.TaskDetailViewID, model.EncodeTaskDetailParams(model.TaskDetailParams{
		TaskID: taskID,
	}))
	return true
}

func (bc *BoardController) handleMoveTaskLeft() bool {
	taskID := bc.getSelectedTaskID()
	if taskID == "" {
		return false
	}

	paneID := bc.boardConfig.GetSelectedPaneID()
	prevPaneID := bc.boardConfig.GetPreviousPaneID(paneID)
	if prevPaneID == "" {
		return false
	}

	newStatus := bc.boardConfig.GetStatusForPane(prevPaneID)
	if !bc.taskStore.UpdateStatus(taskID, newStatus) {
		slog.Error("failed to move task left", "task_id", taskID, "error", "update status failed")
		return false
	}
	slog.Info("task moved left", "task_id", taskID, "from_pane_id", paneID, "to_pane_id", prevPaneID, "new_status", newStatus)

	// move selection to follow the task
	bc.selectTaskInPane(prevPaneID, taskID)
	return true
}

func (bc *BoardController) handleMoveTaskRight() bool {
	taskID := bc.getSelectedTaskID()
	if taskID == "" {
		return false
	}

	paneID := bc.boardConfig.GetSelectedPaneID()
	nextPaneID := bc.boardConfig.GetNextPaneID(paneID)
	if nextPaneID == "" {
		return false
	}

	newStatus := bc.boardConfig.GetStatusForPane(nextPaneID)
	if !bc.taskStore.UpdateStatus(taskID, newStatus) {
		slog.Error("failed to move task right", "task_id", taskID, "error", "update status failed")
		return false
	}
	slog.Info("task moved right", "task_id", taskID, "from_pane_id", paneID, "to_pane_id", nextPaneID, "new_status", newStatus)

	// move selection to follow the task
	bc.selectTaskInPane(nextPaneID, taskID)
	return true
}

// selectTaskInPane moves selection to a specific task in a pane
func (bc *BoardController) selectTaskInPane(paneID, taskID string) {
	status := bc.boardConfig.GetStatusForPane(paneID)
	tasks := bc.taskStore.GetTasksByStatus(status)

	row := 0
	for i, task := range tasks {
		if task.ID == taskID {
			row = i
			break
		}
	}

	// always update selection to target pane, even if task not found (use row 0)
	bc.boardConfig.SetSelection(paneID, row)
}

func (bc *BoardController) handleNewTask() bool {
	task, err := bc.taskStore.NewTaskTemplate()
	if err != nil {
		slog.Error("failed to create task template", "error", err)
		return false
	}

	bc.navController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
		TaskID: task.ID,
		Draft:  task,
		Focus:  model.EditFieldTitle,
	}))
	slog.Info("new tiki draft started", "task_id", task.ID, "status", task.Status)
	return true
}

func (bc *BoardController) handleDeleteTask() bool {
	taskID := bc.getSelectedTaskID()
	if taskID == "" {
		return false
	}

	bc.taskStore.DeleteTask(taskID)
	return true
}

// getSelectedTaskID returns the ID of the currently selected task
func (bc *BoardController) getSelectedTaskID() string {
	paneID := bc.boardConfig.GetSelectedPaneID()
	status := bc.boardConfig.GetStatusForPane(paneID)
	allTasks := bc.taskStore.GetTasksByStatus(status)

	// Filter tasks by search results if search is active
	var tasks []*task.Task
	if searchResults := bc.boardConfig.GetSearchResults(); searchResults != nil {
		searchTaskMap := make(map[string]bool)
		for _, result := range searchResults {
			searchTaskMap[result.Task.ID] = true
		}
		for _, t := range allTasks {
			if searchTaskMap[t.ID] {
				tasks = append(tasks, t)
			}
		}
	} else {
		tasks = allTasks
	}

	row := bc.boardConfig.GetSelectedRow()
	if row < 0 || row >= len(tasks) {
		return ""
	}
	return tasks[row].ID
}

func (bc *BoardController) handleToggleViewMode() bool {
	bc.boardConfig.ToggleViewMode()
	slog.Info("view mode toggled", "new_mode", bc.boardConfig.GetViewMode())
	return true
}

// HandleSearch processes a search query for the board view, filtering tasks by title.
// It saves the current selection, searches all board panes, and displays matching results.
// Empty queries are ignored.
func (bc *BoardController) HandleSearch(query string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return // Don't search empty/whitespace
	}

	// Save current position (pane + row)
	bc.boardConfig.SavePreSearchState()

	// Search all tasks visible on the board (all board panes: todo, in_progress, review, done, etc.)
	// Build set of statuses from board panes
	boardStatuses := make(map[task.Status]bool)
	for _, pane := range bc.boardConfig.GetPanes() {
		boardStatuses[task.Status(pane.Status)] = true
	}

	// Filter: tasks with board statuses only
	filterFunc := func(t *task.Task) bool {
		return boardStatuses[t.Status]
	}

	results := bc.taskStore.Search(query, filterFunc)

	// Store results
	bc.boardConfig.SetSearchResults(results, query)

	// Jump to first result's pane
	if len(results) > 0 {
		firstTask := results[0].Task
		pane := bc.boardConfig.GetPaneByStatus(firstTask.Status)
		if pane != nil {
			bc.boardConfig.SetSelection(pane.ID, 0)
		}
	}
}
