package model

import (
	"sync"
	"testing"
	"time"
)

// fakeSink records the latest progress state pushed to it.
type fakeSink struct {
	mu       sync.Mutex
	done     int
	total    int
	cleared  bool
	setCalls int
}

func (s *fakeSink) SetProgress(done, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done, s.total, s.cleared = done, total, false
	s.setCalls++
}

func (s *fakeSink) ClearProgress() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleared = true
}

func (s *fakeSink) snapshot() (int, int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.done, s.total, s.cleared
}

func (s *fakeSink) setCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setCalls
}

// syncRedraw runs the update immediately (no tview queue in tests).
func syncRedraw(fn func()) { fn() }

// TestReporter_StartProgressActivatesWithoutReport guards the indeterminate
// consumers (markdown render, ruki runs) that call StartProgress + Done but
// never Report: StartProgress alone must make the sink active (an indeterminate
// bar), and Done must clear it. Without this, no bar ever shows.
func TestReporter_StartProgressActivatesWithoutReport(t *testing.T) {
	sink := &fakeSink{}
	hub := NewProgressHub(sink, syncRedraw)
	defer hub.Stop()

	r := hub.StartProgress("rendering") // no Report — indeterminate
	hub.DrainForTest()
	if sink.setCallCount() == 0 {
		t.Fatal("StartProgress did not activate the sink (no bar shown)")
	}

	r.Done()
	hub.DrainForTest()
	if _, _, cleared := sink.snapshot(); !cleared {
		t.Fatal("Done did not clear the bar")
	}
}

// TestReporter_LateReportAfterDoneIgnored guards the "stuck at 100%" bug: a
// Report for a reporter that has already been Done (e.g. SetMarkdownWithSource
// re-firing the image callback on the UI thread after the render goroutine's
// deferred Done) must NOT re-activate the bar.
func TestReporter_LateReportAfterDoneIgnored(t *testing.T) {
	sink := &fakeSink{}
	hub := NewProgressHub(sink, syncRedraw)
	defer hub.Stop()

	r := hub.StartProgress("rendering")
	r.Report(8, 8) // reaches 100%
	r.Done()       // clears
	r.Report(8, 8) // straggler re-fire on the finished reporter — must be ignored
	hub.DrainForTest()

	if _, _, cleared := sink.snapshot(); !cleared {
		t.Fatal("late Report after Done re-showed the bar (stuck at 100%)")
	}
}

func TestReporter_ReportReachesSink(t *testing.T) {
	sink := &fakeSink{}
	hub := NewProgressHub(sink, syncRedraw)
	defer hub.Stop()

	r := hub.StartProgress("rendering")
	r.Report(3, 10)
	hub.DrainForTest() // block until pump has processed pending events

	done, total, cleared := sink.snapshot()
	if done != 3 || total != 10 || cleared {
		t.Fatalf("got (%d,%d,cleared=%v), want (3,10,false)", done, total, cleared)
	}
}

func TestHub_NewestWinsSupersedesOld(t *testing.T) {
	sink := &fakeSink{}
	hub := NewProgressHub(sink, syncRedraw)
	defer hub.Stop()

	old := hub.StartProgress("first")
	newer := hub.StartProgress("second") // supersedes old
	old.Report(9, 9)                     // must be ignored
	newer.Report(1, 4)
	hub.DrainForTest()

	done, total, _ := sink.snapshot()
	if done != 1 || total != 4 {
		t.Fatalf("got (%d,%d), want (1,4) — old reporter should be ignored", done, total)
	}

	old.Done() // superseded Done must NOT clear the active bar
	hub.DrainForTest()
	if _, _, cleared := sink.snapshot(); cleared {
		t.Fatal("superseded Done() cleared the active progress")
	}
}

func TestReporter_DoneClears(t *testing.T) {
	sink := &fakeSink{}
	hub := NewProgressHub(sink, syncRedraw)
	defer hub.Stop()

	r := hub.StartProgress("x")
	r.Report(2, 5)
	r.Done()
	hub.DrainForTest()
	if _, _, cleared := sink.snapshot(); !cleared {
		t.Fatal("Done() did not clear progress")
	}
}

func TestReporter_ConcurrentReport(t *testing.T) {
	sink := &fakeSink{}
	hub := NewProgressHub(sink, syncRedraw)
	defer hub.Stop()

	r := hub.StartProgress("x")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) { defer wg.Done(); r.Report(n, 50) }(i)
	}
	wg.Wait()
	hub.DrainForTest()
	// no assertion on exact value (coalescing); test passes if -race is clean
}

func TestHub_IndeterminateTicks(t *testing.T) {
	var mu sync.Mutex
	redraws := 0
	countingRedraw := func(fn func()) { mu.Lock(); redraws++; mu.Unlock(); fn() }

	sink := &fakeSink{}
	hub := NewProgressHubWithTick(sink, countingRedraw, 20*time.Millisecond)
	defer hub.Stop()

	r := hub.StartProgress("running query")
	r.Report(0, 0) // indeterminate
	time.Sleep(100 * time.Millisecond)
	r.Done()

	mu.Lock()
	got := redraws
	mu.Unlock()
	if got < 3 {
		t.Fatalf("expected several ticked redraws for indeterminate, got %d", got)
	}
}

func TestHub_DeterminateDoesNotTick(t *testing.T) {
	var mu sync.Mutex
	redraws := 0
	countingRedraw := func(fn func()) { mu.Lock(); redraws++; mu.Unlock(); fn() }

	sink := &fakeSink{}
	hub := NewProgressHubWithTick(sink, countingRedraw, 20*time.Millisecond)
	defer hub.Stop()

	r := hub.StartProgress("rendering")
	r.Report(1, 10) // determinate — one redraw, then no ticks
	hub.DrainForTest()
	time.Sleep(100 * time.Millisecond)
	r.Done()
	hub.DrainForTest()

	mu.Lock()
	got := redraws
	mu.Unlock()
	// three legitimate redraws: StartProgress activation, the Report, and Done.
	// the ticker must NOT fire for a determinate bar (would add ~5 over 100ms
	// at 20ms), so anything beyond 3 means the ticker leaked.
	if got > 3 {
		t.Fatalf("determinate progress should not tick; got %d redraws, want <= 3", got)
	}
}
