package model

// EditField identifies an editable field in task edit mode
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
	EditFieldDescription EditField = "description"
)

// fieldOrder defines the navigation sequence for edit fields
var fieldOrder = []EditField{
	EditFieldTitle,
	EditFieldStatus,
	EditFieldType,
	EditFieldPriority,
	EditFieldPoints,
	EditFieldAssignee,
	EditFieldDue,
	EditFieldRecurrence,
	EditFieldDescription,
}

// noSkip is a predicate that skips nothing, used by NextField/PrevField.
var noSkip = func(EditField) bool { return false }

// NextField returns the next field in the edit cycle (stops at last field, no wrapping).
func NextField(current EditField) EditField {
	return NextFieldSkipping(current, noSkip)
}

// PrevField returns the previous field in the edit cycle (stops at first field, no wrapping).
func PrevField(current EditField) EditField {
	return PrevFieldSkipping(current, noSkip)
}

// NextFieldSkipping returns the next field, skipping fields where skip returns true.
func NextFieldSkipping(current EditField, skip func(EditField) bool) EditField {
	for i, field := range fieldOrder {
		if field == current {
			for j := i + 1; j < len(fieldOrder); j++ {
				if !skip(fieldOrder[j]) {
					return fieldOrder[j]
				}
			}
			return current
		}
	}
	return EditFieldTitle
}

// PrevFieldSkipping returns the previous field, skipping fields where skip returns true.
func PrevFieldSkipping(current EditField, skip func(EditField) bool) EditField {
	for i, field := range fieldOrder {
		if field == current {
			for j := i - 1; j >= 0; j-- {
				if !skip(fieldOrder[j]) {
					return fieldOrder[j]
				}
			}
			return current
		}
	}
	return EditFieldTitle
}

// IsEditableField returns true if the field can be edited (not just viewed)
func IsEditableField(field EditField) bool {
	switch field {
	case EditFieldTitle, EditFieldPriority, EditFieldAssignee, EditFieldPoints, EditFieldDue, EditFieldRecurrence, EditFieldDescription:
		return true
	default:
		// Status is read-only for now
		return false
	}
}

// FieldLabel returns a human-readable label for the field
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
	case EditFieldDescription:
		return "Description"
	default:
		return string(field)
	}
}
