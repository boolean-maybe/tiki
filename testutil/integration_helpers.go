package testutil

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/tikistore"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/view"
	"github.com/boolean-maybe/tiki/view/header"
	"github.com/boolean-maybe/tiki/view/palette"
	"github.com/boolean-maybe/tiki/view/statusline"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TestApp wraps the full MVC stack for integration testing with SimulationScreen
type TestApp struct {
	App               *tview.Application
	Screen            tcell.SimulationScreen
	RootLayout        *view.RootLayout
	TaskStore         store.Store
	NavController     *controller.NavigationController
	InputRouter       *controller.InputRouter
	ViewFactory       *view.ViewFactory
	TaskDir           string
	t                 *testing.T
	PluginConfigs     map[string]*model.PluginConfig
	PluginControllers map[string]controller.PluginControllerInterface
	PluginDefs        []plugin.Plugin
	MutationGate      *service.TaskMutationGate
	Schema            ruki.Schema
	taskController    *controller.TaskController
	statuslineConfig  *model.StatuslineConfig
	headerConfig      *model.HeaderConfig
	viewContext       *model.ViewContext
	layoutModel       *model.LayoutModel
	paletteConfig     *model.ActionPaletteConfig
	quickSelectConfig *model.QuickSelectConfig
	actionPalette     *palette.ActionPalette
	pages             *tview.Pages
}

// NewTestApp bootstraps the full MVC stack for integration testing.
// Mirrors the initialization pattern from main.go.
func NewTestApp(t *testing.T) *TestApp {
	// 0. Isolate config paths: use a temp XDG_CONFIG_HOME so tests don't read the real user config.
	// This installs the default workflow.yaml into the temp config dir, mirroring the production
	// bootstrap sequence (Phase 2.5: InstallDefaultWorkflow).
	tmpConfigHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpConfigHome) // t.Setenv handles restore on cleanup
	config.ResetPathManager()
	if err := config.InstallDefaultWorkflow(); err != nil {
		t.Fatalf("failed to install default workflow for test: %v", err)
	}
	if err := config.LoadWorkflowRegistries(); err != nil {
		t.Fatalf("failed to load workflow registries for test: %v", err)
	}
	t.Cleanup(func() {
		config.ClearStatusRegistry()
		config.ResetPathManager()
	})

	// 0.5. Create ruki schema (needed by plugin parser and controllers)
	schema := rukiRuntime.NewSchema()

	// 1. Create temp dir for task files (auto-cleanup via t.TempDir())
	taskDir := t.TempDir()

	// 2. Initialize Model Layer
	taskStore, err := tikistore.NewTikiStore(taskDir)
	if err != nil {
		t.Fatalf("failed to create task store: %v", err)
	}
	headerConfig := model.NewHeaderConfig()
	layoutModel := model.NewLayoutModel()

	// 3. Create SimulationScreen
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("failed to init simulation screen: %v", err)
	}
	screen.SetSize(80, 40)
	screen.Clear() // Clear screen after resize

	// 4. Create tview.Application with SimulationScreen
	app := tview.NewApplication()
	app.SetScreen(screen)

	// 5. Initialize Controller Layer
	gate := service.BuildGate()
	gate.SetStore(taskStore)

	statuslineConfig := model.NewStatuslineConfig()
	navController := controller.NewNavigationController(app)
	taskController := controller.NewTaskController(taskStore, gate, navController, statuslineConfig)
	// Empty plugin controllers map for tests (no plugins configured by default)
	pluginControllers := make(map[string]controller.PluginControllerInterface)
	inputRouter := controller.NewInputRouter(
		navController,
		taskController,
		pluginControllers,
		taskStore,
		gate,
		statuslineConfig,
		schema,
	)

	// 6. Initialize View Layer
	viewFactory := view.NewViewFactory(taskStore)

	// 7. Create header widget, statusline, and RootLayout
	viewContext := model.NewViewContext()
	headerWidget := header.NewHeaderWidget(headerConfig, viewContext)
	statuslineWidget := statusline.NewStatuslineWidget(statuslineConfig)
	rootLayout := view.NewRootLayout(view.RootLayoutOpts{
		Header:           headerWidget,
		HeaderConfig:     headerConfig,
		ViewContext:      viewContext,
		LayoutModel:      layoutModel,
		ViewFactory:      viewFactory,
		TaskStore:        taskStore,
		App:              app,
		StatuslineConfig: statuslineConfig,
		StatuslineWidget: statuslineWidget,
	})

	rootLayout.SetOnViewActivated(func(v controller.View) {
		if focusSettable, ok := v.(controller.FocusSettable); ok {
			focusSettable.SetFocusSetter(func(p tview.Primitive) {
				app.SetFocus(p)
			})
		}
	})

	currentView := rootLayout.GetContentView()
	if currentView != nil {
		if focusSettable, ok := currentView.(controller.FocusSettable); ok {
			focusSettable.SetFocusSetter(func(p tview.Primitive) {
				app.SetFocus(p)
			})
		}
	}

	// 7.5 Action palette
	paletteConfig := model.NewActionPaletteConfig()
	inputRouter.SetHeaderConfig(headerConfig)
	inputRouter.SetPaletteConfig(paletteConfig)
	actionPalette := palette.NewActionPalette(viewContext, paletteConfig, inputRouter, navController)
	actionPalette.SetChangedFunc()

	// 7.6 QuickSelect
	quickSelectConfig := model.NewQuickSelectConfig()
	inputRouter.SetQuickSelectConfig(quickSelectConfig)
	quickSelect := palette.NewQuickSelect(quickSelectConfig)
	quickSelect.SetChangedFunc()
	inputRouter.SetQuickSelectView(quickSelect)

	// Build Pages root
	pages := tview.NewPages()
	pages.AddPage("base", rootLayout.GetPrimitive(), true, true)
	paletteBox := tview.NewFlex()
	paletteBox.AddItem(tview.NewBox(), 0, 1, false)
	paletteBox.AddItem(actionPalette.GetPrimitive(), palette.PaletteMinWidth, 0, true)
	pages.AddPage("palette", paletteBox, true, false)
	quickSelectBox := tview.NewFlex()
	quickSelectBox.AddItem(tview.NewBox(), 0, 1, false)
	quickSelectBox.AddItem(quickSelect.GetPrimitive(), palette.PaletteMinWidth, 0, true)
	pages.AddPage("quickselect", quickSelectBox, true, false)

	var previousFocus tview.Primitive
	paletteConfig.AddListener(func() {
		if paletteConfig.IsVisible() {
			previousFocus = app.GetFocus()
			actionPalette.OnShow()
			pages.ShowPage("palette")
			app.SetFocus(actionPalette.GetFilterInput())
		} else {
			pages.HidePage("palette")
			if previousFocus != nil {
				app.SetFocus(previousFocus)
			} else if cv := rootLayout.GetContentView(); cv != nil {
				app.SetFocus(cv.GetPrimitive())
			}
			previousFocus = nil
		}
	})

	var qsPreviousFocus tview.Primitive
	quickSelectConfig.AddListener(func() {
		if quickSelectConfig.IsVisible() {
			qsPreviousFocus = app.GetFocus()
			pages.ShowPage("quickselect")
			app.SetFocus(quickSelect.GetFilterInput())
		} else {
			pages.HidePage("quickselect")
			if qsPreviousFocus != nil {
				app.SetFocus(qsPreviousFocus)
			} else if cv := rootLayout.GetContentView(); cv != nil {
				app.SetFocus(cv.GetPrimitive())
			}
			qsPreviousFocus = nil
		}
	})

	// 8. Wire up callbacks
	navController.SetOnViewChanged(func(viewID model.ViewID, params map[string]interface{}) {
		layoutModel.SetContent(viewID, params)
	})
	navController.SetActiveViewGetter(rootLayout.GetContentView)

	// 9. Set up global input capture (matches production)
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if paletteConfig.IsVisible() {
			return event
		}
		if quickSelectConfig.IsVisible() {
			return event
		}
		statuslineConfig.DismissAutoHide()
		if inputRouter.HandleInput(event, navController.CurrentView()) {
			return nil
		}
		return event
	})

	// 10. Set root (Pages)
	app.SetRoot(pages, true).EnableMouse(false)

	// Note: Do NOT call app.Run() - we use app.Draw() + screen.Show() for synchronous testing

	ta := &TestApp{
		App:               app,
		Screen:            screen,
		RootLayout:        rootLayout,
		TaskStore:         taskStore,
		MutationGate:      gate,
		Schema:            schema,
		NavController:     navController,
		InputRouter:       inputRouter,
		TaskDir:           taskDir,
		t:                 t,
		taskController:    taskController,
		statuslineConfig:  statuslineConfig,
		headerConfig:      headerConfig,
		viewContext:       viewContext,
		layoutModel:       layoutModel,
		paletteConfig:     paletteConfig,
		quickSelectConfig: quickSelectConfig,
		actionPalette:     actionPalette,
		pages:             pages,
	}

	// 11. Auto-load plugins since all views are now plugins
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	return ta
}

// Draw forces a synchronous draw without running the app event loop
func (ta *TestApp) Draw() {
	_, width, height := ta.Screen.GetContents()
	ta.pages.SetRect(0, 0, width, height)
	ta.pages.Draw(ta.Screen)
	ta.Screen.Show()
}

// SendKey simulates a key press by directly calling the input capture handler.
// Input flows through app's InputCapture → InputRouter.HandleInput.
// If InputCapture doesn't consume the event, it's forwarded to the focused primitive.
func (ta *TestApp) SendKey(key tcell.Key, ch rune, mod tcell.ModMask) {
	event := tcell.NewEventKey(key, ch, mod)
	// Directly call the input capture handler (synchronous, no event loop needed)
	consumed := false
	if capture := ta.App.GetInputCapture(); capture != nil {
		returnedEvent := capture(event)
		consumed = (returnedEvent == nil)
	}

	// If InputCapture didn't consume the event, send it to the focused primitive
	if !consumed {
		focused := ta.App.GetFocus()
		if focused != nil {
			handler := focused.InputHandler()
			if handler != nil {
				handler(event, func(p tview.Primitive) { ta.App.SetFocus(p) })
			}
		}
	}

	// Redraw after input
	ta.Draw()
}

// GetTextAt extracts text from a screen region starting at (x, y) with given width
func (ta *TestApp) GetTextAt(x, y, width int) string {
	contents, screenWidth, _ := ta.Screen.GetContents()
	var result strings.Builder

	for i := 0; i < width; i++ {
		cellIdx := y*screenWidth + (x + i)
		if cellIdx >= len(contents) {
			break
		}
		cell := contents[cellIdx]
		if len(cell.Runes) > 0 {
			result.WriteRune(cell.Runes[0])
		} else {
			result.WriteRune(' ')
		}
	}

	return strings.TrimSpace(result.String())
}

// FindText searches for a text string anywhere on the screen.
// Returns (found, x, y) where x, y are the coordinates of the first match.
func (ta *TestApp) FindText(needle string) (bool, int, int) {
	_, width, height := ta.Screen.GetContents()

	// Search row by row
	for y := 0; y < height; y++ {
		// Extract full row text
		rowText := ta.GetTextAt(0, y, width)
		if strings.Contains(rowText, needle) {
			// Find x position within row
			x := strings.Index(rowText, needle)
			return true, x, y
		}
	}

	return false, 0, 0
}

// FindTextInRegion searches for a text string within a screen rectangle.
func (ta *TestApp) FindTextInRegion(needle string, rx, ry, rw, rh int) bool {
	for y := ry; y < ry+rh; y++ {
		rowText := ta.GetTextAt(rx, y, rw)
		if strings.Contains(rowText, needle) {
			return true
		}
	}
	return false
}

// DumpScreen prints the current screen content for debugging
func (ta *TestApp) DumpScreen() {
	_, width, height := ta.Screen.GetContents()
	ta.t.Logf("Screen size: %dx%d", width, height)
	for y := 0; y < height; y++ {
		line := ta.GetTextAt(0, y, width)
		if line != "" {
			ta.t.Logf("Row %2d: %s", y, line)
		}
	}
}

// SendKeyToFocused sends a key event directly to the focused primitive's InputHandler.
// Use this for text input into InputField, TextArea, etc.
func (ta *TestApp) SendKeyToFocused(key tcell.Key, ch rune, mod tcell.ModMask) {
	event := tcell.NewEventKey(key, ch, mod)
	focused := ta.App.GetFocus()
	if focused != nil {
		handler := focused.InputHandler()
		if handler != nil {
			handler(event, func(p tview.Primitive) { ta.App.SetFocus(p) })
		}
	}
	ta.Draw()
}

// SendText types a string of characters into the focused primitive
func (ta *TestApp) SendText(text string) {
	for _, ch := range text {
		ta.SendKey(tcell.KeyRune, ch, tcell.ModNone)
	}
}

// EditingTiki returns the current in-memory editing copy (if any).
func (ta *TestApp) EditingTiki() *tikipkg.Tiki {
	return ta.taskController.GetEditingTiki()
}

// DraftTiki returns the current draft tiki (if any).
func (ta *TestApp) DraftTiki() *tikipkg.Tiki {
	return ta.taskController.GetDraftTiki()
}

// Cleanup tears down the test app and releases resources
func (ta *TestApp) Cleanup() {
	ta.actionPalette.Cleanup()
	ta.RootLayout.Cleanup()
	ta.Screen.Fini()
}

// LoadPlugins loads plugins from workflow.yaml files and wires them into the test app.
// This enables testing of plugin-related functionality.
func (ta *TestApp) LoadPlugins() error {
	// Load embedded plugins and the top-level global actions list.
	plugins, globalActions, err := plugin.LoadPluginsAndGlobals(ta.Schema)
	if err != nil {
		return err
	}

	// Create configs and controllers for each plugin
	pluginConfigs := make(map[string]*model.PluginConfig)
	pluginControllers := make(map[string]controller.PluginControllerInterface)

	for _, p := range plugins {
		pc := model.NewPluginConfig(p.GetName())
		pluginConfigs[p.GetName()] = pc

		// Create appropriate controller based on plugin type
		if tp, ok := p.(*plugin.TikiPlugin); ok {
			columns := make([]int, len(tp.Lanes))
			widths := make([]int, len(tp.Lanes))
			for i, lane := range tp.Lanes {
				columns[i] = lane.Columns
				widths[i] = lane.Width
			}
			pc.SetLaneLayout(columns, widths)
			pluginControllers[p.GetName()] = controller.NewPluginController(
				ta.TaskStore, ta.MutationGate, pc, tp, ta.NavController, ta.statuslineConfig, ta.Schema,
			)
		} else if dp, ok := p.(*plugin.DokiPlugin); ok {
			pluginControllers[p.GetName()] = controller.NewDokiController(
				dp, ta.NavController, ta.statuslineConfig, globalActions,
				ta.TaskStore, ta.MutationGate, ta.Schema,
			)
		} else if detailPlugin, ok := p.(*plugin.DetailPlugin); ok {
			// Phase 1: kind: detail uses its own controller. Without this
			// branch the InputRouter cannot find a controller for Detail
			// views, blocking deps/plugin-navigation tests that traverse
			// through a Detail step.
			pluginControllers[p.GetName()] = controller.NewDetailController(
				detailPlugin, ta.NavController, ta.statuslineConfig,
				ta.TaskStore, ta.MutationGate, ta.Schema,
			)
		}
	}

	// Update TestApp fields
	ta.PluginConfigs = pluginConfigs
	ta.PluginControllers = pluginControllers
	ta.PluginDefs = plugins

	// Initialize plugin action registry (must happen after plugins are loaded)
	pluginInfos := make([]controller.PluginInfo, 0, len(plugins))
	for _, p := range plugins {
		pk, pr, pm := p.GetActivationKey()
		pluginInfos = append(pluginInfos, controller.PluginInfo{
			Name:     p.GetName(),
			Key:      pk,
			Rune:     pr,
			Modifier: pm,
			Require:  p.GetRequire(),
		})
	}
	controller.InitPluginActions(pluginInfos)

	// Recreate InputRouter with plugin controllers
	ta.InputRouter = controller.NewInputRouter(
		ta.NavController,
		ta.taskController,
		pluginControllers,
		ta.TaskStore,
		ta.MutationGate,
		ta.statuslineConfig,
		ta.Schema,
	)
	ta.InputRouter.SetHeaderConfig(ta.headerConfig)
	ta.InputRouter.SetPaletteConfig(ta.paletteConfig)
	ta.InputRouter.SetQuickSelectConfig(ta.quickSelectConfig)

	// Rebuild QuickSelect wiring with fresh config so listeners point to the new widget
	ta.quickSelectConfig = model.NewQuickSelectConfig()
	ta.InputRouter.SetQuickSelectConfig(ta.quickSelectConfig)
	quickSelect := palette.NewQuickSelect(ta.quickSelectConfig)
	quickSelect.SetChangedFunc()
	ta.InputRouter.SetQuickSelectView(quickSelect)

	// Update global input capture (matches production pipeline)
	ta.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if ta.paletteConfig.IsVisible() {
			return event
		}
		if ta.quickSelectConfig.IsVisible() {
			return event
		}
		ta.statuslineConfig.DismissAutoHide()
		if ta.InputRouter.HandleInput(event, ta.NavController.CurrentView()) {
			return nil
		}
		return event
	})

	// Update ViewFactory with plugins
	// Convert plugin slice to map for ViewFactory
	pluginDefs := make(map[string]plugin.Plugin)
	for _, p := range plugins {
		pluginDefs[p.GetName()] = p
	}

	viewFactory := view.NewViewFactory(ta.TaskStore)
	viewFactory.SetPlugins(pluginConfigs, pluginDefs, pluginControllers, globalActions)
	// Mirror production wiring: fresh-per-navigation controllers for kind: detail
	// (and wiki) so two pushed views don't share selectedTaskID.
	viewFactory.SetDetailControllerFactory(func(def *plugin.DetailPlugin, selectedTaskID string) *controller.DetailController {
		dc := controller.NewDetailController(def, ta.NavController, ta.statuslineConfig, ta.TaskStore, ta.MutationGate, ta.Schema)
		dc.SetSelectedTaskID(selectedTaskID)
		return dc
	})
	ta.ViewFactory = viewFactory

	// Wire dynamic plugin registration so openDepsEditor can register deps views at runtime.
	// Mirrors bootstrap/init.go:133-135.
	ta.InputRouter.SetPluginRegistrar(func(name string, cfg *model.PluginConfig, def plugin.Plugin, ctrl controller.PluginControllerInterface) {
		viewFactory.RegisterPlugin(name, cfg, def, ctrl)
	})

	// Recreate RootLayout with new view factory
	headerWidget := header.NewHeaderWidget(ta.headerConfig, ta.viewContext)
	ta.RootLayout.Cleanup()
	slConfig := model.NewStatuslineConfig()
	slWidget := statusline.NewStatuslineWidget(slConfig)
	ta.RootLayout = view.NewRootLayout(view.RootLayoutOpts{
		Header:           headerWidget,
		HeaderConfig:     ta.headerConfig,
		ViewContext:      ta.viewContext,
		LayoutModel:      ta.layoutModel,
		ViewFactory:      viewFactory,
		TaskStore:        ta.TaskStore,
		App:              ta.App,
		StatuslineConfig: slConfig,
		StatuslineWidget: slWidget,
	})

	// Re-wire callbacks
	ta.NavController.SetActiveViewGetter(ta.RootLayout.GetContentView)

	// IMPORTANT: Re-wire OnViewActivated callback for focus management
	ta.RootLayout.SetOnViewActivated(func(v controller.View) {
		if focusSettable, ok := v.(controller.FocusSettable); ok {
			focusSettable.SetFocusSetter(func(p tview.Primitive) {
				ta.App.SetFocus(p)
			})
		}
	})

	// Retroactively wire focus setter for current view
	if currentView := ta.RootLayout.GetContentView(); currentView != nil {
		if focusSettable, ok := currentView.(controller.FocusSettable); ok {
			focusSettable.SetFocusSetter(func(p tview.Primitive) {
				ta.App.SetFocus(p)
			})
		}
	}

	// Update palette with new view context
	ta.actionPalette.Cleanup()
	ta.actionPalette = palette.NewActionPalette(ta.viewContext, ta.paletteConfig, ta.InputRouter, ta.NavController)
	ta.actionPalette.SetChangedFunc()

	// Rebuild Pages
	ta.pages.RemovePage("quickselect")
	ta.pages.RemovePage("palette")
	ta.pages.RemovePage("base")
	ta.pages.AddPage("base", ta.RootLayout.GetPrimitive(), true, true)
	paletteBox := tview.NewFlex()
	paletteBox.AddItem(tview.NewBox(), 0, 1, false)
	paletteBox.AddItem(ta.actionPalette.GetPrimitive(), palette.PaletteMinWidth, 0, true)
	ta.pages.AddPage("palette", paletteBox, true, false)
	quickSelectBox := tview.NewFlex()
	quickSelectBox.AddItem(tview.NewBox(), 0, 1, false)
	quickSelectBox.AddItem(quickSelect.GetPrimitive(), palette.PaletteMinWidth, 0, true)
	ta.pages.AddPage("quickselect", quickSelectBox, true, false)

	// wire QuickSelect visibility listener (fresh config = no stale listeners)
	var qsPrev tview.Primitive
	ta.quickSelectConfig.AddListener(func() {
		if ta.quickSelectConfig.IsVisible() {
			qsPrev = ta.App.GetFocus()
			ta.pages.ShowPage("quickselect")
			ta.App.SetFocus(quickSelect.GetFilterInput())
		} else {
			ta.pages.HidePage("quickselect")
			if qsPrev != nil {
				ta.App.SetFocus(qsPrev)
			} else if cv := ta.RootLayout.GetContentView(); cv != nil {
				ta.App.SetFocus(cv.GetPrimitive())
			}
			qsPrev = nil
		}
	})

	ta.App.SetRoot(ta.pages, true)

	return nil
}

// GetHeaderConfig returns the header config for testing visibility assertions.
func (ta *TestApp) GetHeaderConfig() *model.HeaderConfig {
	return ta.headerConfig
}

// GetStatuslineConfig returns the statusline config for testing message assertions.
func (ta *TestApp) GetStatuslineConfig() *model.StatuslineConfig {
	return ta.statuslineConfig
}

// GetPaletteConfig returns the palette config for testing visibility assertions.
func (ta *TestApp) GetPaletteConfig() *model.ActionPaletteConfig {
	return ta.paletteConfig
}

// GetQuickSelectConfig returns the quick-select config for testing visibility assertions.
func (ta *TestApp) GetQuickSelectConfig() *model.QuickSelectConfig {
	return ta.quickSelectConfig
}

// GetPluginConfig retrieves the PluginConfig for a given plugin name.
// Returns nil if the plugin is not loaded.
func (ta *TestApp) GetPluginConfig(pluginName string) *model.PluginConfig {
	return ta.PluginConfigs[pluginName]
}

// NavigateToTask presses Down on the current board view until the task with the given ID
// is the selected item. It opens the task detail (Enter) and returns true if found within
// maxSteps attempts; returns false if the task was not found.
func (ta *TestApp) NavigateToTask(taskID string, maxSteps int) bool {
	for i := 0; i < maxSteps; i++ {
		ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
		ta.Draw()
		if found, _, _ := ta.FindText(taskID); found {
			return true
		}
		// go back and move to next item
		ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
		ta.Draw()
		ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)
		ta.Draw()
	}
	return false
}
