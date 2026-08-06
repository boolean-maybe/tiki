package component

import (
	"time"

	"github.com/boolean-maybe/tiki/theme"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// timeNow is the clock used for seeding an empty field. It is a package var so
// tests can pin it deterministically.
var timeNow = time.Now

// segmentKind identifies one editable component of a date/time value. It is the
// unit of the segmented editor: navigation, cycling, typing, and highlighting
// all operate on the active segment's kind, never a positional index — so a
// widget exposing {year, month, day} and one exposing {year..minute} share the
// same core with different specs.
type segmentKind int

const (
	segYear segmentKind = iota
	segMonth
	segDay
	segHour
	segMinute
)

// bounds returns the inclusive [min, max] a segment kind may hold. Year is
// non-wrapping (cycling clamps to a sane floor); the day's upper bound is a
// coarse 31 and is further clamped to the month's real maximum on rebuild.
func (k segmentKind) bounds() (int, int) {
	switch k {
	case segMonth:
		return 1, 12
	case segDay:
		return 1, 31
	case segHour:
		return 0, 23
	case segMinute:
		return 0, 59
	default: // segYear
		return 1, 9999
	}
}

// digitWidth is the number of digits a segment kind holds (year=4, others=2).
func (k segmentKind) digitWidth() int {
	if k == segYear {
		return 4
	}
	return 2
}

// segmentedTimeEdit is the shared core for the date-only and datetime editors.
// It renders a time.Time as a fixed-width string of digit segments separated by
// literal separators; Left/Right selects a segment, Up/Down cycles the active
// segment wrapping within its own bounds (year clamps), typing overwrites the
// active segment and auto-advances, and the day clamps to the month's maximum
// after any change so the value is always real. An empty field seeds from the
// current time (truncated to the minute) on the first arrow or digit;
// Backspace/Ctrl-U clears back to empty. The change handler fires the canonical
// formatted string (via format) — or "" when cleared — after every change.
//
// The embedded *tview.InputField supplies focus/box behavior only; the custom
// Draw is the sole renderer (the InputField never holds the value or label as
// text, which would double-paint and clip the tail — see Draw/redraw).
type segmentedTimeEdit struct {
	*tview.InputField
	segments []segmentKind          // ordered, e.g. {year,month,day} or {..minute}
	format   func(time.Time) string // canonical formatter ("" when empty)

	value                time.Time
	hasValue             bool
	activeSegment        int    // index into segments
	typedDigits          int    // digits entered into the active segment since last advance
	typedAccum           int    // raw accumulator for the active segment while typing
	label                string // focus-marker markup, painted by Draw (not the InputField)
	emptyPlaceholderText string // muted text drawn when empty ("None"/"Unknown")
	onChange             func(string)
}

// newSegmentedTimeEdit builds the core with the given segment set and formatter.
func newSegmentedTimeEdit(segments []segmentKind, format func(time.Time) string) *segmentedTimeEdit {
	inputField := tview.NewInputField()
	roles := theme.Roles()
	inputField.SetFieldBackgroundColor(roles.SurfaceCanvas().TCell())
	inputField.SetFieldTextColor(roles.TextPrimary().TCell())

	se := &segmentedTimeEdit{
		InputField: inputField,
		segments:   segments,
		format:     format,
	}
	se.redraw()
	return se
}

// setValue seeds the value/empty state and refreshes the display.
func (se *segmentedTimeEdit) setValue(t time.Time, present bool) {
	se.value = t
	se.hasValue = present
	if !present {
		se.value = time.Time{}
	}
	se.redraw()
}

// currentText returns the canonical formatted value, or "" when empty.
func (se *segmentedTimeEdit) currentText() string {
	if !se.hasValue {
		return ""
	}
	return se.format(se.value)
}

// redraw keeps the embedded InputField's text empty: the custom Draw paints both
// label and value, so letting the InputField also render them would double-paint
// with a conflicting cursor-scroll layout and clip the tail at tight widths.
func (se *segmentedTimeEdit) redraw() {
	se.SetText("")
}

// setLabel stores the label (focus-marker markup) for Draw to paint. It is NOT
// forwarded to the InputField (see redraw / Draw for why the core owns drawing).
func (se *segmentedTimeEdit) setLabel(label string) {
	se.label = label
}

// segmentValue reads a segment kind's current numeric value from the time.
func (se *segmentedTimeEdit) segmentValue(k segmentKind) int {
	switch k {
	case segYear:
		return se.value.Year()
	case segMonth:
		return int(se.value.Month())
	case segDay:
		return se.value.Day()
	case segHour:
		return se.value.Hour()
	case segMinute:
		return se.value.Minute()
	}
	return 0
}

// daysInMonth returns the number of days in the given month/year.
func daysInMonth(year int, month time.Month) int {
	// day 0 of the next month == last day of this month.
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
}

// rebuild reconstructs value from the given parts, clamping the day to the
// month's real maximum (Feb 31 -> Feb 28/29), then notifies.
func (se *segmentedTimeEdit) rebuild(year, month, day, hour, min int) {
	if maxDay := daysInMonth(year, time.Month(month)); day > maxDay {
		day = maxDay
	}
	se.value = time.Date(year, time.Month(month), day, hour, min, 0, 0, time.Local)
	se.hasValue = true
	se.redraw()
	if se.onChange != nil {
		se.onChange(se.currentText())
	}
}

// setSegment writes a value into one segment kind and rebuilds.
func (se *segmentedTimeEdit) setSegment(k segmentKind, v int) {
	y, mo, d := se.value.Year(), int(se.value.Month()), se.value.Day()
	h, mi := se.value.Hour(), se.value.Minute()
	switch k {
	case segYear:
		y = v
	case segMonth:
		mo = v
	case segDay:
		d = v
	case segHour:
		h = v
	case segMinute:
		mi = v
	}
	se.rebuild(y, mo, d, h, mi)
}

// ensureSeeded seeds an empty field from the current time truncated to minute.
func (se *segmentedTimeEdit) ensureSeeded() {
	if se.hasValue {
		return
	}
	now := timeNow()
	se.value = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
	se.hasValue = true
}

// moveSegment shifts the active segment, clamped to the valid range.
func (se *segmentedTimeEdit) moveSegment(delta int) {
	se.activeSegment += delta
	if se.activeSegment < 0 {
		se.activeSegment = 0
	}
	if last := len(se.segments) - 1; se.activeSegment > last {
		se.activeSegment = last
	}
	se.typedDigits = 0
}

// cycleSegment adjusts the active segment by delta, wrapping within its bounds
// (except year, which clamps). It seeds an empty field first.
func (se *segmentedTimeEdit) cycleSegment(delta int) {
	se.ensureSeeded()
	k := se.segments[se.activeSegment]
	lo, hi := k.bounds()
	v := se.segmentValue(k) + delta
	if k == segYear { // clamp, no wrap
		if v < lo {
			v = lo
		}
	} else {
		span := hi - lo + 1
		v = lo + ((v-lo)%span+span)%span // modulo wrap within [lo, hi]
	}
	se.setSegment(k, v)
	se.typedDigits = 0
}

// typeDigit folds a typed digit into the active segment. The raw digits are
// accumulated in typedAccum (fresh on the first digit of a segment); the value
// written is that accumulator clamped into [lo, hi], so a leading "0" never
// underflows into time.Date normalization. Auto-advances when the segment fills.
func (se *segmentedTimeEdit) typeDigit(d int) {
	se.ensureSeeded()
	k := se.segments[se.activeSegment]
	lo, hi := k.bounds()

	if se.typedDigits == 0 {
		se.typedAccum = 0
	}
	se.typedAccum = se.typedAccum*10 + d
	se.typedDigits++

	v := se.typedAccum
	if v < lo {
		v = lo
	}
	if v > hi {
		v = hi
	}
	se.setSegment(k, v) // rebuild + onChange

	if se.typedDigits >= k.digitWidth() {
		se.typedDigits = 0
		se.moveSegment(1)
	}
}

// clear resets to the empty state and notifies.
func (se *segmentedTimeEdit) clear() {
	se.value = time.Time{}
	se.hasValue = false
	se.typedDigits = 0
	se.redraw()
	if se.onChange != nil {
		se.onChange("")
	}
}

// segmentCells returns the inclusive [start, end] column offsets of the segment
// at index i within the rendered string. Segments are laid out left to right,
// each digitWidth wide, separated by one separator cell.
func (se *segmentedTimeEdit) segmentCells(i int) (int, int) {
	start := 0
	for s := 0; s < i; s++ {
		start += se.segments[s].digitWidth() + 1 // + separator
	}
	end := start + se.segments[i].digitWidth() - 1
	return start, end
}

// InputHandler routes segment navigation, cycling, typing, and clearing.
func (se *segmentedTimeEdit) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, _ func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyLeft:
			se.moveSegment(-1)
		case tcell.KeyRight:
			se.moveSegment(1)
		case tcell.KeyUp:
			se.cycleSegment(1)
		case tcell.KeyDown:
			se.cycleSegment(-1)
		case tcell.KeyRune:
			if r := event.Rune(); r >= '0' && r <= '9' {
				se.typeDigit(int(r - '0'))
			}
			// non-digits ignored
		case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete, tcell.KeyCtrlU:
			se.clear()
		}
	}
}

// placeholder is the muted text drawn when the field has no value. Each wrapper
// sets emptyPlaceholderText ("None" for date, "Unknown" for datetime) to match
// its read-only render; the default is "None".
func (se *segmentedTimeEdit) placeholder() string {
	if se.emptyPlaceholderText != "" {
		return se.emptyPlaceholderText
	}
	return "None"
}

// Draw renders the label followed by the value, highlighting the active
// segment's cells with the canonical selection band when focused and non-empty.
// It draws exactly the rendered string (label markup interpreted via
// tview.Print), preserving the reserved column footprint.
func (se *segmentedTimeEdit) Draw(screen tcell.Screen) {
	se.DrawForSubclass(screen, se)
	x, y, width, height := se.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}

	roles := theme.Roles()
	base, active := highlightStyles()

	col := x
	if se.label != "" {
		_, drawn := tview.Print(screen, se.label, col, y, width, tview.AlignLeft, roles.TextPrimary().TCell())
		col += drawn
	}

	text := se.currentText()
	if text == "" {
		muted := tcell.StyleDefault.Foreground(roles.TextMuted().TCell()).
			Background(roles.SurfaceCanvas().TCell())
		for _, ch := range se.placeholder() {
			if col >= x+width {
				return
			}
			screen.SetContent(col, y, ch, nil, muted)
			col++
		}
		return
	}

	lo, hi := se.segmentCells(se.activeSegment)
	drawHighlightedText(screen, col, y, x+width, text, lo, hi, se.HasFocus(), base, active)
}

// drawHighlightedText paints text starting at (col,y), clipped at maxX, in the
// base style — except runes in the inclusive [lo,hi] index range, which use the
// active (selection-band) style when highlight is true. Shared by the segmented
// date/datetime editor and the recurrence editor so both mark their active part
// with the identical selection band. A [lo,hi] of {-1,-1} highlights nothing.
func drawHighlightedText(screen tcell.Screen, col, y, maxX int, text string, lo, hi int, highlight bool, base, active tcell.Style) {
	// iterate by RUNE index (not the byte index `range` yields): lo/hi come from
	// rune-based helpers (segmentCells/partCells), and a multi-byte separator
	// (e.g. "·") would otherwise shift the band one cell per multi-byte rune.
	runeIdx := 0
	for _, ch := range text {
		if col >= maxX {
			return
		}
		style := base
		if highlight && runeIdx >= lo && runeIdx <= hi {
			style = active
		}
		screen.SetContent(col, y, ch, nil, style)
		col++
		runeIdx++
	}
}

// highlightStyles returns the base and active (selection-band) styles shared by
// the segmented and recurrence editors: primary text on the canvas, and primary
// text on the SurfaceSelection band for the active part.
func highlightStyles() (base, active tcell.Style) {
	roles := theme.Roles()
	base = tcell.StyleDefault.
		Foreground(roles.TextPrimary().TCell()).
		Background(roles.SurfaceCanvas().TCell())
	active = tcell.StyleDefault.
		Foreground(roles.TextPrimary().TCell()).
		Background(roles.SurfaceSelection().TCell())
	return base, active
}
