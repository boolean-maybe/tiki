package model

import (
	"log/slog"
	"sync"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/task"
)

// ViewMode represents the display mode for task boxes
type ViewMode string

const (
	ViewModeCompact  ViewMode = "compact"  // 3-line display (5 total height with border)
	ViewModeExpanded ViewMode = "expanded" // 7-line display (9 total height with border)
)

// PluginSelectionListener is called when plugin selection changes
type PluginSelectionListener func()

// PluginConfig holds selection state for a plugin view
type PluginConfig struct {
	mu               sync.RWMutex
	pluginName       string
	selectedLane     int
	selectedIndices  []int
	laneColumns      []int
	scrollOffsets    []int // per-lane viewport position (top visible row)
	preSearchLane    int
	preSearchIndices []int
	viewMode         ViewMode // compact or expanded display
	configIndex      int      // index in workflow.yaml views array (-1 if not from a config file)
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
	pc.SetLaneLayout([]int{4})
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

// SetLaneLayout configures lane columns and resets selection state as needed.
func (pc *PluginConfig) SetLaneLayout(columns []int) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.laneColumns = normalizeLaneColumns(columns)
	pc.selectedIndices = ensureSelectionLength(pc.selectedIndices, len(pc.laneColumns))
	pc.preSearchIndices = ensureSelectionLength(pc.preSearchIndices, len(pc.laneColumns))
	pc.scrollOffsets = ensureSelectionLength(pc.scrollOffsets, len(pc.laneColumns))

	if pc.selectedLane < 0 || pc.selectedLane >= len(pc.laneColumns) {
		pc.selectedLane = 0
	}
}

// GetLaneCount returns the number of lanes.
func (pc *PluginConfig) GetLaneCount() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return len(pc.laneColumns)
}

// GetSelectedLane returns the selected lane index.
func (pc *PluginConfig) GetSelectedLane() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.selectedLane
}

// SetSelectedLane sets the selected lane index.
func (pc *PluginConfig) SetSelectedLane(lane int) {
	pc.mu.Lock()
	if lane < 0 || lane >= len(pc.laneColumns) {
		pc.mu.Unlock()
		return
	}
	changed := pc.selectedLane != lane
	pc.selectedLane = lane
	pc.mu.Unlock()
	if changed {
		pc.notifyListeners()
	}
}

// GetSelectedIndex returns the selected task index for the current lane.
func (pc *PluginConfig) GetSelectedIndex() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.indexForLane(pc.selectedLane)
}

// GetSelectedIndexForLane returns the selected index for a lane.
func (pc *PluginConfig) GetSelectedIndexForLane(lane int) int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.indexForLane(lane)
}

// SetSelectedIndex sets the selected task index for the current lane.
func (pc *PluginConfig) SetSelectedIndex(idx int) {
	pc.mu.Lock()
	pc.setIndexForLane(pc.selectedLane, idx)
	pc.mu.Unlock()
	pc.notifyListeners()
}

// SetSelectedIndexForLane sets the selected index for a specific lane.
func (pc *PluginConfig) SetSelectedIndexForLane(lane int, idx int) {
	pc.mu.Lock()
	pc.setIndexForLane(lane, idx)
	pc.mu.Unlock()
	pc.notifyListeners()
}

// GetScrollOffsetForLane returns the scroll offset (top visible row) for a lane.
func (pc *PluginConfig) GetScrollOffsetForLane(lane int) int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	if lane < 0 || lane >= len(pc.scrollOffsets) {
		return 0
	}
	return pc.scrollOffsets[lane]
}

// SetScrollOffsetForLane sets the scroll offset for a specific lane.
func (pc *PluginConfig) SetScrollOffsetForLane(lane int, offset int) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if lane < 0 || lane >= len(pc.scrollOffsets) {
		return
	}
	pc.scrollOffsets[lane] = offset
}

func (pc *PluginConfig) SetSelectedLaneAndIndex(lane int, idx int) {
	pc.mu.Lock()
	if lane < 0 || lane >= len(pc.selectedIndices) {
		pc.mu.Unlock()
		return
	}
	if len(pc.selectedIndices) == 0 {
		pc.mu.Unlock()
		return
	}
	changed := pc.selectedLane != lane || pc.selectedIndices[lane] != idx
	pc.selectedLane = lane
	pc.selectedIndices[lane] = idx
	pc.mu.Unlock()

	if changed {
		pc.notifyListeners()
	}
}

// GetColumnsForLane returns the number of grid columns for a lane.
func (pc *PluginConfig) GetColumnsForLane(lane int) int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.columnsForLane(lane)
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

// MoveSelection moves selection in a direction within the current lane.
func (pc *PluginConfig) MoveSelection(direction string, taskCount int) bool {
	if taskCount == 0 {
		return false
	}

	pc.mu.Lock()
	lane := pc.selectedLane
	columns := pc.columnsForLane(lane)
	oldIndex := pc.indexForLane(lane)
	row := oldIndex / columns
	col := oldIndex % columns
	numRows := (taskCount + columns - 1) / columns

	switch direction {
	case "up":
		if row > 0 {
			pc.setIndexForLane(lane, oldIndex-columns)
		}
	case "down":
		newIdx := oldIndex + columns
		if row < numRows-1 && newIdx < taskCount {
			pc.setIndexForLane(lane, newIdx)
		}
	case "left":
		if col > 0 {
			pc.setIndexForLane(lane, oldIndex-1)
		}
	case "right":
		if col < columns-1 && oldIndex+1 < taskCount {
			pc.setIndexForLane(lane, oldIndex+1)
		}
	}

	moved := pc.indexForLane(lane) != oldIndex
	pc.mu.Unlock()

	if moved {
		pc.notifyListeners()
	}
	return moved
}

// ClampSelection ensures selection is within bounds for the current lane.
func (pc *PluginConfig) ClampSelection(taskCount int) {
	pc.mu.Lock()
	lane := pc.selectedLane
	index := pc.indexForLane(lane)
	if index >= taskCount {
		pc.setIndexForLane(lane, taskCount-1)
	}
	if pc.indexForLane(lane) < 0 {
		pc.setIndexForLane(lane, 0)
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
	pc.preSearchLane = pc.selectedLane
	pc.preSearchIndices = ensureSelectionLength(pc.preSearchIndices, len(pc.laneColumns))
	copy(pc.preSearchIndices, pc.selectedIndices)
	selectedIndex := pc.indexForLane(pc.selectedLane)
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
	if len(pc.preSearchIndices) == len(pc.laneColumns) {
		pc.selectedIndices = ensureSelectionLength(pc.selectedIndices, len(pc.laneColumns))
		copy(pc.selectedIndices, pc.preSearchIndices)
		pc.selectedLane = pc.preSearchLane
	} else if len(pc.selectedIndices) > 0 {
		pc.selectedLane = 0
		pc.setIndexForLane(0, 0)
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

func (pc *PluginConfig) indexForLane(lane int) int {
	if len(pc.selectedIndices) == 0 {
		return 0
	}
	if lane < 0 || lane >= len(pc.selectedIndices) {
		slog.Warn("lane index out of range", "lane", lane, "count", len(pc.selectedIndices))
		return 0
	}
	return pc.selectedIndices[lane]
}

func (pc *PluginConfig) setIndexForLane(lane int, idx int) {
	if len(pc.selectedIndices) == 0 {
		return
	}
	if lane < 0 || lane >= len(pc.selectedIndices) {
		slog.Warn("lane index out of range", "lane", lane, "count", len(pc.selectedIndices))
		return
	}
	pc.selectedIndices[lane] = idx
}

func (pc *PluginConfig) columnsForLane(lane int) int {
	if len(pc.laneColumns) == 0 {
		return 1
	}
	if lane < 0 || lane >= len(pc.laneColumns) {
		slog.Warn("lane columns out of range", "lane", lane, "count", len(pc.laneColumns))
		return 1
	}
	return pc.laneColumns[lane]
}

func normalizeLaneColumns(columns []int) []int {
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
