package model

import (
	"log/slog"
	"sync"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/task"
)

// BoardConfig defines board columns, status-to-column mappings, and selection state.
// It tracks which column and row is currently selected.

// SelectionListener is called when board selection changes
type SelectionListener func()

// BoardConfig holds column definitions and status mappings for the board view
type BoardConfig struct {
	mu             sync.RWMutex // protects selectedColID and selectedRow
	columns        []*Column
	statusToCol    map[task.Status]string    // status -> column ID
	colToStatus    map[string]task.Status    // column ID -> status
	selectedColID  string                    // currently selected column
	selectedRow    int                       // selected task index within column
	viewMode       ViewMode                  // compact or expanded display
	listeners      map[int]SelectionListener // listener ID -> listener
	nextListenerID int
	searchState    SearchState // search state (embedded)
}

// NewBoardConfig creates a board config with default columns
func NewBoardConfig() *BoardConfig {
	bc := &BoardConfig{
		statusToCol:    make(map[task.Status]string),
		colToStatus:    make(map[string]task.Status),
		viewMode:       ViewModeCompact,
		listeners:      make(map[int]SelectionListener),
		nextListenerID: 1, // Start at 1 to avoid conflict with zero-value sentinel
	}

	// default kanban columns
	defaultColumns := []*Column{
		{ID: "col-todo", Name: "To Do", Status: string(task.StatusTodo), Position: 0},
		{ID: "col-progress", Name: "In Progress", Status: string(task.StatusInProgress), Position: 1},
		{ID: "col-review", Name: "Review", Status: string(task.StatusReview), Position: 2},
		{ID: "col-done", Name: "Done", Status: string(task.StatusDone), Position: 3},
	}

	for _, col := range defaultColumns {
		bc.AddColumn(col)
	}

	if len(bc.columns) > 0 {
		bc.selectedColID = bc.columns[0].ID
	}

	return bc
}

// AddColumn adds a column and updates mappings
func (bc *BoardConfig) AddColumn(col *Column) {
	bc.columns = append(bc.columns, col)
	bc.statusToCol[task.Status(col.Status)] = col.ID
	bc.colToStatus[col.ID] = task.Status(col.Status)
}

// GetColumns returns all columns in position order
func (bc *BoardConfig) GetColumns() []*Column {
	return bc.columns
}

// GetColumnByID returns a column by its ID
func (bc *BoardConfig) GetColumnByID(id string) *Column {
	for _, col := range bc.columns {
		if col.ID == id {
			return col
		}
	}
	return nil
}

// GetColumnByStatus returns the column for a given status
func (bc *BoardConfig) GetColumnByStatus(status task.Status) *Column {
	colID, ok := bc.statusToCol[task.StatusColumn(status)]
	if !ok {
		return nil
	}
	return bc.GetColumnByID(colID)
}

// GetStatusForColumn returns the status mapped to a column
func (bc *BoardConfig) GetStatusForColumn(colID string) task.Status {
	return bc.colToStatus[colID]
}

// GetSelectedColumnID returns the currently selected column ID
func (bc *BoardConfig) GetSelectedColumnID() string {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.selectedColID
}

// SetSelectedColumn sets the selected column by ID
func (bc *BoardConfig) SetSelectedColumn(colID string) {
	bc.mu.Lock()
	bc.selectedColID = colID
	bc.mu.Unlock()
	bc.notifyListeners()
}

// GetSelectedRow returns the selected task index within current column
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

// SetSelection sets both column and row atomically with a single notification.
// use when changing both values together to avoid double refresh.
func (bc *BoardConfig) SetSelection(colID string, row int) {
	bc.mu.Lock()
	bc.selectedColID = colID
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

// MoveSelectionLeft moves selection to the previous column
func (bc *BoardConfig) MoveSelectionLeft() bool {
	idx := bc.getColumnIndex(bc.selectedColID)
	if idx > 0 {
		bc.SetSelection(bc.columns[idx-1].ID, 0)
		return true
	}
	return false
}

// MoveSelectionRight moves selection to the next column
func (bc *BoardConfig) MoveSelectionRight() bool {
	idx := bc.getColumnIndex(bc.selectedColID)
	if idx < len(bc.columns)-1 {
		bc.SetSelection(bc.columns[idx+1].ID, 0)
		return true
	}
	return false
}

// getColumnIndex returns the index of a column by ID
func (bc *BoardConfig) getColumnIndex(colID string) int {
	for i, col := range bc.columns {
		if col.ID == colID {
			return i
		}
	}
	return -1
}

// GetNextColumnID returns the column to the right, or empty if at edge
func (bc *BoardConfig) GetNextColumnID(colID string) string {
	idx := bc.getColumnIndex(colID)
	if idx >= 0 && idx < len(bc.columns)-1 {
		return bc.columns[idx+1].ID
	}
	return ""
}

// GetPreviousColumnID returns the column to the left, or empty if at edge
func (bc *BoardConfig) GetPreviousColumnID(colID string) string {
	idx := bc.getColumnIndex(colID)
	if idx > 0 {
		return bc.columns[idx-1].ID
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

// SavePreSearchState saves current column and row for later restoration
func (bc *BoardConfig) SavePreSearchState() {
	bc.searchState.SavePreSearchColumnState(bc.selectedColID, bc.selectedRow)
}

// SetSearchResults sets filtered search results and query
func (bc *BoardConfig) SetSearchResults(results []task.SearchResult, query string) {
	bc.searchState.SetSearchResults(results, query)
	bc.notifyListeners()
}

// ClearSearchResults clears search and restores pre-search selection
func (bc *BoardConfig) ClearSearchResults() {
	_, preSearchCol, preSearchRow := bc.searchState.ClearSearchResults()
	bc.selectedColID = preSearchCol
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
