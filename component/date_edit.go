package component

import (
	"time"

	"github.com/boolean-maybe/tiki/workflow/value"
)

// DateEdit is a segmented editor for date-only values in "YYYY-MM-DD" format.
// It is a thin wrapper over segmentedTimeEdit exposing the year, month, and day
// segments, so its date-part behaviour is identical to DateTimeEdit's:
// Left/Right selects a segment, Up/Down cycles the active segment (wrapping
// within its bounds; the day clamps to the month's maximum), typing overwrites
// the active segment and auto-advances, an empty field seeds from today on the
// first arrow/digit, and Backspace/Ctrl-U clears back to empty.
type DateEdit struct {
	*segmentedTimeEdit
}

// NewDateEdit creates a new segmented date editor.
func NewDateEdit() *DateEdit {
	core := newSegmentedTimeEdit(
		[]segmentKind{segYear, segMonth, segDay},
		formatDateOnly,
	)
	core.emptyPlaceholderText = "None"
	return &DateEdit{segmentedTimeEdit: core}
}

// formatDateOnly renders a time.Time as YYYY-MM-DD, or "" when zero.
func formatDateOnly(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(value.DateFormat)
}

// SetChangeHandler sets the callback invoked on every accepted change.
func (de *DateEdit) SetChangeHandler(handler func(string)) *DateEdit {
	de.onChange = handler
	return de
}

// SetLabel stores the focus-marker label for Draw to paint.
func (de *DateEdit) SetLabel(label string) *DateEdit {
	de.setLabel(label)
	return de
}

// SetInitialValue seeds the editor from a canonical date string. Empty or
// invalid input leaves the editor in the empty state.
func (de *DateEdit) SetInitialValue(text string) *DateEdit {
	parsed, ok := value.ParseDate(text)
	de.setValue(parsed, ok && !parsed.IsZero())
	return de
}

// GetCurrentText returns the canonical formatted date, or "" when empty.
func (de *DateEdit) GetCurrentText() string {
	return de.currentText()
}
