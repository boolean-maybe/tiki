//go:build ignore
// +build ignore

// This is a standalone test application for the CompletionPrompt component.
// Run with: go run view/completion_prompt_test_app.go

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()

	// Sample word list for testing
	words := []string{
		"apple",
		"application",
		"banana",
		"berry",
		"cherry",
		"chocolate",
		"date",
		"dragonfruit",
	}

	// Result display
	resultText := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[yellow]Results will appear here[white]\n\nPress Ctrl+C to quit")

	// Create the completion prompt
	prompt := NewCompletionPrompt(words).
		SetLabel("Enter fruit: ").
		SetSubmitHandler(func(text string) {
			resultText.SetText(fmt.Sprintf(
				"[green]Submitted:[white] %s\n\n"+
					"[yellow]Try typing:[white]\n"+
					"- 'a' (shows 'apple' hint, 'app' shows 'application')\n"+
					"- 'b' (no hint - multiple matches)\n"+
					"- 'd' (no hint - multiple matches)\n"+
					"- 'dr' (shows 'dragonfruit' hint)\n"+
					"- Press Tab to accept hint\n"+
					"- Press Enter to submit without hint",
				text,
			))
		})

	// Instructions
	instructions := tview.NewTextView().
		SetDynamicColors(true).
		SetText(
			"[yellow]CompletionPrompt Test[white]\n\n" +
				"[green]Instructions:[white]\n" +
				"1. Type letters to see auto-completion hints in grey\n" +
				"2. Press [yellow]Tab[white] to accept the hint\n" +
				"3. Press [yellow]Enter[white] to submit (ignores hint)\n" +
				"4. Hints appear only when there's exactly one match\n\n" +
				"[yellow]Try typing:[white] a, app, dr, c, ch",
		)

	// Layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(instructions, 10, 0, false).
		AddItem(prompt, 1, 0, true).
		AddItem(resultText, 0, 1, false)

	// Set up the application
	app.SetRoot(flex, true).SetFocus(prompt)

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// CompletionPrompt implementation (copied from view/completion_prompt.go for standalone testing)

type CompletionPrompt struct {
	*tview.InputField
	words       []string
	currentHint string
	onSubmit    func(text string)
	hintColor   tcell.Color
}

func NewCompletionPrompt(words []string) *CompletionPrompt {
	inputField := tview.NewInputField()
	inputField.SetFieldBackgroundColor(tcell.ColorDefault)
	inputField.SetFieldTextColor(tcell.ColorWhite)

	cp := &CompletionPrompt{
		InputField: inputField,
		words:      words,
		hintColor:  tcell.ColorGray,
	}

	return cp
}

func (cp *CompletionPrompt) SetSubmitHandler(handler func(text string)) *CompletionPrompt {
	cp.onSubmit = handler
	return cp
}

func (cp *CompletionPrompt) SetLabel(label string) *CompletionPrompt {
	cp.InputField.SetLabel(label)
	return cp
}

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

	if len(matches) == 1 {
		cp.currentHint = matches[0][len(text):]
	} else {
		cp.currentHint = ""
	}
}

func (cp *CompletionPrompt) Draw(screen tcell.Screen) {
	cp.InputField.Draw(screen)

	if cp.currentHint != "" {
		x, y, width, height := cp.GetRect()
		if width <= 0 || height <= 0 {
			return
		}

		label := cp.InputField.GetLabel()
		labelWidth := len(label)
		textLength := len(cp.GetText())

		hintX := x + labelWidth + textLength
		hintY := y

		style := tcell.StyleDefault.Foreground(cp.hintColor)
		for i, ch := range cp.currentHint {
			if hintX+i >= x+width {
				break
			}
			screen.SetContent(hintX+i, hintY, ch, nil, style)
		}
	}
}

func (cp *CompletionPrompt) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return cp.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		key := event.Key()

		switch key {
		case tcell.KeyTab:
			if cp.currentHint != "" {
				currentText := cp.GetText()
				cp.SetText(currentText + cp.currentHint)
				cp.currentHint = ""
			}
			return

		case tcell.KeyEnter:
			if cp.onSubmit != nil {
				cp.onSubmit(cp.GetText())
			}
			return

		default:
			handler := cp.InputField.InputHandler()
			if handler != nil {
				handler(event, setFocus)
			}
			cp.updateHint()
		}
	})
}
