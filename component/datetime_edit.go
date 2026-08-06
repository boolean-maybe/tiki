package component

import (
	"github.com/boolean-maybe/tiki/workflow/value"
)

// DateTimeEdit is a segmented editor for datetime values in "YYYY-MM-DD HH:MM"
// format. It is a thin wrapper over segmentedTimeEdit exposing the year, month,
// day, hour, and minute segments. See segmentedTimeEdit for the interaction
// model (Left/Right selects a segment, Up/Down cycles it, typing overwrites,
// day clamps to month max, empty seeds from now, Backspace clears).
type DateTimeEdit struct {
	*segmentedTimeEdit
}

// NewDateTimeEdit creates a new segmented datetime editor.
func NewDateTimeEdit() *DateTimeEdit {
	core := newSegmentedTimeEdit(
		[]segmentKind{segYear, segMonth, segDay, segHour, segMinute},
		value.FormatDateTime,
	)
	core.emptyPlaceholderText = "Unknown"
	return &DateTimeEdit{segmentedTimeEdit: core}
}

// SetChangeHandler sets the callback invoked on every accepted change.
func (de *DateTimeEdit) SetChangeHandler(handler func(string)) *DateTimeEdit {
	de.onChange = handler
	return de
}

// SetLabel stores the focus-marker label for Draw to paint.
func (de *DateTimeEdit) SetLabel(label string) *DateTimeEdit {
	de.setLabel(label)
	return de
}

// SetInitialValue seeds the editor from a canonical datetime string. Empty or
// invalid input leaves the editor in the empty state.
func (de *DateTimeEdit) SetInitialValue(text string) *DateTimeEdit {
	parsed, ok := value.ParseDateTime(text)
	de.setValue(parsed, ok && !parsed.IsZero())
	return de
}

// GetCurrentText returns the canonical formatted value, or "" when empty.
func (de *DateTimeEdit) GetCurrentText() string {
	return de.currentText()
}
