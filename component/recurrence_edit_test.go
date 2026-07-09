package component

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/ruki/recurrence"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestRecurrenceEdit_DefaultIsNone(t *testing.T) {
	re := NewRecurrenceEdit()
	if got := re.GetValue(); got != "" {
		t.Errorf("expected empty cron for None, got %q", got)
	}
	if got := re.displayText(); got != "None" {
		t.Errorf("expected display 'None', got %q", got)
	}
}

func TestRecurrenceEdit_SetInitialValue_Daily(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue(string(recurrence.RecurrenceDaily))

	if got := re.GetValue(); got != string(recurrence.RecurrenceDaily) {
		t.Errorf("expected %q, got %q", recurrence.RecurrenceDaily, got)
	}
	if got := re.displayText(); got != "Daily" {
		t.Errorf("expected display 'Daily', got %q", got)
	}
}

func TestRecurrenceEdit_SetInitialValue_Weekly(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("0 0 * * FRI")

	if got := re.GetValue(); got != "0 0 * * FRI" {
		t.Errorf("expected '0 0 * * FRI', got %q", got)
	}
	if got := re.displayText(); got != "Weekly · Friday" {
		t.Errorf("expected display 'Weekly · Friday', got %q", got)
	}
}

func TestRecurrenceEdit_SetInitialValue_Monthly(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("0 0 15 * *")

	if got := re.GetValue(); got != "0 0 15 * *" {
		t.Errorf("expected '0 0 15 * *', got %q", got)
	}
	if got := re.displayText(); got != "Monthly · 15th" {
		t.Errorf("expected display 'Monthly · 15th', got %q", got)
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
	re.SetInitialValue(string(recurrence.RecurrenceDaily))

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
	// display is a constant separator now; the active part is marked by the band.
	if got := re.displayText(); got != "Weekly · Monday" {
		t.Errorf("expected display 'Weekly · Monday', got %q", got)
	}
	// value part "Monday" occupies runes after "Weekly · " (6 + 3 = 9) → [9,14].
	lo, hi := re.partCells()
	if lo != 9 || hi != 14 {
		t.Errorf("value part cells = (%d,%d), want (9,14)", lo, hi)
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
	// frequency part "Weekly" active → highlight range [0,5].
	lo, hi := re.partCells()
	if lo != 0 || hi != 5 {
		t.Errorf("frequency part cells = (%d,%d), want (0,5)", lo, hi)
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
	if re.label != "Recurrence: " {
		t.Errorf("expected label 'Recurrence: ', got %q", re.label)
	}
}

// drawRecurrence renders a RecurrenceEdit to a simulation screen, returning the
// visible row and per-column styles so a test can assert the active-part band.
func drawRecurrence(t *testing.T, re *RecurrenceEdit, width int, focused bool) (string, []tcell.Style) {
	t.Helper()
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(width, 1)
	re.SetRect(0, 0, width, 1)
	if focused {
		re.Focus(func(tview.Primitive) {})
	}
	re.Draw(screen)
	screen.Show()

	cells, w, _ := screen.GetContents()
	var row []rune
	styles := make([]tcell.Style, w)
	for x := 0; x < w; x++ {
		r := cells[x].Runes
		if len(r) > 0 && r[0] != 0 {
			row = append(row, r[0])
		} else {
			row = append(row, ' ')
		}
		styles[x] = cells[x].Style
	}
	return string(row), styles
}

// TestRecurrenceEdit_DrawNoArrowSeparator confirms the ">" glyph hack is gone:
// the active part is marked by the band, not a separator swap.
func TestRecurrenceEdit_DrawNoArrowSeparator(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("0 0 * * MON")
	re.MovePartRight()
	row, _ := drawRecurrence(t, re, 30, true)
	if strings.Contains(row, ">") {
		t.Errorf("row still shows the '>' separator hack: %q", row)
	}
	if !strings.Contains(row, "Weekly · Monday") {
		t.Errorf("row = %q, want it to contain 'Weekly · Monday'", row)
	}
}

// TestRecurrenceEdit_DrawHighlightsActivePart confirms the active part carries a
// distinct (selection-band) style from the inactive part when focused.
func TestRecurrenceEdit_DrawHighlightsActivePart(t *testing.T) {
	re := NewRecurrenceEdit()
	re.SetInitialValue("0 0 * * MON") // "Weekly · Monday"
	re.MovePartRight()                // value part (Monday) active

	row, styles := drawRecurrence(t, re, 30, true)
	if !strings.HasPrefix(row, "Weekly · Monday") {
		t.Fatalf("recurrence not drawn at col 0: %q", row)
	}
	_, activeStyle := highlightStyles()
	bandBgActive := decomposeBg(activeStyle)

	// value part "Monday" occupies rune cells [9,14]; those cells must carry the
	// band, and the frequency/separator cells (0-8) must not — exact ranges guard
	// against the byte-vs-rune off-by-one a multi-byte separator can introduce.
	for col := 0; col <= 14; col++ {
		_, bg, _ := styles[col].Decompose()
		inValue := col >= 9 && col <= 14
		if inValue && bg != bandBgActive {
			t.Errorf("col %d (value part) bg=%v, want band %v", col, bg, bandBgActive)
		}
		if !inValue && bg == bandBgActive {
			t.Errorf("col %d (freq/sep) unexpectedly banded", col)
		}
	}
}

// decomposeBg extracts the background color from a style for comparison.
func decomposeBg(s tcell.Style) tcell.Color {
	_, bg, _ := s.Decompose()
	return bg
}
