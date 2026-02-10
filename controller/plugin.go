package controller

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

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
	pc := &PluginController{
		taskStore:     taskStore,
		pluginConfig:  pluginConfig,
		pluginDef:     pluginDef,
		navController: navController,
		registry:      PluginViewActions(),
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
		if r := getPluginActionRune(actionID); r != 0 {
			return pc.handlePluginAction(r)
		}
		return false
	}
}

func (pc *PluginController) handleNav(direction string) bool {
	lane := pc.pluginConfig.GetSelectedLane()
	tasks := pc.GetFilteredTasksForLane(lane)
	if direction == "left" || direction == "right" {
		if pc.pluginConfig.MoveSelection(direction, len(tasks)) {
			return true
		}
		return pc.handleLaneSwitch(direction)
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

func (pc *PluginController) handleLaneSwitch(direction string) bool {
	currentLane := pc.pluginConfig.GetSelectedLane()
	nextLane := currentLane
	switch direction {
	case "left":
		nextLane--
	case "right":
		nextLane++
	default:
		return false
	}

	for nextLane >= 0 && nextLane < len(pc.pluginDef.Lanes) {
		tasks := pc.GetFilteredTasksForLane(nextLane)
		if len(tasks) > 0 {
			pc.pluginConfig.SetSelectedLane(nextLane)
			// Select the task at top of viewport (scroll offset) rather than keeping stale index
			scrollOffset := pc.pluginConfig.GetScrollOffsetForLane(nextLane)
			if scrollOffset >= len(tasks) {
				scrollOffset = len(tasks) - 1
			}
			if scrollOffset < 0 {
				scrollOffset = 0
			}
			pc.pluginConfig.SetSelectedIndexForLane(nextLane, scrollOffset)
			return true
		}
		switch direction {
		case "left":
			nextLane--
		case "right":
			nextLane++
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

	taskID := pc.getSelectedTaskID()
	if taskID == "" {
		return false
	}

	taskItem := pc.taskStore.GetTask(taskID)
	if taskItem == nil {
		return false
	}

	currentUser := getCurrentUserName(pc.taskStore)
	updated, err := plugin.ApplyLaneAction(taskItem, pa.Action, currentUser)
	if err != nil {
		slog.Error("failed to apply plugin action", "task_id", taskID, "key", string(r), "error", err)
		return false
	}

	if err := pc.taskStore.UpdateTask(updated); err != nil {
		slog.Error("failed to update task after plugin action", "task_id", taskID, "key", string(r), "error", err)
		return false
	}

	pc.ensureSearchResultIncludesTask(updated)
	slog.Info("plugin action applied", "task_id", taskID, "key", string(r), "label", pa.Label, "plugin", pc.pluginDef.Name)
	return true
}

func (pc *PluginController) handleMoveTask(offset int) bool {
	taskID := pc.getSelectedTaskID()
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

	taskItem := pc.taskStore.GetTask(taskID)
	if taskItem == nil {
		return false
	}

	currentUser := getCurrentUserName(pc.taskStore)
	updated, err := plugin.ApplyLaneAction(taskItem, pc.pluginDef.Lanes[targetLane].Action, currentUser)
	if err != nil {
		slog.Error("failed to apply lane action", "task_id", taskID, "error", err)
		return false
	}

	if err := pc.taskStore.UpdateTask(updated); err != nil {
		slog.Error("failed to update task after lane move", "task_id", taskID, "error", err)
		return false
	}

	pc.ensureSearchResultIncludesTask(updated)
	pc.selectTaskInLane(targetLane, taskID)
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

	// Search across all tasks; lane membership is decided per lane
	results := pc.taskStore.Search(query, nil)
	if len(results) == 0 {
		pc.pluginConfig.SetSearchResults([]task.SearchResult{}, query)
		return
	}

	pc.pluginConfig.SetSearchResults(results, query)
	if pc.selectFirstNonEmptyLane() {
		return
	}
}

// getSelectedTaskID returns the ID of the currently selected task
func (pc *PluginController) getSelectedTaskID() string {
	lane := pc.pluginConfig.GetSelectedLane()
	tasks := pc.GetFilteredTasksForLane(lane)
	idx := pc.pluginConfig.GetSelectedIndexForLane(lane)
	if idx < 0 || idx >= len(tasks) {
		return ""
	}
	return tasks[idx].ID
}

// GetFilteredTasksForLane returns tasks filtered and sorted for a specific lane.
func (pc *PluginController) GetFilteredTasksForLane(lane int) []*task.Task {
	if pc.pluginDef == nil {
		return nil
	}
	if lane < 0 || lane >= len(pc.pluginDef.Lanes) {
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
		laneFilter := pc.pluginDef.Lanes[lane].Filter
		if laneFilter == nil || laneFilter.Evaluate(task, now, currentUser) {
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

func (pc *PluginController) selectTaskInLane(lane int, taskID string) {
	if lane < 0 || lane >= len(pc.pluginDef.Lanes) {
		return
	}

	tasks := pc.GetFilteredTasksForLane(lane)
	targetIndex := 0
	for i, task := range tasks {
		if task.ID == taskID {
			targetIndex = i
			break
		}
	}

	pc.pluginConfig.SetSelectedLane(lane)
	pc.pluginConfig.SetSelectedIndexForLane(lane, targetIndex)
}

func (pc *PluginController) selectFirstNonEmptyLane() bool {
	for lane := range pc.pluginDef.Lanes {
		tasks := pc.GetFilteredTasksForLane(lane)
		if len(tasks) > 0 {
			pc.pluginConfig.SetSelectedLaneAndIndex(lane, 0)
			return true
		}
	}
	return false
}

func (pc *PluginController) EnsureFirstNonEmptyLaneSelection() bool {
	if pc.pluginDef == nil {
		return false
	}
	currentLane := pc.pluginConfig.GetSelectedLane()
	if currentLane >= 0 && currentLane < len(pc.pluginDef.Lanes) {
		tasks := pc.GetFilteredTasksForLane(currentLane)
		if len(tasks) > 0 {
			return false
		}
	}
	return pc.selectFirstNonEmptyLane()
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
