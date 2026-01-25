package model

import (
	"testing"

	"github.com/boolean-maybe/tiki/task"
)

func TestSearchState_GridBasedFlow(t *testing.T) {
	ss := &SearchState{}

	// Initially no search active
	if ss.IsSearchActive() {
		t.Error("IsSearchActive() = true, want false initially")
	}

	// Save grid-based pre-search state
	ss.SavePreSearchState(5)

	// Set search results
	results := []task.SearchResult{
		{Task: &task.Task{ID: "TIKI-1", Title: "Test 1"}, Score: 0.9},
		{Task: &task.Task{ID: "TIKI-2", Title: "Test 2"}, Score: 0.7},
	}
	ss.SetSearchResults(results, "test query")

	// Verify search is active
	if !ss.IsSearchActive() {
		t.Error("IsSearchActive() = false, want true after SetSearchResults")
	}

	// Verify query
	if ss.GetSearchQuery() != "test query" {
		t.Errorf("GetSearchQuery() = %q, want %q", ss.GetSearchQuery(), "test query")
	}

	// Verify results
	gotResults := ss.GetSearchResults()
	if len(gotResults) != 2 {
		t.Errorf("len(GetSearchResults()) = %d, want 2", len(gotResults))
	}

	// Clear search and verify restoration
	preIndex, prePane, preRow := ss.ClearSearchResults()
	if preIndex != 5 {
		t.Errorf("ClearSearchResults() preIndex = %d, want 5", preIndex)
	}
	if prePane != "" {
		t.Errorf("ClearSearchResults() prePane = %q, want empty", prePane)
	}
	if preRow != 0 {
		t.Errorf("ClearSearchResults() preRow = %d, want 0", preRow)
	}

	// Verify search is no longer active
	if ss.IsSearchActive() {
		t.Error("IsSearchActive() = true, want false after ClearSearchResults")
	}

	// Verify query cleared
	if ss.GetSearchQuery() != "" {
		t.Errorf("GetSearchQuery() = %q, want empty after clear", ss.GetSearchQuery())
	}

	// Verify results cleared
	if ss.GetSearchResults() != nil {
		t.Error("GetSearchResults() != nil, want nil after clear")
	}
}

func TestSearchState_PaneBasedFlow(t *testing.T) {
	ss := &SearchState{}

	// Save pane-based pre-search state
	ss.SavePreSearchPaneState("in_progress", 3)

	// Set search results
	results := []task.SearchResult{
		{Task: &task.Task{ID: "TIKI-10", Title: "Match"}, Score: 1.0},
	}
	ss.SetSearchResults(results, "match")

	// Verify active
	if !ss.IsSearchActive() {
		t.Error("IsSearchActive() = false, want true")
	}

	// Clear and verify column state restored
	preIndex, prePane, preRow := ss.ClearSearchResults()
	if preIndex != 0 {
		t.Errorf("ClearSearchResults() preIndex = %d, want 0", preIndex)
	}
	if prePane != "in_progress" {
		t.Errorf("ClearSearchResults() prePane = %q, want %q", prePane, "in_progress")
	}
	if preRow != 3 {
		t.Errorf("ClearSearchResults() preRow = %d, want 3", preRow)
	}
}

func TestSearchState_MultipleSearchCycles(t *testing.T) {
	ss := &SearchState{}

	// First search cycle
	ss.SavePreSearchState(10)
	ss.SetSearchResults([]task.SearchResult{
		{Task: &task.Task{ID: "TIKI-1"}, Score: 0.8},
	}, "first")

	if ss.GetSearchQuery() != "first" {
		t.Errorf("GetSearchQuery() = %q, want %q", ss.GetSearchQuery(), "first")
	}

	// Clear first search
	preIndex, _, _ := ss.ClearSearchResults()
	if preIndex != 10 {
		t.Errorf("first ClearSearchResults() preIndex = %d, want 10", preIndex)
	}

	// Second search cycle with different state
	ss.SavePreSearchState(20)
	ss.SetSearchResults([]task.SearchResult{
		{Task: &task.Task{ID: "TIKI-2"}, Score: 0.9},
		{Task: &task.Task{ID: "TIKI-3"}, Score: 0.6},
	}, "second")

	if ss.GetSearchQuery() != "second" {
		t.Errorf("GetSearchQuery() = %q, want %q", ss.GetSearchQuery(), "second")
	}

	results := ss.GetSearchResults()
	if len(results) != 2 {
		t.Errorf("len(GetSearchResults()) = %d, want 2", len(results))
	}

	// Clear second search
	preIndex, _, _ = ss.ClearSearchResults()
	if preIndex != 20 {
		t.Errorf("second ClearSearchResults() preIndex = %d, want 20", preIndex)
	}
}

func TestSearchState_EmptySearchResults(t *testing.T) {
	ss := &SearchState{}

	// Search with empty results
	ss.SetSearchResults([]task.SearchResult{}, "no matches")

	// Should still be considered active search (empty results != nil results)
	if !ss.IsSearchActive() {
		t.Error("IsSearchActive() = false, want true for empty results")
	}

	if ss.GetSearchQuery() != "no matches" {
		t.Errorf("GetSearchQuery() = %q, want %q", ss.GetSearchQuery(), "no matches")
	}

	results := ss.GetSearchResults()
	if results == nil {
		t.Error("GetSearchResults() = nil, want empty slice")
	}
	if len(results) != 0 {
		t.Errorf("len(GetSearchResults()) = %d, want 0", len(results))
	}
}

func TestSearchState_NilSearchResults(t *testing.T) {
	ss := &SearchState{}

	// Explicitly set nil results
	ss.SetSearchResults(nil, "")

	// nil results are not considered an active search
	// This is by design - nil means no search, empty slice means search with no matches
	if ss.IsSearchActive() {
		t.Error("IsSearchActive() = true, want false for nil results")
	}

	// Clear should keep it inactive
	ss.ClearSearchResults()
	if ss.IsSearchActive() {
		t.Error("IsSearchActive() = true, want false after clear")
	}
}

func TestSearchState_StateOverwriting(t *testing.T) {
	ss := &SearchState{}

	// Save grid state
	ss.SavePreSearchState(5)

	// Overwrite with pane state
	ss.SavePreSearchPaneState("ready", 2)

	// Clear - should have both states available but prefer column
	preIndex, prePane, preRow := ss.ClearSearchResults()
	if preIndex != 5 {
		t.Errorf("preIndex = %d, want 5 (grid state preserved)", preIndex)
	}
	if prePane != "ready" {
		t.Errorf("prePane = %q, want %q", prePane, "ready")
	}
	if preRow != 2 {
		t.Errorf("preRow = %d, want 2", preRow)
	}
}

func TestSearchState_ConcurrentAccess(t *testing.T) {
	ss := &SearchState{}

	// This test verifies that concurrent reads/writes don't panic
	// It's a basic thread-safety smoke test
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := range 100 {
			ss.SavePreSearchState(i)
			ss.SetSearchResults([]task.SearchResult{
				{Task: &task.Task{ID: "TIKI-1"}, Score: 0.5},
			}, "concurrent")
			ss.ClearSearchResults()
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for range 100 {
			_ = ss.IsSearchActive()
			_ = ss.GetSearchQuery()
			_ = ss.GetSearchResults()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// If we get here without panic, test passes
}

func TestSearchState_QueryPreservation(t *testing.T) {
	ss := &SearchState{}

	// Set results with query
	ss.SetSearchResults([]task.SearchResult{
		{Task: &task.Task{ID: "TIKI-1"}, Score: 1.0},
	}, "important query")

	// Query should persist across result retrievals
	if ss.GetSearchQuery() != "important query" {
		t.Errorf("GetSearchQuery() = %q, want %q", ss.GetSearchQuery(), "important query")
	}

	// Getting results shouldn't clear query
	_ = ss.GetSearchResults()
	if ss.GetSearchQuery() != "important query" {
		t.Error("GetSearchQuery() changed after GetSearchResults()")
	}

	// Only ClearSearchResults should clear it
	ss.ClearSearchResults()
	if ss.GetSearchQuery() != "" {
		t.Errorf("GetSearchQuery() = %q, want empty after clear", ss.GetSearchQuery())
	}
}

func TestSearchState_ZeroValueState(t *testing.T) {
	// Zero value should be usable without initialization
	ss := &SearchState{}

	if ss.IsSearchActive() {
		t.Error("zero value IsSearchActive() = true, want false")
	}

	if ss.GetSearchQuery() != "" {
		t.Error("zero value GetSearchQuery() should be empty")
	}

	if ss.GetSearchResults() != nil {
		t.Error("zero value GetSearchResults() should be nil")
	}

	// Clear on zero value should not panic and return zero values
	preIndex, prePane, preRow := ss.ClearSearchResults()
	if preIndex != 0 || prePane != "" || preRow != 0 {
		t.Error("ClearSearchResults() on zero value should return zero values")
	}
}
