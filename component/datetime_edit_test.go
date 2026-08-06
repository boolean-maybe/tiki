package component

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// press sends a key event through the widget's InputHandler.
func press(de *DateTimeEdit, key tcell.Key, r rune) {
	pressCore(de.segmentedTimeEdit, key, r)
}

// pressCore drives any segmented editor core directly.
func pressCore(se *segmentedTimeEdit, key tcell.Key, r rune) {
	ev := tcell.NewEventKey(key, r, tcell.ModNone)
	se.InputHandler()(ev, func(tview.Primitive) {})
}

func TestDateTimeEdit_InitialValueRoundTrip(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("2026-07-08 14:30")
	if got := de.GetCurrentText(); got != "2026-07-08 14:30" {
		t.Errorf("GetCurrentText = %q, want 2026-07-08 14:30", got)
	}
}

func TestDateTimeEdit_EmptyInitialValue(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("")
	if got := de.GetCurrentText(); got != "" {
		t.Errorf("GetCurrentText = %q, want empty", got)
	}
}

func TestDateTimeEdit_InvalidInitialValueIsEmpty(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("garbage")
	if got := de.GetCurrentText(); got != "" {
		t.Errorf("GetCurrentText = %q, want empty (invalid seed ignored)", got)
	}
}

// testWithFixedNow pins timeNow for deterministic seed tests.
func testWithFixedNow(t *testing.T, fixed time.Time) {
	t.Helper()
	prev := timeNow
	timeNow = func() time.Time { return fixed }
	t.Cleanup(func() { timeNow = prev })
}

func TestDateTimeEdit_EmptySeedsFromNowOnArrow(t *testing.T) {
	testWithFixedNow(t, time.Date(2026, 7, 8, 9, 15, 30, 0, time.Local))
	de := NewDateTimeEdit()
	de.SetInitialValue("")
	press(de, tcell.KeyDown, 0) // day segment default (index 0 = year); still seeds
	if de.GetCurrentText() == "" {
		t.Fatal("expected a seeded value after first arrow, got empty")
	}
	// seed is truncated to the minute (no seconds)
	if de.value.Second() != 0 {
		t.Errorf("seed not truncated to minute: %v", de.value)
	}
}

func TestDateTimeEdit_UpDownCyclesActiveSegmentWrapping(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("2026-12-31 23:59")
	// active segment defaults to year (0). Move to month (1).
	press(de, tcell.KeyRight, 0)
	press(de, tcell.KeyUp, 0) // month 12 -> wrap to 1, year unchanged
	got := de.GetCurrentText()
	if got[:7] != "2026-01" {
		t.Errorf("month wrap: got %q, want prefix 2026-01", got)
	}
}

func TestDateTimeEdit_MinuteWrapsWithoutCarry(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("2026-06-15 10:59")
	// move to minute segment (index 4): Right x4
	for i := 0; i < 4; i++ {
		press(de, tcell.KeyRight, 0)
	}
	press(de, tcell.KeyUp, 0) // 59 -> 00, hour stays 10
	if got := de.GetCurrentText(); got != "2026-06-15 10:00" {
		t.Errorf("minute wrap: got %q, want 2026-06-15 10:00", got)
	}
}

func TestDateTimeEdit_DayClampsOnMonthChange(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("2026-01-31 08:00") // Jan 31, non-leap year 2026
	press(de, tcell.KeyRight, 0)           // to month
	press(de, tcell.KeyUp, 0)              // Jan -> Feb; day 31 clamps to 28
	if got := de.GetCurrentText(); got != "2026-02-28 08:00" {
		t.Errorf("day clamp: got %q, want 2026-02-28 08:00", got)
	}
}

func TestDateTimeEdit_SegmentNavigationClampsAtEnds(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("2026-06-15 10:30")
	press(de, tcell.KeyLeft, 0) // already at year(0) -> stays 0
	if de.activeSegment != 0 {
		t.Errorf("left at start: activeSegment = %d, want 0", de.activeSegment)
	}
	last := len(de.segments) - 1
	for i := 0; i < 10; i++ {
		press(de, tcell.KeyRight, 0) // clamps at minute(4)
	}
	if de.activeSegment != last {
		t.Errorf("right at end: activeSegment = %d, want %d", de.activeSegment, last)
	}
}

func TestDateTimeEdit_ChangeHandlerFires(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("2026-06-15 10:30")
	var last string
	fired := 0
	de.SetChangeHandler(func(s string) { last = s; fired++ })
	press(de, tcell.KeyRight, 0)
	press(de, tcell.KeyUp, 0) // month 6 -> 7
	if fired == 0 {
		t.Fatal("change handler never fired")
	}
	if last[:7] != "2026-07" {
		t.Errorf("handler value = %q, want prefix 2026-07", last)
	}
}

func TestDateTimeEdit_TypeOverwritesSegmentAndAdvances(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("2026-06-15 10:30")
	press(de, tcell.KeyRight, 0) // to month segment (index 1)
	press(de, tcell.KeyRune, '0')
	press(de, tcell.KeyRune, '9') // month = 09, auto-advance to day
	if got := de.GetCurrentText(); got[:7] != "2026-09" {
		t.Errorf("typed month: got %q, want prefix 2026-09", got)
	}
	if de.activeSegment != 2 {
		t.Errorf("auto-advance: activeSegment = %d, want 2 (day)", de.activeSegment)
	}
}

func TestDateTimeEdit_TypeClampsToSegmentMax(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("2026-06-15 10:30")
	press(de, tcell.KeyRight, 0) // month
	press(de, tcell.KeyRune, '9')
	press(de, tcell.KeyRune, '9') // 99 -> clamp to 12
	if got := de.GetCurrentText(); got[:7] != "2026-12" {
		t.Errorf("typed clamp: got %q, want prefix 2026-12", got)
	}
}

func TestDateTimeEdit_TypeSeedsEmpty(t *testing.T) {
	testWithFixedNow(t, time.Date(2026, 7, 8, 9, 15, 0, 0, time.Local))
	de := NewDateTimeEdit()
	de.SetInitialValue("")
	press(de, tcell.KeyRune, '2') // typing into year seeds then overwrites
	if de.GetCurrentText() == "" {
		t.Fatal("typing into empty field should seed a value")
	}
}

func TestDateTimeEdit_NonDigitIgnored(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("2026-06-15 10:30")
	press(de, tcell.KeyRune, 'x')
	if got := de.GetCurrentText(); got != "2026-06-15 10:30" {
		t.Errorf("non-digit changed value: got %q", got)
	}
}

func TestDateTimeEdit_SegmentCells(t *testing.T) {
	// "2026-07-08 14:30"
	//  0123456789012345
	cases := []struct {
		seg            int
		wantLo, wantHi int
	}{
		{0, 0, 3},   // year
		{1, 5, 6},   // month
		{2, 8, 9},   // day
		{3, 11, 12}, // hour
		{4, 14, 15}, // minute
	}
	de := NewDateTimeEdit()
	for _, c := range cases {
		lo, hi := de.segmentCells(c.seg)
		if lo != c.wantLo || hi != c.wantHi {
			t.Errorf("segmentCells(%d) = (%d,%d), want (%d,%d)", c.seg, lo, hi, c.wantLo, c.wantHi)
		}
	}
}

// drawToScreen renders the widget onto a simulation screen and returns the
// visible rune row plus the tcell styles per column, so a render test can
// assert both the drawn text and which cells carry the active-segment highlight.
func drawToScreen(t *testing.T, se *segmentedTimeEdit, width int, focused bool) (string, []tcell.Style) {
	t.Helper()
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(width, 1)
	se.SetRect(0, 0, width, 1)
	if focused {
		se.Focus(func(tview.Primitive) {})
	}
	se.Draw(screen)
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

func TestDateTimeEdit_DrawShowsValue(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("2026-07-08 14:30")
	row, _ := drawToScreen(t, de.segmentedTimeEdit, 20, false)
	if !strings.Contains(row, "2026-07-08 14:30") {
		t.Errorf("drawn row = %q, want it to contain the datetime", row)
	}
}

func TestDateTimeEdit_DrawShowsEmptyPlaceholder(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("")
	row, _ := drawToScreen(t, de.segmentedTimeEdit, 20, false)
	if !strings.Contains(row, "Unknown") {
		t.Errorf("drawn empty row = %q, want it to contain Unknown", row)
	}
}

func TestDateTimeEdit_DrawHighlightsActiveSegmentWhenFocused(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetInitialValue("2026-07-08 14:30")
	press(de, tcell.KeyRight, 0) // active segment = month (cells 5-6)

	row, styles := drawToScreen(t, de.segmentedTimeEdit, 20, true)
	idx := strings.Index(row, "2026-07-08 14:30")
	if idx < 0 {
		t.Fatalf("datetime not drawn: %q", row)
	}
	// month segment occupies text cells 5-6 → screen cols idx+5, idx+6.
	monthFg, monthBg, _ := styles[idx+5].Decompose()
	// a non-month cell (year, col idx+0) must differ from the highlighted month.
	yearFg, yearBg, _ := styles[idx+0].Decompose()
	if monthFg == yearFg && monthBg == yearBg {
		t.Errorf("active month segment not visually distinct from year segment (both fg=%v bg=%v)", monthFg, monthBg)
	}
}

// TestDateTimeEdit_LabelMarkupNotDrawnLiterally reproduces the bug where a
// focus-marker label carrying tview color-tag markup ("[#ffff00]► [-]") was
// painted rune-by-rune, so the literal "[", "#", "f"… showed on screen instead
// of a colored arrow. The visible row must not contain a raw markup bracket.
func TestDateTimeEdit_LabelMarkupNotDrawnLiterally(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetLabel("[#ffff00]► [-:-:-]") // tview color-tag markup, like getFocusMarker
	de.SetInitialValue("2026-07-08 14:30")
	row, _ := drawToScreen(t, de.segmentedTimeEdit, 30, true)
	if strings.Contains(row, "[#") || strings.Contains(row, "[-") {
		t.Errorf("label markup drawn literally: row = %q", row)
	}
	if !strings.Contains(row, "2026-07-08 14:30") {
		t.Errorf("value missing after label: row = %q", row)
	}
}

// TestDateTimeEdit_MarkerPlusValueFitsReservedWidth pins that with the focus
// marker (2 visible cells) plus the 16-cell datetime, an 18-cell inner width
// draws the full value with no clip — mirroring the grid's reserved width
// (value measure + editFocusMarkerReserve).
func TestDateTimeEdit_MarkerPlusValueFitsReservedWidth(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetLabel("[#ffff00]► [-:-:-]")
	de.SetInitialValue("2026-07-08 14:30")
	row, _ := drawToScreen(t, de.segmentedTimeEdit, 18, true) // 2 (marker) + 16 (value)
	if !strings.Contains(row, "2026-07-08 14:30") {
		t.Errorf("value clipped at reserved width: row = %q", row)
	}
}

// TestDateTimeEdit_FullValueAtReservedColumnWidth reproduces the minute-clip
// seen in the bug-tracker Due editor: at the grid-reserved column width (value
// 16 + breathing 1 + marker 2 = 19) the focused editor must draw the FULL
// "YYYY-MM-DD HH:MM" with the marker, not clip the trailing minutes.
func TestDateTimeEdit_FullValueAtReservedColumnWidth(t *testing.T) {
	de := NewDateTimeEdit()
	de.SetLabel("[#ffff00]► [-:-:-]")
	de.SetInitialValue("2027-07-08 18:45")
	row, _ := drawToScreen(t, de.segmentedTimeEdit, 19, true)
	if !strings.Contains(row, "2027-07-08 18:45") {
		t.Errorf("minute segment clipped at reserved width 19: row = %q", row)
	}
}
