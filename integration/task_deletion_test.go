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
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Second Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Verify TIKI-1 visible
	found, _, _ := ta.FindText("TIKI-1")
	if !found {
		ta.DumpScreen()
		t.Fatalf("TIKI-1 should be visible before delete")
	}

	// Press 'd' to delete first task
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload and verify task deleted
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	task := ta.TaskStore.GetTask("TIKI-1")
	if task != nil {
		t.Errorf("TIKI-1 should be deleted from store")
	}

	// Verify file removed
	taskPath := filepath.Join(ta.TaskDir, "tiki-1.md")
	if _, err := os.Stat(taskPath); !os.IsNotExist(err) {
		t.Errorf("TIKI-1 file should be deleted")
	}

	// Verify TIKI-2 still visible
	found2, _, _ := ta.FindText("TIKI-2")
	if !found2 {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should still be visible after deleting TIKI-1")
	}
}

// TestTaskDeletion_SelectionMoves verifies selection moves to next task after delete
func TestTaskDeletion_SelectionMoves(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create three tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Second Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-3", "Third Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
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

	// Delete TIKI-2
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Selection should move to next task (TIKI-3, which is now at row 1)
	selectedRow := ta.BoardConfig.GetSelectedRow()
	if selectedRow != 1 {
		t.Errorf("selection after delete = row %d, want row 1", selectedRow)
	}

	// Verify TIKI-3 is visible
	found3, _, _ := ta.FindText("TIKI-3")
	if !found3 {
		ta.DumpScreen()
		t.Errorf("TIKI-3 should be visible after deleting TIKI-2")
	}
}

// TestTaskDeletion_LastTaskInPane verifies deleting last task resets selection
func TestTaskDeletion_LastTaskInPane(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create only one task in todo pane
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Only Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Verify TIKI-1 visible
	found, _, _ := ta.FindText("TIKI-1")
	if !found {
		ta.DumpScreen()
		t.Fatalf("TIKI-1 should be visible")
	}

	// Delete the only task
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Verify task deleted
	task := ta.TaskStore.GetTask("TIKI-1")
	if task != nil {
		t.Errorf("TIKI-1 should be deleted")
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
		taskID := fmt.Sprintf("TIKI-%d", i)
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

	// Delete first task again (was TIKI-2, now at top)
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Delete first task again (was TIKI-3, now at top)
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload and verify only 2 tasks remain
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	allTasks := ta.TaskStore.GetAllTasks()
	if len(allTasks) != 2 {
		t.Errorf("expected 2 tasks remaining, got %d", len(allTasks))
	}

	// Verify TIKI-4 and TIKI-5 still exist
	task4 := ta.TaskStore.GetTask("TIKI-4")
	task5 := ta.TaskStore.GetTask("TIKI-5")
	if task4 == nil || task5 == nil {
		t.Errorf("TIKI-4 and TIKI-5 should still exist")
	}
}

// TestTaskDeletion_FromDifferentPane verifies deleting from non-todo pane
func TestTaskDeletion_FromDifferentPane(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task in in_progress pane
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "In Progress Task", taskpkg.StatusInProgress, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Move to in_progress pane (Right arrow)
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)

	// Verify TIKI-1 visible
	found, _, _ := ta.FindText("TIKI-1")
	if !found {
		ta.DumpScreen()
		t.Fatalf("TIKI-1 should be visible in in_progress pane")
	}

	// Delete task
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload and verify deleted
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	task := ta.TaskStore.GetTask("TIKI-1")
	if task != nil {
		t.Errorf("TIKI-1 should be deleted")
	}
}

// TestTaskDeletion_CannotDeleteFromTaskDetail verifies 'd' doesn't work in task detail
func TestTaskDeletion_CannotDeleteFromTaskDetail(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Task to Not Delete", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
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

	task := ta.TaskStore.GetTask("TIKI-1")
	if task == nil {
		t.Errorf("TIKI-1 should NOT be deleted from task detail view")
	}

	// Verify we're still on task detail (or moved somewhere else, but task exists)
	if task == nil {
		t.Errorf("task should still exist after pressing 'd' in task detail")
	}
}

// TestTaskDeletion_WithMultiplePanes verifies deletion doesn't affect other panes
func TestTaskDeletion_WithMultiplePanes(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create tasks in different panes
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Todo Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "In Progress Task", taskpkg.StatusInProgress, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-3", "Done Task", taskpkg.StatusDone, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Delete TIKI-1 from todo pane
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Verify TIKI-1 deleted
	if ta.TaskStore.GetTask("TIKI-1") != nil {
		t.Errorf("TIKI-1 should be deleted")
	}

	// Verify TIKI-2 and TIKI-3 still exist (in other panes)
	if ta.TaskStore.GetTask("TIKI-2") == nil {
		t.Errorf("TIKI-2 (in different pane) should still exist")
	}
	if ta.TaskStore.GetTask("TIKI-3") == nil {
		t.Errorf("TIKI-3 (in different pane) should still exist")
	}
}
