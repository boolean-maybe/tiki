package view

import (
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/view/renderer"
	"github.com/boolean-maybe/tiki/view/taskdetail"
)

// ViewFactory instantiates views by ID, injecting required dependencies.
// It holds references to shared state (stores, configs) needed by views.

// ViewFactory creates views on demand
type ViewFactory struct {
	taskStore   store.Store
	boardConfig *model.BoardConfig
	renderer    renderer.MarkdownRenderer
	// Plugin support
	pluginConfigs     map[string]*model.PluginConfig
	pluginDefs        map[string]plugin.Plugin
	pluginControllers map[string]controller.PluginControllerInterface
}

// NewViewFactory creates a view factory
func NewViewFactory(taskStore store.Store, boardConfig *model.BoardConfig) *ViewFactory {
	// try to create glamour renderer, fallback to plain text if fails
	var mdRenderer renderer.MarkdownRenderer
	glamourRenderer, err := renderer.NewGlamourRenderer()
	if err != nil {
		mdRenderer = &renderer.FallbackRenderer{}
	} else {
		mdRenderer = glamourRenderer
	}

	return &ViewFactory{
		taskStore:   taskStore,
		boardConfig: boardConfig,
		renderer:    mdRenderer,
	}
}

// SetPlugins configures plugin support in the factory
func (f *ViewFactory) SetPlugins(
	configs map[string]*model.PluginConfig,
	defs map[string]plugin.Plugin,
	controllers map[string]controller.PluginControllerInterface,
) {
	f.pluginConfigs = configs
	f.pluginDefs = defs
	f.pluginControllers = controllers
}

// CreateView instantiates a view by ID with optional parameters
func (f *ViewFactory) CreateView(viewID model.ViewID, params map[string]interface{}) controller.View {
	var v controller.View

	switch viewID {
	case model.BoardViewID:
		v = NewBoardView(f.taskStore, f.boardConfig)

	case model.TaskDetailViewID:
		taskID := model.DecodeTaskDetailParams(params).TaskID
		v = taskdetail.NewTaskDetailView(f.taskStore, taskID, f.renderer)

	case model.TaskEditViewID:
		taskID := model.DecodeTaskEditParams(params).TaskID
		v = taskdetail.NewTaskEditView(f.taskStore, taskID, f.renderer)
		editParams := model.DecodeTaskEditParams(params)
		if editParams.Draft != nil {
			if tev, ok := v.(*taskdetail.TaskEditView); ok {
				tev.SetFallbackTask(editParams.Draft)
			}
		}

	default:
		// Check if it's a plugin view
		if model.IsPluginViewID(viewID) {
			pluginName := model.GetPluginName(viewID)
			pluginConfig := f.pluginConfigs[pluginName]
			pluginDef := f.pluginDefs[pluginName]
			pluginControllerInterface := f.pluginControllers[pluginName]

			if pluginDef != nil {
				if tikiPlugin, ok := pluginDef.(*plugin.TikiPlugin); ok && pluginConfig != nil && pluginControllerInterface != nil {
					// For TikiPlugins, we need the specific PluginController for GetFilteredTasks
					if tikiController, ok := pluginControllerInterface.(*controller.PluginController); ok {
						v = NewPluginView(
							f.taskStore,
							pluginConfig,
							tikiPlugin,
							tikiController.GetFilteredTasksForPane,
							tikiController.EnsureFirstNonEmptyPaneSelection,
						)
					} else {
						// Fallback if controller type doesn't match
						v = NewBoardView(f.taskStore, f.boardConfig)
					}
				} else if dokiPlugin, ok := pluginDef.(*plugin.DokiPlugin); ok {
					v = NewDokiView(dokiPlugin, f.renderer)
				} else {
					// Unknown plugin type or missing config/controller for tiki
					v = NewBoardView(f.taskStore, f.boardConfig)
				}
			} else {
				// Fallback if plugin not found
				v = NewBoardView(f.taskStore, f.boardConfig)
			}
		} else {
			// fallback to board view
			v = NewBoardView(f.taskStore, f.boardConfig)
		}
	}

	return v
}
