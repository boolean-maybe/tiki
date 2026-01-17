package model

import (
	"testing"

	"github.com/boolean-maybe/tiki/task"
)

func TestBoardConfig_Initialization(t *testing.T) {
	config := NewBoardConfig()

	// Verify default columns exist
	columns := config.GetColumns()
	if len(columns) != 4 {
		t.Fatalf("column count = %d, want 4", len(columns))
	}

	// Verify column order
	expectedColumns := []struct {
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

	for i, expected := range expectedColumns {
		col := columns[i]
		if col.ID != expected.id {
			t.Errorf("columns[%d].ID = %q, want %q", i, col.ID, expected.id)
		}
		if col.Name != expected.name {
			t.Errorf("columns[%d].Name = %q, want %q", i, col.Name, expected.name)
		}
		if col.Status != expected.status {
			t.Errorf("columns[%d].Status = %q, want %q", i, col.Status, expected.status)
		}
		if col.Position != expected.pos {
			t.Errorf("columns[%d].Position = %d, want %d", i, col.Position, expected.pos)
		}
	}

	// Verify first column is selected by default
	if config.GetSelectedColumnID() != "col-todo" {
		t.Errorf("default selected column = %q, want %q", config.GetSelectedColumnID(), "col-todo")
	}
}

func TestBoardConfig_ColumnLookup(t *testing.T) {
	config := NewBoardConfig()

	// Test GetColumnByID
	col := config.GetColumnByID("col-progress")
	if col == nil {
		t.Fatal("GetColumnByID(col-progress) returned nil")
	}
	if col.Name != "In Progress" {
		t.Errorf("column name = %q, want %q", col.Name, "In Progress")
	}

	// Test non-existent ID
	col = config.GetColumnByID("non-existent")
	if col != nil {
		t.Error("GetColumnByID(non-existent) should return nil")
	}

	// Test GetColumnByStatus
	col = config.GetColumnByStatus(task.StatusReview)
	if col == nil {
		t.Fatal("GetColumnByStatus(review) returned nil")
	}
	if col.ID != "col-review" {
		t.Errorf("column ID = %q, want %q", col.ID, "col-review")
	}

	col = config.GetColumnByStatus(task.StatusWaiting)
	if col == nil {
		t.Fatal("GetColumnByStatus(waiting) returned nil")
	}
	if col.ID != "col-review" {
		t.Errorf("column ID = %q, want %q", col.ID, "col-review")
	}

	// Test non-mapped status (backlog not in default columns)
	col = config.GetColumnByStatus(task.StatusBacklog)
	if col != nil {
		t.Error("GetColumnByStatus(backlog) should return nil for unmapped status")
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
			status := config.GetStatusForColumn(tt.colID)
			if status != tt.expected {
				t.Errorf("GetStatusForColumn(%q) = %q, want %q", tt.colID, status, tt.expected)
			}
		})
	}

	// Test unmapped column
	status := config.GetStatusForColumn("non-existent")
	if status != "" {
		t.Errorf("GetStatusForColumn(non-existent) = %q, want empty string", status)
	}
}

func TestBoardConfig_MoveSelectionLeft(t *testing.T) {
	config := NewBoardConfig()

	// Start at second column
	config.SetSelectedColumn("col-progress")
	config.SetSelectedRow(5)

	// Move left should succeed and reset row to 0
	moved := config.MoveSelectionLeft()
	if !moved {
		t.Error("MoveSelectionLeft() returned false, want true")
	}
	if config.GetSelectedColumnID() != "col-todo" {
		t.Errorf("selected column = %q, want %q", config.GetSelectedColumnID(), "col-todo")
	}
	if config.GetSelectedRow() != 0 {
		t.Errorf("selected row = %d, want 0", config.GetSelectedRow())
	}

	// Already at leftmost - should return false
	moved = config.MoveSelectionLeft()
	if moved {
		t.Error("MoveSelectionLeft() at leftmost returned true, want false")
	}
	if config.GetSelectedColumnID() != "col-todo" {
		t.Error("column should not change when blocked")
	}
}

func TestBoardConfig_MoveSelectionRight(t *testing.T) {
	config := NewBoardConfig()

	// Start at first column (default)
	config.SetSelectedRow(3)

	// Move right should succeed and reset row to 0
	moved := config.MoveSelectionRight()
	if !moved {
		t.Error("MoveSelectionRight() returned false, want true")
	}
	if config.GetSelectedColumnID() != "col-progress" {
		t.Errorf("selected column = %q, want %q", config.GetSelectedColumnID(), "col-progress")
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
	if config.GetSelectedColumnID() != "col-done" {
		t.Error("column should not change when blocked")
	}
}

func TestBoardConfig_GetNextColumnID(t *testing.T) {
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
			result := config.GetNextColumnID(tt.colID)
			if result != tt.expected {
				t.Errorf("GetNextColumnID(%q) = %q, want %q", tt.colID, result, tt.expected)
			}
		})
	}
}

func TestBoardConfig_GetPreviousColumnID(t *testing.T) {
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
			result := config.GetPreviousColumnID(tt.colID)
			if result != tt.expected {
				t.Errorf("GetPreviousColumnID(%q) = %q, want %q", tt.colID, result, tt.expected)
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
	if config.GetSelectedColumnID() != "col-review" {
		t.Errorf("column = %q, want col-review", config.GetSelectedColumnID())
	}
	if config.GetSelectedRow() != 7 {
		t.Errorf("row = %d, want 7", config.GetSelectedRow())
	}

	// Separate calls should trigger two notifications
	notificationCount = 0
	config.SetSelectedColumn("col-done")
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

	// Test SetSelectedColumn
	notified = false
	config.SetSelectedColumn("col-progress")
	if !notified {
		t.Error("SetSelectedColumn() did not trigger listener")
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
	config.SetSelectedColumn("col-review")
	if count1 != 1 {
		t.Errorf("listener1 count = %d, want 1", count1)
	}
	if count2 != 1 {
		t.Errorf("listener2 count = %d, want 1", count2)
	}

	// Remove second listener
	config.RemoveSelectionListener(id2)

	// Only first should be notified
	config.SetSelectedColumn("col-done")
	if count1 != 2 {
		t.Errorf("listener1 count = %d, want 2", count1)
	}
	if count2 != 1 {
		t.Errorf("listener2 count = %d, want 1 (should not change)", count2)
	}
}
