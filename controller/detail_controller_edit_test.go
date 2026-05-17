package controller

import (
	"testing"

	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
)

// fakeDetailEditView is a light stand-in for ConfigurableDetailView used to
// drive DetailController edit-mode tests without spinning up tview.
type fakeDetailEditView struct {
	editing         bool
	enterReturn     bool
	fullscreen      bool
	registry        *ActionRegistry
	changeHandler   func(bool)
	fieldHandlers   map[string]func(string)
	flushCalls      int  // count of FlushFocusedEditor invocations
	flushBeforeExit bool // whether the most recent flush happened while still editing
}

func (f *fakeDetailEditView) IsFullscreen() bool { return f.fullscreen }
func (f *fakeDetailEditView) EnterFullscreen()   { f.fullscreen = true }
func (f *fakeDetailEditView) ExitFullscreen()    { f.fullscreen = false }

func newFakeDetailEditView() *fakeDetailEditView {
	return &fakeDetailEditView{enterReturn: true, fieldHandlers: make(map[string]func(string))}
}

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
func (f *fakeDetailEditView) Metadata() []string                     { return []string{"status", "type", "priority"} }
func (f *fakeDetailEditView) FlushFocusedEditor() {
	f.flushCalls++
	f.flushBeforeExit = f.editing
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
	dc.SetSelectedTaskID(tk.ID)

	view := newFakeDetailEditView()
	dc.BindEditView(view)
	return dc, view, tc, tikiStore
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
