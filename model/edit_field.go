package model

// EditField identifies an editable field in tiki edit mode.
type EditField string

const (
	EditFieldTitle       EditField = "title"
	EditFieldDescription EditField = "description"
)
