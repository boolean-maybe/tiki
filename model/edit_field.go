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
	EditFieldDescription EditField = "description"
)

// fieldOrder defines the navigation sequence for edit fields
var fieldOrder = []EditField{
	EditFieldTitle,
	EditFieldStatus,
	EditFieldType,
	EditFieldPriority,
	EditFieldAssignee,
	EditFieldPoints,
	EditFieldDescription,
}

// NextField returns the next field in the edit cycle (stops at last field, no wrapping)
func NextField(current EditField) EditField {
	for i, field := range fieldOrder {
		if field == current {
			// stop at last field instead of wrapping
			if i == len(fieldOrder)-1 {
				return current
			}
			return fieldOrder[i+1]
		}
	}
	// default to title if current field not found
	return EditFieldTitle
}

// PrevField returns the previous field in the edit cycle (stops at first field, no wrapping)
func PrevField(current EditField) EditField {
	for i, field := range fieldOrder {
		if field == current {
			// stop at first field instead of wrapping
			if i == 0 {
				return current
			}
			return fieldOrder[i-1]
		}
	}
	// default to title if current field not found
	return EditFieldTitle
}

// IsEditableField returns true if the field can be edited (not just viewed)
func IsEditableField(field EditField) bool {
	switch field {
	case EditFieldTitle, EditFieldPriority, EditFieldAssignee, EditFieldPoints, EditFieldDescription:
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
	case EditFieldDescription:
		return "Description"
	default:
		return string(field)
	}
}
