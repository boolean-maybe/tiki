package model

import (
	"sync"

	"github.com/boolean-maybe/tiki/task"
)

// SearchState holds reusable search state that can be embedded in any view config
type SearchState struct {
	mu             sync.RWMutex
	searchResults  []task.SearchResult // nil = no active search
	preSearchIndex int                 // for grid views (backlog, plugin)
	preSearchCol   string              // for board view (column ID)
	preSearchRow   int                 // for board view (row within column)
	searchQuery    string              // current search term (for UI restoration)
}

// SavePreSearchState saves the current selection index for grid-based views
func (ss *SearchState) SavePreSearchState(index int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.preSearchIndex = index
}

// SavePreSearchColumnState saves the current column and row for board view
func (ss *SearchState) SavePreSearchColumnState(colID string, row int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.preSearchCol = colID
	ss.preSearchRow = row
}

// SetSearchResults sets filtered search results and query
func (ss *SearchState) SetSearchResults(results []task.SearchResult, query string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.searchResults = results
	ss.searchQuery = query
}

// ClearSearchResults clears search and returns the pre-search state
// Returns: (preSearchIndex, preSearchCol, preSearchRow)
func (ss *SearchState) ClearSearchResults() (int, string, int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.searchResults = nil
	ss.searchQuery = ""

	return ss.preSearchIndex, ss.preSearchCol, ss.preSearchRow
}

// IsSearchActive returns true if search is currently active
func (ss *SearchState) IsSearchActive() bool {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.searchResults != nil
}

// GetSearchQuery returns the current search query
func (ss *SearchState) GetSearchQuery() string {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.searchQuery
}

// GetSearchResults returns current search results (nil if no search active)
func (ss *SearchState) GetSearchResults() []task.SearchResult {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.searchResults
}
