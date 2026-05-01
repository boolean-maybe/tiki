package controller

import (
	"log/slog"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// DokiController handles non-board view actions (wiki, detail, search).
// These views are read-only and do not own task filtering/sorting state.
//
// Global-action wiring:
//   - `kind: view` actions dispatch through HandleAction → handleGlobalAction
//     to switch to the named view; the current selection (carried in from
//     the source view's navigation params) propagates into the target.
//   - `kind: ruki` actions dispatch through the shared PluginExecutor so
//     the same mutation/pipe/clipboard pipeline used by board views fires
//     here too. If the view was navigated to with a selected task id, that
//     id is carried into the ruki ExecutionInput.
type DokiController struct {
	pluginDef      plugin.Plugin
	navController  *NavigationController
	statusline     *model.StatuslineConfig
	registry       *ActionRegistry
	globalActions  []plugin.PluginAction
	executor       *PluginExecutor
	selectedTaskID string // 6B.3: carried from source view via PluginViewParams
}

// NewDokiController creates a controller backing any non-board plugin view.
// globalActions is the workflow's top-level actions list; both `kind: view`
// and `kind: ruki` entries are wired through. taskStore / mutationGate /
// schema may be nil for minimal test fixtures that don't exercise ruki
// globals (in which case kind: ruki actions silently fall through).
func NewDokiController(
	pluginDef plugin.Plugin,
	navController *NavigationController,
	statusline *model.StatuslineConfig,
	globalActions []plugin.PluginAction,
	taskStore store.Store,
	mutationGate *service.TaskMutationGate,
	schema ruki.Schema,
) *DokiController {
	dc := &DokiController{
		pluginDef:     pluginDef,
		navController: navController,
		statusline:    statusline,
		registry:      DokiViewActions(),
		globalActions: globalActions,
	}
	if taskStore != nil && mutationGate != nil && schema != nil {
		dc.executor = NewPluginExecutor(taskStore, mutationGate, statusline, schema,
			pluginDef.GetName(), nil)
	}
	dc.mergeGlobalActions()
	return dc
}

// mergeGlobalActions registers surfaceable top-level actions into this
// view's action registry so header + palette see them, and keyboard
// dispatch routes through HandleAction below.
//
// Not everything is surfaced:
//   - view-kind actions surface unconditionally.
//   - ruki-kind actions surface only when the executor is wired AND the
//     action is non-interactive. The doki controller does not implement
//     the input/choose pipeline (GetActionInputSpec / GetActionChooseSpec
//     return empty), so dispatching an `input:` or `choose()` action here
//     would fire it with an uninitialized prompt. Filter them out at
//     registration time so the UI reflects what can actually run.
//     Implementing the interactive pipeline on non-board views is a
//     future enhancement; see phase6.md.
func (dc *DokiController) mergeGlobalActions() {
	for _, ga := range dc.globalActions {
		switch ga.Kind {
		case plugin.ActionKindView:
			// surfaced unconditionally; navigation has no store deps
		case plugin.ActionKindRuki:
			if dc.executor == nil {
				// no executor plumbing; don't surface an action we can't fire
				continue
			}
			if ga.HasInput || ga.HasChoose {
				slog.Debug("interactive ruki global not surfaced on non-board view",
					"view", dc.pluginDef.GetName(), "key", ga.KeyStr, "label", ga.Label,
					"input", ga.HasInput, "choose", ga.HasChoose)
				continue
			}
		default:
			continue
		}
		actionID := ActionID("plugin_action:" + ga.KeyStr)
		dc.registry.Register(Action{
			ID:           actionID,
			Key:          ga.Key,
			Rune:         ga.Rune,
			Modifier:     ga.Modifier,
			Label:        ga.Label,
			ShowInHeader: ga.ShowInHeader,
			Require:      toRequirements(ga.Require),
		})
	}
}

// toRequirements converts the plugin action's []string requirements to the
// controller's []Requirement slice. Kept local to avoid a wider dependency
// shift across action packages.
func toRequirements(raw []string) []Requirement {
	if len(raw) == 0 {
		return nil
	}
	out := make([]Requirement, 0, len(raw))
	for _, r := range raw {
		out = append(out, Requirement(r))
	}
	return out
}

// SetSelectedTaskID records the task id this view was navigated to with.
// Invoked by the view layer after decoding PluginViewParams so outbound
// `kind: view` actions and inbound `kind: ruki` globals see the selection.
func (dc *DokiController) SetSelectedTaskID(id string) {
	dc.selectedTaskID = id
}

// GetActionRegistry returns the actions for the view
func (dc *DokiController) GetActionRegistry() *ActionRegistry {
	return dc.registry
}

// GetPluginName returns the plugin name
func (dc *DokiController) GetPluginName() string {
	return dc.pluginDef.GetName()
}

// ShowNavigation returns true — doki views show plugin navigation keys.
func (dc *DokiController) ShowNavigation() bool { return true }

// HandleAction routes navigation actions to the NavigableMarkdown component
// and dispatches global actions (both `kind: view` and `kind: ruki`).
func (dc *DokiController) HandleAction(actionID ActionID) bool {
	switch actionID {
	case ActionNavigateBack, ActionNavigateForward:
		// handled by the view's NavigableMarkdown component
		return false
	}
	if keyStr := getPluginActionKeyStr(actionID); keyStr != "" {
		return dc.handleGlobalAction(keyStr)
	}
	return false
}

// handleGlobalAction dispatches a global action by its canonical key string.
// View-kind actions switch the current view. Ruki-kind actions run through
// the shared PluginExecutor against an empty selection (doki views have no
// intrinsic selection; selection passthrough from a source view lands in
// 6B.3).
func (dc *DokiController) handleGlobalAction(keyStr string) bool {
	for i := range dc.globalActions {
		ga := &dc.globalActions[i]
		if ga.KeyStr != keyStr {
			continue
		}
		switch ga.Kind {
		case plugin.ActionKindView:
			if ga.TargetView == "" {
				return false
			}
			// 6B.15/6B.20/6B.22: honor the target view's own require:
			// in full. The doki view's carried selection is 0 or 1
			// depending on whether a task id was threaded in via
			// PluginViewParams; the target context is built fresh
			// from the target's own identity so view:* requirements
			// resolve against the target, not this view.
			selected := 0
			if dc.selectedTaskID != "" {
				selected = 1
			}
			if !TargetViewEnabled(ga.TargetView, selected) {
				return false
			}
			var params map[string]interface{}
			if dc.selectedTaskID != "" {
				params = model.EncodePluginViewParams(model.PluginViewParams{TaskID: dc.selectedTaskID})
			}
			dc.navController.PushView(model.MakePluginViewID(ga.TargetView), params)
			return true
		case plugin.ActionKindRuki:
			if dc.executor == nil {
				return false
			}
			// 6B.13 belt-and-suspenders: mergeGlobalActions filters
			// HasInput/HasChoose actions out of the registry so they
			// shouldn't reach here, but an action registered via a
			// different path (future code) could. Refuse rather than
			// fire with an empty input/choose payload.
			if ga.HasInput || ga.HasChoose {
				slog.Debug("interactive ruki global refused on non-board view",
					"view", dc.pluginDef.GetName(), "key", keyStr)
				return false
			}
			var selection []string
			if dc.selectedTaskID != "" {
				selection = []string{dc.selectedTaskID}
			}
			input, ok := dc.executor.BuildExecutionInput(ga, selection)
			if !ok {
				return false
			}
			return dc.executor.Execute(ga, input)
		}
	}
	return false
}

// HandleSearch is not applicable for DokiPlugins (documentation views don't have search)
func (dc *DokiController) HandleSearch(query string) {}

func (dc *DokiController) GetActionInputSpec(ActionID) (string, ruki.ValueType, bool) {
	return "", 0, false
}
func (dc *DokiController) CanStartActionInput(ActionID) (string, ruki.ValueType, bool) {
	return "", 0, false
}
func (dc *DokiController) HandleActionInput(ActionID, string) InputSubmitResult {
	return InputKeepEditing
}
func (dc *DokiController) GetActionChooseSpec(ActionID) (string, bool) { return "", false }
func (dc *DokiController) CanStartActionChoose(ActionID) (string, []*task.Task, bool) {
	return "", nil, false
}
func (dc *DokiController) HandleActionChoose(ActionID, string) bool { return false }
