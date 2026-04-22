package controller

import (
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/task"

	"github.com/rivo/tview"
)

// Test utilities for controller unit tests

// newMockNavigationController creates a new mock navigation controller
func newMockNavigationController() *NavigationController {
	return &NavigationController{
		app:      nil, // unit tests don't need the tview.Application
		navState: newViewStack(),
	}
}

// mockSelectableView implements SelectableView for unit tests.
type mockSelectableView struct {
	selectedID string
}

func (m *mockSelectableView) GetPrimitive() tview.Primitive      { return nil }
func (m *mockSelectableView) GetActionRegistry() *ActionRegistry { return NewActionRegistry() }
func (m *mockSelectableView) GetViewID() model.ViewID            { return "" }
func (m *mockSelectableView) OnFocus()                           {}
func (m *mockSelectableView) OnBlur()                            {}
func (m *mockSelectableView) GetSelectedID() string              { return m.selectedID }
func (m *mockSelectableView) SetSelectedID(_ string)             {}

// Test fixtures

// newTestTask creates a test task with default values
func newTestTask() *task.Task {
	return &task.Task{
		ID:       "TIKI-1",
		Title:    "Test Task",
		Status:   task.StatusReady,
		Type:     task.TypeStory,
		Priority: 3,
		Points:   5,
	}
}

// newTestTaskWithID creates a test task with ID "DRAFT-1"
func newTestTaskWithID() *task.Task {
	t := newTestTask()
	t.ID = "DRAFT-1"
	return t
}
