package view

import (
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// InputBox is a single-line input field with a configurable prompt
type InputBox struct {
	*tview.InputField
	onSubmit func(text string) controller.InputSubmitResult
	onCancel func()
}

// NewInputBox creates a new input box widget with the default "> " prompt
func NewInputBox() *InputBox {
	colors := config.GetColors()
	inputField := tview.NewInputField()

	inputField.SetLabel("> ")
	inputField.SetLabelColor(colors.InputBoxLabelColor.TCell())
	inputField.SetFieldBackgroundColor(colors.InputBoxBackgroundColor.TCell())
	inputField.SetFieldTextColor(colors.InputBoxTextColor.TCell())
	inputField.SetBorder(true)
	inputField.SetBorderColor(colors.TaskBoxUnselectedBorder.TCell())

	sb := &InputBox{
		InputField: inputField,
	}

	return sb
}

// SetPrompt changes the prompt label displayed before the input text.
func (sb *InputBox) SetPrompt(label string) *InputBox {
	sb.SetLabel(label)
	return sb
}

// SetSubmitHandler sets the callback for when Enter is pressed.
// The callback returns an InputSubmitResult controlling box disposition.
func (sb *InputBox) SetSubmitHandler(handler func(text string) controller.InputSubmitResult) *InputBox {
	sb.onSubmit = handler
	return sb
}

// SetCancelHandler sets the callback for when Escape is pressed
func (sb *InputBox) SetCancelHandler(handler func()) *InputBox {
	sb.onCancel = handler
	return sb
}

// Clear clears the input text
func (sb *InputBox) Clear() *InputBox {
	sb.SetText("")
	return sb
}

// InputHandler handles key input for the input box
func (sb *InputBox) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return sb.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()

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

		if sb.isAllowedKey(event) {
			handler := sb.InputField.InputHandler()
			if handler != nil {
				handler(event, setFocus)
			}
		}
	})
}

// isAllowedKey returns true if the key should be processed by the InputField
func (sb *InputBox) isAllowedKey(event *tcell.EventKey) bool {
	key := event.Key()

	switch key {
	case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete:
		return true
	case tcell.KeyRune:
		return true
	}

	return false
}
