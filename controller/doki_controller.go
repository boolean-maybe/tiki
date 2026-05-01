package controller

import (
	"log/slog"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/task"
)

// DokiController handles non-board view actions (wiki, detail, search).
// These views are read-only and do not own task filtering/sorting state.
//
// As of Phase 6A, DokiController registers global `kind: view` actions so
// cross-view navigation works from any view kind. Global `kind: ruki` actions
// require the store/mutation/schema plumbing that PluginController owns; they
// are intentionally NOT executed from non-board views in 6A and will land in
// Phase 6B when the executor is hoisted into a shared helper.
type DokiController struct {
	pluginDef     plugin.Plugin
	navController *NavigationController
	statusline    *model.StatuslineConfig
	registry      *ActionRegistry
	globalActions []plugin.PluginAction
}

// NewDokiController creates a controller backing any non-board plugin view.
// globalActions is the workflow's top-level actions list; only `kind: view`
// entries are wired through at Phase 6A.
func NewDokiController(
	pluginDef plugin.Plugin,
	navController *NavigationController,
	statusline *model.StatuslineConfig,
	globalActions []plugin.PluginAction,
) *DokiController {
	dc := &DokiController{
		pluginDef:     pluginDef,
		navController: navController,
		statusline:    statusline,
		registry:      DokiViewActions(),
		globalActions: globalActions,
	}
	dc.mergeGlobalViewActions()
	return dc
}

// mergeGlobalViewActions adds every `kind: view` action from the workflow's
// top-level actions list into this view's action registry. Ruki-kind globals
// are deliberately excluded at 6A — see the type comment.
func (dc *DokiController) mergeGlobalViewActions() {
	for _, ga := range dc.globalActions {
		if ga.Kind != plugin.ActionKindView {
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
// and dispatches global `kind: view` actions to switch to the target view.
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
// In 6A only `kind: view` actions are dispatched; ruki-kind globals are
// silently ignored from non-board views and will be wired in 6B.
func (dc *DokiController) handleGlobalAction(keyStr string) bool {
	for i := range dc.globalActions {
		ga := &dc.globalActions[i]
		if ga.KeyStr != keyStr {
			continue
		}
		if ga.Kind == plugin.ActionKindView {
			if ga.TargetView == "" {
				return false
			}
			dc.navController.PushView(model.MakePluginViewID(ga.TargetView), nil)
			return true
		}
		// ga.Kind == plugin.ActionKindRuki — not wired from non-board views in 6A.
		slog.Debug("ruki-kind global action ignored from non-board view (6B)",
			"key", keyStr, "view", dc.pluginDef.GetName())
		return false
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
