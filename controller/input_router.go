package controller

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

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
	GetActionChooseSpec(ActionID) (label string, hasChoose bool)
	CanStartActionChoose(ActionID) (label string, candidates []*tikipkg.Tiki, ok bool)
	HandleActionChoose(ActionID, string) bool
}

// QuickSelectView abstracts the QuickSelect view to avoid import cycles.
type QuickSelectView interface {
	OnShow(tikis []*tikipkg.Tiki)
	GetFilterInput() tview.Primitive
}

// TikiViewProvider is implemented by controllers that back a TikiPlugin view.
// The view factory uses this to create PluginView without knowing the concrete controller type.
type TikiViewProvider interface {
	GetFilteredTasksForLane(lane int) []*tikipkg.Tiki
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
	quickSelectConfig *model.QuickSelectConfig
	quickSelectView   QuickSelectView
	workflowPath      string
	clipboardWriter   func([][]string) error
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

// SetQuickSelectConfig wires the quick-select config for choose() dispatch.
func (ir *InputRouter) SetQuickSelectConfig(qc *model.QuickSelectConfig) {
	ir.quickSelectConfig = qc
}

// SetQuickSelectView wires the quick-select view (concrete type satisfies the interface).
func (ir *InputRouter) SetQuickSelectView(qv QuickSelectView) {
	ir.quickSelectView = qv
}

// SetWorkflowPath sets the resolved workflow.yaml path for the Edit Workflow action.
func (ir *InputRouter) SetWorkflowPath(path string) {
	ir.workflowPath = path
}

// SetClipboardWriter overrides the clipboard backend used by the Execute prompt.
// Intended for tests that must avoid the real system clipboard. nil restores the default.
func (ir *InputRouter) SetClipboardWriter(fn func([][]string) error) {
	ir.clipboardWriter = fn
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

	// palette fires regardless of focus context (Ctrl+A can't conflict with typing)
	if action := ir.globalActions.Match(event); action != nil {
		if action.ID == ActionOpenPalette {
			ctx := BuildAppContext(currentView, ir.navController.GetActiveView())
			if ActionEnabled(*action, ctx) {
				return ir.handleGlobalAction(action.ID)
			}
			return false
		}
	}

	// if the input box is focused, let it handle all remaining input (including F10)
	if activeView := ir.navController.GetActiveView(); activeView != nil {
		if iv, ok := activeView.(InputableView); ok && iv.IsInputBoxFocused() {
			return false
		}
	}

	// pre-gate: global actions that must fire before task-edit Prepare() and before
	// search/fullscreen/editor gates
	if action := ir.globalActions.Match(event); action != nil {
		if action.ID == ActionToggleHeader {
			ctx := BuildAppContext(currentView, ir.navController.GetActiveView())
			if ActionEnabled(*action, ctx) {
				return ir.handleGlobalAction(action.ID)
			}
			return false
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
	if stop, handled := ir.maybeHandleDetailEditMode(activeView, currentView, event); stop {
		return handled
	}

	// check global actions first
	if action := ir.globalActions.Match(event); action != nil {
		ctx := BuildAppContext(currentView, activeView)
		if ActionEnabled(*action, ctx) {
			return ir.handleGlobalAction(action.ID)
		}
		return false
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

// detailEditModeView is the narrow contract InputRouter uses to detect a
// configurable detail view that has been flipped into Phase 2 edit mode.
// Stays in this file (rather than interfaces.go) because it is purely an
// implementation detail of input dispatch.
type detailEditModeView interface {
	IsEditMode() bool
	IsEditFieldFocused() bool
}

// maybeHandleDetailEditMode intercepts a small set of keys when the active
// view is a configurable detail view in in-place edit mode:
//   - Tab/Shift-Tab traverse editable metadata fields (via the controller).
//   - Ctrl+S commits the edit session and exits edit mode.
//   - Esc cancels the edit session and exits edit mode without popping
//     the view from the nav stack.
//
// Up/Down/typing are handed back to the focused EditSelectList by
// returning stop=false; the registry-based dispatcher then matches them
// to ActionNextValue/ActionPrevValue or the widget's own InputHandler.
func (ir *InputRouter) maybeHandleDetailEditMode(activeView View, currentView *ViewEntry, event *tcell.EventKey) (stop bool, handled bool) {
	if currentView == nil || !model.IsPluginViewID(currentView.ViewID) {
		return false, false
	}
	editView, ok := activeView.(detailEditModeView)
	if !ok || !editView.IsEditMode() {
		return false, false
	}

	pluginName := model.GetPluginName(currentView.ViewID)
	ctrl, hasCtrl := ir.pluginControllers[pluginName]
	if !hasCtrl {
		return false, false
	}

	switch event.Key() {
	case tcell.KeyEscape:
		return true, ctrl.HandleAction(ActionDetailCancel)
	case tcell.KeyCtrlS:
		return true, ctrl.HandleAction(ActionDetailSave)
	case tcell.KeyTab:
		return true, ctrl.HandleAction(ActionNextField)
	case tcell.KeyBacktab:
		return true, ctrl.HandleAction(ActionPrevField)
	}
	return false, false
}

// SetPluginRegistrar sets the callback used to register dynamically created plugins
// (e.g., the deps editor) with the view factory.
func (ir *InputRouter) SetPluginRegistrar(fn func(name string, cfg *model.PluginConfig, def plugin.Plugin, ctrl PluginControllerInterface)) {
	ir.registerPlugin = fn
}

// openDepsEditor creates (or reopens) a deps editor plugin for the
// given task ID. sourceDetailViewName is the configurable detail view
// the user opened the deps editor from (empty when opened from a
// non-detail context); the resolver attached to the deps controller
// uses it as the preferred return target so a workflow with multiple
// kind: detail views or a renamed detail view sends Enter back to the
// caller, not to a hardcoded "Detail".
func (ir *InputRouter) openDepsEditor(taskID, sourceDetailViewName string) bool {
	name := "Dependency:" + taskID
	viewID := model.MakePluginViewID(name)

	// reopen if already created — but refresh the resolver first so a
	// second open from a different configurable detail view sends Enter
	// back to the new caller, not the one captured on first open. Map
	// key is keyed by task id, so the controller instance is reused
	// across opens; without this swap, the closure over the original
	// sourceDetailViewName would route Enter to a stale view.
	if existing, exists := ir.pluginControllers[name]; exists {
		if dc, ok := existing.(*DepsController); ok {
			dc.SetDetailViewResolver(ir.makeDetailViewResolver(sourceDetailViewName))
		}
		ir.navController.PushView(viewID, nil)
		return true
	}

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{
			Name:        name,
			Description: model.DepsEditorViewDesc,
			ConfigIndex: -1,
			Kind:        plugin.KindBoard,
			Background:  config.GetColors().DepsEditorBackground,
		},
		TaskID: taskID,
		Lanes: []plugin.TikiLane{
			{Name: "Blocks"},
			{Name: "All"},
			{Name: "Depends"},
		},
	}

	if vm := config.GetPluginViewMode("Dependency"); vm != "" {
		pluginDef.Mode = vm
	}

	pluginConfig := model.NewPluginConfig("Dependency")
	pluginConfig.SetLaneLayout([]int{1, 2, 1}, []int{25, 50, 25})
	if pluginDef.Mode == "expanded" {
		pluginConfig.SetViewMode("expanded")
	}

	resolver := ir.makeDetailViewResolver(sourceDetailViewName)
	ctrl := NewDepsController(ir.taskStore, ir.mutationGate, pluginConfig, pluginDef, ir.navController, ir.statusline, ir.schema, resolver)

	if ir.registerPlugin != nil {
		ir.registerPlugin(name, pluginConfig, pluginDef, ctrl)
	}
	ir.pluginControllers[name] = ctrl

	ir.navController.PushView(viewID, nil)
	return true
}

// makeDetailViewResolver returns a closure that picks the configurable
// detail view name to use when the deps editor's Open action fires.
// Resolution order at dispatch time:
//  1. preferred — if non-empty AND still backed by a *DetailController
//     in the live registry. This is the view the deps editor was
//     opened from, so Enter returns the user where they came from.
//  2. any other plugin in pluginControllers whose controller is a
//     *DetailController. Map iteration order is non-deterministic, but
//     workflows with one detail view (the common case) always resolve
//     to it; workflows with several already had to pick *some* default.
//  3. empty string — no detail plugin loaded; the caller refuses Open.
//
// The resolver runs at dispatch time rather than at controller
// construction so it sees plugin registrations that occur later (e.g.
// dynamically added plugins) and so a renamed-on-reload detail view
// stays reachable.
func (ir *InputRouter) makeDetailViewResolver(preferred string) func() string {
	return func() string {
		if preferred != "" {
			if ctrl, ok := ir.pluginControllers[preferred]; ok {
				if _, isDetail := ctrl.(*DetailController); isDetail {
					return preferred
				}
			}
		}
		for name, ctrl := range ir.pluginControllers {
			if _, isDetail := ctrl.(*DetailController); isDetail {
				return name
			}
		}
		return ""
	}
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
		currentView := ir.navController.CurrentView()
		activeView := ir.navController.GetActiveView()
		ctx := BuildAppContext(currentView, activeView)
		if !ActionEnabled(*action, ctx) {
			return false
		}
		if action.ID == ActionSearch {
			return ir.handleSearchInput(ctrl)
		}
		if action.ID == ActionExecute {
			return ir.startExecuteInput()
		}
		if handled, ok := ir.dispatchDetailViewSharedAction(action.ID, currentView); ok {
			return handled
		}
		if targetPluginName := GetPluginNameFromAction(action.ID); targetPluginName != "" {
			// 6B.18 + 6B.24: selection passthrough + target-scoped
			// require evaluation in a single shared helper so direct
			// activation matches `kind: view` semantics.
			return ir.activateTargetView(viewID, model.MakePluginViewID(targetPluginName), targetPluginName)
		}
		if _, hasChoose := ctrl.GetActionChooseSpec(action.ID); hasChoose {
			return ir.startActionChoose(ctrl, action.ID)
		}
		if _, _, hasInput := ctrl.GetActionInputSpec(action.ID); hasInput {
			return ir.startActionInput(ctrl, action.ID)
		}
		return ctrl.HandleAction(action.ID)
	}
	return false
}

// dispatchDetailViewSharedAction handles actions that the configurable
// detail view inherits from the legacy task-detail view: opening the
// deps editor, invoking the AI chat agent, and opening the underlying
// markdown file in $EDITOR. The configurable detail view's controller
// is too narrow to own these paths (deps editor needs the InputRouter,
// chat needs the suspend/resume runner, edit-source needs the
// TaskController's reload semantics), so the router dispatches them
// directly when the carried selection is present.
//
// Returns (handled, true) when the action was recognized; (_, false)
// when the action is not a shared detail-view action and the caller
// should fall through to the controller dispatch path.
func (ir *InputRouter) dispatchDetailViewSharedAction(id ActionID, currentView *ViewEntry) (bool, bool) {
	switch id {
	case ActionEditDeps, ActionChat, ActionEditSource:
	default:
		return false, false
	}
	if currentView == nil {
		return false, true
	}
	taskID := model.DecodePluginViewParams(currentView.Params).TaskID
	if taskID == "" {
		return false, true
	}
	switch id {
	case ActionEditDeps:
		// Carry the source plugin name so deps editor's Open returns
		// here, not to a hardcoded "Detail" plugin (handles renamed
		// or multiple kind: detail views correctly).
		sourceName := model.GetPluginName(currentView.ViewID)
		return ir.openDepsEditor(taskID, sourceName), true
	case ActionChat:
		return ir.runChatForTask(taskID), true
	case ActionEditSource:
		ir.taskController.SetCurrentTask(taskID)
		return ir.taskController.HandleAction(ActionEditSource), true
	}
	return false, true
}

// runChatForTask invokes the configured AI agent against the given task
// file path, then reloads the task to surface any agent-applied edits.
// Mirrors the legacy task-detail chat path so the configurable detail
// view's `c` keybinding behaves identically.
func (ir *InputRouter) runChatForTask(taskID string) bool {
	agent := config.GetAIAgent()
	if agent == "" {
		return false
	}
	taskFilePath := ir.taskStore.PathForID(taskID)
	if taskFilePath == "" {
		taskFilePath = filepath.Join(config.GetDocDir(), taskID+".md")
	}
	name, args := resolveAgentCommand(agent, taskFilePath)
	ir.navController.SuspendAndRun(name, args...)
	if err := ir.taskStore.ReloadTask(taskID); err != nil && ir.statusline != nil {
		ir.statusline.SetMessage("reload failed: "+err.Error(), model.MessageLevelError, true)
	}
	return true
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
	case ActionEditWorkflow:
		if ir.workflowPath == "" {
			ir.statusline.SetMessage("no workflow file found", model.MessageLevelError, true)
			return true
		}
		if err := ir.navController.SuspendAndEdit(ir.workflowPath); err != nil {
			ir.statusline.SetMessage("editor failed: "+err.Error(), model.MessageLevelError, true)
		} else {
			ir.statusline.SetMessage("restart tiki to apply workflow changes", model.MessageLevelInfo, true)
		}
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

	activeView := ir.navController.GetActiveView()
	ctx := BuildAppContext(currentView, activeView)

	// global actions
	if action := ir.globalActions.GetByID(id); action != nil {
		if !ActionEnabled(*action, ctx) {
			return false
		}
		return ir.handleGlobalAction(id)
	}

	// enforce requirements for palette-dispatched actions
	if activeView != nil {
		for _, a := range activeView.GetActionRegistry().GetActions() {
			if a.ID == id && !ActionEnabled(a, ctx) {
				return false
			}
		}
	}

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
		// Legacy task-detail entry point: no source detail plugin name
		// to carry, so the resolver falls back to any kind: detail
		// plugin loaded from workflow.yaml.
		return ir.openDepsEditor(taskID, "")
	case ActionChat:
		agent := config.GetAIAgent()
		if agent == "" {
			return false
		}
		taskID := ir.taskController.GetCurrentTaskID()
		if taskID == "" {
			return false
		}
		taskFilePath := ir.taskStore.PathForID(taskID)
		if taskFilePath == "" {
			// Unknown to the store — fall back to the id-derived default so
			// the chat agent still gets a plausible target path.
			taskFilePath = filepath.Join(config.GetDocDir(), taskID+".md")
		}
		name, args := resolveAgentCommand(agent, taskFilePath)
		ir.navController.SuspendAndRun(name, args...)
		// Surface reload errors — the chat agent may have edited the file
		// into a state the strict loader refuses (id collision, invalid
		// type, …) and the user needs to know instead of seeing stale data.
		if err := ir.taskStore.ReloadTask(taskID); err != nil && ir.statusline != nil {
			ir.statusline.SetMessage("reload failed: "+err.Error(), model.MessageLevelError, true)
		}
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

// currentSelectionID returns the active view's currently-selected task id,
// or the empty string when no view/selection is available.
func (ir *InputRouter) currentSelectionID() string {
	active := ir.navController.GetActiveView()
	if active == nil {
		return ""
	}
	sv, ok := active.(SelectableView)
	if !ok {
		return ""
	}
	return sv.GetSelectedID()
}

// currentSelectionParams encodes the active view's selection into
// PluginViewParams for navigation dispatch that needs to carry selection.
// Returns nil when there is no active view, no selection, or the active
// view doesn't implement SelectableView; in those cases
// ReplaceView/PushView receives nil params which matches pre-6B.18
// behavior.
func (ir *InputRouter) currentSelectionParams() map[string]interface{} {
	id := ir.currentSelectionID()
	if id == "" {
		return nil
	}
	return model.EncodePluginViewParams(model.PluginViewParams{TaskID: id})
}

// activateTargetView is the shared direct-activation dispatcher. It
// evaluates the target view's require: list in target-scope — matching
// what `kind: view` action dispatch does — so both activation paths
// agree on whether navigation is allowed. Without this, direct activation
// would fall back to source-scoped evaluation via the merged require on
// the plugin:<name> activation action, producing different answers for
// `view:*` and other target-scoped requirements (6B.24).
//
// Returns true when navigation fired, false when the target-scoped
// gate refused (the caller's UI-level gate already passed, so returning
// false here silently swallows the keypress — matching how refused
// `kind: view` dispatches behave).
func (ir *InputRouter) activateTargetView(currentViewID, targetViewID model.ViewID, targetName string) bool {
	if currentViewID == targetViewID {
		// no-op navigation: pressing the view's own activation key
		// while already on it. The merged `!view:<self>` require
		// handles this at UI level; defensively match the legacy
		// success-without-navigation behavior here too.
		return true
	}
	carried := 0
	if ir.currentSelectionID() != "" {
		carried = 1
	}
	if !TargetViewEnabled(targetName, carried) {
		return false
	}
	ir.navController.ReplaceView(targetViewID, ir.currentSelectionParams())
	return true
}

// dispatchPluginAction handles palette-dispatched plugin actions by ActionID.
func (ir *InputRouter) dispatchPluginAction(id ActionID, viewID model.ViewID) bool {
	if targetPluginName := GetPluginNameFromAction(id); targetPluginName != "" {
		// 6B.18 + 6B.24: shared helper runs target-scoped require check
		// and carries selection in one place.
		return ir.activateTargetView(viewID, model.MakePluginViewID(targetPluginName), targetPluginName)
	}

	pluginName := model.GetPluginName(viewID)
	ctrl, ok := ir.pluginControllers[pluginName]
	if !ok {
		return false
	}

	if id == ActionSearch {
		return ir.handleSearchInput(ctrl)
	}
	if id == ActionExecute {
		return ir.startExecuteInput()
	}
	if handled, ok := ir.dispatchDetailViewSharedAction(id, ir.navController.CurrentView()); ok {
		return handled
	}

	if _, hasChoose := ctrl.GetActionChooseSpec(id); hasChoose {
		return ir.startActionChoose(ctrl, id)
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

// startExecuteInput opens the InputBar in action-input mode for ad-hoc ruki
// execution. Submitted text is run through rukiRuntime.RunQuery, the same path
// used by `tiki exec`, so validation, mutation persistence and triggers stay
// aligned with the CLI.
func (ir *InputRouter) startExecuteInput() bool {
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
		return ir.handleExecuteInput(text)
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

// handleExecuteInput runs the submitted ruki statement through the shared CLI
// runtime. Errors are shown in the statusline and keep the InputBar open so
// the user can correct the statement without retyping it.
func (ir *InputRouter) handleExecuteInput(text string) InputSubmitResult {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return InputKeepEditing
	}

	var buf bytes.Buffer
	opts := rukiRuntime.RunQueryOptions{ClipboardWriter: ir.clipboardWriter}
	if err := rukiRuntime.RunQueryWithOptions(ir.mutationGate, trimmed, &buf, opts); err != nil {
		if ir.statusline != nil {
			ir.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
		}
		return InputKeepEditing
	}

	if ir.statusline != nil {
		if summary := strings.TrimSpace(buf.String()); summary != "" {
			ir.statusline.SetMessage(summary, model.MessageLevelInfo, true)
		}
	}
	return InputClose
}

// startActionChoose opens the QuickSelect picker for an action that uses choose().
func (ir *InputRouter) startActionChoose(ctrl PluginControllerInterface, actionID ActionID) bool {
	if ir.quickSelectConfig == nil || ir.quickSelectView == nil {
		return false
	}
	_, tikis, ok := ctrl.CanStartActionChoose(actionID)
	if !ok {
		return false
	}

	ir.quickSelectConfig.SetOnSelect(func(taskID string) {
		ctrl.HandleActionChoose(actionID, taskID)
	})
	ir.quickSelectConfig.SetOnCancel(func() {})
	ir.quickSelectView.OnShow(tikis)
	ir.quickSelectConfig.SetVisible(true)
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
		currentView := ir.navController.CurrentView()
		activeView := ir.navController.GetActiveView()
		ctx := BuildAppContext(currentView, activeView)
		if !ActionEnabled(*action, ctx) {
			return false
		}
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
			// Legacy task-detail entry point: no source detail plugin
			// name to carry, so the resolver falls back to any kind:
			// detail plugin loaded from workflow.yaml.
			return ir.openDepsEditor(taskID, "")
		case ActionChat:
			agent := config.GetAIAgent()
			if agent == "" {
				return false
			}
			taskID := ir.taskController.GetCurrentTaskID()
			if taskID == "" {
				return false
			}
			taskFilePath := ir.taskStore.PathForID(taskID)
			if taskFilePath == "" {
				taskFilePath = filepath.Join(config.GetDocDir(), taskID+".md")
			}
			name, args := resolveAgentCommand(agent, taskFilePath)
			ir.navController.SuspendAndRun(name, args...)
			if err := ir.taskStore.ReloadTask(taskID); err != nil && ir.statusline != nil {
				ir.statusline.SetMessage("reload failed: "+err.Error(), model.MessageLevelError, true)
			}
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
