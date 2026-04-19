package controller

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// PluginControllerInterface defines the common interface for all plugin controllers
type PluginControllerInterface interface {
	GetActionRegistry() *ActionRegistry
	GetPluginName() string
	HandleAction(ActionID) bool
	HandleSearch(string)
	ShowNavigation() bool
	GetActionInputSpec(ActionID) (prompt string, typ ruki.ValueType, hasInput bool)
	CanStartActionInput(ActionID) (prompt string, typ ruki.ValueType, ok bool)
	HandleActionInput(ActionID, string) InputSubmitResult
}

// TikiViewProvider is implemented by controllers that back a TikiPlugin view.
// The view factory uses this to create PluginView without knowing the concrete controller type.
type TikiViewProvider interface {
	GetFilteredTasksForLane(lane int) []*task.Task
	EnsureFirstNonEmptyLaneSelection() bool
	GetActionRegistry() *ActionRegistry
	ShowNavigation() bool
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
	mutationGate      *service.TaskMutationGate
	statusline        *model.StatuslineConfig
	schema            ruki.Schema
	registerPlugin    func(name string, cfg *model.PluginConfig, def plugin.Plugin, ctrl PluginControllerInterface)
	headerConfig      *model.HeaderConfig
	paletteConfig     *model.ActionPaletteConfig
}

// NewInputRouter creates an input router
func NewInputRouter(
	navController *NavigationController,
	taskController *TaskController,
	pluginControllers map[string]PluginControllerInterface,
	taskStore store.Store,
	mutationGate *service.TaskMutationGate,
	statusline *model.StatuslineConfig,
	schema ruki.Schema,
) *InputRouter {
	return &InputRouter{
		navController:     navController,
		taskController:    taskController,
		taskEditCoord:     NewTaskEditCoordinator(navController, taskController),
		pluginControllers: pluginControllers,
		globalActions:     DefaultGlobalActions(),
		taskStore:         taskStore,
		mutationGate:      mutationGate,
		statusline:        statusline,
		schema:            schema,
	}
}

// SetHeaderConfig wires the header config for fullscreen-aware header toggling.
func (ir *InputRouter) SetHeaderConfig(hc *model.HeaderConfig) {
	ir.headerConfig = hc
}

// SetPaletteConfig wires the palette config for ActionOpenPalette dispatch.
func (ir *InputRouter) SetPaletteConfig(pc *model.ActionPaletteConfig) {
	ir.paletteConfig = pc
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

	// if the input box is focused, let it handle all input (including '*' and F10)
	if activeView := ir.navController.GetActiveView(); activeView != nil {
		if iv, ok := activeView.(InputableView); ok && iv.IsInputBoxFocused() {
			return false
		}
	}

	// pre-gate: ActionOpenPalette (*) and ActionToggleHeader (F10) must fire before
	// task-edit Prepare and before search/fullscreen/editor gates, so they stay truly
	// global without triggering edit-session setup or focus churn.
	if action := ir.globalActions.Match(event); action != nil {
		if action.ID == ActionOpenPalette || action.ID == ActionToggleHeader {
			return ir.handleGlobalAction(action.ID)
		}
	}

	if currentView == nil {
		return false
	}

	activeView := ir.navController.GetActiveView()

	isTaskEditView := currentView.ViewID == model.TaskEditViewID

	// ensure task edit view is prepared even when title/description inputs have focus
	if isTaskEditView {
		ir.taskEditCoord.Prepare(activeView, model.DecodeTaskEditParams(currentView.Params))
	}

	if stop, handled := ir.maybeHandleInputBox(activeView, event); stop {
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

// maybeHandleInputBox handles input box focus/visibility semantics.
// stop=true means input routing should stop and return handled.
func (ir *InputRouter) maybeHandleInputBox(activeView View, event *tcell.EventKey) (stop bool, handled bool) {
	inputableView, ok := activeView.(InputableView)
	if !ok {
		return false, false
	}
	if inputableView.IsInputBoxFocused() {
		return true, false
	}
	// visible but not focused (passive mode): Esc dismisses via cancel path
	// so search-specific teardown fires (clearing results)
	if inputableView.IsInputBoxVisible() && event.Key() == tcell.KeyEscape {
		inputableView.CancelInputBox()
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
	// Only TaskEditView has inline editors now
	if !isTaskEditView {
		return false, false
	}

	if titleEditableView, ok := activeView.(TitleEditableView); ok {
		if titleEditableView.IsTitleInputFocused() {
			return true, ir.taskEditCoord.HandleKey(activeView, event)
		}
	}

	if descEditableView, ok := activeView.(DescriptionEditableView); ok {
		if descEditableView.IsDescriptionTextAreaFocused() {
			return true, ir.taskEditCoord.HandleKey(activeView, event)
		}
	}

	if tagsView, ok := activeView.(interface{ IsTagsTextAreaFocused() bool }); ok {
		if tagsView.IsTagsTextAreaFocused() {
			return true, ir.taskEditCoord.HandleKey(activeView, event)
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

// SetPluginRegistrar sets the callback used to register dynamically created plugins
// (e.g., the deps editor) with the view factory.
func (ir *InputRouter) SetPluginRegistrar(fn func(name string, cfg *model.PluginConfig, def plugin.Plugin, ctrl PluginControllerInterface)) {
	ir.registerPlugin = fn
}

// openDepsEditor creates (or reopens) a deps editor plugin for the given task ID.
func (ir *InputRouter) openDepsEditor(taskID string) bool {
	name := "Dependency:" + taskID
	viewID := model.MakePluginViewID(name)

	// reopen if already created
	if _, exists := ir.pluginControllers[name]; exists {
		ir.navController.PushView(viewID, nil)
		return true
	}

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{
			Name:        name,
			Description: model.DepsEditorViewDesc,
			ConfigIndex: -1,
			Type:        "tiki",
			Background:  config.GetColors().DepsEditorBackground,
		},
		TaskID: taskID,
		Lanes: []plugin.TikiLane{
			{Name: "Blocks"},
			{Name: "All"},
			{Name: "Depends"},
		},
	}

	pluginConfig := model.NewPluginConfig("Dependency")
	pluginConfig.SetLaneLayout([]int{1, 2, 1}, []int{25, 50, 25})
	if vm := config.GetPluginViewMode("Dependency"); vm != "" {
		pluginConfig.SetViewMode(vm)
	}

	ctrl := NewDepsController(ir.taskStore, ir.mutationGate, pluginConfig, pluginDef, ir.navController, ir.statusline, ir.schema)

	if ir.registerPlugin != nil {
		ir.registerPlugin(name, pluginConfig, pluginDef, ctrl)
	}
	ir.pluginControllers[name] = ctrl

	ir.navController.PushView(viewID, nil)
	return true
}

// handlePluginInput routes input to the appropriate plugin controller
func (ir *InputRouter) handlePluginInput(event *tcell.EventKey, viewID model.ViewID) bool {
	pluginName := model.GetPluginName(viewID)
	ctrl, ok := ir.pluginControllers[pluginName]
	if !ok {
		slog.Warn("plugin controller not found", "plugin", pluginName)
		return false
	}

	registry := ctrl.GetActionRegistry()
	if action := registry.Match(event); action != nil {
		if action.ID == ActionSearch {
			return ir.handleSearchInput(ctrl)
		}
		if targetPluginName := GetPluginNameFromAction(action.ID); targetPluginName != "" {
			targetViewID := model.MakePluginViewID(targetPluginName)
			if viewID != targetViewID {
				ir.navController.ReplaceView(targetViewID, nil)
				return true
			}
			return true
		}
		if _, _, hasInput := ctrl.GetActionInputSpec(action.ID); hasInput {
			return ir.startActionInput(ctrl, action.ID)
		}
		return ctrl.HandleAction(action.ID)
	}
	return false
}

// handleGlobalAction processes actions available in all views
func (ir *InputRouter) handleGlobalAction(actionID ActionID) bool {
	switch actionID {
	case ActionBack:
		if v := ir.navController.GetActiveView(); v != nil && v.GetViewID() == model.TaskEditViewID {
			return ir.taskEditCoord.CancelAndClose()
		}
		return ir.navController.HandleBack()
	case ActionQuit:
		ir.navController.HandleQuit()
		return true
	case ActionRefresh:
		_ = ir.taskStore.Reload()
		return true
	case ActionOpenPalette:
		if ir.paletteConfig != nil {
			ir.paletteConfig.SetVisible(true)
		}
		return true
	case ActionToggleHeader:
		ir.toggleHeader()
		return true
	default:
		return false
	}
}

// toggleHeader toggles the stored user preference and recomputes effective visibility
// against the live active view so fullscreen/header-hidden views stay force-hidden.
func (ir *InputRouter) toggleHeader() {
	if ir.headerConfig == nil {
		return
	}
	newPref := !ir.headerConfig.GetUserPreference()
	ir.headerConfig.SetUserPreference(newPref)

	visible := newPref
	if v := ir.navController.GetActiveView(); v != nil {
		if hv, ok := v.(interface{ RequiresHeaderHidden() bool }); ok && hv.RequiresHeaderHidden() {
			visible = false
		}
		if fv, ok := v.(FullscreenView); ok && fv.IsFullscreen() {
			visible = false
		}
	}
	ir.headerConfig.SetVisible(visible)
}

// HandleAction dispatches a palette-selected action by ID against the given view entry.
// This is the controller-side fallback for palette execution — the palette tries
// view.HandlePaletteAction first, then falls back here.
func (ir *InputRouter) HandleAction(id ActionID, currentView *ViewEntry) bool {
	if currentView == nil {
		return false
	}

	// block palette-dispatched actions while an input box is in editing mode
	if activeView := ir.navController.GetActiveView(); activeView != nil {
		if iv, ok := activeView.(InputableView); ok && iv.IsInputBoxFocused() {
			return false
		}
	}

	// global actions
	if ir.globalActions.ContainsID(id) {
		return ir.handleGlobalAction(id)
	}

	activeView := ir.navController.GetActiveView()

	switch currentView.ViewID {
	case model.TaskDetailViewID:
		taskID := model.DecodeTaskDetailParams(currentView.Params).TaskID
		if taskID != "" {
			ir.taskController.SetCurrentTask(taskID)
		}
		return ir.dispatchTaskAction(id, currentView.Params)

	case model.TaskEditViewID:
		if activeView != nil {
			ir.taskEditCoord.Prepare(activeView, model.DecodeTaskEditParams(currentView.Params))
		}
		return ir.dispatchTaskEditAction(id, activeView)

	default:
		if model.IsPluginViewID(currentView.ViewID) {
			return ir.dispatchPluginAction(id, currentView.ViewID)
		}
		return false
	}
}

// dispatchTaskAction handles palette-dispatched task detail actions by ActionID.
func (ir *InputRouter) dispatchTaskAction(id ActionID, _ map[string]interface{}) bool {
	switch id {
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
	case ActionEditDesc:
		taskID := ir.taskController.GetCurrentTaskID()
		if taskID == "" {
			return false
		}
		ir.navController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
			TaskID:   taskID,
			Focus:    model.EditFieldDescription,
			DescOnly: true,
		}))
		return true
	case ActionEditTags:
		taskID := ir.taskController.GetCurrentTaskID()
		if taskID == "" {
			return false
		}
		ir.navController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
			TaskID:   taskID,
			TagsOnly: true,
		}))
		return true
	case ActionEditDeps:
		taskID := ir.taskController.GetCurrentTaskID()
		if taskID == "" {
			return false
		}
		return ir.openDepsEditor(taskID)
	case ActionChat:
		agent := config.GetAIAgent()
		if agent == "" {
			return false
		}
		taskID := ir.taskController.GetCurrentTaskID()
		if taskID == "" {
			return false
		}
		filename := strings.ToLower(taskID) + ".md"
		taskFilePath := filepath.Join(config.GetTaskDir(), filename)
		name, args := resolveAgentCommand(agent, taskFilePath)
		ir.navController.SuspendAndRun(name, args...)
		_ = ir.taskStore.ReloadTask(taskID)
		return true
	case ActionCloneTask:
		return ir.taskController.HandleAction(id)
	default:
		return ir.taskController.HandleAction(id)
	}
}

// dispatchTaskEditAction handles palette-dispatched task edit actions by ActionID.
func (ir *InputRouter) dispatchTaskEditAction(id ActionID, activeView View) bool {
	switch id {
	case ActionSaveTask:
		if activeView != nil {
			return ir.taskEditCoord.CommitAndClose(activeView)
		}
		return false
	default:
		return false
	}
}

// dispatchPluginAction handles palette-dispatched plugin actions by ActionID.
func (ir *InputRouter) dispatchPluginAction(id ActionID, viewID model.ViewID) bool {
	if targetPluginName := GetPluginNameFromAction(id); targetPluginName != "" {
		targetViewID := model.MakePluginViewID(targetPluginName)
		if viewID != targetViewID {
			ir.navController.ReplaceView(targetViewID, nil)
			return true
		}
		return true
	}

	pluginName := model.GetPluginName(viewID)
	ctrl, ok := ir.pluginControllers[pluginName]
	if !ok {
		return false
	}

	if id == ActionSearch {
		return ir.handleSearchInput(ctrl)
	}

	if _, _, hasInput := ctrl.GetActionInputSpec(id); hasInput {
		return ir.startActionInput(ctrl, id)
	}

	return ctrl.HandleAction(id)
}

// startActionInput opens the input box for an action that requires user input.
func (ir *InputRouter) startActionInput(ctrl PluginControllerInterface, actionID ActionID) bool {
	_, _, ok := ctrl.CanStartActionInput(actionID)
	if !ok {
		return false
	}

	activeView := ir.navController.GetActiveView()
	inputableView, ok := activeView.(InputableView)
	if !ok {
		return false
	}

	app := ir.navController.GetApp()
	inputableView.SetFocusSetter(func(p tview.Primitive) {
		app.SetFocus(p)
	})

	inputableView.SetInputSubmitHandler(func(text string) InputSubmitResult {
		return ctrl.HandleActionInput(actionID, text)
	})

	inputableView.SetInputCancelHandler(func() {
		inputableView.CancelInputBox()
	})

	inputBox := inputableView.ShowInputBox("> ", "")
	if inputBox != nil {
		app.SetFocus(inputBox)
	}

	return true
}

// handleSearchInput opens the input box in search mode for the active view.
// Blocked when search is already passive — user must Esc first.
func (ir *InputRouter) handleSearchInput(ctrl interface{ HandleSearch(string) }) bool {
	activeView := ir.navController.GetActiveView()
	inputableView, ok := activeView.(InputableView)
	if !ok {
		return false
	}

	if inputableView.IsSearchPassive() {
		return true
	}

	app := ir.navController.GetApp()
	inputableView.SetFocusSetter(func(p tview.Primitive) {
		app.SetFocus(p)
	})

	inputableView.SetInputSubmitHandler(func(text string) InputSubmitResult {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return InputKeepEditing
		}
		ctrl.HandleSearch(trimmed)
		return InputShowPassive
	})

	inputBox := inputableView.ShowSearchBox()
	if inputBox != nil {
		app.SetFocus(inputBox)
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

	registry := ir.navController.GetActiveView().GetActionRegistry()
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
		case ActionEditDesc:
			taskID := ir.taskController.GetCurrentTaskID()
			if taskID == "" {
				return false
			}
			ir.navController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
				TaskID:   taskID,
				Focus:    model.EditFieldDescription,
				DescOnly: true,
			}))
			return true
		case ActionEditTags:
			taskID := ir.taskController.GetCurrentTaskID()
			if taskID == "" {
				return false
			}
			ir.navController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
				TaskID:   taskID,
				TagsOnly: true,
			}))
			return true
		case ActionEditDeps:
			taskID := ir.taskController.GetCurrentTaskID()
			if taskID == "" {
				return false
			}
			return ir.openDepsEditor(taskID)
		case ActionChat:
			agent := config.GetAIAgent()
			if agent == "" {
				return false
			}
			taskID := ir.taskController.GetCurrentTaskID()
			if taskID == "" {
				return false
			}
			filename := strings.ToLower(taskID) + ".md"
			taskFilePath := filepath.Join(config.GetTaskDir(), filename)
			name, args := resolveAgentCommand(agent, taskFilePath)
			ir.navController.SuspendAndRun(name, args...)
			_ = ir.taskStore.ReloadTask(taskID)
			return true
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
