package component

import (
	"fmt"
	"strconv"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"

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
	freqIndex   int // index into frequencies
	weekdayIdx  int // index into weekdays
	day         int // 1-31 for monthly
	activePart  int // 0=frequency, 1=value
	onChange    func(string)
}

// NewRecurrenceEdit creates a new recurrence editor.
func NewRecurrenceEdit() *RecurrenceEdit {
	inputField := tview.NewInputField()
	inputField.SetFieldBackgroundColor(config.GetContentBackgroundColor())
	inputField.SetFieldTextColor(config.GetContentTextColor())

	re := &RecurrenceEdit{
		InputField:  inputField,
		frequencies: taskpkg.AllFrequencies(),
		weekdays:    taskpkg.AllWeekdays(),
		freqIndex:   0, // None
		weekdayIdx:  0, // Monday
		day:         1,
	}

	re.updateDisplay()
	return re
}

// SetLabel sets the label displayed before the input field.
func (re *RecurrenceEdit) SetLabel(label string) *RecurrenceEdit {
	re.InputField.SetLabel(label)
	return re
}

// SetChangeHandler sets the callback that fires on any value change.
func (re *RecurrenceEdit) SetChangeHandler(handler func(string)) *RecurrenceEdit {
	re.onChange = handler
	return re
}

// SetInitialValue populates both parts from a cron string.
func (re *RecurrenceEdit) SetInitialValue(cron string) *RecurrenceEdit {
	r := taskpkg.Recurrence(cron)
	freq := taskpkg.FrequencyFromRecurrence(r)

	re.freqIndex = 0
	for i, f := range re.frequencies {
		if f == string(freq) {
			re.freqIndex = i
			break
		}
	}

	switch freq {
	case taskpkg.FrequencyWeekly:
		if day, ok := taskpkg.WeekdayFromRecurrence(r); ok {
			for i, w := range re.weekdays {
				if w == day {
					re.weekdayIdx = i
					break
				}
			}
		}
	case taskpkg.FrequencyMonthly:
		if d, ok := taskpkg.DayOfMonthFromRecurrence(r); ok {
			re.day = d
		}
	}

	re.activePart = 0
	re.updateDisplay()
	return re
}

// GetValue assembles the current cron expression from the editor state.
func (re *RecurrenceEdit) GetValue() string {
	freq := taskpkg.RecurrenceFrequency(re.frequencies[re.freqIndex])

	switch freq {
	case taskpkg.FrequencyDaily:
		return string(taskpkg.RecurrenceDaily)
	case taskpkg.FrequencyWeekly:
		return string(taskpkg.WeeklyRecurrence(re.weekdays[re.weekdayIdx]))
	case taskpkg.FrequencyMonthly:
		return string(taskpkg.MonthlyRecurrence(re.day))
	default:
		return string(taskpkg.RecurrenceNone)
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
	freq := taskpkg.RecurrenceFrequency(re.frequencies[re.freqIndex])
	switch freq {
	case taskpkg.FrequencyWeekly:
		re.weekdayIdx--
		if re.weekdayIdx < 0 {
			re.weekdayIdx = len(re.weekdays) - 1
		}
	case taskpkg.FrequencyMonthly:
		re.day--
		if re.day < 1 {
			re.day = 31
		}
	}
}

func (re *RecurrenceEdit) cycleValueNext() {
	freq := taskpkg.RecurrenceFrequency(re.frequencies[re.freqIndex])
	switch freq {
	case taskpkg.FrequencyWeekly:
		re.weekdayIdx = (re.weekdayIdx + 1) % len(re.weekdays)
	case taskpkg.FrequencyMonthly:
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
	freq := taskpkg.RecurrenceFrequency(re.frequencies[re.freqIndex])
	return freq == taskpkg.FrequencyWeekly || freq == taskpkg.FrequencyMonthly
}

func (re *RecurrenceEdit) updateDisplay() {
	freq := taskpkg.RecurrenceFrequency(re.frequencies[re.freqIndex])
	sep := " : "
	if re.activePart == 1 {
		sep = " > "
	}

	var text string
	switch freq {
	case taskpkg.FrequencyWeekly:
		text = fmt.Sprintf("Weekly%s%s", sep, re.weekdays[re.weekdayIdx])
	case taskpkg.FrequencyMonthly:
		text = fmt.Sprintf("Monthly%s%s", sep, strconv.Itoa(re.day)+taskpkg.OrdinalSuffix(re.day))
	default:
		text = re.frequencies[re.freqIndex]
	}

	re.SetText(text)
}

func (re *RecurrenceEdit) emitChange() {
	if re.onChange != nil {
		re.onChange(re.GetValue())
	}
}
