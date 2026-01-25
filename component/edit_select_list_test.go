package component

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestEditSelectList_ArrowNavigation(t *testing.T) {
	values := []string{"ready", "in_progress", "review", "done"}
	esl := NewEditSelectList(values, true)

	tests := []struct {
		name          string
		initialIndex  int
		key           tcell.Key
		expectedText  string
		expectedIndex int
		description   string
	}{
		{
			name:          "down from initial (-1) goes to first",
			initialIndex:  -1,
			key:           tcell.KeyDown,
			expectedText:  "ready",
			expectedIndex: 0,
			description:   "Down arrow from free-form mode should select first value",
		},
		{
			name:          "up from initial (-1) goes to last",
			initialIndex:  -1,
			key:           tcell.KeyUp,
			expectedText:  "done",
			expectedIndex: 3,
			description:   "Up arrow from free-form mode should select last value",
		},
		{
			name:          "down from first goes to second",
			initialIndex:  0,
			key:           tcell.KeyDown,
			expectedText:  "in_progress",
			expectedIndex: 1,
			description:   "Down arrow should move to next value",
		},
		{
			name:          "up from first wraps to last",
			initialIndex:  0,
			key:           tcell.KeyUp,
			expectedText:  "done",
			expectedIndex: 3,
			description:   "Up arrow from first value should wrap to last",
		},
		{
			name:          "down from last wraps to first",
			initialIndex:  3,
			key:           tcell.KeyDown,
			expectedText:  "ready",
			expectedIndex: 0,
			description:   "Down arrow from last value should wrap to first",
		},
		{
			name:          "up from last goes to third",
			initialIndex:  3,
			key:           tcell.KeyUp,
			expectedText:  "review",
			expectedIndex: 2,
			description:   "Up arrow should move to previous value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			esl.currentIndex = tt.initialIndex
			if tt.initialIndex >= 0 {
				esl.InputField.SetText(values[tt.initialIndex])
			}

			event := tcell.NewEventKey(tt.key, 0, tcell.ModNone)
			handler := esl.InputHandler()
			handler(event, func(p tview.Primitive) {})

			if esl.GetText() != tt.expectedText {
				t.Errorf("%s: expected text '%s', got '%s'", tt.description, tt.expectedText, esl.GetText())
			}

			if esl.currentIndex != tt.expectedIndex {
				t.Errorf("%s: expected index %d, got %d", tt.description, tt.expectedIndex, esl.currentIndex)
			}
		})
	}
}

func TestEditSelectList_FreeFormTyping(t *testing.T) {
	values := []string{"ready", "in_progress", "review", "done"}
	esl := NewEditSelectList(values, true)

	// Start at a specific index
	esl.currentIndex = 1
	esl.InputField.SetText(values[1]) // "in_progress"

	// Simulate typing (any character key resets index to -1)
	event := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	handler := esl.InputHandler()
	handler(event, func(p tview.Primitive) {})

	if esl.currentIndex != -1 {
		t.Errorf("Expected index to be -1 after typing, got %d", esl.currentIndex)
	}
}

func TestEditSelectList_SubmitHandler(t *testing.T) {
	values := []string{"ready", "in_progress", "review", "done"}
	esl := NewEditSelectList(values, true)

	var submittedText string
	esl.SetSubmitHandler(func(text string) {
		submittedText = text
	})

	// Set some text
	esl.SetText("custom_value")

	// Simulate Enter key
	event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	handler := esl.InputHandler()
	handler(event, func(p tview.Primitive) {})

	if submittedText != "custom_value" {
		t.Errorf("Expected submitted text 'custom_value', got '%s'", submittedText)
	}
}

func TestEditSelectList_SubmitFromList(t *testing.T) {
	values := []string{"ready", "in_progress", "review", "done"}
	esl := NewEditSelectList(values, true)

	var submittedText string
	esl.SetSubmitHandler(func(text string) {
		submittedText = text
	})

	// Navigate to a value using down arrow
	event := tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	handler := esl.InputHandler()
	handler(event, func(p tview.Primitive) {})

	// Submit
	event = tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	handler(event, func(p tview.Primitive) {})

	if submittedText != "ready" {
		t.Errorf("Expected submitted text 'ready', got '%s'", submittedText)
	}
}

func TestEditSelectList_SetInitialValue(t *testing.T) {
	values := []string{"ready", "in_progress", "review", "done"}
	esl := NewEditSelectList(values, true)

	// Set to a value that exists in the list
	esl.SetInitialValue("review")

	if esl.GetText() != "review" {
		t.Errorf("Expected text 'review', got '%s'", esl.GetText())
	}

	if esl.currentIndex != 2 {
		t.Errorf("Expected index 2, got %d", esl.currentIndex)
	}

	// Set to a value that doesn't exist in the list
	esl.SetInitialValue("custom")

	if esl.GetText() != "custom" {
		t.Errorf("Expected text 'custom', got '%s'", esl.GetText())
	}

	if esl.currentIndex != -1 {
		t.Errorf("Expected index -1 for non-list value, got %d", esl.currentIndex)
	}
}

func TestEditSelectList_Clear(t *testing.T) {
	values := []string{"ready", "in_progress", "review", "done"}
	esl := NewEditSelectList(values, true)

	esl.SetInitialValue("review")
	esl.Clear()

	if esl.GetText() != "" {
		t.Errorf("Expected empty text after Clear, got '%s'", esl.GetText())
	}

	if esl.currentIndex != -1 {
		t.Errorf("Expected index -1 after Clear, got %d", esl.currentIndex)
	}
}

func TestEditSelectList_SetText(t *testing.T) {
	values := []string{"ready", "in_progress", "review", "done"}
	esl := NewEditSelectList(values, true)

	// Start with a list selection
	esl.currentIndex = 2
	esl.InputField.SetText(values[2])

	// SetText should reset to free-form mode
	esl.SetText("new_text")

	if esl.GetText() != "new_text" {
		t.Errorf("Expected text 'new_text', got '%s'", esl.GetText())
	}

	if esl.currentIndex != -1 {
		t.Errorf("Expected index -1 after SetText, got %d", esl.currentIndex)
	}
}

func TestEditSelectList_EmptyValues(t *testing.T) {
	esl := NewEditSelectList([]string{}, true)

	// Arrow keys should do nothing with empty list
	event := tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	handler := esl.InputHandler()
	handler(event, func(p tview.Primitive) {})

	if esl.GetText() != "" {
		t.Errorf("Expected empty text, got '%s'", esl.GetText())
	}

	if esl.currentIndex != -1 {
		t.Errorf("Expected index -1, got %d", esl.currentIndex)
	}
}

func TestEditSelectList_NavigationAfterTyping(t *testing.T) {
	values := []string{"ready", "in_progress", "review", "done"}
	esl := NewEditSelectList(values, true)

	// Type some text (simulated by SetText which sets index to -1)
	esl.SetText("custom")

	// Now press down arrow - should go to first item
	event := tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	handler := esl.InputHandler()
	handler(event, func(p tview.Primitive) {})

	if esl.GetText() != "ready" {
		t.Errorf("Expected text 'ready' after down arrow from free-form, got '%s'", esl.GetText())
	}

	if esl.currentIndex != 0 {
		t.Errorf("Expected index 0, got %d", esl.currentIndex)
	}
}

func TestEditSelectList_SetLabel(t *testing.T) {
	values := []string{"ready"}
	esl := NewEditSelectList(values, true)

	esl.SetLabel("Status: ")

	if esl.GetLabel() != "Status: " {
		t.Errorf("Expected label 'Status: ', got '%s'", esl.GetLabel())
	}
}

func TestEditSelectList_TypingDisabled_IgnoresNonArrowKeys(t *testing.T) {
	values := []string{"ready", "in_progress", "done"}
	esl := NewEditSelectList(values, false) // typing disabled

	esl.SetInitialValue("ready")

	// Simulate typing various characters
	handler := esl.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone), func(p tview.Primitive) {})
	handler(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone), func(p tview.Primitive) {})
	handler(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone), func(p tview.Primitive) {})

	// Value should remain unchanged
	if esl.GetText() != "ready" {
		t.Errorf("Expected text unchanged at 'ready', got '%s'", esl.GetText())
	}
	if esl.currentIndex != 0 {
		t.Errorf("Expected index unchanged at 0, got %d", esl.currentIndex)
	}
}

func TestEditSelectList_TypingDisabled_ArrowKeysStillWork(t *testing.T) {
	values := []string{"ready", "in_progress", "done"}
	esl := NewEditSelectList(values, false)

	handler := esl.InputHandler()

	// Down arrow should still work
	handler(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), func(p tview.Primitive) {})
	if esl.GetText() != "ready" {
		t.Errorf("Expected 'ready', got '%s'", esl.GetText())
	}

	// Up arrow should still work
	handler(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), func(p tview.Primitive) {})
	if esl.GetText() != "done" {
		t.Errorf("Expected 'done', got '%s'", esl.GetText())
	}
}

func TestEditSelectList_TypingEnabled_AllowsFreeForm(t *testing.T) {
	values := []string{"ready", "in_progress", "done"}
	esl := NewEditSelectList(values, true) // typing enabled

	esl.SetInitialValue("ready")

	// Simulate typing
	esl.SetText("custom_value")

	if esl.GetText() != "custom_value" {
		t.Errorf("Expected 'custom_value', got '%s'", esl.GetText())
	}
	if esl.currentIndex != -1 {
		t.Errorf("Expected index -1, got %d", esl.currentIndex)
	}
}

func TestEditSelectList_SubmitCallbackNotFiredWhenTypingDisabled(t *testing.T) {
	values := []string{"ready", "in_progress", "done"}
	esl := NewEditSelectList(values, false)

	callCount := 0
	esl.SetSubmitHandler(func(text string) {
		callCount++
	})

	esl.SetInitialValue("ready")
	callCount = 0 // Reset after SetInitialValue

	// Try to type
	handler := esl.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone), func(p tview.Primitive) {})

	// Callback should not have been called
	if callCount != 0 {
		t.Errorf("Expected no callbacks, got %d", callCount)
	}
}
