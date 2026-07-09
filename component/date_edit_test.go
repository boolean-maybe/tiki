package component

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// pressDate drives a DateEdit's core.
func pressDate(de *DateEdit, key tcell.Key, r rune) {
	pressCore(de.segmentedTimeEdit, key, r)
}

func TestDateEdit_InitialValueRoundTrip(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2026-07-08")
	if got := de.GetCurrentText(); got != "2026-07-08" {
		t.Errorf("GetCurrentText = %q, want 2026-07-08", got)
	}
}

func TestDateEdit_EmptyInitialValue(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("")
	if got := de.GetCurrentText(); got != "" {
		t.Errorf("GetCurrentText = %q, want empty", got)
	}
}

func TestDateEdit_InvalidInitialValueIsEmpty(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2026-07-08 14:30") // datetime string is not a valid date-only value
	if got := de.GetCurrentText(); got != "" {
		t.Errorf("GetCurrentText = %q, want empty (invalid date-only seed ignored)", got)
	}
}

// TestDateEdit_DatePartMatchesDateTime is the contract this refactor exists to
// guarantee: the date-only editor's segment behaviour is identical to the
// datetime editor's date part. Same navigation, same within-segment wrap, same
// day clamp — verified by driving identical key sequences and comparing the
// date portion.
func TestDateEdit_DatePartMatchesDateTime(t *testing.T) {
	d := NewDateEdit()
	dt := NewDateTimeEdit()
	d.SetInitialValue("2026-12-31")
	dt.SetInitialValue("2026-12-31 00:00")

	// move to month, cycle up: both wrap Dec->Jan with year unchanged.
	pressDate(d, tcell.KeyRight, 0)
	press(dt, tcell.KeyRight, 0)
	pressDate(d, tcell.KeyUp, 0)
	press(dt, tcell.KeyUp, 0)

	if d.GetCurrentText() != "2026-01-31" {
		t.Errorf("date month wrap: got %q, want 2026-01-31", d.GetCurrentText())
	}
	if dt.GetCurrentText()[:10] != d.GetCurrentText() {
		t.Errorf("date part diverged: date=%q datetime=%q", d.GetCurrentText(), dt.GetCurrentText()[:10])
	}
}

func TestDateEdit_MonthWrapsWithinSegment(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2026-12-15")
	pressDate(de, tcell.KeyRight, 0) // month
	pressDate(de, tcell.KeyUp, 0)    // 12 -> 1, year unchanged
	if got := de.GetCurrentText(); got != "2026-01-15" {
		t.Errorf("month wrap: got %q, want 2026-01-15", got)
	}
}

func TestDateEdit_DayClampsOnMonthChange(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2026-01-31") // Jan 31, non-leap year 2026
	pressDate(de, tcell.KeyRight, 0) // month
	pressDate(de, tcell.KeyUp, 0)    // Jan -> Feb; day 31 clamps to 28
	if got := de.GetCurrentText(); got != "2026-02-28" {
		t.Errorf("day clamp: got %q, want 2026-02-28", got)
	}
}

func TestDateEdit_EmptySeedsFromTodayOnArrow(t *testing.T) {
	testWithFixedNow(t, time.Date(2026, 7, 8, 9, 15, 30, 0, time.Local))
	de := NewDateEdit()
	de.SetInitialValue("")
	pressDate(de, tcell.KeyUp, 0)
	if de.GetCurrentText() == "" {
		t.Fatal("expected a seeded value after first arrow, got empty")
	}
}

func TestDateEdit_TypeOverwritesSegmentAndAdvances(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2026-06-15")
	pressDate(de, tcell.KeyRight, 0) // month
	pressDate(de, tcell.KeyRune, '0')
	pressDate(de, tcell.KeyRune, '9') // month = 09, auto-advance to day
	if got := de.GetCurrentText(); got != "2026-09-15" {
		t.Errorf("typed month: got %q, want 2026-09-15", got)
	}
	if de.activeSegment != 2 {
		t.Errorf("auto-advance: activeSegment = %d, want 2 (day)", de.activeSegment)
	}
}

func TestDateEdit_SegmentNavigationClampsAtEnds(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2026-06-15")
	pressDate(de, tcell.KeyLeft, 0) // already at year -> stays 0
	if de.activeSegment != 0 {
		t.Errorf("left at start: activeSegment = %d, want 0", de.activeSegment)
	}
	last := len(de.segments) - 1 // day (index 2)
	for i := 0; i < 10; i++ {
		pressDate(de, tcell.KeyRight, 0)
	}
	if de.activeSegment != last {
		t.Errorf("right at end: activeSegment = %d, want %d", de.activeSegment, last)
	}
}

func TestDateEdit_ClearsToEmpty(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2026-06-15")
	var last string
	fired := 0
	de.SetChangeHandler(func(s string) { last = s; fired++ })
	pressDate(de, tcell.KeyBackspace, 0)
	if de.GetCurrentText() != "" {
		t.Errorf("after clear: GetCurrentText = %q, want empty", de.GetCurrentText())
	}
	if fired == 0 || last != "" {
		t.Errorf("clear should fire onChange(\"\"); fired=%d last=%q", fired, last)
	}
}

func TestDateEdit_SegmentCells(t *testing.T) {
	// "2026-07-08"
	//  0123456789
	cases := []struct{ seg, wantLo, wantHi int }{
		{0, 0, 3}, // year
		{1, 5, 6}, // month
		{2, 8, 9}, // day
	}
	de := NewDateEdit()
	for _, c := range cases {
		lo, hi := de.segmentCells(c.seg)
		if lo != c.wantLo || hi != c.wantHi {
			t.Errorf("segmentCells(%d) = (%d,%d), want (%d,%d)", c.seg, lo, hi, c.wantLo, c.wantHi)
		}
	}
}

func TestDateEdit_DrawShowsValue(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("2026-07-08")
	row, _ := drawToScreen(t, de.segmentedTimeEdit, 15, false)
	if !strings.Contains(row, "2026-07-08") {
		t.Errorf("drawn row = %q, want it to contain the date", row)
	}
}

func TestDateEdit_DrawShowsEmptyPlaceholder(t *testing.T) {
	de := NewDateEdit()
	de.SetInitialValue("")
	row, _ := drawToScreen(t, de.segmentedTimeEdit, 15, false)
	if !strings.Contains(row, "None") {
		t.Errorf("drawn empty row = %q, want it to contain None", row)
	}
}

func TestDateEdit_FluentAPI(t *testing.T) {
	de := NewDateEdit().SetLabel("x").SetInitialValue("2026-07-08")
	de.SetChangeHandler(func(string) {})
	if de.GetCurrentText() != "2026-07-08" {
		t.Errorf("fluent chain lost value: %q", de.GetCurrentText())
	}
}
