package component

import (
	"strings"

	"github.com/boolean-maybe/tiki/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CompletionPrompt is an input field with auto-completion hints.
// When user input is a prefix of exactly one word from the word list,
// the component displays a greyed completion hint.
type CompletionPrompt struct {
	*tview.InputField
	words       []string
	currentHint string
	onSubmit    func(text string)
	hintColor   tcell.Color
}

// NewCompletionPrompt creates a new completion prompt with the given word list.
func NewCompletionPrompt(words []string) *CompletionPrompt {
	inputField := tview.NewInputField()

	// Configure the input field
	inputField.SetFieldBackgroundColor(config.GetContentBackgroundColor())
	inputField.SetFieldTextColor(config.GetContentTextColor())

	cp := &CompletionPrompt{
		InputField: inputField,
		words:      words,
		hintColor:  tcell.ColorGray,
	}

	return cp
}

// SetSubmitHandler sets the callback for when Enter is pressed.
// Only the user-typed text is passed to the callback (hint is ignored).
func (cp *CompletionPrompt) SetSubmitHandler(handler func(text string)) *CompletionPrompt {
	cp.onSubmit = handler
	return cp
}

// SetLabel sets the label displayed before the input field.
func (cp *CompletionPrompt) SetLabel(label string) *CompletionPrompt {
	cp.InputField.SetLabel(label)
	return cp
}

// SetHintColor sets the color for the completion hint text.
func (cp *CompletionPrompt) SetHintColor(color tcell.Color) *CompletionPrompt {
	cp.hintColor = color
	return cp
}

// Clear clears the input text and hint.
func (cp *CompletionPrompt) Clear() *CompletionPrompt {
	cp.SetText("")
	cp.currentHint = ""
	return cp
}

// updateHint recalculates the completion hint based on current input.
// Case-insensitive prefix matching is used.
func (cp *CompletionPrompt) updateHint() {
	text := cp.GetText()
	if text == "" {
		cp.currentHint = ""
		return
	}

	textLower := strings.ToLower(text)
	var matches []string

	for _, word := range cp.words {
		if strings.HasPrefix(strings.ToLower(word), textLower) {
			matches = append(matches, word)
		}
	}

	// Only show hint if exactly one match
	if len(matches) == 1 {
		// Hint is the remaining characters (preserving original case from word list)
		cp.currentHint = matches[0][len(text):]
	} else {
		cp.currentHint = ""
	}
}

// Draw renders the input field and the completion hint.
func (cp *CompletionPrompt) Draw(screen tcell.Screen) {
	// First, let the InputField draw itself normally
	cp.InputField.Draw(screen)

	// If there's a hint, draw it in grey after the user's input
	if cp.currentHint != "" {
		x, y, width, height := cp.GetRect()
		if width <= 0 || height <= 0 {
			return
		}

		// Calculate position for hint text
		// Position = field start + label width + current text length
		label := cp.GetLabel()
		labelWidth := len(label)
		textLength := len(cp.GetText())

		// Hint starts after the label and current text
		hintX := x + labelWidth + textLength
		hintY := y

		// Draw each character of the hint
		style := tcell.StyleDefault.Foreground(cp.hintColor)
		for i, ch := range cp.currentHint {
			if hintX+i >= x+width {
				break // Don't draw beyond field width
			}
			screen.SetContent(hintX+i, hintY, ch, nil, style)
		}
	}
}

// InputHandler handles keyboard input for the completion prompt.
func (cp *CompletionPrompt) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return cp.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()

		switch key {
		case tcell.KeyTab:
			// Accept the hint if it exists
			if cp.currentHint != "" {
				currentText := cp.GetText()
				cp.SetText(currentText + cp.currentHint)
				cp.currentHint = ""
			}
			// Don't propagate Tab to InputField
			return

		case tcell.KeyEnter:
			// Submit only the user-typed text (ignore hint)
			if cp.onSubmit != nil {
				cp.onSubmit(cp.GetText())
			}
			// Don't propagate Enter to InputField
			return

		default:
			// Let InputField handle the key first
			handler := cp.InputField.InputHandler()
			if handler != nil {
				handler(event, setFocus)
			}

			// After handling, update the hint based on new text
			cp.updateHint()
		}
	})
}
