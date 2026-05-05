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
	tikipkg "github.com/boolean-maybe/tiki/tiki"
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
	// detailViewResolver returns the name of the configurable detail
	// view to open when the user presses Enter on a row in the deps
	// editor. The resolver runs at dispatch time (not construction
	// time) so workflows that reload or rename their detail views
	// don't leave the controller pointing at a stale name. Returns
	// the empty string when no detail view is available; callers must
	// then refuse the open.
	detailViewResolver func() string
}

// NewDepsController creates a dependency editor controller. The
// detailViewResolver is consulted on each Open action to find the
// target detail view name; passing nil disables the Open action
// (returns false). The router supplies a resolver that prefers the
// view the deps editor was opened from, then falls back to any other
// kind: detail plugin loaded from workflow.yaml.
func NewDepsController(
	taskStore store.Store,
	mutationGate *service.TaskMutationGate,
	pluginConfig *model.PluginConfig,
	pluginDef *plugin.TikiPlugin,
	navController *NavigationController,
	statusline *model.StatuslineConfig,
	schema ruki.Schema,
	detailViewResolver func() string,
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
		detailViewResolver: detailViewResolver,
	}
}

// SetDetailViewResolver swaps the resolver used by the Open action.
// Called by the router when a deps editor is reopened from a different
// configurable detail view, so Enter routes back to the most recent
// caller rather than the original one captured when this controller
// was first constructed. Passing nil disables Open until a resolver is
// installed again.
func (dc *DepsController) SetDetailViewResolver(fn func() string) {
	dc.detailViewResolver = fn
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
		// Phase 3: route deps-editor "Open" through a configurable
		// detail view. The resolver hands back either the view this
		// editor was opened from or a fallback kind: detail plugin;
		// without it (or when it returns ""), the workflow has no
		// detail view loaded and we refuse the open instead of
		// pushing a missing plugin id onto the nav stack.
		if dc.detailViewResolver == nil {
			return false
		}
		targetName := dc.detailViewResolver()
		if targetName == "" {
			return false
		}
		dc.navController.PushView(
			model.MakePluginViewID(targetName),
			model.EncodePluginViewParams(model.PluginViewParams{TaskID: taskID}),
		)
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

// GetFilteredTasksForLane returns tikis for a given lane of the deps editor.
// Lane 0 (Blocks): tikis whose dependsOn contains the context tiki.
// Lane 1 (All): all tikis minus context tiki, blocks set, and depends set.
// Lane 2 (Depends): tikis listed in the context tiki's dependsOn.
func (dc *DepsController) GetFilteredTasksForLane(lane int) []*tikipkg.Tiki {
	if lane < 0 || lane >= len(dc.pluginDef.Lanes) {
		return nil
	}

	contextTiki := dc.taskStore.GetTiki(dc.pluginDef.TaskID)
	if contextTiki == nil {
		return nil
	}

	allTikis := dc.taskStore.GetAllTikis()
	blocksSet := findBlockedTikis(allTikis, contextTiki.ID)
	dependsSet := dc.resolveDependsTikis(contextTiki, allTikis)

	var result []*tikipkg.Tiki
	switch lane {
	case depsLaneAll:
		result = dc.computeAllLane(allTikis, contextTiki.ID, blocksSet, dependsSet)
	case depsLaneBlocks:
		result = blocksSet
	case depsLaneDepends:
		result = dependsSet
	}
	sortTikisByPriorityTitle(result)

	// narrow by search results if active
	if searchResults := dc.pluginConfig.GetSearchResults(); searchResults != nil {
		searchMap := make(map[string]bool, len(searchResults))
		for _, tk := range searchResults {
			searchMap[tk.ID] = true
		}
		result = filterTikisBySearch(result, searchMap)
	}

	return result
}

// handleMoveTask applies dependency changes based on the source→target lane transition.
//
//	From → To      | What changes
//	All → Blocks   | moved tiki: dependsOn += [contextTikiID]
//	All → Depends  | context tiki: dependsOn += [movedTikiID]
//	Blocks → All   | moved tiki: dependsOn -= [contextTikiID]
//	Depends → All  | context tiki: dependsOn -= [movedTikiID]
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

	// Build a ruki UPDATE query for the dependency change. Phase 4's
	// assignment-RHS auto-zero carve-out treats absent dependsOn as an
	// empty list during `+`/`-` arithmetic, so the same statement shape
	// covers both "target already has dependsOn" and "target has none."
	var query string
	switch {
	case sourceLane == depsLaneAll && targetLane == depsLaneBlocks:
		query = fmt.Sprintf(`update where id = "%s" set dependsOn = dependsOn + ["%s"]`, movedTaskID, contextTaskID)
	case sourceLane == depsLaneAll && targetLane == depsLaneDepends:
		query = fmt.Sprintf(`update where id = "%s" set dependsOn = dependsOn + ["%s"]`, contextTaskID, movedTaskID)
	case sourceLane == depsLaneBlocks && targetLane == depsLaneAll:
		query = fmt.Sprintf(`update where id = "%s" set dependsOn = dependsOn - ["%s"]`, movedTaskID, contextTaskID)
	case sourceLane == depsLaneDepends && targetLane == depsLaneAll:
		query = fmt.Sprintf(`update where id = "%s" set dependsOn = dependsOn - ["%s"]`, contextTaskID, movedTaskID)
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
	result, err := executor.Execute(stmt, dc.taskStore.GetAllTikis())
	if err != nil {
		slog.Error("deps move: failed to execute ruki query", "query", query, "error", err)
		return false
	}

	if result.Update == nil || len(result.Update.Updated) == 0 {
		return false
	}

	for _, tk := range result.Update.Updated {
		if err := dc.mutationGate.UpdateTiki(context.Background(), tk); err != nil {
			slog.Error("deps move: failed to update task", "task_id", tk.ID, "error", err)
			if dc.statusline != nil {
				dc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
			}
			return false
		}
	}

	dc.selectTaskInLane(targetLane, movedTaskID, dc.GetFilteredTasksForLane)
	return true
}

// resolveDependsTikis looks up full tiki objects for the context tiki's dependsOn IDs.
func (dc *DepsController) resolveDependsTikis(contextTiki *tikipkg.Tiki, allTikis []*tikipkg.Tiki) []*tikipkg.Tiki {
	deps, _, _ := contextTiki.StringSliceField("dependsOn")
	if len(deps) == 0 {
		return nil
	}
	idMap := make(map[string]bool, len(deps))
	for _, id := range deps {
		idMap[strings.ToUpper(id)] = true
	}
	var result []*tikipkg.Tiki
	for _, tk := range allTikis {
		if idMap[tk.ID] {
			result = append(result, tk)
		}
	}
	return result
}

// computeAllLane returns all tikis minus the context tiki, blocks set, and depends set.
func (dc *DepsController) computeAllLane(allTikis []*tikipkg.Tiki, contextID string, blocks, depends []*tikipkg.Tiki) []*tikipkg.Tiki {
	exclude := make(map[string]bool, len(blocks)+len(depends)+1)
	exclude[contextID] = true
	for _, tk := range blocks {
		exclude[tk.ID] = true
	}
	for _, tk := range depends {
		exclude[tk.ID] = true
	}
	var result []*tikipkg.Tiki
	for _, tk := range allTikis {
		if !exclude[tk.ID] {
			result = append(result, tk)
		}
	}
	return result
}

// findBlockedTikis returns all tikis whose dependsOn contains the given ID.
// This is the tiki-native equivalent of task.FindBlockedTasks.
func findBlockedTikis(allTikis []*tikipkg.Tiki, contextID string) []*tikipkg.Tiki {
	var result []*tikipkg.Tiki
	for _, tk := range allTikis {
		deps, _, _ := tk.StringSliceField("dependsOn")
		for _, dep := range deps {
			if strings.ToUpper(dep) == contextID {
				result = append(result, tk)
				break
			}
		}
	}
	return result
}
