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
	// dokiControllerFactory creates a fresh DokiController for each view navigation,
	// preventing the shared-singleton problem where SetSelectedTaskID on one navigation
	// would overwrite the selected task ID seen by a previously-pushed view of the same plugin.
	dokiControllerFactory func(pluginDef plugin.Plugin, selectedTaskID string) *controller.DokiController
	// detailControllerFactory mirrors dokiControllerFactory for kind: detail
	// views: each navigation gets its own DetailController so multiple Detail
	// views on the stack hold independent selectedTaskID values. Without this,
	// the most recent navigation overwrites the selection of every earlier
	// detail view of the same plugin.
	detailControllerFactory func(pluginDef *plugin.DetailPlugin, selectedTaskID string) *controller.DetailController
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

// SetDokiControllerFactory registers a factory function that creates a fresh
// DokiController per view navigation, capturing nav/status/gate/schema from
// the bootstrap context. Must be called before any doki view is created.
func (f *ViewFactory) SetDokiControllerFactory(fn func(pluginDef plugin.Plugin, selectedTaskID string) *controller.DokiController) {
	f.dokiControllerFactory = fn
}

// SetDetailControllerFactory registers a factory for fresh DetailControllers
// per navigation. Same rationale as SetDokiControllerFactory but for
// kind: detail views, so two pushed Detail views can each carry their own
// selected task id without trampling each other.
func (f *ViewFactory) SetDetailControllerFactory(fn func(pluginDef *plugin.DetailPlugin, selectedTaskID string) *controller.DetailController) {
	f.detailControllerFactory = fn
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
				tev.SetFallbackTiki(editParams.Draft)
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
			case plugin.KindWiki:
				dokiPlugin, ok := pluginDef.(*plugin.DokiPlugin)
				if !ok {
					slog.Error("wiki plugin is not a DokiPlugin", "plugin", pluginName)
					break
				}
				pluginParams := model.DecodePluginViewParams(params)
				// Create a fresh DokiController per navigation so each view
				// instance on the nav stack holds its own selectedTaskID. The
				// shared map entry is updated so InputRouter always dispatches
				// through the controller that owns the currently-active view.
				if f.dokiControllerFactory != nil {
					freshCtrl := f.dokiControllerFactory(pluginDef, pluginParams.TaskID)
					f.pluginControllers[pluginName] = freshCtrl
				} else if dc, ok := pluginControllerInterface.(*controller.DokiController); ok {
					dc.SetSelectedTaskID(pluginParams.TaskID)
				}
				v = NewDokiView(dokiPlugin, f.imageManager, f.mermaidOpts, f.globalActions, f.taskStore, pluginParams.TaskID)
			case plugin.KindDetail:
				detailPlugin, ok := pluginDef.(*plugin.DetailPlugin)
				if !ok {
					slog.Error("detail plugin is not a DetailPlugin", "plugin", pluginName)
					break
				}
				pluginParams := model.DecodePluginViewParams(params)
				// Build (or refresh) the controller that owns this view's
				// selectedTaskID. Each navigation gets a fresh controller —
				// matching the wiki/doki path — so two pushed Detail views
				// of the same plugin don't overwrite each other's selection.
				// The shared map is updated to the freshest controller so the
				// InputRouter dispatches keys against the active view.
				var dc *controller.DetailController
				if f.detailControllerFactory != nil {
					dc = f.detailControllerFactory(detailPlugin, pluginParams.TaskID)
					f.pluginControllers[pluginName] = dc
				} else if existing, ok := pluginControllerInterface.(*controller.DetailController); ok {
					existing.SetSelectedTaskID(pluginParams.TaskID)
					dc = existing
				}
				registry := controller.DetailViewActions()
				if dc != nil {
					registry = dc.GetActionRegistry()
				}
				v = taskdetail.NewConfigurableDetailView(
					f.taskStore,
					pluginParams.TaskID,
					detailPlugin.Name,
					detailPlugin.Metadata,
					registry,
					f.imageManager,
					f.mermaidOpts,
				)
			default:
				slog.Error("unknown plugin kind", "plugin", pluginName, "kind", pluginDef.GetKind())
			}
		} else {
			slog.Error("unknown view ID", "viewID", viewID)
		}
	}

	return v
}
