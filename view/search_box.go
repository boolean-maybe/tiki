package view

import (
	"github.com/boolean-maybe/tiki/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SearchBox is a single-line input field with a "> " prompt
type SearchBox struct {
	*tview.InputField
	onSubmit func(text string)
	onCancel func()
}

// NewSearchBox creates a new search box widget
func NewSearchBox() *SearchBox {
	colors := config.GetColors()
	inputField := tview.NewInputField()

	inputField.SetLabel("> ")
	inputField.SetLabelColor(colors.SearchBoxLabelColor.TCell())
	inputField.SetFieldBackgroundColor(colors.ContentBackgroundColor.TCell())
	inputField.SetFieldTextColor(colors.ContentTextColor.TCell())
	inputField.SetBorder(true)
	inputField.SetBorderColor(colors.TaskBoxUnselectedBorder.TCell())

	sb := &SearchBox{
		InputField: inputField,
	}

	return sb
}

// SetSubmitHandler sets the callback for when Enter is pressed
func (sb *SearchBox) SetSubmitHandler(handler func(text string)) *SearchBox {
	sb.onSubmit = handler
	return sb
}

// SetCancelHandler sets the callback for when Escape is pressed
func (sb *SearchBox) SetCancelHandler(handler func()) *SearchBox {
	sb.onCancel = handler
	return sb
}

// Clear clears the search text
func (sb *SearchBox) Clear() *SearchBox {
	sb.SetText("")
	return sb
}

// InputHandler handles key input for the search box
func (sb *SearchBox) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return sb.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()

		// Handle submit and cancel
		switch key {
		case tcell.KeyEnter:
			if sb.onSubmit != nil {
				sb.onSubmit(sb.GetText())
			}
			return
		case tcell.KeyEscape:
			if sb.onCancel != nil {
				sb.onCancel()
			}
			return
		}

		// Only allow typing and basic editing - block everything else
		if sb.isAllowedKey(event) {
			handler := sb.InputField.InputHandler()
			if handler != nil {
				handler(event, setFocus)
			}
		}
		// All other keys silently ignored (consumed)
	})
}

// isAllowedKey returns true if the key should be processed by the InputField
func (sb *SearchBox) isAllowedKey(event *tcell.EventKey) bool {
	key := event.Key()

	// Allow basic editing keys
	switch key {
	case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete:
		return true

	// Allow printable characters (letters, digits, symbols)
	case tcell.KeyRune:
		return true
	}

	// Block everything else:
	// - All arrows (Left, Right, Up, Down)
	// - Tab, Home, End, PageUp, PageDown
	// - All function keys (F1-F12)
	// - All control sequences
	return false
}
