package view

import (
	"testing"

	"github.com/rivo/tview"
)

// mockPrimitive is a simple primitive for testing
type mockPrimitive struct {
	*tview.Box
}

func newMockPrimitive() *mockPrimitive {
	return &mockPrimitive{
		Box: tview.NewBox(),
	}
}

// Helper to create a scrollable list with N items for testing
func createTestList(itemCount int, itemHeight int) *ScrollableList {
	list := NewScrollableList().SetItemHeight(itemHeight)
	for i := 0; i < itemCount; i++ {
		list.AddItem(newMockPrimitive())
	}
	return list
}

// Helper to simulate setting a fixed height for the list
func setListHeight(list *ScrollableList, height int) {
	// Set the rect to simulate screen dimensions
	list.SetRect(0, 0, 100, height)
}

// TestScrollingDown tests scrolling down through the list
func TestScrollingDown(t *testing.T) {
	// 10 items, each 5 units tall, viewport shows 5 items (height=25)
	list := createTestList(10, 5)
	setListHeight(list, 25) // Can show 5 items (25/5=5)

	// Start at item 0
	list.SetSelection(0)
	if list.scrollOffset != 0 {
		t.Errorf("Initial scrollOffset should be 0, got %d", list.scrollOffset)
	}

	// Move down to item 1 - should not scroll yet
	list.SetSelection(1)
	if list.scrollOffset != 0 {
		t.Errorf("At item 1, scrollOffset should be 0, got %d", list.scrollOffset)
	}

	// Move down to item 4 (last visible in initial view) - should not scroll
	list.SetSelection(4)
	if list.scrollOffset != 0 {
		t.Errorf("At item 4, scrollOffset should be 0, got %d", list.scrollOffset)
	}

	// Move down to item 5 - should scroll to show 1-5
	list.SetSelection(5)
	if list.scrollOffset != 1 {
		t.Errorf("At item 5, scrollOffset should be 1 (showing 1-5), got %d", list.scrollOffset)
	}

	// Move down to item 6 - should scroll to show 2-6
	list.SetSelection(6)
	if list.scrollOffset != 2 {
		t.Errorf("At item 6, scrollOffset should be 2 (showing 2-6), got %d", list.scrollOffset)
	}

	// Move down to item 9 (last item) - should scroll to show 5-9
	list.SetSelection(9)
	if list.scrollOffset != 5 {
		t.Errorf("At item 9, scrollOffset should be 5 (showing 5-9), got %d", list.scrollOffset)
	}
}

// TestScrollingUpFromBottom tests the critical case: scrolling up from bottom
func TestScrollingUpFromBottom(t *testing.T) {
	// 10 items, each 5 units tall, viewport shows 5 items (height=25)
	list := createTestList(10, 5)
	setListHeight(list, 25) // Can show 5 items (25/5=5)

	// First scroll down to the bottom (item 9)
	list.SetSelection(9)
	if list.scrollOffset != 5 {
		t.Errorf("At item 9, scrollOffset should be 5 (showing 5-9), got %d", list.scrollOffset)
	}

	// Now press Up: move to item 8 - should NOT scroll, still showing 5-9
	list.SetSelection(8)
	if list.scrollOffset != 5 {
		t.Errorf("At item 8 (moved up from 9), scrollOffset should still be 5 (showing 5-9), got %d", list.scrollOffset)
	}

	// Press Up: move to item 7 - should NOT scroll, still showing 5-9
	list.SetSelection(7)
	if list.scrollOffset != 5 {
		t.Errorf("At item 7 (moved up from 8), scrollOffset should still be 5 (showing 5-9), got %d", list.scrollOffset)
	}

	// Press Up: move to item 6 - should NOT scroll, still showing 5-9
	list.SetSelection(6)
	if list.scrollOffset != 5 {
		t.Errorf("At item 6 (moved up from 7), scrollOffset should still be 5 (showing 5-9), got %d", list.scrollOffset)
	}

	// Press Up: move to item 5 - should NOT scroll, still showing 5-9
	list.SetSelection(5)
	if list.scrollOffset != 5 {
		t.Errorf("At item 5 (moved up from 6), scrollOffset should still be 5 (showing 5-9), got %d", list.scrollOffset)
	}

	// Press Up: move to item 4 - NOW should scroll to show 4-8
	list.SetSelection(4)
	if list.scrollOffset != 4 {
		t.Errorf("At item 4 (moved up from 5), scrollOffset should be 4 (showing 4-8), got %d", list.scrollOffset)
	}
}

// TestScrollingUpToTop tests scrolling all the way to the top
func TestScrollingUpToTop(t *testing.T) {
	// 10 items, each 5 units tall, viewport shows 5 items (height=25)
	list := createTestList(10, 5)
	setListHeight(list, 25)

	// Start at item 5 (middle)
	list.SetSelection(5)
	if list.scrollOffset != 1 {
		t.Errorf("At item 5, scrollOffset should be 1, got %d", list.scrollOffset)
	}

	// Move up to item 4
	list.SetSelection(4)
	if list.scrollOffset != 1 {
		t.Errorf("At item 4, scrollOffset should still be 1 (showing 1-5), got %d", list.scrollOffset)
	}

	// Move up to item 3
	list.SetSelection(3)
	if list.scrollOffset != 1 {
		t.Errorf("At item 3, scrollOffset should still be 1 (showing 1-5), got %d", list.scrollOffset)
	}

	// Move up to item 2
	list.SetSelection(2)
	if list.scrollOffset != 1 {
		t.Errorf("At item 2, scrollOffset should still be 1 (showing 1-5), got %d", list.scrollOffset)
	}

	// Move up to item 1 - still showing 1-5
	list.SetSelection(1)
	if list.scrollOffset != 1 {
		t.Errorf("At item 1, scrollOffset should still be 1 (showing 1-5), got %d", list.scrollOffset)
	}

	// Move up to item 0 - should scroll to show 0-4
	list.SetSelection(0)
	if list.scrollOffset != 0 {
		t.Errorf("At item 0, scrollOffset should be 0 (showing 0-4), got %d", list.scrollOffset)
	}
}

// TestScrollingDownThenUpComplete tests a full down-then-up cycle
func TestScrollingDownThenUpComplete(t *testing.T) {
	// 10 items, each 5 units tall, viewport shows 5 items (height=25)
	list := createTestList(10, 5)
	setListHeight(list, 25)

	// Scroll all the way down
	for i := 0; i <= 9; i++ {
		list.SetSelection(i)
	}
	if list.scrollOffset != 5 {
		t.Errorf("After scrolling to bottom, scrollOffset should be 5, got %d", list.scrollOffset)
	}

	// Now scroll all the way up
	expectedOffsets := []int{5, 5, 5, 5, 5, 4, 3, 2, 1, 0}
	for i := 9; i >= 0; i-- {
		list.SetSelection(i)
		expected := expectedOffsets[9-i]
		if list.scrollOffset != expected {
			t.Errorf("At item %d (scrolling up), scrollOffset should be %d, got %d", i, expected, list.scrollOffset)
		}
	}
}

// TestEdgeCaseFewerItemsThanViewport tests when there are fewer items than viewport can hold
func TestEdgeCaseFewerItemsThanViewport(t *testing.T) {
	// 3 items, viewport shows 5 items
	list := createTestList(3, 5)
	setListHeight(list, 25)

	// Move through all items - should never scroll
	list.SetSelection(0)
	if list.scrollOffset != 0 {
		t.Errorf("At item 0 with 3 items, scrollOffset should be 0, got %d", list.scrollOffset)
	}

	list.SetSelection(1)
	if list.scrollOffset != 0 {
		t.Errorf("At item 1 with 3 items, scrollOffset should be 0, got %d", list.scrollOffset)
	}

	list.SetSelection(2)
	if list.scrollOffset != 0 {
		t.Errorf("At item 2 with 3 items, scrollOffset should be 0, got %d", list.scrollOffset)
	}
}

// TestEdgeCaseExactFit tests when items exactly fill the viewport
func TestEdgeCaseExactFit(t *testing.T) {
	// 5 items, viewport shows exactly 5 items
	list := createTestList(5, 5)
	setListHeight(list, 25)

	// Should never need to scroll
	for i := 0; i < 5; i++ {
		list.SetSelection(i)
		if list.scrollOffset != 0 {
			t.Errorf("At item %d with exact fit, scrollOffset should be 0, got %d", i, list.scrollOffset)
		}
	}
}

// TestEdgeCaseOneMoreThanViewport tests the boundary case of viewport+1 items
func TestEdgeCaseOneMoreThanViewport(t *testing.T) {
	// 6 items, viewport shows 5 items
	list := createTestList(6, 5)
	setListHeight(list, 25)

	// Items 0-4 should not scroll
	for i := 0; i <= 4; i++ {
		list.SetSelection(i)
		if list.scrollOffset != 0 {
			t.Errorf("At item %d, scrollOffset should be 0, got %d", i, list.scrollOffset)
		}
	}

	// Item 5 should scroll by 1
	list.SetSelection(5)
	if list.scrollOffset != 1 {
		t.Errorf("At item 5, scrollOffset should be 1 (showing 1-5), got %d", list.scrollOffset)
	}

	// Move back up to 4 - should not scroll back yet
	list.SetSelection(4)
	if list.scrollOffset != 1 {
		t.Errorf("At item 4 (moved up from 5), scrollOffset should still be 1, got %d", list.scrollOffset)
	}

	// Move back up to 3
	list.SetSelection(3)
	if list.scrollOffset != 1 {
		t.Errorf("At item 3 (moved up from 4), scrollOffset should still be 1, got %d", list.scrollOffset)
	}

	// Move back up to 2
	list.SetSelection(2)
	if list.scrollOffset != 1 {
		t.Errorf("At item 2 (moved up from 3), scrollOffset should still be 1, got %d", list.scrollOffset)
	}

	// Move back up to 1 - still showing 1-5
	list.SetSelection(1)
	if list.scrollOffset != 1 {
		t.Errorf("At item 1 (moved up from 2), scrollOffset should still be 1, got %d", list.scrollOffset)
	}

	// Move back up to 0 - NOW should scroll to 0
	list.SetSelection(0)
	if list.scrollOffset != 0 {
		t.Errorf("At item 0 (moved up from 1), scrollOffset should be 0, got %d", list.scrollOffset)
	}
}

// TestRefreshCycle tests the pattern used in BoardView: Clear() + AddItem() + SetSelection()
func TestRefreshCycle(t *testing.T) {
	// 10 items, viewport shows 5
	list := createTestList(10, 5)
	setListHeight(list, 25)

	// Scroll to bottom
	list.SetSelection(9)
	if list.scrollOffset != 5 {
		t.Errorf("At item 9, scrollOffset should be 5, got %d", list.scrollOffset)
	}

	// Now simulate a refresh: Clear() + re-add items + SetSelection()
	// This is what BoardView.refresh() does
	oldScrollOffset := list.scrollOffset

	list.Clear() // Should preserve scrollOffset
	if list.scrollOffset != oldScrollOffset {
		t.Errorf("After Clear(), scrollOffset changed from %d to %d", oldScrollOffset, list.scrollOffset)
	}

	// Re-add items
	for i := 0; i < 10; i++ {
		list.AddItem(newMockPrimitive())
	}

	// Set selection to item 8 (moved up from 9)
	list.SetSelection(8)
	if list.scrollOffset != 5 {
		t.Errorf("After refresh with selection at 8, scrollOffset should still be 5 (showing 5-9), got %d", list.scrollOffset)
	}
}

// TestLargeItemHeight tests with different item heights
func TestLargeItemHeight(t *testing.T) {
	// 10 items, each 10 units tall, viewport shows 3 items (height=30)
	list := createTestList(10, 10)
	setListHeight(list, 30) // Can show 3 items (30/10=3)

	// Start at 0
	list.SetSelection(0)
	if list.scrollOffset != 0 {
		t.Errorf("At item 0, scrollOffset should be 0, got %d", list.scrollOffset)
	}

	// Move to item 2 (last visible) - should not scroll
	list.SetSelection(2)
	if list.scrollOffset != 0 {
		t.Errorf("At item 2, scrollOffset should be 0 (showing 0-2), got %d", list.scrollOffset)
	}

	// Move to item 3 - should scroll to show 1-3
	list.SetSelection(3)
	if list.scrollOffset != 1 {
		t.Errorf("At item 3, scrollOffset should be 1 (showing 1-3), got %d", list.scrollOffset)
	}

	// Move to item 9 (last) - should scroll to show 7-9
	list.SetSelection(9)
	if list.scrollOffset != 7 {
		t.Errorf("At item 9, scrollOffset should be 7 (showing 7-9), got %d", list.scrollOffset)
	}

	// Move up to 8 - should NOT scroll yet
	list.SetSelection(8)
	if list.scrollOffset != 7 {
		t.Errorf("At item 8 (moved up from 9), scrollOffset should still be 7 (showing 7-9), got %d", list.scrollOffset)
	}

	// Move up to 7 - should NOT scroll yet (showing 7-9)
	list.SetSelection(7)
	if list.scrollOffset != 7 {
		t.Errorf("At item 7 (moved up from 8), scrollOffset should still be 7 (showing 7-9), got %d", list.scrollOffset)
	}

	// Move up to 6 - NOW should scroll to show 6-8
	list.SetSelection(6)
	if list.scrollOffset != 6 {
		t.Errorf("At item 6 (moved up from 7), scrollOffset should be 6 (showing 6-8), got %d", list.scrollOffset)
	}
}
