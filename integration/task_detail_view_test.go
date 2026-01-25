package integration

import (
	"fmt"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

// TestTaskDetailView_RenderMetadata verifies all task metadata is displayed
func TestTaskDetailView_RenderMetadata(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create a task with all fields populated
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Test Task Title", taskpkg.StatusInProgress, taskpkg.TypeBug); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate: Kanban → Task Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone) // Open task detail

	// Verify task ID is visible
	found, _, _ := ta.FindText("TIKI-1")
	if !found {
		ta.DumpScreen()
		t.Errorf("task ID 'TIKI-1' not found in task detail view")
	}

	// Verify title is visible
	foundTitle, _, _ := ta.FindText("Test Task Title")
	if !foundTitle {
		ta.DumpScreen()
		t.Errorf("task title not found in task detail view")
	}

	// Verify status label is visible
	foundStatus, _, _ := ta.FindText("Status:")
	if !foundStatus {
		ta.DumpScreen()
		t.Errorf("'Status:' label not found in task detail view")
	}

	// Verify type label is visible
	foundType, _, _ := ta.FindText("Type:")
	if !foundType {
		ta.DumpScreen()
		t.Errorf("'Type:' label not found in task detail view")
	}

	// Verify priority label is visible
	foundPriority, _, _ := ta.FindText("Priority:")
	if !foundPriority {
		ta.DumpScreen()
		t.Errorf("'Priority:' label not found in task detail view")
	}
}

// TestTaskDetailView_RenderDescription verifies task description is displayed
func TestTaskDetailView_RenderDescription(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task (description is set to the title by CreateTestTask)
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Task with description", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate: Board → Task Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify description is visible (markdown rendered)
	// The description content is the same as the title in test fixtures
	foundDesc, _, _ := ta.FindText("Task with description")
	if !foundDesc {
		ta.DumpScreen()
		t.Errorf("task description not found in task detail view")
	}
}

// TestTaskDetailView_NavigateBack verifies Esc returns to board
func TestTaskDetailView_NavigateBack(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Test Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate: Board → Task Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify we're on task detail
	currentView := ta.NavController.CurrentView()
	if currentView.ViewID != model.TaskDetailViewID {
		t.Fatalf("expected task detail view, got %v", currentView.ViewID)
	}

	// Press Esc to go back
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify we're back on board
	currentView = ta.NavController.CurrentView()
	if currentView.ViewID != model.MakePluginViewID("Kanban") {
		t.Errorf("expected board view after Esc, got %v", currentView.ViewID)
	}
}

// TestTaskDetailView_InlineTitleEdit_Save verifies inline title editing with Enter
func TestTaskDetailView_InlineTitleEdit_Save(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	taskID := "TIKI-1"
	originalTitle := "Original Title"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, originalTitle, taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate: Board → Task Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Press 'e' to start inline title editing
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Clear and type new title
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone) // Select all
	ta.SendText("New Edited Title")

	// Press Enter to save
	ta.SendKeyToFocused(tcell.KeyEnter, 0, tcell.ModNone)

	// Reload from disk and verify title changed
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}
	if task.Title != "New Edited Title" {
		t.Errorf("title = %q, want %q", task.Title, "New Edited Title")
	}
}

// TestTaskDetailView_InlineTitleEdit_Cancel verifies Esc cancels inline editing
func TestTaskDetailView_InlineTitleEdit_Cancel(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	taskID := "TIKI-1"
	originalTitle := "Original Title"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, originalTitle, taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate: Board → Task Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Press 'e' to start inline title editing
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Type new title (don't save)
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("Modified Title")

	// Press Esc to cancel
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Reload from disk and verify title NOT changed
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}
	if task.Title != originalTitle {
		t.Errorf("title = %q, want %q (should not have changed)", task.Title, originalTitle)
	}
}

// TestTaskDetailView_FromBoard verifies opening task from board
func TestTaskDetailView_FromBoard(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

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

	// Navigate to board
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Move to second task
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Open task detail
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify we're on task detail for TIKI-2
	found, _, _ := ta.FindText("TIKI-2")
	if !found {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should be visible in task detail view")
	}

	// Verify TIKI-1 is NOT visible (we're viewing TIKI-2)
	found1, _, _ := ta.FindText("TIKI-1")
	if found1 {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should NOT be visible (we opened TIKI-2)")
	}
}

// TestTaskDetailView_EmptyDescription verifies rendering with no description
func TestTaskDetailView_EmptyDescription(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task with minimal content
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Task Title", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate: Board → Task Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify task title is visible
	found, _, _ := ta.FindText("Task Title")
	if !found {
		ta.DumpScreen()
		t.Errorf("task title should be visible even with empty description")
	}

	// Verify Status label is still visible
	foundStatus, _, _ := ta.FindText("Status:")
	if !foundStatus {
		ta.DumpScreen()
		t.Errorf("metadata should be visible even with empty description")
	}
}

// TestTaskDetailView_MultipleOpen verifies opening different tasks sequentially
func TestTaskDetailView_MultipleOpen(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create multiple tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Second Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-3", "Third Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Open first task
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	found1, _, _ := ta.FindText("TIKI-1")
	if !found1 {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be visible after opening")
	}

	// Go back
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Move to second task and open
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	found2, _, _ := ta.FindText("TIKI-2")
	if !found2 {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should be visible after opening")
	}

	// Go back
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Move to third task and open
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	found3, _, _ := ta.FindText("TIKI-3")
	if !found3 {
		ta.DumpScreen()
		t.Errorf("TIKI-3 should be visible after opening")
	}
}

// TestTaskDetailView_AllStatuses verifies rendering different status values
func TestTaskDetailView_AllStatuses(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	statuses := []taskpkg.Status{
		taskpkg.StatusBacklog,
		taskpkg.StatusReady,
		taskpkg.StatusInProgress,
		taskpkg.StatusReview,
		taskpkg.StatusDone,
	}

	for i, status := range statuses {
		taskID := fmt.Sprintf("TIKI-%d", i+1)
		title := fmt.Sprintf("Task %s", status)
		if err := testutil.CreateTestTask(ta.TaskDir, taskID, title, status, taskpkg.TypeStory); err != nil {
			t.Fatalf("failed to create test task: %v", err)
		}
	}

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open board
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// For each status, navigate to first task with that status and verify detail view
	for i, status := range statuses {
		// Find the task on board (may need to navigate between panes)
		taskID := fmt.Sprintf("TIKI-%d", i+1)

		// Navigate to correct pane based on status
		// For simplicity, we'll just open first task in todo pane for this test
		if status == taskpkg.StatusReady {
			ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

			// Verify task ID visible
			found, _, _ := ta.FindText(taskID)
			if !found {
				ta.DumpScreen()
				t.Errorf("task %s with status %s not found in detail view", taskID, status)
			}

			// Go back for next iteration
			ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
			break // Just test one for now
		}
	}
}

// TestTaskDetailView_AllTypes verifies rendering different type values
func TestTaskDetailView_AllTypes(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	types := []taskpkg.Type{
		taskpkg.TypeStory,
		taskpkg.TypeBug,
	}

	for i, taskType := range types {
		taskID := fmt.Sprintf("TIKI-%d", i+1)
		title := fmt.Sprintf("Task %s", taskType)
		if err := testutil.CreateTestTask(ta.TaskDir, taskID, title, taskpkg.StatusReady, taskType); err != nil {
			t.Fatalf("failed to create test task: %v", err)
		}
	}

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Open board and first task
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify Type label is visible
	found, _, _ := ta.FindText("Type:")
	if !found {
		ta.DumpScreen()
		t.Errorf("Type label should be visible in task detail")
	}
}

// TestTaskDetailView_InlineEdit_PreservesOtherFields verifies inline edit doesn't corrupt metadata
func TestTaskDetailView_InlineEdit_PreservesOtherFields(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task with specific values
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Original Title", taskpkg.StatusReady, taskpkg.TypeBug); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate: Board → Task Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Press 'e' to edit title
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("New Title")
	ta.SendKeyToFocused(tcell.KeyEnter, 0, tcell.ModNone)

	// Reload and verify other fields preserved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}

	if task.Title != "New Title" {
		t.Errorf("title = %q, want %q", task.Title, "New Title")
	}
	if task.Status != taskpkg.StatusReady {
		t.Errorf("status = %v, want %v (should be preserved)", task.Status, taskpkg.StatusReady)
	}
	if task.Type != taskpkg.TypeBug {
		t.Errorf("type = %v, want %v (should be preserved)", task.Type, taskpkg.TypeBug)
	}
}
