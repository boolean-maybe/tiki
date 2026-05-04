package component

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/config"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/util"
	"github.com/boolean-maybe/tiki/util/gradient"
)

// TaskRowColors holds the color configuration for rendering a task row.
type TaskRowColors struct {
	IDGradient         config.Gradient
	IDFallback         config.Color
	TitleColor         config.Color
	SelectionFg        config.Color
	SelectionBg        config.Color
	StatusDoneColor    config.Color
	StatusPendingColor config.Color
}

// DefaultTaskRowColors returns TaskRowColors from the current theme config.
func DefaultTaskRowColors() TaskRowColors {
	colors := config.GetColors()
	return TaskRowColors{
		IDGradient:         colors.TaskBoxIDColor,
		IDFallback:         colors.FallbackTaskIDColor,
		TitleColor:         colors.TaskBoxTitleColor,
		SelectionFg:        colors.TaskListSelectionFg,
		SelectionBg:        colors.TaskListSelectionBg,
		StatusDoneColor:    colors.TaskListStatusDoneColor,
		StatusPendingColor: colors.TaskListStatusPendingColor,
	}
}

// RenderTaskRow builds a tview-tagged string for a single tiki row.
func RenderTaskRow(tk *tikipkg.Tiki, selected bool, width int, idColumnWidth int, colors TaskRowColors) string {
	status, _, _ := tk.StringField(tikipkg.FieldStatus)
	var statusIndicator string
	if config.GetStatusRegistry().IsDone(status) {
		statusIndicator = colors.StatusDoneColor.Tag().String() + "✓[-]"
	} else {
		statusIndicator = colors.StatusPendingColor.Tag().String() + "○[-]"
	}

	idText := gradient.RenderAdaptiveGradientText(tk.ID, colors.IDGradient, colors.IDFallback)
	if padding := idColumnWidth - len(tk.ID); padding > 0 {
		idText += fmt.Sprintf("%*s", padding, "")
	}

	titleAvailable := max(width-1-1-idColumnWidth-1, 0)
	truncatedTitle := tview.Escape(util.TruncateText(tk.Title, titleAvailable))

	row := fmt.Sprintf("%s %s %s%s", statusIndicator, idText, colors.TitleColor.Tag().String(), truncatedTitle)

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
		if len(tk.ID) > w {
			w = len(tk.ID)
		}
	}
	return w
}

// TaskList displays tikis in a compact tabular format with three columns:
// status indicator, tiki ID (gradient-rendered), and title.
// It supports configurable visible row count, scrolling, and row selection.
type TaskList struct {
	*tview.Box
	tikis              []*tikipkg.Tiki
	maxVisibleRows     int
	scrollOffset       int
	selectionIndex     int
	idColumnWidth      int             // computed from widest ID
	idGradient         config.Gradient // gradient for ID text
	idFallback         config.Color    // fallback solid color for ID
	titleColor         config.Color    // color for title text
	selectionColor     config.Color    // foreground color for selected row highlight
	selectionBgColor   config.Color    // background color for selected row highlight
	statusDoneColor    config.Color    // color for done status indicator
	statusPendingColor config.Color    // color for pending status indicator
}

// NewTaskList creates a new TaskList with the given maximum visible row count.
func NewTaskList(maxVisibleRows int) *TaskList {
	colors := DefaultTaskRowColors()
	return &TaskList{
		Box:                tview.NewBox(),
		maxVisibleRows:     maxVisibleRows,
		idGradient:         colors.IDGradient,
		idFallback:         colors.IDFallback,
		titleColor:         colors.TitleColor,
		selectionColor:     colors.SelectionFg,
		selectionBgColor:   colors.SelectionBg,
		statusDoneColor:    colors.StatusDoneColor,
		statusPendingColor: colors.StatusPendingColor,
	}
}

// SetTasks replaces the tiki data, recomputes the ID column width, and clamps scroll/selection.
func (tl *TaskList) SetTasks(tikis []*tikipkg.Tiki) *TaskList {
	tl.tikis = tikis
	tl.recomputeIDColumnWidth()
	tl.clampSelection()
	tl.clampScroll()
	return tl
}

// SetSelection sets the selected row index, clamped to valid bounds.
func (tl *TaskList) SetSelection(index int) *TaskList {
	tl.selectionIndex = index
	tl.clampSelection()
	tl.ensureSelectionVisible()
	return tl
}

// GetSelectedIndex returns the current selection index.
func (tl *TaskList) GetSelectedIndex() int {
	return tl.selectionIndex
}

// GetSelectedTask returns the currently selected tiki, or nil if none.
func (tl *TaskList) GetSelectedTask() *tikipkg.Tiki {
	if tl.selectionIndex < 0 || tl.selectionIndex >= len(tl.tikis) {
		return nil
	}
	return tl.tikis[tl.selectionIndex]
}

// ScrollUp moves the selection up by one row.
func (tl *TaskList) ScrollUp() {
	if tl.selectionIndex > 0 {
		tl.selectionIndex--
		tl.ensureSelectionVisible()
	}
}

// ScrollDown moves the selection down by one row.
func (tl *TaskList) ScrollDown() {
	if tl.selectionIndex < len(tl.tikis)-1 {
		tl.selectionIndex++
		tl.ensureSelectionVisible()
	}
}

// SetIDColors overrides the gradient and fallback color for the ID column.
func (tl *TaskList) SetIDColors(g config.Gradient, fallback config.Color) *TaskList {
	tl.idGradient = g
	tl.idFallback = fallback
	return tl
}

// SetTitleColor overrides the color for the title column.
func (tl *TaskList) SetTitleColor(color config.Color) *TaskList {
	tl.titleColor = color
	return tl
}

// Draw renders the TaskList onto the screen.
func (tl *TaskList) Draw(screen tcell.Screen) {
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
		row := tl.buildRow(tk, itemIndex == tl.selectionIndex, width)
		tview.Print(screen, row, x, y+i, width, tview.AlignLeft, tcell.ColorDefault)
	}
}

func (tl *TaskList) buildRow(tk *tikipkg.Tiki, selected bool, width int) string {
	return RenderTaskRow(tk, selected, width, tl.idColumnWidth, TaskRowColors{
		IDGradient:         tl.idGradient,
		IDFallback:         tl.idFallback,
		TitleColor:         tl.titleColor,
		SelectionFg:        tl.selectionColor,
		SelectionBg:        tl.selectionBgColor,
		StatusDoneColor:    tl.statusDoneColor,
		StatusPendingColor: tl.statusPendingColor,
	})
}

// ensureSelectionVisible adjusts scrollOffset so the selected row is within the viewport.
func (tl *TaskList) ensureSelectionVisible() {
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
func (tl *TaskList) visibleRowCount(height int) int {
	maxVisible := height
	if tl.maxVisibleRows > 0 && maxVisible > tl.maxVisibleRows {
		maxVisible = tl.maxVisibleRows
	}
	if maxVisible > len(tl.tikis) {
		maxVisible = len(tl.tikis)
	}
	return maxVisible
}

func (tl *TaskList) recomputeIDColumnWidth() {
	tl.idColumnWidth = ComputeIDColumnWidth(tl.tikis)
}

// clampSelection ensures selectionIndex is within [0, len(tikis)-1].
func (tl *TaskList) clampSelection() {
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
func (tl *TaskList) clampScroll() {
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
