package controller

import (
	"context"
	"log/slog"
	"strings"

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
		if existing := globalActions.MatchBinding(a.Key, a.Rune, a.Modifier); existing != nil {
			slog.Warn("plugin action key shadows global action and will be unreachable",
				"plugin", pluginDef.Name, "key", a.KeyStr,
				"plugin_action", a.Label, "global_action", existing.Label)
		} else if existing := pc.registry.MatchBinding(a.Key, a.Rune, a.Modifier); existing != nil {
			slog.Warn("plugin action key shadows built-in action and will be unreachable",
				"plugin", pluginDef.Name, "key", a.KeyStr,
				"plugin_action", a.Label, "built_in_action", existing.Label)
		}
		action := Action{
			ID:           pluginActionID(a.KeyStr),
			Key:          a.Key,
			Rune:         a.Rune,
			Modifier:     a.Modifier,
			Label:        a.Label,
			ShowInHeader: a.ShowInHeader,
		}
		if len(a.Require) > 0 {
			reqs := make([]Requirement, len(a.Require))
			for i, r := range a.Require {
				reqs[i] = Requirement(r)
			}
			action.Require = reqs
		}
		pc.registry.Register(action)
	}

	return pc
}

const pluginActionPrefix = "plugin_action:"

// pluginActionID returns an ActionID for a plugin shortcut action key.
func pluginActionID(keyStr string) ActionID {
	return ActionID(pluginActionPrefix + keyStr)
}

// getPluginActionKeyStr extracts the canonical key string from a plugin action ID.
// Returns empty string if the ID is not a plugin action.
func getPluginActionKeyStr(id ActionID) string {
	s := string(id)
	if !strings.HasPrefix(s, pluginActionPrefix) {
		return ""
	}
	return s[len(pluginActionPrefix):]
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
		if keyStr := getPluginActionKeyStr(actionID); keyStr != "" {
			return pc.handlePluginAction(actionID)
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

// getPluginAction looks up a plugin action by ActionID.
func (pc *PluginController) getPluginAction(actionID ActionID) (*plugin.PluginAction, bool) {
	keyStr := getPluginActionKeyStr(actionID)
	if keyStr == "" {
		return nil, false
	}
	for i := range pc.pluginDef.Actions {
		if pc.pluginDef.Actions[i].KeyStr == keyStr {
			return &pc.pluginDef.Actions[i], true
		}
	}
	return nil, false
}

// buildExecutionInput builds the base ExecutionInput for an action, performing
// selection/create-template preflight. Returns ok=false if the action can't run.
func (pc *PluginController) buildExecutionInput(pa *plugin.PluginAction) (ruki.ExecutionInput, bool) {
	input := ruki.ExecutionInput{}
	ids := pc.getSelectedTaskIDs(pc.GetFilteredTasksForLane)

	if !selectionSatisfies(pa.Require, len(ids)) {
		return input, false
	}
	if len(ids) > 0 {
		input.SelectedTaskIDs = ids
	}

	if pa.Action.IsCreate() {
		template, err := pc.taskStore.NewTaskTemplate()
		if err != nil {
			slog.Error("failed to create task template for plugin action", "error", err)
			return input, false
		}
		input.CreateTemplate = template
	}

	return input, true
}

// selectionSatisfies reports whether the current selection count meets the
// action's selection-cardinality requirements. It delegates to the same
// AppContext/ActionEnabled pipeline used for UI enablement so positive,
// negated, and non-selection requirements all evaluate consistently.
//
// Only selection-cardinality attributes are materialized here — other
// attributes (view:*, ai, etc.) are evaluated by the UI layer before an
// action can be dispatched, and re-checking them from the controller would
// either duplicate state plumbing or produce false negatives. We therefore
// ignore non-selection requirements instead of rejecting the action.
func selectionSatisfies(reqs []string, count int) bool {
	ctx := NewAppContext()
	applySelectionCardinality(ctx, count)

	a := Action{Require: selectionRequirements(reqs)}
	return ActionEnabled(a, ctx)
}

// selectionRequirements filters a raw requirement list down to just the
// selection-cardinality tokens (positive or negated) so selectionSatisfies
// only evaluates against the selection context it actually populates.
func selectionRequirements(reqs []string) []Requirement {
	out := make([]Requirement, 0, len(reqs))
	for _, r := range reqs {
		attr := r
		if len(attr) > 0 && attr[0] == '!' {
			attr = attr[1:]
		}
		switch attr {
		case string(RequireID),
			string(RequireSelectionOne),
			string(RequireSelectionAny),
			string(RequireSelectionMany):
			out = append(out, Requirement(r))
		}
	}
	return out
}

// logSelectionFields returns the slog key/value pairs that describe the
// current selection — task_id only when exactly one is selected, otherwise
// just selected_count.
func logSelectionFields(input ruki.ExecutionInput) []any {
	if id, ok := input.SingleSelectedTaskID(); ok {
		return []any{"task_id", id, "selected_count", 1}
	}
	return []any{"selected_count", input.SelectionCount()}
}

// executeAndApply runs the executor and applies the result (store mutations, pipe, clipboard).
func (pc *PluginController) executeAndApply(pa *plugin.PluginAction, input ruki.ExecutionInput) bool {
	executor := pc.newExecutor()
	allTasks := pc.taskStore.GetAllTasks()

	result, err := executor.Execute(pa.Action, allTasks, input)
	if err != nil {
		args := append(logSelectionFields(input), "key", pa.KeyStr, "error", err)
		slog.Error("failed to execute plugin action", args...)
		if pc.statusline != nil {
			pc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
		}
		return false
	}

	ctx := context.Background()
	switch {
	case result.Select != nil:
		args := append(logSelectionFields(input), "key", pa.KeyStr, "label", pa.Label, "matched", len(result.Select.Tasks))
		slog.Info("select plugin action executed", args...)
		return true
	case result.Update != nil:
		for _, updated := range result.Update.Updated {
			if err := pc.mutationGate.UpdateTask(ctx, updated); err != nil {
				slog.Error("failed to update task after plugin action", "task_id", updated.ID, "key", pa.KeyStr, "error", err)
				if pc.statusline != nil {
					pc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
				}
				return false
			}
			pc.ensureSearchResultIncludesTask(updated)
		}
	case result.Create != nil:
		if err := pc.mutationGate.CreateTask(ctx, result.Create.Task); err != nil {
			slog.Error("failed to create task from plugin action", "key", pa.KeyStr, "error", err)
			if pc.statusline != nil {
				pc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
			}
			return false
		}
	case result.Delete != nil:
		for _, deleted := range result.Delete.Deleted {
			if err := pc.mutationGate.DeleteTask(ctx, deleted); err != nil {
				slog.Error("failed to delete task from plugin action", "task_id", deleted.ID, "key", pa.KeyStr, "error", err)
				if pc.statusline != nil {
					pc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
				}
				return false
			}
		}
	case result.Pipe != nil:
		for _, row := range result.Pipe.Rows {
			if err := service.ExecutePipeCommand(ctx, result.Pipe.Command, row); err != nil {
				slog.Error("pipe command failed", "command", result.Pipe.Command, "args", row, "key", pa.KeyStr, "error", err)
				if pc.statusline != nil {
					pc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
				}
			}
		}
	case result.Clipboard != nil:
		if err := service.ExecuteClipboardPipe(result.Clipboard.Rows); err != nil {
			slog.Error("clipboard pipe failed", "key", pa.KeyStr, "error", err)
			if pc.statusline != nil {
				pc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
			}
			return false
		}
		if pc.statusline != nil {
			pc.statusline.SetMessage("copied to clipboard", model.MessageLevelInfo, true)
		}
	}

	args := append(logSelectionFields(input), "key", pa.KeyStr, "label", pa.Label, "plugin", pc.pluginDef.Name)
	slog.Info("plugin action applied", args...)
	return true
}

// handlePluginAction applies a plugin shortcut action. Ruki-kind actions run
// through the executor pipeline; view-kind actions navigate to another view.
func (pc *PluginController) handlePluginAction(actionID ActionID) bool {
	pa, ok := pc.getPluginAction(actionID)
	if !ok {
		return false
	}
	if pa.Kind == plugin.ActionKindView {
		return pc.handleViewAction(pa)
	}
	input, ok := pc.buildExecutionInput(pa)
	if !ok {
		return false
	}
	return pc.executeAndApply(pa, input)
}

// handleViewAction switches to the target view declared by a kind: view action.
// Selection is evaluated against the current lane so an action requiring
// selection:one is blocked when nothing is selected.
func (pc *PluginController) handleViewAction(pa *plugin.PluginAction) bool {
	ids := pc.getSelectedTaskIDs(pc.GetFilteredTasksForLane)
	if !selectionSatisfies(pa.Require, len(ids)) {
		return false
	}
	if pa.TargetView == "" {
		return false
	}
	pc.navController.PushView(model.MakePluginViewID(pa.TargetView), nil)
	return true
}

// GetActionInputSpec returns the prompt and input type for an action, if it has input.
func (pc *PluginController) GetActionInputSpec(actionID ActionID) (string, ruki.ValueType, bool) {
	pa, ok := pc.getPluginAction(actionID)
	if !ok || !pa.HasInput {
		return "", 0, false
	}
	return pa.Label + ": ", pa.InputType, true
}

// CanStartActionInput checks whether an input-backed action can currently run
// (selection/create-template preflight passes).
func (pc *PluginController) CanStartActionInput(actionID ActionID) (string, ruki.ValueType, bool) {
	pa, ok := pc.getPluginAction(actionID)
	if !ok || !pa.HasInput {
		return "", 0, false
	}
	if _, ok := pc.buildExecutionInput(pa); !ok {
		return "", 0, false
	}
	return pa.Label + ": ", pa.InputType, true
}

// HandleActionInput handles submitted text for an input-backed action.
func (pc *PluginController) HandleActionInput(actionID ActionID, text string) InputSubmitResult {
	pa, ok := pc.getPluginAction(actionID)
	if !ok || !pa.HasInput {
		return InputKeepEditing
	}

	val, err := ruki.ParseScalarValue(pa.InputType, text)
	if err != nil {
		if pc.statusline != nil {
			pc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
		}
		return InputKeepEditing
	}

	input, ok := pc.buildExecutionInput(pa)
	if !ok {
		return InputClose
	}
	input.InputValue = val
	input.HasInput = true

	pc.executeAndApply(pa, input)
	return InputClose
}

// GetActionChooseSpec returns the label and whether the action uses choose().
func (pc *PluginController) GetActionChooseSpec(actionID ActionID) (string, bool) {
	pa, ok := pc.getPluginAction(actionID)
	if !ok || !pa.HasChoose {
		return "", false
	}
	return pa.Label, true
}

// CanStartActionChoose checks preflight and evaluates the subquery to build candidates.
func (pc *PluginController) CanStartActionChoose(actionID ActionID) (string, []*task.Task, bool) {
	pa, ok := pc.getPluginAction(actionID)
	if !ok || !pa.HasChoose {
		return "", nil, false
	}
	input, ok := pc.buildExecutionInput(pa)
	if !ok {
		return "", nil, false
	}

	allTasks := pc.taskStore.GetAllTasks()
	executor := pc.newExecutor()
	candidates, err := executor.EvalSubQueryFilter(pa.ChooseFilter, allTasks, input)
	if err != nil {
		slog.Error("failed to evaluate choose filter", "key", pa.KeyStr, "error", err)
		if pc.statusline != nil {
			pc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
		}
		return "", nil, false
	}
	if len(candidates) == 0 {
		if pc.statusline != nil {
			pc.statusline.SetMessage("no matching tasks for choose()", model.MessageLevelError, true)
		}
		return "", nil, false
	}
	return pa.Label, candidates, true
}

// HandleActionChoose handles the selected task ID from the QuickSelect picker.
func (pc *PluginController) HandleActionChoose(actionID ActionID, taskID string) bool {
	pa, ok := pc.getPluginAction(actionID)
	if !ok || !pa.HasChoose {
		return false
	}
	input, ok := pc.buildExecutionInput(pa)
	if !ok {
		return false
	}
	input.ChooseValue = taskID
	input.HasChoose = true
	return pc.executeAndApply(pa, input)
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
	result, err := executor.Execute(actionStmt, allTasks, ruki.NewSingleSelectionInput(taskID))
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
