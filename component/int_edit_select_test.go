package component

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestNewIntEditSelect(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)

	if ies.min != 0 {
		t.Errorf("Expected min=0, got %d", ies.min)
	}
	if ies.max != 9 {
		t.Errorf("Expected max=9, got %d", ies.max)
	}
	if ies.currentValue != 0 {
		t.Errorf("Expected initial value=0, got %d", ies.currentValue)
	}
	if ies.GetText() != "0" {
		t.Errorf("Expected text='0', got '%s'", ies.GetText())
	}
}

func TestNewIntEditSelectNegativeRange(t *testing.T) {
	ies := NewIntEditSelect(-5, 5, true)

	if ies.min != -5 {
		t.Errorf("Expected min=-5, got %d", ies.min)
	}
	if ies.max != 5 {
		t.Errorf("Expected max=5, got %d", ies.max)
	}
	if ies.currentValue != -5 {
		t.Errorf("Expected initial value=-5, got %d", ies.currentValue)
	}
	if ies.GetText() != "-5" {
		t.Errorf("Expected text='-5', got '%s'", ies.GetText())
	}
}

func TestNewIntEditSelectInvalidRange(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic when min > max")
		}
	}()
	NewIntEditSelect(10, 5, true)
}

func TestSetValue(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)

	ies.SetValue(5)
	if ies.GetValue() != 5 {
		t.Errorf("Expected value=5, got %d", ies.GetValue())
	}
	if ies.GetText() != "5" {
		t.Errorf("Expected text='5', got '%s'", ies.GetText())
	}
}

func TestSetValueClampLow(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)

	ies.SetValue(-5)
	if ies.GetValue() != 0 {
		t.Errorf("Expected value clamped to 0, got %d", ies.GetValue())
	}
	if ies.GetText() != "0" {
		t.Errorf("Expected text='0', got '%s'", ies.GetText())
	}
}

func TestSetValueClampHigh(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)

	ies.SetValue(15)
	if ies.GetValue() != 9 {
		t.Errorf("Expected value clamped to 9, got %d", ies.GetValue())
	}
	if ies.GetText() != "9" {
		t.Errorf("Expected text='9', got '%s'", ies.GetText())
	}
}

func TestClear(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)
	ies.SetValue(7)

	ies.Clear()
	if ies.GetValue() != 0 {
		t.Errorf("Expected value reset to 0, got %d", ies.GetValue())
	}
	if ies.GetText() != "0" {
		t.Errorf("Expected text='0', got '%s'", ies.GetText())
	}
}

func TestIncrement(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)
	ies.SetValue(3)

	ies.increment()
	if ies.GetValue() != 4 {
		t.Errorf("Expected value=4, got %d", ies.GetValue())
	}
	if ies.GetText() != "4" {
		t.Errorf("Expected text='4', got '%s'", ies.GetText())
	}
}

func TestIncrementWrapAround(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)
	ies.SetValue(9)

	ies.increment()
	if ies.GetValue() != 0 {
		t.Errorf("Expected value wrapped to 0, got %d", ies.GetValue())
	}
	if ies.GetText() != "0" {
		t.Errorf("Expected text='0', got '%s'", ies.GetText())
	}
}

func TestDecrement(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)
	ies.SetValue(5)

	ies.decrement()
	if ies.GetValue() != 4 {
		t.Errorf("Expected value=4, got %d", ies.GetValue())
	}
	if ies.GetText() != "4" {
		t.Errorf("Expected text='4', got '%s'", ies.GetText())
	}
}

func TestDecrementWrapAround(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)
	ies.SetValue(0)

	ies.decrement()
	if ies.GetValue() != 9 {
		t.Errorf("Expected value wrapped to 9, got %d", ies.GetValue())
	}
	if ies.GetText() != "9" {
		t.Errorf("Expected text='9', got '%s'", ies.GetText())
	}
}

func TestArrowKeyNavigation(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)
	ies.SetValue(5)

	handler := ies.InputHandler()

	// Test down arrow (increment)
	downEvent := tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	handler(downEvent, nil)

	if ies.GetValue() != 6 {
		t.Errorf("After KeyDown, expected value=6, got %d", ies.GetValue())
	}

	// Test up arrow (decrement)
	upEvent := tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	handler(upEvent, nil)
	handler(upEvent, nil)

	if ies.GetValue() != 4 {
		t.Errorf("After 2x KeyUp, expected value=4, got %d", ies.GetValue())
	}
}

func TestArrowKeyWrapAround(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)
	ies.SetValue(9)

	handler := ies.InputHandler()

	// Down at max wraps to min
	downEvent := tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	handler(downEvent, nil)

	if ies.GetValue() != 0 {
		t.Errorf("After KeyDown at max, expected value=0, got %d", ies.GetValue())
	}

	// Up at min wraps to max
	upEvent := tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	handler(upEvent, nil)

	if ies.GetValue() != 9 {
		t.Errorf("After KeyUp at min, expected value=9, got %d", ies.GetValue())
	}
}

func TestChangeHandler(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)

	var calledWith int
	callCount := 0

	ies.SetChangeHandler(func(value int) {
		calledWith = value
		callCount++
	})

	// Test increment triggers callback
	ies.increment()
	if callCount != 1 {
		t.Errorf("Expected callback called once, got %d", callCount)
	}
	if calledWith != 1 {
		t.Errorf("Expected callback with value=1, got %d", calledWith)
	}

	// Test decrement triggers callback
	ies.decrement()
	if callCount != 2 {
		t.Errorf("Expected callback called twice, got %d", callCount)
	}
	if calledWith != 0 {
		t.Errorf("Expected callback with value=0, got %d", calledWith)
	}
}

func TestValidateAndUpdateValidInput(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)

	var calledWith int
	callCount := 0

	ies.SetChangeHandler(func(value int) {
		calledWith = value
		callCount++
	})

	// Simulate typing "7"
	ies.SetText("7")
	ies.validateAndUpdate()

	if ies.GetValue() != 7 {
		t.Errorf("Expected value=7, got %d", ies.GetValue())
	}
	if callCount != 1 {
		t.Errorf("Expected callback called once, got %d", callCount)
	}
	if calledWith != 7 {
		t.Errorf("Expected callback with value=7, got %d", calledWith)
	}
}

func TestValidateAndUpdateInvalidInput(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)
	ies.SetValue(5)

	// Simulate typing invalid text
	ies.SetText("abc")
	ies.validateAndUpdate()

	// Should revert to last valid value
	if ies.GetValue() != 5 {
		t.Errorf("Expected value unchanged at 5, got %d", ies.GetValue())
	}
	if ies.GetText() != "5" {
		t.Errorf("Expected text reverted to '5', got '%s'", ies.GetText())
	}
}

func TestValidateAndUpdateOutOfRangeLow(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)
	ies.SetValue(5) // Start with a different value

	var calledWith int
	callCount := 0

	ies.SetChangeHandler(func(value int) {
		calledWith = value
		callCount++
	})

	// Simulate typing "-5"
	ies.SetText("-5")
	ies.validateAndUpdate()

	// Should clamp to min
	if ies.GetValue() != 0 {
		t.Errorf("Expected value clamped to 0, got %d", ies.GetValue())
	}
	if ies.GetText() != "0" {
		t.Errorf("Expected text clamped to '0', got '%s'", ies.GetText())
	}
	if callCount != 1 {
		t.Errorf("Expected callback called once, got %d", callCount)
	}
	if calledWith != 0 {
		t.Errorf("Expected callback with value=0, got %d", calledWith)
	}
}

func TestValidateAndUpdateOutOfRangeHigh(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)

	var calledWith int
	callCount := 0

	ies.SetChangeHandler(func(value int) {
		calledWith = value
		callCount++
	})

	// Simulate typing "99"
	ies.SetText("99")
	ies.validateAndUpdate()

	// Should clamp to max
	if ies.GetValue() != 9 {
		t.Errorf("Expected value clamped to 9, got %d", ies.GetValue())
	}
	if ies.GetText() != "9" {
		t.Errorf("Expected text clamped to '9', got '%s'", ies.GetText())
	}
	if callCount != 1 {
		t.Errorf("Expected callback called once, got %d", callCount)
	}
	if calledWith != 9 {
		t.Errorf("Expected callback with value=9, got %d", calledWith)
	}
}

func TestValidateAndUpdateEmptyInput(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true)
	ies.SetValue(5)

	// Simulate empty input
	ies.SetText("")
	ies.validateAndUpdate()

	// Should keep current value, allow empty temporarily
	if ies.GetValue() != 5 {
		t.Errorf("Expected value unchanged at 5, got %d", ies.GetValue())
	}
}

func TestValidateAndUpdateMinusOnly(t *testing.T) {
	ies := NewIntEditSelect(-10, 10, true)
	ies.SetValue(5)

	// Simulate typing just "-" (partial input)
	ies.SetText("-")
	ies.validateAndUpdate()

	// Should keep current value, allow partial input
	if ies.GetValue() != 5 {
		t.Errorf("Expected value unchanged at 5, got %d", ies.GetValue())
	}
}

func TestFluentAPI(t *testing.T) {
	called := false

	ies := NewIntEditSelect(0, 9, true).
		SetLabel("Test: ").
		SetValue(7).
		SetChangeHandler(func(value int) {
			called = true
		})

	if ies.GetLabel() != "Test: " {
		t.Errorf("Expected label='Test: ', got '%s'", ies.GetLabel())
	}
	if ies.GetValue() != 7 {
		t.Errorf("Expected value=7, got %d", ies.GetValue())
	}

	ies.increment()
	if !called {
		t.Error("Expected change handler to be called")
	}
}

func TestNegativeRangeNavigation(t *testing.T) {
	ies := NewIntEditSelect(-5, 5, true)
	ies.SetValue(0)

	// Decrement to negative
	ies.decrement()
	if ies.GetValue() != -1 {
		t.Errorf("Expected value=-1, got %d", ies.GetValue())
	}

	// Continue to boundary
	for i := 0; i < 4; i++ {
		ies.decrement()
	}
	if ies.GetValue() != -5 {
		t.Errorf("Expected value=-5, got %d", ies.GetValue())
	}

	// Wrap around from min to max
	ies.decrement()
	if ies.GetValue() != 5 {
		t.Errorf("Expected value wrapped to 5, got %d", ies.GetValue())
	}
}

func TestIntEditSelect_TypingDisabled_IgnoresDigits(t *testing.T) {
	ies := NewIntEditSelect(0, 9, false) // typing disabled
	ies.SetValue(5)

	handler := ies.InputHandler()

	// Try to type digits
	handler(tcell.NewEventKey(tcell.KeyRune, '7', tcell.ModNone), nil)
	handler(tcell.NewEventKey(tcell.KeyRune, '3', tcell.ModNone), nil)

	// Value should remain unchanged
	if ies.GetValue() != 5 {
		t.Errorf("Expected value unchanged at 5, got %d", ies.GetValue())
	}
	if ies.GetText() != "5" {
		t.Errorf("Expected text '5', got '%s'", ies.GetText())
	}
}

func TestIntEditSelect_TypingDisabled_IgnoresBackspace(t *testing.T) {
	ies := NewIntEditSelect(0, 9, false)
	ies.SetValue(7)

	handler := ies.InputHandler()

	// Try to delete
	handler(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone), nil)
	handler(tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone), nil)
	handler(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModNone), nil)

	// Value should remain unchanged
	if ies.GetValue() != 7 {
		t.Errorf("Expected value unchanged at 7, got %d", ies.GetValue())
	}
}

func TestIntEditSelect_TypingDisabled_ArrowKeysWork(t *testing.T) {
	ies := NewIntEditSelect(0, 9, false)
	ies.SetValue(5)

	handler := ies.InputHandler()

	// Up arrow (decrement)
	handler(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), nil)
	if ies.GetValue() != 4 {
		t.Errorf("Expected value 4 after up arrow, got %d", ies.GetValue())
	}

	// Down arrow (increment)
	handler(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), nil)
	handler(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), nil)
	if ies.GetValue() != 6 {
		t.Errorf("Expected value 6 after down arrows, got %d", ies.GetValue())
	}
}

func TestIntEditSelect_TypingEnabled_AllowsDirectInput(t *testing.T) {
	ies := NewIntEditSelect(0, 9, true) // typing enabled
	ies.SetValue(5)

	// Simulate typing by setting text directly (this mimics what InputField would do)
	ies.SetText("8")
	ies.validateAndUpdate()

	if ies.GetValue() != 8 {
		t.Errorf("Expected value 8, got %d", ies.GetValue())
	}
}

func TestIntEditSelect_ChangeCallbackNotFiredWhenTypingBlocked(t *testing.T) {
	ies := NewIntEditSelect(0, 9, false)
	ies.SetValue(5)

	callCount := 0
	ies.SetChangeHandler(func(value int) {
		callCount++
	})

	handler := ies.InputHandler()

	// Try to type
	handler(tcell.NewEventKey(tcell.KeyRune, '8', tcell.ModNone), nil)

	// Callback should not have been called (value unchanged)
	if callCount != 0 {
		t.Errorf("Expected no callbacks, got %d", callCount)
	}
}
