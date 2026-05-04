package palette

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// QuickSelect is a modal overlay listing candidate tikis, filterable by fuzzy typing.
type QuickSelect struct {
	root           *tview.Flex
	filterInput    *tview.InputField
	listView       *tview.TextView
	hintView       *tview.TextView
	quickSelectCfg *model.QuickSelectConfig

	candidateTikis []*tikipkg.Tiki
	filteredTikis  []*tikipkg.Tiki
	selectedIndex  int
	scrollOffset   int
	idColumnWidth  int
	rowColors      component.TaskRowColors
	lastWidth      int
	lastHeight     int
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
		if (width != qs.lastWidth || height != qs.lastHeight) && width > 0 {
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

// OnShow resets state and receives the pre-filtered candidate tikis.
func (qs *QuickSelect) OnShow(tikis []*tikipkg.Tiki) {
	qs.candidateTikis = tikis
	qs.filterInput.SetText("")
	qs.selectedIndex = 0
	qs.scrollOffset = 0
	qs.idColumnWidth = component.ComputeIDColumnWidth(tikis)
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
		qs.filteredTikis = make([]*tikipkg.Tiki, len(qs.candidateTikis))
		copy(qs.filteredTikis, qs.candidateTikis)
		qs.scrollOffset = 0
		qs.clampSelection()
		return
	}

	type scored struct {
		tiki  *tikipkg.Tiki
		score int
	}
	var matches []scored
	for _, tk := range qs.candidateTikis {
		text := tk.ID + " " + tk.Title
		matched, score := fuzzyMatch(query, text)
		if matched {
			matches = append(matches, scored{tk, score})
		}
	}

	// stable sort by score (lower is better)
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0 && matches[j].score < matches[j-1].score; j-- {
			matches[j], matches[j-1] = matches[j-1], matches[j]
		}
	}

	qs.filteredTikis = make([]*tikipkg.Tiki, len(matches))
	for i, m := range matches {
		qs.filteredTikis[i] = m.tiki
	}
	qs.scrollOffset = 0
	qs.clampSelection()
}

func (qs *QuickSelect) clampSelection() {
	if qs.selectedIndex >= len(qs.filteredTikis) {
		qs.selectedIndex = 0
	}
}

func (qs *QuickSelect) renderList() {
	_, _, width, height := qs.listView.GetInnerRect()
	if width <= 0 {
		width = PaletteMinWidth
	}
	if height <= 0 {
		return
	}
	qs.lastWidth = width
	qs.lastHeight = height

	colors := config.GetColors()
	mutedHex := colors.TaskDetailPlaceholderColor.Hex()

	var buf strings.Builder

	if len(qs.filteredTikis) == 0 {
		buf.WriteString("\n")
		buf.WriteString(fmt.Sprintf("[%s]  no matches", mutedHex))
		qs.listView.SetText(buf.String())
		return
	}

	qs.ensureSelectionVisible()

	maxVisible := height - 1
	if maxVisible <= 0 {
		maxVisible = 1
	}
	endIndex := qs.scrollOffset + maxVisible
	if endIndex > len(qs.filteredTikis) {
		endIndex = len(qs.filteredTikis)
	}

	buf.WriteString("\n")
	for i := qs.scrollOffset; i < endIndex; i++ {
		if i > qs.scrollOffset {
			buf.WriteString("\n")
		}
		row := component.RenderTaskRow(qs.filteredTikis[i], i == qs.selectedIndex, width, qs.idColumnWidth, qs.rowColors)
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
	n := len(qs.filteredTikis)
	if n == 0 {
		return
	}
	qs.selectedIndex += direction
	if qs.selectedIndex < 0 {
		qs.selectedIndex = n - 1
	} else if qs.selectedIndex >= n {
		qs.selectedIndex = 0
	}
	qs.ensureSelectionVisible()
}

func (qs *QuickSelect) ensureSelectionVisible() {
	n := len(qs.filteredTikis)
	if n == 0 {
		qs.scrollOffset = 0
		return
	}

	_, _, _, height := qs.listView.GetInnerRect()
	maxVisible := height - 1
	if maxVisible <= 0 {
		maxVisible = 1
	}

	if qs.selectedIndex < qs.scrollOffset {
		qs.scrollOffset = qs.selectedIndex
	}
	lastVisible := qs.scrollOffset + maxVisible - 1
	if qs.selectedIndex > lastVisible {
		qs.scrollOffset = qs.selectedIndex - maxVisible + 1
	}

	if qs.scrollOffset < 0 {
		qs.scrollOffset = 0
	}
	maxOffset := n - maxVisible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if qs.scrollOffset > maxOffset {
		qs.scrollOffset = maxOffset
	}
}

func (qs *QuickSelect) dispatchSelected() {
	if qs.selectedIndex >= len(qs.filteredTikis) {
		qs.quickSelectCfg.Cancel()
		return
	}
	tk := qs.filteredTikis[qs.selectedIndex]
	qs.quickSelectCfg.Select(tk.ID)
}
