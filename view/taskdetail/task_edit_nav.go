package taskdetail

import (
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"

	"github.com/gdamore/tcell/v2"
)

// This file contains edit mode navigation and field management methods for TaskEditView.

// IsValid returns true if the task passes all validation checks
func (ev *TaskEditView) IsValid() bool {
	return len(ev.validationErrors) == 0
}

// SetFocusedField changes the focused field and re-renders
func (ev *TaskEditView) SetFocusedField(field model.EditField) {
	ev.focusedField = field
	ev.UpdateHeaderForField(field)

	// Refresh rebuilds the layout, which may temporarily lose focus
	ev.refresh()

	// Immediately restore focus to the appropriate widget for the focused field
	// This must happen after refresh() to ensure widget is in the view hierarchy
	if ev.focusSetter == nil {
		return
	}

	switch field {
	case model.EditFieldStatus:
		if ev.statusSelectList != nil {
			// Focus the wrapper which has custom InputHandler
			ev.focusSetter(ev.statusSelectList)
		}
	case model.EditFieldType:
		if ev.typeSelectList != nil {
			// Focus the wrapper which has custom InputHandler
			ev.focusSetter(ev.typeSelectList)
		}
	case model.EditFieldPriority:
		if ev.priorityInput != nil {
			// Focus the wrapper which has custom InputHandler
			ev.focusSetter(ev.priorityInput)
		}
	case model.EditFieldAssignee:
		if ev.assigneeSelectList != nil {
			// Focus the wrapper which has custom InputHandler
			ev.focusSetter(ev.assigneeSelectList)
		}
	case model.EditFieldPoints:
		if ev.pointsInput != nil {
			// Focus the wrapper which has custom InputHandler
			ev.focusSetter(ev.pointsInput)
		}
	case model.EditFieldTitle:
		if ev.titleInput != nil {
			ev.focusSetter(ev.titleInput)
		}
	case model.EditFieldDescription:
		if ev.descTextArea != nil {
			ev.focusSetter(ev.descTextArea)
		}
	}
}

// GetFocusedField returns the currently focused field
func (ev *TaskEditView) GetFocusedField() model.EditField {
	return ev.focusedField
}

// IsEditFieldFocused returns whether any editable field has tview focus
func (ev *TaskEditView) IsEditFieldFocused() bool {
	if ev.statusSelectList != nil && ev.statusSelectList.HasFocus() {
		return true
	}
	if ev.typeSelectList != nil && ev.typeSelectList.HasFocus() {
		return true
	}
	if ev.assigneeSelectList != nil && ev.assigneeSelectList.HasFocus() {
		return true
	}
	if ev.priorityInput != nil && ev.priorityInput.HasFocus() {
		return true
	}
	if ev.pointsInput != nil && ev.pointsInput.HasFocus() {
		return true
	}
	return false
}

// FocusNextField advances to the next field in edit order
func (ev *TaskEditView) FocusNextField() bool {
	nextField := model.NextField(ev.focusedField)
	ev.SetFocusedField(nextField)
	return true
}

// FocusPrevField moves to the previous field in edit order
func (ev *TaskEditView) FocusPrevField() bool {
	prevField := model.PrevField(ev.focusedField)
	ev.SetFocusedField(prevField)
	return true
}

// CycleFieldValueUp cycles the currently focused field's value upward (previous)
func (ev *TaskEditView) CycleFieldValueUp() bool {
	switch ev.focusedField {
	case model.EditFieldStatus:
		if ev.statusSelectList != nil {
			ev.statusSelectList.MoveToPrevious()
			return true
		}
	case model.EditFieldType:
		if ev.typeSelectList != nil {
			ev.typeSelectList.MoveToPrevious()
			return true
		}
	case model.EditFieldPriority:
		if ev.priorityInput != nil {
			ev.priorityInput.InputHandler()(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), nil)
			return true
		}
	case model.EditFieldAssignee:
		if ev.assigneeSelectList != nil {
			ev.assigneeSelectList.MoveToPrevious()
			return true
		}
	case model.EditFieldPoints:
		if ev.pointsInput != nil {
			ev.pointsInput.InputHandler()(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), nil)
			return true
		}
	}
	return false
}

// CycleFieldValueDown cycles the currently focused field's value downward (next)
func (ev *TaskEditView) CycleFieldValueDown() bool {
	switch ev.focusedField {
	case model.EditFieldStatus:
		if ev.statusSelectList != nil {
			ev.statusSelectList.MoveToNext()
			return true
		}
	case model.EditFieldType:
		if ev.typeSelectList != nil {
			ev.typeSelectList.MoveToNext()
			return true
		}
	case model.EditFieldPriority:
		if ev.priorityInput != nil {
			ev.priorityInput.InputHandler()(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), nil)
			return true
		}
	case model.EditFieldAssignee:
		if ev.assigneeSelectList != nil {
			ev.assigneeSelectList.MoveToNext()
			return true
		}
	case model.EditFieldPoints:
		if ev.pointsInput != nil {
			ev.pointsInput.InputHandler()(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), nil)
			return true
		}
	}
	return false
}

// UpdateHeaderForField updates the registry with field-specific actions
func (ev *TaskEditView) UpdateHeaderForField(field model.EditField) {
	ev.registry = controller.GetActionsForField(field)
}
