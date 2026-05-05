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

	// Phase 3: open the configurable detail view directly. The Edit
	// source ('s') keybinding now lives on the configurable detail
	// view's registry; the legacy TaskDetailViewID is no longer the
	// host for the edit-source flow.
	ta.NavController.PushView(
		model.DetailPluginViewID(),
		model.EncodePluginViewParams(model.PluginViewParams{TaskID: taskID}),
	)
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
//
// Phase 3 cleanup: TestTaskEdit_EnterInPointsFieldDoesNotSave and
// TestTaskEdit_TitleChangesSaved were removed. Both opened the legacy
// task edit view via Enter → 'e' and asserted "Enter saves title" /
// "Enter in Points field is a no-op". After Phase 2 the configurable
// detail view's in-place edit mode owns those keystrokes; coverage
// lives in view/taskdetail/configurable_detail_edit_test.go.

// =============================================================================
// PHASE 2: EXISTING TASK SAVE/CANCEL Tests
// =============================================================================
//
// Phase 3 cleanup: TestTaskEdit_CtrlS_FromPointsField_Saves,
// TestTaskEdit_Escape_FromTitleField_Cancels,
// TestTaskEdit_Escape_ClearsEditSessionState, and
// TestTaskEdit_Escape_FromPointsField_Cancels were removed. They
// asserted Ctrl+S/Escape semantics on the legacy 6-field TaskEditView
// reached via Enter → 'e'. Edit mode is now in-place on the
// configurable detail view; equivalent save/cancel coverage lives in
// view/taskdetail/configurable_detail_edit_test.go.

// =============================================================================
// PHASE 3: FIELD NAVIGATION Tests
// =============================================================================
//
// Phase 3 cleanup: TestTaskEdit_Tab_NavigatesForward and
// TestTaskEdit_Navigation_PreservesChanges were removed. Both tested
// Tab traversal across the legacy TaskEditView's 6 fields (Title,
// Status, Type, Priority, Points, Assignee). The configurable detail
// view's edit mode traverses only the workflow-declared metadata
// (default [status, type, priority]); coverage of the new traversal
// lives in view/taskdetail/configurable_detail_edit_test.go.

// =============================================================================
// PHASE 4: MULTI-FIELD OPERATIONS Tests
// =============================================================================
//
// Phase 3 cleanup: TestTaskEdit_MultipleFields_AllSaved and
// TestTaskEdit_MultipleFields_AllDiscarded were removed. They tested
// the legacy 5-field TaskEditView (title/priority/points). The
// configurable detail view's edit mode operates on the workflow's
// declared metadata only (default [status, type, priority]); see
// view/taskdetail/configurable_detail_edit_test.go for save/discard
// coverage of the new path.

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

// TestNewTask_AfterEditingExistingTask_StatusAndTypeNotCorrupted
// Phase 3 cleanup: the original test mixed the legacy "Enter → 'e' →
// Ctrl+S" edit-existing flow (now in-place on the configurable detail
// view) with the surviving 'n' new-task draft flow. The regression it
// guarded against — TaskController state from a prior edit session
// leaking into the next draft — is now covered at the unit level by
// the TaskController edit-session tests in controller/task_detail_test.go,
// which directly exercise the StartEditSession/CommitEditSession/
// ClearDraft state machine without relying on TUI keystroke routing.

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
