package controller

import (
	"github.com/boolean-maybe/tiki/plugin"
)

// DokiController handles doki plugin view actions (documentation/markdown navigation).
// DokiPlugins are read-only documentation views and don't need task filtering/sorting.
type DokiController struct {
	pluginDef     *plugin.DokiPlugin
	navController *NavigationController
	registry      *ActionRegistry
}

// NewDokiController creates a doki controller
func NewDokiController(
	pluginDef *plugin.DokiPlugin,
	navController *NavigationController,
) *DokiController {
	return &DokiController{
		pluginDef:     pluginDef,
		navController: navController,
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
func (dc *DokiController) HandleSearch(query string) {
	// No-op: Doki plugins don't support search
}
