package controller

import (
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"

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

// mockNonEditView implements only View (not TaskEditView).
type mockNonEditView struct{}

func (m *mockNonEditView) GetPrimitive() tview.Primitive      { return nil }
func (m *mockNonEditView) GetActionRegistry() *ActionRegistry { return nil }
func (m *mockNonEditView) GetViewID() model.ViewID            { return "" }
func (m *mockNonEditView) OnFocus()                           {}
func (m *mockNonEditView) OnBlur()                            {}

// mockValidatableEditView adds IsValid() to mockTaskEditView.
type mockValidatableEditView struct {
	mockTaskEditView
	valid bool
}

func (m *mockValidatableEditView) IsValid() bool { return m.valid }

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
	nav := newMockNavigationController()
	tc := NewTaskController(taskStore, nav)
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
	nav := newMockNavigationController()
	tc := NewTaskController(taskStore, nav)

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
	nav := newMockNavigationController()
	tc := NewTaskController(store.NewInMemoryStore(), nav)
	coord := NewTaskEditCoordinator(nav, tc)

	got := coord.commit(&mockNonEditView{})
	if got {
		t.Error("commit() should return false for non-TaskEditView")
	}
}

func TestTaskEditCoordinator_Commit_ValidationFails(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	nav := newMockNavigationController()
	tc := NewTaskController(taskStore, nav)
	tc.SetDraft(newTestTask())

	coord := NewTaskEditCoordinator(nav, tc)
	view := &mockValidatableEditView{
		mockTaskEditView: mockTaskEditView{
			title:       "Valid Title",
			description: "desc",
			tags:        []string{"tag"},
		},
		valid: false,
	}

	got := coord.commit(view)
	if got {
		t.Error("commit() should return false when IsValid() returns false")
	}
}
