package controller

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// PluginController handles plugin view actions: navigation, open, create, delete.
type PluginController struct {
	pluginBase
}

// NewPluginController creates a plugin controller
func NewPluginController(
	taskStore store.Store,
	mutationGate *service.TaskMutationGate,
	pluginConfig *model.PluginConfig,
	pluginDef *plugin.TikiPlugin,
	navController *NavigationController,
	statusline *model.StatuslineConfig,
	schema ruki.Schema,
) *PluginController {
	pc := &PluginController{
		pluginBase: pluginBase{
			taskStore:     taskStore,
			mutationGate:  mutationGate,
			pluginConfig:  pluginConfig,
			pluginDef:     pluginDef,
			navController: navController,
			statusline:    statusline,
			registry:      PluginViewActions(),
			schema:        schema,
		},
	}

	// register plugin-specific shortcut actions, warn about conflicts
	globalActions := DefaultGlobalActions()
	for _, a := range pluginDef.Actions {
		if existing, ok := globalActions.LookupRune(a.Rune); ok {
			slog.Warn("plugin action key shadows global action and will be unreachable",
				"plugin", pluginDef.Name, "key", string(a.Rune),
				"plugin_action", a.Label, "global_action", existing.Label)
		} else if existing, ok := pc.registry.LookupRune(a.Rune); ok {
			slog.Warn("plugin action key shadows built-in action and will be unreachable",
				"plugin", pluginDef.Name, "key", string(a.Rune),
				"plugin_action", a.Label, "built_in_action", existing.Label)
		}
		pc.registry.Register(Action{
			ID:           pluginActionID(a.Rune),
			Key:          tcell.KeyRune,
			Rune:         a.Rune,
			Label:        a.Label,
			ShowInHeader: true,
		})
	}

	return pc
}

const pluginActionPrefix = "plugin_action:"

// pluginActionID returns an ActionID for a plugin shortcut action key.
func pluginActionID(r rune) ActionID {
	return ActionID(pluginActionPrefix + string(r))
}

// getPluginActionRune extracts the rune from a plugin action ID.
// Returns 0 if the ID is not a plugin action.
func getPluginActionRune(id ActionID) rune {
	s := string(id)
	if !strings.HasPrefix(s, pluginActionPrefix) {
		return 0
	}
	rest := s[len(pluginActionPrefix):]
	if len(rest) == 0 {
		return 0
	}
	runes := []rune(rest)
	if len(runes) != 1 {
		return 0
	}
	return runes[0]
}

// ShowNavigation returns true — regular plugin views show plugin navigation keys.
func (pc *PluginController) ShowNavigation() bool { return true }

// EnsureFirstNonEmptyLaneSelection delegates to pluginBase with this controller's filter.
func (pc *PluginController) EnsureFirstNonEmptyLaneSelection() bool {
	return pc.pluginBase.EnsureFirstNonEmptyLaneSelection(pc.GetFilteredTasksForLane)
}

// HandleAction processes a plugin action
func (pc *PluginController) HandleAction(actionID ActionID) bool {
	switch actionID {
	case ActionNavUp:
		return pc.handleNav("up", pc.GetFilteredTasksForLane)
	case ActionNavDown:
		return pc.handleNav("down", pc.GetFilteredTasksForLane)
	case ActionNavLeft:
		return pc.handleNav("left", pc.GetFilteredTasksForLane)
	case ActionNavRight:
		return pc.handleNav("right", pc.GetFilteredTasksForLane)
	case ActionMoveTaskLeft:
		return pc.handleMoveTask(-1)
	case ActionMoveTaskRight:
		return pc.handleMoveTask(1)
	case ActionOpenFromPlugin:
		return pc.handleOpenTask(pc.GetFilteredTasksForLane)
	case ActionNewTask:
		return pc.handleNewTask()
	case ActionDeleteTask:
		return pc.handleDeleteTask(pc.GetFilteredTasksForLane)
	case ActionToggleViewMode:
		pc.pluginConfig.ToggleViewMode()
		return true
	default:
		if r := getPluginActionRune(actionID); r != 0 {
			return pc.handlePluginAction(r)
		}
		return false
	}
}

// HandleSearch processes a search query for the plugin view
func (pc *PluginController) HandleSearch(query string) {
	pc.handleSearch(query, func() bool {
		return pc.selectFirstNonEmptyLane(pc.GetFilteredTasksForLane)
	})
}

// handlePluginAction applies a plugin shortcut action to the currently selected task.
func (pc *PluginController) handlePluginAction(r rune) bool {
	// find the matching action definition
	var pa *plugin.PluginAction
	for i := range pc.pluginDef.Actions {
		if pc.pluginDef.Actions[i].Rune == r {
			pa = &pc.pluginDef.Actions[i]
			break
		}
	}
	if pa == nil {
		return false
	}

	executor := pc.newExecutor()
	allTasks := pc.taskStore.GetAllTasks()

	input := ruki.ExecutionInput{}
	taskID := pc.getSelectedTaskID(pc.GetFilteredTasksForLane)

	if pa.Action.IsUpdate() || pa.Action.IsDelete() {
		if taskID == "" {
			return false
		}
		input.SelectedTaskID = taskID
	}

	if pa.Action.IsCreate() {
		template, err := pc.taskStore.NewTaskTemplate()
		if err != nil {
			slog.Error("failed to create task template for plugin action", "key", string(r), "error", err)
			return false
		}
		input.CreateTemplate = template
	}

	result, err := executor.Execute(pa.Action, allTasks, input)
	if err != nil {
		slog.Error("failed to execute plugin action", "task_id", taskID, "key", string(r), "error", err)
		return false
	}

	ctx := context.Background()
	switch {
	case result.Update != nil:
		for _, updated := range result.Update.Updated {
			if err := pc.mutationGate.UpdateTask(ctx, updated); err != nil {
				slog.Error("failed to update task after plugin action", "task_id", updated.ID, "key", string(r), "error", err)
				if pc.statusline != nil {
					pc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
				}
				return false
			}
			pc.ensureSearchResultIncludesTask(updated)
		}
	case result.Create != nil:
		if err := pc.mutationGate.CreateTask(ctx, result.Create.Task); err != nil {
			slog.Error("failed to create task from plugin action", "key", string(r), "error", err)
			if pc.statusline != nil {
				pc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
			}
			return false
		}
	case result.Delete != nil:
		for _, deleted := range result.Delete.Deleted {
			if err := pc.mutationGate.DeleteTask(ctx, deleted); err != nil {
				slog.Error("failed to delete task from plugin action", "task_id", deleted.ID, "key", string(r), "error", err)
				if pc.statusline != nil {
					pc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
				}
				return false
			}
		}
	}

	slog.Info("plugin action applied", "task_id", taskID, "key", string(r), "label", pa.Label, "plugin", pc.pluginDef.Name)
	return true
}

func (pc *PluginController) handleMoveTask(offset int) bool {
	taskID := pc.getSelectedTaskID(pc.GetFilteredTasksForLane)
	if taskID == "" {
		return false
	}

	if pc.pluginDef == nil || len(pc.pluginDef.Lanes) == 0 {
		return false
	}

	currentLane := pc.pluginConfig.GetSelectedLane()
	targetLane := currentLane + offset
	if targetLane < 0 || targetLane >= len(pc.pluginDef.Lanes) {
		return false
	}

	actionStmt := pc.pluginDef.Lanes[targetLane].Action
	if actionStmt == nil {
		return false
	}

	allTasks := pc.taskStore.GetAllTasks()
	executor := pc.newExecutor()
	result, err := executor.Execute(actionStmt, allTasks, ruki.ExecutionInput{SelectedTaskID: taskID})
	if err != nil {
		slog.Error("failed to execute lane action", "task_id", taskID, "error", err)
		return false
	}

	if result.Update == nil || len(result.Update.Updated) == 0 {
		return false
	}

	updated := result.Update.Updated[0]
	if err := pc.mutationGate.UpdateTask(context.Background(), updated); err != nil {
		slog.Error("failed to update task after lane move", "task_id", taskID, "error", err)
		if pc.statusline != nil {
			pc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
		}
		return false
	}

	pc.ensureSearchResultIncludesTask(updated)
	pc.selectTaskInLane(targetLane, taskID, pc.GetFilteredTasksForLane)
	return true
}

// GetFilteredTasksForLane returns tasks filtered and sorted for a specific lane.
func (pc *PluginController) GetFilteredTasksForLane(lane int) []*task.Task {
	if pc.pluginDef == nil {
		return nil
	}
	if lane < 0 || lane >= len(pc.pluginDef.Lanes) {
		return nil
	}

	filterStmt := pc.pluginDef.Lanes[lane].Filter
	allTasks := pc.taskStore.GetAllTasks()

	var filtered []*task.Task
	if filterStmt == nil {
		filtered = allTasks
	} else {
		executor := pc.newExecutor()
		result, err := executor.Execute(filterStmt, allTasks)
		if err != nil {
			slog.Error("failed to execute lane filter", "lane", lane, "error", err)
			return nil
		}
		filtered = result.Select.Tasks
	}

	// narrow by search results if active
	if searchResults := pc.pluginConfig.GetSearchResults(); searchResults != nil {
		searchTaskMap := make(map[string]bool, len(searchResults))
		for _, result := range searchResults {
			searchTaskMap[result.Task.ID] = true
		}
		filtered = filterTasksBySearch(filtered, searchTaskMap)
	}

	return filtered
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
