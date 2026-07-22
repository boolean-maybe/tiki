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
	"github.com/boolean-maybe/tiki/view/tikidetail"
)

// ViewFactory instantiates views by ID, injecting required dependencies.
// It holds references to shared state (stores, configs) needed by views.

// ViewFactory creates views on demand
type ViewFactory struct {
	tikiStore    store.Store
	imageManager *navtview.ImageManager
	mermaidOpts  *nav.MermaidOptions
	// progressHub + redraw let detail views resolve images off the UI
	// goroutine while the statusline shows a progress bar. Set via
	// SetProgressHub after the app + hub exist in bootstrap.
	progressHub *model.ProgressHub
	redraw      func(func())
	// Plugin support
	pluginConfigs     map[string]*model.PluginConfig
	pluginDefs        map[string]plugin.Plugin
	pluginControllers map[string]controller.PluginControllerInterface
	globalActions     []plugin.PluginAction
	// wikiControllerFactory creates a fresh WikiController for each view navigation,
	// preventing the shared-singleton problem where SetSelectedTikiID on one navigation
	// would overwrite the selected tiki ID seen by a previously-pushed view of the same plugin.
	wikiControllerFactory func(pluginDef plugin.Plugin, selectedTikiID string) *controller.WikiController
	// detailControllerFactory mirrors wikiControllerFactory for kind: detail
	// views: each navigation gets its own DetailController so multiple Detail
	// views on the stack hold independent selectedTikiID values. Without this,
	// the most recent navigation overwrites the selection of every earlier
	// detail view of the same plugin.
	detailControllerFactory func(pluginDef *plugin.DetailPlugin, selectedTikiID string) *controller.DetailController
}

// NewViewFactory creates a view factory
func NewViewFactory(tikiStore store.Store) *ViewFactory {
	// Configure image resolver with the unified document root as the primary
	// search root so images referenced from nested or root-level `.md`
	// documents resolve (e.g. `.doc/projects/foo/diagram.png` or
	// `.doc/assets/logo.png`).
	searchRoots := []string{config.GetDocDir()}
	resolver := nav.NewImageResolver(searchRoots)
	resolver.SetDarkMode(!config.IsLightTheme())
	resolver.SetSVGScaleFactor(1.6)
	imgMgr := navtview.NewImageManager(resolver, 8, 16)
	imgMgr.SetMaxRows(config.GetMaxImageRows())
	imgMgr.SetSupported(util.SupportsKittyGraphics())

	return &ViewFactory{
		tikiStore:    tikiStore,
		imageManager: imgMgr,
		mermaidOpts:  &nav.MermaidOptions{MinDiagramWidth: 280},
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

// SetProgressHub wires the progress hub and a UI-goroutine redraw func into
// detail views so image resolution can run off-thread with a statusline bar.
func (f *ViewFactory) SetProgressHub(hub *model.ProgressHub, redraw func(func())) {
	f.progressHub = hub
	f.redraw = redraw
}

// SetWikiControllerFactory registers a factory function that creates a fresh
// WikiController per view navigation, capturing nav/status/gate/schema from
// the bootstrap context. Must be called before any wiki view is created.
func (f *ViewFactory) SetWikiControllerFactory(fn func(pluginDef plugin.Plugin, selectedTikiID string) *controller.WikiController) {
	f.wikiControllerFactory = fn
}

// SetDetailControllerFactory registers a factory for fresh DetailControllers
// per navigation. Same rationale as SetWikiControllerFactory but for
// kind: detail views, so two pushed Detail views can each carry their own
// selected tiki id without trampling each other.
func (f *ViewFactory) SetDetailControllerFactory(fn func(pluginDef *plugin.DetailPlugin, selectedTikiID string) *controller.DetailController) {
	f.detailControllerFactory = fn
}

// CreateView instantiates a view by ID with optional parameters.
// Plugin views are the only views the factory builds; built-in view IDs no
// longer route through here.
func (f *ViewFactory) CreateView(viewID model.ViewID, params map[string]interface{}) controller.View {
	if !model.IsPluginViewID(viewID) {
		slog.Error("unknown view ID", "viewID", viewID)
		return nil
	}

	pluginName := model.GetPluginName(viewID)
	pluginConfig := f.pluginConfigs[pluginName]
	pluginDef := f.pluginDefs[pluginName]
	pluginControllerInterface := f.pluginControllers[pluginName]

	if pluginDef == nil {
		slog.Error("plugin not found", "plugin", pluginName)
		return nil
	}

	switch pluginDef.GetKind() {
	case plugin.KindBoard, plugin.KindList:
		tikiPlugin, ok := pluginDef.(*plugin.WorkflowPlugin)
		if !ok {
			slog.Error("board/list plugin is not a WorkflowPlugin", "plugin", pluginName)
			return nil
		}
		if pluginConfig == nil || pluginControllerInterface == nil {
			slog.Error("missing plugin config or controller", "plugin", pluginName)
			return nil
		}
		tikiCtrl, ok := pluginControllerInterface.(controller.TikiViewProvider)
		if !ok {
			slog.Error("plugin controller does not implement TikiViewProvider", "plugin", pluginName)
			return nil
		}
		return NewPluginView(
			f.tikiStore,
			pluginConfig,
			tikiPlugin,
			tikiCtrl.GetFilteredTikisForLane,
			tikiCtrl.EnsureFirstNonEmptyLaneSelection,
			tikiCtrl.GetActionRegistry(),
			tikiCtrl.ShowNavigation(),
		)
	case plugin.KindWiki:
		wikiPlugin, ok := pluginDef.(*plugin.WikiPlugin)
		if !ok {
			slog.Error("wiki plugin is not a WikiPlugin", "plugin", pluginName)
			return nil
		}
		pluginParams := model.DecodePluginViewParams(params)
		effective := wikiPlugin
		if pluginParams.DocumentPath != "" {
			// per-navigation path override — copy so the registered def stays immutable
			clone := *wikiPlugin
			clone.DocumentPath = pluginParams.DocumentPath
			effective = &clone
		}
		// create a fresh WikiController per navigation so each view
		// instance on the nav stack holds its own selectedTikiID. the
		// shared map entry is updated so InputRouter always dispatches
		// through the controller that owns the currently-active view.
		if f.wikiControllerFactory != nil {
			freshCtrl := f.wikiControllerFactory(pluginDef, pluginParams.TikiID)
			f.pluginControllers[pluginName] = freshCtrl
		} else if dc, ok := pluginControllerInterface.(*controller.WikiController); ok {
			dc.SetSelectedTikiID(pluginParams.TikiID)
		}
		return NewWikiView(effective, f.imageManager, f.mermaidOpts, f.globalActions, f.tikiStore, pluginParams.TikiID)
	case plugin.KindDetail:
		detailPlugin, ok := pluginDef.(*plugin.DetailPlugin)
		if !ok {
			slog.Error("detail plugin is not a DetailPlugin", "plugin", pluginName)
			return nil
		}
		pluginParams := model.DecodePluginViewParams(params)
		// build (or refresh) the controller that owns this view's
		// selectedTikiID. each navigation gets a fresh controller —
		// matching the wiki path — so two pushed Detail views
		// of the same plugin don't overwrite each other's selection.
		// the shared map is updated to the freshest controller so the
		// InputRouter dispatches keys against the active view.
		var dc *controller.DetailController
		if f.detailControllerFactory != nil {
			dc = f.detailControllerFactory(detailPlugin, pluginParams.TikiID)
			f.pluginControllers[pluginName] = dc
		} else if existing, ok := pluginControllerInterface.(*controller.DetailController); ok {
			existing.SetSelectedTikiID(pluginParams.TikiID)
			dc = existing
		}
		registry := controller.DetailViewActions()
		if dc != nil {
			registry = dc.GetActionRegistry()
		}
		cv := tikidetail.NewConfigurableDetailView(
			f.tikiStore,
			pluginParams.TikiID,
			detailPlugin,
			registry,
			f.imageManager,
			f.mermaidOpts,
			f.progressHub,
			f.redraw,
		)
		if dc != nil {
			dc.BindEditView(cv)
			dc.ApplyDetailMode(pluginParams.Mode, pluginParams.Focus, pluginParams.Draft)
		}
		return cv
	default:
		slog.Error("unknown plugin kind", "plugin", pluginName, "kind", pluginDef.GetKind())
		return nil
	}
}
