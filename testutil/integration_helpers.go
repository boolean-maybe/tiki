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
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/view"
	"github.com/boolean-maybe/tiki/view/header"
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

	// Mirror main.go wiring: provide views a focus setter as they become active.
	rootLayout.SetOnViewActivated(func(v controller.View) {
		// generic focus settable check (covers TaskEditView and any other view with focus needs)
		if focusSettable, ok := v.(controller.FocusSettable); ok {
			focusSettable.SetFocusSetter(func(p tview.Primitive) {
				app.SetFocus(p)
			})
		}
	})

	// IMPORTANT: Retroactively wire focus setter for any view already active
	// (RootLayout may have activated a view during construction before callback was set)
	currentView := rootLayout.GetContentView()
	if currentView != nil {
		if focusSettable, ok := currentView.(controller.FocusSettable); ok {
			focusSettable.SetFocusSetter(func(p tview.Primitive) {
				app.SetFocus(p)
			})
		}
	}

	// 8. Wire up callbacks
	navController.SetOnViewChanged(func(viewID model.ViewID, params map[string]interface{}) {
		layoutModel.SetContent(viewID, params)
	})
	navController.SetActiveViewGetter(rootLayout.GetContentView)

	// 9. Set up global input capture
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		handled := inputRouter.HandleInput(event, navController.CurrentView())
		if handled {
			return nil // consume event
		}
		return event // pass through
	})

	// 10. Set root layout
	app.SetRoot(rootLayout.GetPrimitive(), true).EnableMouse(false)

	// Note: Do NOT call app.Run() - we use app.Draw() + screen.Show() for synchronous testing

	ta := &TestApp{
		App:              app,
		Screen:           screen,
		RootLayout:       rootLayout,
		TaskStore:        taskStore,
		MutationGate:     gate,
		Schema:           schema,
		NavController:    navController,
		InputRouter:      inputRouter,
		TaskDir:          taskDir,
		t:                t,
		taskController:   taskController,
		statuslineConfig: statuslineConfig,
		headerConfig:     headerConfig,
		viewContext:      viewContext,
		layoutModel:      layoutModel,
	}

	// 11. Auto-load plugins since all views are now plugins
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	return ta
}

// Draw forces a synchronous draw without running the app event loop
func (ta *TestApp) Draw() {
	// Get screen dimensions and set the root layout's rect
	_, width, height := ta.Screen.GetContents()
	ta.RootLayout.GetPrimitive().SetRect(0, 0, width, height)
	ta.RootLayout.GetPrimitive().Draw(ta.Screen)
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

// EditingTask returns the current in-memory editing copy (if any).
func (ta *TestApp) EditingTask() *taskpkg.Task {
	return ta.taskController.GetEditingTask()
}

// DraftTask returns the current draft task (if any).
func (ta *TestApp) DraftTask() *taskpkg.Task {
	return ta.taskController.GetDraftTask()
}

// Cleanup tears down the test app and releases resources
func (ta *TestApp) Cleanup() {
	ta.RootLayout.Cleanup()
	ta.Screen.Fini()
	// TaskDir cleanup handled automatically by t.TempDir()
}

// LoadPlugins loads plugins from workflow.yaml files and wires them into the test app.
// This enables testing of plugin-related functionality.
func (ta *TestApp) LoadPlugins() error {
	// Load embedded plugins
	plugins, err := plugin.LoadPlugins(ta.Schema)
	if err != nil {
		return err
	}

	// Create configs and controllers for each plugin
	pluginConfigs := make(map[string]*model.PluginConfig)
	pluginControllers := make(map[string]controller.PluginControllerInterface)

	for _, p := range plugins {
		pc := model.NewPluginConfig(p.GetName())
		pc.SetConfigIndex(p.GetConfigIndex())
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
				dp, ta.NavController, ta.statuslineConfig,
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

	// Update global input capture to handle plugin switching keys
	ta.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Check if search box has focus - if so, let it handle ALL input
		if activeView := ta.NavController.GetActiveView(); activeView != nil {
			if searchableView, ok := activeView.(controller.SearchableView); ok {
				if searchableView.IsSearchBoxFocused() {
					return event
				}
			}
		}

		currentView := ta.NavController.CurrentView()
		if currentView != nil {
			// Handle plugin switching between plugins
			if model.IsPluginViewID(currentView.ViewID) {
				if action := controller.GetPluginActions().Match(event); action != nil {
					pluginName := controller.GetPluginNameFromAction(action.ID)
					if pluginName != "" {
						targetPluginID := model.MakePluginViewID(pluginName)
						// Don't switch to the same plugin we're already viewing
						if currentView.ViewID == targetPluginID {
							return nil // no-op
						}
						// Replace current plugin with target plugin
						ta.NavController.ReplaceView(targetPluginID, nil)
						return nil
					}
				}
			}
		}

		// Let InputRouter handle the rest
		handled := ta.InputRouter.HandleInput(event, ta.NavController.CurrentView())
		if handled {
			return nil // consume event
		}
		return event // pass through
	})

	// Update ViewFactory with plugins
	// Convert plugin slice to map for ViewFactory
	pluginDefs := make(map[string]plugin.Plugin)
	for _, p := range plugins {
		pluginDefs[p.GetName()] = p
	}

	viewFactory := view.NewViewFactory(ta.TaskStore)
	viewFactory.SetPlugins(pluginConfigs, pluginDefs, pluginControllers)
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

	// Set new root
	ta.App.SetRoot(ta.RootLayout.GetPrimitive(), true)

	return nil
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
