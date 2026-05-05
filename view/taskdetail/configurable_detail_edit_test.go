package taskdetail

import (
	"testing"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/store"
)

// TestConfigurableDetailView_EnterAndExitEditMode verifies the in-place
// edit-mode toggle: entering flips the flag, exits revert it, and the
// action registry swaps as the controller installed it.
func TestConfigurableDetailView_EnterAndExitEditMode(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI100")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	cv := NewConfigurableDetailView(
		s, tk.ID, "Detail",
		[]string{"status", "type", "priority"},
		controller.DetailViewActions(),
		nil, nil,
	)
	editReg := controller.DetailEditModeActions()
	cv.SetEditModeRegistry(editReg)

	if cv.IsEditMode() {
		t.Fatal("view should start in read-only mode")
	}
	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode returned false on a view with implemented editors")
	}
	if !cv.IsEditMode() {
		t.Error("IsEditMode should be true after EnterEditMode")
	}
	if cv.GetActionRegistry() != editReg {
		t.Error("registry should switch to edit-mode registry")
	}
	cv.ExitEditMode()
	if cv.IsEditMode() {
		t.Error("IsEditMode should be false after ExitEditMode")
	}
}

// TestConfigurableDetailView_TabTraversesEditableFields ensures Tab
// advances focus across editable metadata in `metadata:` order, and
// Shift-Tab moves backward, both stopping at the boundaries.
func TestConfigurableDetailView_TabTraversesEditableFields(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI101")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, "Detail",
		[]string{"status", "type", "priority"},
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode failed")
	}
	if got := cv.GetFocusedFieldName(); got != "status" {
		t.Errorf("initial focus = %q, want %q", got, "status")
	}
	if !cv.FocusNextField() {
		t.Fatal("FocusNextField returned false at status")
	}
	if got := cv.GetFocusedFieldName(); got != "type" {
		t.Errorf("after Tab = %q, want %q", got, "type")
	}
	if !cv.FocusNextField() {
		t.Fatal("FocusNextField returned false at type")
	}
	if got := cv.GetFocusedFieldName(); got != "priority" {
		t.Errorf("after Tab = %q, want %q", got, "priority")
	}
	if cv.FocusNextField() {
		t.Error("FocusNextField should return false at last field")
	}
	if !cv.FocusPrevField() {
		t.Fatal("FocusPrevField at priority returned false")
	}
	if got := cv.GetFocusedFieldName(); got != "type" {
		t.Errorf("after Shift+Tab = %q, want %q", got, "type")
	}
}

// TestConfigurableDetailView_StubFieldsAreSkippedInTraversal asserts
// fields whose semantic type only has a stub editor (e.g. text/integer in
// Phase 2) render but do not participate in Tab traversal.
func TestConfigurableDetailView_StubFieldsAreSkippedInTraversal(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI102")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, "Detail",
		// assignee/points/due are stubs in Phase 2; only status/type/priority
		// are implemented.
		[]string{"status", "assignee", "type", "points", "priority", "due"},
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode failed")
	}
	visited := []string{cv.GetFocusedFieldName()}
	for cv.FocusNextField() {
		visited = append(visited, cv.GetFocusedFieldName())
	}
	want := []string{"status", "type", "priority"}
	if len(visited) != len(want) {
		t.Fatalf("visited %v, want %v", visited, want)
	}
	for i, name := range want {
		if visited[i] != name {
			t.Errorf("visited[%d] = %q, want %q", i, visited[i], name)
		}
	}
}

// TestConfigurableDetailView_NoEditableFieldsLeavesViewMode asserts the
// edit-mode toggle is a no-op when no configured field has an
// implemented editor — the view stays in read-only mode rather than
// trapping the user with no usable fields.
func TestConfigurableDetailView_NoEditableFieldsLeavesViewMode(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI103")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, "Detail",
		[]string{"assignee", "points", "due"}, // all stubs
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if cv.EnterEditMode() {
		t.Error("EnterEditMode should return false when no field has an implemented editor")
	}
	if cv.IsEditMode() {
		t.Error("IsEditMode should remain false")
	}
}

// TestFieldRegistry_StatusTypePriorityImplemented asserts that exactly the
// three Phase 2 default fields advertise editor implementations; the rest
// remain stubs surfaced via Capability.
func TestFieldRegistry_StatusTypePriorityImplemented(t *testing.T) {
	implemented := []SemanticType{SemanticStatus, SemanticType_, SemanticPriority}
	for _, sem := range implemented {
		t.Run(string(sem), func(t *testing.T) {
			ui, _ := LookupType(sem)
			if ui.Capability != EditorImplemented {
				t.Errorf("%q: Capability = %v, want EditorImplemented", sem, ui.Capability)
			}
			if ui.Edit == nil {
				t.Errorf("%q: Edit factory is nil", sem)
			}
		})
	}
	stubs := []SemanticType{
		SemanticText, SemanticInteger, SemanticBoolean,
		SemanticDate, SemanticDateTime, SemanticRecurrence,
		SemanticEnum, SemanticStringList, SemanticTaskIDList,
	}
	for _, sem := range stubs {
		t.Run(string(sem)+"_stub", func(t *testing.T) {
			ui, _ := LookupType(sem)
			if ui.Capability != EditorStub {
				t.Errorf("%q: expected EditorStub, got %v", sem, ui.Capability)
			}
		})
	}
}

// TestConfigurableDetailView_FiresActionChangeHandlerOnToggle locks in
// the contract RootLayout depends on: edit-mode toggles must invoke the
// ActionChangeNotifier handler so the header bar and palette resync to
// the new registry. Without this fire, dispatch keeps working but the UI
// keeps showing read-only actions while edit mode is active.
func TestConfigurableDetailView_FiresActionChangeHandlerOnToggle(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI104")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, "Detail",
		[]string{"status", "type", "priority"},
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	calls := 0
	cv.SetActionChangeHandler(func() { calls++ })

	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode failed")
	}
	if calls != 1 {
		t.Errorf("after EnterEditMode: handler fired %d times, want 1", calls)
	}
	cv.ExitEditMode()
	if calls != 2 {
		t.Errorf("after ExitEditMode: handler fired %d times, want 2", calls)
	}
}

// TestFieldHasEditor_OnlyImplementedFieldsReturnTrue verifies the
// FieldHasEditor predicate the view uses to gate Tab traversal.
func TestFieldHasEditor_OnlyImplementedFieldsReturnTrue(t *testing.T) {
	for _, name := range []string{"status", "type", "priority"} {
		if !FieldHasEditor(name) {
			t.Errorf("FieldHasEditor(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"assignee", "points", "due", "recurrence", "tags", "dependsOn"} {
		if FieldHasEditor(name) {
			t.Errorf("FieldHasEditor(%q) = true, want false (stub editor in Phase 2)", name)
		}
	}
	if FieldHasEditor("not_a_field") {
		t.Error("FieldHasEditor on unknown field should return false")
	}
}
