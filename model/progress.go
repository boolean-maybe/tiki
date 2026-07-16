package model

import (
	"sync/atomic"
	"time"
)

// defaultTickInterval drives the indeterminate animation (~8 fps).
const defaultTickInterval = 120 * time.Millisecond

// ProgressEvent is a single progress update. Total == 0 means indeterminate.
type ProgressEvent struct {
	Done  int
	Total int
	Label string
}

// ProgressSink receives progress state (the statusline in production).
type ProgressSink interface {
	SetProgress(done, total int)
	ClearProgress()
}

// Reporter is the handle a consumer holds to report progress. Report is safe to
// call from any goroutine; Done is idempotent and should be deferred.
type Reporter struct {
	hub   *ProgressHub
	id    uint64
	label string
}

// Report publishes a progress update. Non-blocking: drops if the pump is busy.
func (r *Reporter) Report(done, total int) {
	if r == nil {
		return
	}
	r.hub.report(r.id, ProgressEvent{Done: done, Total: total, Label: r.label})
}

// Done marks this reporter finished. Idempotent; no-op if superseded.
func (r *Reporter) Done() {
	if r == nil {
		return
	}
	r.hub.done(r.id)
}

// hubMsg is a single unit of work handed to the pump goroutine.
type hubMsg struct {
	id      uint64
	event   ProgressEvent
	isStart bool          // reporter just started: activate as indeterminate until first Report
	isDone  bool          // reporter finished: clear the bar
	ack     chan struct{} // test-only drain barrier: closed after this msg is handled (FIFO)
}

// ProgressHub owns the single active reporter (newest wins) and one pump
// goroutine that coalesces events and pushes to the sink via redraw. The pump
// goroutine exclusively owns activeID and indeterminate — StartProgress only
// mints ids atomically and never touches pump-owned state, so the design is
// race-free without locking those fields.
type ProgressHub struct {
	sink         ProgressSink
	redraw       func(func())
	tickInterval time.Duration
	events       chan hubMsg
	stop         chan struct{}

	nextID atomic.Uint64

	// pump-goroutine-owned state (no external access)
	activeID      uint64
	doneID        uint64 // highest id that has been Done; late Reports for it are ignored
	indeterminate bool
}

// NewProgressHub starts the pump goroutine with the default tick interval.
// redraw wraps sink mutation so it runs on the UI goroutine
// (app.QueueUpdateDraw in production, a synchronous fn() in tests).
func NewProgressHub(sink ProgressSink, redraw func(func())) *ProgressHub {
	return NewProgressHubWithTick(sink, redraw, defaultTickInterval)
}

// NewProgressHubWithTick is NewProgressHub with an explicit tick interval, used
// by tests to exercise the indeterminate animation quickly.
func NewProgressHubWithTick(sink ProgressSink, redraw func(func()), tick time.Duration) *ProgressHub {
	h := &ProgressHub{
		sink:         sink,
		redraw:       redraw,
		tickInterval: tick,
		events:       make(chan hubMsg, 64),
		stop:         make(chan struct{}),
	}
	go h.pump()
	return h
}

// StartProgress supersedes any in-flight reporter and returns a fresh one. It
// announces the reporter to the pump as an indeterminate activation so a bar
// shows immediately for consumers that never call Report (markdown render, ruki
// runs). The pump adopts the newest id it sees, so minting a higher id also
// supersedes any older reporter. The start send is blocking so the activation
// is never dropped by a busy pump.
func (h *ProgressHub) StartProgress(label string) *Reporter {
	id := h.nextID.Add(1)
	select {
	case h.events <- hubMsg{id: id, isStart: true}:
	case <-h.stop:
	}
	return &Reporter{hub: h, id: id, label: label}
}

// Stop shuts the pump down.
func (h *ProgressHub) Stop() { close(h.stop) }

func (h *ProgressHub) report(id uint64, ev ProgressEvent) {
	select {
	case h.events <- hubMsg{id: id, event: ev}:
	default: // pump busy — coalescing means dropping stale updates is fine
	}
}

func (h *ProgressHub) done(id uint64) {
	select {
	case h.events <- hubMsg{id: id, isDone: true}:
	case <-h.stop:
	}
}

func (h *ProgressHub) pump() {
	ticker := time.NewTicker(h.tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-h.stop:
			return
		case msg := <-h.events:
			h.handle(msg)
		case <-ticker.C:
			h.tick()
		}
	}
}

// handle applies one message. Runs only on the pump goroutine.
func (h *ProgressHub) handle(msg hubMsg) {
	if msg.ack != nil {
		close(msg.ack) // test drain barrier: all prior FIFO events are handled
		return
	}
	if msg.id < h.activeID {
		return // superseded reporter — ignore
	}
	if msg.isStart {
		h.activeID = msg.id
		h.indeterminate = true // no total yet — show an animated bar immediately
		h.redraw(func() { h.sink.SetProgress(0, 0) })
		return
	}
	if msg.isDone {
		h.activeID = msg.id
		h.doneID = msg.id
		h.indeterminate = false
		h.redraw(func() { h.sink.ClearProgress() })
		return
	}
	// a Report can arrive after its own Done: SetMarkdownWithSource re-runs
	// image resolution on the UI thread and re-fires the still-installed
	// callback, and per-image callbacks fire from concurrent goroutines with no
	// ordering vs Done. Ignoring finished ids stops such a straggler Report from
	// re-showing a bar the Done already cleared (the "stuck at 100%" bug).
	if msg.id <= h.doneID {
		return
	}
	h.activeID = msg.id
	ev := msg.event
	h.indeterminate = ev.Total <= 0
	h.redraw(func() { h.sink.SetProgress(ev.Done, ev.Total) })
}

// tick forces a redraw while an indeterminate reporter is active so the
// statusline's animation frame advances. Runs only on the pump goroutine.
func (h *ProgressHub) tick() {
	if h.activeID == 0 || !h.indeterminate {
		return
	}
	h.redraw(func() { h.sink.SetProgress(0, 0) })
}

// DrainForTest blocks until the pump has processed all events queued before
// this call. The ack rides the events channel, so FIFO ordering guarantees
// every prior event is handled before the ack closes — a true barrier. Test-only
// helper; exported so tests in other packages (controller/tikidetail) can sync
// with the pump.
func (h *ProgressHub) DrainForTest() {
	ack := make(chan struct{})
	select {
	case h.events <- hubMsg{ack: ack}:
	case <-h.stop:
		return
	}
	<-ack
}
