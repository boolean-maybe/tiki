package viewer

import (
	"fmt"

	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/boolean-maybe/tiki/theme"
	"github.com/boolean-maybe/tiki/view/statusline"
	"github.com/rivo/tview"
)

// progressBarCols is the compact width (in cells) of the block-shade bar
// rendered into the standalone viewer's status bar.
const progressBarCols = 12

// progressStatusBar adapts the standalone viewer's status bar TextView to the
// model.ProgressSink interface. While progress is active it renders the
// block-shade bar; when it clears, it restores the normal status text. Its
// methods run on the UI goroutine (invoked through the hub's QueueUpdateDraw
// redraw), so the TextView mutations are safe.
type progressStatusBar struct {
	bar       *tview.TextView
	viewer    func() *navtview.TextViewViewer
	animFrame int
	active    bool
}

// SetProgress renders the block-shade bar into the status bar. Indeterminate
// progress advances the animation frame each call.
func (p *progressStatusBar) SetProgress(done, total int) {
	if total <= 0 {
		p.animFrame++
	}
	p.active = true
	roles := theme.Roles()
	bar := statusline.RenderProgressBar(done, total, p.animFrame, progressBarCols)
	fg := roles.TextSecondary().Hex()
	p.bar.SetText(fmt.Sprintf(" [%s]%s[-]", fg, bar))
}

// ClearProgress restores the normal status text.
func (p *progressStatusBar) ClearProgress() {
	p.active = false
	if v := p.viewer(); v != nil {
		updateStatusBar(p.bar, v)
	}
}
