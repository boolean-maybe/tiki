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
	tasks := pc.GetFilteredTasks()
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

// HandleSearch processes a search query for the plugin view
func (pc *PluginController) HandleSearch(query string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return // Don't search empty/whitespace
	}

	// Save current position
	pc.pluginConfig.SavePreSearchState()

	// Get current user and time ONCE before filtering (not per task!)
	now := time.Now()
	currentUser, _, _ := pc.taskStore.GetCurrentUser()

	// Get plugin's filter as a function
	filterFunc := func(t *task.Task) bool {
		if pc.pluginDef.Filter == nil {
			return true
		}
		return pc.pluginDef.Filter.Evaluate(t, now, currentUser)
	}

	// Search within filtered results
	results := pc.taskStore.Search(query, filterFunc)

	pc.pluginConfig.SetSearchResults(results, query)
	pc.pluginConfig.SetSelectedIndex(0)
}

// getSelectedTaskID returns the ID of the currently selected task
func (pc *PluginController) getSelectedTaskID() string {
	tasks := pc.GetFilteredTasks()
	idx := pc.pluginConfig.GetSelectedIndex()
	if idx < 0 || idx >= len(tasks) {
		return ""
	}
	return tasks[idx].ID
}

// GetFilteredTasks returns tasks filtered and sorted according to plugin rules
func (pc *PluginController) GetFilteredTasks() []*task.Task {
	// Check if search is active - if so, return search results instead
	searchResults := pc.pluginConfig.GetSearchResults()
	if searchResults != nil {
		// Extract tasks from search results
		tasks := make([]*task.Task, len(searchResults))
		for i, result := range searchResults {
			tasks[i] = result.Task
		}
		return tasks
	}

	// Normal filtering path when search is not active
	allTasks := pc.taskStore.GetAllTasks()
	now := time.Now()

	// Get current user for "my tasks" type filters
	currentUser := ""
	if user, _, err := pc.taskStore.GetCurrentUser(); err == nil {
		currentUser = user
	}

	// Apply filter
	var filtered []*task.Task
	for _, task := range allTasks {
		if pc.pluginDef.Filter == nil || pc.pluginDef.Filter.Evaluate(task, now, currentUser) {
			filtered = append(filtered, task)
		}
	}

	// Apply sort
	if len(pc.pluginDef.Sort) > 0 {
		plugin.SortTasks(filtered, pc.pluginDef.Sort)
	}

	return filtered
}
