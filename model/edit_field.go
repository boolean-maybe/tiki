package model

// EditField identifies an editable field in tiki edit mode.
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
