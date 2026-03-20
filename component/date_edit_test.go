package component

import (
	"testing"
	"time"

	taskpkg "github.com/boolean-maybe/tiki/task"

	"github.com/gdamore/tcell/v2"
)

func TestNewDateEdit(t *testing.T) {
	de := NewDateEdit()

	if de.currentText != "" {
		t.Errorf("Expected empty currentText, got %q", de.currentText)
	}
	if de.GetText() != "" {
		t.Errorf("Expected empty text, got %q", de.GetText())
	}
}

func TestDateEdit_SetInitialValue(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2025-06-15")

	if de.currentText != "2025-06-15" {
		t.Errorf("Expected currentText='2025-06-15', got %q", de.currentText)
	}
	if de.GetText() != "2025-06-15" {
		t.Errorf("Expected text='2025-06-15', got %q", de.GetText())
	}
}

func TestDateEdit_SetInitialValueEmpty(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("")

	if de.currentText != "" {
		t.Errorf("Expected empty currentText, got %q", de.currentText)
	}
}

func TestDateEdit_IncrementDay(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2025-06-15")

	de.incrementDay()

	if de.currentText != "2025-06-16" {
		t.Errorf("Expected '2025-06-16', got %q", de.currentText)
	}
	if de.GetText() != "2025-06-16" {
		t.Errorf("Expected text '2025-06-16', got %q", de.GetText())
	}
}

func TestDateEdit_DecrementDay(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2025-06-15")

	de.decrementDay()

	if de.currentText != "2025-06-14" {
		t.Errorf("Expected '2025-06-14', got %q", de.currentText)
	}
}

func TestDateEdit_IncrementFromEmpty(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("")

	de.incrementDay()

	// should start from tomorrow and add one more day
	expected := time.Now().AddDate(0, 0, 1).AddDate(0, 0, 1).Format(taskpkg.DateFormat)
	if de.currentText != expected {
		t.Errorf("Expected %q, got %q", expected, de.currentText)
	}
}

func TestDateEdit_DecrementFromEmpty(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("")

	de.decrementDay()

	// should start from tomorrow and subtract one day = today
	expected := time.Now().Format(taskpkg.DateFormat)
	if de.currentText != expected {
		t.Errorf("Expected %q, got %q", expected, de.currentText)
	}
}

func TestDateEdit_ArrowKeyHandling(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2025-01-15")

	handler := de.InputHandler()

	// down arrow increments
	handler(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), nil)
	if de.currentText != "2025-01-16" {
		t.Errorf("After KeyDown, expected '2025-01-16', got %q", de.currentText)
	}

	// up arrow decrements
	handler(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), nil)
	if de.currentText != "2025-01-15" {
		t.Errorf("After KeyUp, expected '2025-01-15', got %q", de.currentText)
	}
}

func TestDateEdit_ChangeHandler(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2025-06-15")

	var calledWith string
	callCount := 0
	de.SetChangeHandler(func(s string) {
		calledWith = s
		callCount++
	})

	de.incrementDay()

	if callCount != 1 {
		t.Errorf("Expected callback called once, got %d", callCount)
	}
	if calledWith != "2025-06-16" {
		t.Errorf("Expected callback with '2025-06-16', got %q", calledWith)
	}
}

func TestDateEdit_ValidateAndUpdate_ValidDate(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2025-06-15")

	var calledWith string
	de.SetChangeHandler(func(s string) {
		calledWith = s
	})

	de.SetText("2025-07-20")
	de.validateAndUpdate()

	if de.currentText != "2025-07-20" {
		t.Errorf("Expected currentText '2025-07-20', got %q", de.currentText)
	}
	if calledWith != "2025-07-20" {
		t.Errorf("Expected callback with '2025-07-20', got %q", calledWith)
	}
}

func TestDateEdit_ValidateAndUpdate_InvalidDate(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2025-06-15")

	de.SetText("not-a-date")
	de.validateAndUpdate()

	// should revert
	if de.currentText != "2025-06-15" {
		t.Errorf("Expected currentText unchanged at '2025-06-15', got %q", de.currentText)
	}
	if de.GetText() != "2025-06-15" {
		t.Errorf("Expected text reverted to '2025-06-15', got %q", de.GetText())
	}
}

func TestDateEdit_ValidateAndUpdate_EmptyClears(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2025-06-15")

	var calledWith string
	de.SetChangeHandler(func(s string) {
		calledWith = s
	})

	de.SetText("")
	de.validateAndUpdate()

	// empty is valid (clears due date)
	if de.currentText != "" {
		t.Errorf("Expected empty currentText, got %q", de.currentText)
	}
	if calledWith != "" {
		t.Errorf("Expected callback with empty string, got %q", calledWith)
	}
}

func TestDateEdit_OnlyDigitsAndHyphens(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2025-06-15")
	de.clearOnType = false // simulate already-typed state

	handler := de.InputHandler()

	// letters should be ignored
	handler(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone), nil)
	handler(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone), nil)

	// text should be unchanged
	if de.GetText() != "2025-06-15" {
		t.Errorf("Expected text unchanged, got %q", de.GetText())
	}
}

func TestDateEdit_CtrlU_ClearsToNone(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2025-06-15")

	var calledWith string
	callCount := 0
	de.SetChangeHandler(func(s string) {
		calledWith = s
		callCount++
	})

	handler := de.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModNone), nil)

	if de.currentText != "" {
		t.Errorf("Expected empty currentText after Ctrl+U, got %q", de.currentText)
	}
	if de.GetText() != "" {
		t.Errorf("Expected empty text after Ctrl+U, got %q", de.GetText())
	}
	if callCount != 1 {
		t.Errorf("Expected callback called once, got %d", callCount)
	}
	if calledWith != "" {
		t.Errorf("Expected callback with empty string, got %q", calledWith)
	}
}

func TestDateEdit_MonthBoundary(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2025-01-31")

	de.incrementDay()

	if de.currentText != "2025-02-01" {
		t.Errorf("Expected '2025-02-01', got %q", de.currentText)
	}
}

func TestDateEdit_Backspace_ClearsToNone(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2025-06-15")

	var calledWith string
	callCount := 0
	de.SetChangeHandler(func(s string) {
		calledWith = s
		callCount++
	})

	handler := de.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone), nil)

	if de.currentText != "" {
		t.Errorf("Expected empty currentText after Backspace, got %q", de.currentText)
	}
	if de.GetText() != "" {
		t.Errorf("Expected empty text after Backspace, got %q", de.GetText())
	}
	if callCount != 1 {
		t.Errorf("Expected callback called once, got %d", callCount)
	}
	if calledWith != "" {
		t.Errorf("Expected callback with empty string, got %q", calledWith)
	}
}

func TestDateEdit_FluentAPI(t *testing.T) {
	called := false
	de := NewDateEdit().
		SetLabel("Due: ").
		SetInitialValue("2025-03-01").
		SetChangeHandler(func(s string) {
			called = true
		})

	if de.GetLabel() != "Due: " {
		t.Errorf("Expected label 'Due: ', got %q", de.GetLabel())
	}
	if de.currentText != "2025-03-01" {
		t.Errorf("Expected currentText '2025-03-01', got %q", de.currentText)
	}

	de.incrementDay()
	if !called {
		t.Error("Expected change handler to be called")
	}
}
