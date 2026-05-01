package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/internal/app"
	"github.com/boolean-maybe/tiki/internal/background"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/tikistore"
	"github.com/boolean-maybe/tiki/util/sysinfo"
	"github.com/boolean-maybe/tiki/view"
	"github.com/boolean-maybe/tiki/view/header"
	"github.com/boolean-maybe/tiki/view/palette"
	"github.com/boolean-maybe/tiki/view/statusline"
)

// Result contains all initialized application components.
type Result struct {
	Cfg      *config.Config
	LogLevel slog.Level
	// SystemInfo contains client environment information collected during bootstrap.
	// Fields include: OS, Architecture, TermType, DetectedTheme, ColorSupport, ColorCount.
	// Collected early using terminfo lookup (no screen initialization needed).
	SystemInfo        *sysinfo.SystemInfo
	MutationGate      *service.TaskMutationGate
	TikiStore         *tikistore.TikiStore
	TaskStore         store.Store
	HeaderConfig      *model.HeaderConfig
	LayoutModel       *model.LayoutModel
	Plugins           []plugin.Plugin
	PluginConfigs     map[string]*model.PluginConfig
	PluginDefs        map[string]plugin.Plugin
	App               *tview.Application
	Controllers       *Controllers
	InputRouter       *controller.InputRouter
	ViewFactory       *view.ViewFactory
	HeaderWidget      *header.HeaderWidget
	StatuslineConfig  *model.StatuslineConfig
	StatuslineWidget  *statusline.StatuslineWidget
	RootLayout        *view.RootLayout
	PaletteConfig     *model.ActionPaletteConfig
	QuickSelectConfig *model.QuickSelectConfig
	ActionPalette     *palette.ActionPalette
	ViewContext       *model.ViewContext
	AppRoot           tview.Primitive // Pages root for app.SetRoot
	Context           context.Context
	CancelFunc        context.CancelFunc
	TikiSkillContent  string
	DokiSkillContent  string
	WorkflowPath      string
	WorkflowScope     config.Scope
}

// Bootstrap orchestrates the complete application initialization sequence.
// It takes the embedded AI skill content and returns all initialized components.
func Bootstrap(tikiSkillContent, dokiSkillContent string) (*Result, error) {
	// Phase 0: Configuration and logging — loaded first so store.git and store.name
	// are available before any git checks or side effects.
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	logLevel := InitLogging(cfg)

	// Phase 0.5: Validate store backend before any side effects
	if name := config.GetStoreName(); name != "tiki" {
		return nil, fmt.Errorf("unknown store backend: %q (supported: tiki)", name)
	}

	// Phase 1: Pre-flight git check (skipped when git is disabled)
	if config.GetStoreGit() {
		if err := EnsureGitRepo(); err != nil {
			return nil, err
		}
	}

	// Phase 2: Project initialization (creates dirs and seeds sample files)
	var gitAdd func(...string) error
	if config.GetStoreGit() {
		gitAdd = tikistore.NewGitAdder("")
	}
	proceed, err := EnsureProjectInitialized(tikiSkillContent, dokiSkillContent, gitAdd)
	if err != nil {
		return nil, err
	}
	if !proceed {
		return nil, nil // User chose not to proceed
	}

	// Phase 2.5: Install default workflow to user config dir (first-run or upgrade)
	// Runs on every launch outside BootstrapSystem so that upgrades from older versions
	// get workflow.yaml installed even though their project is already initialized.
	if err := config.InstallDefaultWorkflow(); err != nil {
		slog.Warn("failed to install default workflow", "error", err)
	}

	// Phase 2.7: Load workflow registries (statuses, types, custom fields)
	if err := config.LoadWorkflowRegistries(); err != nil {
		return nil, fmt.Errorf("load workflow registries: %w", err)
	}

	// Phase 2.8: Resolve workflow file location for statusline and edit action
	workflowPath, workflowScope := config.FindWorkflowFileWithScope()

	// Phase 3.5: System information collection and gradient support initialization
	// Collect early (before app creation) using terminfo lookup for future visual adjustments
	systemInfo := InitColorAndGradientSupport(cfg)

	// Phase 3.7: Mutation gate (before store, so validators can register early)
	gate := service.BuildGate()

	// Phase 4: Store initialization
	tikiStore, taskStore, err := InitStores()
	if err != nil {
		return nil, err
	}
	gate.SetStore(taskStore)

	// Phase 5: Model initialization
	headerConfig, layoutModel := InitHeaderAndLayoutModels()
	statuslineConfig := InitStatuslineModel(tikiStore, workflowScope)

	// Phase 5.5: Ruki schema (needed by plugin parser and trigger system)
	schema := rukiRuntime.NewSchema()

	// Phase 6: Plugin system
	plugins, globalActions, err := LoadPlugins(schema)
	if err != nil {
		return nil, err
	}
	InitPluginActionRegistry(plugins)
	viewContext := model.NewViewContext()
	pluginConfigs, pluginDefs := BuildPluginConfigsAndDefs(plugins)

	// Phase 6.5: Trigger system — share the same identity projection as the
	// runtime/plugin executors so email-only identity configurations reach
	// `user()` calls inside triggers
	userFunc, err := store.CurrentUserDisplayFunc(taskStore)
	if err != nil {
		return nil, fmt.Errorf("resolve current user: %w", err)
	}
	triggerEngine, triggerCount, err := service.LoadAndRegisterTriggers(gate, schema, userFunc)
	if err != nil {
		return nil, fmt.Errorf("load triggers: %w", err)
	}
	if triggerCount > 0 {
		slog.Info("triggers loaded", "count", triggerCount)
	}

	// Phase 7: Application and controllers
	application := app.NewApp()
	app.SetupSignalHandler(application)

	controllers := BuildControllers(
		application,
		taskStore,
		gate,
		plugins,
		globalActions,
		pluginConfigs,
		statuslineConfig,
		schema,
	)

	// Phase 8: Input routing
	inputRouter := controller.NewInputRouter(
		controllers.Nav,
		controllers.Task,
		controllers.Plugins,
		taskStore,
		gate,
		statuslineConfig,
		schema,
	)

	// Phase 9: View factory and layout
	viewFactory := view.NewViewFactory(taskStore)
	viewFactory.SetPlugins(pluginConfigs, pluginDefs, controllers.Plugins, globalActions)

	// Wire dynamic plugin registration (deps editor creates plugins at runtime)
	inputRouter.SetPluginRegistrar(func(name string, cfg *model.PluginConfig, def plugin.Plugin, ctrl controller.PluginControllerInterface) {
		viewFactory.RegisterPlugin(name, cfg, def, ctrl)
	})

	headerWidget := header.NewHeaderWidget(headerConfig, viewContext)
	statuslineWidget := statusline.NewStatuslineWidget(statuslineConfig)
	rootLayout := view.NewRootLayout(view.RootLayoutOpts{
		Header:           headerWidget,
		HeaderConfig:     headerConfig,
		ViewContext:      viewContext,
		LayoutModel:      layoutModel,
		ViewFactory:      viewFactory,
		TaskStore:        taskStore,
		App:              application,
		StatuslineWidget: statuslineWidget,
		StatuslineConfig: statuslineConfig,
	})

	// Phase 10: View wiring
	wireOnViewActivated(rootLayout, application)

	// Phase 11: Background tasks
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel stored in Result.CancelFunc, called by app shutdown
	background.StartBurndownHistoryBuilder(ctx, tikiStore, headerConfig, application)
	triggerEngine.StartScheduler(ctx)

	// Phase 11.5: Action palette
	paletteConfig := model.NewActionPaletteConfig()
	inputRouter.SetHeaderConfig(headerConfig)
	inputRouter.SetPaletteConfig(paletteConfig)
	inputRouter.SetWorkflowPath(workflowPath)

	actionPalette := palette.NewActionPalette(viewContext, paletteConfig, inputRouter, controllers.Nav)
	actionPalette.SetChangedFunc()

	// Phase 11.6: QuickSelect
	quickSelectConfig := model.NewQuickSelectConfig()
	inputRouter.SetQuickSelectConfig(quickSelectConfig)

	quickSelect := palette.NewQuickSelect(quickSelectConfig)
	quickSelect.SetChangedFunc()
	inputRouter.SetQuickSelectView(quickSelect)

	// Build Pages root: base = rootLayout, overlay = palette + quickselect
	pages := tview.NewPages()
	pages.AddPage("base", rootLayout.GetPrimitive(), true, true)
	paletteOverlay := buildPaletteOverlay(actionPalette)
	pages.AddPage("palette", paletteOverlay, true, false)
	quickSelectOverlay := buildQuickSelectOverlay(quickSelect)
	pages.AddPage("quickselect", quickSelectOverlay, true, false)

	// Wire palette visibility to Pages show/hide and focus management
	var previousFocus tview.Primitive
	paletteConfig.AddListener(func() {
		if paletteConfig.IsVisible() {
			previousFocus = application.GetFocus()
			actionPalette.OnShow()
			pages.ShowPage("palette")
			application.SetFocus(actionPalette.GetFilterInput())
		} else {
			pages.HidePage("palette")
			restoreFocusAfterPalette(application, previousFocus, rootLayout)
			previousFocus = nil
		}
	})

	// Wire QuickSelect visibility
	var qsPreviousFocus tview.Primitive
	quickSelectConfig.AddListener(func() {
		if quickSelectConfig.IsVisible() {
			qsPreviousFocus = application.GetFocus()
			pages.ShowPage("quickselect")
			application.SetFocus(quickSelect.GetFilterInput())
		} else {
			pages.HidePage("quickselect")
			if qsPreviousFocus != nil {
				application.SetFocus(qsPreviousFocus)
			} else if cv := rootLayout.GetContentView(); cv != nil {
				application.SetFocus(cv.GetPrimitive())
			}
			qsPreviousFocus = nil
		}
	})

	// Phase 12: Navigation and input wiring
	wireNavigation(controllers.Nav, layoutModel, rootLayout)
	app.InstallGlobalInputCapture(application, paletteConfig, quickSelectConfig, statuslineConfig, inputRouter, controllers.Nav)

	// Phase 13: Initial view — use the first plugin marked default: true,
	// or fall back to the first plugin in the list.
	controllers.Nav.PushView(model.MakePluginViewID(plugin.DefaultPlugin(plugins).GetName()), nil)

	return &Result{
		Cfg:               cfg,
		LogLevel:          logLevel,
		SystemInfo:        systemInfo,
		MutationGate:      gate,
		TikiStore:         tikiStore,
		TaskStore:         taskStore,
		HeaderConfig:      headerConfig,
		LayoutModel:       layoutModel,
		Plugins:           plugins,
		PluginConfigs:     pluginConfigs,
		PluginDefs:        pluginDefs,
		App:               application,
		Controllers:       controllers,
		InputRouter:       inputRouter,
		ViewFactory:       viewFactory,
		HeaderWidget:      headerWidget,
		StatuslineConfig:  statuslineConfig,
		StatuslineWidget:  statuslineWidget,
		RootLayout:        rootLayout,
		PaletteConfig:     paletteConfig,
		QuickSelectConfig: quickSelectConfig,
		ActionPalette:     actionPalette,
		ViewContext:       viewContext,
		AppRoot:           pages,
		Context:           ctx,
		CancelFunc:        cancel,
		TikiSkillContent:  tikiSkillContent,
		DokiSkillContent:  dokiSkillContent,
		WorkflowPath:      workflowPath,
		WorkflowScope:     workflowScope,
	}, nil
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

// paletteOverlayFlex is a Flex that recomputes the palette width on every draw
// to maintain 1/3 terminal width with a minimum floor.
type paletteOverlayFlex struct {
	*tview.Flex
	palette         tview.Primitive
	spacer          *tview.Flex
	lastPaletteSize int
}

func buildPaletteOverlay(ap *palette.ActionPalette) *paletteOverlayFlex {
	overlay := &paletteOverlayFlex{
		Flex:    tview.NewFlex(),
		palette: ap.GetPrimitive(),
	}
	overlay.spacer = tview.NewFlex()
	overlay.Flex.AddItem(overlay.spacer, 0, 1, false)
	overlay.Flex.AddItem(overlay.palette, palette.PaletteMinWidth, 0, true)
	overlay.lastPaletteSize = palette.PaletteMinWidth
	return overlay
}

func (o *paletteOverlayFlex) Draw(screen tcell.Screen) {
	_, _, w, _ := o.GetRect()
	pw := w / 3
	if pw < palette.PaletteMinWidth {
		pw = palette.PaletteMinWidth
	}
	if pw != o.lastPaletteSize {
		o.Flex.Clear()
		o.Flex.AddItem(o.spacer, 0, 1, false)
		o.Flex.AddItem(o.palette, pw, 0, true)
		o.lastPaletteSize = pw
	}
	o.Flex.Draw(screen)
}

// quickSelectOverlayFlex mirrors paletteOverlayFlex for the QuickSelect picker.
type quickSelectOverlayFlex struct {
	*tview.Flex
	picker   tview.Primitive
	spacer   *tview.Flex
	lastSize int
}

func buildQuickSelectOverlay(qs *palette.QuickSelect) *quickSelectOverlayFlex {
	overlay := &quickSelectOverlayFlex{
		Flex:   tview.NewFlex(),
		picker: qs.GetPrimitive(),
	}
	overlay.spacer = tview.NewFlex()
	overlay.Flex.AddItem(overlay.spacer, 0, 1, false)
	overlay.Flex.AddItem(overlay.picker, palette.PaletteMinWidth, 0, true)
	overlay.lastSize = palette.PaletteMinWidth
	return overlay
}

func (o *quickSelectOverlayFlex) Draw(screen tcell.Screen) {
	_, _, w, _ := o.GetRect()
	pw := w / 3
	if pw < palette.PaletteMinWidth {
		pw = palette.PaletteMinWidth
	}
	if pw != o.lastSize {
		o.Flex.Clear()
		o.Flex.AddItem(o.spacer, 0, 1, false)
		o.Flex.AddItem(o.picker, pw, 0, true)
		o.lastSize = pw
	}
	o.Flex.Draw(screen)
}

// restoreFocusAfterPalette restores focus to the previously focused primitive,
// falling back to FocusRestorer on the active view, then to the content view root.
func restoreFocusAfterPalette(application *tview.Application, previousFocus tview.Primitive, rootLayout *view.RootLayout) {
	if previousFocus != nil {
		application.SetFocus(previousFocus)
		return
	}
	if contentView := rootLayout.GetContentView(); contentView != nil {
		if restorer, ok := contentView.(controller.FocusRestorer); ok {
			if restorer.RestoreFocus() {
				return
			}
		}
		application.SetFocus(contentView.GetPrimitive())
	}
}

// InitColorAndGradientSupport collects system information, auto-corrects TERM if needed,
// and initializes gradient support flags based on terminal color capabilities.
// Returns the collected SystemInfo for use in bootstrap result.
func InitColorAndGradientSupport(cfg *config.Config) *sysinfo.SystemInfo {
	_ = cfg
	// Collect initial system information using terminfo lookup
	systemInfo := sysinfo.NewSystemInfo()
	slog.Debug("collected system information",
		"os", systemInfo.OS,
		"arch", systemInfo.Architecture,
		"term", systemInfo.TermType,
		"theme", systemInfo.DetectedTheme,
		"color_support", systemInfo.ColorSupport,
		"color_count", systemInfo.ColorCount)

	// Auto-correct TERM if insufficient color support detected
	// This commonly happens in Docker containers or minimal environments
	if systemInfo.ColorCount < 256 && systemInfo.TermType != "" {
		slog.Info("limited color support detected, upgrading TERM for better experience",
			"original_term", systemInfo.TermType,
			"original_colors", systemInfo.ColorCount,
			"new_term", "xterm-256color")
		if err := sysinfo.SetTermEnv("xterm-256color"); err != nil {
			slog.Warn("failed to set TERM environment variable", "error", err)
		}
		// Re-collect system info to get updated color capabilities
		systemInfo = sysinfo.NewSystemInfo()
		slog.Debug("updated system information after TERM correction",
			"color_support", systemInfo.ColorSupport,
			"color_count", systemInfo.ColorCount)
	}

	// Initialize gradient support based on terminal color capabilities
	threshold := config.GetGradientThreshold()
	if systemInfo.ColorCount < threshold {
		config.UseGradients = false
		config.UseWideGradients = false
		slog.Debug("gradients disabled",
			"colorCount", systemInfo.ColorCount,
			"threshold", threshold)
	} else {
		config.UseGradients = true
		// Wide gradients (caption rows) require truecolor to avoid visible banding
		// 256-color terminals show noticeable banding on screen-wide gradients
		config.UseWideGradients = systemInfo.ColorCount >= 16777216
		slog.Debug("gradients enabled",
			"colorCount", systemInfo.ColorCount,
			"threshold", threshold,
			"wideGradients", config.UseWideGradients)
	}

	// set tview global styles so all primitives inherit the theme colors.
	// PrimaryTextColor must be set for light theme — tview defaults to white,
	// which is invisible on light backgrounds.
	colors := config.GetColors()
	tview.Styles.PrimitiveBackgroundColor = colors.ContentBackgroundColor.TCell()
	if config.IsLightTheme() {
		tview.Styles.PrimaryTextColor = colors.ContentTextColor.TCell()
	}

	return systemInfo
}
