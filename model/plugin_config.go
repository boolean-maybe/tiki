package model

import (
	"log/slog"
	"sync"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/task"
)

// PluginSelectionListener is called when plugin selection changes
type PluginSelectionListener func()

// PluginConfig holds selection state for a plugin view
type PluginConfig struct {
	mu               sync.RWMutex
	pluginName       string
	selectedPane     int
	selectedIndices  []int
	paneColumns      []int
	preSearchPane    int
	preSearchIndices []int
	viewMode         ViewMode // compact or expanded display
	configIndex      int      // index in config.yaml plugins array (-1 if embedded/not in config)
	listeners        map[int]PluginSelectionListener
	nextListenerID   int
	searchState      SearchState // search state (embedded)
}

// NewPluginConfig creates a plugin config
func NewPluginConfig(name string) *PluginConfig {
	pc := &PluginConfig{
		pluginName:     name,
		viewMode:       ViewModeCompact,
		configIndex:    -1, // Default to -1 (not in config)
		listeners:      make(map[int]PluginSelectionListener),
		nextListenerID: 1, // Start at 1 to avoid conflict with zero-value sentinel
	}
	pc.SetPaneLayout([]int{4})
	return pc
}

// SetConfigIndex sets the config index for this plugin
func (pc *PluginConfig) SetConfigIndex(index int) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.configIndex = index
}

// GetPluginName returns the plugin name
func (pc *PluginConfig) GetPluginName() string {
	return pc.pluginName
}

// SetPaneLayout configures pane columns and resets selection state as needed.
func (pc *PluginConfig) SetPaneLayout(columns []int) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.paneColumns = normalizePaneColumns(columns)
	pc.selectedIndices = ensureSelectionLength(pc.selectedIndices, len(pc.paneColumns))
	pc.preSearchIndices = ensureSelectionLength(pc.preSearchIndices, len(pc.paneColumns))

	if pc.selectedPane < 0 || pc.selectedPane >= len(pc.paneColumns) {
		pc.selectedPane = 0
	}
}

// GetPaneCount returns the number of panes.
func (pc *PluginConfig) GetPaneCount() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return len(pc.paneColumns)
}

// GetSelectedPane returns the selected pane index.
func (pc *PluginConfig) GetSelectedPane() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.selectedPane
}

// SetSelectedPane sets the selected pane index.
func (pc *PluginConfig) SetSelectedPane(pane int) {
	pc.mu.Lock()
	if pane < 0 || pane >= len(pc.paneColumns) {
		pc.mu.Unlock()
		return
	}
	changed := pc.selectedPane != pane
	pc.selectedPane = pane
	pc.mu.Unlock()
	if changed {
		pc.notifyListeners()
	}
}

// GetSelectedIndex returns the selected task index for the current pane.
func (pc *PluginConfig) GetSelectedIndex() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.indexForPane(pc.selectedPane)
}

// GetSelectedIndexForPane returns the selected index for a pane.
func (pc *PluginConfig) GetSelectedIndexForPane(pane int) int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.indexForPane(pane)
}

// SetSelectedIndex sets the selected task index for the current pane.
func (pc *PluginConfig) SetSelectedIndex(idx int) {
	pc.mu.Lock()
	pc.setIndexForPane(pc.selectedPane, idx)
	pc.mu.Unlock()
	pc.notifyListeners()
}

// SetSelectedIndexForPane sets the selected index for a specific pane.
func (pc *PluginConfig) SetSelectedIndexForPane(pane int, idx int) {
	pc.mu.Lock()
	pc.setIndexForPane(pane, idx)
	pc.mu.Unlock()
	pc.notifyListeners()
}

func (pc *PluginConfig) SetSelectedPaneAndIndex(pane int, idx int) {
	pc.mu.Lock()
	if pane < 0 || pane >= len(pc.selectedIndices) {
		pc.mu.Unlock()
		return
	}
	if len(pc.selectedIndices) == 0 {
		pc.mu.Unlock()
		return
	}
	changed := pc.selectedPane != pane || pc.selectedIndices[pane] != idx
	pc.selectedPane = pane
	pc.selectedIndices[pane] = idx
	pc.mu.Unlock()

	if changed {
		pc.notifyListeners()
	}
}

// GetColumnsForPane returns the number of grid columns for a pane.
func (pc *PluginConfig) GetColumnsForPane(pane int) int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.columnsForPane(pane)
}

// AddSelectionListener registers a callback for selection changes
func (pc *PluginConfig) AddSelectionListener(listener PluginSelectionListener) int {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	id := pc.nextListenerID
	pc.nextListenerID++
	pc.listeners[id] = listener
	return id
}

// RemoveSelectionListener removes a listener by ID
func (pc *PluginConfig) RemoveSelectionListener(id int) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	delete(pc.listeners, id)
}

func (pc *PluginConfig) notifyListeners() {
	pc.mu.RLock()
	listeners := make([]PluginSelectionListener, 0, len(pc.listeners))
	for _, l := range pc.listeners {
		listeners = append(listeners, l)
	}
	pc.mu.RUnlock()

	for _, l := range listeners {
		l()
	}
}

// MoveSelection moves selection in a direction within the current pane.
func (pc *PluginConfig) MoveSelection(direction string, taskCount int) bool {
	if taskCount == 0 {
		return false
	}

	pc.mu.Lock()
	pane := pc.selectedPane
	columns := pc.columnsForPane(pane)
	oldIndex := pc.indexForPane(pane)
	row := oldIndex / columns
	col := oldIndex % columns
	numRows := (taskCount + columns - 1) / columns

	switch direction {
	case "up":
		if row > 0 {
			pc.setIndexForPane(pane, oldIndex-columns)
		}
	case "down":
		newIdx := oldIndex + columns
		if row < numRows-1 && newIdx < taskCount {
			pc.setIndexForPane(pane, newIdx)
		}
	case "left":
		if col > 0 {
			pc.setIndexForPane(pane, oldIndex-1)
		}
	case "right":
		if col < columns-1 && oldIndex+1 < taskCount {
			pc.setIndexForPane(pane, oldIndex+1)
		}
	}

	moved := pc.indexForPane(pane) != oldIndex
	pc.mu.Unlock()

	if moved {
		pc.notifyListeners()
	}
	return moved
}

// ClampSelection ensures selection is within bounds for the current pane.
func (pc *PluginConfig) ClampSelection(taskCount int) {
	pc.mu.Lock()
	pane := pc.selectedPane
	index := pc.indexForPane(pane)
	if index >= taskCount {
		pc.setIndexForPane(pane, taskCount-1)
	}
	if pc.indexForPane(pane) < 0 {
		pc.setIndexForPane(pane, 0)
	}
	pc.mu.Unlock()
}

// GetViewMode returns the current view mode
func (pc *PluginConfig) GetViewMode() ViewMode {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.viewMode
}

// ToggleViewMode switches between compact and expanded view modes
func (pc *PluginConfig) ToggleViewMode() {
	pc.mu.Lock()
	if pc.viewMode == ViewModeCompact {
		pc.viewMode = ViewModeExpanded
	} else {
		pc.viewMode = ViewModeCompact
	}
	newMode := pc.viewMode
	pluginName := pc.pluginName
	configIndex := pc.configIndex
	pc.mu.Unlock()

	// Save to config (same pattern as BoardConfig)
	if err := config.SavePluginViewMode(pluginName, configIndex, string(newMode)); err != nil {
		slog.Error("failed to save plugin view mode", "plugin", pluginName, "error", err)
	}

	pc.notifyListeners()
}

// SetViewMode sets the view mode from a string value
func (pc *PluginConfig) SetViewMode(mode string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if mode == "expanded" {
		pc.viewMode = ViewModeExpanded
	} else {
		pc.viewMode = ViewModeCompact
	}
}

// SavePreSearchState saves current selection for later restoration
func (pc *PluginConfig) SavePreSearchState() {
	pc.mu.Lock()
	pc.preSearchPane = pc.selectedPane
	pc.preSearchIndices = ensureSelectionLength(pc.preSearchIndices, len(pc.paneColumns))
	copy(pc.preSearchIndices, pc.selectedIndices)
	selectedIndex := pc.indexForPane(pc.selectedPane)
	pc.mu.Unlock()
	pc.searchState.SavePreSearchState(selectedIndex)
}

// SetSearchResults sets filtered search results and query
func (pc *PluginConfig) SetSearchResults(results []task.SearchResult, query string) {
	pc.searchState.SetSearchResults(results, query)
	pc.notifyListeners()
}

// ClearSearchResults clears search and restores pre-search selection
func (pc *PluginConfig) ClearSearchResults() {
	pc.searchState.ClearSearchResults()
	pc.mu.Lock()
	if len(pc.preSearchIndices) == len(pc.paneColumns) {
		pc.selectedIndices = ensureSelectionLength(pc.selectedIndices, len(pc.paneColumns))
		copy(pc.selectedIndices, pc.preSearchIndices)
		pc.selectedPane = pc.preSearchPane
	} else if len(pc.selectedIndices) > 0 {
		pc.selectedPane = 0
		pc.setIndexForPane(0, 0)
	}
	pc.mu.Unlock()
	pc.notifyListeners()
}

// GetSearchResults returns current search results (nil if no search active)
func (pc *PluginConfig) GetSearchResults() []task.SearchResult {
	return pc.searchState.GetSearchResults()
}

// IsSearchActive returns true if search is currently active
func (pc *PluginConfig) IsSearchActive() bool {
	return pc.searchState.IsSearchActive()
}

// GetSearchQuery returns the current search query
func (pc *PluginConfig) GetSearchQuery() string {
	return pc.searchState.GetSearchQuery()
}

func (pc *PluginConfig) indexForPane(pane int) int {
	if len(pc.selectedIndices) == 0 {
		return 0
	}
	if pane < 0 || pane >= len(pc.selectedIndices) {
		slog.Warn("pane index out of range", "pane", pane, "count", len(pc.selectedIndices))
		return 0
	}
	return pc.selectedIndices[pane]
}

func (pc *PluginConfig) setIndexForPane(pane int, idx int) {
	if len(pc.selectedIndices) == 0 {
		return
	}
	if pane < 0 || pane >= len(pc.selectedIndices) {
		slog.Warn("pane index out of range", "pane", pane, "count", len(pc.selectedIndices))
		return
	}
	pc.selectedIndices[pane] = idx
}

func (pc *PluginConfig) columnsForPane(pane int) int {
	if len(pc.paneColumns) == 0 {
		return 1
	}
	if pane < 0 || pane >= len(pc.paneColumns) {
		slog.Warn("pane columns out of range", "pane", pane, "count", len(pc.paneColumns))
		return 1
	}
	return pc.paneColumns[pane]
}

func normalizePaneColumns(columns []int) []int {
	if len(columns) == 0 {
		return []int{1}
	}
	normalized := make([]int, len(columns))
	for i, value := range columns {
		if value <= 0 {
			normalized[i] = 1
		} else {
			normalized[i] = value
		}
	}
	return normalized
}

func ensureSelectionLength(current []int, size int) []int {
	if size <= 0 {
		return []int{}
	}
	if len(current) == size {
		return current
	}
	next := make([]int, size)
	copy(next, current)
	return next
}
