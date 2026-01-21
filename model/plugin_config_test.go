package model

import (
	"sync"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/task"
)

func TestNewPluginConfig(t *testing.T) {
	pc := NewPluginConfig("testplugin")

	if pc == nil {
		t.Fatal("NewPluginConfig() returned nil")
	}

	if pc.GetPluginName() != "testplugin" {
		t.Errorf("GetPluginName() = %q, want %q", pc.GetPluginName(), "testplugin")
	}

	if pc.GetSelectedIndex() != 0 {
		t.Errorf("initial GetSelectedIndex() = %d, want 0", pc.GetSelectedIndex())
	}

	if pc.GetColumnsForPane(0) != 4 {
		t.Errorf("GetColumnsForPane(0) = %d, want 4", pc.GetColumnsForPane(0))
	}

	if pc.GetViewMode() != ViewModeCompact {
		t.Errorf("initial GetViewMode() = %v, want ViewModeCompact", pc.GetViewMode())
	}

	if pc.IsSearchActive() {
		t.Error("initial IsSearchActive() = true, want false")
	}
}

func TestPluginConfig_SelectionIndexing(t *testing.T) {
	pc := NewPluginConfig("test")

	// Set selection
	pc.SetSelectedIndex(5)

	if pc.GetSelectedIndex() != 5 {
		t.Errorf("GetSelectedIndex() = %d, want 5", pc.GetSelectedIndex())
	}

	// Update selection
	pc.SetSelectedIndex(10)

	if pc.GetSelectedIndex() != 10 {
		t.Errorf("GetSelectedIndex() after update = %d, want 10", pc.GetSelectedIndex())
	}
}

func TestPluginConfig_MoveSelection_RightLeft(t *testing.T) {
	pc := NewPluginConfig("test")
	// Grid: 4 columns, 12 tasks (3 rows)
	// [ 0  1  2  3]
	// [ 4  5  6  7]
	// [ 8  9 10 11]

	// Start at index 5 (row 1, col 1)
	pc.SetSelectedIndex(5)

	// Move right -> 6
	moved := pc.MoveSelection("right", 12)
	if !moved {
		t.Error("MoveSelection(right) should return true")
	}
	if pc.GetSelectedIndex() != 6 {
		t.Errorf("after right: GetSelectedIndex() = %d, want 6", pc.GetSelectedIndex())
	}

	// Move left -> 5
	moved = pc.MoveSelection("left", 12)
	if !moved {
		t.Error("MoveSelection(left) should return true")
	}
	if pc.GetSelectedIndex() != 5 {
		t.Errorf("after left: GetSelectedIndex() = %d, want 5", pc.GetSelectedIndex())
	}
}

func TestPluginConfig_MoveSelection_UpDown(t *testing.T) {
	pc := NewPluginConfig("test")
	// Grid: 4 columns, 12 tasks (3 rows)
	// [ 0  1  2  3]
	// [ 4  5  6  7]
	// [ 8  9 10 11]

	// Start at index 5 (row 1, col 1)
	pc.SetSelectedIndex(5)

	// Move down -> 9 (same column, next row)
	moved := pc.MoveSelection("down", 12)
	if !moved {
		t.Error("MoveSelection(down) should return true")
	}
	if pc.GetSelectedIndex() != 9 {
		t.Errorf("after down: GetSelectedIndex() = %d, want 9", pc.GetSelectedIndex())
	}

	// Move up -> 5
	moved = pc.MoveSelection("up", 12)
	if !moved {
		t.Error("MoveSelection(up) should return true")
	}
	if pc.GetSelectedIndex() != 5 {
		t.Errorf("after up: GetSelectedIndex() = %d, want 5", pc.GetSelectedIndex())
	}
}

func TestPluginConfig_MoveSelection_EdgeCases(t *testing.T) {
	pc := NewPluginConfig("test")
	// Grid: 4 columns, 6 tasks
	// [ 0  1  2  3]
	// [ 4  5]

	tests := []struct {
		name      string
		start     int
		direction string
		taskCount int
		wantIndex int
		wantMoved bool
	}{
		{
			name:      "left at left edge",
			start:     4,
			direction: "left",
			taskCount: 6,
			wantIndex: 4,
			wantMoved: false,
		},
		{
			name:      "right at right edge",
			start:     3,
			direction: "right",
			taskCount: 6,
			wantIndex: 3,
			wantMoved: false,
		},
		{
			name:      "up at top",
			start:     1,
			direction: "up",
			taskCount: 6,
			wantIndex: 1,
			wantMoved: false,
		},
		{
			name:      "down at bottom",
			start:     5,
			direction: "down",
			taskCount: 6,
			wantIndex: 5,
			wantMoved: false,
		},
		{
			name:      "right at partial row end",
			start:     5,
			direction: "right",
			taskCount: 6,
			wantIndex: 5, // Can't move right from last item
			wantMoved: false,
		},
		{
			name:      "down from partial row",
			start:     1,
			direction: "down",
			taskCount: 6,
			wantIndex: 5, // 1 + 4 = 5
			wantMoved: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc.SetSelectedIndex(tt.start)
			moved := pc.MoveSelection(tt.direction, tt.taskCount)

			if moved != tt.wantMoved {
				t.Errorf("MoveSelection() moved = %v, want %v", moved, tt.wantMoved)
			}

			if pc.GetSelectedIndex() != tt.wantIndex {
				t.Errorf("GetSelectedIndex() = %d, want %d", pc.GetSelectedIndex(), tt.wantIndex)
			}
		})
	}
}

func TestPluginConfig_MoveSelection_EmptyGrid(t *testing.T) {
	pc := NewPluginConfig("test")

	// Moving with 0 tasks should not move
	moved := pc.MoveSelection("right", 0)
	if moved {
		t.Error("MoveSelection with 0 tasks should return false")
	}

	if pc.GetSelectedIndex() != 0 {
		t.Error("GetSelectedIndex() should remain 0")
	}
}

func TestPluginConfig_MoveSelection_SingleItem(t *testing.T) {
	pc := NewPluginConfig("test")
	pc.SetSelectedIndex(0)

	// Any direction with single item should not move
	directions := []string{"up", "down", "left", "right"}
	for _, dir := range directions {
		t.Run(dir, func(t *testing.T) {
			pc.SetSelectedIndex(0) // Reset
			moved := pc.MoveSelection(dir, 1)
			if moved {
				t.Errorf("MoveSelection(%q) with 1 task should return false", dir)
			}
			if pc.GetSelectedIndex() != 0 {
				t.Error("GetSelectedIndex() should remain 0")
			}
		})
	}
}

func TestPluginConfig_ClampSelection(t *testing.T) {
	pc := NewPluginConfig("test")

	// Set index beyond bounds
	pc.SetSelectedIndex(20)
	pc.ClampSelection(5)

	if pc.GetSelectedIndex() != 4 {
		t.Errorf("GetSelectedIndex() after clamp = %d, want 4 (max index for 5 tasks)", pc.GetSelectedIndex())
	}

	// Set negative (though SetSelectedIndex wouldn't normally do this)
	pc.SetSelectedIndex(-5)
	pc.ClampSelection(10)

	if pc.GetSelectedIndex() != 0 {
		t.Errorf("GetSelectedIndex() after clamp = %d, want 0", pc.GetSelectedIndex())
	}

	// Within bounds should not change
	pc.SetSelectedIndex(3)
	pc.ClampSelection(10)

	if pc.GetSelectedIndex() != 3 {
		t.Error("GetSelectedIndex() should not change when within bounds")
	}
}

func TestPluginConfig_ViewMode(t *testing.T) {
	pc := NewPluginConfig("test")

	// Initial mode should be compact
	if pc.GetViewMode() != ViewModeCompact {
		t.Errorf("initial GetViewMode() = %v, want ViewModeCompact", pc.GetViewMode())
	}

	// Set expanded
	pc.SetViewMode("expanded")
	if pc.GetViewMode() != ViewModeExpanded {
		t.Errorf("GetViewMode() after SetViewMode(expanded) = %v, want ViewModeExpanded", pc.GetViewMode())
	}

	// Set compact
	pc.SetViewMode("compact")
	if pc.GetViewMode() != ViewModeCompact {
		t.Errorf("GetViewMode() after SetViewMode(compact) = %v, want ViewModeCompact", pc.GetViewMode())
	}

	// Invalid mode should default to compact
	pc.SetViewMode("invalid")
	if pc.GetViewMode() != ViewModeCompact {
		t.Errorf("GetViewMode() after SetViewMode(invalid) = %v, want ViewModeCompact", pc.GetViewMode())
	}
}

func TestPluginConfig_ToggleViewMode(t *testing.T) {
	pc := NewPluginConfig("test")

	// Note: ToggleViewMode calls config.SavePluginViewMode which will fail in tests
	// but should not affect the toggle logic

	initial := pc.GetViewMode()

	// Toggle
	pc.ToggleViewMode()

	// Should be opposite
	after := pc.GetViewMode()
	if initial == ViewModeCompact && after != ViewModeExpanded {
		t.Error("ToggleViewMode() from compact should go to expanded")
	}
	if initial == ViewModeExpanded && after != ViewModeCompact {
		t.Error("ToggleViewMode() from expanded should go to compact")
	}

	// Toggle back
	pc.ToggleViewMode()

	// Should return to initial
	if pc.GetViewMode() != initial {
		t.Error("ToggleViewMode() twice should return to initial state")
	}
}

func TestPluginConfig_SearchState(t *testing.T) {
	pc := NewPluginConfig("test")

	// Initially no search
	if pc.IsSearchActive() {
		t.Error("IsSearchActive() = true, want false initially")
	}

	// Save pre-search state
	pc.SetSelectedIndex(5)
	pc.SavePreSearchState()

	// Set search results
	results := []task.SearchResult{
		{Task: &task.Task{ID: "TIKI-1", Title: "Match"}, Score: 1.0},
		{Task: &task.Task{ID: "TIKI-2", Title: "Match 2"}, Score: 0.8},
	}
	pc.SetSearchResults(results, "match")

	// Should be active
	if !pc.IsSearchActive() {
		t.Error("IsSearchActive() = false, want true after SetSearchResults")
	}

	// Verify query
	if pc.GetSearchQuery() != "match" {
		t.Errorf("GetSearchQuery() = %q, want %q", pc.GetSearchQuery(), "match")
	}

	// Verify results
	got := pc.GetSearchResults()
	if len(got) != 2 {
		t.Errorf("len(GetSearchResults()) = %d, want 2", len(got))
	}

	// Change selection during search
	pc.SetSelectedIndex(1)

	// Clear search - should restore pre-search selection
	pc.ClearSearchResults()

	if pc.IsSearchActive() {
		t.Error("IsSearchActive() = true, want false after clear")
	}

	if pc.GetSelectedIndex() != 5 {
		t.Errorf("GetSelectedIndex() after clear = %d, want 5 (pre-search)", pc.GetSelectedIndex())
	}
}

func TestPluginConfig_SelectionListener(t *testing.T) {
	pc := NewPluginConfig("test")

	called := false
	listener := func() {
		called = true
	}

	listenerID := pc.AddSelectionListener(listener)

	// SetSelectedIndex should notify
	pc.SetSelectedIndex(5)

	time.Sleep(10 * time.Millisecond)

	if !called {
		t.Error("listener not called after SetSelectedIndex")
	}

	// MoveSelection should notify if moved
	called = false
	pc.MoveSelection("right", 10)

	time.Sleep(10 * time.Millisecond)

	if !called {
		t.Error("listener not called after MoveSelection")
	}

	// Remove listener
	pc.RemoveSelectionListener(listenerID)

	called = false
	pc.SetSelectedIndex(10)

	time.Sleep(10 * time.Millisecond)

	if called {
		t.Error("listener called after RemoveSelectionListener")
	}
}

func TestPluginConfig_MultipleListeners(t *testing.T) {
	pc := NewPluginConfig("test")

	var mu sync.Mutex
	callCounts := make(map[int]int)

	listener1 := func() {
		mu.Lock()
		callCounts[1]++
		mu.Unlock()
	}
	listener2 := func() {
		mu.Lock()
		callCounts[2]++
		mu.Unlock()
	}

	id1 := pc.AddSelectionListener(listener1)
	id2 := pc.AddSelectionListener(listener2)

	// Both should be notified
	pc.SetSelectedIndex(5)

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCounts[1] != 1 || callCounts[2] != 1 {
		t.Errorf("callCounts = %v, want both 1", callCounts)
	}
	mu.Unlock()

	// Remove one
	pc.RemoveSelectionListener(id1)

	pc.SetSelectedIndex(10)

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCounts[1] != 1 || callCounts[2] != 2 {
		t.Errorf("callCounts after remove = %v, want {1:1, 2:2}", callCounts)
	}
	mu.Unlock()

	// Remove second
	pc.RemoveSelectionListener(id2)

	pc.SetSelectedIndex(15)

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCounts[1] != 1 || callCounts[2] != 2 {
		t.Error("callCounts changed after both listeners removed")
	}
	mu.Unlock()
}

func TestPluginConfig_NotifyOnlyWhenMoved(t *testing.T) {
	pc := NewPluginConfig("test")

	callCount := 0
	pc.AddSelectionListener(func() {
		callCount++
	})

	// Move that doesn't actually move (at edge)
	pc.SetSelectedIndex(0)
	callCount = 0 // Reset after SetSelectedIndex

	time.Sleep(10 * time.Millisecond)

	pc.MoveSelection("left", 10) // Can't move left from 0

	time.Sleep(10 * time.Millisecond)

	if callCount > 0 {
		t.Error("listener called when MoveSelection didn't move")
	}

	// Move that does move
	pc.MoveSelection("right", 10)

	time.Sleep(10 * time.Millisecond)

	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 after actual move", callCount)
	}
}

func TestPluginConfig_ConcurrentAccess(t *testing.T) {
	pc := NewPluginConfig("test")

	done := make(chan bool)

	// Selection writer
	go func() {
		for i := range 50 {
			pc.SetSelectedIndex(i % 20)
			pc.MoveSelection("right", 20)
		}
		done <- true
	}()

	// View mode writer
	go func() {
		for range 50 {
			pc.ToggleViewMode()
		}
		done <- true
	}()

	// Search writer
	go func() {
		for i := range 25 {
			pc.SavePreSearchState()
			pc.SetSearchResults([]task.SearchResult{{Task: &task.Task{ID: "T-1"}, Score: 1.0}}, "query")
			if i%2 == 0 {
				pc.ClearSearchResults()
			}
		}
		done <- true
	}()

	// Reader
	go func() {
		for range 100 {
			_ = pc.GetSelectedIndex()
			_ = pc.GetViewMode()
			_ = pc.IsSearchActive()
			_ = pc.GetSearchResults()
		}
		done <- true
	}()

	// Wait for all
	for range 4 {
		<-done
	}

	// If we get here without panic, test passes
}

func TestPluginConfig_SetConfigIndex(t *testing.T) {
	pc := NewPluginConfig("test")

	// SetConfigIndex doesn't have a getter, but we're testing it doesn't panic
	pc.SetConfigIndex(5)
	pc.SetConfigIndex(-1)
	pc.SetConfigIndex(0)

	// Verify it doesn't affect other operations
	pc.SetSelectedIndex(3)
	if pc.GetSelectedIndex() != 3 {
		t.Error("SetConfigIndex affected GetSelectedIndex")
	}
}

func TestPluginConfig_GridNavigation_PartialLastRow(t *testing.T) {
	pc := NewPluginConfig("test")

	// Grid with 10 tasks:
	// [ 0  1  2  3]
	// [ 4  5  6  7]
	// [ 8  9]

	// From index 1, move down twice should go to 5, then 9
	pc.SetSelectedIndex(1)
	pc.MoveSelection("down", 10)
	if pc.GetSelectedIndex() != 5 {
		t.Errorf("after first down: GetSelectedIndex() = %d, want 5", pc.GetSelectedIndex())
	}

	pc.MoveSelection("down", 10)
	if pc.GetSelectedIndex() != 9 {
		t.Errorf("after second down: GetSelectedIndex() = %d, want 9", pc.GetSelectedIndex())
	}

	// Can't move down anymore
	moved := pc.MoveSelection("down", 10)
	if moved {
		t.Error("should not be able to move down from last row")
	}
}

func TestPluginConfig_GridNavigation_AllCorners(t *testing.T) {
	pc := NewPluginConfig("test")

	// Grid: 4x3 = 12 tasks
	// [ 0  1  2  3]
	// [ 4  5  6  7]
	// [ 8  9 10 11]

	corners := []struct {
		name       string
		index      int
		direction  string
		shouldMove bool
	}{
		{"top-left up", 0, "up", false},
		{"top-left left", 0, "left", false},
		{"top-right up", 3, "up", false},
		{"top-right right", 3, "right", false},
		{"bottom-left down", 8, "down", false},
		{"bottom-left left", 8, "left", false},
		{"bottom-right down", 11, "down", false},
		{"bottom-right right", 11, "right", false},
	}

	for _, tc := range corners {
		t.Run(tc.name, func(t *testing.T) {
			pc.SetSelectedIndex(tc.index)
			moved := pc.MoveSelection(tc.direction, 12)
			if moved != tc.shouldMove {
				t.Errorf("MoveSelection(%q) from corner moved = %v, want %v",
					tc.direction, moved, tc.shouldMove)
			}
			if pc.GetSelectedIndex() != tc.index {
				t.Error("selection changed when it shouldn't at corner")
			}
		})
	}
}
