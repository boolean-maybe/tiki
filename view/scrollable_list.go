package view

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ScrollableList is a container that displays a list of primitives and handles vertical scrolling.
// It ensures that the selected item is always visible.
type ScrollableList struct {
	*tview.Box

	items          []tview.Primitive
	itemHeight     int
	scrollOffset   int
	selectionIndex int
}

// NewScrollableList creates a new scrollable list container
func NewScrollableList() *ScrollableList {
	return &ScrollableList{
		Box:            tview.NewBox(),
		items:          make([]tview.Primitive, 0),
		itemHeight:     1, // default, should be set by caller
		scrollOffset:   0,
		selectionIndex: -1,
	}
}

// SetItemHeight sets the height of each item in the list
func (s *ScrollableList) SetItemHeight(height int) *ScrollableList {
	s.itemHeight = height
	return s
}

// AddItem adds a primitive to the list
func (s *ScrollableList) AddItem(item tview.Primitive) *ScrollableList {
	s.items = append(s.items, item)
	return s
}

// Clear removes all items from the list
func (s *ScrollableList) Clear() *ScrollableList {
	s.items = make([]tview.Primitive, 0)
	// Keep scrollOffset to preserve position during refresh
	s.selectionIndex = -1
	return s
}

// SetSelection sets the index of the selected item and scrolls to keep it visible
func (s *ScrollableList) SetSelection(index int) {
	s.selectionIndex = index
	s.ensureSelectionVisible()
}

// GetScrollOffset returns the current scroll offset (first visible item index)
func (s *ScrollableList) GetScrollOffset() int {
	return s.scrollOffset
}

// ensureSelectionVisible adjusts scrollOffset to keep selectionIndex in view
func (s *ScrollableList) ensureSelectionVisible() {
	// If no items, preserve scrollOffset (will be adjusted after items are added)
	if len(s.items) == 0 {
		return
	}

	// Ensure scrollOffset is within valid bounds
	if s.scrollOffset >= len(s.items) {
		s.scrollOffset = len(s.items) - 1
	}
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}

	if s.selectionIndex < 0 {
		return
	}

	// Calculate view dimensions
	_, _, _, height := s.GetInnerRect()
	if height <= 0 {
		return
	}

	maxVisible := height / s.itemHeight
	if maxVisible <= 0 {
		return
	}

	// Calculate the last visible index
	lastVisibleIndex := s.scrollOffset + maxVisible - 1
	if lastVisibleIndex >= len(s.items) {
		lastVisibleIndex = len(s.items) - 1
	}

	// Adjust scroll offset if selection is out of view
	// When scrolling up: only adjust when selection goes ABOVE the first visible item
	if s.selectionIndex < s.scrollOffset {
		s.scrollOffset = s.selectionIndex
	} else if s.selectionIndex > lastVisibleIndex {
		// When scrolling down: adjust to show the selected item at the bottom
		s.scrollOffset = s.selectionIndex - maxVisible + 1
	}

	// Ensure valid bounds for scrollOffset
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
	maxScrollOffset := len(s.items) - maxVisible
	if maxScrollOffset < 0 {
		maxScrollOffset = 0
	}
	if s.scrollOffset > maxScrollOffset {
		s.scrollOffset = maxScrollOffset
	}
}

// Draw draws this primitive onto the screen
func (s *ScrollableList) Draw(screen tcell.Screen) {
	s.DrawForSubclass(screen, s)

	x, y, width, height := s.GetInnerRect()
	if height <= 0 || width <= 0 {
		return
	}

	// Re-run scroll calculation in case height changed (resize)
	s.ensureSelectionVisible()

	maxVisible := height / s.itemHeight

	// Loop through visible items
	for i := 0; i < maxVisible; i++ {
		itemIndex := s.scrollOffset + i
		if itemIndex >= len(s.items) {
			break
		}

		item := s.items[itemIndex]

		// set position and size for the item
		itemY := y + (i * s.itemHeight)
		item.SetRect(x, itemY, width, s.itemHeight)

		// draw the item
		item.Draw(screen)
	}
}

// Focus is called when this primitive receives focus
func (s *ScrollableList) Focus(delegate func(p tview.Primitive)) {
	// We don't necessarily need to pass focus to children if they are just visual boxes.
	// But if children need focus (e.g. if they had buttons), we would need to manage that.
	// For now, the Board/Backlog handle input at the controller level, and these are just display.
	// So we might not strictly need to delegate focus, but it's good practice if we want children to draw focused styles.

	// In this specific use case (TaskBox), the "selected" state is passed in during creation/refresh
	// via border color changes. The TaskBox itself doesn't handle input or focus events in the tview sense.
	// So we can just let the Box handle focus for now.
	s.Box.Focus(delegate)
}
