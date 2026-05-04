package controller

import (
	"github.com/boolean-maybe/tiki/model"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

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

// newTestTask creates a workflow tiki with default test values.
func newTestTask() *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.ID = "000001"
	tk.Title = "Test Task"
	tk.Set(tikipkg.FieldStatus, "ready")
	tk.Set(tikipkg.FieldType, "story")
	tk.Set(tikipkg.FieldPriority, 3)
	tk.Set(tikipkg.FieldPoints, 5)
	return tk
}

// newTestTaskWithID creates a workflow tiki with ID "DRAFT1".
func newTestTaskWithID() *tikipkg.Tiki {
	tk := newTestTask()
	tk.ID = "DRAFT1"
	return tk
}

// newTestTiki creates a test tiki with default values (matches newTestTask).
func newTestTiki() *tikipkg.Tiki {
	return newTestTask()
}

// newTestTikiWithID creates a test tiki with ID "DRAFT1".
func newTestTikiWithID() *tikipkg.Tiki {
	return newTestTaskWithID()
}
