package view

import (
	"log/slog"

	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/util"
	"github.com/boolean-maybe/tiki/view/taskdetail"
)

// ViewFactory instantiates views by ID, injecting required dependencies.
// It holds references to shared state (stores, configs) needed by views.

// ViewFactory creates views on demand
type ViewFactory struct {
	taskStore    store.Store
	imageManager *navtview.ImageManager
	mermaidOpts  *nav.MermaidOptions
	// Plugin support
	pluginConfigs     map[string]*model.PluginConfig
	pluginDefs        map[string]plugin.Plugin
	pluginControllers map[string]controller.PluginControllerInterface
	globalActions     []plugin.PluginAction
}

// NewViewFactory creates a view factory
func NewViewFactory(taskStore store.Store) *ViewFactory {
	// Configure image resolver with the unified document root as the primary
	// search root so images referenced from nested or root-level `.md`
	// documents resolve (e.g. `.doc/projects/foo/diagram.png` or
	// `.doc/assets/logo.png`). The legacy task directory is kept as a
	// fallback so existing projects with `.doc/tiki/markdown.png` keep
	// rendering during the Phase 2 → Phase 8 transition.
	searchRoots := []string{config.GetDocDir(), config.GetTaskDir()}
	resolver := nav.NewImageResolver(searchRoots)
	resolver.SetDarkMode(!config.IsLightTheme())
	imgMgr := navtview.NewImageManager(resolver, 8, 16)
	imgMgr.SetMaxRows(config.GetMaxImageRows())
	imgMgr.SetSupported(util.SupportsKittyGraphics())

	return &ViewFactory{
		taskStore:    taskStore,
		imageManager: imgMgr,
		mermaidOpts:  &nav.MermaidOptions{},
	}
}

// SetPlugins configures plugin support in the factory. globalActions carries
// the workflow's top-level `actions:` list so non-board views can surface
// `kind: view` entries in their own registry.
func (f *ViewFactory) SetPlugins(
	configs map[string]*model.PluginConfig,
	defs map[string]plugin.Plugin,
	controllers map[string]controller.PluginControllerInterface,
	globalActions []plugin.PluginAction,
) {
	f.pluginConfigs = configs
	f.pluginDefs = defs
	f.pluginControllers = controllers
	f.globalActions = globalActions
}

// RegisterPlugin registers a dynamically created plugin (e.g., deps editor) with the view factory.
func (f *ViewFactory) RegisterPlugin(name string, cfg *model.PluginConfig, def plugin.Plugin, ctrl controller.PluginControllerInterface) {
	f.pluginConfigs[name] = cfg
	f.pluginDefs[name] = def
	f.pluginControllers[name] = ctrl
}

// CreateView instantiates a view by ID with optional parameters
func (f *ViewFactory) CreateView(viewID model.ViewID, params map[string]interface{}) controller.View {
	var v controller.View

	switch viewID {
	case model.TaskDetailViewID:
		detailParams := model.DecodeTaskDetailParams(params)
		v = taskdetail.NewTaskDetailView(f.taskStore, detailParams.TaskID, detailParams.ReadOnly, f.imageManager, f.mermaidOpts)

	case model.TaskEditViewID:
		editParams := model.DecodeTaskEditParams(params)
		v = taskdetail.NewTaskEditView(f.taskStore, editParams.TaskID, f.imageManager)
		if tev, ok := v.(*taskdetail.TaskEditView); ok {
			if editParams.Draft != nil {
				tev.SetFallbackTask(editParams.Draft)
			}
			if editParams.DescOnly {
				tev.SetDescOnly(true)
			}
			if editParams.TagsOnly {
				tev.SetTagsOnly(true)
			}
		}

	default:
		// Check if it's a plugin view
		if model.IsPluginViewID(viewID) {
			pluginName := model.GetPluginName(viewID)
			pluginConfig := f.pluginConfigs[pluginName]
			pluginDef := f.pluginDefs[pluginName]
			pluginControllerInterface := f.pluginControllers[pluginName]

			if pluginDef == nil {
				slog.Error("plugin not found", "plugin", pluginName)
				break
			}
			switch pluginDef.GetKind() {
			case plugin.KindBoard, plugin.KindList:
				tikiPlugin, ok := pluginDef.(*plugin.TikiPlugin)
				if !ok {
					slog.Error("board/list plugin is not a TikiPlugin", "plugin", pluginName)
					break
				}
				if pluginConfig == nil || pluginControllerInterface == nil {
					slog.Error("missing plugin config or controller", "plugin", pluginName)
					break
				}
				tikiCtrl, ok := pluginControllerInterface.(controller.TikiViewProvider)
				if !ok {
					slog.Error("plugin controller does not implement TikiViewProvider", "plugin", pluginName)
					break
				}
				v = NewPluginView(
					f.taskStore,
					pluginConfig,
					tikiPlugin,
					tikiCtrl.GetFilteredTasksForLane,
					tikiCtrl.EnsureFirstNonEmptyLaneSelection,
					tikiCtrl.GetActionRegistry(),
					tikiCtrl.ShowNavigation(),
				)
			case plugin.KindWiki, plugin.KindDetail:
				dokiPlugin, ok := pluginDef.(*plugin.DokiPlugin)
				if !ok {
					slog.Error("wiki/detail plugin is not a DokiPlugin", "plugin", pluginName)
					break
				}
				v = NewDokiView(dokiPlugin, f.imageManager, f.mermaidOpts, f.globalActions)
			default:
				slog.Error("unknown plugin kind", "plugin", pluginName, "kind", pluginDef.GetKind())
			}
		} else {
			slog.Error("unknown view ID", "viewID", viewID)
		}
	}

	return v
}
