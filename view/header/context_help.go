package header

import (
	"fmt"
	"strings"

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

// SetActions updates the display with the given action registry (backward compatible)
// Returns the calculated visible width of the rendered content
func (chw *ContextHelpWidget) SetActions(registry *controller.ActionRegistry) int {
	if registry == nil {
		chw.SetText("")
		chw.width = 0
		return 0
	}

	// Section 1: Global actions (always present)
	globalRegistry := controller.DefaultGlobalActions()
	var globalActions []controller.Action
	globalIDs := make(map[controller.ActionID]bool)
	for _, action := range globalRegistry.GetHeaderActions() {
		globalActions = append(globalActions, action)
		globalIDs[action.ID] = true
	}

	// Section 2: Plugin "go to" actions (from centralized registry)
	pluginActions := controller.GetPluginActions().GetHeaderActions()

	// Section 3: View-specific actions (exclude globals and plugin actions)
	viewActions := extractViewActions(registry, globalIDs)

	return chw.renderActionsGrid(globalActions, pluginActions, viewActions)
}

// GetWidth returns the current calculated width of the content
func (chw *ContextHelpWidget) GetWidth() int {
	return chw.width
}

// SetActionsFromModel updates the display with actions from model.HeaderAction
// This is the new model-based interface for the refactored architecture.
func (chw *ContextHelpWidget) SetActionsFromModel(viewActions, pluginActions []model.HeaderAction) int {
	// Section 1: Global actions (always present)
	globalRegistry := controller.DefaultGlobalActions()
	var globalControllerActions []controller.Action
	globalIDs := make(map[controller.ActionID]bool)
	for _, action := range globalRegistry.GetHeaderActions() {
		globalControllerActions = append(globalControllerActions, action)
		globalIDs[action.ID] = true
	}

	// Section 2: Plugin "go to" actions (from HeaderConfig via model.HeaderAction)
	pluginControllerActions := convertHeaderActions(pluginActions)

	// Section 3: View-specific actions (from HeaderConfig via model.HeaderAction)
	viewControllerActions := extractViewActionsFromModel(viewActions, globalIDs)

	return chw.renderActionsGrid(globalControllerActions, pluginControllerActions, viewControllerActions)
}

// Primitive returns the underlying tview primitive
func (chw *ContextHelpWidget) Primitive() tview.Primitive {
	return chw.TextView
}

// renderActionsGrid renders the actions grid - the core rendering logic shared by both methods
func (chw *ContextHelpWidget) renderActionsGrid(
	globalActions, pluginActions, viewActions []controller.Action,
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

	// Create and populate grid
	gridData := createEmptyGrid(numRows, dims.totalCols)
	populateGridCells(gridData, globalActions, pluginActions, viewActions, dims, numRows)

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

// populateGridCells fills the grid with action data from all three sections
func populateGridCells(
	gridData [][]cellData,
	globalActions, pluginActions, viewActions []controller.Action,
	dims gridDimensions,
	numRows int,
) {
	// Fill global actions
	fillGridSection(gridData, globalActions, 0, numRows, colorTypeGlobal)

	// Fill plugin actions
	fillGridSection(gridData, pluginActions, dims.globalCols, numRows, colorTypePlugin)

	// Fill view actions
	fillGridSection(gridData, viewActions, dims.globalCols+dims.pluginCols, numRows, colorTypeView)
}

// fillGridSection fills a section of the grid with actions of a specific color type
func fillGridSection(gridData [][]cellData, actions []controller.Action, colOffset, numRows, colorType int) {
	for i, action := range actions {
		if action.ID == "" {
			continue // skip empty padding cells
		}

		col := colOffset + i/numRows
		row := i % numRows
		keyStr := util.FormatKeyBinding(action.Key, action.Rune, action.Modifier)

		gridData[row][col] = cellData{
			key:       keyStr,
			label:     action.Label,
			keyLen:    len([]rune(keyStr)) + 2,
			labelLen:  len([]rune(action.Label)),
			colorType: colorType,
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

		// Render cell with colors
		scheme := getColorScheme(cell.colorType)
		line.WriteString(fmt.Sprintf("[%s]<%s>[%s]", scheme.KeyColor, cell.key, scheme.LabelColor))

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
