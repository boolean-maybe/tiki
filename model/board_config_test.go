package model

import (
	"testing"

	"github.com/boolean-maybe/tiki/task"
)

func TestBoardConfig_Initialization(t *testing.T) {
	config := NewBoardConfig()

	// Verify default panes exist
	panes := config.GetPanes()
	if len(panes) != 4 {
		t.Fatalf("pane count = %d, want 4", len(panes))
	}

	// Verify pane order
	expectedPanes := []struct {
		id     string
		name   string
		status string
		pos    int
	}{
		{"col-todo", "To Do", string(task.StatusTodo), 0},
		{"col-progress", "In Progress", string(task.StatusInProgress), 1},
		{"col-review", "Review", string(task.StatusReview), 2},
		{"col-done", "Done", string(task.StatusDone), 3},
	}

	for i, expected := range expectedPanes {
		pane := panes[i]
		if pane.ID != expected.id {
			t.Errorf("panes[%d].ID = %q, want %q", i, pane.ID, expected.id)
		}
		if pane.Name != expected.name {
			t.Errorf("panes[%d].Name = %q, want %q", i, pane.Name, expected.name)
		}
		if pane.Status != expected.status {
			t.Errorf("panes[%d].Status = %q, want %q", i, pane.Status, expected.status)
		}
		if pane.Position != expected.pos {
			t.Errorf("panes[%d].Position = %d, want %d", i, pane.Position, expected.pos)
		}
	}

	// Verify first pane is selected by default
	if config.GetSelectedPaneID() != "col-todo" {
		t.Errorf("default selected pane = %q, want %q", config.GetSelectedPaneID(), "col-todo")
	}
}

func TestBoardConfig_PaneLookup(t *testing.T) {
	config := NewBoardConfig()

	// Test GetPaneByID
	pane := config.GetPaneByID("col-progress")
	if pane == nil {
		t.Fatal("GetPaneByID(col-progress) returned nil")
	}
	if pane.Name != "In Progress" {
		t.Errorf("pane name = %q, want %q", pane.Name, "In Progress")
	}

	// Test non-existent ID
	pane = config.GetPaneByID("non-existent")
	if pane != nil {
		t.Error("GetPaneByID(non-existent) should return nil")
	}

	// Test GetPaneByStatus
	pane = config.GetPaneByStatus(task.StatusReview)
	if pane == nil {
		t.Fatal("GetPaneByStatus(review) returned nil")
	}
	if pane.ID != "col-review" {
		t.Errorf("pane ID = %q, want %q", pane.ID, "col-review")
	}

	pane = config.GetPaneByStatus(task.StatusWaiting)
	if pane == nil {
		t.Fatal("GetPaneByStatus(waiting) returned nil")
	}
	if pane.ID != "col-review" {
		t.Errorf("pane ID = %q, want %q", pane.ID, "col-review")
	}

	// Test non-mapped status (backlog not in default panes)
	pane = config.GetPaneByStatus(task.StatusBacklog)
	if pane != nil {
		t.Error("GetPaneByStatus(backlog) should return nil for unmapped status")
	}
}

func TestBoardConfig_StatusMapping(t *testing.T) {
	config := NewBoardConfig()

	tests := []struct {
		colID    string
		expected task.Status
	}{
		{"col-todo", task.StatusTodo},
		{"col-progress", task.StatusInProgress},
		{"col-review", task.StatusReview},
		{"col-done", task.StatusDone},
	}

	for _, tt := range tests {
		t.Run(tt.colID, func(t *testing.T) {
			status := config.GetStatusForPane(tt.colID)
			if status != tt.expected {
				t.Errorf("GetStatusForPane(%q) = %q, want %q", tt.colID, status, tt.expected)
			}
		})
	}

	// Test unmapped pane
	status := config.GetStatusForPane("non-existent")
	if status != "" {
		t.Errorf("GetStatusForPane(non-existent) = %q, want empty string", status)
	}
}

func TestBoardConfig_MoveSelectionLeft(t *testing.T) {
	config := NewBoardConfig()

	// Start at second pane
	config.SetSelectedPane("col-progress")
	config.SetSelectedRow(5)

	// Move left should succeed and reset row to 0
	moved := config.MoveSelectionLeft()
	if !moved {
		t.Error("MoveSelectionLeft() returned false, want true")
	}
	if config.GetSelectedPaneID() != "col-todo" {
		t.Errorf("selected pane = %q, want %q", config.GetSelectedPaneID(), "col-todo")
	}
	if config.GetSelectedRow() != 0 {
		t.Errorf("selected row = %d, want 0", config.GetSelectedRow())
	}

	// Already at leftmost - should return false
	moved = config.MoveSelectionLeft()
	if moved {
		t.Error("MoveSelectionLeft() at leftmost returned true, want false")
	}
	if config.GetSelectedPaneID() != "col-todo" {
		t.Error("pane should not change when blocked")
	}
}

func TestBoardConfig_MoveSelectionRight(t *testing.T) {
	config := NewBoardConfig()

	// Start at first pane (default)
	config.SetSelectedRow(3)

	// Move right should succeed and reset row to 0
	moved := config.MoveSelectionRight()
	if !moved {
		t.Error("MoveSelectionRight() returned false, want true")
	}
	if config.GetSelectedPaneID() != "col-progress" {
		t.Errorf("selected pane = %q, want %q", config.GetSelectedPaneID(), "col-progress")
	}
	if config.GetSelectedRow() != 0 {
		t.Errorf("selected row = %d, want 0", config.GetSelectedRow())
	}

	// Move to rightmost
	config.MoveSelectionRight() // to review
	config.MoveSelectionRight() // to done

	// Already at rightmost - should return false
	moved = config.MoveSelectionRight()
	if moved {
		t.Error("MoveSelectionRight() at rightmost returned true, want false")
	}
	if config.GetSelectedPaneID() != "col-done" {
		t.Error("pane should not change when blocked")
	}
}

func TestBoardConfig_GetNextPaneID(t *testing.T) {
	config := NewBoardConfig()

	tests := []struct {
		name     string
		colID    string
		expected string
	}{
		{"first to second", "col-todo", "col-progress"},
		{"second to third", "col-progress", "col-review"},
		{"third to fourth", "col-review", "col-done"},
		{"last returns empty", "col-done", ""},
		{"non-existent returns empty", "non-existent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetNextPaneID(tt.colID)
			if result != tt.expected {
				t.Errorf("GetNextPaneID(%q) = %q, want %q", tt.colID, result, tt.expected)
			}
		})
	}
}

func TestBoardConfig_GetPreviousPaneID(t *testing.T) {
	config := NewBoardConfig()

	tests := []struct {
		name     string
		colID    string
		expected string
	}{
		{"second to first", "col-progress", "col-todo"},
		{"third to second", "col-review", "col-progress"},
		{"fourth to third", "col-done", "col-review"},
		{"first returns empty", "col-todo", ""},
		{"non-existent returns empty", "non-existent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetPreviousPaneID(tt.colID)
			if result != tt.expected {
				t.Errorf("GetPreviousPaneID(%q) = %q, want %q", tt.colID, result, tt.expected)
			}
		})
	}
}

func TestBoardConfig_SetSelectionAtomicity(t *testing.T) {
	config := NewBoardConfig()

	notificationCount := 0
	listener := func() {
		notificationCount++
	}
	config.AddSelectionListener(listener)

	// SetSelection should trigger single notification
	notificationCount = 0
	config.SetSelection("col-review", 7)
	if notificationCount != 1 {
		t.Errorf("SetSelection triggered %d notifications, want 1", notificationCount)
	}
	if config.GetSelectedPaneID() != "col-review" {
		t.Errorf("pane = %q, want col-review", config.GetSelectedPaneID())
	}
	if config.GetSelectedRow() != 7 {
		t.Errorf("row = %d, want 7", config.GetSelectedRow())
	}

	// Separate calls should trigger two notifications
	notificationCount = 0
	config.SetSelectedPane("col-done")
	config.SetSelectedRow(3)
	if notificationCount != 2 {
		t.Errorf("separate calls triggered %d notifications, want 2", notificationCount)
	}
}

func TestBoardConfig_SetSelectedRowSilent(t *testing.T) {
	config := NewBoardConfig()

	notified := false
	listener := func() {
		notified = true
	}
	config.AddSelectionListener(listener)

	// SetSelectedRowSilent should NOT trigger notification
	notified = false
	config.SetSelectedRowSilent(5)
	if notified {
		t.Error("SetSelectedRowSilent() triggered listener, want no notification")
	}
	if config.GetSelectedRow() != 5 {
		t.Errorf("row = %d, want 5", config.GetSelectedRow())
	}

	// Regular SetSelectedRow SHOULD trigger notification
	notified = false
	config.SetSelectedRow(10)
	if !notified {
		t.Error("SetSelectedRow() did not trigger listener")
	}
}

func TestBoardConfig_ListenerNotification(t *testing.T) {
	config := NewBoardConfig()

	notified := false
	listener := func() {
		notified = true
	}
	listenerID := config.AddSelectionListener(listener)

	// Test SetSelectedPane
	notified = false
	config.SetSelectedPane("col-progress")
	if !notified {
		t.Error("SetSelectedPane() did not trigger listener")
	}

	// Test SetSelectedRow
	notified = false
	config.SetSelectedRow(3)
	if !notified {
		t.Error("SetSelectedRow() did not trigger listener")
	}

	// Test MoveSelectionLeft
	notified = false
	config.MoveSelectionLeft()
	if !notified {
		t.Error("MoveSelectionLeft() did not trigger listener on successful move")
	}

	// Test MoveSelectionRight
	notified = false
	config.MoveSelectionRight()
	if !notified {
		t.Error("MoveSelectionRight() did not trigger listener on successful move")
	}

	// Remove listener
	config.RemoveSelectionListener(listenerID)

	// Should not notify after removal
	notified = false
	config.SetSelectedRow(5)
	if notified {
		t.Error("listener was notified after removal")
	}
}

func TestBoardConfig_MultipleListeners(t *testing.T) {
	config := NewBoardConfig()

	count1 := 0
	count2 := 0

	listener1 := func() { count1++ }
	listener2 := func() { count2++ }

	config.AddSelectionListener(listener1)
	id2 := config.AddSelectionListener(listener2)

	// Both should be notified
	config.SetSelectedPane("col-review")
	if count1 != 1 {
		t.Errorf("listener1 count = %d, want 1", count1)
	}
	if count2 != 1 {
		t.Errorf("listener2 count = %d, want 1", count2)
	}

	// Remove second listener
	config.RemoveSelectionListener(id2)

	// Only first should be notified
	config.SetSelectedPane("col-done")
	if count1 != 2 {
		t.Errorf("listener1 count = %d, want 2", count1)
	}
	if count2 != 1 {
		t.Errorf("listener2 count = %d, want 1 (should not change)", count2)
	}
}
