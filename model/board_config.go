package model

import (
	"log/slog"
	"sync"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/task"
)

// BoardConfig defines board panes, status-to-pane mappings, and selection state.
// It tracks which pane and row is currently selected.

// SelectionListener is called when board selection changes
type SelectionListener func()

// BoardConfig holds pane definitions and status mappings for the board view
type BoardConfig struct {
	mu             sync.RWMutex // protects selectedPaneID and selectedRow
	panes          []*Pane
	statusToPane   map[task.Status]string    // status -> pane ID
	paneToStatus   map[string]task.Status    // pane ID -> status
	selectedPaneID string                    // currently selected pane
	selectedRow    int                       // selected task index within pane
	viewMode       ViewMode                  // compact or expanded display
	listeners      map[int]SelectionListener // listener ID -> listener
	nextListenerID int
	searchState    SearchState // search state (embedded)
}

// NewBoardConfig creates a board config with default panes
func NewBoardConfig() *BoardConfig {
	bc := &BoardConfig{
		statusToPane:   make(map[task.Status]string),
		paneToStatus:   make(map[string]task.Status),
		viewMode:       ViewModeCompact,
		listeners:      make(map[int]SelectionListener),
		nextListenerID: 1, // Start at 1 to avoid conflict with zero-value sentinel
	}

	// default kanban panes
	defaultPanes := []*Pane{
		{ID: "col-todo", Name: "To Do", Status: string(task.StatusTodo), Position: 0},
		{ID: "col-progress", Name: "In Progress", Status: string(task.StatusInProgress), Position: 1},
		{ID: "col-review", Name: "Review", Status: string(task.StatusReview), Position: 2},
		{ID: "col-done", Name: "Done", Status: string(task.StatusDone), Position: 3},
	}

	for _, pane := range defaultPanes {
		bc.AddPane(pane)
	}

	if len(bc.panes) > 0 {
		bc.selectedPaneID = bc.panes[0].ID
	}

	return bc
}

// AddPane adds a pane and updates mappings
func (bc *BoardConfig) AddPane(pane *Pane) {
	bc.panes = append(bc.panes, pane)
	bc.statusToPane[task.Status(pane.Status)] = pane.ID
	bc.paneToStatus[pane.ID] = task.Status(pane.Status)
}

// GetPanes returns all panes in position order
func (bc *BoardConfig) GetPanes() []*Pane {
	return bc.panes
}

// GetPaneByID returns a pane by its ID
func (bc *BoardConfig) GetPaneByID(id string) *Pane {
	for _, pane := range bc.panes {
		if pane.ID == id {
			return pane
		}
	}
	return nil
}

// GetPaneByStatus returns the pane for a given status
func (bc *BoardConfig) GetPaneByStatus(status task.Status) *Pane {
	paneID, ok := bc.statusToPane[task.StatusPane(status)]
	if !ok {
		return nil
	}
	return bc.GetPaneByID(paneID)
}

// GetStatusForPane returns the status mapped to a pane
func (bc *BoardConfig) GetStatusForPane(paneID string) task.Status {
	return bc.paneToStatus[paneID]
}

// GetSelectedPaneID returns the currently selected pane ID
func (bc *BoardConfig) GetSelectedPaneID() string {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.selectedPaneID
}

// SetSelectedPane sets the selected pane by ID
func (bc *BoardConfig) SetSelectedPane(paneID string) {
	bc.mu.Lock()
	bc.selectedPaneID = paneID
	bc.mu.Unlock()
	bc.notifyListeners()
}

// GetSelectedRow returns the selected task index within current pane
func (bc *BoardConfig) GetSelectedRow() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.selectedRow
}

// SetSelectedRow sets the selected task index
func (bc *BoardConfig) SetSelectedRow(row int) {
	bc.mu.Lock()
	bc.selectedRow = row
	bc.mu.Unlock()
	bc.notifyListeners()
}

// SetSelection sets both pane and row atomically with a single notification.
// use when changing both values together to avoid double refresh.
func (bc *BoardConfig) SetSelection(paneID string, row int) {
	bc.mu.Lock()
	bc.selectedPaneID = paneID
	bc.selectedRow = row
	bc.mu.Unlock()
	bc.notifyListeners()
}

// SetSelectedRowSilent sets the selected task index without notifying listeners.
// use only for bounds clamping during refresh to avoid infinite loops.
func (bc *BoardConfig) SetSelectedRowSilent(row int) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.selectedRow = row
}

// AddSelectionListener registers a callback for selection changes.
// returns a listener ID that can be used to remove the listener.
func (bc *BoardConfig) AddSelectionListener(listener SelectionListener) int {
	id := bc.nextListenerID
	bc.nextListenerID++
	bc.listeners[id] = listener
	return id
}

// RemoveSelectionListener removes a previously registered listener by ID
func (bc *BoardConfig) RemoveSelectionListener(id int) {
	delete(bc.listeners, id)
}

// notifyListeners calls all registered listeners.
// Listeners are called outside the lock to prevent deadlocks.
func (bc *BoardConfig) notifyListeners() {
	bc.mu.RLock()
	listeners := make([]SelectionListener, 0, len(bc.listeners))
	for _, l := range bc.listeners {
		listeners = append(listeners, l)
	}
	bc.mu.RUnlock()

	// Call listeners OUTSIDE the lock
	for _, l := range listeners {
		l()
	}
}

// MoveSelectionLeft moves selection to the previous pane
func (bc *BoardConfig) MoveSelectionLeft() bool {
	idx := bc.getPaneIndex(bc.selectedPaneID)
	if idx > 0 {
		bc.SetSelection(bc.panes[idx-1].ID, 0)
		return true
	}
	return false
}

// MoveSelectionRight moves selection to the next pane
func (bc *BoardConfig) MoveSelectionRight() bool {
	idx := bc.getPaneIndex(bc.selectedPaneID)
	if idx < len(bc.panes)-1 {
		bc.SetSelection(bc.panes[idx+1].ID, 0)
		return true
	}
	return false
}

// getPaneIndex returns the index of a pane by ID
func (bc *BoardConfig) getPaneIndex(paneID string) int {
	for i, pane := range bc.panes {
		if pane.ID == paneID {
			return i
		}
	}
	return -1
}

// GetNextPaneID returns the pane to the right, or empty if at edge
func (bc *BoardConfig) GetNextPaneID(paneID string) string {
	idx := bc.getPaneIndex(paneID)
	if idx >= 0 && idx < len(bc.panes)-1 {
		return bc.panes[idx+1].ID
	}
	return ""
}

// GetPreviousPaneID returns the pane to the left, or empty if at edge
func (bc *BoardConfig) GetPreviousPaneID(paneID string) string {
	idx := bc.getPaneIndex(paneID)
	if idx > 0 {
		return bc.panes[idx-1].ID
	}
	return ""
}

// GetViewMode returns the current view mode
func (bc *BoardConfig) GetViewMode() ViewMode {
	return bc.viewMode
}

// ToggleViewMode switches between compact and expanded view modes
func (bc *BoardConfig) ToggleViewMode() {
	if bc.viewMode == ViewModeCompact {
		bc.viewMode = ViewModeExpanded
	} else {
		bc.viewMode = ViewModeCompact
	}

	// Save to config using unified approach (board is index -1, means create/update by name)
	if err := config.SavePluginViewMode("Board", -1, string(bc.viewMode)); err != nil {
		slog.Error("failed to save board view mode", "error", err)
	}

	bc.notifyListeners()
}

// SetViewMode sets the view mode from a string value
func (bc *BoardConfig) SetViewMode(mode string) {
	if mode == "expanded" {
		bc.viewMode = ViewModeExpanded
	} else {
		bc.viewMode = ViewModeCompact
	}
}

// SavePreSearchState saves current pane and row for later restoration
func (bc *BoardConfig) SavePreSearchState() {
	bc.searchState.SavePreSearchPaneState(bc.selectedPaneID, bc.selectedRow)
}

// SetSearchResults sets filtered search results and query
func (bc *BoardConfig) SetSearchResults(results []task.SearchResult, query string) {
	bc.searchState.SetSearchResults(results, query)
	bc.notifyListeners()
}

// ClearSearchResults clears search and restores pre-search selection
func (bc *BoardConfig) ClearSearchResults() {
	_, preSearchPane, preSearchRow := bc.searchState.ClearSearchResults()
	bc.selectedPaneID = preSearchPane
	bc.selectedRow = preSearchRow
	bc.notifyListeners()
}

// GetSearchResults returns current search results (nil if no search active)
func (bc *BoardConfig) GetSearchResults() []task.SearchResult {
	return bc.searchState.GetSearchResults()
}

// IsSearchActive returns true if search is currently active
func (bc *BoardConfig) IsSearchActive() bool {
	return bc.searchState.IsSearchActive()
}

// GetSearchQuery returns the current search query
func (bc *BoardConfig) GetSearchQuery() string {
	return bc.searchState.GetSearchQuery()
}
