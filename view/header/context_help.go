package header

import (
	"fmt"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/util"
	"github.com/boolean-maybe/tiki/view/grid"

	"github.com/rivo/tview"
)

// cellData holds data for a single cell in the action grid
type cellData struct {
	key       string
	label     string
	keyLen    int
	labelLen  int
	colorType int // 0=global, 1=plugin, 2=view
	enabled   bool
}

const (
	colorTypeGlobal = 0
	colorTypePlugin = 1
	colorTypeView   = 2
)

// ContextHelpWidget displays keyboard shortcuts in a three-section grid layout
type ContextHelpWidget struct {
	*tview.TextView
	width int // calculated visible width of content
}

// NewContextHelpWidget creates a new context help display widget
func NewContextHelpWidget() *ContextHelpWidget {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetTextAlign(tview.AlignLeft)
	tv.SetWrap(false)

	return &ContextHelpWidget{
		TextView: tv,
		width:    0,
	}
}

// GetWidth returns the current calculated width of the content
func (chw *ContextHelpWidget) GetWidth() int {
	return chw.width
}

// SetActionsFromModel updates the display with actions from model.HeaderAction.
// All three sections (global, plugin, view) flow through the same model.HeaderAction
// path so enabled/disabled state is consistent.
func (chw *ContextHelpWidget) SetActionsFromModel(globalActions, viewActions, pluginActions []model.HeaderAction) int {
	globalIDs := make(map[controller.ActionID]bool)
	for _, a := range globalActions {
		globalIDs[controller.ActionID(a.ID)] = true
	}

	globalConverted := convertHeaderActions(globalActions)
	pluginConverted := convertHeaderActions(pluginActions)
	viewConverted := extractViewActionsFromModel(viewActions, globalIDs)

	globalEnabled := extractEnabledMap(globalActions)
	pluginEnabled := extractEnabledMap(pluginActions)
	viewEnabled := extractEnabledMap(viewActions)

	return chw.renderActionsGridWithEnabled(globalConverted, pluginConverted, viewConverted, globalEnabled, pluginEnabled, viewEnabled)
}

// Primitive returns the underlying tview primitive
func (chw *ContextHelpWidget) Primitive() tview.Primitive {
	return chw.TextView
}

// renderActionsGridWithEnabled renders the grid with per-action enabled state.
func (chw *ContextHelpWidget) renderActionsGridWithEnabled(
	globalActions, pluginActions, viewActions []controller.Action,
	globalEnabled, pluginEnabled, viewEnabled map[controller.ActionID]bool,
) int {
	numRows := HeaderHeight

	// Pad actions to complete columns
	globalActions = grid.PadToFullRows(globalActions, numRows)
	if len(pluginActions) > 0 {
		pluginActions = grid.PadToFullRows(pluginActions, numRows)
	}

	// Calculate grid dimensions
	dims := calculateGridDimensions(globalActions, pluginActions, viewActions, numRows)
	if dims.totalCols == 0 {
		chw.SetText("")
		chw.width = 0
		return 0
	}

	// Create and populate grid with enabled state
	gridData := createEmptyGrid(numRows, dims.totalCols)
	fillGridSectionWithEnabled(gridData, globalActions, 0, numRows, colorTypeGlobal, globalEnabled)
	fillGridSectionWithEnabled(gridData, pluginActions, dims.globalCols, numRows, colorTypePlugin, pluginEnabled)
	fillGridSectionWithEnabled(gridData, viewActions, dims.globalCols+dims.pluginCols, numRows, colorTypeView, viewEnabled)

	// Calculate column widths
	maxKeyLenPerCol := calculateMaxLengths(gridData, dims.totalCols, numRows, func(cell cellData) int { return cell.keyLen })
	maxLabelLenPerCol := calculateMaxLengths(gridData, dims.totalCols, numRows, func(cell cellData) int { return cell.labelLen })

	// Render grid to text
	lines := buildOutputLines(gridData, maxKeyLenPerCol, maxLabelLenPerCol, numRows, dims.totalCols)
	chw.SetText(" " + strings.Join(lines, "\n "))

	// Calculate and store width
	chw.width = calculateMaxLineWidth(lines) + 1
	return chw.width
}

// gridDimensions holds calculated grid layout dimensions
type gridDimensions struct {
	globalCols int
	pluginCols int
	viewCols   int
	totalCols  int
}

// calculateGridDimensions calculates how many columns are needed for each section
func calculateGridDimensions(globalActions, pluginActions, viewActions []controller.Action, numRows int) gridDimensions {
	globalCols := len(globalActions) / numRows

	pluginCols := 0
	if len(pluginActions) > 0 {
		pluginCols = len(pluginActions) / numRows
	}

	viewCols := 0
	if len(viewActions) > 0 {
		viewCols = (len(viewActions) + numRows - 1) / numRows
	}

	return gridDimensions{
		globalCols: globalCols,
		pluginCols: pluginCols,
		viewCols:   viewCols,
		totalCols:  globalCols + pluginCols + viewCols,
	}
}

// createEmptyGrid creates a 2D grid of cellData initialized to zero values
func createEmptyGrid(numRows, numCols int) [][]cellData {
	gridData := make([][]cellData, numRows)
	for i := range gridData {
		gridData[i] = make([]cellData, numCols)
	}
	return gridData
}

// fillGridSectionWithEnabled fills a section with per-action enabled state.
func fillGridSectionWithEnabled(gridData [][]cellData, actions []controller.Action, colOffset, numRows, colorType int, enabledMap map[controller.ActionID]bool) {
	for i, action := range actions {
		if action.ID == "" {
			continue
		}

		col := colOffset + i/numRows
		row := i % numRows
		keyStr := util.FormatKeyBinding(action.Key, action.Rune, action.Modifier)

		enabled := true
		if e, ok := enabledMap[action.ID]; ok {
			enabled = e
		}

		gridData[row][col] = cellData{
			key:       keyStr,
			label:     action.Label,
			keyLen:    len([]rune(keyStr)) + 2,
			labelLen:  len([]rune(action.Label)),
			colorType: colorType,
			enabled:   enabled,
		}
	}
}

// calculateMaxLengths finds the maximum value for each column using the provided extractor function
func calculateMaxLengths(gridData [][]cellData, numCols, numRows int, extractor func(cellData) int) []int {
	maxLengths := make([]int, numCols)
	for col := 0; col < numCols; col++ {
		maxLen := 0
		for row := 0; row < numRows; row++ {
			if length := extractor(gridData[row][col]); length > maxLen {
				maxLen = length
			}
		}
		maxLengths[col] = maxLen
	}
	return maxLengths
}

// buildOutputLines converts the grid data into formatted text lines
func buildOutputLines(
	gridData [][]cellData,
	maxKeyLenPerCol, maxLabelLenPerCol []int,
	numRows, numCols int,
) []string {
	lines := make([]string, numRows)
	for row := 0; row < numRows; row++ {
		lines[row] = buildGridRow(gridData[row], maxKeyLenPerCol, maxLabelLenPerCol, numCols)
	}
	return lines
}

// buildGridRow builds a single row of the grid output
func buildGridRow(rowData []cellData, maxKeyLenPerCol, maxLabelLenPerCol []int, numCols int) string {
	var line strings.Builder

	for col := 0; col < numCols; col++ {
		cell := rowData[col]

		if cell.key == "" {
			// Empty cell - add padding if not last column
			if col < numCols-1 {
				colWidth := maxKeyLenPerCol[col] + 1 + maxLabelLenPerCol[col] + HeaderColumnSpacing
				line.WriteString(strings.Repeat(" ", colWidth))
			}
			continue
		}

		if !cell.enabled {
			mutedTag := config.GetColors().TaskDetailPlaceholderColor.Tag().String()
			fmt.Fprintf(&line, "%s<%s>%s", mutedTag, cell.key, mutedTag)
		} else {
			scheme := getColorScheme(cell.colorType)
			fmt.Fprintf(&line, "%s<%s>%s", scheme.KeyColor.Tag().String(), cell.key, scheme.LabelColor.Tag().String())
		}

		// Add key padding
		if keyPadding := maxKeyLenPerCol[col] - cell.keyLen; keyPadding > 0 {
			line.WriteString(strings.Repeat(" ", keyPadding))
		}

		// Add label
		line.WriteString(" ")
		line.WriteString(cell.label)

		// Add label padding if not last column
		if col < numCols-1 {
			labelPadding := maxLabelLenPerCol[col] - cell.labelLen + HeaderColumnSpacing
			if labelPadding > 0 {
				line.WriteString(strings.Repeat(" ", labelPadding))
			}
		}
	}

	return line.String()
}

// calculateMaxLineWidth finds the maximum visible width among all lines
func calculateMaxLineWidth(lines []string) int {
	maxWidth := 0
	for _, line := range lines {
		if w := visibleWidthIgnoringTviewTags(line); w > maxWidth {
			maxWidth = w
		}
	}
	return maxWidth
}

// visibleWidthIgnoringTviewTags calculates the visible width of a string with tview tags
func visibleWidthIgnoringTviewTags(s string) int {
	visibleCount := 0
	inTag := false
	for _, r := range s {
		if r == '[' {
			inTag = true
			continue
		}
		if inTag && r == ']' {
			inTag = false
			continue
		}
		if !inTag {
			visibleCount++
		}
	}
	return visibleCount
}
