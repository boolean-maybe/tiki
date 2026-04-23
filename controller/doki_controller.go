package controller

import (
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/task"
)

// DokiController handles doki plugin view actions (documentation/markdown navigation).
// DokiPlugins are read-only documentation views and don't need task filtering/sorting.
type DokiController struct {
	pluginDef     *plugin.DokiPlugin
	navController *NavigationController
	statusline    *model.StatuslineConfig
	registry      *ActionRegistry
}

// NewDokiController creates a doki controller
func NewDokiController(
	pluginDef *plugin.DokiPlugin,
	navController *NavigationController,
	statusline *model.StatuslineConfig,
) *DokiController {
	return &DokiController{
		pluginDef:     pluginDef,
		navController: navController,
		statusline:    statusline,
		registry:      DokiViewActions(),
	}
}

// GetActionRegistry returns the actions for the doki view
func (dc *DokiController) GetActionRegistry() *ActionRegistry {
	return dc.registry
}

// GetPluginName returns the plugin name
func (dc *DokiController) GetPluginName() string {
	return dc.pluginDef.Name
}

// ShowNavigation returns true — doki views show plugin navigation keys.
func (dc *DokiController) ShowNavigation() bool { return true }

// HandleAction processes a doki action
// Note: Most doki actions (Tab, Shift+Tab, Alt+Left, Alt+Right) are handled
// directly by the NavigableMarkdown component in the view. The controller
// just needs to return false to allow the view to handle them.
func (dc *DokiController) HandleAction(actionID ActionID) bool {
	switch actionID {
	case ActionNavigateBack:
		// Let the view's NavigableMarkdown component handle this
		return false
	case ActionNavigateForward:
		// Let the view's NavigableMarkdown component handle this
		return false
	default:
		return false
	}
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
