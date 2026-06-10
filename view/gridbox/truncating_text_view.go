package gridbox

import (
	"github.com/boolean-maybe/tiki/util"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// truncatingTextView is a single-line tview.TextView that truncates its text
// to the available width with a trailing ellipsis ("…") instead of hard-
// clipping at the cell edge. Plain TextViews silently clip overflow, which on
// a fixed-height card reads as wrong data (a comma-joined list cut mid-token,
// or a value that looks empty). This wrapper makes the cut visible.
//
// Truncation is non-destructive: the wrapper holds the full source text and
// re-truncates from it on every Draw against the live rect width, so widening
// the column restores the full text. The width is only known at Draw time
// (tview lays out Flex children before calling their Draw), which is why this
// is a Draw-time wrapper rather than rebuild-time text mutation — the latter
// cannot see grow-column (:fr) widths, which resolve only after layout.
//
// Use only for single-line content. Multi-row primitives (word-wrapped prose,
// the WordList/TikiList list columns) wrap their own content and are not
// TextViews, so they are never wrapped in this.
type truncatingTextView struct {
	*tview.TextView
	source string // full untruncated text, the source of truth for each Draw
}

// NewTruncatingTextView returns an empty single-line truncating text view with
// dynamic colors enabled and no border padding, matching the plain value/label
// primitives gridbox content uses.
func NewTruncatingTextView() *truncatingTextView {
	tv := tview.NewTextView().SetDynamicColors(true)
	tv.SetBorderPadding(0, 0, 0, 0)
	return &truncatingTextView{TextView: tv}
}

// SetText records the full text as the truncation source and seeds the
// embedded view with it. Returns the wrapper for chaining parity with tview.
func (t *truncatingTextView) SetText(text string) *truncatingTextView {
	t.source = text
	t.TextView.SetText(text)
	return t
}

// Draw truncates the source text to the current inner width (color-aware) and
// renders it. The embedded view is left holding the full source after Draw so
// the next Draw — possibly at a wider width — truncates from the complete text
// rather than an already-clipped copy.
func (t *truncatingTextView) Draw(screen tcell.Screen) {
	_, _, width, _ := t.GetInnerRect()
	if width > 0 {
		t.TextView.SetText(util.TruncateTextWithColors(t.source, width))
	}
	t.TextView.Draw(screen)
	t.TextView.SetText(t.source)
}
