package controller

import (
	"log/slog"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	"github.com/gdamore/tcell/v2"
)

// DetailController backs `kind: detail` views. It surfaces both per-view
// actions (declared on the view itself) and global actions, dispatches
// `kind: view` navigations with selection passthrough, and routes ruki
// actions through the shared executor pipeline used by board views.
//
// Phase 1 scope: read-only view, fullscreen toggle, action dispatch.
// In-place edit mode lands in Phase 2.
type DetailController struct {
	pluginDef      *plugin.DetailPlugin
	navController  *NavigationController
	statusline     *model.StatuslineConfig
	registry       *ActionRegistry
	executor       *PluginExecutor
	selectedTaskID string
}

// NewDetailController builds a controller for a kind: detail plugin view.
// taskStore / mutationGate / schema may be nil only in trivial test fixtures
// that don't exercise ruki actions; in normal use the executor is wired so
// per-view ruki actions can fire.
func NewDetailController(
	pluginDef *plugin.DetailPlugin,
	navController *NavigationController,
	statusline *model.StatuslineConfig,
	taskStore store.Store,
	mutationGate *service.TaskMutationGate,
	schema ruki.Schema,
) *DetailController {
	dc := &DetailController{
		pluginDef:     pluginDef,
		navController: navController,
		statusline:    statusline,
		registry:      DetailViewActions(),
	}
	if taskStore != nil && mutationGate != nil && schema != nil {
		dc.executor = NewPluginExecutor(taskStore, mutationGate, statusline, schema,
			pluginDef.GetName(), nil)
	}
	dc.registerPluginActions()
	return dc
}

// registerPluginActions adds the plugin's per-view (and merged global) actions
// to the registry. Mirrors the surfacing rules from PluginController so the
// header/palette show the same set the controller can actually fire.
func (dc *DetailController) registerPluginActions() {
	for _, a := range dc.pluginDef.Actions {
		switch a.Kind {
		case plugin.ActionKindView:
			// surface unconditionally; navigation has no executor deps
		case plugin.ActionKindRuki:
			if dc.executor == nil {
				continue
			}
			if a.HasInput || a.HasChoose {
				slog.Debug("interactive ruki action not surfaced on detail view",
					"view", dc.pluginDef.Name, "key", a.KeyStr)
				continue
			}
		default:
			continue
		}
		dc.registry.Register(Action{
			ID:           pluginActionID(a.KeyStr),
			Key:          a.Key,
			Rune:         a.Rune,
			Modifier:     a.Modifier,
			Label:        a.Label,
			ShowInHeader: a.ShowInHeader,
			Require:      toRequirements(a.Require),
		})
	}
}

// SetSelectedTaskID updates the carried selection. Called by the harness
// when navigation params arrive after construction.
func (dc *DetailController) SetSelectedTaskID(id string) {
	dc.selectedTaskID = id
}

// GetActionRegistry returns the view's action registry.
func (dc *DetailController) GetActionRegistry() *ActionRegistry { return dc.registry }

// GetPluginName returns the plugin name.
func (dc *DetailController) GetPluginName() string { return dc.pluginDef.Name }

// ShowNavigation returns false — detail views don't show plugin nav keys.
func (dc *DetailController) ShowNavigation() bool { return false }

// HandleAction routes plugin actions. Built-in detail actions like
// Fullscreen are handled by the view itself via the input router; this
// method dispatches workflow-declared actions.
func (dc *DetailController) HandleAction(actionID ActionID) bool {
	if keyStr := getPluginActionKeyStr(actionID); keyStr != "" {
		return dc.handlePluginAction(keyStr)
	}
	return false
}

// handlePluginAction dispatches a plugin action by canonical key string.
func (dc *DetailController) handlePluginAction(keyStr string) bool {
	for i := range dc.pluginDef.Actions {
		a := &dc.pluginDef.Actions[i]
		if a.KeyStr != keyStr {
			continue
		}
		switch a.Kind {
		case plugin.ActionKindView:
			return dc.dispatchViewAction(a)
		case plugin.ActionKindRuki:
			return dc.dispatchRukiAction(a)
		}
	}
	return false
}

// dispatchViewAction navigates to the target view, carrying the current
// selection as PluginViewParams.
//
// Self-target actions (target == this plugin's name) are refused as a
// belt-and-suspenders guard. The loader already filters these out of the
// per-view Actions slice; this catches any case where an author declares
// the same action per-view, where a dynamic plugin path injects one, or
// where a future merge change reintroduces them. Without the guard, Enter
// on Detail would push another Detail copy onto the stack indefinitely.
func (dc *DetailController) dispatchViewAction(a *plugin.PluginAction) bool {
	if a.TargetView == "" {
		return false
	}
	if a.TargetView == dc.pluginDef.Name {
		return false
	}
	carried := 0
	if dc.selectedTaskID != "" {
		carried = 1
	}
	if !TargetViewEnabled(a.TargetView, carried) {
		return false
	}
	var params map[string]interface{}
	if dc.selectedTaskID != "" {
		params = model.EncodePluginViewParams(model.PluginViewParams{TaskID: dc.selectedTaskID})
	}
	dc.navController.PushView(model.MakePluginViewID(a.TargetView), params)
	return true
}

// dispatchRukiAction runs a ruki-kind action through the shared executor.
func (dc *DetailController) dispatchRukiAction(a *plugin.PluginAction) bool {
	if dc.executor == nil {
		return false
	}
	if a.HasInput || a.HasChoose {
		// belt-and-suspenders: filtered out at registration too
		return false
	}
	var selection []string
	if dc.selectedTaskID != "" {
		selection = []string{dc.selectedTaskID}
	}
	input, ok := dc.executor.BuildExecutionInput(a, selection)
	if !ok {
		return false
	}
	return dc.executor.Execute(a, input)
}

// HandleSearch is a no-op for detail views.
func (dc *DetailController) HandleSearch(string) {}

// Phase 1 stubs for input/choose pipelines. Phase 2 may extend these as
// editor support lands; for now interactive ruki actions are filtered out
// at registration time so these are not reached for detail views.
func (dc *DetailController) GetActionInputSpec(ActionID) (string, ruki.ValueType, bool) {
	return "", 0, false
}
func (dc *DetailController) CanStartActionInput(ActionID) (string, ruki.ValueType, bool) {
	return "", 0, false
}
func (dc *DetailController) HandleActionInput(ActionID, string) InputSubmitResult {
	return InputKeepEditing
}
func (dc *DetailController) GetActionChooseSpec(ActionID) (string, bool) { return "", false }
func (dc *DetailController) CanStartActionChoose(ActionID) (string, []*tikipkg.Tiki, bool) {
	return "", nil, false
}
func (dc *DetailController) HandleActionChoose(ActionID, string) bool { return false }

// DetailViewActions returns the built-in action registry for kind: detail.
// Phase 1 surfaces Fullscreen and a stub Edit entry; the latter is gated
// off until Phase 2 wires in-place edit mode.
func DetailViewActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionFullscreen, Key: tcell.KeyRune, Rune: 'f', Label: "Full screen", ShowInHeader: true})
	// Edit is registered with a stub ID so Phase 2 can replace the handler
	// without changing the keybinding contract. Today the controller has no
	// handler for it, so pressing 'e' is a no-op (predictable stub).
	r.Register(Action{ID: ActionDetailEditStub, Key: tcell.KeyRune, Rune: 'e', Label: "Edit (stub)", ShowInHeader: true, Require: []Requirement{RequireID}})
	return r
}
