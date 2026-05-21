package controller

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"

	"github.com/gdamore/tcell/v2"
)

// DetailController backs `kind: detail` views. It surfaces both per-view
// actions (declared on the view itself) and global actions, dispatches
// `kind: view` navigations with selection passthrough, and routes ruki
// actions through the shared executor pipeline used by board views.
//
// Phase 1 scope: read-only view, fullscreen toggle, action dispatch.
// Phase 2 added in-place edit mode: the controller owns the edit-session
// lifecycle (start, commit, cancel) and routes per-field saves through
// TikiEditSession callbacks. The view itself owns the editor widgets and
// traversal state.
type DetailController struct {
	pluginDef      *plugin.DetailPlugin
	navController  *NavigationController
	statusline     *model.StatuslineConfig
	registry       *ActionRegistry
	executor       *PluginExecutor
	selectedTikiID string

	// Phase 2 edit-mode plumbing.
	editSession *TikiEditSession
	editView    DetailEditableView
}

// DetailEditableView is the contract the configurable detail view exposes
// to its controller for in-place edit mode plumbing. Kept narrow so the
// controller is decoupled from view internals and remains testable
// without spinning up tview.
type DetailEditableView interface {
	IsEditMode() bool
	EnterEditMode() bool
	// EnterEditModeWithFocus starts in-place edit mode with the given field
	// focused. When focusField is empty, behaves identically to EnterEditMode().
	EnterEditModeWithFocus(focusField model.EditField) bool
	ExitEditMode()
	FocusNextField() bool
	FocusPrevField() bool
	GetFocusedFieldName() string
	IsEditFieldFocused() bool
	SetEditModeRegistry(*ActionRegistry)
	SetEditModeChangeHandler(func(bool))
	SetEditFieldChangeHandler(string, func(string))
	SetEditTikiSource(func() *tikipkg.Tiki)
	Layout() []string
	// FlushFocusedEditor pushes the currently focused editor's value
	// through its onChange handler. Required before commit because some
	// editors (notably the tags textarea) only emit on Ctrl+S, and the
	// app-level input router consumes Ctrl+S to dispatch ActionDetailSave
	// before the focused widget sees it. Without an explicit flush, the
	// edit-in-progress would be discarded silently.
	FlushFocusedEditor()
}

// NewDetailController builds a controller for a kind: detail plugin view.
// tikiStore / mutationGate / schema may be nil only in trivial test fixtures
// that don't exercise ruki actions; in normal use the executor is wired so
// per-view ruki actions can fire. editSession is required for Phase 2
// in-place edit-mode dispatch; passing nil leaves the controller in
// Phase-1-compatible read-only behavior (Edit becomes a no-op).
func NewDetailController(
	pluginDef *plugin.DetailPlugin,
	navController *NavigationController,
	statusline *model.StatuslineConfig,
	tikiStore store.Store,
	mutationGate *service.TikiMutationGate,
	schema ruki.Schema,
	editSession *TikiEditSession,
) *DetailController {
	dc := &DetailController{
		pluginDef:     pluginDef,
		navController: navController,
		statusline:    statusline,
		registry:      DetailViewActions(),
		editSession:   editSession,
	}
	if tikiStore != nil && mutationGate != nil && schema != nil {
		dc.executor = NewPluginExecutor(tikiStore, mutationGate, statusline, schema,
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

// SetSelectedTikiID updates the carried selection. Called by the harness
// when navigation params arrive after construction.
func (dc *DetailController) SetSelectedTikiID(id string) {
	dc.selectedTikiID = id
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
	if dc.editSession != nil {
		tc := dc.editSession
		v.SetEditTikiSource(func() *tikipkg.Tiki {
			if tk := tc.GetDraftTiki(); tk != nil {
				return tk
			}
			return tc.GetEditingTiki()
		})
	}
	dc.wireEditFieldHandlers(v)
}

// wireEditFieldHandlers installs the per-field save callbacks on the view.
// Each handler forwards the editor's emitted value to the corresponding
// TikiEditSession.SaveX method on the editing copy. The actual disk write
// happens later in CommitEditSession when the user presses Ctrl+S.
//
// The send side (registry editor → onChange string) and receive side here
// together form the typed-bridging chain: the registry factory owns the
// typed→string conversion, and this method owns the string→typed parse so
// each Save* method gets its expected typed argument.
func (dc *DetailController) wireEditFieldHandlers(v DetailEditableView) {
	if dc.editSession == nil {
		return
	}
	v.SetEditFieldChangeHandler(tikipkg.FieldStatus, func(display string) {
		dc.editSession.SaveStatus(display)
	})
	v.SetEditFieldChangeHandler(tikipkg.FieldType, func(display string) {
		dc.editSession.SaveType(display)
	})
	v.SetEditFieldChangeHandler(tikipkg.FieldPriority, func(canonicalKey string) {
		// SemanticEnum editor emits canonical keys directly; no display→key
		// conversion needed at the controller boundary.
		dc.editSession.SavePriority(canonicalKey)
	})
	v.SetEditFieldChangeHandler(tikipkg.FieldPoints, func(display string) {
		// IntEditSelect enforces digits-only input; the err branch is a
		// defensive guard rather than a user-visible error path.
		if n, err := strconv.Atoi(display); err == nil {
			dc.editSession.SavePoints(n)
		}
	})
	v.SetEditFieldChangeHandler(tikipkg.FieldAssignee, func(display string) {
		dc.editSession.SaveAssignee(display)
	})
	v.SetEditFieldChangeHandler(tikipkg.FieldDue, func(display string) {
		dc.editSession.SaveDue(display)
	})
	v.SetEditFieldChangeHandler(tikipkg.FieldRecurrence, func(display string) {
		dc.editSession.SaveRecurrence(display)
	})
	v.SetEditFieldChangeHandler(tikipkg.FieldTags, func(display string) {
		dc.editSession.SaveTags(strings.Fields(display))
	})
	// Wire a SemanticEnum save handler for any workflow-declared enum
	// field in this view's layout that doesn't already have a built-in
	// handler above. Without this, custom enums (e.g. severity in
	// bug-tracker.yaml) would render as editable but never persist their
	// edits, because no save handler would be installed for them.
	for _, name := range v.Layout() {
		if _, hasBuiltin := builtinEditFieldHandlers[name]; hasBuiltin {
			continue
		}
		wfd, ok := workflow.Field(name)
		if !ok || wfd.Type != workflow.TypeEnum {
			continue
		}
		fieldName := name // capture for closure
		v.SetEditFieldChangeHandler(fieldName, func(canonicalKey string) {
			dc.editSession.SaveWorkflowEnum(fieldName, canonicalKey)
		})
	}
}

// builtinEditFieldHandlers names the fields whose save handlers are wired
// directly above in wireEditFieldHandlers. The custom-enum loop skips
// these so it doesn't double-register or shadow the typed Save* methods.
var builtinEditFieldHandlers = map[string]struct{}{
	tikipkg.FieldStatus:     {},
	tikipkg.FieldType:       {},
	tikipkg.FieldPriority:   {},
	tikipkg.FieldPoints:     {},
	tikipkg.FieldAssignee:   {},
	tikipkg.FieldDue:        {},
	tikipkg.FieldRecurrence: {},
	tikipkg.FieldTags:       {},
}

// HandleAction routes plugin actions: the fullscreen toggle, edit-mode
// commands, and workflow-declared per-view actions.
func (dc *DetailController) HandleAction(actionID ActionID) bool {
	switch actionID {
	case ActionFullscreen:
		return dc.toggleFullscreen()
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

// fullscreenToggler is the narrow fullscreen contract the controller needs
// from its bound view. Kept separate from FullscreenView (which embeds the
// tview-rooted View interface) so test fakes don't need to satisfy the
// whole tview surface.
type fullscreenToggler interface {
	IsFullscreen() bool
	EnterFullscreen()
	ExitFullscreen()
}

// toggleFullscreen flips the bound view's fullscreen state. The bound
// editView is the same instance the view factory wires as the active view,
// so the assertion always succeeds for *ConfigurableDetailView.
func (dc *DetailController) toggleFullscreen() bool {
	fv, ok := dc.editView.(fullscreenToggler)
	if !ok {
		return false
	}
	if fv.IsFullscreen() {
		fv.ExitFullscreen()
	} else {
		fv.EnterFullscreen()
	}
	return true
}

// enterEditMode starts a TikiEditSession edit session and flips the view
// into edit mode. Returns false if no editable field is configured.
func (dc *DetailController) enterEditMode() bool {
	return dc.enterEditModeWithFocus("")
}

// enterEditModeWithFocus mirrors enterEditMode but threads a focus
// field through to the view's EnterEditModeWithFocus. Empty focusField
// is equivalent to enterEditMode.
func (dc *DetailController) enterEditModeWithFocus(focusField model.EditField) bool {
	if dc.editView == nil || dc.editSession == nil || dc.selectedTikiID == "" {
		return false
	}
	if dc.editView.IsEditMode() {
		return true
	}
	if dc.editSession.StartEditSession(dc.selectedTikiID) == nil {
		return false
	}
	if !dc.editView.EnterEditModeWithFocus(focusField) {
		dc.editSession.CancelEditSession()
		return false
	}
	return true
}

// commitEdit persists the edit session via TikiEditSession. On success the
// view leaves edit mode; on failure the session stays open so the user
// can correct invalid input.
func (dc *DetailController) commitEdit() bool {
	if dc.editView == nil || dc.editSession == nil {
		return false
	}
	if !dc.editView.IsEditMode() {
		return false
	}
	// Flush the focused editor before commit. The Ctrl+S path goes through
	// the app-level input router (which doesn't re-dispatch the event to
	// the focused widget), so editors that only emit on Ctrl+S — like the
	// tags textarea — would otherwise lose their unsaved input.
	dc.editView.FlushFocusedEditor()
	if err := dc.editSession.CommitEditSession(); err != nil {
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
	if dc.editSession != nil {
		dc.editSession.CancelEditSession()
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
	if dc.selectedTikiID != "" {
		carried = 1
	}
	if !TargetViewEnabled(a.TargetView, carried) {
		return false
	}
	var params map[string]interface{}
	if dc.selectedTikiID != "" {
		params = model.EncodePluginViewParams(model.PluginViewParams{TikiID: dc.selectedTikiID})
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
	if dc.selectedTikiID != "" {
		selection = []string{dc.selectedTikiID}
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
// stub with a real edit-mode toggle (ActionDetailEdit). Phase 3 added
// the dependency-editor opener (Ctrl+D) so the configurable detail view
// fully replaces the legacy TikiDetailView's deps-open shortcut.
func DetailViewActions() *ActionRegistry {
	r := NewActionRegistry()
	idReq := []Requirement{RequireID}
	r.Register(Action{ID: ActionFullscreen, Key: tcell.KeyRune, Rune: 'f', Label: "Full screen", ShowInHeader: true})
	r.Register(Action{ID: ActionDetailEdit, Key: tcell.KeyRune, Rune: 'e', Label: "Edit", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionEditSource, Key: tcell.KeyRune, Rune: 's', Label: "Edit source", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionEditDeps, Key: tcell.KeyCtrlD, Modifier: tcell.ModCtrl, Label: "Dependencies", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionChat, Key: tcell.KeyRune, Rune: 'c', Label: "Chat", ShowInHeader: true, Require: []Requirement{RequireAI, RequireID}})
	return r
}

// DetailEditModeActions returns the action registry surfaced while a
// configurable detail view is in in-place edit mode. Mirrors the
// TikiEditView contract: Save commits the in-flight session, Tab/Shift-Tab
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
