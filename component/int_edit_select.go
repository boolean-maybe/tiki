package component

import (
	"strconv"
	"strings"

	"github.com/boolean-maybe/tiki/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// IntEditSelect is a one-line input field for integer values within a range.
// It supports both:
// 1. Direct numeric typing with validation (if allowTyping is true)
// 2. Arrow key navigation to increment/decrement values
//
// Arrow keys wrap around at boundaries (up at min goes to max, down at max goes to min).
// If allowTyping is false, only arrow keys work; typing is silently ignored.
// The change handler is called immediately on every valid change.
type IntEditSelect struct {
	*tview.InputField
	min          int
	max          int
	currentValue int
	onChange     func(value int)
	clearOnType  bool // flag to clear field on next keystroke
	allowTyping  bool // Controls whether direct typing is allowed
}

// NewIntEditSelect creates a new integer input field with the specified range [min, max].
// The initial value is set to min. If allowTyping is true, users can type digits;
// if false, only arrow keys work.
func NewIntEditSelect(min, max int, allowTyping bool) *IntEditSelect {
	if min > max {
		panic("IntEditSelect: min cannot be greater than max")
	}

	inputField := tview.NewInputField()
	inputField.SetFieldBackgroundColor(config.GetContentBackgroundColor())
	inputField.SetFieldTextColor(config.GetContentTextColor())

	ies := &IntEditSelect{
		InputField:   inputField,
		min:          min,
		max:          max,
		currentValue: min,
		allowTyping:  allowTyping,
	}

	// Set initial text
	inputField.SetText(strconv.Itoa(min))

	// Set focus handler to enable text replacement on first keystroke
	inputField.SetFocusFunc(func() {
		ies.clearOnType = true
	})

	return ies
}

// SetChangeHandler sets the callback function that is called whenever the value changes.
func (ies *IntEditSelect) SetChangeHandler(handler func(value int)) *IntEditSelect {
	ies.onChange = handler
	return ies
}

// SetLabel sets the label displayed before the input field.
func (ies *IntEditSelect) SetLabel(label string) *IntEditSelect {
	ies.InputField.SetLabel(label)
	return ies
}

// SetValue sets the current value, clamping it to the valid range [min, max].
func (ies *IntEditSelect) SetValue(value int) *IntEditSelect {
	// Clamp to range
	if value < ies.min {
		value = ies.min
	} else if value > ies.max {
		value = ies.max
	}

	ies.currentValue = value
	ies.SetText(strconv.Itoa(value))
	return ies
}

// GetValue returns the current integer value.
func (ies *IntEditSelect) GetValue() int {
	return ies.currentValue
}

// Clear resets the value to the minimum value in the range.
func (ies *IntEditSelect) Clear() *IntEditSelect {
	return ies.SetValue(ies.min)
}

// increment increases the value by 1, wrapping from max to min.
func (ies *IntEditSelect) increment() {
	newValue := ies.currentValue + 1
	if newValue > ies.max {
		newValue = ies.min
	}

	ies.currentValue = newValue
	ies.SetText(strconv.Itoa(newValue))

	// Trigger callback
	if ies.onChange != nil {
		ies.onChange(newValue)
	}
}

// decrement decreases the value by 1, wrapping from min to max.
func (ies *IntEditSelect) decrement() {
	newValue := ies.currentValue - 1
	if newValue < ies.min {
		newValue = ies.max
	}

	ies.currentValue = newValue
	ies.SetText(strconv.Itoa(newValue))

	// Trigger callback
	if ies.onChange != nil {
		ies.onChange(newValue)
	}
}

// validateAndUpdate parses the current text and updates currentValue if valid.
// If the text is invalid or out of range, it reverts to the last valid value.
func (ies *IntEditSelect) validateAndUpdate() {
	text := strings.TrimSpace(ies.GetText())

	// Allow empty input temporarily (user might be typing)
	if text == "" || text == "-" {
		return
	}

	// Try to parse as integer
	value, err := strconv.Atoi(text)
	if err != nil {
		// Invalid input, revert to current value
		ies.SetText(strconv.Itoa(ies.currentValue))
		return
	}

	// Check if in range
	if value < ies.min || value > ies.max {
		// Out of range, clamp to nearest boundary
		if value < ies.min {
			value = ies.min
		} else {
			value = ies.max
		}
		ies.SetText(strconv.Itoa(value))
	}

	// Update current value if it changed
	if value != ies.currentValue {
		ies.currentValue = value

		// Trigger callback
		if ies.onChange != nil {
			ies.onChange(value)
		}
	}
}

// InputHandler handles keyboard input for the integer input field.
func (ies *IntEditSelect) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	// Get the base InputField handler
	baseHandler := ies.InputField.InputHandler()

	// Return our custom handler that intercepts arrow keys
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()

		switch key {
		case tcell.KeyUp:
			// Decrement value (wraps at min to max)
			ies.clearOnType = false // user is navigating, not typing fresh
			ies.decrement()
			return

		case tcell.KeyDown:
			// Increment value (wraps at max to min)
			ies.clearOnType = false // user is navigating, not typing fresh
			ies.increment()
			return

		case tcell.KeyRune:
			// If typing is disabled, silently ignore
			if !ies.allowTyping {
				return
			}

			// Only allow digits and minus sign
			ch := event.Rune()
			if (ch >= '0' && ch <= '9') || (ch == '-' && ies.min < 0) {
				// If clearOnType flag is set, clear the field first
				if ies.clearOnType {
					ies.SetText("")
					ies.clearOnType = false
				}
				// Let InputField handle the character
				if baseHandler != nil {
					baseHandler(event, setFocus)
				}
				// Validate after input
				ies.validateAndUpdate()
			}
			// Ignore other characters
			return

		case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete:
			// If typing is disabled, silently ignore
			if !ies.allowTyping {
				return
			}

			// Allow deletion
			ies.clearOnType = false // user is editing, not starting fresh
			if baseHandler != nil {
				baseHandler(event, setFocus)
			}
			// Validate after deletion
			ies.validateAndUpdate()
			return

		case tcell.KeyCtrlU:
			// If typing is disabled, silently ignore
			if !ies.allowTyping {
				return
			}

			// Ctrl+U clears the field (standard tview behavior)
			ies.clearOnType = false // user is editing, not starting fresh
			if baseHandler != nil {
				baseHandler(event, setFocus)
			}
			// After clearing, validate (should revert to min or stay empty)
			ies.validateAndUpdate()
			return

		default:
			// Let InputField handle other keys (Tab, Enter, etc.)
			if baseHandler != nil {
				baseHandler(event, setFocus)
			}
		}
	}
}
