package palette

import (
	"fmt"
	"sort"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/util"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const PaletteMinWidth = 30

// sectionType identifies which section a palette row belongs to.
type sectionType int

const (
	sectionGlobal sectionType = iota
	sectionViews
	sectionView
)

// paletteRow is a single entry in the rendered palette list.
type paletteRow struct {
	action    controller.Action
	section   sectionType
	enabled   bool
	separator bool // true for section header/separator rows
	label     string
}

// ActionPalette is a modal overlay listing all available actions, filterable by fuzzy typing.
type ActionPalette struct {
	root          *tview.Flex
	filterInput   *tview.InputField
	listView      *tview.TextView
	hintView      *tview.TextView
	viewContext   *model.ViewContext
	paletteConfig *model.ActionPaletteConfig
	inputRouter   *controller.InputRouter
	navController *controller.NavigationController

	rows          []paletteRow
	visibleRows   []int // indices into rows for current filter
	selectedIndex int   // index into visibleRows
	lastWidth     int   // width used for last render, to detect resize

	viewContextListenerID int
}

// NewActionPalette creates the palette widget.
func NewActionPalette(
	viewContext *model.ViewContext,
	paletteConfig *model.ActionPaletteConfig,
	inputRouter *controller.InputRouter,
	navController *controller.NavigationController,
) *ActionPalette {
	colors := config.GetColors()

	ap := &ActionPalette{
		viewContext:   viewContext,
		paletteConfig: paletteConfig,
		inputRouter:   inputRouter,
		navController: navController,
	}

	// filter input
	ap.filterInput = tview.NewInputField()
	ap.filterInput.SetLabel(" ")
	ap.filterInput.SetFieldBackgroundColor(colors.ContentBackgroundColor.TCell())
	ap.filterInput.SetFieldTextColor(colors.InputFieldTextColor.TCell())
	ap.filterInput.SetLabelColor(colors.SearchBoxLabelColor.TCell())
	ap.filterInput.SetPlaceholder("Type to search")
	ap.filterInput.SetPlaceholderStyle(tcell.StyleDefault.
		Foreground(colors.TaskDetailPlaceholderColor.TCell()).
		Background(colors.ContentBackgroundColor.TCell()))
	ap.filterInput.SetBackgroundColor(colors.ContentBackgroundColor.TCell())

	// list area
	ap.listView = tview.NewTextView().SetDynamicColors(true)
	ap.listView.SetBackgroundColor(colors.ContentBackgroundColor.TCell())
	ap.listView.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		if width != ap.lastWidth && width > 0 {
			ap.renderList()
		}
		return x, y, width, height
	})

	// bottom hint
	ap.hintView = tview.NewTextView().SetDynamicColors(true)
	ap.hintView.SetBackgroundColor(colors.ContentBackgroundColor.TCell())
	mutedHex := colors.TaskDetailPlaceholderColor.Hex()
	ap.hintView.SetText(fmt.Sprintf(" [%s]↑↓ Select  ⏎ Run  Esc Close", mutedHex))

	// root layout
	ap.root = tview.NewFlex().SetDirection(tview.FlexRow)
	ap.root.SetBackgroundColor(colors.ContentBackgroundColor.TCell())
	ap.root.SetBorderColor(colors.TaskBoxUnselectedBorder.TCell())
	ap.root.SetBorder(true)
	ap.root.AddItem(ap.filterInput, 1, 0, true)
	ap.root.AddItem(ap.listView, 0, 1, false)
	ap.root.AddItem(ap.hintView, 1, 0, false)

	// wire filter input to intercept all palette keys
	ap.filterInput.SetInputCapture(ap.handleFilterInput)

	// subscribe to view context changes
	ap.viewContextListenerID = viewContext.AddListener(func() {
		ap.rebuildRows()
		ap.renderList()
	})

	return ap
}

// GetPrimitive returns the root tview primitive for embedding in a Pages overlay.
func (ap *ActionPalette) GetPrimitive() tview.Primitive {
	return ap.root
}

// GetFilterInput returns the input field that should receive focus when the palette opens.
func (ap *ActionPalette) GetFilterInput() *tview.InputField {
	return ap.filterInput
}

// OnShow resets state and rebuilds rows when the palette becomes visible.
func (ap *ActionPalette) OnShow() {
	ap.filterInput.SetText("")
	ap.selectedIndex = 0
	ap.rebuildRows()
	ap.renderList()
}

// Cleanup removes all listeners.
func (ap *ActionPalette) Cleanup() {
	ap.viewContext.RemoveListener(ap.viewContextListenerID)
}

func (ap *ActionPalette) rebuildRows() {
	ap.rows = nil

	currentView := ap.navController.CurrentView()
	activeView := ap.navController.GetActiveView()

	globalActions := controller.DefaultGlobalActions().GetPaletteActions()
	globalIDs := make(map[controller.ActionID]bool, len(globalActions))
	for _, a := range globalActions {
		globalIDs[a.ID] = true
	}

	// global section
	if len(globalActions) > 0 {
		ap.rows = append(ap.rows, paletteRow{separator: true, label: "Global", section: sectionGlobal})
		for _, a := range globalActions {
			ap.rows = append(ap.rows, paletteRow{
				action:  a,
				section: sectionGlobal,
				enabled: actionEnabled(a, currentView, activeView),
			})
		}
	}

	// views section (plugin activation keys) — only if active view shows navigation
	pluginIDs := make(map[controller.ActionID]bool)
	if activeView != nil {
		if np, ok := activeView.(controller.NavigationProvider); ok && np.ShowNavigation() {
			pluginActions := controller.GetPluginActions().GetPaletteActions()
			if len(pluginActions) > 0 {
				ap.rows = append(ap.rows, paletteRow{separator: true, label: "Views", section: sectionViews})
				for _, a := range pluginActions {
					pluginIDs[a.ID] = true
					ap.rows = append(ap.rows, paletteRow{
						action:  a,
						section: sectionViews,
						enabled: actionEnabled(a, currentView, activeView),
					})
				}
			}
		}
	}

	// view section — current view's own actions, deduped against global + plugin
	if activeView != nil {
		viewActions := activeView.GetActionRegistry().GetPaletteActions()
		var filtered []controller.Action
		for _, a := range viewActions {
			if globalIDs[a.ID] || pluginIDs[a.ID] {
				continue
			}
			filtered = append(filtered, a)
		}
		if len(filtered) > 0 {
			ap.rows = append(ap.rows, paletteRow{separator: true, label: "View", section: sectionView})
			for _, a := range filtered {
				ap.rows = append(ap.rows, paletteRow{
					action:  a,
					section: sectionView,
					enabled: actionEnabled(a, currentView, activeView),
				})
			}
		}
	}

	ap.filterRows()
}

func actionEnabled(a controller.Action, currentView *controller.ViewEntry, activeView controller.View) bool {
	if a.IsEnabled == nil {
		return true
	}
	return a.IsEnabled(currentView, activeView)
}

func (ap *ActionPalette) filterRows() {
	query := ap.filterInput.GetText()
	ap.visibleRows = nil

	if query == "" {
		for i := range ap.rows {
			ap.visibleRows = append(ap.visibleRows, i)
		}
		ap.stripEmptySections()
		ap.clampSelection()
		return
	}

	type scored struct {
		idx   int
		score int
	}

	// group by section, score each, sort within section
	sectionScored := make(map[sectionType][]scored)
	for i, row := range ap.rows {
		if row.separator {
			continue
		}
		matched, score := fuzzyMatch(query, row.action.Label)
		if matched {
			sectionScored[row.section] = append(sectionScored[row.section], scored{i, score})
		}
	}

	for _, section := range []sectionType{sectionGlobal, sectionViews, sectionView} {
		items := sectionScored[section]
		if len(items) == 0 {
			continue
		}
		sort.Slice(items, func(a, b int) bool {
			if items[a].score != items[b].score {
				return items[a].score < items[b].score
			}
			la := strings.ToLower(ap.rows[items[a].idx].action.Label)
			lb := strings.ToLower(ap.rows[items[b].idx].action.Label)
			if la != lb {
				return la < lb
			}
			return ap.rows[items[a].idx].action.ID < ap.rows[items[b].idx].action.ID
		})

		// find section separator
		for i, row := range ap.rows {
			if row.separator && row.section == section {
				ap.visibleRows = append(ap.visibleRows, i)
				break
			}
		}
		for _, item := range items {
			ap.visibleRows = append(ap.visibleRows, item.idx)
		}
	}

	ap.stripEmptySections()
	ap.clampSelection()
}

// stripEmptySections removes section separators that have no visible action rows after them.
func (ap *ActionPalette) stripEmptySections() {
	var result []int
	for i, vi := range ap.visibleRows {
		row := ap.rows[vi]
		if row.separator {
			// check if next visible row is a non-separator in same section
			hasContent := false
			for j := i + 1; j < len(ap.visibleRows); j++ {
				next := ap.rows[ap.visibleRows[j]]
				if next.separator {
					break
				}
				hasContent = true
				break
			}
			if !hasContent {
				continue
			}
		}
		result = append(result, vi)
	}
	ap.visibleRows = result
}

func (ap *ActionPalette) clampSelection() {
	if ap.selectedIndex >= len(ap.visibleRows) {
		ap.selectedIndex = 0
	}
	// skip to first selectable (non-separator, enabled) row
	ap.selectedIndex = ap.nextSelectableFrom(ap.selectedIndex, 1)
}

func (ap *ActionPalette) nextSelectableFrom(start, direction int) int {
	n := len(ap.visibleRows)
	if n == 0 {
		return 0
	}
	for i := 0; i < n; i++ {
		idx := (start + i*direction + n) % n
		row := ap.rows[ap.visibleRows[idx]]
		if !row.separator && row.enabled {
			return idx
		}
	}
	return start
}

func (ap *ActionPalette) renderList() {
	colors := config.GetColors()
	_, _, width, _ := ap.listView.GetInnerRect()
	if width <= 0 {
		width = PaletteMinWidth
	}
	ap.lastWidth = width

	globalScheme := sectionColors(sectionGlobal)
	viewsScheme := sectionColors(sectionViews)
	viewScheme := sectionColors(sectionView)

	mutedHex := colors.TaskDetailPlaceholderColor.Hex()
	selBgHex := colors.TaskListSelectionBg.Hex()

	var buf strings.Builder

	if len(ap.visibleRows) == 0 {
		buf.WriteString(fmt.Sprintf("[%s]  no matches", mutedHex))
		ap.listView.SetText(buf.String())
		return
	}

	keyColWidth := 12

	for vi, rowIdx := range ap.visibleRows {
		row := ap.rows[rowIdx]

		if row.separator {
			if vi > 0 {
				buf.WriteString("\n")
				line := strings.Repeat("─", width)
				buf.WriteString(fmt.Sprintf("[%s]%s[-]", mutedHex, line))
			}
			continue
		}

		keyStr := util.FormatKeyBinding(row.action.Key, row.action.Rune, row.action.Modifier)
		label := row.action.Label

		// truncate label if needed
		maxLabel := width - keyColWidth - 4
		if maxLabel < 5 {
			maxLabel = 5
		}
		if len([]rune(label)) > maxLabel {
			label = string([]rune(label)[:maxLabel-1]) + "…"
		}

		var scheme sectionColorPair
		switch row.section {
		case sectionGlobal:
			scheme = globalScheme
		case sectionViews:
			scheme = viewsScheme
		case sectionView:
			scheme = viewScheme
		}

		selected := vi == ap.selectedIndex

		if vi > 0 {
			buf.WriteString("\n")
		}

		// build visible text: key column + label
		visibleLen := 1 + keyColWidth + 1 + len([]rune(label)) // leading space + key + space + label
		pad := ""
		if visibleLen < width {
			pad = strings.Repeat(" ", width-visibleLen)
		}

		if !row.enabled {
			buf.WriteString(fmt.Sprintf(" [%s]%-*s %s%s[-]", mutedHex, keyColWidth, keyStr, label, pad))
		} else if selected {
			buf.WriteString(fmt.Sprintf("[%s:%s:b] %-*s[-:-:-][:%s:] %s%s[-:-:-]",
				scheme.keyHex, selBgHex, keyColWidth, keyStr,
				selBgHex, label, pad))
		} else {
			buf.WriteString(fmt.Sprintf(" [%s]%-*s[-] %s%s", scheme.keyHex, keyColWidth, keyStr, label, pad))
		}
	}

	ap.listView.SetText(buf.String())
}

type sectionColorPair struct {
	keyHex   string
	labelHex string
}

func sectionColors(s sectionType) sectionColorPair {
	colors := config.GetColors()
	switch s {
	case sectionGlobal:
		return sectionColorPair{
			keyHex:   colors.HeaderActionGlobalKeyColor.Hex(),
			labelHex: colors.HeaderActionGlobalLabelColor.Hex(),
		}
	case sectionViews:
		return sectionColorPair{
			keyHex:   colors.HeaderActionPluginKeyColor.Hex(),
			labelHex: colors.HeaderActionPluginLabelColor.Hex(),
		}
	case sectionView:
		return sectionColorPair{
			keyHex:   colors.HeaderActionViewKeyColor.Hex(),
			labelHex: colors.HeaderActionViewLabelColor.Hex(),
		}
	default:
		return sectionColorPair{
			keyHex:   colors.HeaderActionGlobalKeyColor.Hex(),
			labelHex: colors.HeaderActionGlobalLabelColor.Hex(),
		}
	}
}

// handleFilterInput owns all palette keyboard behavior.
func (ap *ActionPalette) handleFilterInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		ap.paletteConfig.SetVisible(false)
		return nil

	case tcell.KeyEnter:
		ap.dispatchSelected()
		return nil

	case tcell.KeyUp:
		ap.moveSelection(-1)
		ap.renderList()
		return nil

	case tcell.KeyDown:
		ap.moveSelection(1)
		ap.renderList()
		return nil

	case tcell.KeyCtrlU:
		ap.filterInput.SetText("")
		ap.filterRows()
		ap.renderList()
		return nil

	case tcell.KeyRune:
		// let the input field handle the rune, then re-filter
		return event

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return event

	default:
		// swallow everything else
		return nil
	}
}

// SetChangedFunc wires a callback that re-filters when the input text changes.
func (ap *ActionPalette) SetChangedFunc() {
	ap.filterInput.SetChangedFunc(func(text string) {
		ap.filterRows()
		ap.renderList()
	})
}

func (ap *ActionPalette) moveSelection(direction int) {
	n := len(ap.visibleRows)
	if n == 0 {
		return
	}
	start := ap.selectedIndex + direction
	if start < 0 {
		start = n - 1
	} else if start >= n {
		start = 0
	}
	ap.selectedIndex = ap.nextSelectableFrom(start, direction)
}

func (ap *ActionPalette) dispatchSelected() {
	if ap.selectedIndex >= len(ap.visibleRows) {
		ap.paletteConfig.SetVisible(false)
		return
	}

	row := ap.rows[ap.visibleRows[ap.selectedIndex]]
	if row.separator || !row.enabled {
		return
	}

	actionID := row.action.ID

	// close palette BEFORE dispatch (clean focus transition)
	ap.paletteConfig.SetVisible(false)

	// try view-local handler first
	if activeView := ap.navController.GetActiveView(); activeView != nil {
		if handler, ok := activeView.(controller.PaletteActionHandler); ok {
			if handler.HandlePaletteAction(actionID) {
				return
			}
		}
	}

	// fall back to controller-side dispatch
	ap.inputRouter.HandleAction(actionID, ap.navController.CurrentView())
}
