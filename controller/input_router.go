package controller

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/model"
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

// TikiViewProvider is implemented by controllers that back a WorkflowPlugin view.
// The view factory uses this to create PluginView without knowing the concrete controller type.
type TikiViewProvider interface {
	GetFilteredTikisForLane(lane int) []*tikipkg.Tiki
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
	tikiEditSession   *TikiEditSession
	pluginControllers map[string]PluginControllerInterface // keyed by plugin name
	globalActions     *ActionRegistry
	tikiStore         store.Store
	mutationGate      *service.TikiMutationGate
	statusline        *model.StatuslineConfig
	schema            ruki.Schema
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
	tikiEditSession *TikiEditSession,
	pluginControllers map[string]PluginControllerInterface,
	tikiStore store.Store,
	mutationGate *service.TikiMutationGate,
	statusline *model.StatuslineConfig,
	schema ruki.Schema,
) *InputRouter {
	return &InputRouter{
		navController:     navController,
		tikiEditSession:   tikiEditSession,
		pluginControllers: pluginControllers,
		globalActions:     DefaultGlobalActions(),
		tikiStore:         tikiStore,
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
// 3. Configurable detail view in edit mode (Esc/Ctrl+S/Tab/Shift-Tab/Left/Right)
// 4. Global actions (Esc, Refresh)
// 5. View-specific actions (based on current view)
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

	// pre-gate: global actions that must fire before tiki-edit Prepare() and before
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

	if stop, handled := ir.maybeHandleInputBox(activeView, event); stop {
		return handled
	}
	if stop, handled := ir.maybeHandleFullscreenEscape(activeView, event); stop {
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
	if model.IsPluginViewID(currentView.ViewID) {
		return ir.handlePluginInput(event, currentView.ViewID)
	}
	return false
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
		if routeFieldAwareSave(activeView) {
			return true, true
		}
		return true, ctrl.HandleAction(ActionDetailSave)
	case tcell.KeyTab:
		return true, ctrl.HandleAction(ActionNextField)
	case tcell.KeyBacktab:
		return true, ctrl.HandleAction(ActionPrevField)
	case tcell.KeyLeft:
		if nav, ok := activeView.(RecurrencePartNavigable); ok {
			if nav.MoveRecurrencePartLeft() {
				return true, true
			}
		}
		return false, false
	case tcell.KeyRight:
		if nav, ok := activeView.(RecurrencePartNavigable); ok {
			if nav.MoveRecurrencePartRight() {
				return true, true
			}
		}
		return false, false
	}

	// When a free-form text editor (title InputField, assignee InputField,
	// description/tags TextArea, or an adapter wrapping one) holds focus,
	// typing keys must reach the widget rather than the global action
	// registry. Without this, single-letter globals like 'r' (Refresh),
	// 'q' (Quit), 'F1'..'F4' shadow the keystroke and the user cannot type
	// those characters into the field. Tab/Backtab/Esc/Ctrl-S are already
	// consumed above so this only covers the runes-and-backspace path.
	if isTextInputFocused(ir.navController.GetApp()) && isTextInputKey(event) {
		return true, false
	}

	return false, false
}

// isTextInputFocused reports whether the currently focused tview primitive
// is a free-form text editor — either tview.InputField/TextArea directly,
// or an adapter struct that embeds one (e.g. titleEditAdapter wraps
// *tview.InputField for title editing). Detection walks the focused
// value's struct fields via reflection so adapters don't need to register
// with a marker interface across packages.
func isTextInputFocused(app *tview.Application) bool {
	if app == nil {
		return false
	}
	focus := app.GetFocus()
	if focus == nil {
		return false
	}
	switch focus.(type) {
	case *tview.InputField, *tview.TextArea:
		return true
	}
	// adapter case: wrapper struct that embeds *tview.InputField or *tview.TextArea
	v := reflect.ValueOf(focus)
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < v.NumField(); i++ {
		fv := v.Field(i)
		if fv.Kind() != reflect.Ptr {
			continue
		}
		switch fv.Interface().(type) {
		case *tview.InputField, *tview.TextArea:
			return true
		}
	}
	return false
}

// isTextInputKey reports whether the event is a key that a free-form text
// editor needs to consume (printable rune, backspace, delete, navigation
// inside the field). Tab/Backtab/Esc/Ctrl-S are deliberately excluded —
// they belong to the edit-mode action registry.
func isTextInputKey(event *tcell.EventKey) bool {
	switch event.Key() {
	case tcell.KeyRune,
		tcell.KeyBackspace, tcell.KeyBackspace2,
		tcell.KeyDelete,
		tcell.KeyHome, tcell.KeyEnd:
		return true
	}
	return false
}

// routeFieldAwareSave dispatches Ctrl-S through the matching SaveXFromTextArea
// hook when the focused field is a buffered textarea (tags or description).
// Returns true when the hook fired; false leaves Ctrl-S to fall through to
// the standard ActionDetailSave dispatch.
func routeFieldAwareSave(activeView View) bool {
	focusable, ok := activeView.(FieldFocusableView)
	if !ok {
		return false
	}
	switch focusable.GetFocusedField() {
	case model.EditFieldTags:
		if tv, ok := activeView.(TagsTextAreaSavable); ok {
			tv.SaveTagsFromTextArea()
			return true
		}
	case model.EditFieldDescription:
		if dv, ok := activeView.(DescriptionTextAreaSavable); ok {
			dv.SaveDescriptionFromTextArea()
			return true
		}
	}
	return false
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
// detail view inherits from the legacy tiki-detail view: invoking the
// AI chat agent and opening the underlying markdown file in $EDITOR.
// The configurable detail view's controller is too narrow to own these
// paths (chat needs the suspend/resume runner, edit-source needs the
// TikiEditSession's reload semantics), so the router dispatches them
// directly when the carried selection is present.
//
// Returns (handled, true) when the action was recognized; (_, false)
// when the action is not a shared detail-view action and the caller
// should fall through to the controller dispatch path.
func (ir *InputRouter) dispatchDetailViewSharedAction(id ActionID, currentView *ViewEntry) (bool, bool) {
	switch id {
	case ActionChat, ActionEditSource:
	default:
		return false, false
	}
	if currentView == nil {
		return false, true
	}
	tikiID := model.DecodePluginViewParams(currentView.Params).TikiID
	if tikiID == "" {
		return false, true
	}
	switch id {
	case ActionChat:
		return ir.runChatForTiki(tikiID), true
	case ActionEditSource:
		ir.tikiEditSession.SetCurrentTiki(tikiID)
		return ir.tikiEditSession.HandleAction(ActionEditSource), true
	}
	return false, true
}

// runChatForTiki invokes the configured AI agent against the given tiki
// file path, then reloads the tiki to surface any agent-applied edits.
// Mirrors the legacy tiki-detail chat path so the configurable detail
// view's `c` keybinding behaves identically.
func (ir *InputRouter) runChatForTiki(tikiID string) bool {
	agent := config.GetAIAgent()
	if agent == "" {
		return false
	}
	tikiFilePath := ir.tikiStore.PathForID(tikiID)
	if tikiFilePath == "" {
		tikiFilePath = filepath.Join(config.GetDocDir(), tikiID+".md")
	}
	name, args := resolveAgentCommand(agent, tikiFilePath)
	ir.navController.SuspendAndRun(name, args...)
	if err := ir.tikiStore.ReloadTiki(tikiID); err != nil && ir.statusline != nil {
		ir.statusline.SetMessage("reload failed: "+err.Error(), model.MessageLevelError, true)
	}
	return true
}

// handleGlobalAction processes actions available in all views
func (ir *InputRouter) handleGlobalAction(actionID ActionID) bool {
	switch actionID {
	case ActionBack:
		return ir.navController.HandleBack()
	case ActionQuit:
		ir.navController.HandleQuit()
		return true
	case ActionRefresh:
		_ = ir.tikiStore.Reload()
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

	if model.IsPluginViewID(currentView.ViewID) {
		return ir.dispatchPluginAction(id, currentView.ViewID)
	}
	return false
}

// currentSelectionID returns the active view's currently-selected tiki id,
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
	return model.EncodePluginViewParams(model.PluginViewParams{TikiID: id})
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

	ir.quickSelectConfig.SetOnSelect(func(tikiID string) {
		ctrl.HandleActionChoose(actionID, tikiID)
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
