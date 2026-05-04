package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	"github.com/gdamore/tcell/v2"
)

// findTaskByTitle finds a tiki by its title in a slice of tikis
func findTaskByTitle(tikis []*tikipkg.Tiki, title string) *tikipkg.Tiki {
	for _, tk := range tikis {
		if tk.Title == title {
			return tk
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
	task := findTaskByTitle(ta.TaskStore.GetAllTikis(), "My New Task")
	if task == nil {
		t.Fatalf("new task not found in store")
		return
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
	task := findTaskByTitle(ta.TaskStore.GetAllTikis(), "Task To Discard")
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

	// Tab to another field (Points): Title → Status → Type → Priority → Points (4 tabs)
	for i := 0; i < 4; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Press Ctrl+S to save
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Verify: file should be created
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	task := findTaskByTitle(ta.TaskStore.GetAllTikis(), "Task Saved With CtrlS")
	if task == nil {
		t.Fatalf("new task not found in store")
		return
	}
	if task.Title != "Task Saved With CtrlS" {
		t.Errorf("title = %q, want %q", task.Title, "Task Saved With CtrlS")
	}
}

func TestEditSource_DuplicateCaseIDs_Repro(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task with lowercase suffix ID directly in the store.
	taskID := "6EQDUE"
	tk := tikipkg.New()
	tk.ID = taskID
	tk.Title = "Edit Source Duplicate"
	tk.Set("type", string(taskpkg.TypeStory))
	tk.Set("status", string(taskpkg.StatusBacklog))
	tk.Set("priority", 3)
	tk.Set("points", 1)
	if err := ta.TaskStore.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki failed: %v", err)
	}
	if tk.ID != "6EQDUE" {
		t.Fatalf("expected task ID to be normalized, got %q", tk.ID)
	}

	// Mock editor to modify the task file and return immediately.
	ta.NavController.SetEditorOpener(func(path string) error {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content = append(content, '\n')
		return os.WriteFile(path, content, 0644) //nolint:gosec // G703: path is a controlled temp file provided by the app
	})

	// Open task detail view directly.
	ta.NavController.PushView(model.TaskDetailViewID, model.EncodeTaskDetailParams(model.TaskDetailParams{
		TaskID: taskID,
	}))
	ta.Draw()

	// Trigger "Edit source" (key 's') which reloads the task after editor returns.
	ta.SendKey(tcell.KeyRune, 's', tcell.ModNone)

	// Expect a single task in store (no case-duplicate).
	tasks := ta.TaskStore.GetAllTikis()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task after edit source, got %d", len(tasks))
	}

	foundUpper := false
	for _, tsk := range tasks {
		switch tsk.ID {
		case "6EQDUE":
			foundUpper = true
		}
	}
	if !foundUpper {
		t.Fatalf("expected uppercase ID variant, foundUpper=%v", foundUpper)
	}

	// Ensure file path is the lowercased ID (edit source uses this file).
	taskFilePath := filepath.Join(ta.TaskDir, strings.ToLower(taskID)+".md")
	if _, err := os.Stat(taskFilePath); os.IsNotExist(err) {
		t.Fatalf("expected task file to exist at %s", taskFilePath)
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
	tasks := ta.TaskStore.GetAllTikis()
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
	taskID := "000001"
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

	// Tab to Points field: Title → Status → Type → Priority → Points (4 tabs)
	for i := 0; i < 4; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Press Enter while in Points field - should NOT save the task
	ta.SendKeyToFocused(tcell.KeyEnter, 0, tcell.ModNone)

	// Reload from disk and verify title was NOT saved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTiki(taskID)
	if task == nil {
		t.Fatalf("task not found")
		return
	}
	if task.Title != originalTitle {
		t.Errorf("title was saved when it shouldn't have been: got %q, want %q", task.Title, originalTitle)
	}
}

func TestTaskEdit_TitleChangesSaved(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task
	taskID := "000001"
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
	task := ta.TaskStore.GetTiki(taskID)
	if task == nil {
		t.Fatalf("task not found")
		return
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
	taskID := "000001"
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

	// Tab to Points field: Title → Status → Type → Priority → Points (4 tabs)
	for i := 0; i < 4; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Press Ctrl+S while in Points field - should save the task
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload from disk and verify title WAS saved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTiki(taskID)
	if task == nil {
		t.Fatalf("task not found")
		return
	}
	if task.Title != "Modified Title" {
		t.Errorf("title = %q, want %q (Ctrl+S should save from any field)", task.Title, "Modified Title")
	}
}

func TestTaskEdit_Escape_FromTitleField_Cancels(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task
	taskID := "000001"
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
	task := ta.TaskStore.GetTiki(taskID)
	if task == nil {
		t.Fatalf("task not found")
		return
	}
	if task.Title != originalTitle {
		t.Errorf("title = %q, want %q (Escape should cancel)", task.Title, originalTitle)
	}
}

func TestTaskEdit_Escape_ClearsEditSessionState(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task
	taskID := "000001"
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
	if ta.EditingTiki() == nil {
		t.Fatalf("expected editing task to be non-nil after starting edit session")
	}

	// Press Escape to cancel - should discard changes and clear session state.
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	if ta.EditingTiki() != nil {
		t.Fatalf("expected editing task to be nil after cancel, got %+v", ta.EditingTiki())
	}
	if ta.DraftTiki() != nil {
		t.Fatalf("expected draft task to be nil after cancel, got %+v", ta.DraftTiki())
	}
}

func TestTaskEdit_Escape_FromPointsField_Cancels(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task
	taskID := "000001"
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

	// Tab to Points field: Title → Status → Type → Priority → Points (4 tabs)
	for i := 0; i < 4; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Press Escape while in Points field - should discard all changes
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Reload from disk and verify title was NOT saved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	task := ta.TaskStore.GetTiki(taskID)
	if task == nil {
		t.Fatalf("task not found")
		return
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
	taskID := "000001"
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

	// Tab again should move to Points field
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)

	// Set points to 5 (default is 1, so press up 4 times)
	for i := 0; i < 4; i++ {
		ta.SendKeyToFocused(tcell.KeyUp, 0, tcell.ModNone)
	}

	// Save with Ctrl+S
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload and verify Points was set
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	tk := ta.TaskStore.GetTiki(taskID)
	if tk == nil {
		t.Fatalf("task not found")
		return
	}
	points, _, _ := tk.IntField("points")
	if points != 5 {
		t.Errorf("points = %d, want 5 (Tab should navigate to Points field)", points)
	}
}

func TestTaskEdit_Navigation_PreservesChanges(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task
	taskID := "000001"
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

	// Tab to Points field (4 tabs)
	for i := 0; i < 4; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Set points to 8 (default is 1, so press up 7 times)
	for i := 0; i < 7; i++ {
		ta.SendKeyToFocused(tcell.KeyUp, 0, tcell.ModNone)
	}

	// Save with Ctrl+S
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload and verify both title and points were saved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	tk := ta.TaskStore.GetTiki(taskID)
	if tk == nil {
		t.Fatalf("task not found")
		return
	}
	if tk.Title != "New Title" {
		t.Errorf("title = %q, want %q (changes should be preserved during navigation)", tk.Title, "New Title")
	}
	points, _, _ := tk.IntField("points")
	if points != 8 {
		t.Errorf("points = %d, want 8 (changes should be preserved during navigation)", points)
	}
}

// =============================================================================
// PHASE 4: MULTI-FIELD OPERATIONS Tests
// =============================================================================

func TestTaskEdit_MultipleFields_AllSaved(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task with initial values
	taskID := "000001"
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

	// Tab to Points field (1 more tab: Priority → Points)
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	// Set points to 8 (default is 1, so press up 7 times)
	for i := 0; i < 7; i++ {
		ta.SendKeyToFocused(tcell.KeyUp, 0, tcell.ModNone)
	}

	// Save with Ctrl+S
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload and verify all changes were saved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	tk := ta.TaskStore.GetTiki(taskID)
	if tk == nil {
		t.Fatalf("task not found")
		return
	}
	if tk.Title != "New Multi-Field Title" {
		t.Errorf("title = %q, want %q", tk.Title, "New Multi-Field Title")
	}
	priority, _, _ := tk.IntField("priority")
	if priority != 5 {
		t.Errorf("priority = %d, want 5", priority)
	}
	points, _, _ := tk.IntField("points")
	if points != 8 {
		t.Errorf("points = %d, want 8", points)
	}
}

func TestTaskEdit_MultipleFields_AllDiscarded(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a task with initial values
	taskID := "000001"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Original Title", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	// Set initial priority and points
	initTk := ta.TaskStore.GetTiki(taskID)
	if initTk == nil {
		t.Fatalf("task not found after creation")
		return
	}
	updTk := initTk.Clone()
	updTk.Set("priority", 3)
	updTk.Set("points", 5)
	_ = ta.TaskStore.UpdateTiki(updTk)
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

	// Tab to Points field and change it (1 tab: Priority → Points)
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	// Change points (arrow keys - doesn't matter since we're testing discard)
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Press Escape to cancel - all changes should be discarded
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Reload and verify NO changes were saved
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	reloadedTk := ta.TaskStore.GetTiki(taskID)
	if reloadedTk == nil {
		t.Fatalf("task not found")
		return
	}
	if reloadedTk.Title != "Original Title" {
		t.Errorf("title = %q, want %q (all changes should be discarded)", reloadedTk.Title, "Original Title")
	}
	reloadedPriority, _, _ := reloadedTk.IntField("priority")
	if reloadedPriority != 3 {
		t.Errorf("priority = %d, want 3 (all changes should be discarded)", reloadedPriority)
	}
	reloadedPoints, _, _ := reloadedTk.IntField("points")
	if reloadedPoints != 5 {
		t.Errorf("points = %d, want 5 (all changes should be discarded)", reloadedPoints)
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
	// set priority to 4 (default is 3, so press down 1 time)
	ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone)

	// Tab to Points field (1 more tab: Priority → Points)
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	// set points to 9 (default is 1, so press up 8 times)
	for i := 0; i < 8; i++ {
		ta.SendKeyToFocused(tcell.KeyUp, 0, tcell.ModNone)
	}

	// Save with Ctrl+S
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Verify: file should be created with all fields
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	tk := findTaskByTitle(ta.TaskStore.GetAllTikis(), "New Task With Multiple Fields")
	if tk == nil {
		t.Fatalf("new task not found in store")
		return
	}
	if tk.Title != "New Task With Multiple Fields" {
		t.Errorf("title = %q, want %q", tk.Title, "New Task With Multiple Fields")
	}
	priority, _, _ := tk.IntField("priority")
	if priority != 4 {
		t.Errorf("priority = %d, want 4", priority)
	}
	points, _, _ := tk.IntField("points")
	if points != 9 {
		t.Errorf("points = %d, want 9", points)
	}
}

// =============================================================================
// REGRESSION TESTS
// =============================================================================

func TestNewTask_AfterEditingExistingTask_StatusAndTypeNotCorrupted(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create and edit an existing task first
	taskID := "000001"
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
	newTk := findTaskByTitle(ta.TaskStore.GetAllTikis(), "New Task After Edit")
	if newTk == nil {
		t.Fatalf("new task not found in store")
		return
	}
	if newTk.Title != "New Task After Edit" {
		t.Errorf("title = %q, want %q", newTk.Title, "New Task After Edit")
	}
	// Check status and type are not corrupted
	newStatus, _, _ := newTk.StringField("status")
	if newStatus != string(taskpkg.StatusBacklog) {
		t.Errorf("status = %v, want %v (status should not be corrupted)", newStatus, taskpkg.StatusBacklog)
	}
	newType, _, _ := newTk.StringField("type")
	if newType != string(taskpkg.TypeStory) {
		t.Errorf("type = %v, want %v (type should not be corrupted)", newType, taskpkg.TypeStory)
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

	tk := findTaskByTitle(ta.TaskStore.GetAllTikis(), "Hey")
	if tk == nil {
		t.Fatalf("new task not found in store")
		return
	}

	heyStatus, _, _ := tk.StringField("status")
	heyType, _, _ := tk.StringField("type")
	t.Logf("Task found: Title=%q, Status=%v, Type=%v", tk.Title, heyStatus, heyType)

	if tk.Title != "Hey" {
		t.Errorf("title = %q, want %q", tk.Title, "Hey")
	}
	if heyStatus != string(taskpkg.StatusReview) {
		t.Errorf("status = %v, want %v", heyStatus, taskpkg.StatusReview)
	}
	if heyType != string(taskpkg.TypeBug) {
		t.Errorf("type = %v, want %v", heyType, taskpkg.TypeBug)
	}
}
