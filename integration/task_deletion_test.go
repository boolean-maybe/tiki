package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

// TestTaskDeletion_FromBoard verifies 'd' deletes task from board
func TestTaskDeletion_FromBoard(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-1", "First Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-2", "Second Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Verify TEST-1 visible
	found, _, _ := ta.FindText("TEST-1")
	if !found {
		ta.DumpScreen()
		t.Fatalf("TEST-1 should be visible before delete")
	}

	// Press 'd' to delete first task
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload and verify task deleted
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	task := ta.TaskStore.GetTask("TEST-1")
	if task != nil {
		t.Errorf("TEST-1 should be deleted from store")
	}

	// Verify file removed
	taskPath := filepath.Join(ta.TaskDir, "test-1.md")
	if _, err := os.Stat(taskPath); !os.IsNotExist(err) {
		t.Errorf("TEST-1 file should be deleted")
	}

	// Verify TEST-2 still visible
	found2, _, _ := ta.FindText("TEST-2")
	if !found2 {
		ta.DumpScreen()
		t.Errorf("TEST-2 should still be visible after deleting TEST-1")
	}
}

// TestTaskDeletion_SelectionMoves verifies selection moves to next task after delete
func TestTaskDeletion_SelectionMoves(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create three tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-1", "First Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-2", "Second Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-3", "Third Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Move to second task (row 1)
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Verify we're on row 1
	if ta.BoardConfig.GetSelectedRow() != 1 {
		t.Fatalf("expected row 1, got %d", ta.BoardConfig.GetSelectedRow())
	}

	// Delete TEST-2
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Selection should move to next task (TEST-3, which is now at row 1)
	selectedRow := ta.BoardConfig.GetSelectedRow()
	if selectedRow != 1 {
		t.Errorf("selection after delete = row %d, want row 1", selectedRow)
	}

	// Verify TEST-3 is visible
	found3, _, _ := ta.FindText("TEST-3")
	if !found3 {
		ta.DumpScreen()
		t.Errorf("TEST-3 should be visible after deleting TEST-2")
	}
}

// TestTaskDeletion_LastTaskInColumn verifies deleting last task resets selection
func TestTaskDeletion_LastTaskInColumn(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create only one task in todo column
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-1", "Only Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Verify TEST-1 visible
	found, _, _ := ta.FindText("TEST-1")
	if !found {
		ta.DumpScreen()
		t.Fatalf("TEST-1 should be visible")
	}

	// Delete the only task
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Verify task deleted
	task := ta.TaskStore.GetTask("TEST-1")
	if task != nil {
		t.Errorf("TEST-1 should be deleted")
	}

	// Verify selection reset to 0
	if ta.BoardConfig.GetSelectedRow() != 0 {
		t.Errorf("selection should reset to 0 after deleting last task, got %d", ta.BoardConfig.GetSelectedRow())
	}

	// Verify no crash occurred (board is empty)
	// This is implicit - if we got here without panic, test passes
}

// TestTaskDeletion_MultipleSequential verifies deleting multiple tasks in sequence
func TestTaskDeletion_MultipleSequential(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create five tasks
	for i := 1; i <= 5; i++ {
		taskID := fmt.Sprintf("TEST-%d", i)
		title := fmt.Sprintf("Task %d", i)
		if err := testutil.CreateTestTask(ta.TaskDir, taskID, title, taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
			t.Fatalf("failed to create task: %v", err)
		}
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Delete first task
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Delete first task again (was TEST-2, now at top)
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Delete first task again (was TEST-3, now at top)
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload and verify only 2 tasks remain
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	allTasks := ta.TaskStore.GetAllTasks()
	if len(allTasks) != 2 {
		t.Errorf("expected 2 tasks remaining, got %d", len(allTasks))
	}

	// Verify TEST-4 and TEST-5 still exist
	task4 := ta.TaskStore.GetTask("TEST-4")
	task5 := ta.TaskStore.GetTask("TEST-5")
	if task4 == nil || task5 == nil {
		t.Errorf("TEST-4 and TEST-5 should still exist")
	}
}

// TestTaskDeletion_FromDifferentColumn verifies deleting from non-todo column
func TestTaskDeletion_FromDifferentColumn(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task in in_progress column
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-1", "In Progress Task", taskpkg.StatusInProgress, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Move to in_progress column (Right arrow)
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)

	// Verify TEST-1 visible
	found, _, _ := ta.FindText("TEST-1")
	if !found {
		ta.DumpScreen()
		t.Fatalf("TEST-1 should be visible in in_progress column")
	}

	// Delete task
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload and verify deleted
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	task := ta.TaskStore.GetTask("TEST-1")
	if task != nil {
		t.Errorf("TEST-1 should be deleted")
	}
}

// TestTaskDeletion_CannotDeleteFromTaskDetail verifies 'd' doesn't work in task detail
func TestTaskDeletion_CannotDeleteFromTaskDetail(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-1", "Task to Not Delete", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate: Board → Task Detail
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify we're on task detail
	currentView := ta.NavController.CurrentView()
	if currentView.ViewID != model.TaskDetailViewID {
		t.Fatalf("expected task detail view, got %v", currentView.ViewID)
	}

	// Press 'd' (should not delete from task detail view)
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload and verify task still exists
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	task := ta.TaskStore.GetTask("TEST-1")
	if task == nil {
		t.Errorf("TEST-1 should NOT be deleted from task detail view")
	}

	// Verify we're still on task detail (or moved somewhere else, but task exists)
	if task == nil {
		t.Errorf("task should still exist after pressing 'd' in task detail")
	}
}

// TestTaskDeletion_WithMultipleColumns verifies deletion doesn't affect other columns
func TestTaskDeletion_WithMultipleColumns(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create tasks in different columns
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-1", "Todo Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-2", "In Progress Task", taskpkg.StatusInProgress, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TEST-3", "Done Task", taskpkg.StatusDone, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Delete TEST-1 from todo column
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Verify TEST-1 deleted
	if ta.TaskStore.GetTask("TEST-1") != nil {
		t.Errorf("TEST-1 should be deleted")
	}

	// Verify TEST-2 and TEST-3 still exist (in other columns)
	if ta.TaskStore.GetTask("TEST-2") == nil {
		t.Errorf("TEST-2 (in different column) should still exist")
	}
	if ta.TaskStore.GetTask("TEST-3") == nil {
		t.Errorf("TEST-3 (in different column) should still exist")
	}
}
