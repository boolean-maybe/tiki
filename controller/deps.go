package controller

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
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
	pluginBase
}

// NewDepsController creates a dependency editor controller.
func NewDepsController(
	taskStore store.Store,
	mutationGate *service.TaskMutationGate,
	pluginConfig *model.PluginConfig,
	pluginDef *plugin.TikiPlugin,
	navController *NavigationController,
	statusline *model.StatuslineConfig,
	schema ruki.Schema,
) *DepsController {
	return &DepsController{
		pluginBase: pluginBase{
			taskStore:     taskStore,
			mutationGate:  mutationGate,
			pluginConfig:  pluginConfig,
			pluginDef:     pluginDef,
			navController: navController,
			statusline:    statusline,
			registry:      DepsViewActions(),
			schema:        schema,
		},
	}
}

func (dc *DepsController) ShowNavigation() bool { return false }

// EnsureFirstNonEmptyLaneSelection delegates to pluginBase with this controller's filter.
func (dc *DepsController) EnsureFirstNonEmptyLaneSelection() bool {
	return dc.pluginBase.EnsureFirstNonEmptyLaneSelection(dc.GetFilteredTasksForLane)
}

// HandleAction routes actions to the appropriate handler.
func (dc *DepsController) HandleAction(actionID ActionID) bool {
	switch actionID {
	case ActionNavUp:
		return dc.handleNav("up", dc.GetFilteredTasksForLane)
	case ActionNavDown:
		return dc.handleNav("down", dc.GetFilteredTasksForLane)
	case ActionNavLeft:
		return dc.handleNav("left", dc.GetFilteredTasksForLane)
	case ActionNavRight:
		return dc.handleNav("right", dc.GetFilteredTasksForLane)
	case ActionMoveTaskLeft:
		return dc.handleMoveTask(-1)
	case ActionMoveTaskRight:
		return dc.handleMoveTask(1)
	case ActionOpenFromPlugin:
		taskID := dc.getSelectedTaskID(dc.GetFilteredTasksForLane)
		if taskID == "" {
			return false
		}
		dc.navController.PushView(model.TaskDetailViewID, model.EncodeTaskDetailParams(model.TaskDetailParams{
			TaskID:   taskID,
			ReadOnly: true,
		}))
		return true
	case ActionNewTask:
		return dc.handleNewTask()
	case ActionDeleteTask:
		return dc.handleDeleteTask(dc.GetFilteredTasksForLane)
	case ActionToggleViewMode:
		dc.pluginConfig.ToggleViewMode()
		return true
	default:
		return false
	}
}

// HandleSearch processes a search query, narrowing visible tasks within each lane.
func (dc *DepsController) HandleSearch(query string) {
	dc.handleSearch(query, func() bool {
		return dc.selectFirstNonEmptyLane(dc.GetFilteredTasksForLane)
	})
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

	movedTaskID := dc.getSelectedTaskID(dc.GetFilteredTasksForLane)
	if movedTaskID == "" {
		return false
	}

	sourceLane := dc.pluginConfig.GetSelectedLane()
	targetLane := sourceLane + offset
	if targetLane < 0 || targetLane >= len(dc.pluginDef.Lanes) {
		return false
	}

	contextTaskID := dc.pluginDef.TaskID

	// build a ruki UPDATE query for the dependency change
	var query string
	switch {
	case sourceLane == depsLaneAll && targetLane == depsLaneBlocks:
		query = fmt.Sprintf(`update where id = "%s" set dependsOn=dependsOn+["%s"]`, movedTaskID, contextTaskID)
	case sourceLane == depsLaneAll && targetLane == depsLaneDepends:
		query = fmt.Sprintf(`update where id = "%s" set dependsOn=dependsOn+["%s"]`, contextTaskID, movedTaskID)
	case sourceLane == depsLaneBlocks && targetLane == depsLaneAll:
		query = fmt.Sprintf(`update where id = "%s" set dependsOn=dependsOn-["%s"]`, movedTaskID, contextTaskID)
	case sourceLane == depsLaneDepends && targetLane == depsLaneAll:
		query = fmt.Sprintf(`update where id = "%s" set dependsOn=dependsOn-["%s"]`, contextTaskID, movedTaskID)
	default:
		return false
	}

	parser := ruki.NewParser(dc.schema)
	stmt, err := parser.ParseAndValidateStatement(query, ruki.ExecutorRuntimePlugin)
	if err != nil {
		slog.Error("deps move: failed to parse ruki query", "query", query, "error", err)
		return false
	}

	executor := dc.newExecutor()
	result, err := executor.Execute(stmt, dc.taskStore.GetAllTasks())
	if err != nil {
		slog.Error("deps move: failed to execute ruki query", "query", query, "error", err)
		return false
	}

	if result.Update == nil || len(result.Update.Updated) == 0 {
		return false
	}

	for _, updated := range result.Update.Updated {
		if err := dc.mutationGate.UpdateTask(context.Background(), updated); err != nil {
			slog.Error("deps move: failed to update task", "task_id", updated.ID, "error", err)
			if dc.statusline != nil {
				dc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
			}
			return false
		}
	}

	dc.selectTaskInLane(targetLane, movedTaskID, dc.GetFilteredTasksForLane)
	return true
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
