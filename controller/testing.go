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

// newTestTiki creates a workflow tiki with default test values.
func newTestTiki() *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.ID = "000001"
	tk.Title = "Test Tiki"
	tk.Set(tikipkg.FieldStatus, "ready")
	tk.Set(tikipkg.FieldType, "story")
	tk.Set(tikipkg.FieldPriority, "medium")
	tk.Set(tikipkg.FieldPoints, 5)
	return tk
}

// newTestTikiWithID creates a workflow tiki with ID "DRAFT1".
func newTestTikiWithID() *tikipkg.Tiki {
	tk := newTestTiki()
	tk.ID = "DRAFT1"
	return tk
}
