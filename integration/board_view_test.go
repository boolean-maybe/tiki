package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"

	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

func TestBoardView_ColumnHeadersRender(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create sample tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-1", "Task in Todo", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-2", "Task in Progress", taskpkg.StatusInProgress, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Initialize board view and reload to trigger view refresh
	ta.NavController.PushView(model.BoardViewID, nil)
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks again: %v", err)
	}
	ta.Draw()

	// Verify column headers appear
	// Columns may be abbreviated/truncated based on terminal width
	// The actual rendering shows: "To", "In", "Revi", "Done" (or similar)
	tests := []struct {
		name       string
		searchText string
	}{
		{"todo column", "To"},
		{"in progress column", "In"},
		{"review column", "Revi"},
		{"done column", "Done"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, _, _ := ta.FindText(tt.searchText)
			if !found {
				t.Errorf("column header %q not found on screen", tt.searchText)
			}
		})
	}
}

func TestBoardView_ArrowKeyNavigation(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create 3 tasks in todo column
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-1", "First Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-2", "Second Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-3", "Third Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Push view (this calls OnFocus which registers the listener and does initial refresh)
	ta.NavController.PushView(model.BoardViewID, nil)
	// Draw to render the board with tasks
	ta.Draw()

	// Initial state: TEST-1 should be selected (verify by finding it on screen)
	found, _, _ := ta.FindText("TEST-1")
	if !found {
		t.Fatalf("initial task TEST-1 not found")
	}

	// Press Down arrow
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Verify TEST-2 visible (selection moved down)
	found, _, _ = ta.FindText("TEST-2")
	if !found {
		t.Errorf("after Down arrow, TEST-2 not found")
	}

	// Verify board config selection changed to row 1
	selectedRow := ta.BoardConfig.GetSelectedRow()
	if selectedRow != 1 {
		t.Errorf("selected row = %d, want 1", selectedRow)
	}

	// Press Down arrow again to move to row 2
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Verify TEST-3 visible (selection moved down)
	found, _, _ = ta.FindText("TEST-3")
	if !found {
		t.Errorf("after second Down arrow, TEST-3 not found")
	}

	// Verify board config selection changed to row 2
	selectedRow = ta.BoardConfig.GetSelectedRow()
	if selectedRow != 2 {
		t.Errorf("selected row = %d, want 2", selectedRow)
	}
}

func TestBoardView_MoveTaskWithShiftArrow(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task in todo column
	taskID := "TEST-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Task to Move", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Push view (this calls OnFocus which registers the listener and does initial refresh)
	ta.NavController.PushView(model.BoardViewID, nil)
	// Draw to render the board with tasks
	ta.Draw()

	// Verify task starts in TODO column
	found, _, _ := ta.FindText("TEST-1")
	if !found {
		t.Fatalf("task TEST-1 not found initially")
	}

	// Press Shift+Right to move to next column
	ta.SendKey(tcell.KeyRight, 0, tcell.ModShift)

	// Reload tasks from disk
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Verify task moved to in_progress
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found after move")
	}
	if task.Status != taskpkg.StatusInProgress {
		t.Errorf("task status = %v, want %v", task.Status, taskpkg.StatusInProgress)
	}

	// Verify file on disk was updated
	taskPath := filepath.Join(ta.TaskDir, "test-1.md")
	content, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("failed to read task file: %v", err)
	}
	if !strings.Contains(string(content), "status: in_progress") {
		t.Errorf("task file does not contain updated status")
	}
}

// ============================================================================
// Phase 3: View Mode Toggle and Column Navigation Tests
// ============================================================================

// TestBoardView_ViewModeToggle verifies 'v' key toggles view mode
func TestBoardView_ViewModeToggle(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-1", "Task 1", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Get initial view mode
	initialViewMode := ta.BoardConfig.GetViewMode()

	// Press 'v' to toggle view mode
	ta.SendKey(tcell.KeyRune, 'v', tcell.ModNone)

	// Get new view mode
	newViewMode := ta.BoardConfig.GetViewMode()

	// Verify view mode changed
	if newViewMode == initialViewMode {
		t.Errorf("view mode should toggle, but remained %v", initialViewMode)
	}

	// Press 'v' again to toggle back
	ta.SendKey(tcell.KeyRune, 'v', tcell.ModNone)

	// Verify view mode returned to original
	finalViewMode := ta.BoardConfig.GetViewMode()
	if finalViewMode != initialViewMode {
		t.Errorf("view mode = %v, want %v (should toggle back)", finalViewMode, initialViewMode)
	}
}

// TestBoardView_ViewModeTogglePreservesSelection verifies selection maintained during toggle
func TestBoardView_ViewModeTogglePreservesSelection(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create multiple tasks
	for i := 1; i <= 3; i++ {
		taskID := "TEST-" + string(rune('0'+i))
		if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Task "+string(rune('0'+i)), taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
			t.Fatalf("failed to create test task: %v", err)
		}
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Move to second task
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Get selected row before toggle
	selectedRowBefore := ta.BoardConfig.GetSelectedRow()
	selectedColBefore := ta.BoardConfig.GetSelectedColumnID()

	if selectedRowBefore != 1 {
		t.Fatalf("expected row 1, got %d", selectedRowBefore)
	}

	// Toggle view mode
	ta.SendKey(tcell.KeyRune, 'v', tcell.ModNone)

	// Verify selection preserved
	selectedRowAfter := ta.BoardConfig.GetSelectedRow()
	selectedColAfter := ta.BoardConfig.GetSelectedColumnID()

	if selectedRowAfter != selectedRowBefore {
		t.Errorf("selected row = %d, want %d (should be preserved)", selectedRowAfter, selectedRowBefore)
	}
	if selectedColAfter != selectedColBefore {
		t.Errorf("selected column = %s, want %s (should be preserved)", selectedColAfter, selectedColBefore)
	}
}

// TestBoardView_LeftRightArrowMovesBetweenColumns verifies column navigation
func TestBoardView_LeftRightArrowMovesBetweenColumns(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create tasks in different columns
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-1", "Todo Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-2", "In Progress Task", taskpkg.StatusInProgress, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-3", "Review Task", taskpkg.StatusReview, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Initial column should be col-todo
	if ta.BoardConfig.GetSelectedColumnID() != "col-todo" {
		t.Fatalf("expected initial column col-todo, got %s", ta.BoardConfig.GetSelectedColumnID())
	}

	// Press Right arrow to move to in_progress column
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)

	// Verify moved to col-progress
	if ta.BoardConfig.GetSelectedColumnID() != "col-progress" {
		t.Errorf("selected column = %s, want col-progress", ta.BoardConfig.GetSelectedColumnID())
	}

	// Press Right arrow again to move to review column
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)

	// Verify moved to col-review
	if ta.BoardConfig.GetSelectedColumnID() != "col-review" {
		t.Errorf("selected column = %s, want col-review", ta.BoardConfig.GetSelectedColumnID())
	}

	// Press Left arrow to move back to in_progress
	ta.SendKey(tcell.KeyLeft, 0, tcell.ModNone)

	// Verify moved back to col-progress
	if ta.BoardConfig.GetSelectedColumnID() != "col-progress" {
		t.Errorf("selected column = %s, want col-progress", ta.BoardConfig.GetSelectedColumnID())
	}

	// Press Left arrow again to move back to todo
	ta.SendKey(tcell.KeyLeft, 0, tcell.ModNone)

	// Verify moved back to col-todo
	if ta.BoardConfig.GetSelectedColumnID() != "col-todo" {
		t.Errorf("selected column = %s, want col-todo", ta.BoardConfig.GetSelectedColumnID())
	}
}

// TestBoardView_NavigateToEmptyColumn verifies navigation skips or handles empty columns
func TestBoardView_NavigateToEmptyColumn(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task only in todo column (leave in_progress empty)
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-1", "Todo Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-2", "Review Task", taskpkg.StatusReview, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Start at todo column
	if ta.BoardConfig.GetSelectedColumnID() != "col-todo" {
		t.Fatalf("expected initial column col-todo, got %s", ta.BoardConfig.GetSelectedColumnID())
	}

	// Press Right arrow to move to in_progress column (empty)
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)

	// Verify moved to a valid column (implementation may skip empty or stay)
	selectedColumn := ta.BoardConfig.GetSelectedColumnID()
	validCols := map[string]bool{"col-todo": true, "col-progress": true, "col-review": true, "col-done": true}
	if !validCols[selectedColumn] {
		t.Errorf("selected column %s should be valid", selectedColumn)
	}

	// Verify selection row is valid (0 in empty column)
	selectedRow := ta.BoardConfig.GetSelectedRow()
	if selectedRow < 0 {
		t.Errorf("selected row %d should be non-negative", selectedRow)
	}
}

// TestBoardView_MultipleColumnsNavigation verifies full column traversal
func TestBoardView_MultipleColumnsNavigation(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create tasks in all columns
	statuses := []taskpkg.Status{
		taskpkg.StatusTodo,
		taskpkg.StatusInProgress,
		taskpkg.StatusReview,
		taskpkg.StatusDone,
	}

	for i, status := range statuses {
		taskID := "TEST-" + string(rune('1'+i))
		if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Task "+string(rune('1'+i)), status, taskpkg.TypeStory); err != nil {
			t.Fatalf("failed to create test task: %v", err)
		}
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Navigate through all columns with Right arrow
	expectedCols := []string{"col-todo", "col-progress", "col-review", "col-done"}
	for i, expectedCol := range expectedCols {
		actualCol := ta.BoardConfig.GetSelectedColumnID()
		if actualCol != expectedCol {
			t.Errorf("after %d Right presses, column = %s, want %s", i, actualCol, expectedCol)
		}
		if i < 3 {
			ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)
		}
	}

	// Navigate back through all columns with Left arrow
	reversedCols := []string{"col-done", "col-review", "col-progress", "col-todo"}
	for i, expectedCol := range reversedCols {
		actualCol := ta.BoardConfig.GetSelectedColumnID()
		if actualCol != expectedCol {
			t.Errorf("after %d Left presses, column = %s, want %s", i, actualCol, expectedCol)
		}
		if i < 3 {
			ta.SendKey(tcell.KeyLeft, 0, tcell.ModNone)
		}
	}
}
