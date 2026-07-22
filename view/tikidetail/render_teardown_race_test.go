package tikidetail

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/view/markdown"
)

// TestConfigurableDetailView_RenderDescriptionAfterTeardown guards against the
// nil-pointer panic seen when a tiki is opened and then quickly navigated away
// (e.g. pressing 'n' to open a new tiki then Enter): renderDescription launches
// an off-UI-goroutine render whose deferred redraw closure must not re-read
// cv.navMarkdown, because refresh()/OnBlur() may nil that field before the
// closure drains. The closure captures the live viewer instead, so tearing down
// the field mid-render no longer crashes.
func TestConfigurableDetailView_RenderDescriptionAfterTeardown(t *testing.T) {
	dir := t.TempDir()
	writeTestPNG(t, filepath.Join(dir, "a.png"))
	md := "![a.png](a.png)\n\ndescription body\n"
	sourceFile := filepath.Join(dir, "doc.md")

	sink := &progressSink{}
	// capture every redraw closure without running it. The redraw sink is shared
	// by the ProgressHub (statusline frames) and renderDescription's completion
	// closure; collecting all of them lets the test nil cv.navMarkdown
	// (mimicking refresh/OnBlur) before draining — the exact ordering that
	// panics in production.
	var mu sync.Mutex
	var pending []func()
	redraw := func(fn func()) {
		mu.Lock()
		pending = append(pending, fn)
		mu.Unlock()
	}
	hub := model.NewProgressHub(sink, redraw)
	defer hub.Stop()

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

	// wait for the render goroutine to finish and enqueue its completion closure.
	deadline := time.Now().Add(5 * time.Second)
	for {
		hub.DrainForTest()
		mu.Lock()
		n := len(pending)
		mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("render goroutine never enqueued its redraw closure")
		}
		time.Sleep(time.Millisecond)
	}
	// give the render goroutine a beat to enqueue its own closure too.
	time.Sleep(20 * time.Millisecond)
	hub.DrainForTest()

	// simulate refresh()/OnBlur() tearing down the viewer while redraw
	// closures are still pending.
	navMD.Close()
	cv.navMarkdown = nil

	// run every pending redraw closure: before the fix, renderDescription's
	// closure dereferences the now-nil cv.navMarkdown and panics.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("redraw closure panicked after teardown: %v", r)
		}
	}()
	mu.Lock()
	closures := pending
	mu.Unlock()
	for _, fn := range closures {
		fn()
	}
}
