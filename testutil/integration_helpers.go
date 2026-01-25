package testutil

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/tikistore"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/view"
	"github.com/boolean-maybe/tiki/view/header"

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
	TaskDir           string
	t                 *testing.T
	PluginConfigs     map[string]*model.PluginConfig
	PluginControllers map[string]controller.PluginControllerInterface
	PluginDefs        []plugin.Plugin
	taskController    *controller.TaskController
	headerConfig      *model.HeaderConfig
	layoutModel       *model.LayoutModel
}

// NewTestApp bootstraps the full MVC stack for integration testing.
// Mirrors the initialization pattern from main.go.
func NewTestApp(t *testing.T) *TestApp {
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
	navController := controller.NewNavigationController(app)
	taskController := controller.NewTaskController(taskStore, navController)
	// Empty plugin controllers map for tests (no plugins configured by default)
	pluginControllers := make(map[string]controller.PluginControllerInterface)
	inputRouter := controller.NewInputRouter(
		navController,
		taskController,
		pluginControllers,
		taskStore,
	)

	// 6. Initialize View Layer
	viewFactory := view.NewViewFactory(taskStore)

	// 7. Create header widget and RootLayout
	headerWidget := header.NewHeaderWidget(headerConfig)
	rootLayout := view.NewRootLayout(headerWidget, headerConfig, layoutModel, viewFactory, taskStore, app)

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
		App:            app,
		Screen:         screen,
		RootLayout:     rootLayout,
		TaskStore:      taskStore,
		NavController:  navController,
		InputRouter:    inputRouter,
		TaskDir:        taskDir,
		t:              t,
		taskController: taskController,
		headerConfig:   headerConfig,
		layoutModel:    layoutModel,
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

// GetCell extracts the rune and style at a specific screen position
func (ta *TestApp) GetCell(x, y int) (rune, tcell.Style) {
	contents, width, _ := ta.Screen.GetContents()
	idx := y*width + x
	if idx >= len(contents) {
		return ' ', tcell.StyleDefault
	}
	cell := contents[idx]
	if len(cell.Runes) > 0 {
		return cell.Runes[0], cell.Style
	}
	return ' ', cell.Style
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

// LoadPlugins loads embedded plugins and wires them into the test app.
// This enables testing of plugin-related functionality.
func (ta *TestApp) LoadPlugins() error {
	// Load embedded plugins
	plugins, err := plugin.LoadPlugins()
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
			columns := make([]int, len(tp.Panes))
			for i, pane := range tp.Panes {
				columns[i] = pane.Columns
			}
			pc.SetPaneLayout(columns)
			pluginControllers[p.GetName()] = controller.NewPluginController(
				ta.TaskStore, pc, tp, ta.NavController,
			)
		} else if dp, ok := p.(*plugin.DokiPlugin); ok {
			pluginControllers[p.GetName()] = controller.NewDokiController(
				dp, ta.NavController,
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

	// Recreate RootLayout with new view factory
	headerWidget := header.NewHeaderWidget(ta.headerConfig)
	ta.RootLayout.Cleanup()
	ta.RootLayout = view.NewRootLayout(headerWidget, ta.headerConfig, ta.layoutModel, viewFactory, ta.TaskStore, ta.App)

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
