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

	// Configure the input field (border drawn manually in Draw)
	inputField.SetLabel("> ")
	inputField.SetLabelColor(colors.SearchBoxLabelColor)
	inputField.SetFieldBackgroundColor(config.GetContentBackgroundColor())
	inputField.SetFieldTextColor(config.GetContentTextColor())
	inputField.SetBorder(false)

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

// Draw renders the search box with single-line borders
// (overrides InputField.Draw to avoid double-line focus borders)
func (sb *SearchBox) Draw(screen tcell.Screen) {
	x, y, width, height := sb.GetRect()
	if width <= 0 || height <= 0 {
		return
	}

	// Fill interior with theme-aware background color
	bgColor := config.GetContentBackgroundColor()
	bgStyle := tcell.StyleDefault.Background(bgColor)
	for row := y; row < y+height; row++ {
		for col := x; col < x+width; col++ {
			screen.SetContent(col, row, ' ', nil, bgStyle)
		}
	}

	// Draw single-line border using shared utility
	DrawSingleLineBorder(screen, x, y, width, height)

	// Draw InputField inside border (offset by 1 for border)
	sb.SetRect(x+1, y+1, width-2, height-2)
	sb.InputField.Draw(screen)
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
