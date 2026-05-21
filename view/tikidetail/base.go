package tikidetail

import (
	"path/filepath"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/rivo/tview"
)

// Base contains shared state and methods used by ConfigurableDetailView.
// The fixed-shape grid layout produced by grid_layout.go drives both view
// and edit modes; this struct holds the layout primitives and the
// dependencies needed to render the always-present description section.
type Base struct {
	root    *tview.Flex
	content *tview.Flex

	tikiStore    store.Store
	tikiID       string
	imageManager *navtview.ImageManager
	mermaidOpts  *nav.MermaidOptions
	descView     tview.Primitive

	fallbackTiki    *tikipkg.Tiki
	tikiEditSession *controller.TikiEditSession

	fullscreen         bool
	focusSetter        func(tview.Primitive)
	onFullscreenChange func(bool)
}

// build initializes the root and content flex layouts.
func (b *Base) build() {
	b.content = tview.NewFlex().SetDirection(tview.FlexRow)
	b.root = tview.NewFlex().SetDirection(tview.FlexRow)
	b.root.AddItem(b.content, 0, 1, true)
}

// GetTiki returns the tiki from the store or the fallback tiki.
func (b *Base) GetTiki() *tikipkg.Tiki {
	tk := b.tikiStore.GetTiki(b.tikiID)
	if tk == nil && b.fallbackTiki != nil && b.fallbackTiki.ID == b.tikiID {
		tk = b.fallbackTiki
	}
	return tk
}

// GetPrimitive returns the root tview primitive.
func (b *Base) GetPrimitive() tview.Primitive {
	return b.root
}

// SetFallbackTiki sets a tiki to render when it does not yet exist in the store (draft mode).
func (b *Base) SetFallbackTiki(tk *tikipkg.Tiki) {
	b.fallbackTiki = tk
}

// SetTikiEditSession wires the edit-session controller used to drive in-place edit mode.
func (b *Base) SetTikiEditSession(tc *controller.TikiEditSession) {
	b.tikiEditSession = tc
}

// SetFocusSetter sets the callback for requesting focus changes.
func (b *Base) SetFocusSetter(setter func(tview.Primitive)) {
	b.focusSetter = setter
}

// SetFullscreenChangeHandler sets the callback for when fullscreen state changes.
func (b *Base) SetFullscreenChangeHandler(handler func(isFullscreen bool)) {
	b.onFullscreenChange = handler
}

// IsFullscreen reports whether the view is currently in fullscreen mode.
func (b *Base) IsFullscreen() bool {
	return b.fullscreen
}

// defaultString returns def if s is empty, otherwise s.
func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// tikiSourcePathFor returns the on-disk source path of the tiki, falling
// back to the conventional doc-dir derived path when the tiki has no
// stored Path (e.g. drafts that haven't been written yet).
func tikiSourcePathFor(tk *tikipkg.Tiki) string {
	if tk == nil {
		return ""
	}
	if tk.Path != "" {
		return tk.Path
	}
	return filepath.Join(config.GetDocDir(), tk.ID+".md")
}
