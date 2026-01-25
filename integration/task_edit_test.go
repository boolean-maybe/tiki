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

// findTaskByTitle finds a task by its title in a slice of tasks
func findTaskByTitle(tasks []*taskpkg.Task, title string) *taskpkg.Task {
	for _, t := range tasks {
		if t.Title == title {
			return t
		}
	}
	return nil
}

// =============================================================================
// NEW TASK CREATION (Draft Mode) Tests
// =============================================================================

func TestNewTask_Enter_SavesAndCreatesFile(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Start on board view
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Press 'n' to create new task (opens edit view with title focused)
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Type title
	ta.SendText("My New Task")

	// Press Enter to save
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify: file should be created
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Find the new task by title (IDs are now random)
	task := findTaskByTitle(ta.TaskStore.GetAllTasks(), "My New Task")
	if task == nil {
		t.Fatalf("new task not found in store")
	}
	if task.Title != "My New Task" {
		t.Errorf("title = %q, want %q", task.Title, "My New Task")
	}

	// Verify file exists on disk (filename uses lowercase ID)
	taskPath := filepath.Join(ta.TaskDir, strings.ToLower(task.ID)+".md")
	if _, err := os.Stat(taskPath); os.IsNotExist(err) {
		t.Errorf("task file was not created at %s", taskPath)
	}
}

func TestNewTask_Escape_DiscardsWithoutCreatingFile(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Start on board view
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Press 'n' to create new task
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Type title
	ta.SendText("Task To Discard")

	// Press Escape to cancel
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify: no file should be created
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Should have no tasks (find by title since IDs are random)
	task := findTaskByTitle(ta.TaskStore.GetAllTasks(), "Task To Discard")
	if task != nil {
		t.Errorf("task should not exist after escape, but found: %+v", task)
	}

	// Verify no tiki files on disk
	files, _ := filepath.Glob(filepath.Join(ta.TaskDir, "tiki-*.md"))
	if len(files) > 0 {
		t.Errorf("task files should not exist, but found: %v", files)
	}
}

func TestNewTask_CtrlS_SavesAndCreatesFile(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Start on board view
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Press 'n' to create new task
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Type title
	ta.SendText("Task Saved With CtrlS")

	// Tab to another field (Points)
	for i := 0; i < 5; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Press Ctrl+S to save
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Verify: file should be created
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	task := findTaskByTitle(ta.TaskStore.GetAllTasks(), "Task Saved With CtrlS")
	if task == nil {
		t.Fatalf("new task not found in store")
	}
	if task.Title != "Task Saved With CtrlS" {
		t.Errorf("title = %q, want %q", task.Title, "Task Saved With CtrlS")
	}
}

func TestNewTask_EmptyTitle_DoesNotSave(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Start on board view
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Press 'n' to create new task
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Don't type anything - leave title empty
	// Press Enter to try to save
	ta.SendKeyToFocused(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify: no file should be created (empty title validation)
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Should have no tasks
	tasks := ta.TaskStore.GetAllTasks()
	if len(tasks) > 0 {
		t.Errorf("task with empty title should not be saved, but found: %+v", tasks)
	}

	// Verify no tiki files on disk
	files, _ := filepath.Glob(filepath.Join(ta.TaskDir, "tiki-*.md"))
	if len(files) > 0 {
		t.Errorf("task files should not exist, but found: %v", files)
	}
}

// =============================================================================
// EXISTING TASK EDITING Tests
// =============================================================================

func TestTaskEdit_EnterInPointsFieldDoesNotSave(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task
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
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone) // Open task detail

	// Press 'e' to edit title (starts in title field)
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Change the title
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("Modified Title")

	// Tab to Points field: Title → Status → Type → Priority → Assignee → Points (5 tabs)
	for i := 0; i < 5; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Press Enter while in Points field - should NOT save the task
	ta.SendKeyToFocused(tcell.KeyEnter, 0, tcell.ModNone)

	// Reload from disk and verify title was NOT saved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}
	if task.Title != originalTitle {
		t.Errorf("title was saved when it shouldn't have been: got %q, want %q", task.Title, originalTitle)
	}
}

func TestTaskEdit_TitleChangesSaved(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task
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
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone) // Open task detail

	// Press 'e' to edit title
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Clear existing title and type new one
	// Ctrl+L selects all text in tview, then typing replaces selection
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("Updated Title")

	// Press Enter to save
	ta.SendKeyToFocused(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify: reload from disk and check title changed
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}
	if task.Title != "Updated Title" {
		t.Errorf("title = %q, want %q", task.Title, "Updated Title")
	}
}

// =============================================================================
// PHASE 2: EXISTING TASK SAVE/CANCEL Tests
// =============================================================================

func TestTaskEdit_CtrlS_FromPointsField_Saves(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task
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
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone) // Open task detail

	// Press 'e' to edit title
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Change the title
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("Modified Title")

	// Tab to Points field: Title → Status → Type → Priority → Assignee → Points (5 tabs)
	for i := 0; i < 5; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Press Ctrl+S while in Points field - should save the task
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload from disk and verify title WAS saved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}
	if task.Title != "Modified Title" {
		t.Errorf("title = %q, want %q (Ctrl+S should save from any field)", task.Title, "Modified Title")
	}
}

func TestTaskEdit_Escape_FromTitleField_Cancels(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task
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
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone) // Open task detail

	// Press 'e' to edit title
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Change the title
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("Modified Title")

	// Press Escape to cancel - should discard changes
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Reload from disk and verify title was NOT saved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}
	if task.Title != originalTitle {
		t.Errorf("title = %q, want %q (Escape should cancel)", task.Title, originalTitle)
	}
}

func TestTaskEdit_Escape_ClearsEditSessionState(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task
	taskID := "TIKI-1"
	originalTitle := "Original Title"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, originalTitle, taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate: Board → Task Detail → Task Edit
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone) // Open task detail
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Make sure an edit session is actually started (coordinator prepares on first input event in edit view)
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("Modified Title")
	if ta.EditingTask() == nil {
		t.Fatalf("expected editing task to be non-nil after starting edit session")
	}

	// Press Escape to cancel - should discard changes and clear session state.
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	if ta.EditingTask() != nil {
		t.Fatalf("expected editing task to be nil after cancel, got %+v", ta.EditingTask())
	}
	if ta.DraftTask() != nil {
		t.Fatalf("expected draft task to be nil after cancel, got %+v", ta.DraftTask())
	}
}

func TestTaskEdit_Escape_FromPointsField_Cancels(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task
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
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone) // Open task detail

	// Press 'e' to edit title
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Change the title
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("Modified Title")

	// Tab to Points field: Title → Status → Type → Priority → Assignee → Points (5 tabs)
	for i := 0; i < 5; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Press Escape while in Points field - should discard all changes
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Reload from disk and verify title was NOT saved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}
	if task.Title != originalTitle {
		t.Errorf("title = %q, want %q (Escape should cancel from any field)", task.Title, originalTitle)
	}
}

// =============================================================================
// PHASE 3: FIELD NAVIGATION Tests
// =============================================================================

func TestTaskEdit_Tab_NavigatesForward(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task
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
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone) // Open task detail

	// Press 'e' to edit title (starts in title field)
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Type in title field
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("Title Text")

	// Tab should move to Status field
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)

	// Tab again should move to Type field
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)

	// Tab again should move to Priority field
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)

	// Tab again should move to Assignee field
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)

	// Tab again should move to Points field
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)

	// Set points to 5 (default is 1, so press down 4 times)
	for i := 0; i < 4; i++ {
		ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone)
	}

	// Save with Ctrl+S
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload and verify Points was set
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}
	if task.Points != 5 {
		t.Errorf("points = %d, want 5 (Tab should navigate to Points field)", task.Points)
	}
}

func TestTaskEdit_Navigation_PreservesChanges(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Original Title", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate: Board → Task Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone) // Open task detail

	// Press 'e' to edit title
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Change title
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("New Title")

	// Tab to Points field (5 tabs)
	for i := 0; i < 5; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Set points to 8 (default is 1, so press down 7 times)
	for i := 0; i < 7; i++ {
		ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone)
	}

	// Save with Ctrl+S
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload and verify both title and points were saved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}
	if task.Title != "New Title" {
		t.Errorf("title = %q, want %q (changes should be preserved during navigation)", task.Title, "New Title")
	}
	if task.Points != 8 {
		t.Errorf("points = %d, want 8 (changes should be preserved during navigation)", task.Points)
	}
}

// =============================================================================
// PHASE 4: MULTI-FIELD OPERATIONS Tests
// =============================================================================

func TestTaskEdit_MultipleFields_AllSaved(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task with initial values
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Original Title", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate: Board → Task Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone) // Open task detail

	// Press 'e' to edit
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Change title
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("New Multi-Field Title")

	// Tab to Priority field (3 tabs: Status, Type, Priority)
	for i := 0; i < 3; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	// Set priority to 5 (fixture has 3, so press down 2 times: 3->4->5)
	for i := 0; i < 2; i++ {
		ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone)
	}

	// Tab to Points field (2 more tabs: Assignee, Points)
	for i := 0; i < 2; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	// Set points to 8 (default is 1, so press down 7 times)
	for i := 0; i < 7; i++ {
		ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone)
	}

	// Save with Ctrl+S
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload and verify all changes were saved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}
	if task.Title != "New Multi-Field Title" {
		t.Errorf("title = %q, want %q", task.Title, "New Multi-Field Title")
	}
	if task.Priority != 5 {
		t.Errorf("priority = %d, want 5", task.Priority)
	}
	if task.Points != 8 {
		t.Errorf("points = %d, want 8", task.Points)
	}
}

func TestTaskEdit_MultipleFields_AllDiscarded(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task with initial values
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Original Title", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	// Set initial priority and points
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found after creation")
	}
	task.Priority = 3
	task.Points = 5
	_ = ta.TaskStore.UpdateTask(task)
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate: Board → Task Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone) // Open task detail

	// Press 'e' to edit
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Change title
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("Modified Title")

	// Tab to Priority field and change it
	for i := 0; i < 3; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	// Change priority (arrow keys - doesn't matter since we're testing discard)
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Tab to Points field and change it
	for i := 0; i < 2; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	// Change points (arrow keys - doesn't matter since we're testing discard)
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Press Escape to cancel - all changes should be discarded
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Reload and verify NO changes were saved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task = ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}
	if task.Title != "Original Title" {
		t.Errorf("title = %q, want %q (all changes should be discarded)", task.Title, "Original Title")
	}
	if task.Priority != 3 {
		t.Errorf("priority = %d, want 3 (all changes should be discarded)", task.Priority)
	}
	if task.Points != 5 {
		t.Errorf("points = %d, want 5 (all changes should be discarded)", task.Points)
	}
}

func TestNewTask_MultipleFields_AllSaved(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Start on board view
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Press 'n' to create new task
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Type title
	ta.SendText("New Task With Multiple Fields")

	// Tab to Priority field (3 tabs)
	for i := 0; i < 3; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	// Set priority to 4 (default is 3 from new.md template, so press down 1 time)
	ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone)

	// Tab to Points field (2 more tabs)
	for i := 0; i < 2; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	// Set points to 9 (default is 1 from new.md template, so press down 8 times)
	for i := 0; i < 8; i++ {
		ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone)
	}

	// Save with Ctrl+S
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Verify: file should be created with all fields
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := findTaskByTitle(ta.TaskStore.GetAllTasks(), "New Task With Multiple Fields")
	if task == nil {
		t.Fatalf("new task not found in store")
	}
	if task.Title != "New Task With Multiple Fields" {
		t.Errorf("title = %q, want %q", task.Title, "New Task With Multiple Fields")
	}
	if task.Priority != 4 {
		t.Errorf("priority = %d, want 4", task.Priority)
	}
	if task.Points != 9 {
		t.Errorf("points = %d, want 9", task.Points)
	}
}

// =============================================================================
// REGRESSION TESTS
// =============================================================================

func TestNewTask_AfterEditingExistingTask_StatusAndTypeNotCorrupted(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create and edit an existing task first
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Existing Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate to board and edit the existing task
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)  // Open task detail
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone) // Start editing

	// Make a change and save
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("Edited Existing Task")
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Now press Escape to go back to board
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Press 'n' to create a new task
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Type title - this should NOT corrupt status/type
	ta.SendText("New Task After Edit")

	// Save the new task
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Verify: new task should have default status (backlog) and type (story)
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	newTask := findTaskByTitle(ta.TaskStore.GetAllTasks(), "New Task After Edit")
	if newTask == nil {
		t.Fatalf("new task not found in store")
	}
	if newTask.Title != "New Task After Edit" {
		t.Errorf("title = %q, want %q", newTask.Title, "New Task After Edit")
	}
	// Check status and type are not corrupted
	if newTask.Status != taskpkg.StatusBacklog {
		t.Errorf("status = %v, want %v (status should not be corrupted)", newTask.Status, taskpkg.StatusBacklog)
	}
	if newTask.Type != taskpkg.TypeStory {
		t.Errorf("type = %v, want %v (type should not be corrupted)", newTask.Type, taskpkg.TypeStory)
	}
}

func TestNewTask_WithStatusAndType_Saves(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Start on board view
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Press 'n' to create new task
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Set title
	ta.SendText("Hey")

	// Tab to Status field (1 tab)
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)

	// Cycle status to Review (press down arrow several times)
	// Status order: Backlog -> Ready -> In Progress -> Review -> Done
	for i := 0; i < 3; i++ {
		ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)
	}

	// Tab to Type field (1 tab)
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)

	// Cycle type to Bug (press down arrow once)
	// Type order: Story -> Bug
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Save with Ctrl+S
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Verify: file should be created
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	task := findTaskByTitle(ta.TaskStore.GetAllTasks(), "Hey")
	if task == nil {
		t.Fatalf("new task not found in store")
	}

	t.Logf("Task found: Title=%q, Status=%v, Type=%v", task.Title, task.Status, task.Type)

	if task.Title != "Hey" {
		t.Errorf("title = %q, want %q", task.Title, "Hey")
	}
	if task.Status != taskpkg.StatusReview {
		t.Errorf("status = %v, want %v", task.Status, taskpkg.StatusReview)
	}
	if task.Type != taskpkg.TypeBug {
		t.Errorf("type = %v, want %v", task.Type, taskpkg.TypeBug)
	}
}
