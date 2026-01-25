package controller

import (
	"log/slog"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// PluginControllerInterface defines the common interface for all plugin controllers
type PluginControllerInterface interface {
	GetActionRegistry() *ActionRegistry
	GetPluginName() string
	HandleAction(ActionID) bool
	HandleSearch(string)
}

// InputRouter dispatches input events to appropriate controllers
// InputRouter is a dispatcher. It doesn't know what to do with actions—it only knows where to send them

// - Receive a raw key event
// - Determine which controller should handle it (based on current view)
// - Forward the event to that controller
// - Return whether the event was consumed

type InputRouter struct {
	navController     *NavigationController
	taskController    *TaskController
	taskEditCoord     *TaskEditCoordinator
	pluginControllers map[string]PluginControllerInterface // keyed by plugin name
	globalActions     *ActionRegistry
	taskStore         store.Store
}

// NewInputRouter creates an input router
func NewInputRouter(
	navController *NavigationController,
	taskController *TaskController,
	pluginControllers map[string]PluginControllerInterface,
	taskStore store.Store,
) *InputRouter {
	return &InputRouter{
		navController:     navController,
		taskController:    taskController,
		taskEditCoord:     NewTaskEditCoordinator(navController, taskController),
		pluginControllers: pluginControllers,
		globalActions:     DefaultGlobalActions(),
		taskStore:         taskStore,
	}
}

// HandleInput processes a key event for the current view and routes it to the appropriate handler.
// It processes events through multiple handlers in order:
// 1. Search input (if search is active)
// 2. Fullscreen escape (Esc key in fullscreen views)
// 3. Inline editors (title/description editing)
// 4. Task edit field focus (field navigation)
// 5. Global actions (Esc, Refresh)
// 6. View-specific actions (based on current view)
// Returns true if the event was handled, false otherwise.
func (ir *InputRouter) HandleInput(event *tcell.EventKey, currentView *ViewEntry) bool {
	slog.Debug("input received", "name", event.Name(), "key", int(event.Key()), "rune", string(event.Rune()), "modifiers", int(event.Modifiers()))

	if currentView == nil {
		return false
	}

	activeView := ir.navController.GetActiveView()

	isTaskEditView := currentView.ViewID == model.TaskEditViewID

	// ensure task edit view is prepared even when title/description inputs have focus
	if isTaskEditView {
		ir.taskEditCoord.Prepare(activeView, model.DecodeTaskEditParams(currentView.Params))
	}

	if stop, handled := ir.maybeHandleSearchInput(activeView, event); stop {
		return handled
	}
	if stop, handled := ir.maybeHandleFullscreenEscape(activeView, event); stop {
		return handled
	}
	if stop, handled := ir.maybeHandleInlineEditors(activeView, isTaskEditView, event); stop {
		return handled
	}
	if stop, handled := ir.maybeHandleTaskEditFieldFocus(activeView, isTaskEditView, event); stop {
		return handled
	}

	// check global actions first
	if action := ir.globalActions.Match(event); action != nil {
		return ir.handleGlobalAction(action.ID)
	}

	// route to view-specific controller
	switch currentView.ViewID {
	case model.TaskDetailViewID:
		return ir.handleTaskInput(event, currentView.Params)
	case model.TaskEditViewID:
		return ir.handleTaskEditInput(event, currentView.Params)
	default:
		// Check if it's a plugin view
		if model.IsPluginViewID(currentView.ViewID) {
			return ir.handlePluginInput(event, currentView.ViewID)
		}
		return false
	}
}

// maybeHandleSearchInput handles search box focus/visibility semantics.
// stop=true means input routing should stop and return handled.
func (ir *InputRouter) maybeHandleSearchInput(activeView View, event *tcell.EventKey) (stop bool, handled bool) {
	searchableView, ok := activeView.(SearchableView)
	if !ok {
		return false, false
	}
	if searchableView.IsSearchBoxFocused() {
		// Search box has focus and handles input through tview.
		return true, false
	}
	// Search is visible but grid has focus - handle Esc to close search.
	if searchableView.IsSearchVisible() && event.Key() == tcell.KeyEscape {
		searchableView.HideSearch()
		return true, true
	}
	return false, false
}

// maybeHandleFullscreenEscape exits fullscreen before bubbling Esc to global handler.
func (ir *InputRouter) maybeHandleFullscreenEscape(activeView View, event *tcell.EventKey) (stop bool, handled bool) {
	fullscreenView, ok := activeView.(FullscreenView)
	if !ok {
		return false, false
	}
	if fullscreenView.IsFullscreen() && event.Key() == tcell.KeyEscape {
		fullscreenView.ExitFullscreen()
		return true, true
	}
	return false, false
}

// maybeHandleInlineEditors handles focused title/description editors (and their cancel semantics).
func (ir *InputRouter) maybeHandleInlineEditors(activeView View, isTaskEditView bool, event *tcell.EventKey) (stop bool, handled bool) {
	if titleEditableView, ok := activeView.(TitleEditableView); ok {
		if titleEditableView.IsTitleInputFocused() {
			if isTaskEditView {
				return true, ir.taskEditCoord.HandleKey(activeView, event)
			}
			// Title input has focus and handles input through tview.
			return true, false
		}
		// Title is being edited but input doesn't have focus - handle Esc to cancel.
		if titleEditableView.IsTitleEditing() && !isTaskEditView && event.Key() == tcell.KeyEscape {
			titleEditableView.HideTitleEditor()
			return true, true
		}
	}

	if descEditableView, ok := activeView.(DescriptionEditableView); ok {
		if descEditableView.IsDescriptionTextAreaFocused() {
			if isTaskEditView {
				return true, ir.taskEditCoord.HandleKey(activeView, event)
			}
			// Description text area has focus and handles input through tview.
			return true, false
		}
		// Description is being edited but text area doesn't have focus - handle Esc to cancel.
		if descEditableView.IsDescriptionEditing() && !isTaskEditView && event.Key() == tcell.KeyEscape {
			descEditableView.HideDescriptionEditor()
			return true, true
		}
	}

	return false, false
}

// maybeHandleTaskEditFieldFocus routes keys to task edit coordinator when an edit field has focus.
func (ir *InputRouter) maybeHandleTaskEditFieldFocus(activeView View, isTaskEditView bool, event *tcell.EventKey) (stop bool, handled bool) {
	fieldFocusableView, ok := activeView.(FieldFocusableView)
	if !ok || !isTaskEditView {
		return false, false
	}
	if fieldFocusableView.IsEditFieldFocused() {
		return true, ir.taskEditCoord.HandleKey(activeView, event)
	}
	return false, false
}

// handlePluginInput routes input to the appropriate plugin controller
func (ir *InputRouter) handlePluginInput(event *tcell.EventKey, viewID model.ViewID) bool {
	pluginName := model.GetPluginName(viewID)
	controller, ok := ir.pluginControllers[pluginName]
	if !ok {
		slog.Warn("plugin controller not found", "plugin", pluginName)
		return false
	}

	registry := controller.GetActionRegistry()
	if action := registry.Match(event); action != nil {
		// Handle search action specially - show search box
		if action.ID == ActionSearch {
			return ir.handleSearchAction(controller)
		}
		// Handle plugin activation keys - switch to different plugin
		if targetPluginName := GetPluginNameFromAction(action.ID); targetPluginName != "" {
			targetViewID := model.MakePluginViewID(targetPluginName)
			if viewID != targetViewID {
				ir.navController.ReplaceView(targetViewID, nil)
				return true
			}
			return true // already on this plugin, consume the event
		}
		return controller.HandleAction(action.ID)
	}
	return false
}

// handleGlobalAction processes actions available in all views
func (ir *InputRouter) handleGlobalAction(actionID ActionID) bool {
	switch actionID {
	case ActionBack:
		if v := ir.navController.GetActiveView(); v != nil && v.GetViewID() == model.TaskEditViewID {
			// Cancel edit session (discards changes) and close.
			// This keeps the ActionBack behavior consistent across input paths.
			return ir.taskEditCoord.CancelAndClose()
		}
		return ir.navController.HandleBack()
	case ActionQuit:
		ir.navController.HandleQuit()
		return true
	case ActionRefresh:
		_ = ir.taskStore.Reload()
		return true
	default:
		return false
	}
}

// handleSearchAction is a generic handler for ActionSearch across all searchable views
func (ir *InputRouter) handleSearchAction(controller interface{ HandleSearch(string) }) bool {
	activeView := ir.navController.GetActiveView()
	searchableView, ok := activeView.(SearchableView)
	if !ok {
		return false
	}

	// Set up focus callback
	app := ir.navController.GetApp()
	searchableView.SetFocusSetter(func(p tview.Primitive) {
		app.SetFocus(p)
	})

	// Wire up search submit handler to controller
	searchableView.SetSearchSubmitHandler(controller.HandleSearch)

	// Show search box and focus it
	searchBox := searchableView.ShowSearch()
	if searchBox != nil {
		app.SetFocus(searchBox)
	}

	return true
}

// handleTaskInput routes input to the task controller
func (ir *InputRouter) handleTaskInput(event *tcell.EventKey, params map[string]interface{}) bool {
	// set current task from params
	taskID := model.DecodeTaskDetailParams(params).TaskID
	if taskID != "" {
		ir.taskController.SetCurrentTask(taskID)
	}

	registry := ir.taskController.GetActionRegistry()
	if action := registry.Match(event); action != nil {
		switch action.ID {
		case ActionEditTitle:
			taskID := ir.taskController.GetCurrentTaskID()
			if taskID == "" {
				return false
			}

			ir.navController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
				TaskID: taskID,
				Focus:  model.EditFieldTitle,
			}))
			return true
		case ActionFullscreen:
			activeView := ir.navController.GetActiveView()
			if fullscreenView, ok := activeView.(FullscreenView); ok {
				if fullscreenView.IsFullscreen() {
					fullscreenView.ExitFullscreen()
				} else {
					fullscreenView.EnterFullscreen()
				}
				return true
			}
			return false
		default:
			return ir.taskController.HandleAction(action.ID)
		}
	}
	return false
}

// handleTaskEditInput routes input while in the task edit view
func (ir *InputRouter) handleTaskEditInput(event *tcell.EventKey, params map[string]interface{}) bool {
	activeView := ir.navController.GetActiveView()
	ir.taskEditCoord.Prepare(activeView, model.DecodeTaskEditParams(params))

	// Handle arrow keys for cycling field values (before checking registry)
	key := event.Key()
	if key == tcell.KeyUp {
		if ir.taskEditCoord.CycleFieldValueUp(activeView) {
			return true
		}
	}
	if key == tcell.KeyDown {
		if ir.taskEditCoord.CycleFieldValueDown(activeView) {
			return true
		}
	}

	registry := ir.taskController.GetEditActionRegistry()
	if action := registry.Match(event); action != nil {
		switch action.ID {
		case ActionSaveTask:
			return ir.taskEditCoord.CommitAndClose(activeView)
		case ActionNextField:
			return ir.taskEditCoord.FocusNextField(activeView)
		case ActionPrevField:
			return ir.taskEditCoord.FocusPrevField(activeView)
		default:
			return false
		}
	}
	return false
}
