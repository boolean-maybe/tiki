package controller

import (
	"log/slog"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	"github.com/gdamore/tcell/v2"
)

// parsePriorityDisplay reverses a priority display label (e.g. "P1", "P2",
// "—") back to its numeric value (1..5; 0 = absent). Mirrors the helper
// used in TaskEditView so the two paths stay consistent.
func parsePriorityDisplay(display string) int {
	return taskpkg.PriorityFromDisplay(display)
}

// DetailController backs `kind: detail` views. It surfaces both per-view
// actions (declared on the view itself) and global actions, dispatches
// `kind: view` navigations with selection passthrough, and routes ruki
// actions through the shared executor pipeline used by board views.
//
// Phase 1 scope: read-only view, fullscreen toggle, action dispatch.
// Phase 2 added in-place edit mode: the controller owns the edit-session
// lifecycle (start, commit, cancel) and routes per-field saves through
// TaskController callbacks. The view itself owns the editor widgets and
// traversal state.
type DetailController struct {
	pluginDef      *plugin.DetailPlugin
	navController  *NavigationController
	statusline     *model.StatuslineConfig
	registry       *ActionRegistry
	executor       *PluginExecutor
	selectedTaskID string

	// Phase 2 edit-mode plumbing.
	taskController *TaskController
	editView       DetailEditableView
}

// DetailEditableView is the contract the configurable detail view exposes
// to its controller for Phase 2 edit-mode plumbing. Kept narrow so the
// controller is decoupled from view internals and remains testable
// without spinning up tview.
type DetailEditableView interface {
	IsEditMode() bool
	EnterEditMode() bool
	ExitEditMode()
	FocusNextField() bool
	FocusPrevField() bool
	GetFocusedFieldName() string
	IsEditFieldFocused() bool
	SetEditModeRegistry(*ActionRegistry)
	SetEditModeChangeHandler(func(bool))
	SetEditFieldChangeHandler(string, func(string))
}

// NewDetailController builds a controller for a kind: detail plugin view.
// taskStore / mutationGate / schema may be nil only in trivial test fixtures
// that don't exercise ruki actions; in normal use the executor is wired so
// per-view ruki actions can fire. taskController is required for Phase 2
// in-place edit-mode dispatch; passing nil leaves the controller in
// Phase-1-compatible read-only behavior (Edit becomes a no-op).
func NewDetailController(
	pluginDef *plugin.DetailPlugin,
	navController *NavigationController,
	statusline *model.StatuslineConfig,
	taskStore store.Store,
	mutationGate *service.TaskMutationGate,
	schema ruki.Schema,
	taskController *TaskController,
) *DetailController {
	dc := &DetailController{
		pluginDef:      pluginDef,
		navController:  navController,
		statusline:     statusline,
		registry:       DetailViewActions(),
		taskController: taskController,
	}
	if taskStore != nil && mutationGate != nil && schema != nil {
		dc.executor = NewPluginExecutor(taskStore, mutationGate, statusline, schema,
			pluginDef.GetName(), nil)
	}
	dc.registerPluginActions()
	return dc
}

// registerPluginActions adds the plugin's per-view (and merged global) actions
// to the registry. Mirrors the surfacing rules from PluginController so the
// header/palette show the same set the controller can actually fire.
func (dc *DetailController) registerPluginActions() {
	for _, a := range dc.pluginDef.Actions {
		switch a.Kind {
		case plugin.ActionKindView:
			// surface unconditionally; navigation has no executor deps
		case plugin.ActionKindRuki:
			if dc.executor == nil {
				continue
			}
			if a.HasInput || a.HasChoose {
				slog.Debug("interactive ruki action not surfaced on detail view",
					"view", dc.pluginDef.Name, "key", a.KeyStr)
				continue
			}
		default:
			continue
		}
		dc.registry.Register(Action{
			ID:           pluginActionID(a.KeyStr),
			Key:          a.Key,
			Rune:         a.Rune,
			Modifier:     a.Modifier,
			Label:        a.Label,
			ShowInHeader: a.ShowInHeader,
			Require:      toRequirements(a.Require),
		})
	}
}

// SetSelectedTaskID updates the carried selection. Called by the harness
// when navigation params arrive after construction.
func (dc *DetailController) SetSelectedTaskID(id string) {
	dc.selectedTaskID = id
}

// GetActionRegistry returns the view's action registry.
func (dc *DetailController) GetActionRegistry() *ActionRegistry { return dc.registry }

// GetPluginName returns the plugin name.
func (dc *DetailController) GetPluginName() string { return dc.pluginDef.Name }

// ShowNavigation returns false — detail views don't show plugin nav keys.
func (dc *DetailController) ShowNavigation() bool { return false }

// BindEditView attaches the detail view so the controller can drive
// in-place edit mode (toggle, traversal, save/cancel). The view factory
// wires this immediately after constructing the view.
func (dc *DetailController) BindEditView(v DetailEditableView) {
	dc.editView = v
	if v == nil {
		return
	}
	v.SetEditModeRegistry(DetailEditModeActions())
	v.SetEditModeChangeHandler(func(editing bool) {
		if editing {
			dc.registry = DetailEditModeActions()
		} else {
			dc.registry = DetailViewActions()
			dc.registerPluginActions()
		}
	})
	dc.wireEditFieldHandlers(v)
}

// wireEditFieldHandlers installs the per-field save callbacks on the view.
// Each handler forwards the editor's display value to the corresponding
// TaskController.SaveX method on the editing copy. The actual disk write
// happens later in CommitEditSession when the user presses Ctrl+S.
func (dc *DetailController) wireEditFieldHandlers(v DetailEditableView) {
	if dc.taskController == nil {
		return
	}
	v.SetEditFieldChangeHandler("status", func(display string) {
		dc.taskController.SaveStatus(display)
	})
	v.SetEditFieldChangeHandler("type", func(display string) {
		dc.taskController.SaveType(display)
	})
	v.SetEditFieldChangeHandler("priority", func(display string) {
		dc.taskController.SavePriority(parsePriorityDisplay(display))
	})
}

// HandleAction routes plugin actions. Built-in detail actions like
// Fullscreen are handled by the view itself via the input router; this
// method dispatches workflow-declared actions and Phase 2 edit-mode
// commands.
func (dc *DetailController) HandleAction(actionID ActionID) bool {
	switch actionID {
	case ActionDetailEdit:
		return dc.enterEditMode()
	case ActionDetailSave:
		return dc.commitEdit()
	case ActionDetailCancel:
		return dc.cancelEdit()
	case ActionNextField:
		return dc.focusNext()
	case ActionPrevField:
		return dc.focusPrev()
	}
	if keyStr := getPluginActionKeyStr(actionID); keyStr != "" {
		return dc.handlePluginAction(keyStr)
	}
	return false
}

// enterEditMode starts a TaskController edit session and flips the view
// into edit mode. Returns false if no editable field is configured.
func (dc *DetailController) enterEditMode() bool {
	if dc.editView == nil || dc.taskController == nil || dc.selectedTaskID == "" {
		return false
	}
	if dc.editView.IsEditMode() {
		return true
	}
	if dc.taskController.StartEditSession(dc.selectedTaskID) == nil {
		return false
	}
	if !dc.editView.EnterEditMode() {
		dc.taskController.CancelEditSession()
		return false
	}
	return true
}

// commitEdit persists the edit session via TaskController. On success the
// view leaves edit mode; on failure the session stays open so the user
// can correct invalid input.
func (dc *DetailController) commitEdit() bool {
	if dc.editView == nil || dc.taskController == nil {
		return false
	}
	if !dc.editView.IsEditMode() {
		return false
	}
	if err := dc.taskController.CommitEditSession(); err != nil {
		if dc.statusline != nil {
			dc.statusline.SetMessage(rejectionMessage(err), model.MessageLevelError, true)
		}
		return false
	}
	dc.editView.ExitEditMode()
	return true
}

// cancelEdit drops in-flight edits and leaves edit mode.
func (dc *DetailController) cancelEdit() bool {
	if dc.editView == nil {
		return false
	}
	if !dc.editView.IsEditMode() {
		return false
	}
	if dc.taskController != nil {
		dc.taskController.CancelEditSession()
	}
	dc.editView.ExitEditMode()
	return true
}

func (dc *DetailController) focusNext() bool {
	if dc.editView == nil || !dc.editView.IsEditMode() {
		return false
	}
	return dc.editView.FocusNextField()
}

func (dc *DetailController) focusPrev() bool {
	if dc.editView == nil || !dc.editView.IsEditMode() {
		return false
	}
	return dc.editView.FocusPrevField()
}

// IsEditMode reports whether the bound view is in in-place edit mode.
// Used by the input router to gate edit-mode key routing.
func (dc *DetailController) IsEditMode() bool {
	return dc.editView != nil && dc.editView.IsEditMode()
}

// handlePluginAction dispatches a plugin action by canonical key string.
func (dc *DetailController) handlePluginAction(keyStr string) bool {
	for i := range dc.pluginDef.Actions {
		a := &dc.pluginDef.Actions[i]
		if a.KeyStr != keyStr {
			continue
		}
		switch a.Kind {
		case plugin.ActionKindView:
			return dc.dispatchViewAction(a)
		case plugin.ActionKindRuki:
			return dc.dispatchRukiAction(a)
		}
	}
	return false
}

// dispatchViewAction navigates to the target view, carrying the current
// selection as PluginViewParams.
//
// Self-target actions (target == this plugin's name) are refused as a
// belt-and-suspenders guard. The loader already filters these out of the
// per-view Actions slice; this catches any case where an author declares
// the same action per-view, where a dynamic plugin path injects one, or
// where a future merge change reintroduces them. Without the guard, Enter
// on Detail would push another Detail copy onto the stack indefinitely.
func (dc *DetailController) dispatchViewAction(a *plugin.PluginAction) bool {
	if a.TargetView == "" {
		return false
	}
	if a.TargetView == dc.pluginDef.Name {
		return false
	}
	carried := 0
	if dc.selectedTaskID != "" {
		carried = 1
	}
	if !TargetViewEnabled(a.TargetView, carried) {
		return false
	}
	var params map[string]interface{}
	if dc.selectedTaskID != "" {
		params = model.EncodePluginViewParams(model.PluginViewParams{TaskID: dc.selectedTaskID})
	}
	dc.navController.PushView(model.MakePluginViewID(a.TargetView), params)
	return true
}

// dispatchRukiAction runs a ruki-kind action through the shared executor.
func (dc *DetailController) dispatchRukiAction(a *plugin.PluginAction) bool {
	if dc.executor == nil {
		return false
	}
	if a.HasInput || a.HasChoose {
		// belt-and-suspenders: filtered out at registration too
		return false
	}
	var selection []string
	if dc.selectedTaskID != "" {
		selection = []string{dc.selectedTaskID}
	}
	input, ok := dc.executor.BuildExecutionInput(a, selection)
	if !ok {
		return false
	}
	return dc.executor.Execute(a, input)
}

// HandleSearch is a no-op for detail views.
func (dc *DetailController) HandleSearch(string) {}

// Phase 1 stubs for input/choose pipelines. Phase 2 may extend these as
// editor support lands; for now interactive ruki actions are filtered out
// at registration time so these are not reached for detail views.
func (dc *DetailController) GetActionInputSpec(ActionID) (string, ruki.ValueType, bool) {
	return "", 0, false
}
func (dc *DetailController) CanStartActionInput(ActionID) (string, ruki.ValueType, bool) {
	return "", 0, false
}
func (dc *DetailController) HandleActionInput(ActionID, string) InputSubmitResult {
	return InputKeepEditing
}
func (dc *DetailController) GetActionChooseSpec(ActionID) (string, bool) { return "", false }
func (dc *DetailController) CanStartActionChoose(ActionID) (string, []*tikipkg.Tiki, bool) {
	return "", nil, false
}
func (dc *DetailController) HandleActionChoose(ActionID, string) bool { return false }

// DetailViewActions returns the built-in action registry for kind: detail
// when the view is in read-only mode. Phase 2 replaced the Phase 1 Edit
// stub with a real edit-mode toggle (ActionDetailEdit).
func DetailViewActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionFullscreen, Key: tcell.KeyRune, Rune: 'f', Label: "Full screen", ShowInHeader: true})
	r.Register(Action{ID: ActionDetailEdit, Key: tcell.KeyRune, Rune: 'e', Label: "Edit", ShowInHeader: true, Require: []Requirement{RequireID}})
	return r
}

// DetailEditModeActions returns the action registry surfaced while a
// configurable detail view is in in-place edit mode. Mirrors the
// TaskEditView contract: Save commits the in-flight session, Tab/Shift-Tab
// traverse editable metadata fields, Esc cancels.
func DetailEditModeActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionDetailSave, Key: tcell.KeyCtrlS, Label: "Save", ShowInHeader: true})
	r.Register(Action{ID: ActionDetailCancel, Key: tcell.KeyEscape, Label: "Cancel", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionNextField, Key: tcell.KeyTab, Label: "Next", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionPrevField, Key: tcell.KeyBacktab, Label: "Prev", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionNextValue, Key: tcell.KeyDown, Label: "Next ↓", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionPrevValue, Key: tcell.KeyUp, Label: "Prev ↑", ShowInHeader: true, HideFromPalette: true})
	return r
}
