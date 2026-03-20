package component

import (
	"testing"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

func TestRecurrenceEdit_DefaultIsNone(t *testing.T) {
	re := NewRecurrenceEdit()
	if got := re.GetValue(); got != "" {
		t.Errorf("expected empty cron for None, got %q", got)
	}
	if got := re.GetText(); got != "None" {
		t.Errorf("expected display 'None', got %q", got)
	}
}

func TestRecurrenceEdit_SetInitialValue_Daily(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue(string(taskpkg.RecurrenceDaily))

	if got := re.GetValue(); got != string(taskpkg.RecurrenceDaily) {
		t.Errorf("expected %q, got %q", taskpkg.RecurrenceDaily, got)
	}
	if got := re.GetText(); got != "Daily" {
		t.Errorf("expected display 'Daily', got %q", got)
	}
}

func TestRecurrenceEdit_SetInitialValue_Weekly(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("0 0 * * FRI")

	if got := re.GetValue(); got != "0 0 * * FRI" {
		t.Errorf("expected '0 0 * * FRI', got %q", got)
	}
	if got := re.GetText(); got != "Weekly : Friday" {
		t.Errorf("expected display 'Weekly : Friday', got %q", got)
	}
}

func TestRecurrenceEdit_SetInitialValue_Monthly(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("0 0 15 * *")

	if got := re.GetValue(); got != "0 0 15 * *" {
		t.Errorf("expected '0 0 15 * *', got %q", got)
	}
	if got := re.GetText(); got != "Monthly : 15th" {
		t.Errorf("expected display 'Monthly : 15th', got %q", got)
	}
}

func TestRecurrenceEdit_SetInitialValue_None(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("")

	if got := re.GetValue(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestRecurrenceEdit_FrequencySwitchResetsValue(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("0 0 * * FRI") // Weekly on Friday

	// cycle frequency forward: Weekly → Monthly
	re.CycleNext()

	// value should reset to day 1 (default)
	if got := re.GetValue(); got != "0 0 1 * *" {
		t.Errorf("expected '0 0 1 * *' after switch to Monthly, got %q", got)
	}

	// cycle again: Monthly → None (wraps)
	re.CycleNext()
	if got := re.GetValue(); got != "" {
		t.Errorf("expected empty after switch to None, got %q", got)
	}
}

func TestRecurrenceEdit_ChangeHandlerFires(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue(string(taskpkg.RecurrenceDaily))

	var lastValue string
	callCount := 0
	re.SetChangeHandler(func(v string) {
		lastValue = v
		callCount++
	})

	// cycle frequency: Daily → Weekly (defaults to Monday)
	re.CycleNext()

	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
	if lastValue != "0 0 * * MON" {
		t.Errorf("expected '0 0 * * MON', got %q", lastValue)
	}
}

func TestRecurrenceEdit_MovePartRight(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("0 0 * * MON") // Weekly — has value part

	// move to value part
	re.MovePartRight()
	if re.activePart != 1 {
		t.Errorf("expected activePart=1, got %d", re.activePart)
	}
	if got := re.GetText(); got != "Weekly > Monday" {
		t.Errorf("expected 'Weekly > Monday', got %q", got)
	}
}

func TestRecurrenceEdit_MovePartLeft(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("0 0 * * MON")

	// move to value part first, then back to frequency
	re.MovePartRight()
	re.MovePartLeft()
	if re.activePart != 0 {
		t.Errorf("expected activePart=0, got %d", re.activePart)
	}
	if got := re.GetText(); got != "Weekly : Monday" {
		t.Errorf("expected 'Weekly : Monday', got %q", got)
	}
}

func TestRecurrenceEdit_MovePartRightNoopForNone(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("") // None — no value part

	re.MovePartRight()
	if re.activePart != 0 {
		t.Errorf("expected activePart=0 (no value part to move to), got %d", re.activePart)
	}
}

func TestRecurrenceEdit_CycleValuePart(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("0 0 15 * *") // Monthly on 15th

	// switch to value part
	re.MovePartRight()

	// CycleNext increments day
	re.CycleNext()
	if got := re.GetValue(); got != "0 0 16 * *" {
		t.Errorf("expected '0 0 16 * *' after CycleNext, got %q", got)
	}

	// CyclePrev decrements
	re.CyclePrev()
	if got := re.GetValue(); got != "0 0 15 * *" {
		t.Errorf("expected '0 0 15 * *' after CyclePrev, got %q", got)
	}
}

func TestRecurrenceEdit_CycleWeekdayValuePart(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("0 0 * * MON")

	re.MovePartRight()
	re.CycleNext()
	if got := re.GetValue(); got != "0 0 * * TUE" {
		t.Errorf("expected '0 0 * * TUE', got %q", got)
	}
}

func TestRecurrenceEdit_MonthlyDayWraps(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("0 0 31 * *")

	re.MovePartRight()
	re.CycleNext()
	if re.day != 1 {
		t.Errorf("expected day=1 after wrap, got %d", re.day)
	}
}

func TestRecurrenceEdit_SetLabel(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetLabel("Recurrence: ")
	if got := re.GetLabel(); got != "Recurrence: " {
		t.Errorf("expected label 'Recurrence: ', got %q", got)
	}
}
