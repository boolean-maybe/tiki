package tikidetail

import (
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

// tiki_edit_nav.go contains edit-mode navigation and field management
// methods for TikiEditView. After the grid migration these dispatch through
// ev.editors (a unified FieldEditorWidget map keyed by canonical field
// name) instead of the per-typed-widget switch tables that used to live
// here. Title and Description retain dedicated paths because they live
// outside the metadata grid.

// IsValid returns true if the tiki passes all validation checks.
func (ev *TikiEditView) IsValid() bool {
	return len(ev.validationErrors) == 0
}

// ValidationErrors returns the current list of validation error messages.
func (ev *TikiEditView) ValidationErrors() []string {
	return ev.validationErrors
}

// SetFocusedField changes the focused field, refreshes the layout (so the
// focused field's read-only render switches to its editor primitive), and
// hands focus to the matching widget.
func (ev *TikiEditView) SetFocusedField(field model.EditField) {
	ev.focusedField = field
	ev.UpdateHeaderForField(field)

	ev.refresh()

	if ev.focusSetter == nil {
		return
	}

	switch field {
	case model.EditFieldTitle:
		if ev.titleInput != nil {
			ev.focusSetter(ev.titleInput)
		}
		return
	case model.EditFieldDescription:
		if ev.descTextArea != nil {
			ev.focusSetter(ev.descTextArea)
		}
		return
	}

	name := editFieldToFieldName(field)
	if name == "" {
		return
	}
	if w, ok := ev.editors[name]; ok && w != nil {
		ev.focusSetter(w)
	}
}

// editFieldToFieldName maps an EditField enum to its canonical metadata
// field name. Title and Description return "" — they're handled by the
// dedicated branch in SetFocusedField above.
//
// Workflow-declared TypeEnum fields use their field name as the EditField
// identity (see model.MetadataToEditFieldOrder); for those the EditField
// string IS the field name, so we resolve via the workflow catalog when
// none of the built-in cases match. Without this fallback, custom enums
// like severity would be unreachable for keyboard editing — the focus,
// cycling, and validation paths would all return "" and bail out.
func editFieldToFieldName(field model.EditField) string {
	switch field {
	case model.EditFieldStatus:
		return tikipkg.FieldStatus
	case model.EditFieldType:
		return tikipkg.FieldType
	case model.EditFieldPriority:
		return tikipkg.FieldPriority
	case model.EditFieldPoints:
		return tikipkg.FieldPoints
	case model.EditFieldAssignee:
		return tikipkg.FieldAssignee
	case model.EditFieldDue:
		return tikipkg.FieldDue
	case model.EditFieldRecurrence:
		return tikipkg.FieldRecurrence
	case model.EditFieldTags:
		return tikipkg.FieldTags
	}
	// Workflow-only enum fields: EditField string equals the field name.
	// Confirm the catalog has it as TypeEnum to avoid masking unrelated
	// EditField extensions.
	name := string(field)
	if name == "" {
		return ""
	}
	if wfd, ok := workflow.Field(name); ok && wfd.Type == workflow.TypeEnum {
		return name
	}
	return ""
}

// GetFocusedField returns the currently focused field.
func (ev *TikiEditView) GetFocusedField() model.EditField {
	return ev.focusedField
}

// IsEditFieldFocused returns whether any editable widget has tview focus.
// Aggregates title, description, and every cached metadata editor.
func (ev *TikiEditView) IsEditFieldFocused() bool {
	if ev.titleInput != nil && ev.titleInput.HasFocus() {
		return true
	}
	if ev.descTextArea != nil && ev.descTextArea.HasFocus() {
		return true
	}
	for _, w := range ev.editors {
		if w != nil && w.HasFocus() {
			return true
		}
	}
	return false
}

// FocusNextField advances to the next field in the per-instance edit order.
func (ev *TikiEditView) FocusNextField() bool {
	if ev.descOnly || ev.tagsOnly {
		return false
	}
	nextField := model.NextFieldInOrder(ev.focusedField, ev.editFieldOrder, ev.shouldSkipField)
	ev.SetFocusedField(nextField)
	return true
}

// FocusPrevField moves to the previous field in the per-instance edit order.
func (ev *TikiEditView) FocusPrevField() bool {
	if ev.descOnly || ev.tagsOnly {
		return false
	}
	prevField := model.PrevFieldInOrder(ev.focusedField, ev.editFieldOrder, ev.shouldSkipField)
	ev.SetFocusedField(prevField)
	return true
}

// shouldSkipField returns true for fields that should be skipped during
// navigation. Read-only descriptors are already filtered out by
// MetadataToEditFieldOrder; this skip predicate covers dynamic state like
// Due being read-only when recurrence is set.
func (ev *TikiEditView) shouldSkipField(field model.EditField) bool {
	return field == model.EditFieldDue && ev.isDueReadOnly()
}

// CycleFieldValueUp cycles the currently focused field's value upward (-1).
func (ev *TikiEditView) CycleFieldValueUp() bool {
	return ev.cycleFocusedField(-1)
}

// CycleFieldValueDown cycles the currently focused field's value downward (+1).
func (ev *TikiEditView) CycleFieldValueDown() bool {
	return ev.cycleFocusedField(+1)
}

// cycleFocusedField dispatches a CycleValue call to the focused metadata
// editor. Returns false when the focused field has no cyclable widget
// (e.g. tags, title, description) or when the widget refuses to cycle
// (e.g. due in recurrence-driven read-only mode).
func (ev *TikiEditView) cycleFocusedField(direction int) bool {
	name := editFieldToFieldName(ev.focusedField)
	if name == "" {
		return false
	}
	w, ok := ev.editors[name]
	if !ok || w == nil {
		return false
	}
	return w.CycleValue(direction)
}

// MoveRecurrencePartLeft moves the recurrence editor to the frequency part.
func (ev *TikiEditView) MoveRecurrencePartLeft() bool {
	if ev.focusedField != model.EditFieldRecurrence {
		return false
	}
	w, ok := ev.editors[tikipkg.FieldRecurrence]
	if !ok || w == nil {
		return false
	}
	if re, ok := w.(*recurrenceEditAdapter); ok {
		re.MovePartLeft()
		return true
	}
	return false
}

// MoveRecurrencePartRight moves the recurrence editor to the value part.
func (ev *TikiEditView) MoveRecurrencePartRight() bool {
	if ev.focusedField != model.EditFieldRecurrence {
		return false
	}
	w, ok := ev.editors[tikipkg.FieldRecurrence]
	if !ok || w == nil {
		return false
	}
	if re, ok := w.(*recurrenceEditAdapter); ok {
		re.MovePartRight()
		return true
	}
	return false
}

// IsRecurrenceValueFocused returns true when the recurrence field's value
// part is active.
func (ev *TikiEditView) IsRecurrenceValueFocused() bool {
	if ev.focusedField != model.EditFieldRecurrence {
		return false
	}
	w, ok := ev.editors[tikipkg.FieldRecurrence]
	if !ok || w == nil {
		return false
	}
	if re, ok := w.(*recurrenceEditAdapter); ok {
		return re.IsValueFocused()
	}
	return false
}

// UpdateHeaderForField updates the registry with field-specific actions.
func (ev *TikiEditView) UpdateHeaderForField(field model.EditField) {
	if ev.descOnly {
		ev.registry = controller.DescOnlyEditActions()
	} else if ev.tagsOnly {
		ev.registry = controller.TagsOnlyEditActions()
	} else {
		ev.registry = controller.GetActionsForField(field)
	}
	if ev.actionChangeHandler != nil {
		ev.actionChangeHandler()
	}
}
