package controller

import (
	"log/slog"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// PluginController handles plugin view actions: navigation, open, create, delete.
type PluginController struct {
	taskStore     store.Store
	pluginConfig  *model.PluginConfig
	pluginDef     *plugin.TikiPlugin
	navController *NavigationController
	registry      *ActionRegistry
}

// NewPluginController creates a plugin controller
func NewPluginController(
	taskStore store.Store,
	pluginConfig *model.PluginConfig,
	pluginDef *plugin.TikiPlugin,
	navController *NavigationController,
) *PluginController {
	return &PluginController{
		taskStore:     taskStore,
		pluginConfig:  pluginConfig,
		pluginDef:     pluginDef,
		navController: navController,
		registry:      PluginViewActions(),
	}
}

// GetActionRegistry returns the actions for the plugin view
func (pc *PluginController) GetActionRegistry() *ActionRegistry {
	return pc.registry
}

// GetPluginName returns the plugin name
func (pc *PluginController) GetPluginName() string {
	return pc.pluginDef.Name
}

// GetPluginDefinition returns the plugin definition
func (pc *PluginController) GetPluginDefinition() *plugin.TikiPlugin {
	return pc.pluginDef
}

// HandleAction processes a plugin action
func (pc *PluginController) HandleAction(actionID ActionID) bool {
	switch actionID {
	case ActionNavUp:
		return pc.handleNav("up")
	case ActionNavDown:
		return pc.handleNav("down")
	case ActionNavLeft:
		return pc.handleNav("left")
	case ActionNavRight:
		return pc.handleNav("right")
	case ActionMoveTaskLeft:
		return pc.handleMoveTask(-1)
	case ActionMoveTaskRight:
		return pc.handleMoveTask(1)
	case ActionOpenFromPlugin:
		return pc.handleOpenTask()
	case ActionNewTask:
		return pc.handleNewTask()
	case ActionDeleteTask:
		return pc.handleDeleteTask()
	case ActionToggleViewMode:
		return pc.handleToggleViewMode()
	default:
		return false
	}
}

func (pc *PluginController) handleNav(direction string) bool {
	pane := pc.pluginConfig.GetSelectedPane()
	tasks := pc.GetFilteredTasksForPane(pane)
	if direction == "left" || direction == "right" {
		if pc.pluginConfig.MoveSelection(direction, len(tasks)) {
			return true
		}
		return pc.handlePaneSwitch(direction)
	}
	return pc.pluginConfig.MoveSelection(direction, len(tasks))
}

func (pc *PluginController) handleOpenTask() bool {
	taskID := pc.getSelectedTaskID()
	if taskID == "" {
		return false
	}

	pc.navController.PushView(model.TaskDetailViewID, model.EncodeTaskDetailParams(model.TaskDetailParams{
		TaskID: taskID,
	}))
	return true
}

func (pc *PluginController) handlePaneSwitch(direction string) bool {
	currentPane := pc.pluginConfig.GetSelectedPane()
	nextPane := currentPane
	switch direction {
	case "left":
		nextPane--
	case "right":
		nextPane++
	default:
		return false
	}

	for nextPane >= 0 && nextPane < len(pc.pluginDef.Panes) {
		tasks := pc.GetFilteredTasksForPane(nextPane)
		if len(tasks) > 0 {
			pc.pluginConfig.SetSelectedPane(nextPane)
			pc.pluginConfig.ClampSelection(len(tasks))
			return true
		}
		switch direction {
		case "left":
			nextPane--
		case "right":
			nextPane++
		}
	}
	return false
}

func (pc *PluginController) handleNewTask() bool {
	task, err := pc.taskStore.NewTaskTemplate()
	if err != nil {
		slog.Error("failed to create task template", "error", err)
		return false
	}

	pc.navController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
		TaskID: task.ID,
		Draft:  task,
		Focus:  model.EditFieldTitle,
	}))
	slog.Info("new tiki draft started from plugin", "task_id", task.ID, "plugin", pc.pluginDef.Name)
	return true
}

func (pc *PluginController) handleDeleteTask() bool {
	taskID := pc.getSelectedTaskID()
	if taskID == "" {
		return false
	}

	pc.taskStore.DeleteTask(taskID)
	return true
}

func (pc *PluginController) handleToggleViewMode() bool {
	pc.pluginConfig.ToggleViewMode()
	return true
}

func (pc *PluginController) handleMoveTask(offset int) bool {
	taskID := pc.getSelectedTaskID()
	if taskID == "" {
		return false
	}

	if pc.pluginDef == nil || len(pc.pluginDef.Panes) == 0 {
		return false
	}

	currentPane := pc.pluginConfig.GetSelectedPane()
	targetPane := currentPane + offset
	if targetPane < 0 || targetPane >= len(pc.pluginDef.Panes) {
		return false
	}

	taskItem := pc.taskStore.GetTask(taskID)
	if taskItem == nil {
		return false
	}

	currentUser := getCurrentUserName(pc.taskStore)
	updated, err := plugin.ApplyPaneAction(taskItem, pc.pluginDef.Panes[targetPane].Action, currentUser)
	if err != nil {
		slog.Error("failed to apply pane action", "task_id", taskID, "error", err)
		return false
	}

	if err := pc.taskStore.UpdateTask(updated); err != nil {
		slog.Error("failed to update task after pane move", "task_id", taskID, "error", err)
		return false
	}

	pc.ensureSearchResultIncludesTask(updated)
	pc.selectTaskInPane(targetPane, taskID)
	return true
}

// HandleSearch processes a search query for the plugin view
func (pc *PluginController) HandleSearch(query string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return // Don't search empty/whitespace
	}

	// Save current position
	pc.pluginConfig.SavePreSearchState()

	// Search across all tasks; pane membership is decided per pane
	results := pc.taskStore.Search(query, nil)
	if len(results) == 0 {
		pc.pluginConfig.ClearSearchResults()
		return
	}

	pc.pluginConfig.SetSearchResults(results, query)
	if pc.selectFirstNonEmptyPane() {
		return
	}
}

// getSelectedTaskID returns the ID of the currently selected task
func (pc *PluginController) getSelectedTaskID() string {
	pane := pc.pluginConfig.GetSelectedPane()
	tasks := pc.GetFilteredTasksForPane(pane)
	idx := pc.pluginConfig.GetSelectedIndexForPane(pane)
	if idx < 0 || idx >= len(tasks) {
		return ""
	}
	return tasks[idx].ID
}

// GetFilteredTasksForPane returns tasks filtered and sorted for a specific pane.
func (pc *PluginController) GetFilteredTasksForPane(pane int) []*task.Task {
	if pc.pluginDef == nil {
		return nil
	}
	if pane < 0 || pane >= len(pc.pluginDef.Panes) {
		return nil
	}

	// Check if search is active - if so, return search results instead
	searchResults := pc.pluginConfig.GetSearchResults()

	// Normal filtering path when search is not active
	allTasks := pc.taskStore.GetAllTasks()
	now := time.Now()

	// Get current user for "my tasks" type filters
	currentUser := getCurrentUserName(pc.taskStore)

	// Apply filter
	var filtered []*task.Task
	for _, task := range allTasks {
		paneFilter := pc.pluginDef.Panes[pane].Filter
		if paneFilter == nil || paneFilter.Evaluate(task, now, currentUser) {
			filtered = append(filtered, task)
		}
	}

	if searchResults != nil {
		searchTaskMap := make(map[string]bool, len(searchResults))
		for _, result := range searchResults {
			searchTaskMap[result.Task.ID] = true
		}
		filtered = filterTasksBySearch(filtered, searchTaskMap)
	}

	// Apply sort
	if len(pc.pluginDef.Sort) > 0 {
		plugin.SortTasks(filtered, pc.pluginDef.Sort)
	}

	return filtered
}

func (pc *PluginController) selectTaskInPane(pane int, taskID string) {
	if pane < 0 || pane >= len(pc.pluginDef.Panes) {
		return
	}

	tasks := pc.GetFilteredTasksForPane(pane)
	targetIndex := 0
	for i, task := range tasks {
		if task.ID == taskID {
			targetIndex = i
			break
		}
	}

	pc.pluginConfig.SetSelectedPane(pane)
	pc.pluginConfig.SetSelectedIndexForPane(pane, targetIndex)
}

func (pc *PluginController) selectFirstNonEmptyPane() bool {
	for pane := range pc.pluginDef.Panes {
		tasks := pc.GetFilteredTasksForPane(pane)
		if len(tasks) > 0 {
			pc.pluginConfig.SetSelectedPaneAndIndex(pane, 0)
			return true
		}
	}
	return false
}

func (pc *PluginController) EnsureFirstNonEmptyPaneSelection() bool {
	if pc.pluginDef == nil {
		return false
	}
	currentPane := pc.pluginConfig.GetSelectedPane()
	if currentPane >= 0 && currentPane < len(pc.pluginDef.Panes) {
		tasks := pc.GetFilteredTasksForPane(currentPane)
		if len(tasks) > 0 {
			return false
		}
	}
	return pc.selectFirstNonEmptyPane()
}

func (pc *PluginController) ensureSearchResultIncludesTask(updated *task.Task) {
	if updated == nil {
		return
	}
	searchResults := pc.pluginConfig.GetSearchResults()
	if searchResults == nil {
		return
	}
	for _, result := range searchResults {
		if result.Task != nil && result.Task.ID == updated.ID {
			return
		}
	}

	searchResults = append(searchResults, task.SearchResult{
		Task:  updated,
		Score: 1.0,
	})
	pc.pluginConfig.SetSearchResults(searchResults, pc.pluginConfig.GetSearchQuery())
}

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
