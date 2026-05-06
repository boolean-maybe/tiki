package model

import (
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// EditField identifies an editable field in task edit mode.
type EditField string

const (
	EditFieldTitle       EditField = "title"
	EditFieldStatus      EditField = "status"
	EditFieldType        EditField = "type"
	EditFieldPriority    EditField = "priority"
	EditFieldAssignee    EditField = "assignee"
	EditFieldPoints      EditField = "points"
	EditFieldDue         EditField = "due"
	EditFieldRecurrence  EditField = "recurrence"
	EditFieldTags        EditField = "tags"
	EditFieldDescription EditField = "description"
)

// defaultFieldOrder is the legacy navigation sequence for edit fields. It is
// retained as the default order used by the wrapper helpers (NextField,
// PrevField, NextFieldSkipping, PrevFieldSkipping) when callers have not
// provided a per-instance order. Tags joins the order between Recurrence
// and Description so the default 9-item view → 10-item view migration
// preserves the bracketed layout.
var defaultFieldOrder = []EditField{
	EditFieldTitle,
	EditFieldStatus,
	EditFieldType,
	EditFieldPriority,
	EditFieldPoints,
	EditFieldAssignee,
	EditFieldDue,
	EditFieldRecurrence,
	EditFieldTags,
	EditFieldDescription,
}

// noSkip is a predicate that skips nothing.
var noSkip = func(EditField) bool { return false }

// NextField returns the next field in the default cycle (no wrapping).
func NextField(current EditField) EditField {
	return NextFieldInOrder(current, defaultFieldOrder, noSkip)
}

// PrevField returns the previous field in the default cycle (no wrapping).
func PrevField(current EditField) EditField {
	return PrevFieldInOrder(current, defaultFieldOrder, noSkip)
}

// NextFieldSkipping returns the next field in the default order, skipping
// fields where skip returns true.
func NextFieldSkipping(current EditField, skip func(EditField) bool) EditField {
	return NextFieldInOrder(current, defaultFieldOrder, skip)
}

// PrevFieldSkipping returns the previous field in the default order,
// skipping fields where skip returns true.
func PrevFieldSkipping(current EditField, skip func(EditField) bool) EditField {
	return PrevFieldInOrder(current, defaultFieldOrder, skip)
}

// NextFieldInOrder returns the next field in `order` after `current`,
// skipping any field where `skip` returns true. Stops at the last field
// (no wrapping). Returns `current` when there is no later non-skipped
// field. Falls back to the first field of `order` when `current` is not
// in the slice.
func NextFieldInOrder(current EditField, order []EditField, skip func(EditField) bool) EditField {
	if skip == nil {
		skip = noSkip
	}
	for i, field := range order {
		if field == current {
			for j := i + 1; j < len(order); j++ {
				if !skip(order[j]) {
					return order[j]
				}
			}
			return current
		}
	}
	if len(order) == 0 {
		return current
	}
	return order[0]
}

// PrevFieldInOrder returns the previous field in `order` before `current`,
// skipping any field where `skip` returns true. Stops at the first field
// (no wrapping).
func PrevFieldInOrder(current EditField, order []EditField, skip func(EditField) bool) EditField {
	if skip == nil {
		skip = noSkip
	}
	for i, field := range order {
		if field == current {
			for j := i - 1; j >= 0; j-- {
				if !skip(order[j]) {
					return order[j]
				}
			}
			return current
		}
	}
	if len(order) == 0 {
		return current
	}
	return order[0]
}

// IsEditableField returns true if the field can be edited (not just viewed).
func IsEditableField(field EditField) bool {
	switch field {
	case EditFieldTitle, EditFieldStatus, EditFieldType, EditFieldPriority,
		EditFieldAssignee, EditFieldPoints, EditFieldDue, EditFieldRecurrence,
		EditFieldTags, EditFieldDescription:
		return true
	default:
		return false
	}
}

// FieldLabel returns a human-readable label for the field.
func FieldLabel(field EditField) string {
	switch field {
	case EditFieldTitle:
		return "Title"
	case EditFieldStatus:
		return "Status"
	case EditFieldType:
		return "Type"
	case EditFieldPriority:
		return "Priority"
	case EditFieldAssignee:
		return "Assignee"
	case EditFieldPoints:
		return "Story Points"
	case EditFieldDue:
		return "Due"
	case EditFieldRecurrence:
		return "Recurrence"
	case EditFieldTags:
		return "Tags"
	case EditFieldDescription:
		return "Description"
	default:
		return string(field)
	}
}

// MetadataToEditFieldOrder maps each canonical metadata field name to its
// EditField enum value, preserving order. Names without a matching EditField
// (e.g. createdBy/createdAt/updatedAt — read-only descriptors) are skipped
// so the returned slice is the metadata-only portion of the navigation
// sequence (callers bracket it with Title and Description).
func MetadataToEditFieldOrder(metadata []string) []EditField {
	result := make([]EditField, 0, len(metadata))
	for _, name := range metadata {
		switch name {
		case tikipkg.FieldStatus:
			result = append(result, EditFieldStatus)
		case tikipkg.FieldType:
			result = append(result, EditFieldType)
		case tikipkg.FieldPriority:
			result = append(result, EditFieldPriority)
		case tikipkg.FieldPoints:
			result = append(result, EditFieldPoints)
		case tikipkg.FieldAssignee:
			result = append(result, EditFieldAssignee)
		case tikipkg.FieldDue:
			result = append(result, EditFieldDue)
		case tikipkg.FieldRecurrence:
			result = append(result, EditFieldRecurrence)
		case tikipkg.FieldTags:
			result = append(result, EditFieldTags)
		}
	}
	return result
}
