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
			if a.HasInput {
				slog.Debug("input() ruki action not surfaced on detail view",
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

// FieldFocusChangeNotifier is the optional view-side hook used by the
// controller to refresh statusline hints when the focused edit field
// changes. Views that don't implement it lose the per-field hint surface
// but otherwise function normally.
type FieldFocusChangeNotifier interface {
	SetFieldFocusChangeHandler(func(model.EditField))
}

// titleSaveSetter, descriptionSaveSetter, and tagsSaveSetter are narrow
// view-side hooks invoked by BindEditView to install the field-specific
// save / cancel callbacks. The configurable detail view (and any future
// view) implements only the setters its field set actually exposes —
// fakes need no opt-in for fields they don't exercise.
//
// The save semantics differ by field: title commits-and-closes, while
// description and tags commit-and-stay so the user can keep editing
// after the textarea-style Ctrl-S without the view popping away.
type titleSaveSetter interface {
	SetTitleSaveHandler(func(string))
	SetTitleCancelHandler(func())
}

type descriptionSaveSetter interface {
	SetDescriptionSaveHandler(func(string))
	SetDescriptionCancelHandler(func())
}

type tagsSaveSetter interface {
	SetTagsSaveHandler(func(string))
	SetTagsCancelHandler(func())
}

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
	if notifier, ok := v.(FieldFocusChangeNotifier); ok {
		notifier.SetFieldFocusChangeHandler(dc.updateFieldHint)
	}
	dc.wireFieldSaveHandlers(v)
}

// wireFieldSaveHandlers installs commit callbacks on the view's
// title / description / tags editors when the view exposes them.
// Title commits-and-closes (Enter on a single-line input ends the edit).
// Description and tags commit-and-stay so a Ctrl-S inside the textarea
// persists without popping the view.
func (dc *DetailController) wireFieldSaveHandlers(v DetailEditableView) {
	if t, ok := v.(titleSaveSetter); ok {
		t.SetTitleSaveHandler(func(string) { _ = dc.commitEdit() })
		t.SetTitleCancelHandler(func() { _ = dc.cancelEdit() })
	}
	if d, ok := v.(descriptionSaveSetter); ok {
		d.SetDescriptionSaveHandler(func(string) { dc.commitEditNoClose() })
		d.SetDescriptionCancelHandler(func() { _ = dc.cancelEdit() })
	}
	if t, ok := v.(tagsSaveSetter); ok {
		t.SetTagsSaveHandler(func(string) { dc.commitEditNoClose() })
		t.SetTagsCancelHandler(func() { _ = dc.cancelEdit() })
	}
}

// commitEditNoClose persists the in-flight edit session and immediately
// re-opens a fresh session against the same tiki, leaving the view in
// edit mode. Used for description / tags saves where the user typically
// keeps editing after Ctrl-S.
func (dc *DetailController) commitEditNoClose() {
	if dc.editView == nil || dc.editSession == nil {
		return
	}
	if !dc.editView.IsEditMode() {
		return
	}
	dc.editView.FlushFocusedEditor()
	if validator, ok := dc.editView.(interface {
		IsValid() bool
		ValidationErrors() []string
	}); ok && !validator.IsValid() {
		if dc.statusline != nil {
			if errs := validator.ValidationErrors(); len(errs) > 0 {
				dc.statusline.SetMessage(strings.Join(errs, "; "), model.MessageLevelError, true)
			}
		}
		return
	}
	if err := dc.editSession.CommitEditSession(); err != nil {
		if dc.statusline != nil {
			dc.statusline.SetMessage(rejectionMessage(err), model.MessageLevelError, true)
		}
		return
	}
	dc.editSession.StartEditSession(dc.selectedTikiID)
}

// updateFieldHint refreshes the statusline hint to reflect the controls
// available on the currently focused edit-mode field. Workflow-declared
// enums fall through to the generic ↑↓ hint via a workflow.Field lookup
// so custom enums (e.g. severity in bug-tracker.yaml) get the same hint
// without per-field hard-coding.
func (dc *DetailController) updateFieldHint(focused model.EditField) {
	if dc.statusline == nil {
		return
	}
	switch focused {
	case model.EditFieldStatus, model.EditFieldType, model.EditFieldPriority,
		model.EditFieldAssignee, model.EditFieldPoints, model.EditFieldDue:
		dc.statusline.SetMessage("↑↓ change value", model.MessageLevelInfo, false)
		return
	case model.EditFieldRecurrence:
		if nav, ok := dc.editView.(RecurrencePartNavigable); ok && nav.IsRecurrenceValueFocused() {
			dc.statusline.SetMessage("← edit pattern  ↑↓ change value", model.MessageLevelInfo, false)
		} else {
			dc.statusline.SetMessage("↑↓ change pattern  → edit value", model.MessageLevelInfo, false)
		}
		return
	}
	if wfd, ok := workflow.Field(string(focused)); ok && wfd.Type == workflow.TypeEnum {
		dc.statusline.SetMessage("↑↓ change value", model.MessageLevelInfo, false)
		return
	}
	dc.statusline.ClearMessage()
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
	v.SetEditFieldChangeHandler("title", func(text string) {
		dc.editSession.SaveTitle(text)
	})
	v.SetEditFieldChangeHandler("description", func(text string) {
		dc.editSession.SaveDescription(text)
	})
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
	"title":                 {},
	"description":           {},
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

// ApplyDetailMode runs the per-mode setup carried in PluginViewParams.
// Called by the view factory after BindEditView on a freshly built
// DetailController that has just been pushed onto the nav stack. For plain
// view (empty / DetailModeView) it is a no-op; the other four modes either
// enter edit mode with a specific focus, install a restricted action
// registry, or thread an already-created draft into the edit session.
func (dc *DetailController) ApplyDetailMode(mode plugin.DetailMode, focus model.EditField, draft *tikipkg.Tiki) bool {
	if dc.editView == nil {
		return false
	}
	switch mode {
	case "", plugin.DetailModeView:
		return true
	case plugin.DetailModeEdit:
		return dc.enterEditModeWithFocus(focus)
	case plugin.DetailModeNew:
		if draft == nil || dc.editSession == nil {
			return false
		}
		// drafts are not yet in the store — adopt the in-memory copy directly
		// instead of going through StartEditSession (which expects an existing
		// tiki to load).
		dc.editSession.SetDraft(draft)
		dc.selectedTikiID = draft.ID
		if dc.editView.IsEditMode() {
			return true
		}
		if !dc.editView.EnterEditModeWithFocus(model.EditFieldTitle) {
			dc.editSession.ClearDraft()
			return false
		}
		return true
	case plugin.DetailModeEditDesc:
		dc.editView.SetEditModeRegistry(DescOnlyEditActions())
		return dc.enterEditModeWithFocus(model.EditFieldDescription)
	case plugin.DetailModeEditTags:
		dc.editView.SetEditModeRegistry(TagsOnlyEditActions())
		return dc.enterEditModeWithFocus(model.EditFieldTags)
	}
	return false
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
	if validator, ok := dc.editView.(interface {
		IsValid() bool
		ValidationErrors() []string
	}); ok {
		if !validator.IsValid() {
			if dc.statusline != nil {
				if errs := validator.ValidationErrors(); len(errs) > 0 {
					dc.statusline.SetMessage(strings.Join(errs, "; "), model.MessageLevelError, true)
				}
			}
			return false
		}
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

// cancelEdit drops in-flight edits and leaves edit mode. When the cancel
// targets an unsaved draft (mode: new), the detail view has nothing to fall
// back to — the draft was never persisted, so view mode would render
// "(no tiki selected)". Pop the view stack instead so the user lands back
// on the originating board.
func (dc *DetailController) cancelEdit() bool {
	if dc.editView == nil {
		return false
	}
	if !dc.editView.IsEditMode() {
		return false
	}
	cancellingDraft := dc.editSession != nil && dc.editSession.GetDraftTiki() != nil
	if dc.editSession != nil {
		dc.editSession.CancelEditSession()
		if cancellingDraft {
			dc.editSession.ClearDraft()
		}
	}
	dc.editView.ExitEditMode()
	if cancellingDraft && dc.navController != nil {
		dc.navController.PopView()
	}
	return true
}

// focusNext flushes the currently focused editor before advancing. The
// title editor (and similar single-line inputs) only commits on Enter, so
// without this flush a user who types a title then Tabs away loses the
// typed value — the editing tiki keeps its empty title and the read-only
// title row renders blank during the post-Tab refresh.
func (dc *DetailController) focusNext() bool {
	if dc.editView == nil || !dc.editView.IsEditMode() {
		return false
	}
	dc.editView.FlushFocusedEditor()
	return dc.editView.FocusNextField()
}

func (dc *DetailController) focusPrev() bool {
	if dc.editView == nil || !dc.editView.IsEditMode() {
		return false
	}
	dc.editView.FlushFocusedEditor()
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
	var ids []string
	if dc.selectedTikiID != "" {
		ids = []string{dc.selectedTikiID}
	}
	var tikiStore store.Store
	if dc.executor != nil {
		tikiStore = dc.executor.tikiStore
	}
	pvp, ok := buildDetailViewParams(a.Mode, ids, tikiStore)
	if !ok {
		return false
	}
	dc.navController.PushView(model.MakePluginViewID(a.TargetView), model.EncodePluginViewParams(pvp))
	return true
}

// dispatchRukiAction runs a ruki-kind action through the shared executor.
// HasChoose actions never reach this path — the input router routes them
// through GetActionChooseSpec → CanStartActionChoose → HandleActionChoose.
func (dc *DetailController) dispatchRukiAction(a *plugin.PluginAction) bool {
	if dc.executor == nil {
		return false
	}
	if a.HasInput {
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

// Phase 1 stubs for the text-input pipeline. Ruki actions using `input(...)`
// are filtered out at registration time, so this pipeline is not reached for
// detail views. (Choose-driven ruki actions take a separate path: see
// GetActionChooseSpec / CanStartActionChoose / HandleActionChoose.)
func (dc *DetailController) GetActionInputSpec(ActionID) (string, ruki.ValueType, bool) {
	return "", 0, false
}
func (dc *DetailController) CanStartActionInput(ActionID) (string, ruki.ValueType, bool) {
	return "", 0, false
}
func (dc *DetailController) HandleActionInput(ActionID, string) InputSubmitResult {
	return InputKeepEditing
}

// findChooseAction looks up the per-view action carrying a `choose:` field
// (kind: view) or a `choose(...)` ruki call (kind: ruki) by canonical action id.
// Returns nil for any other action (plain kind: view without choose:, ruki
// without choose(), unknown ids).
func (dc *DetailController) findChooseAction(actionID ActionID) *plugin.PluginAction {
	keyStr := getPluginActionKeyStr(actionID)
	if keyStr == "" {
		return nil
	}
	for i := range dc.pluginDef.Actions {
		a := &dc.pluginDef.Actions[i]
		if a.KeyStr != keyStr {
			continue
		}
		if !a.HasChoose {
			return nil
		}
		if a.Kind != plugin.ActionKindView && a.Kind != plugin.ActionKindRuki {
			return nil
		}
		return a
	}
	return nil
}

// GetActionChooseSpec recognizes choose-driven actions so the input router
// routes them through the QuickSelect pipeline. Both kind: view actions with
// `choose:` and kind: ruki actions calling `choose(...)` are supported.
func (dc *DetailController) GetActionChooseSpec(actionID ActionID) (string, bool) {
	a := dc.findChooseAction(actionID)
	if a == nil {
		return "", false
	}
	return a.Label, true
}

// CanStartActionChoose evaluates the choose subquery to build candidates,
// using the detail view's own selected tiki id as the source-context "id()"
// rather than a board-style cursor selection. Eval errors surface to the
// statusline via PluginExecutor.EvalChooseFilter; empty results flow
// through and open an empty QuickSelect so the user sees the list state
// directly.
func (dc *DetailController) CanStartActionChoose(actionID ActionID) (string, []*tikipkg.Tiki, bool) {
	a := dc.findChooseAction(actionID)
	if a == nil || dc.executor == nil {
		return "", nil, false
	}
	var selection []string
	if dc.selectedTikiID != "" {
		selection = []string{dc.selectedTikiID}
	}
	input, ok := dc.executor.BuildExecutionInput(a, selection)
	if !ok {
		return "", nil, false
	}
	candidates, ok := dc.executor.EvalChooseFilter(a, input)
	if !ok {
		return "", nil, false
	}
	return a.Label, candidates, true
}

// HandleActionChoose receives the chosen tiki id from QuickSelect. For
// kind: view actions the chosen id replaces the source view's selection in
// the navigation params, so the target detail view opens on the picked tiki.
// For kind: ruki actions the chosen id is fed back into the validated
// statement via ExecutionInput.ChooseValue and the executor runs the update.
func (dc *DetailController) HandleActionChoose(actionID ActionID, tikiID string) bool {
	a := dc.findChooseAction(actionID)
	if a == nil || tikiID == "" {
		return false
	}
	if a.Kind == plugin.ActionKindRuki {
		return dc.handleRukiActionWithChosenID(a, tikiID)
	}
	if a.TargetView == "" || a.TargetView == dc.pluginDef.Name {
		return false
	}
	if !TargetViewEnabled(a.TargetView, 1) {
		return false
	}
	var tikiStore store.Store
	if dc.executor != nil {
		tikiStore = dc.executor.tikiStore
	}
	pvp, ok := buildDetailViewParams(a.Mode, []string{tikiID}, tikiStore)
	if !ok {
		return false
	}
	dc.navController.PushView(model.MakePluginViewID(a.TargetView), model.EncodePluginViewParams(pvp))
	return true
}

// handleRukiActionWithChosenID runs a ruki action whose `choose(...)` call
// has been resolved to a concrete tiki id by the QuickSelect overlay. The
// chosen id flows in via ExecutionInput.ChooseValue; the executor's
// choose() builtin reads it instead of prompting again.
func (dc *DetailController) handleRukiActionWithChosenID(a *plugin.PluginAction, tikiID string) bool {
	if dc.executor == nil {
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
	input.ChooseValue = tikiID
	input.HasChoose = true
	return dc.executor.Execute(a, input)
}

// DetailViewActions returns the built-in action registry for kind: detail
// when the view is in read-only mode.
func DetailViewActions() *ActionRegistry {
	r := NewActionRegistry()
	idReq := []Requirement{RequireID}
	r.Register(Action{ID: ActionFullscreen, Key: tcell.KeyRune, Rune: 'f', Label: "Full screen", ShowInHeader: true})
	r.Register(Action{ID: ActionDetailEdit, Key: tcell.KeyRune, Rune: 'e', Label: "Edit", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionEditSource, Key: tcell.KeyRune, Rune: 's', Label: "Edit source", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionChat, Key: tcell.KeyRune, Rune: 'c', Label: "Chat", ShowInHeader: true, Require: []Requirement{RequireAI, RequireID}})
	return r
}

// DetailEditModeActions returns the action registry surfaced while a
// configurable detail view is in in-place edit mode. Save commits the
// in-flight session, Tab/Shift-Tab traverse editable metadata fields,
// arrow keys cycle enum values, and Esc cancels.
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
