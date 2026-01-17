package controller

import (
	"log/slog"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/util"

	"github.com/rivo/tview"
)

// NavigationController handles view transitions: push, pop, and managing the navigation stack.
// It does NOT create views - that's handled by RootLayout which observes the LayoutModel.

// NavigationController manages the navigation stack and delegates view creation to RootLayout
type NavigationController struct {
	app              *tview.Application
	navState         *viewStack
	activeViewGetter func() View                                              // returns the currently displayed view from RootLayout
	onViewChanged    func(viewID model.ViewID, params map[string]interface{}) // callback when view changes (for layoutModel sync)
}

// NewNavigationController creates a navigation controller
func NewNavigationController(app *tview.Application) *NavigationController {
	return &NavigationController{
		app:      app,
		navState: newViewStack(),
	}
}

// SetActiveViewGetter sets the function to retrieve the currently displayed view
func (nc *NavigationController) SetActiveViewGetter(getter func() View) {
	nc.activeViewGetter = getter
}

// SetOnViewChanged registers a callback that runs when the view changes (for layoutModel sync)
func (nc *NavigationController) SetOnViewChanged(callback func(viewID model.ViewID, params map[string]interface{})) {
	nc.onViewChanged = callback
}

// PushView navigates to a new view, adding it to the stack
func (nc *NavigationController) PushView(viewID model.ViewID, params map[string]interface{}) {
	// push onto navigation stack
	nc.navState.push(viewID, params)

	// notify layoutModel of view change - RootLayout will create the view
	if nc.onViewChanged != nil {
		nc.onViewChanged(viewID, params)
	}
}

// ReplaceView replaces the current view with a new one (maintains stack depth)
func (nc *NavigationController) ReplaceView(viewID model.ViewID, params map[string]interface{}) bool {
	// Replace in navigation stack
	if !nc.navState.replaceTopView(viewID, params) {
		return false
	}

	// notify layoutModel of view change - RootLayout will create the view
	if nc.onViewChanged != nil {
		nc.onViewChanged(viewID, params)
	}

	return true
}

// PopView returns to the previous view
func (nc *NavigationController) PopView() bool {
	if !nc.navState.canGoBack() {
		return false
	}

	// pop current view
	nc.navState.pop()

	// get previous view entry
	prevEntry := nc.navState.currentView()
	if prevEntry == nil {
		return false
	}

	// notify layoutModel of view change - RootLayout will create the view
	if nc.onViewChanged != nil {
		nc.onViewChanged(prevEntry.ViewID, prevEntry.Params)
	}

	return true
}

// GetActiveView returns the currently displayed view (from RootLayout)
func (nc *NavigationController) GetActiveView() View {
	if nc.activeViewGetter != nil {
		return nc.activeViewGetter()
	}
	return nil
}

// CurrentView returns the current view entry from the navigation stack
func (nc *NavigationController) CurrentView() *ViewEntry {
	return nc.navState.currentView()
}

// CurrentViewID returns the view ID of the current view
func (nc *NavigationController) CurrentViewID() model.ViewID {
	return nc.navState.currentViewID()
}

// Depth returns the current stack depth (for testing)
func (nc *NavigationController) Depth() int {
	return nc.navState.depth()
}

// GetApp returns the tview application
func (nc *NavigationController) GetApp() *tview.Application {
	return nc.app
}

// HandleBack processes the back/escape action
func (nc *NavigationController) HandleBack() bool {
	return nc.PopView()
}

// HandleQuit stops the application
func (nc *NavigationController) HandleQuit() {
	nc.app.Stop()
}

// SuspendAndEdit suspends the tview application and opens the specified file in the user's default editor.
// After the editor exits, the application resumes and redraws.
func (nc *NavigationController) SuspendAndEdit(filePath string) {
	nc.app.Suspend(func() {
		if err := util.OpenInEditor(filePath); err != nil {
			slog.Error("failed to open editor", "file", filePath, "error", err)
		}
	})
}
