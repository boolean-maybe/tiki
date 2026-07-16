package tikidetail

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/view/markdown"
)

// progressSink records whether progress is currently active, so tests can
// assert the render bar starts and ends cleared.
type progressSink struct {
	mu     sync.Mutex
	active bool
}

func (s *progressSink) SetProgress(int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = true
}

func (s *progressSink) ClearProgress() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
}

func (s *progressSink) isActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, image.Black)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

// asyncRedrawQueue mimics tview QueueUpdateDraw: FIFO closures on one goroutine.
func asyncRedrawQueue() (redraw func(func()), settle func()) {
	q := make(chan func(), 512)
	go func() {
		for fn := range q {
			fn()
		}
	}()
	redraw = func(fn func()) { q <- fn }
	settle = func() {
		b := make(chan struct{})
		q <- func() { close(b) }
		<-b
	}
	return redraw, settle
}

// waitCleared drains the hub pump and settles the redraw queue each poll, so
// any Done enqueued by the render goroutine is fully applied before the sink is
// checked, then reports whether progress ended inactive. Draining the pump
// (not just sleeping) removes the timing window where the goroutine's Done has
// not yet been processed. Bounded so a genuine stuck bar still fails.
func waitCleared(hub *model.ProgressHub, sink *progressSink, settle func()) bool {
	for i := 0; i < 500; i++ {
		hub.DrainForTest() // process any queued events (incl. the goroutine's Done)
		settle()           // run any redraws those events enqueued
		if !sink.isActive() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

// TestConfigurableDetailView_RenderDescriptionClears drives the real
// renderDescription path end-to-end with an async (queued) redraw like tview's
// QueueUpdateDraw: the indeterminate "rendering" bar must always end cleared,
// never stuck. Repeated to catch ordering races between the render redraw and
// the deferred Done.
func TestConfigurableDetailView_RenderDescriptionClears(t *testing.T) {
	dir := t.TempDir()
	names := []string{"a.png", "b.png", "c.png", "d.png"}
	md := ""
	for _, n := range names {
		writeTestPNG(t, filepath.Join(dir, n))
		md += "![" + n + "](" + n + ")\n\n"
	}
	sourceFile := filepath.Join(dir, "doc.md")

	for iter := 0; iter < 50; iter++ {
		sink := &progressSink{}
		redraw, settle := asyncRedrawQueue()
		hub := model.NewProgressHub(sink, redraw)

		resolver := nav.NewImageResolver([]string{dir})
		im := navtview.NewImageManager(resolver, 8, 16)
		navMD := markdown.NewNavigableMarkdown(markdown.NavigableMarkdownConfig{
			SearchRoots:  []string{dir},
			ImageManager: im,
		})

		cv := &ConfigurableDetailView{progressHub: hub, redraw: redraw}
		cv.imageManager = im
		cv.navMarkdown = navMD

		cv.renderDescription(md, sourceFile)

		// wait for the render goroutine's Done→ClearProgress to reach the sink,
		// then tear down. Stopping the hub before Done lands would drop it
		// (done() selects on h.stop) — shutdown-only in production, but the test
		// must let the render finish first.
		cleared := waitCleared(hub, sink, settle)
		hub.Stop()
		navMD.Close()

		if !cleared {
			t.Fatalf("iter %d: render bar stuck active (not cleared within deadline)", iter)
		}
	}
}
