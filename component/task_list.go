package component

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/util"
	"github.com/boolean-maybe/tiki/util/gradient"
)

// TaskList displays tasks in a compact tabular format with three columns:
// status indicator, tiki ID (gradient-rendered), and title.
// It supports configurable visible row count, scrolling, and row selection.
type TaskList struct {
	*tview.Box
	tasks              []*task.Task
	maxVisibleRows     int
	scrollOffset       int
	selectionIndex     int
	idColumnWidth      int             // computed from widest ID
	idGradient         config.Gradient // gradient for ID text
	idFallback         tcell.Color     // fallback solid color for ID
	titleColor         string          // tview color tag for title, e.g. "[#b8b8b8]"
	selectionColor     string          // tview color tag for selected row highlight
	statusDoneColor    string          // tview color tag for done status indicator
	statusPendingColor string          // tview color tag for pending status indicator
}

// NewTaskList creates a new TaskList with the given maximum visible row count.
func NewTaskList(maxVisibleRows int) *TaskList {
	colors := config.GetColors()
	return &TaskList{
		Box:                tview.NewBox(),
		maxVisibleRows:     maxVisibleRows,
		idGradient:         colors.TaskBoxIDColor,
		idFallback:         config.FallbackTaskIDColor,
		titleColor:         colors.TaskBoxTitleColor,
		selectionColor:     colors.TaskListSelectionColor,
		statusDoneColor:    colors.TaskListStatusDoneColor,
		statusPendingColor: colors.TaskListStatusPendingColor,
	}
}

// SetTasks replaces the task data, recomputes the ID column width, and clamps scroll/selection.
func (tl *TaskList) SetTasks(tasks []*task.Task) *TaskList {
	tl.tasks = tasks
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

// GetSelectedTask returns the currently selected task, or nil if none.
func (tl *TaskList) GetSelectedTask() *task.Task {
	if tl.selectionIndex < 0 || tl.selectionIndex >= len(tl.tasks) {
		return nil
	}
	return tl.tasks[tl.selectionIndex]
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
	if tl.selectionIndex < len(tl.tasks)-1 {
		tl.selectionIndex++
		tl.ensureSelectionVisible()
	}
}

// SetIDColors overrides the gradient and fallback color for the ID column.
func (tl *TaskList) SetIDColors(g config.Gradient, fallback tcell.Color) *TaskList {
	tl.idGradient = g
	tl.idFallback = fallback
	return tl
}

// SetTitleColor overrides the tview color tag for the title column.
func (tl *TaskList) SetTitleColor(color string) *TaskList {
	tl.titleColor = color
	return tl
}

// Draw renders the TaskList onto the screen.
func (tl *TaskList) Draw(screen tcell.Screen) {
	tl.DrawForSubclass(screen, tl)

	x, y, width, height := tl.GetInnerRect()
	if width <= 0 || height <= 0 || len(tl.tasks) == 0 {
		return
	}

	tl.ensureSelectionVisible()

	visibleRows := tl.visibleRowCount(height)

	for i := range visibleRows {
		itemIndex := tl.scrollOffset + i
		if itemIndex >= len(tl.tasks) {
			break
		}

		t := tl.tasks[itemIndex]
		row := tl.buildRow(t, itemIndex == tl.selectionIndex, width)
		tview.Print(screen, row, x, y+i, width, tview.AlignLeft, tcell.ColorDefault)
	}
}

// buildRow constructs the tview-tagged string for a single row.
func (tl *TaskList) buildRow(t *task.Task, selected bool, width int) string {
	// Status indicator: done = checkmark, else circle
	var statusIndicator string
	if config.GetStatusRegistry().IsDone(string(t.Status)) {
		statusIndicator = tl.statusDoneColor + "\u2713[-]"
	} else {
		statusIndicator = tl.statusPendingColor + "\u25CB[-]"
	}

	// Gradient-rendered ID, padded to idColumnWidth
	idText := gradient.RenderAdaptiveGradientText(t.ID, tl.idGradient, tl.idFallback)
	// Pad with spaces if ID is shorter than column width
	if padding := tl.idColumnWidth - len(t.ID); padding > 0 {
		idText += fmt.Sprintf("%*s", padding, "")
	}

	// Title: fill remaining width, truncated
	// Layout: "X IDID  Title" => status(1) + space(1) + id(idColumnWidth) + space(1) + title
	titleAvailable := max(width-1-1-tl.idColumnWidth-1, 0)
	truncatedTitle := tview.Escape(util.TruncateText(t.Title, titleAvailable))

	row := fmt.Sprintf("%s %s %s%s[-]", statusIndicator, idText, tl.titleColor, truncatedTitle)

	if selected {
		row = tl.selectionColor + row
	}

	return row
}

// ensureSelectionVisible adjusts scrollOffset so the selected row is within the viewport.
func (tl *TaskList) ensureSelectionVisible() {
	if len(tl.tasks) == 0 {
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
	if maxVisible > len(tl.tasks) {
		maxVisible = len(tl.tasks)
	}
	return maxVisible
}

// recomputeIDColumnWidth calculates the width needed for the widest task ID.
func (tl *TaskList) recomputeIDColumnWidth() {
	tl.idColumnWidth = 0
	for _, t := range tl.tasks {
		if len(t.ID) > tl.idColumnWidth {
			tl.idColumnWidth = len(t.ID)
		}
	}
}

// clampSelection ensures selectionIndex is within [0, len(tasks)-1].
func (tl *TaskList) clampSelection() {
	if len(tl.tasks) == 0 {
		tl.selectionIndex = 0
		return
	}
	if tl.selectionIndex < 0 {
		tl.selectionIndex = 0
	}
	if tl.selectionIndex >= len(tl.tasks) {
		tl.selectionIndex = len(tl.tasks) - 1
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

	maxOffset := max(len(tl.tasks)-maxVisible, 0)
	if tl.scrollOffset > maxOffset {
		tl.scrollOffset = maxOffset
	}
}
