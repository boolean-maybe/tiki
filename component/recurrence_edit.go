package component

import (
	"strconv"

	"github.com/boolean-maybe/ruki/recurrence"
	"github.com/boolean-maybe/tiki/theme"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RecurrenceEdit is a single-line recurrence editor with two logical parts.
// Part 1 (frequency): None, Daily, Weekly, Monthly — cycled with Up/Down.
// Part 2 (value): weekday for Weekly, day 1-31 for Monthly — cycled with Up/Down.
// Left/Right switches which part Up/Down controls. The display shows both parts
// with ">" marking the active part (e.g. "Weekly > Monday").
type RecurrenceEdit struct {
	*tview.InputField

	frequencies []string
	weekdays    []string
	freqIndex   int    // index into frequencies
	weekdayIdx  int    // index into weekdays
	day         int    // 1-31 for monthly
	activePart  int    // 0=frequency, 1=value
	label       string // focus-marker markup, painted by Draw (not the InputField)
	onChange    func(string)
}

// NewRecurrenceEdit creates a new recurrence editor.
func NewRecurrenceEdit() *RecurrenceEdit {
	inputField := tview.NewInputField()
	roles := theme.Roles()
	inputField.SetFieldBackgroundColor(roles.SurfaceCanvas().TCell())
	inputField.SetFieldTextColor(roles.TextPrimary().TCell())

	re := &RecurrenceEdit{
		InputField:  inputField,
		frequencies: recurrence.AllFrequencies(),
		weekdays:    recurrence.AllWeekdays(),
		freqIndex:   0, // None
		weekdayIdx:  0, // Monday
		day:         1,
	}

	re.updateDisplay()
	return re
}

// SetLabel stores the focus-marker label for Draw to paint. It is NOT forwarded
// to the embedded InputField: the custom Draw is the sole renderer (same reason
// as segmentedTimeEdit — avoids double-painting the label).
func (re *RecurrenceEdit) SetLabel(label string) *RecurrenceEdit {
	re.label = label
	return re
}

// SetChangeHandler sets the callback that fires on any value change.
func (re *RecurrenceEdit) SetChangeHandler(handler func(string)) *RecurrenceEdit {
	re.onChange = handler
	return re
}

// SetInitialValue populates both parts from a cron string.
func (re *RecurrenceEdit) SetInitialValue(cron string) *RecurrenceEdit {
	r := recurrence.Recurrence(cron)
	freq := recurrence.FrequencyFromRecurrence(r)

	re.freqIndex = 0
	for i, f := range re.frequencies {
		if f == string(freq) {
			re.freqIndex = i
			break
		}
	}

	switch freq {
	case recurrence.FrequencyWeekly:
		if day, ok := recurrence.WeekdayFromRecurrence(r); ok {
			for i, w := range re.weekdays {
				if w == day {
					re.weekdayIdx = i
					break
				}
			}
		}
	case recurrence.FrequencyMonthly:
		if d, ok := recurrence.DayOfMonthFromRecurrence(r); ok {
			re.day = d
		}
	}

	re.activePart = 0
	re.updateDisplay()
	return re
}

// GetValue assembles the current cron expression from the editor state.
func (re *RecurrenceEdit) GetValue() string {
	freq := recurrence.RecurrenceFrequency(re.frequencies[re.freqIndex])

	switch freq {
	case recurrence.FrequencyDaily:
		return string(recurrence.RecurrenceDaily)
	case recurrence.FrequencyWeekly:
		return string(recurrence.WeeklyRecurrence(re.weekdays[re.weekdayIdx]))
	case recurrence.FrequencyMonthly:
		return string(recurrence.MonthlyRecurrence(re.day))
	default:
		return string(recurrence.RecurrenceNone)
	}
}

// InputHandler handles keyboard input for the recurrence editor.
func (re *RecurrenceEdit) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, _ func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyLeft:
			re.activePart = 0
			re.updateDisplay()
		case tcell.KeyRight:
			if re.hasValuePart() {
				re.activePart = 1
				re.updateDisplay()
			}
		case tcell.KeyUp:
			re.cyclePrev()
		case tcell.KeyDown:
			re.cycleNext()
		}
	}
}

// CyclePrev cycles the active part's value backward.
func (re *RecurrenceEdit) CyclePrev() {
	re.cyclePrev()
}

// CycleNext cycles the active part's value forward.
func (re *RecurrenceEdit) CycleNext() {
	re.cycleNext()
}

// IsValueFocused returns true when the value part (weekday/day) is active.
func (re *RecurrenceEdit) IsValueFocused() bool {
	return re.activePart == 1
}

// MovePartLeft moves the active part to frequency (part 0).
func (re *RecurrenceEdit) MovePartLeft() {
	re.activePart = 0
	re.updateDisplay()
}

// MovePartRight moves the active part to value (part 1), if a value part exists.
func (re *RecurrenceEdit) MovePartRight() {
	if re.hasValuePart() {
		re.activePart = 1
		re.updateDisplay()
	}
}

func (re *RecurrenceEdit) cyclePrev() {
	if re.activePart == 0 {
		re.freqIndex--
		if re.freqIndex < 0 {
			re.freqIndex = len(re.frequencies) - 1
		}
		re.resetValueDefaults()
	} else {
		re.cycleValuePrev()
	}
	re.updateDisplay()
	re.emitChange()
}

func (re *RecurrenceEdit) cycleNext() {
	if re.activePart == 0 {
		re.freqIndex = (re.freqIndex + 1) % len(re.frequencies)
		re.resetValueDefaults()
	} else {
		re.cycleValueNext()
	}
	re.updateDisplay()
	re.emitChange()
}

func (re *RecurrenceEdit) cycleValuePrev() {
	freq := recurrence.RecurrenceFrequency(re.frequencies[re.freqIndex])
	switch freq {
	case recurrence.FrequencyWeekly:
		re.weekdayIdx--
		if re.weekdayIdx < 0 {
			re.weekdayIdx = len(re.weekdays) - 1
		}
	case recurrence.FrequencyMonthly:
		re.day--
		if re.day < 1 {
			re.day = 31
		}
	}
}

func (re *RecurrenceEdit) cycleValueNext() {
	freq := recurrence.RecurrenceFrequency(re.frequencies[re.freqIndex])
	switch freq {
	case recurrence.FrequencyWeekly:
		re.weekdayIdx = (re.weekdayIdx + 1) % len(re.weekdays)
	case recurrence.FrequencyMonthly:
		re.day++
		if re.day > 31 {
			re.day = 1
		}
	}
}

func (re *RecurrenceEdit) resetValueDefaults() {
	re.weekdayIdx = 0
	re.day = 1
	if !re.hasValuePart() {
		re.activePart = 0
	}
}

func (re *RecurrenceEdit) hasValuePart() bool {
	freq := recurrence.RecurrenceFrequency(re.frequencies[re.freqIndex])
	return freq == recurrence.FrequencyWeekly || freq == recurrence.FrequencyMonthly
}

// recurrenceSep is the constant separator between the frequency and value
// parts. The active part is marked by the selection-band highlight in Draw, not
// by swapping this separator (that was the old " : " / " > " glyph hack).
const recurrenceSep = " · "

// displayParts returns the frequency-part text, the value-part text (empty when
// the frequency has no value part), and whether a value part exists.
func (re *RecurrenceEdit) displayParts() (freqText, valueText string, hasValue bool) {
	freq := recurrence.RecurrenceFrequency(re.frequencies[re.freqIndex])
	switch freq {
	case recurrence.FrequencyWeekly:
		return "Weekly", re.weekdays[re.weekdayIdx], true
	case recurrence.FrequencyMonthly:
		return "Monthly", strconv.Itoa(re.day) + recurrence.OrdinalSuffix(re.day), true
	default:
		return re.frequencies[re.freqIndex], "", false
	}
}

// displayText assembles the full "Freq · Value" (or "Freq") string.
func (re *RecurrenceEdit) displayText() string {
	freqText, valueText, hasValue := re.displayParts()
	if !hasValue {
		return freqText
	}
	return freqText + recurrenceSep + valueText
}

// partCells returns the inclusive [lo, hi] rune range of the active part within
// displayText, or {-1,-1} when there is nothing to highlight (the value part is
// active but absent — which cannot normally happen since activePart resets).
func (re *RecurrenceEdit) partCells() (int, int) {
	freqText, valueText, hasValue := re.displayParts()
	if re.activePart == 0 || !hasValue {
		return 0, len([]rune(freqText)) - 1
	}
	start := len([]rune(freqText)) + len([]rune(recurrenceSep))
	return start, start + len([]rune(valueText)) - 1
}

func (re *RecurrenceEdit) updateDisplay() {
	// keep the InputField's text EMPTY: Draw is the sole renderer (it paints the
	// value with the active-part selection band). Letting the InputField also
	// hold the text would double-paint — same discipline as segmentedTimeEdit.
	re.SetText("")
}

func (re *RecurrenceEdit) emitChange() {
	if re.onChange != nil {
		re.onChange(re.GetValue())
	}
}

// Draw renders the label then the "Freq · Value" text, highlighting the active
// part with the canonical selection band when focused — the same scheme the
// segmented date/datetime editors use (via the shared highlightStyles /
// drawHighlightedText helpers). The core owns all drawing; the InputField holds
// no text (see updateDisplay).
func (re *RecurrenceEdit) Draw(screen tcell.Screen) {
	re.DrawForSubclass(screen, re)
	x, y, width, height := re.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}

	base, active := highlightStyles()

	col := x
	if re.label != "" {
		_, drawn := tview.Print(screen, re.label, col, y, width, tview.AlignLeft, theme.Roles().TextPrimary().TCell())
		col += drawn
	}

	lo, hi := re.partCells()
	drawHighlightedText(screen, col, y, x+width, re.displayText(), lo, hi, re.HasFocus(), base, active)
}
