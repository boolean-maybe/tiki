package controller

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
)

// fakeDetailEditView is a light stand-in for ConfigurableDetailView used to
// drive DetailController edit-mode tests without spinning up tview.
type fakeDetailEditView struct {
	editing            bool
	enterReturn        bool
	fullscreen         bool
	registry           *ActionRegistry
	changeHandler      func(bool)
	fieldHandlers      map[string]func(string)
	flushCalls         int             // count of FlushFocusedEditor invocations
	flushBeforeExit    bool            // whether the most recent flush happened while still editing
	focusField         model.EditField // last EnterEditModeWithFocus argument
	valid              bool            // value returned by IsValid (defaults true via newFakeDetailEditView)
	validationErrs     []string        // value returned by ValidationErrors
	focusChangeHandler func(model.EditField)
}

func (f *fakeDetailEditView) IsFullscreen() bool { return f.fullscreen }
func (f *fakeDetailEditView) EnterFullscreen()   { f.fullscreen = true }
func (f *fakeDetailEditView) ExitFullscreen()    { f.fullscreen = false }

func newFakeDetailEditView() *fakeDetailEditView {
	return &fakeDetailEditView{
		enterReturn:   true,
		valid:         true,
		fieldHandlers: make(map[string]func(string)),
	}
}

func (f *fakeDetailEditView) IsValid() bool              { return f.valid }
func (f *fakeDetailEditView) ValidationErrors() []string { return f.validationErrs }

func (f *fakeDetailEditView) IsEditMode() bool         { return f.editing }
func (f *fakeDetailEditView) IsEditFieldFocused() bool { return f.editing }
func (f *fakeDetailEditView) EnterEditMode() bool {
	if !f.enterReturn {
		return false
	}
	f.editing = true
	if f.changeHandler != nil {
		f.changeHandler(true)
	}
	return true
}
func (f *fakeDetailEditView) EnterEditModeWithFocus(focusField model.EditField) bool {
	f.focusField = focusField
	return f.EnterEditMode()
}
func (f *fakeDetailEditView) ExitEditMode() {
	if !f.editing {
		return
	}
	f.editing = false
	if f.changeHandler != nil {
		f.changeHandler(false)
	}
}
func (f *fakeDetailEditView) FocusNextField() bool        { return f.editing }
func (f *fakeDetailEditView) FocusPrevField() bool        { return f.editing }
func (f *fakeDetailEditView) GetFocusedFieldName() string { return "status" }
func (f *fakeDetailEditView) SetEditModeRegistry(r *ActionRegistry) {
	f.registry = r
}
func (f *fakeDetailEditView) SetEditModeChangeHandler(h func(bool)) {
	f.changeHandler = h
}
func (f *fakeDetailEditView) SetEditFieldChangeHandler(name string, h func(string)) {
	if h == nil {
		delete(f.fieldHandlers, name)
		return
	}
	f.fieldHandlers[name] = h
}
func (f *fakeDetailEditView) SetEditTikiSource(func() *tikipkg.Tiki) {}
func (f *fakeDetailEditView) Layout() []string                       { return []string{"status", "type", "priority"} }
func (f *fakeDetailEditView) FlushFocusedEditor() {
	f.flushCalls++
	f.flushBeforeExit = f.editing
}
func (f *fakeDetailEditView) SetFieldFocusChangeHandler(h func(model.EditField)) {
	f.focusChangeHandler = h
}

// newDetailEditTestRig wires a TikiEditSession, mutation gate, store, and a
// committed test tiki so DetailController edit-mode lifecycles can be
// exercised end-to-end (sans tview).
func newDetailEditTestRig(t *testing.T) (*DetailController, *fakeDetailEditView, *TikiEditSession, store.Store) {
	t.Helper()

	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	nav := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, nav, nil)

	tk := tikipkg.New()
	tk.ID = "TIKI200"
	tk.Title = "Test"
	tk.Set(tikipkg.FieldStatus, "ready")
	tk.Set(tikipkg.FieldType, "story")
	tk.Set(tikipkg.FieldPriority, "medium")
	if err := tikiStore.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	pluginDef := newTestDetailPlugin([]string{"status", "type", "priority"}, nil)
	dc := NewDetailController(pluginDef, nav, nil, tikiStore, gate, rukiRuntime.NewSchema(), tc)
	dc.SetSelectedTikiID(tk.ID)

	view := newFakeDetailEditView()
	dc.BindEditView(view)
	return dc, view, tc, tikiStore
}

// newDetailEditTestRigWithStatusline mirrors newDetailEditTestRig but
// wires a real StatuslineConfig so tests can assert the message the
// validation gate emits on commit failure.
func newDetailEditTestRigWithStatusline(t *testing.T) (*DetailController, *fakeDetailEditView, *model.StatuslineConfig) {
	t.Helper()

	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	nav := newMockNavigationController()
	sl := model.NewStatuslineConfig()
	tc := NewTikiEditSession(tikiStore, gate, nav, sl)

	tk := tikipkg.New()
	tk.ID = "TIKI201"
	tk.Title = "Test"
	tk.Set(tikipkg.FieldStatus, "ready")
	tk.Set(tikipkg.FieldType, "story")
	tk.Set(tikipkg.FieldPriority, "medium")
	if err := tikiStore.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	pluginDef := newTestDetailPlugin([]string{"status", "type", "priority"}, nil)
	dc := NewDetailController(pluginDef, nav, sl, tikiStore, gate, rukiRuntime.NewSchema(), tc)
	dc.SetSelectedTikiID(tk.ID)

	view := newFakeDetailEditView()
	dc.BindEditView(view)
	return dc, view, sl
}

// TestDetailController_EnterEditModeStartsSession verifies that
// ActionDetailEdit toggles the view into edit mode and starts a
// TikiEditSession edit session bound to the selected tiki.
func TestDetailController_EnterEditModeStartsSession(t *testing.T) {
	dc, view, tc, _ := newDetailEditTestRig(t)

	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("ActionDetailEdit returned false")
	}
	if !view.IsEditMode() {
		t.Error("view should be in edit mode after ActionDetailEdit")
	}
	if tc.GetEditingTiki() == nil {
		t.Error("TikiEditSession should have an editing tiki after edit toggle")
	}
}

// TestDetailController_CancelEditDropsSession asserts that
// ActionDetailCancel cancels the in-flight edit session and exits the
// view's edit mode without writing changes.
func TestDetailController_CancelEditDropsSession(t *testing.T) {
	dc, view, tc, _ := newDetailEditTestRig(t)

	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("EnterEditMode")
	}
	// Mutate the editing copy so we can verify cancel reverts.
	tc.SaveStatus(enumDisplay("status", "inProgress"))

	if !dc.HandleAction(ActionDetailCancel) {
		t.Fatal("ActionDetailCancel returned false")
	}
	if view.IsEditMode() {
		t.Error("view should not be in edit mode after Cancel")
	}
	if tc.GetEditingTiki() != nil {
		t.Error("TikiEditSession editing tiki should be cleared after Cancel")
	}
}

// TestDetailController_CancelDraftPopsView pins that cancelling edit mode on
// an unsaved draft (the mode: new flow) pops the navigation stack instead
// of leaving the user on a detail view bound to a tiki ID that doesn't
// exist in the store. Without this, the screen renders "(no tiki selected)"
// after pressing Esc on a "n" draft.
func TestDetailController_CancelDraftPopsView(t *testing.T) {
	dc, view, tc, _ := newDetailEditTestRig(t)

	// Push a previous-view entry so PopView has somewhere to go.
	dc.navController.PushView(model.ViewID("source"), nil)
	dc.navController.PushView(model.ViewID("plugin:Detail"), nil)
	stackDepthBefore := dc.navController.Depth()

	draft := newTestDraftTiki("DRAFT9")
	if !dc.ApplyDetailMode(plugin.DetailModeNew, "", draft) {
		t.Fatal("ApplyDetailMode returned false for new mode")
	}
	if !view.IsEditMode() {
		t.Fatal("expected edit mode after ApplyDetailMode(new)")
	}

	if !dc.HandleAction(ActionDetailCancel) {
		t.Fatal("ActionDetailCancel returned false")
	}
	if view.IsEditMode() {
		t.Error("view should not be in edit mode after Cancel")
	}
	if tc.GetDraftTiki() != nil {
		t.Error("draft should be cleared after cancelling a new-mode edit")
	}
	if got := dc.navController.Depth(); got >= stackDepthBefore {
		t.Errorf("nav depth = %d, want < %d (cancel should pop the draft view)", got, stackDepthBefore)
	}
}

// TestDetailController_SaveCommitsAndExits verifies a happy-path commit:
// ActionDetailSave writes via TikiEditSession.CommitEditSession and exits
// edit mode.
func TestDetailController_SaveCommitsAndExits(t *testing.T) {
	dc, view, tc, tikiStore := newDetailEditTestRig(t)

	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("EnterEditMode")
	}
	// Apply a change through the field-registry change handler the
	// controller installed on the view.
	saver, ok := view.fieldHandlers["status"]
	if !ok || saver == nil {
		t.Fatal("status save handler not installed by controller")
	}
	saver(enumDisplay("status", "inProgress"))

	if !dc.HandleAction(ActionDetailSave) {
		t.Fatal("ActionDetailSave returned false")
	}
	if view.IsEditMode() {
		t.Error("view should not be in edit mode after Save")
	}
	if tc.GetEditingTiki() != nil {
		t.Error("editing tiki should be cleared after Save")
	}
	got := tikiStore.GetTiki("TIKI200")
	if got == nil {
		t.Fatal("tiki disappeared from store after save")
	}
	if v, _, _ := got.StringField(tikipkg.FieldStatus); v != "inProgress" {
		t.Errorf("status not persisted: got %q, want %q", v, "inProgress")
	}
}

// TestDetailController_SaveFlushesFocusedEditorBeforeCommit pins the
// non-obvious contract that ActionDetailSave must flush the currently
// focused editor before calling CommitEditSession. Editors like the tags
// textarea only push their value on Ctrl+S, but the app-level input
// router consumes Ctrl+S to dispatch ActionDetailSave — without the
// flush, that pending input would be silently lost.
func TestDetailController_SaveFlushesFocusedEditorBeforeCommit(t *testing.T) {
	dc, view, _, _ := newDetailEditTestRig(t)

	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("EnterEditMode")
	}
	if view.flushCalls != 0 {
		t.Fatalf("flush called before save: %d", view.flushCalls)
	}
	if !dc.HandleAction(ActionDetailSave) {
		t.Fatal("ActionDetailSave returned false")
	}
	if view.flushCalls != 1 {
		t.Errorf("FlushFocusedEditor called %d times, want 1", view.flushCalls)
	}
	if !view.flushBeforeExit {
		t.Error("FlushFocusedEditor must run while still in edit mode (before exit)")
	}
}

// TestDetailController_RegistrySwitchesOnEditMode verifies that the
// controller's registry swaps to the edit-mode actions while editing,
// and reverts to the read-only registry on exit.
func TestDetailController_RegistrySwitchesOnEditMode(t *testing.T) {
	dc, _, _, _ := newDetailEditTestRig(t)

	viewModeRegistry := dc.GetActionRegistry()
	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("EnterEditMode")
	}
	editRegistry := dc.GetActionRegistry()
	if viewModeRegistry == editRegistry {
		t.Error("registry should swap when entering edit mode")
	}
	if !dc.HandleAction(ActionDetailCancel) {
		t.Fatal("Cancel")
	}
	if dc.GetActionRegistry() == editRegistry {
		t.Error("registry should revert after exiting edit mode")
	}
}

// TestDetailController_TraversalActionsRoutedThroughHandleAction asserts
// that the input router can dispatch ActionNextField / ActionPrevField
// through the controller's HandleAction surface while in edit mode.
func TestDetailController_TraversalActionsRoutedThroughHandleAction(t *testing.T) {
	dc, _, _, _ := newDetailEditTestRig(t)

	if dc.HandleAction(ActionNextField) {
		t.Error("Tab should not work outside edit mode")
	}
	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("EnterEditMode")
	}
	if !dc.HandleAction(ActionNextField) {
		t.Error("Tab should be handled in edit mode")
	}
	if !dc.HandleAction(ActionPrevField) {
		t.Error("Shift+Tab should be handled in edit mode")
	}
}

// TestDetailController_TitleHandlerPersistsTypedText pins that the title
// edit-field change handler routes typed text through SaveTitle into the
// editing tiki. Without this, typing a title then leaving the input (Tab,
// commit, etc.) would silently lose the value because the title editor's
// onChange callback would have nowhere to land.
func TestDetailController_TitleHandlerPersistsTypedText(t *testing.T) {
	dc, view, tc, _ := newDetailEditTestRig(t)
	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("EnterEditMode")
	}
	saver, ok := view.fieldHandlers["title"]
	if !ok || saver == nil {
		t.Fatal("title save handler not installed by controller")
	}
	saver("My new task title")
	if got := tc.GetEditingTiki().Title; got != "My new task title" {
		t.Errorf("editing tiki title = %q, want %q", got, "My new task title")
	}
}

// TestDetailController_TabFlushesFocusedEditorBeforeMoving pins that Tab
// (ActionNextField) and Shift+Tab (ActionPrevField) flush the currently
// focused editor before moving focus. The title editor only commits its
// text on Enter — without a flush before Tab, a user who types a title
// then presses Tab loses the typed value, and the post-Tab refresh
// renders the title row blank from the still-empty editing tiki.
func TestDetailController_TabFlushesFocusedEditorBeforeMoving(t *testing.T) {
	dc, view, _, _ := newDetailEditTestRig(t)
	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("EnterEditMode")
	}
	view.flushCalls = 0

	if !dc.HandleAction(ActionNextField) {
		t.Fatal("ActionNextField returned false")
	}
	if view.flushCalls != 1 {
		t.Errorf("Tab: FlushFocusedEditor called %d times, want 1", view.flushCalls)
	}

	if !dc.HandleAction(ActionPrevField) {
		t.Fatal("ActionPrevField returned false")
	}
	if view.flushCalls != 2 {
		t.Errorf("Shift+Tab: FlushFocusedEditor called %d times total, want 2", view.flushCalls)
	}
}

// TestDetailController_FullscreenAction asserts that pressing 'f' in the
// detail view toggles fullscreen on the bound view via HandleAction. This
// is the path the input router takes when registry.Match resolves 'f' to
// ActionFullscreen and falls through to ctrl.HandleAction.
func TestDetailController_FullscreenAction(t *testing.T) {
	dc, fv, _, _ := newDetailEditTestRig(t)

	if fv.IsFullscreen() {
		t.Fatal("view should start non-fullscreen")
	}
	if !dc.HandleAction(ActionFullscreen) {
		t.Fatal("ActionFullscreen should be handled")
	}
	if !fv.IsFullscreen() {
		t.Error("first invocation should enter fullscreen")
	}
	if !dc.HandleAction(ActionFullscreen) {
		t.Fatal("ActionFullscreen should be handled (toggle off)")
	}
	if fv.IsFullscreen() {
		t.Error("second invocation should exit fullscreen")
	}
}

// TestDetailEditModeActions_HasExpectedKeys asserts the canonical edit-mode
// action registry contains Save (Ctrl+S), Cancel (Esc), and Tab/Shift-Tab
// as the plan requires.
func TestDetailEditModeActions_HasExpectedKeys(t *testing.T) {
	r := DetailEditModeActions()
	if r.GetByID(ActionDetailSave) == nil {
		t.Error("missing ActionDetailSave")
	}
	if r.GetByID(ActionDetailCancel) == nil {
		t.Error("missing ActionDetailCancel")
	}
	if r.GetByID(ActionNextField) == nil {
		t.Error("missing ActionNextField (Tab)")
	}
	if r.GetByID(ActionPrevField) == nil {
		t.Error("missing ActionPrevField (Shift+Tab)")
	}
}

// TestDetailController_EnterEditModeWithFocusStartsSession verifies that
// enterEditModeWithFocus starts an edit session, flips the view into
// edit mode, and threads the focus argument through to the view.
func TestDetailController_EnterEditModeWithFocusStartsSession(t *testing.T) {
	dc, view, tc, _ := newDetailEditTestRig(t)

	if !dc.enterEditModeWithFocus(model.EditFieldTitle) {
		t.Fatal("enterEditModeWithFocus returned false")
	}
	if !view.IsEditMode() {
		t.Error("view should be in edit mode")
	}
	if tc.GetEditingTiki() == nil {
		t.Error("TikiEditSession should have an editing tiki")
	}
	if view.focusField != model.EditFieldTitle {
		t.Errorf("view.focusField = %q, want %q", view.focusField, model.EditFieldTitle)
	}
}

// TestDetailController_EnterEditModeWithFocusEmptyMatchesEnterEditMode
// pins that an empty focus argument behaves like enterEditMode (the view
// receives the empty value and applies its default-focus fallback).
func TestDetailController_EnterEditModeWithFocusEmptyMatchesEnterEditMode(t *testing.T) {
	dc, view, _, _ := newDetailEditTestRig(t)

	if !dc.enterEditModeWithFocus("") {
		t.Fatal(`enterEditModeWithFocus("") returned false`)
	}
	if !view.IsEditMode() {
		t.Error("view should be in edit mode")
	}
	if view.focusField != "" {
		t.Errorf("view.focusField = %q, want empty", view.focusField)
	}
}

// TestDetailController_CommitEditRejectsInvalidWithStatuslineMessage
// pins the validation gate inside commitEdit: an invalid view aborts
// the commit and surfaces the joined error list to the statusline.
// Mirrors the gate that lived in TikiEditCoordinator before the
// configurable detail view absorbed in-place edit duties.
func TestDetailController_CommitEditRejectsInvalidWithStatuslineMessage(t *testing.T) {
	dc, view, sl := newDetailEditTestRigWithStatusline(t)

	view.valid = false
	view.validationErrs = []string{"title required", "points out of range"}

	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("ActionDetailEdit returned false")
	}
	if dc.commitEdit() {
		t.Fatal("commitEdit returned true but view is invalid")
	}
	msg, level, _ := sl.GetMessage()
	if !strings.Contains(msg, "title required") {
		t.Errorf("statusline message = %q, want it to contain %q", msg, "title required")
	}
	if !strings.Contains(msg, "points out of range") {
		t.Errorf("statusline message = %q, want it to contain %q", msg, "points out of range")
	}
	if level != model.MessageLevelError {
		t.Errorf("statusline level = %v, want MessageLevelError", level)
	}
	if !view.IsEditMode() {
		t.Error("view should remain in edit mode after rejected commit")
	}
}

// TestDetailController_CommitEditAcceptsWhenValid pins the happy path
// once the validation gate is in place: a valid view commits and exits
// edit mode just like before.
func TestDetailController_CommitEditAcceptsWhenValid(t *testing.T) {
	dc, view, _ := newDetailEditTestRigWithStatusline(t)

	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("ActionDetailEdit returned false")
	}
	if !dc.commitEdit() {
		t.Fatal("commitEdit returned false on a valid view")
	}
	if view.IsEditMode() {
		t.Error("view should exit edit mode after successful commit")
	}
}

// TestDetailController_EnterEditModeWithFocusNoSelectionReturnsFalse
// pins the precondition that enterEditModeWithFocus is a no-op when no
// tiki is selected.
func TestDetailController_EnterEditModeWithFocusNoSelectionReturnsFalse(t *testing.T) {
	dc, view, _, _ := newDetailEditTestRig(t)
	dc.SetSelectedTikiID("")

	if dc.enterEditModeWithFocus(model.EditFieldTitle) {
		t.Fatal("expected false when no tiki is selected")
	}
	if view.IsEditMode() {
		t.Error("view should not be in edit mode")
	}
}

// TestDetailController_FocusChangeUpdatesStatuslineHint pins the
// per-field hint surface ported from TikiEditCoordinator.updateFieldHint.
// On focus of a value-cyclable field the hint shows "↑↓ change value";
// on a free-text field (e.g. title) the statusline is cleared.
func TestDetailController_FocusChangeUpdatesStatuslineHint(t *testing.T) {
	dc, view, sl := newDetailEditTestRigWithStatusline(t)
	dc.BindEditView(view) // re-bind so the focus-change handler is installed
	if view.focusChangeHandler == nil {
		t.Fatal("controller did not wire a field focus-change handler")
	}

	view.focusChangeHandler(model.EditFieldStatus)
	msg, level, _ := sl.GetMessage()
	if !strings.Contains(msg, "↑↓ change value") {
		t.Errorf("status hint = %q, want it to contain %q", msg, "↑↓ change value")
	}
	if level != model.MessageLevelInfo {
		t.Errorf("hint level = %v, want MessageLevelInfo", level)
	}

	view.focusChangeHandler(model.EditFieldTitle)
	if msg, _, _ := sl.GetMessage(); msg != "" {
		t.Errorf("title focus should clear hint, got %q", msg)
	}
}

// TestDetailController_FocusChangeRecurrenceHint pins the two-mode
// recurrence hint: "edit pattern" guidance when the part-nav reports
// pattern-focused, "edit value" guidance when value-focused.
func TestDetailController_FocusChangeRecurrenceHint(t *testing.T) {
	dc, view, sl := newDetailEditTestRigWithStatusline(t)
	rv := &recurrenceFakeView{fakeDetailEditView: view}
	dc.BindEditView(rv)
	if rv.focusChangeHandler == nil {
		t.Fatal("controller did not wire a field focus-change handler")
	}

	rv.valueFocused = false
	rv.focusChangeHandler(model.EditFieldRecurrence)
	if msg, _, _ := sl.GetMessage(); !strings.Contains(msg, "change pattern") {
		t.Errorf("pattern-focused hint = %q, want it to mention pattern change", msg)
	}

	rv.valueFocused = true
	rv.focusChangeHandler(model.EditFieldRecurrence)
	if msg, _, _ := sl.GetMessage(); !strings.Contains(msg, "edit pattern") {
		t.Errorf("value-focused hint = %q, want it to mention pattern editing", msg)
	}
}

// recurrenceFakeView extends the fake with the RecurrencePartNavigable
// duck-type so the recurrence-hint branch is exercised.
type recurrenceFakeView struct {
	*fakeDetailEditView
	valueFocused bool
}

func (r *recurrenceFakeView) MoveRecurrencePartLeft() bool  { return false }
func (r *recurrenceFakeView) MoveRecurrencePartRight() bool { return false }
func (r *recurrenceFakeView) IsRecurrenceValueFocused() bool {
	return r.valueFocused
}

// titleSaveFake satisfies the controller's narrow title-save setter so
// BindEditView installs the title save / cancel callbacks on the fake.
type titleSaveFake struct {
	*fakeDetailEditView
	titleSave   func(string)
	titleCancel func()
}

func (t *titleSaveFake) SetTitleSaveHandler(h func(string)) { t.titleSave = h }
func (t *titleSaveFake) SetTitleCancelHandler(h func())     { t.titleCancel = h }

// descSaveFake satisfies the controller's narrow description-save setter.
type descSaveFake struct {
	*fakeDetailEditView
	descSave   func(string)
	descCancel func()
}

func (d *descSaveFake) SetDescriptionSaveHandler(h func(string)) { d.descSave = h }
func (d *descSaveFake) SetDescriptionCancelHandler(h func())     { d.descCancel = h }

// tagsSaveFake satisfies the controller's narrow tags-save setter.
type tagsSaveFake struct {
	*fakeDetailEditView
	tagsSave   func(string)
	tagsCancel func()
}

func (t *tagsSaveFake) SetTagsSaveHandler(h func(string)) { t.tagsSave = h }
func (t *tagsSaveFake) SetTagsCancelHandler(h func())     { t.tagsCancel = h }

// TestDetailController_TitleSaveHandlerCommitsAndExits pins the
// title-save semantics: triggering the wired handler commits the
// session and exits edit mode (close-on-save).
func TestDetailController_TitleSaveHandlerCommitsAndExits(t *testing.T) {
	dc, view, _ := newDetailEditTestRigWithStatusline(t)
	tv := &titleSaveFake{fakeDetailEditView: view}
	dc.BindEditView(tv)
	if tv.titleSave == nil {
		t.Fatal("BindEditView did not install a title save handler")
	}

	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("EnterEditMode")
	}
	tv.titleSave("New Title")
	if view.IsEditMode() {
		t.Error("title save should exit edit mode (close-on-save)")
	}
}

// TestDetailController_DescriptionSaveHandlerCommitsAndStays pins the
// description-save semantics: the wired handler commits and re-opens
// a fresh session, leaving the view in edit mode (stay-on-save).
func TestDetailController_DescriptionSaveHandlerCommitsAndStays(t *testing.T) {
	dc, view, _ := newDetailEditTestRigWithStatusline(t)
	dv := &descSaveFake{fakeDetailEditView: view}
	dc.BindEditView(dv)
	if dv.descSave == nil {
		t.Fatal("BindEditView did not install a description save handler")
	}

	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("EnterEditMode")
	}
	dv.descSave("New body")
	if !view.IsEditMode() {
		t.Error("description save should keep the view in edit mode (stay-on-save)")
	}
}

// TestDetailController_TagsSaveHandlerCommitsAndStays pins the
// tags-save semantics in the metadata-grid flow: stay-on-save.
func TestDetailController_TagsSaveHandlerCommitsAndStays(t *testing.T) {
	dc, view, _ := newDetailEditTestRigWithStatusline(t)
	tv := &tagsSaveFake{fakeDetailEditView: view}
	dc.BindEditView(tv)
	if tv.tagsSave == nil {
		t.Fatal("BindEditView did not install a tags save handler")
	}

	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("EnterEditMode")
	}
	tv.tagsSave("alpha beta")
	if !view.IsEditMode() {
		t.Error("tags save should keep the view in edit mode (stay-on-save)")
	}
}
