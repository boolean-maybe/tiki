package controller

import (
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// mockTaskEditView implements View + TaskEditView for coordinator commit tests.
type mockTaskEditView struct {
	title       string
	description string
	tags        []string
}

func (m *mockTaskEditView) GetPrimitive() tview.Primitive      { return nil }
func (m *mockTaskEditView) GetActionRegistry() *ActionRegistry { return nil }
func (m *mockTaskEditView) GetViewID() model.ViewID            { return "" }
func (m *mockTaskEditView) OnFocus()                           {}
func (m *mockTaskEditView) OnBlur()                            {}
func (m *mockTaskEditView) GetEditedTitle() string             { return m.title }
func (m *mockTaskEditView) GetEditedDescription() string       { return m.description }
func (m *mockTaskEditView) GetEditedTags() []string            { return m.tags }

// mockFieldFocusableView implements FieldFocusableView + RecurrencePartNavigable for hint tests.
type mockFieldFocusableView struct {
	mockTaskEditView
	focusedField model.EditField
	valueFocused bool
}

func (m *mockFieldFocusableView) SetFocusedField(field model.EditField) { m.focusedField = field }
func (m *mockFieldFocusableView) GetFocusedField() model.EditField      { return m.focusedField }
func (m *mockFieldFocusableView) FocusNextField() bool {
	m.focusedField = model.NextField(m.focusedField)
	return true
}
func (m *mockFieldFocusableView) FocusPrevField() bool {
	m.focusedField = model.PrevField(m.focusedField)
	return true
}
func (m *mockFieldFocusableView) IsEditFieldFocused() bool       { return true }
func (m *mockFieldFocusableView) MoveRecurrencePartLeft() bool   { m.valueFocused = false; return true }
func (m *mockFieldFocusableView) MoveRecurrencePartRight() bool  { m.valueFocused = true; return true }
func (m *mockFieldFocusableView) IsRecurrenceValueFocused() bool { return m.valueFocused }

// mockNonEditView implements only View (not TaskEditView).
type mockNonEditView struct{}

func (m *mockNonEditView) GetPrimitive() tview.Primitive      { return nil }
func (m *mockNonEditView) GetActionRegistry() *ActionRegistry { return nil }
func (m *mockNonEditView) GetViewID() model.ViewID            { return "" }
func (m *mockNonEditView) OnFocus()                           {}
func (m *mockNonEditView) OnBlur()                            {}

// mockValidatableEditView adds IsValid()/ValidationErrors() to mockTaskEditView.
type mockValidatableEditView struct {
	mockTaskEditView
	valid  bool
	errors []string
}

func (m *mockValidatableEditView) IsValid() bool              { return m.valid }
func (m *mockValidatableEditView) ValidationErrors() []string { return m.errors }

// --- HandleKey tests ---

func TestTaskEditCoordinator_HandleKey_TagsOnly_Tab(t *testing.T) {
	coord := &TaskEditCoordinator{tagsOnly: true}
	event := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)

	got := coord.HandleKey(nil, event)
	if got {
		t.Error("HandleKey(Tab) should return false in tagsOnly mode")
	}
}

func TestTaskEditCoordinator_HandleKey_TagsOnly_Backtab(t *testing.T) {
	coord := &TaskEditCoordinator{tagsOnly: true}
	event := tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)

	got := coord.HandleKey(nil, event)
	if got {
		t.Error("HandleKey(Backtab) should return false in tagsOnly mode")
	}
}

func TestTaskEditCoordinator_HandleKey_Escape(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	nav := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, nav, nil)
	tc.SetDraft(newTestTask())

	coord := NewTaskEditCoordinator(nav, tc)
	event := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)

	got := coord.HandleKey(nil, event)
	if !got {
		t.Error("HandleKey(Escape) should return true")
	}

	if tc.GetDraftTask() != nil {
		t.Error("Escape should clear draft task")
	}
}

// --- commit tests ---

func TestTaskEditCoordinator_Commit_SavesTags(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	nav := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, nav, nil)

	draft := newTestTask()
	draft.Title = "Tagged Task"
	tc.SetDraft(draft)

	coord := NewTaskEditCoordinator(nav, tc)
	view := &mockTaskEditView{
		title:       "Tagged Task",
		description: "some description",
		tags:        []string{"api", "backend"},
	}

	got := coord.commit(view)
	if !got {
		t.Fatal("commit() should return true")
	}

	// verify task was committed to store with correct tags
	saved := taskStore.GetTask(draft.ID)
	if saved == nil {
		t.Fatal("task not found in store after commit")
	}
	if len(saved.Tags) != 2 || saved.Tags[0] != "api" || saved.Tags[1] != "backend" {
		t.Errorf("saved tags = %v, want [api backend]", saved.Tags)
	}
}

func TestTaskEditCoordinator_Commit_NonEditView(t *testing.T) {
	s := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(s)
	nav := newMockNavigationController()
	tc := NewTaskController(s, gate, nav, nil)
	coord := NewTaskEditCoordinator(nav, tc)

	got := coord.commit(&mockNonEditView{})
	if got {
		t.Error("commit() should return false for non-TaskEditView")
	}
}

func TestTaskEditCoordinator_Commit_ValidationFails(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	sl := model.NewStatuslineConfig()
	nav := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, nav, sl)
	tc.SetDraft(newTestTask())

	coord := NewTaskEditCoordinator(nav, tc)
	view := &mockValidatableEditView{
		mockTaskEditView: mockTaskEditView{
			title:       "",
			description: "desc",
			tags:        []string{"tag"},
		},
		valid:  false,
		errors: []string{"title is required"},
	}

	got := coord.commit(view)
	if got {
		t.Error("commit() should return false when IsValid() returns false")
	}

	msg, level, autoHide := sl.GetMessage()
	if msg != "title is required" {
		t.Errorf("statusline message = %q, want %q", msg, "title is required")
	}
	if level != model.MessageLevelError {
		t.Errorf("level = %q, want %q", level, model.MessageLevelError)
	}
	if !autoHide {
		t.Error("autoHide should be true for validation errors")
	}
}

func TestTaskEditCoordinator_Commit_ValidationMultipleErrors(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	sl := model.NewStatuslineConfig()
	nav := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, nav, sl)
	tc.SetDraft(newTestTask())

	coord := NewTaskEditCoordinator(nav, tc)
	view := &mockValidatableEditView{
		mockTaskEditView: mockTaskEditView{
			title:       "",
			description: "desc",
		},
		valid:  false,
		errors: []string{"title is required", "priority must be between 1 and 5"},
	}

	got := coord.commit(view)
	if got {
		t.Fatal("commit() should return false when IsValid() returns false")
	}

	msg, level, _ := sl.GetMessage()
	want := "title is required; priority must be between 1 and 5"
	if msg != want {
		t.Errorf("message = %q, want %q", msg, want)
	}
	if level != model.MessageLevelError {
		t.Errorf("level = %q, want %q", level, model.MessageLevelError)
	}
}

func TestTaskEditCoordinator_Commit_ValidationFailsNilStatusline(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	nav := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, nav, nil)
	tc.SetDraft(newTestTask())

	coord := NewTaskEditCoordinator(nav, tc)
	view := &mockValidatableEditView{
		mockTaskEditView: mockTaskEditView{
			title:       "",
			description: "desc",
		},
		valid:  false,
		errors: []string{"title is required"},
	}

	// should not panic
	got := coord.commit(view)
	if got {
		t.Error("commit() should return false when IsValid() returns false")
	}
}

// --- field hint tests ---

func TestTaskEditCoordinator_FieldHint_RecurrencePatternFocused(t *testing.T) {
	sl := model.NewStatuslineConfig()
	nav := newMockNavigationController()
	s := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(s)
	tc := NewTaskController(s, gate, nav, sl)

	coord := NewTaskEditCoordinator(nav, tc)
	view := &mockFieldFocusableView{focusedField: model.EditFieldRecurrence, valueFocused: false}

	coord.updateFieldHint(view)

	msg, level, _ := sl.GetMessage()
	if msg != "\u2191\u2193 change pattern  \u2192 edit value" {
		t.Errorf("message = %q, want pattern hint", msg)
	}
	if level != model.MessageLevelInfo {
		t.Errorf("level = %q, want %q", level, model.MessageLevelInfo)
	}
}

func TestTaskEditCoordinator_FieldHint_RecurrenceValueFocused(t *testing.T) {
	sl := model.NewStatuslineConfig()
	nav := newMockNavigationController()
	s := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(s)
	tc := NewTaskController(s, gate, nav, sl)

	coord := NewTaskEditCoordinator(nav, tc)
	view := &mockFieldFocusableView{focusedField: model.EditFieldRecurrence, valueFocused: true}

	coord.updateFieldHint(view)

	msg, level, _ := sl.GetMessage()
	if msg != "\u2190 edit pattern  \u2191\u2193 change value" {
		t.Errorf("message = %q, want value hint", msg)
	}
	if level != model.MessageLevelInfo {
		t.Errorf("level = %q, want %q", level, model.MessageLevelInfo)
	}
}

func TestTaskEditCoordinator_FieldHint_NonRecurrenceClearsHint(t *testing.T) {
	sl := model.NewStatuslineConfig()
	nav := newMockNavigationController()
	s := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(s)
	tc := NewTaskController(s, gate, nav, sl)

	coord := NewTaskEditCoordinator(nav, tc)

	// set a hint first
	sl.SetMessage("some hint", model.MessageLevelInfo, false)

	view := &mockFieldFocusableView{focusedField: model.EditFieldTitle}
	coord.updateFieldHint(view)

	msg, _, _ := sl.GetMessage()
	if msg != "" {
		t.Errorf("message = %q, want empty after focusing non-recurrence field", msg)
	}
}

func TestTaskEditCoordinator_FieldHint_FocusNextSetsHint(t *testing.T) {
	sl := model.NewStatuslineConfig()
	nav := newMockNavigationController()
	s := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(s)
	tc := NewTaskController(s, gate, nav, sl)

	coord := NewTaskEditCoordinator(nav, tc)
	// Due is right before Recurrence in navigation order
	view := &mockFieldFocusableView{focusedField: model.EditFieldDue}

	coord.FocusNextField(view)

	if view.focusedField != model.EditFieldRecurrence {
		t.Fatalf("expected recurrence, got %q", view.focusedField)
	}
	msg, _, _ := sl.GetMessage()
	if msg != "\u2191\u2193 change pattern  \u2192 edit value" {
		t.Errorf("message = %q, want hint after FocusNextField to recurrence", msg)
	}
}

func TestTaskEditCoordinator_FieldHint_CancelClearsHint(t *testing.T) {
	sl := model.NewStatuslineConfig()
	nav := newMockNavigationController()
	s := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(s)
	tc := NewTaskController(s, gate, nav, sl)

	coord := NewTaskEditCoordinator(nav, tc)
	sl.SetMessage("some hint", model.MessageLevelInfo, false)

	coord.CancelAndClose()

	msg, _, _ := sl.GetMessage()
	if msg != "" {
		t.Errorf("message = %q, want empty after CancelAndClose", msg)
	}
}

func TestTaskEditCoordinator_FieldHint_NilStatuslineNoOp(t *testing.T) {
	s := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(s)
	nav := newMockNavigationController()
	tc := NewTaskController(s, gate, nav, nil)

	coord := NewTaskEditCoordinator(nav, tc)
	view := &mockFieldFocusableView{focusedField: model.EditFieldRecurrence}

	// should not panic
	coord.updateFieldHint(view)
	coord.clearFieldHint()
}

func TestTaskEditCoordinator_Commit_ErrorDisplaysStatusline(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	gate.OnUpdate(func(_, _ *task.Task, _ []*task.Task) *service.Rejection {
		return &service.Rejection{Reason: "blocked by trigger"}
	})

	sl := model.NewStatuslineConfig()
	nav := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, nav, sl)

	original := newTestTask()
	_ = taskStore.CreateTask(original)
	tc.StartEditSession(original.ID)

	coord := NewTaskEditCoordinator(nav, tc)
	view := &mockTaskEditView{
		title:       original.Title,
		description: "updated desc",
		tags:        nil,
	}

	got := coord.commit(view)
	if got {
		t.Fatal("commit() should return false when update is rejected")
	}

	msg, level, _ := sl.GetMessage()
	if msg == "" {
		t.Fatal("statusline message should be set on commit error")
	}
	if level != model.MessageLevelError {
		t.Errorf("message level = %q, want %q", level, model.MessageLevelError)
	}
	if msg != "blocked by trigger" {
		t.Errorf("message = %q, want %q", msg, "blocked by trigger")
	}
}

func TestTaskEditCoordinator_Commit_ErrorNilStatuslineNoOp(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	gate.OnUpdate(func(_, _ *task.Task, _ []*task.Task) *service.Rejection {
		return &service.Rejection{Reason: "blocked"}
	})

	nav := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, nav, nil) // nil statusline

	original := newTestTask()
	_ = taskStore.CreateTask(original)
	tc.StartEditSession(original.ID)

	coord := NewTaskEditCoordinator(nav, tc)
	view := &mockTaskEditView{
		title:       original.Title,
		description: "updated desc",
		tags:        nil,
	}

	// should not panic
	got := coord.commit(view)
	if got {
		t.Error("commit() should return false when update is rejected")
	}
}

func TestTaskEditCoordinator_Commit_MultipleRejectionsDisplayCleanly(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	gate.OnUpdate(func(_, _ *task.Task, _ []*task.Task) *service.Rejection {
		return &service.Rejection{Reason: "WIP limit reached"}
	})
	gate.OnUpdate(func(_, _ *task.Task, _ []*task.Task) *service.Rejection {
		return &service.Rejection{Reason: "blocked by freeze"}
	})

	sl := model.NewStatuslineConfig()
	nav := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, nav, sl)

	original := newTestTask()
	_ = taskStore.CreateTask(original)
	tc.StartEditSession(original.ID)

	coord := NewTaskEditCoordinator(nav, tc)
	view := &mockTaskEditView{
		title:       original.Title,
		description: "updated desc",
		tags:        nil,
	}

	got := coord.commit(view)
	if got {
		t.Fatal("commit() should return false when update is rejected")
	}

	msg, level, _ := sl.GetMessage()
	want := "WIP limit reached; blocked by freeze"
	if msg != want {
		t.Errorf("message = %q, want %q", msg, want)
	}
	if level != model.MessageLevelError {
		t.Errorf("level = %q, want %q", level, model.MessageLevelError)
	}
}
