package component

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/util"
)

// TikiRowColors holds the color configuration for rendering a tiki row.
type TikiRowColors struct {
	IDPaint            theme.Paint
	TitleColor         theme.Color
	SelectionFg        theme.Color
	SelectionBg        theme.Color
	StatusDoneColor    theme.Color
	StatusPendingColor theme.Color
}

// DefaultTikiRowColors returns TikiRowColors derived from the active theme roles.
func DefaultTikiRowColors() TikiRowColors {
	roles := theme.Roles()
	idPaint, _ := roles.PaintResolver()("tiki.id", "accent")
	return TikiRowColors{
		IDPaint:            idPaint,
		TitleColor:         theme.NewColor(roles.TextSecondary().TCell()),
		SelectionFg:        theme.NewColor(roles.TextPrimary().TCell()),
		SelectionBg:        theme.NewColor(roles.SurfaceSelection().TCell()),
		StatusDoneColor:    theme.NewColor(roles.TextLabel().TCell()),
		StatusPendingColor: theme.NewColor(roles.TextPrimary().TCell()),
	}
}

// RenderTikiRow builds a tview-tagged string for a single tiki row.
// The leading status ✓/○ indicator was dropped when status became an
// ordinary enum field — workflows that want a glyph can use the enum
// value's emoji metadata.
func RenderTikiRow(tk *tikipkg.Tiki, selected bool, width int, idColumnWidth int, colors TikiRowColors) string {
	idText := colors.IDPaint.PaintString(tk.ID())
	if padding := idColumnWidth - len(tk.ID()); padding > 0 {
		idText += fmt.Sprintf("%*s", padding, "")
	}

	titleAvailable := max(width-1-idColumnWidth-1, 0)
	truncatedTitle := tview.Escape(util.TruncateText(tk.Title(), titleAvailable))

	row := fmt.Sprintf("%s %s%s", idText, colors.TitleColor.Tag().String(), truncatedTitle)

	if selected {
		row = colors.SelectionFg.Tag().WithBg(colors.SelectionBg).String() + row
	}

	visibleWidth := tview.TaggedStringWidth(row)
	if pad := width - visibleWidth; pad > 0 {
		row += strings.Repeat(" ", pad)
	}

	return row + "[-:-:-]"
}

// ComputeIDColumnWidth returns the width needed for the widest tiki ID.
func ComputeIDColumnWidth(tikis []*tikipkg.Tiki) int {
	w := 0
	for _, tk := range tikis {
		if len(tk.ID()) > w {
			w = len(tk.ID())
		}
	}
	return w
}

// TikiList displays tikis in a compact tabular format with three columns:
// status indicator, tiki ID (gradient-rendered), and title.
// It supports configurable visible row count, scrolling, and row selection.
type TikiList struct {
	*tview.Box
	tikis              []*tikipkg.Tiki
	maxVisibleRows     int
	scrollOffset       int
	selectionIndex     int
	idColumnWidth      int         // computed from widest ID
	idPaint            theme.Paint // paint for ID text (gradient on capable terminals, solid otherwise)
	titleColor         theme.Color // color for title text
	selectionColor     theme.Color // foreground color for selected row highlight
	selectionBgColor   theme.Color // background color for selected row highlight
	statusDoneColor    theme.Color // color for done status indicator
	statusPendingColor theme.Color // color for pending status indicator
	selectable         bool        // when false, no row is ever drawn highlighted
}

// NewTikiList creates a new TikiList with the given maximum visible row count.
// Lists are selectable by default (the interactive palette relies on a
// highlighted row); callers that display a static, non-interactive list — such
// as a metadata-grid value cell — call SetSelectable(false) so no row carries
// the selection background.
func NewTikiList(maxVisibleRows int) *TikiList {
	colors := DefaultTikiRowColors()
	return &TikiList{
		Box:                tview.NewBox(),
		maxVisibleRows:     maxVisibleRows,
		idPaint:            colors.IDPaint,
		titleColor:         colors.TitleColor,
		selectionColor:     colors.SelectionFg,
		selectionBgColor:   colors.SelectionBg,
		statusDoneColor:    colors.StatusDoneColor,
		statusPendingColor: colors.StatusPendingColor,
		selectable:         true,
	}
}

// SetSelectable controls whether the list highlights a selected row. A
// non-selectable list (e.g. a read-only metadata-grid value) draws every row
// in plain text — no selection background — matching the surrounding value
// cells. Returns self for chaining.
func (tl *TikiList) SetSelectable(selectable bool) *TikiList {
	tl.selectable = selectable
	return tl
}

// SetTikis replaces the tiki data, recomputes the ID column width, and clamps scroll/selection.
func (tl *TikiList) SetTikis(tikis []*tikipkg.Tiki) *TikiList {
	tl.tikis = tikis
	tl.recomputeIDColumnWidth()
	tl.clampSelection()
	tl.clampScroll()
	return tl
}

// SetSelection sets the selected row index, clamped to valid bounds.
func (tl *TikiList) SetSelection(index int) *TikiList {
	tl.selectionIndex = index
	tl.clampSelection()
	tl.ensureSelectionVisible()
	return tl
}

// GetSelectedIndex returns the current selection index.
func (tl *TikiList) GetSelectedIndex() int {
	return tl.selectionIndex
}

// GetSelectedTiki returns the currently selected tiki, or nil if none.
func (tl *TikiList) GetSelectedTiki() *tikipkg.Tiki {
	if tl.selectionIndex < 0 || tl.selectionIndex >= len(tl.tikis) {
		return nil
	}
	return tl.tikis[tl.selectionIndex]
}

// ScrollUp moves the selection up by one row.
func (tl *TikiList) ScrollUp() {
	if tl.selectionIndex > 0 {
		tl.selectionIndex--
		tl.ensureSelectionVisible()
	}
}

// ScrollDown moves the selection down by one row.
func (tl *TikiList) ScrollDown() {
	if tl.selectionIndex < len(tl.tikis)-1 {
		tl.selectionIndex++
		tl.ensureSelectionVisible()
	}
}

// SetIDPaint overrides the Paint used to render the ID column.
func (tl *TikiList) SetIDPaint(p theme.Paint) *TikiList {
	tl.idPaint = p
	return tl
}

// SetTitleColor overrides the color for the title column.
func (tl *TikiList) SetTitleColor(color theme.Color) *TikiList {
	tl.titleColor = color
	return tl
}

// Draw renders the TikiList onto the screen.
func (tl *TikiList) Draw(screen tcell.Screen) {
	tl.DrawForSubclass(screen, tl)

	x, y, width, height := tl.GetInnerRect()
	if width <= 0 || height <= 0 || len(tl.tikis) == 0 {
		return
	}

	tl.ensureSelectionVisible()

	visibleRows := tl.visibleRowCount(height)

	for i := range visibleRows {
		itemIndex := tl.scrollOffset + i
		if itemIndex >= len(tl.tikis) {
			break
		}

		tk := tl.tikis[itemIndex]
		selected := tl.selectable && itemIndex == tl.selectionIndex
		row := tl.buildRow(tk, selected, width)
		tview.Print(screen, row, x, y+i, width, tview.AlignLeft, theme.Roles().SurfaceTransparent().TCell())
	}
}

func (tl *TikiList) buildRow(tk *tikipkg.Tiki, selected bool, width int) string {
	return RenderTikiRow(tk, selected, width, tl.idColumnWidth, TikiRowColors{
		IDPaint:            tl.idPaint,
		TitleColor:         tl.titleColor,
		SelectionFg:        tl.selectionColor,
		SelectionBg:        tl.selectionBgColor,
		StatusDoneColor:    tl.statusDoneColor,
		StatusPendingColor: tl.statusPendingColor,
	})
}

// ensureSelectionVisible adjusts scrollOffset so the selected row is within the viewport.
func (tl *TikiList) ensureSelectionVisible() {
	if len(tl.tikis) == 0 {
		return
	}

	_, _, _, height := tl.GetInnerRect()
	maxVisible := tl.visibleRowCount(height)
	if maxVisible <= 0 {
		return
	}

	// Selection above viewport
	if tl.selectionIndex < tl.scrollOffset {
		tl.scrollOffset = tl.selectionIndex
	}

	// Selection below viewport
	lastVisible := tl.scrollOffset + maxVisible - 1
	if tl.selectionIndex > lastVisible {
		tl.scrollOffset = tl.selectionIndex - maxVisible + 1
	}

	tl.clampScroll()
}

// visibleRowCount returns the number of rows that can be displayed.
func (tl *TikiList) visibleRowCount(height int) int {
	maxVisible := height
	if tl.maxVisibleRows > 0 && maxVisible > tl.maxVisibleRows {
		maxVisible = tl.maxVisibleRows
	}
	if maxVisible > len(tl.tikis) {
		maxVisible = len(tl.tikis)
	}
	return maxVisible
}

func (tl *TikiList) recomputeIDColumnWidth() {
	tl.idColumnWidth = ComputeIDColumnWidth(tl.tikis)
}

// clampSelection ensures selectionIndex is within [0, len(tikis)-1].
func (tl *TikiList) clampSelection() {
	if len(tl.tikis) == 0 {
		tl.selectionIndex = 0
		return
	}
	if tl.selectionIndex < 0 {
		tl.selectionIndex = 0
	}
	if tl.selectionIndex >= len(tl.tikis) {
		tl.selectionIndex = len(tl.tikis) - 1
	}
}

// clampScroll ensures scrollOffset stays within valid bounds.
func (tl *TikiList) clampScroll() {
	if tl.scrollOffset < 0 {
		tl.scrollOffset = 0
	}

	_, _, _, height := tl.GetInnerRect()
	maxVisible := tl.visibleRowCount(height)
	if maxVisible <= 0 {
		return
	}

	maxOffset := max(len(tl.tikis)-maxVisible, 0)
	if tl.scrollOffset > maxOffset {
		tl.scrollOffset = maxOffset
	}
}
