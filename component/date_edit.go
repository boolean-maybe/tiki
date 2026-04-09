package component

import (
	"time"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DateEdit is a one-line input field for date values in YYYY-MM-DD format.
// It supports both:
// 1. Direct typing with validation (digits and hyphens only)
// 2. Arrow key navigation to increment/decrement by one day
//
// Arrow keys on an empty field start from tomorrow.
// The change handler is called immediately on every valid change.
type DateEdit struct {
	*tview.InputField
	currentText string
	onChange    func(string)
	clearOnType bool
}

// NewDateEdit creates a new date input field.
func NewDateEdit() *DateEdit {
	inputField := tview.NewInputField()
	colors := config.GetColors()
	inputField.SetFieldBackgroundColor(colors.ContentBackgroundColor)
	inputField.SetFieldTextColor(colors.ContentTextColor)

	de := &DateEdit{
		InputField: inputField,
	}

	inputField.SetFocusFunc(func() {
		de.clearOnType = true
	})

	return de
}

// SetChangeHandler sets the callback function that is called whenever the value changes.
func (de *DateEdit) SetChangeHandler(handler func(string)) *DateEdit {
	de.onChange = handler
	return de
}

// SetLabel sets the label displayed before the input field.
func (de *DateEdit) SetLabel(label string) *DateEdit {
	de.InputField.SetLabel(label)
	return de
}

// SetInitialValue sets the current date text. Empty string is valid (no due date).
func (de *DateEdit) SetInitialValue(value string) *DateEdit {
	de.currentText = value
	de.SetText(value)
	return de
}

// GetCurrentText returns the last validated date text.
func (de *DateEdit) GetCurrentText() string {
	return de.currentText
}

// incrementDay adds one day to the current date. If empty, starts from tomorrow.
func (de *DateEdit) incrementDay() {
	base := de.parseOrTomorrow()
	newDate := base.AddDate(0, 0, 1)
	de.applyDate(newDate)
}

// decrementDay subtracts one day from the current date. If empty, starts from tomorrow.
func (de *DateEdit) decrementDay() {
	base := de.parseOrTomorrow()
	newDate := base.AddDate(0, 0, -1)
	de.applyDate(newDate)
}

func (de *DateEdit) parseOrTomorrow() time.Time {
	text := de.GetText()
	if parsed, ok := taskpkg.ParseDueDate(text); ok && !parsed.IsZero() {
		return parsed
	}
	return time.Now().AddDate(0, 0, 1)
}

func (de *DateEdit) applyDate(t time.Time) {
	formatted := t.Format(taskpkg.DateFormat)
	de.currentText = formatted
	de.SetText(formatted)
	if de.onChange != nil {
		de.onChange(formatted)
	}
}

// validateAndUpdate checks the current text against ParseDueDate.
// Valid+changed → update currentText and call onChange. Invalid → revert.
func (de *DateEdit) validateAndUpdate() {
	text := de.GetText()

	parsed, ok := taskpkg.ParseDueDate(text)
	if !ok {
		// invalid — revert to last valid text
		de.SetText(de.currentText)
		return
	}

	// valid date or empty
	var newText string
	if !parsed.IsZero() {
		newText = parsed.Format(taskpkg.DateFormat)
	}

	if newText != de.currentText {
		de.currentText = newText
		if de.onChange != nil {
			de.onChange(newText)
		}
	}
}

// InputHandler handles keyboard input for the date input field.
func (de *DateEdit) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	baseHandler := de.InputField.InputHandler()

	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()

		switch key {
		case tcell.KeyUp:
			de.clearOnType = false
			de.decrementDay()

		case tcell.KeyDown:
			de.clearOnType = false
			de.incrementDay()

		case tcell.KeyRune:
			ch := event.Rune()
			if (ch >= '0' && ch <= '9') || ch == '-' {
				if de.clearOnType {
					de.SetText("")
					de.clearOnType = false
				}
				if baseHandler != nil {
					baseHandler(event, setFocus)
				}
				de.validateAndUpdate()
			}
			// ignore other characters

		case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete:
			de.clearOnType = false
			de.currentText = ""
			de.SetText("")
			if de.onChange != nil {
				de.onChange("")
			}

		case tcell.KeyCtrlU:
			de.clearOnType = false
			de.currentText = ""
			de.SetText("")
			if de.onChange != nil {
				de.onChange("")
			}

		default:
			if baseHandler != nil {
				baseHandler(event, setFocus)
			}
		}
	}
}
