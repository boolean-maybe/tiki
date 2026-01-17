package component

import (
	"github.com/boolean-maybe/tiki/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// EditSelectList is a one-line input field that allows both:
// 1. Free-form text entry (if allowTyping is true)
// 2. Arrow key navigation through a predefined list of values
//
// When the user presses up/down arrow keys, the previous/next value
// from the configured list is selected. If allowTyping is true, the user
// can also type any value; if false, only arrow keys work.
// The submit handler is called immediately on every change (typing or arrow navigation).
type EditSelectList struct {
	*tview.InputField
	values       []string
	currentIndex int // -1 means user is typing freely
	onSubmit     func(text string)
	allowTyping  bool // Controls whether direct typing is allowed
}

// NewEditSelectList creates a new edit/select list with the given values.
// If allowTyping is true, users can type freely; if false, only arrow keys work.
func NewEditSelectList(values []string, allowTyping bool) *EditSelectList {
	inputField := tview.NewInputField()

	// Configure the input field
	inputField.SetFieldBackgroundColor(config.GetContentBackgroundColor())
	inputField.SetFieldTextColor(config.GetContentTextColor())

	esl := &EditSelectList{
		InputField:   inputField,
		values:       values,
		currentIndex: -1, // Start with no selection
		allowTyping:  allowTyping,
	}

	return esl
}

// SetSubmitHandler sets the callback for when Enter is pressed.
func (esl *EditSelectList) SetSubmitHandler(handler func(text string)) *EditSelectList {
	esl.onSubmit = handler
	return esl
}

// SetLabel sets the label displayed before the input field.
func (esl *EditSelectList) SetLabel(label string) *EditSelectList {
	esl.InputField.SetLabel(label)
	return esl
}

// SetText sets the current text and resets the index to -1 (free-form mode).
func (esl *EditSelectList) SetText(text string) *EditSelectList {
	esl.InputField.SetText(text)
	esl.currentIndex = -1
	return esl
}

// Clear clears the input text and resets the index.
func (esl *EditSelectList) Clear() *EditSelectList {
	esl.SetText("")
	esl.currentIndex = -1
	return esl
}

// SetInitialValue sets the text to one of the predefined values.
// If the value is found in the list, the index is set accordingly.
func (esl *EditSelectList) SetInitialValue(value string) *EditSelectList {
	esl.InputField.SetText(value)

	// Try to find the value in the list
	for i, v := range esl.values {
		if v == value {
			esl.currentIndex = i
			return esl
		}
	}

	// Value not found in list, set index to -1
	esl.currentIndex = -1
	return esl
}

// MoveToNext cycles to the next value in the list.
func (esl *EditSelectList) MoveToNext() {
	if len(esl.values) == 0 {
		return
	}

	// If currently in free-form mode (-1), start at beginning
	if esl.currentIndex == -1 {
		esl.currentIndex = 0
	} else {
		esl.currentIndex = (esl.currentIndex + 1) % len(esl.values)
	}

	esl.InputField.SetText(esl.values[esl.currentIndex])

	// Trigger save callback
	if esl.onSubmit != nil {
		esl.onSubmit(esl.GetText())
	}
}

// MoveToPrevious cycles to the previous value in the list.
func (esl *EditSelectList) MoveToPrevious() {
	if len(esl.values) == 0 {
		return
	}

	// If currently in free-form mode (-1), start at end
	if esl.currentIndex == -1 {
		esl.currentIndex = len(esl.values) - 1
	} else {
		esl.currentIndex--
		if esl.currentIndex < 0 {
			esl.currentIndex = len(esl.values) - 1
		}
	}

	esl.InputField.SetText(esl.values[esl.currentIndex])

	// Trigger save callback
	if esl.onSubmit != nil {
		esl.onSubmit(esl.GetText())
	}
}

// InputHandler handles keyboard input for the edit/select list.
func (esl *EditSelectList) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	// Get the base InputField handler
	baseHandler := esl.InputField.InputHandler()

	// Return our custom handler that intercepts arrow keys
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()

		switch key {
		case tcell.KeyUp:
			// Move to previous value in list
			esl.MoveToPrevious()
			return

		case tcell.KeyDown:
			// Move to next value in list
			esl.MoveToNext()
			return

		default:
			// If typing is disabled, silently ignore all other keys
			if !esl.allowTyping {
				return
			}

			// Let InputField handle other keys
			if baseHandler != nil {
				baseHandler(event, setFocus)
			}

			// After user types, switch to free-form mode
			esl.currentIndex = -1

			// Save immediately after typing
			if esl.onSubmit != nil {
				esl.onSubmit(esl.GetText())
			}
		}
	}
}
