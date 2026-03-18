package controller

import (
	"log/slog"
	"strings"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// lane indices for the deps editor
const (
	depsLaneBlocks  = 0
	depsLaneAll     = 1
	depsLaneDepends = 2
)

// DepsController handles the dependency editor plugin view.
// Unlike PluginController, move logic here updates different tasks depending on
// the source/target lane pair — sometimes the moved task, sometimes the context task.
type DepsController struct {
	taskStore     store.Store
	pluginConfig  *model.PluginConfig
	pluginDef     *plugin.TikiPlugin
	navController *NavigationController
	registry      *ActionRegistry
}

// NewDepsController creates a dependency editor controller.
func NewDepsController(
	taskStore store.Store,
	pluginConfig *model.PluginConfig,
	pluginDef *plugin.TikiPlugin,
	navController *NavigationController,
) *DepsController {
	return &DepsController{
		taskStore:     taskStore,
		pluginConfig:  pluginConfig,
		pluginDef:     pluginDef,
		navController: navController,
		registry:      DepsViewActions(),
	}
}

func (dc *DepsController) GetActionRegistry() *ActionRegistry { return dc.registry }
func (dc *DepsController) GetPluginName() string              { return dc.pluginDef.Name }
func (dc *DepsController) ShowNavigation() bool               { return false }

// HandleAction routes actions to the appropriate handler.
func (dc *DepsController) HandleAction(actionID ActionID) bool {
	switch actionID {
	case ActionNavUp:
		return dc.handleNav("up")
	case ActionNavDown:
		return dc.handleNav("down")
	case ActionNavLeft:
		return dc.handleNav("left")
	case ActionNavRight:
		return dc.handleNav("right")
	case ActionMoveTaskLeft:
		return dc.handleMoveTask(-1)
	case ActionMoveTaskRight:
		return dc.handleMoveTask(1)
	case ActionToggleViewMode:
		dc.pluginConfig.ToggleViewMode()
		return true
	default:
		return false
	}
}

// HandleSearch processes a search query, narrowing visible tasks within each lane.
func (dc *DepsController) HandleSearch(query string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return
	}
	dc.pluginConfig.SavePreSearchState()
	results := dc.taskStore.Search(query, nil)
	if len(results) == 0 {
		dc.pluginConfig.SetSearchResults([]task.SearchResult{}, query)
		return
	}
	dc.pluginConfig.SetSearchResults(results, query)
	dc.selectFirstNonEmptyLane()
}

// GetFilteredTasksForLane returns tasks for a given lane of the deps editor.
// Lane 0 (Blocks): tasks whose dependsOn contains the context task.
// Lane 1 (All): all tasks minus context task, blocks set, and depends set.
// Lane 2 (Depends): tasks listed in the context task's dependsOn.
func (dc *DepsController) GetFilteredTasksForLane(lane int) []*task.Task {
	if lane < 0 || lane >= len(dc.pluginDef.Lanes) {
		return nil
	}

	contextTask := dc.taskStore.GetTask(dc.pluginDef.TaskID)
	if contextTask == nil {
		return nil
	}

	allTasks := dc.taskStore.GetAllTasks()
	blocksSet := task.FindBlockedTasks(allTasks, contextTask.ID)
	dependsSet := dc.resolveDependsTasks(contextTask, allTasks)

	var result []*task.Task
	switch lane {
	case depsLaneAll:
		result = dc.computeAllLane(allTasks, contextTask.ID, blocksSet, dependsSet)
	case depsLaneBlocks:
		result = blocksSet
	case depsLaneDepends:
		result = dependsSet
	}

	// narrow by search results if active
	if searchResults := dc.pluginConfig.GetSearchResults(); searchResults != nil {
		searchMap := make(map[string]bool, len(searchResults))
		for _, sr := range searchResults {
			searchMap[sr.Task.ID] = true
		}
		result = filterTasksBySearch(result, searchMap)
	}

	return result
}

// EnsureFirstNonEmptyLaneSelection selects the first non-empty lane if the current lane is empty.
func (dc *DepsController) EnsureFirstNonEmptyLaneSelection() bool {
	currentLane := dc.pluginConfig.GetSelectedLane()
	if currentLane >= 0 && currentLane < len(dc.pluginDef.Lanes) {
		if len(dc.GetFilteredTasksForLane(currentLane)) > 0 {
			return false
		}
	}
	return dc.selectFirstNonEmptyLane()
}

// handleMoveTask applies dependency changes based on the source→target lane transition.
//
//	From → To      | What changes
//	All → Blocks   | moved task: dependsOn += [contextTaskID]
//	All → Depends  | context task: dependsOn += [movedTaskID]
//	Blocks → All   | moved task: dependsOn -= [contextTaskID]
//	Depends → All  | context task: dependsOn -= [movedTaskID]
func (dc *DepsController) handleMoveTask(offset int) bool {
	if offset != -1 && offset != 1 {
		return false
	}

	movedTaskID := dc.getSelectedTaskID()
	if movedTaskID == "" {
		return false
	}

	sourceLane := dc.pluginConfig.GetSelectedLane()
	targetLane := sourceLane + offset
	if targetLane < 0 || targetLane >= len(dc.pluginDef.Lanes) {
		return false
	}

	contextTaskID := dc.pluginDef.TaskID

	// determine which tasks to update and how
	type update struct {
		taskID string
		action plugin.LaneAction
	}
	var updates []update

	switch {
	case sourceLane == depsLaneAll && targetLane == depsLaneBlocks:
		updates = append(updates, update{movedTaskID, depsAction(plugin.ActionOperatorAdd, contextTaskID)})
	case sourceLane == depsLaneAll && targetLane == depsLaneDepends:
		updates = append(updates, update{contextTaskID, depsAction(plugin.ActionOperatorAdd, movedTaskID)})
	case sourceLane == depsLaneBlocks && targetLane == depsLaneAll:
		updates = append(updates, update{movedTaskID, depsAction(plugin.ActionOperatorRemove, contextTaskID)})
	case sourceLane == depsLaneDepends && targetLane == depsLaneAll:
		updates = append(updates, update{contextTaskID, depsAction(plugin.ActionOperatorRemove, movedTaskID)})
	default:
		return false
	}

	for _, u := range updates {
		taskItem := dc.taskStore.GetTask(u.taskID)
		if taskItem == nil {
			slog.Error("deps move: task not found", "task_id", u.taskID)
			return false
		}
		updated, err := plugin.ApplyLaneAction(taskItem, u.action, "")
		if err != nil {
			slog.Error("deps move: failed to apply action", "task_id", u.taskID, "error", err)
			return false
		}
		if err := dc.taskStore.UpdateTask(updated); err != nil {
			slog.Error("deps move: failed to update task", "task_id", u.taskID, "error", err)
			return false
		}
	}

	dc.selectTaskInLane(targetLane, movedTaskID)
	return true
}

// depsAction builds a LaneAction that adds or removes a single task ID from dependsOn.
func depsAction(op plugin.ActionOperator, taskID string) plugin.LaneAction {
	return plugin.LaneAction{
		Ops: []plugin.LaneActionOp{{
			Field:     plugin.ActionFieldDependsOn,
			Operator:  op,
			DependsOn: []string{taskID},
		}},
	}
}

// resolveDependsTasks looks up full task objects for the context task's DependsOn IDs.
func (dc *DepsController) resolveDependsTasks(contextTask *task.Task, allTasks []*task.Task) []*task.Task {
	if len(contextTask.DependsOn) == 0 {
		return nil
	}
	idMap := make(map[string]bool, len(contextTask.DependsOn))
	for _, id := range contextTask.DependsOn {
		idMap[strings.ToUpper(id)] = true
	}
	var result []*task.Task
	for _, t := range allTasks {
		if idMap[t.ID] {
			result = append(result, t)
		}
	}
	return result
}

// computeAllLane returns all tasks minus the context task, blocks set, and depends set.
func (dc *DepsController) computeAllLane(allTasks []*task.Task, contextID string, blocks, depends []*task.Task) []*task.Task {
	exclude := make(map[string]bool, len(blocks)+len(depends)+1)
	exclude[contextID] = true
	for _, t := range blocks {
		exclude[t.ID] = true
	}
	for _, t := range depends {
		exclude[t.ID] = true
	}
	var result []*task.Task
	for _, t := range allTasks {
		if !exclude[t.ID] {
			result = append(result, t)
		}
	}
	return result
}

func (dc *DepsController) handleNav(direction string) bool {
	lane := dc.pluginConfig.GetSelectedLane()
	tasks := dc.GetFilteredTasksForLane(lane)
	if direction == "left" || direction == "right" {
		if dc.pluginConfig.MoveSelection(direction, len(tasks)) {
			return true
		}
		return dc.handleLaneSwitch(direction)
	}
	return dc.pluginConfig.MoveSelection(direction, len(tasks))
}

func (dc *DepsController) handleLaneSwitch(direction string) bool {
	currentLane := dc.pluginConfig.GetSelectedLane()
	nextLane := currentLane
	switch direction {
	case "left":
		nextLane--
	case "right":
		nextLane++
	default:
		return false
	}

	for nextLane >= 0 && nextLane < len(dc.pluginDef.Lanes) {
		tasks := dc.GetFilteredTasksForLane(nextLane)
		if len(tasks) > 0 {
			dc.pluginConfig.SetSelectedLane(nextLane)
			scrollOffset := dc.pluginConfig.GetScrollOffsetForLane(nextLane)
			if scrollOffset >= len(tasks) {
				scrollOffset = len(tasks) - 1
			}
			if scrollOffset < 0 {
				scrollOffset = 0
			}
			dc.pluginConfig.SetSelectedIndexForLane(nextLane, scrollOffset)
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

func (dc *DepsController) getSelectedTaskID() string {
	lane := dc.pluginConfig.GetSelectedLane()
	tasks := dc.GetFilteredTasksForLane(lane)
	idx := dc.pluginConfig.GetSelectedIndexForLane(lane)
	if idx < 0 || idx >= len(tasks) {
		return ""
	}
	return tasks[idx].ID
}

func (dc *DepsController) selectTaskInLane(lane int, taskID string) {
	tasks := dc.GetFilteredTasksForLane(lane)
	targetIndex := 0
	for i, t := range tasks {
		if t.ID == taskID {
			targetIndex = i
			break
		}
	}
	dc.pluginConfig.SetSelectedLane(lane)
	dc.pluginConfig.SetSelectedIndexForLane(lane, targetIndex)
}

func (dc *DepsController) selectFirstNonEmptyLane() bool {
	for lane := range dc.pluginDef.Lanes {
		if len(dc.GetFilteredTasksForLane(lane)) > 0 {
			dc.pluginConfig.SetSelectedLaneAndIndex(lane, 0)
			return true
		}
	}
	return false
}
