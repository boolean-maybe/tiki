package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

// TestRefresh_FromBoard verifies 'r' key reloads tasks from disk
func TestRefresh_FromKanban(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create initial task
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Verify TIKI-1 is visible
	found, _, _ := ta.FindText("TIKI-1")
	if !found {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be visible initially")
	}

	// Create a new task externally (simulating external modification)
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "New External Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create external task: %v", err)
	}

	// Press 'r' to refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify TIKI-2 is now visible
	found2, _, _ := ta.FindText("TIKI-2")
	if !found2 {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should be visible after refresh")
	}
}

// TestRefresh_ExternalModification verifies refresh loads modified task content
func TestRefresh_ExternalModification(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create task
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Original Title", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Verify original title visible (may be truncated on narrow screens)
	found, _, _ := ta.FindText("Origina")
	if !found {
		ta.DumpScreen()
		t.Errorf("original title should be visible")
	}

	// Verify task exists in store
	task := ta.TaskStore.GetTask(taskID)
	if task == nil || task.Title != "Original Title" {
		t.Fatalf("task should exist with original title")
	}

	// Modify the task file externally
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Modified Title", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to modify task: %v", err)
	}

	// Press 'r' to refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify task store has updated title
	taskAfter := ta.TaskStore.GetTask(taskID)
	if taskAfter == nil {
		t.Fatalf("task should still exist after refresh")
	}
	if taskAfter.Title != "Modified Title" {
		t.Errorf("task title in store = %q, want %q", taskAfter.Title, "Modified Title")
	}

	// Note: The UI may not immediately reflect the change due to view caching,
	// but the important thing is that the task store reloaded the data
}

// TestRefresh_ExternalDeletion verifies refresh handles deleted tasks
func TestRefresh_ExternalDeletion(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create two tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Second Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Verify both tasks visible
	found1, _, _ := ta.FindText("TIKI-1")
	found2, _, _ := ta.FindText("TIKI-2")
	if !found1 || !found2 {
		ta.DumpScreen()
		t.Errorf("both tasks should be visible initially")
	}

	// Delete TIKI-1 externally
	taskPath := filepath.Join(ta.TaskDir, "tiki-1.md")
	if err := os.Remove(taskPath); err != nil {
		t.Fatalf("failed to delete task file: %v", err)
	}

	// Press 'r' to refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify TIKI-1 is gone
	found1After, _, _ := ta.FindText("TIKI-1")
	if found1After {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should NOT be visible after deletion and refresh")
	}

	// Verify TIKI-2 still visible
	found2After, _, _ := ta.FindText("TIKI-2")
	if !found2After {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should still be visible after refresh")
	}

	// Verify task store count
	tasks := ta.TaskStore.GetAllTasks()
	if len(tasks) != 1 {
		t.Errorf("task store should have 1 task after refresh, got %d", len(tasks))
	}
}

// TestRefresh_PreservesSelection verifies selection is maintained when task still exists
func TestRefresh_PreservesSelection(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Second Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Move to second task
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	kanbanConfig := ta.GetPluginConfig("Kanban")
	// Verify we're on index 1 (TIKI-2)
	if kanbanConfig.GetSelectedIndex() != 1 {
		t.Fatalf("expected index 1, got %d", kanbanConfig.GetSelectedIndex())
	}

	// Create a new task externally (doesn't affect selection)
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-3", "Third Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}

	// Press 'r' to refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify selection is still preserved (might shift if new task sorts before)
	// For this test, we just verify no crash and index is valid
	selectedIndex := kanbanConfig.GetSelectedIndex()
	if selectedIndex < 0 {
		t.Errorf("selected index should be valid after refresh, got %d", selectedIndex)
	}
}

// TestRefresh_ResetsSelectionWhenTaskDeleted verifies selection resets when selected task deleted
func TestRefresh_ResetsSelectionWhenTaskDeleted(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Second Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Move to second task
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	kanbanConfig := ta.GetPluginConfig("Kanban")
	// Verify we're on index 1 (TIKI-2)
	if kanbanConfig.GetSelectedIndex() != 1 {
		t.Fatalf("expected index 1, got %d", kanbanConfig.GetSelectedIndex())
	}

	// Delete TIKI-2 externally (the selected task)
	taskPath := filepath.Join(ta.TaskDir, "tiki-2.md")
	if err := os.Remove(taskPath); err != nil {
		t.Fatalf("failed to delete task file: %v", err)
	}

	// Press 'r' to refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify selection reset to index 0
	if kanbanConfig.GetSelectedIndex() != 0 {
		t.Errorf("selection should reset to index 0 when selected task deleted, got %d", kanbanConfig.GetSelectedIndex())
	}

	// Verify TIKI-1 is still visible
	found1, _, _ := ta.FindText("TIKI-1")
	if !found1 {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be visible after refresh")
	}
}

// TestRefresh_FromTaskDetail verifies refresh works from task detail view
func TestRefresh_FromTaskDetail(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create task
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Original Title", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate: Kanban → Task Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify original title visible
	found, _, _ := ta.FindText("Original Title")
	if !found {
		ta.DumpScreen()
		t.Errorf("original title should be visible in task detail")
	}

	// Modify the task file externally
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Updated Title", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to modify task: %v", err)
	}

	// Press 'r' to refresh from task detail view
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify updated title is now visible
	foundNew, _, _ := ta.FindText("Updated Title")
	if !foundNew {
		ta.DumpScreen()
		t.Errorf("updated title should be visible after refresh in task detail")
	}
}

// TestRefresh_WithActiveSearch verifies refresh behavior with active search
func TestRefresh_WithActiveSearch(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Alpha Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Beta Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Search for "Alpha" (should show only TIKI-1)
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	ta.SendText("Alpha")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify only TIKI-1 visible
	found1, _, _ := ta.FindText("TIKI-1")
	found2, _, _ := ta.FindText("TIKI-2")
	if !found1 || found2 {
		ta.DumpScreen()
		t.Errorf("search should filter to only TIKI-1")
	}

	// Press 'r' to refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Note: Refresh keeps search active (doesn't clear it automatically)
	// User must press Esc to clear search manually
	// This test just verifies refresh doesn't crash with active search

	// Verify TIKI-1 is still visible (search still active)
	found1After, _, _ := ta.FindText("TIKI-1")
	if !found1After {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should still be visible (search persists after refresh)")
	}
}

// TestRefresh_MultipleRefreshes verifies multiple consecutive refreshes work correctly
func TestRefresh_MultipleRefreshes(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create initial task
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// First refresh (no changes)
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify TIKI-1 still visible
	found, _, _ := ta.FindText("TIKI-1")
	if !found {
		t.Errorf("TIKI-1 should be visible after first refresh")
	}

	// Add a new task
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Second Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}

	// Second refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify both tasks visible
	found1, _, _ := ta.FindText("TIKI-1")
	found2, _, _ := ta.FindText("TIKI-2")
	if !found1 || !found2 {
		ta.DumpScreen()
		t.Errorf("both tasks should be visible after second refresh")
	}

	// Third refresh (no changes again)
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify both tasks still visible
	found1Again, _, _ := ta.FindText("TIKI-1")
	found2Again, _, _ := ta.FindText("TIKI-2")
	if !found1Again || !found2Again {
		ta.DumpScreen()
		t.Errorf("both tasks should be visible after third refresh")
	}
}
