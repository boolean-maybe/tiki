package palette

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/task"
)

// QuickSelect is a modal overlay listing candidate tasks, filterable by fuzzy typing.
type QuickSelect struct {
	root           *tview.Flex
	filterInput    *tview.InputField
	listView       *tview.TextView
	hintView       *tview.TextView
	quickSelectCfg *model.QuickSelectConfig

	candidateTasks []*task.Task
	filteredTasks  []*task.Task
	selectedIndex  int
	idColumnWidth  int
	rowColors      component.TaskRowColors
	lastWidth      int
}

// NewQuickSelect creates the quick-select widget.
func NewQuickSelect(quickSelectCfg *model.QuickSelectConfig) *QuickSelect {
	colors := config.GetColors()

	qs := &QuickSelect{
		quickSelectCfg: quickSelectCfg,
		rowColors:      component.DefaultTaskRowColors(),
	}

	qs.filterInput = tview.NewInputField()
	qs.filterInput.SetLabel(" ")
	qs.filterInput.SetFieldBackgroundColor(colors.ContentBackgroundColor.TCell())
	qs.filterInput.SetFieldTextColor(colors.InputFieldTextColor.TCell())
	qs.filterInput.SetLabelColor(colors.InputBoxLabelColor.TCell())
	qs.filterInput.SetPlaceholder("Type to filter tasks")
	qs.filterInput.SetPlaceholderStyle(tcell.StyleDefault.
		Foreground(colors.TaskDetailPlaceholderColor.TCell()).
		Background(colors.ContentBackgroundColor.TCell()))
	qs.filterInput.SetBackgroundColor(colors.ContentBackgroundColor.TCell())

	qs.listView = tview.NewTextView().SetDynamicColors(true)
	qs.listView.SetBackgroundColor(colors.ContentBackgroundColor.TCell())
	qs.listView.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		if width != qs.lastWidth && width > 0 {
			qs.renderList()
		}
		return x, y, width, height
	})

	qs.hintView = tview.NewTextView().SetDynamicColors(true)
	qs.hintView.SetBackgroundColor(colors.ContentBackgroundColor.TCell())
	mutedHex := colors.TaskDetailPlaceholderColor.Hex()
	qs.hintView.SetText(fmt.Sprintf(" [%s]↑↓ Select  ⏎ Pick  Esc Cancel", mutedHex))

	qs.root = tview.NewFlex().SetDirection(tview.FlexRow)
	qs.root.SetBackgroundColor(colors.ContentBackgroundColor.TCell())
	qs.root.SetBorder(true)
	qs.root.SetBorderColor(colors.TaskBoxUnselectedBorder.TCell())
	qs.root.AddItem(qs.filterInput, 1, 0, true)
	qs.root.AddItem(qs.listView, 0, 1, false)
	qs.root.AddItem(qs.hintView, 1, 0, false)

	qs.filterInput.SetInputCapture(qs.handleFilterInput)

	return qs
}

// GetPrimitive returns the root tview primitive for embedding in a Pages overlay.
func (qs *QuickSelect) GetPrimitive() tview.Primitive {
	return qs.root
}

// GetFilterInput returns the input field that should receive focus when the picker opens.
func (qs *QuickSelect) GetFilterInput() tview.Primitive {
	return qs.filterInput
}

// OnShow resets state and receives the pre-filtered candidate tasks.
func (qs *QuickSelect) OnShow(tasks []*task.Task) {
	qs.candidateTasks = tasks
	qs.filterInput.SetText("")
	qs.selectedIndex = 0
	qs.idColumnWidth = component.ComputeIDColumnWidth(tasks)
	qs.filterTasks()
	qs.renderList()
}

// SetChangedFunc wires a callback that re-filters when the input text changes.
func (qs *QuickSelect) SetChangedFunc() {
	qs.filterInput.SetChangedFunc(func(text string) {
		qs.filterTasks()
		qs.renderList()
	})
}

func (qs *QuickSelect) filterTasks() {
	query := qs.filterInput.GetText()
	if query == "" {
		qs.filteredTasks = make([]*task.Task, len(qs.candidateTasks))
		copy(qs.filteredTasks, qs.candidateTasks)
		qs.clampSelection()
		return
	}

	type scored struct {
		task  *task.Task
		score int
	}
	var matches []scored
	for _, t := range qs.candidateTasks {
		text := t.ID + " " + t.Title
		matched, score := fuzzyMatch(query, text)
		if matched {
			matches = append(matches, scored{t, score})
		}
	}

	// stable sort by score (lower is better)
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0 && matches[j].score < matches[j-1].score; j-- {
			matches[j], matches[j-1] = matches[j-1], matches[j]
		}
	}

	qs.filteredTasks = make([]*task.Task, len(matches))
	for i, m := range matches {
		qs.filteredTasks[i] = m.task
	}
	qs.clampSelection()
}

func (qs *QuickSelect) clampSelection() {
	if qs.selectedIndex >= len(qs.filteredTasks) {
		qs.selectedIndex = 0
	}
}

func (qs *QuickSelect) renderList() {
	_, _, width, _ := qs.listView.GetInnerRect()
	if width <= 0 {
		width = PaletteMinWidth
	}
	qs.lastWidth = width

	colors := config.GetColors()
	mutedHex := colors.TaskDetailPlaceholderColor.Hex()

	var buf strings.Builder

	if len(qs.filteredTasks) == 0 {
		buf.WriteString(fmt.Sprintf("[%s]  no matches", mutedHex))
		qs.listView.SetText(buf.String())
		return
	}

	for i, t := range qs.filteredTasks {
		if i > 0 {
			buf.WriteString("\n")
		}
		row := component.RenderTaskRow(t, i == qs.selectedIndex, width, qs.idColumnWidth, qs.rowColors)
		buf.WriteString(row)
	}

	qs.listView.SetText(buf.String())
}

func (qs *QuickSelect) handleFilterInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		qs.quickSelectCfg.Cancel()
		return nil

	case tcell.KeyEnter:
		qs.dispatchSelected()
		return nil

	case tcell.KeyUp:
		qs.moveSelection(-1)
		qs.renderList()
		return nil

	case tcell.KeyDown:
		qs.moveSelection(1)
		qs.renderList()
		return nil

	case tcell.KeyCtrlU:
		qs.filterInput.SetText("")
		qs.filterTasks()
		qs.renderList()
		return nil

	case tcell.KeyRune:
		return event

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return event

	default:
		return nil
	}
}

func (qs *QuickSelect) moveSelection(direction int) {
	n := len(qs.filteredTasks)
	if n == 0 {
		return
	}
	qs.selectedIndex += direction
	if qs.selectedIndex < 0 {
		qs.selectedIndex = n - 1
	} else if qs.selectedIndex >= n {
		qs.selectedIndex = 0
	}
}

func (qs *QuickSelect) dispatchSelected() {
	if qs.selectedIndex >= len(qs.filteredTasks) {
		qs.quickSelectCfg.Cancel()
		return
	}
	t := qs.filteredTasks[qs.selectedIndex]
	qs.quickSelectCfg.Select(t.ID)
}
