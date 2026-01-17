package component

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestCompletionPrompt_UpdateHint(t *testing.T) {
	words := []string{"apple", "application", "banana", "berry", "cherry"}

	tests := []struct {
		name         string
		input        string
		expectedHint string
		description  string
	}{
		{
			name:         "empty input",
			input:        "",
			expectedHint: "",
			description:  "Empty input should have no hint",
		},
		{
			name:         "single match",
			input:        "c",
			expectedHint: "herry",
			description:  "Single match should show hint",
		},
		{
			name:         "single match with more chars",
			input:        "applic",
			expectedHint: "ation",
			description:  "Partial input with single match should show remaining chars",
		},
		{
			name:         "complete word",
			input:        "apple",
			expectedHint: "",
			description:  "Complete word match should show empty hint (word matches itself)",
		},
		{
			name:         "multiple matches 'a'",
			input:        "a",
			expectedHint: "",
			description:  "Multiple matches should show no hint",
		},
		{
			name:         "multiple matches 'app'",
			input:        "app",
			expectedHint: "",
			description:  "Multiple matches for 'app' (apple and application) should show no hint",
		},
		{
			name:         "multiple matches 'b'",
			input:        "b",
			expectedHint: "",
			description:  "Multiple matches for 'b' should show no hint",
		},
		{
			name:         "no match",
			input:        "x",
			expectedHint: "",
			description:  "No match should show no hint",
		},
		{
			name:         "case insensitive match",
			input:        "APPLIC",
			expectedHint: "ation",
			description:  "Case insensitive matching should work",
		},
		{
			name:         "mixed case",
			input:        "ChE",
			expectedHint: "rry",
			description:  "Mixed case should match and preserve hint from original word",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := NewCompletionPrompt(words)
			cp.SetText(tt.input)
			cp.updateHint()

			if cp.currentHint != tt.expectedHint {
				t.Errorf("%s: expected hint '%s', got '%s'", tt.description, tt.expectedHint, cp.currentHint)
			}
		})
	}
}

func TestCompletionPrompt_TabCompletion(t *testing.T) {
	words := []string{"apple", "application"}
	cp := NewCompletionPrompt(words)

	// Type "applic" - should show "ation" hint (only matches "application")
	cp.SetText("applic")
	cp.updateHint()

	if cp.currentHint != "ation" {
		t.Fatalf("Expected hint 'ation', got '%s'", cp.currentHint)
	}

	// Simulate Tab key press - should accept the hint
	event := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	handler := cp.InputHandler()
	handler(event, func(p tview.Primitive) {})

	// After Tab, text should be "application" and hint should be empty
	if cp.GetText() != "application" {
		t.Errorf("Expected text 'application' after Tab, got '%s'", cp.GetText())
	}

	if cp.currentHint != "" {
		t.Errorf("Expected empty hint after Tab, got '%s'", cp.currentHint)
	}
}

func TestCompletionPrompt_EnterIgnoresHint(t *testing.T) {
	words := []string{"apple", "application"}
	cp := NewCompletionPrompt(words)

	var submittedText string
	cp.SetSubmitHandler(func(text string) {
		submittedText = text
	})

	// Type "applic" - should show "ation" hint
	cp.SetText("applic")
	cp.updateHint()

	if cp.currentHint != "ation" {
		t.Fatalf("Expected hint 'ation', got '%s'", cp.currentHint)
	}

	// Simulate Enter key press - should submit only "applic"
	event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	handler := cp.InputHandler()
	handler(event, func(p tview.Primitive) {})

	if submittedText != "applic" {
		t.Errorf("Expected submitted text 'applic', got '%s'", submittedText)
	}
}

func TestCompletionPrompt_HintDisappearsOnMismatch(t *testing.T) {
	words := []string{"cherry"}
	cp := NewCompletionPrompt(words)

	// Type "c" - should show "herry" hint
	cp.SetText("c")
	cp.updateHint()

	if cp.currentHint != "herry" {
		t.Fatalf("Expected hint 'herry', got '%s'", cp.currentHint)
	}

	// Type "ca" (which doesn't match "cherry") - hint should disappear
	cp.SetText("ca")
	cp.updateHint()

	if cp.currentHint != "" {
		t.Errorf("Expected empty hint after mismatch, got '%s'", cp.currentHint)
	}
}

func TestCompletionPrompt_SettersAndGetters(t *testing.T) {
	words := []string{"test"}
	cp := NewCompletionPrompt(words)

	// Test SetLabel
	cp.SetLabel("Test: ")
	if cp.GetLabel() != "Test: " {
		t.Errorf("Expected label 'Test: ', got '%s'", cp.GetLabel())
	}

	// Test SetHintColor
	cp.SetHintColor(tcell.ColorRed)
	if cp.hintColor != tcell.ColorRed {
		t.Errorf("Expected hint color Red, got %v", cp.hintColor)
	}

	// Test Clear
	cp.SetText("something")
	cp.currentHint = "hint"
	cp.Clear()
	if cp.GetText() != "" || cp.currentHint != "" {
		t.Errorf("Clear should reset text and hint")
	}
}
