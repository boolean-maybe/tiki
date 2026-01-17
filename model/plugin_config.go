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
	mu             sync.RWMutex
	pluginName     string
	selectedIndex  int
	columns        int      // number of columns in grid (same as backlog: 4)
	viewMode       ViewMode // compact or expanded display
	configIndex    int      // index in config.yaml plugins array (-1 if embedded/not in config)
	listeners      map[int]PluginSelectionListener
	nextListenerID int
	searchState    SearchState // search state (embedded)
}

// NewPluginConfig creates a plugin config
func NewPluginConfig(name string) *PluginConfig {
	return &PluginConfig{
		pluginName:     name,
		columns:        4,
		viewMode:       ViewModeCompact,
		configIndex:    -1, // Default to -1 (not in config)
		listeners:      make(map[int]PluginSelectionListener),
		nextListenerID: 1, // Start at 1 to avoid conflict with zero-value sentinel
	}
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

// GetSelectedIndex returns the selected task index
func (pc *PluginConfig) GetSelectedIndex() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.selectedIndex
}

// SetSelectedIndex sets the selected task index
func (pc *PluginConfig) SetSelectedIndex(idx int) {
	pc.mu.Lock()
	pc.selectedIndex = idx
	pc.mu.Unlock()
	pc.notifyListeners()
}

// GetColumns returns the number of grid columns
func (pc *PluginConfig) GetColumns() int {
	return pc.columns
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

// MoveSelection moves selection in a direction given task count, returns true if moved
func (pc *PluginConfig) MoveSelection(direction string, taskCount int) bool {
	if taskCount == 0 {
		return false
	}

	pc.mu.Lock()
	oldIndex := pc.selectedIndex
	row := pc.selectedIndex / pc.columns
	col := pc.selectedIndex % pc.columns
	numRows := (taskCount + pc.columns - 1) / pc.columns

	switch direction {
	case "up":
		if row > 0 {
			pc.selectedIndex -= pc.columns
		}
	case "down":
		newIdx := pc.selectedIndex + pc.columns
		if row < numRows-1 && newIdx < taskCount {
			pc.selectedIndex = newIdx
		}
	case "left":
		if col > 0 {
			pc.selectedIndex--
		}
	case "right":
		if col < pc.columns-1 && pc.selectedIndex+1 < taskCount {
			pc.selectedIndex++
		}
	}

	moved := pc.selectedIndex != oldIndex
	pc.mu.Unlock()

	if moved {
		pc.notifyListeners()
	}
	return moved
}

// ClampSelection ensures selection is within bounds
func (pc *PluginConfig) ClampSelection(taskCount int) {
	pc.mu.Lock()
	if pc.selectedIndex >= taskCount {
		pc.selectedIndex = taskCount - 1
	}
	if pc.selectedIndex < 0 {
		pc.selectedIndex = 0
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
	pc.mu.RLock()
	selectedIndex := pc.selectedIndex
	pc.mu.RUnlock()
	pc.searchState.SavePreSearchState(selectedIndex)
}

// SetSearchResults sets filtered search results and query
func (pc *PluginConfig) SetSearchResults(results []task.SearchResult, query string) {
	pc.searchState.SetSearchResults(results, query)
	pc.notifyListeners()
}

// ClearSearchResults clears search and restores pre-search selection
func (pc *PluginConfig) ClearSearchResults() {
	preSearchIndex, _, _ := pc.searchState.ClearSearchResults()
	pc.mu.Lock()
	pc.selectedIndex = preSearchIndex
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
