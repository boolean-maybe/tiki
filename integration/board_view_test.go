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

func TestBoardView_PaneHeadersRender(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create sample tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Task in Todo", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Task in Progress", taskpkg.StatusInProgress, taskpkg.TypeStory); err != nil {
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

	// Verify pane headers appear
	// Panes may be abbreviated/truncated based on terminal width
	// The actual rendering shows: "To", "In", "Revi", "Done" (or similar)
	tests := []struct {
		name       string
		searchText string
	}{
		{"todo pane", "To"},
		{"in progress pane", "In"},
		{"review pane", "Revi"},
		{"done pane", "Done"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, _, _ := ta.FindText(tt.searchText)
			if !found {
				t.Errorf("pane header %q not found on screen", tt.searchText)
			}
		})
	}
}

func TestBoardView_ArrowKeyNavigation(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create 3 tasks in todo pane
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Second Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-3", "Third Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Push view (this calls OnFocus which registers the listener and does initial refresh)
	ta.NavController.PushView(model.BoardViewID, nil)
	// Draw to render the board with tasks
	ta.Draw()

	// Initial state: TIKI-1 should be selected (verify by finding it on screen)
	found, _, _ := ta.FindText("TIKI-1")
	if !found {
		t.Fatalf("initial task TIKI-1 not found")
	}

	// Press Down arrow
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Verify TIKI-2 visible (selection moved down)
	found, _, _ = ta.FindText("TIKI-2")
	if !found {
		t.Errorf("after Down arrow, TIKI-2 not found")
	}

	// Verify board config selection changed to row 1
	selectedRow := ta.BoardConfig.GetSelectedRow()
	if selectedRow != 1 {
		t.Errorf("selected row = %d, want 1", selectedRow)
	}

	// Press Down arrow again to move to row 2
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Verify TIKI-3 visible (selection moved down)
	found, _, _ = ta.FindText("TIKI-3")
	if !found {
		t.Errorf("after second Down arrow, TIKI-3 not found")
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

	// Create task in todo pane
	taskID := "TIKI-1"
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

	// Verify task starts in TODO pane
	found, _, _ := ta.FindText("TIKI-1")
	if !found {
		t.Fatalf("task TIKI-1 not found initially")
	}

	// Press Shift+Right to move to next pane
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
	taskPath := filepath.Join(ta.TaskDir, "tiki-1.md")
	content, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("failed to read task file: %v", err)
	}
	if !strings.Contains(string(content), "status: in_progress") {
		t.Errorf("task file does not contain updated status")
	}
}

// ============================================================================
// Phase 3: View Mode Toggle and Pane Navigation Tests
// ============================================================================

// TestBoardView_ViewModeToggle verifies 'v' key toggles view mode
func TestBoardView_ViewModeToggle(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Task 1", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
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
		taskID := "TIKI-" + string(rune('0'+i))
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
	selectedPaneBefore := ta.BoardConfig.GetSelectedPaneID()

	if selectedRowBefore != 1 {
		t.Fatalf("expected row 1, got %d", selectedRowBefore)
	}

	// Toggle view mode
	ta.SendKey(tcell.KeyRune, 'v', tcell.ModNone)

	// Verify selection preserved
	selectedRowAfter := ta.BoardConfig.GetSelectedRow()
	selectedPaneAfter := ta.BoardConfig.GetSelectedPaneID()

	if selectedRowAfter != selectedRowBefore {
		t.Errorf("selected row = %d, want %d (should be preserved)", selectedRowAfter, selectedRowBefore)
	}
	if selectedPaneAfter != selectedPaneBefore {
		t.Errorf("selected pane = %s, want %s (should be preserved)", selectedPaneAfter, selectedPaneBefore)
	}
}

// TestBoardView_LeftRightArrowMovesBetweenPanes verifies pane navigation
func TestBoardView_LeftRightArrowMovesBetweenPanes(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create tasks in different panes
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Todo Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "In Progress Task", taskpkg.StatusInProgress, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-3", "Review Task", taskpkg.StatusReview, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Initial pane should be col-todo
	if ta.BoardConfig.GetSelectedPaneID() != "col-todo" {
		t.Fatalf("expected initial pane col-todo, got %s", ta.BoardConfig.GetSelectedPaneID())
	}

	// Press Right arrow to move to in_progress pane
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)

	// Verify moved to col-progress
	if ta.BoardConfig.GetSelectedPaneID() != "col-progress" {
		t.Errorf("selected pane = %s, want col-progress", ta.BoardConfig.GetSelectedPaneID())
	}

	// Press Right arrow again to move to review pane
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)

	// Verify moved to col-review
	if ta.BoardConfig.GetSelectedPaneID() != "col-review" {
		t.Errorf("selected pane = %s, want col-review", ta.BoardConfig.GetSelectedPaneID())
	}

	// Press Left arrow to move back to in_progress
	ta.SendKey(tcell.KeyLeft, 0, tcell.ModNone)

	// Verify moved back to col-progress
	if ta.BoardConfig.GetSelectedPaneID() != "col-progress" {
		t.Errorf("selected pane = %s, want col-progress", ta.BoardConfig.GetSelectedPaneID())
	}

	// Press Left arrow again to move back to todo
	ta.SendKey(tcell.KeyLeft, 0, tcell.ModNone)

	// Verify moved back to col-todo
	if ta.BoardConfig.GetSelectedPaneID() != "col-todo" {
		t.Errorf("selected pane = %s, want col-todo", ta.BoardConfig.GetSelectedPaneID())
	}
}

// TestBoardView_NavigateToEmptyPane verifies navigation skips or handles empty panes
func TestBoardView_NavigateToEmptyPane(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task only in todo pane (leave in_progress empty)
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Todo Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Review Task", taskpkg.StatusReview, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Start at todo pane
	if ta.BoardConfig.GetSelectedPaneID() != "col-todo" {
		t.Fatalf("expected initial pane col-todo, got %s", ta.BoardConfig.GetSelectedPaneID())
	}

	// Press Right arrow to move to in_progress pane (empty)
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)

	// Verify moved to a valid pane (implementation may skip empty or stay)
	selectedPane := ta.BoardConfig.GetSelectedPaneID()
	validPanes := map[string]bool{"col-todo": true, "col-progress": true, "col-review": true, "col-done": true}
	if !validPanes[selectedPane] {
		t.Errorf("selected pane %s should be valid", selectedPane)
	}

	// Verify selection row is valid (0 in empty pane)
	selectedRow := ta.BoardConfig.GetSelectedRow()
	if selectedRow < 0 {
		t.Errorf("selected row %d should be non-negative", selectedRow)
	}
}

// TestBoardView_MultiplePanesNavigation verifies full pane traversal
func TestBoardView_MultiplePanesNavigation(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create tasks in all panes
	statuses := []taskpkg.Status{
		taskpkg.StatusTodo,
		taskpkg.StatusInProgress,
		taskpkg.StatusReview,
		taskpkg.StatusDone,
	}

	for i, status := range statuses {
		taskID := "TIKI-" + string(rune('1'+i))
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

	// Navigate through all panes with Right arrow
	expectedPanes := []string{"col-todo", "col-progress", "col-review", "col-done"}
	for i, expectedPane := range expectedPanes {
		actualPane := ta.BoardConfig.GetSelectedPaneID()
		if actualPane != expectedPane {
			t.Errorf("after %d Right presses, pane = %s, want %s", i, actualPane, expectedPane)
		}
		if i < 3 {
			ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)
		}
	}

	// Navigate back through all panes with Left arrow
	reversedPanes := []string{"col-done", "col-review", "col-progress", "col-todo"}
	for i, expectedPane := range reversedPanes {
		actualPane := ta.BoardConfig.GetSelectedPaneID()
		if actualPane != expectedPane {
			t.Errorf("after %d Left presses, pane = %s, want %s", i, actualPane, expectedPane)
		}
		if i < 3 {
			ta.SendKey(tcell.KeyLeft, 0, tcell.ModNone)
		}
	}
}
