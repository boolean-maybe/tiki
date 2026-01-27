package bootstrap

import (
	"context"
	"log/slog"

	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/internal/app"
	"github.com/boolean-maybe/tiki/internal/background"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/tikistore"
	"github.com/boolean-maybe/tiki/util/sysinfo"
	"github.com/boolean-maybe/tiki/view"
	"github.com/boolean-maybe/tiki/view/header"
)

// BootstrapResult contains all initialized application components.
type BootstrapResult struct {
	Cfg      *config.Config
	LogLevel slog.Level
	// SystemInfo contains client environment information collected during bootstrap.
	// Fields include: OS, Architecture, TermType, DetectedTheme, ColorSupport, ColorCount.
	// Collected early using terminfo lookup (no screen initialization needed).
	SystemInfo       *sysinfo.SystemInfo
	TikiStore        *tikistore.TikiStore
	TaskStore        store.Store
	HeaderConfig     *model.HeaderConfig
	LayoutModel      *model.LayoutModel
	Plugins          []plugin.Plugin
	PluginConfigs    map[string]*model.PluginConfig
	PluginDefs       map[string]plugin.Plugin
	App              *tview.Application
	Controllers      *Controllers
	InputRouter      *controller.InputRouter
	ViewFactory      *view.ViewFactory
	HeaderWidget     *header.HeaderWidget
	RootLayout       *view.RootLayout
	Context          context.Context
	CancelFunc       context.CancelFunc
	TikiSkillContent string
	DokiSkillContent string
}

// Bootstrap orchestrates the complete application initialization sequence.
// It takes the embedded AI skill content and returns all initialized components.
func Bootstrap(tikiSkillContent, dokiSkillContent string) (*BootstrapResult, error) {
	// Phase 1: Pre-flight checks
	if err := EnsureGitRepo(); err != nil {
		return nil, err
	}

	// Phase 2: Configuration and logging
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	logLevel := InitLogging(cfg)

	// Phase 2.5: System information collection
	// Collect early (before app creation) using terminfo lookup for future visual adjustments
	systemInfo := sysinfo.NewSystemInfo()
	slog.Debug("collected system information",
		"os", systemInfo.OS,
		"arch", systemInfo.Architecture,
		"term", systemInfo.TermType,
		"theme", systemInfo.DetectedTheme,
		"color_support", systemInfo.ColorSupport,
		"color_count", systemInfo.ColorCount)

	// Phase 3: Project initialization
	proceed, err := EnsureProjectInitialized(tikiSkillContent, dokiSkillContent)
	if err != nil {
		return nil, err
	}
	if !proceed {
		return nil, nil // User chose not to proceed
	}

	// Phase 4: Store initialization
	tikiStore, taskStore, err := InitStores()
	if err != nil {
		return nil, err
	}

	// Phase 5: Model initialization
	headerConfig, layoutModel := InitHeaderAndLayoutModels()
	InitHeaderBaseStats(headerConfig, tikiStore)

	// Phase 6: Plugin system
	plugins := LoadPlugins()
	InitPluginActionRegistry(plugins)
	syncHeaderPluginActions(headerConfig)
	pluginConfigs, pluginDefs := BuildPluginConfigsAndDefs(plugins)

	// Phase 7: Application and controllers
	application := app.NewApp()
	app.SetupSignalHandler(application)

	controllers := BuildControllers(
		application,
		taskStore,
		plugins,
		pluginConfigs,
	)

	// Phase 8: Input routing
	inputRouter := controller.NewInputRouter(
		controllers.Nav,
		controllers.Task,
		controllers.Plugins,
		taskStore,
	)

	// Phase 9: View factory and layout
	viewFactory := view.NewViewFactory(taskStore)
	viewFactory.SetPlugins(pluginConfigs, pluginDefs, controllers.Plugins)

	headerWidget := header.NewHeaderWidget(headerConfig)
	rootLayout := view.NewRootLayout(headerWidget, headerConfig, layoutModel, viewFactory, taskStore, application)

	// Phase 10: View wiring
	wireOnViewActivated(rootLayout, application)

	// Phase 11: Background tasks
	ctx, cancel := context.WithCancel(context.Background())
	background.StartBurndownHistoryBuilder(ctx, tikiStore, headerConfig, application)

	// Phase 12: Navigation and input wiring
	wireNavigation(controllers.Nav, layoutModel, rootLayout)
	app.InstallGlobalInputCapture(application, headerConfig, inputRouter, controllers.Nav)

	// Phase 13: Initial view (Kanban plugin is the default)
	controllers.Nav.PushView(model.MakePluginViewID("Kanban"), nil)

	return &BootstrapResult{
		Cfg:              cfg,
		LogLevel:         logLevel,
		SystemInfo:       systemInfo,
		TikiStore:        tikiStore,
		TaskStore:        taskStore,
		HeaderConfig:     headerConfig,
		LayoutModel:      layoutModel,
		Plugins:          plugins,
		PluginConfigs:    pluginConfigs,
		PluginDefs:       pluginDefs,
		App:              application,
		Controllers:      controllers,
		InputRouter:      inputRouter,
		ViewFactory:      viewFactory,
		HeaderWidget:     headerWidget,
		RootLayout:       rootLayout,
		Context:          ctx,
		CancelFunc:       cancel,
		TikiSkillContent: tikiSkillContent,
		DokiSkillContent: dokiSkillContent,
	}, nil
}

// syncHeaderPluginActions syncs plugin action shortcuts from the controller registry
// into the header model.
func syncHeaderPluginActions(headerConfig *model.HeaderConfig) {
	pluginActionsList := convertPluginActions(controller.GetPluginActions())
	headerConfig.SetPluginActions(pluginActionsList)
}

// convertPluginActions converts controller.ActionRegistry to []model.HeaderAction
// for HeaderConfig.
func convertPluginActions(registry *controller.ActionRegistry) []model.HeaderAction {
	if registry == nil {
		return nil
	}

	actions := registry.GetHeaderActions()
	result := make([]model.HeaderAction, len(actions))
	for i, a := range actions {
		result[i] = model.HeaderAction{
			ID:           string(a.ID),
			Key:          a.Key,
			Rune:         a.Rune,
			Label:        a.Label,
			Modifier:     a.Modifier,
			ShowInHeader: a.ShowInHeader,
		}
	}
	return result
}

// wireOnViewActivated wires focus setters into views as they become active.
func wireOnViewActivated(rootLayout *view.RootLayout, app *tview.Application) {
	rootLayout.SetOnViewActivated(func(v controller.View) {
		// generic focus settable check (covers TaskEditView and any other view with focus needs)
		if focusSettable, ok := v.(controller.FocusSettable); ok {
			focusSettable.SetFocusSetter(func(p tview.Primitive) {
				app.SetFocus(p)
			})
		}
	})
}

// wireNavigation wires navigation controller callbacks to keep LayoutModel
// and RootLayout in sync.
func wireNavigation(navController *controller.NavigationController, layoutModel *model.LayoutModel, rootLayout *view.RootLayout) {
	navController.SetOnViewChanged(func(viewID model.ViewID, params map[string]interface{}) {
		layoutModel.SetContent(viewID, params)
	})
	navController.SetActiveViewGetter(rootLayout.GetContentView)
}
