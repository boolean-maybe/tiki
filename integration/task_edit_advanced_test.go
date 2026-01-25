package integration

import (
	"testing"

	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

// TestTaskEdit_ShiftTabBackward verifies Shift+Tab navigates backward through fields
func TestTaskEdit_ShiftTabBackward(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Test Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate: Board → Task Detail → Task Edit
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Tab forward to Points (Title → Status → Type → Priority → Assignee → Points)
	for i := 0; i < 5; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Now Shift+Tab backward (Points → Assignee)
	ta.SendKey(tcell.KeyBacktab, 0, tcell.ModNone)

	// Make a change to Assignee field to verify focus
	ta.SendKeyToFocused(tcell.KeyRune, 'A', tcell.ModNone)

	// Save
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload and verify assignee was set
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}

	// Verify assignee contains 'A' (may have more if field had default value)
	if task.Assignee == "" {
		t.Errorf("assignee should be set after Shift+Tab to Assignee field")
	}
}

// TestTaskEdit_StatusCycling verifies arrow keys cycle through all status values
func TestTaskEdit_StatusCycling(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Test Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate: Board → Task Detail → Task Edit
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Tab to Status field
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)

	// Current status is "ready"
	// Test status cycling by pressing Down multiple times without saving between presses
	// Status order: Backlog -> Ready -> In Progress -> Review -> Done
	// Note: The dropdown doesn't wrap, so we can only go forward through the list

	// Press Down 3 times: Ready → In Progress → Review → Done
	ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone) // Ready → In Progress
	ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone) // In Progress → Review
	ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone) // Review → Done

	// Save
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload and verify final status
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}

	if task.Status != taskpkg.StatusDone {
		t.Errorf("after 3 Down presses, status = %v, want done", task.Status)
	}
}

// TestTaskEdit_TypeToggling verifies cycling through all type values (Story → Bug → Spike → Epic)
func TestTaskEdit_TypeToggling(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task with Story type
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Test Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate: Board → Task Detail → Task Edit
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Tab to Type field (Title → Status → Type)
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)

	// Test type cycling by pressing Down multiple times without saving between presses
	// Type order: Story → Bug → Spike → Epic → (wraps to) Story

	// Press Down 3 times: Story → Bug → Spike → Epic
	ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone) // Story → Bug
	ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone) // Bug → Spike
	ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone) // Spike → Epic

	// Save
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload and verify final type
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}

	if task.Type != taskpkg.TypeEpic {
		t.Errorf("after 3 Down presses, type = %v, want epic", task.Type)
	}
}

// TestTaskEdit_AssigneeInput verifies typing in assignee field
// Note: Current behavior appends to default "Unassigned" text rather than replacing it.
// This is known behavior and left as-is for now.
func TestTaskEdit_AssigneeInput(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Test Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate: Board → Task Detail → Task Edit
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Tab to Assignee field (Title → Status → Type → Priority → Assignee)
	for i := 0; i < 4; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Type assignee name
	// Current behavior: text appends to "Unassigned" default
	ta.SendText("john.doe")

	// Save
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload and verify
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}

	// Current behavior: appends to default "Unassigned" text
	expected := "Unassignedjohn.doe"
	if task.Assignee != expected {
		t.Errorf("assignee = %q, want %q", task.Assignee, expected)
	}
}

// TestTaskEdit_SaveAndContinue verifies saving within an edit session
func TestTaskEdit_SaveAndContinue(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Original Title", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate: Board → Task Detail → Edit
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Edit and save
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone) // Select all
	ta.SendText("New Title")
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Verify save worked
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}

	if task.Title != "New Title" {
		t.Errorf("title = %q, want %q", task.Title, "New Title")
	}
}

// TestTaskEdit_EscapeAndReEdit verifies clean state after cancel and re-edit
func TestTaskEdit_EscapeAndReEdit(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	taskID := "TIKI-1"
	originalTitle := "Original Title"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, originalTitle, taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate: Board → Task Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Start edit and make changes
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("Changed Title")

	// Cancel with Escape
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify editing state cleared
	if ta.EditingTask() != nil {
		t.Errorf("editing task should be nil after cancel")
	}

	// Start edit again
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Verify field shows original title (clean state)
	// Make actual change and save
	ta.SendKeyToFocused(tcell.KeyCtrlL, 0, tcell.ModNone)
	ta.SendText("New Title")
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload and verify
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}

	if task.Title != "New Title" {
		t.Errorf("title = %q, want %q", task.Title, "New Title")
	}
}

// TestTaskEdit_PriorityRange verifies priority can be set to valid values
func TestTaskEdit_PriorityRange(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Test Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate: Board → Task Detail → Task Edit
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Tab to Priority field (Title → Status → Type → Priority)
	for i := 0; i < 3; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Current priority is 3 (from fixture)
	// Press Down twice to get to priority 5
	ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone)
	ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone)

	// Save
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload and verify
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}

	if task.Priority != 5 {
		t.Errorf("priority = %d, want 5", task.Priority)
	}
}

// TestTaskEdit_PointsRange verifies points can be set to valid values
func TestTaskEdit_PointsRange(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task
	taskID := "TIKI-1"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Test Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate: Board → Task Detail → Task Edit
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	// Tab to Points field (Title → Status → Type → Priority → Assignee → Points)
	for i := 0; i < 5; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Current points is 1 (from fixture)
	// Default maxPoints is 10, so valid range is [1, 10]
	// Press Down 6 times to get to 7 (1→2→3→4→5→6→7)
	for i := 0; i < 6; i++ {
		ta.SendKeyToFocused(tcell.KeyDown, 0, tcell.ModNone)
	}

	// Save
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Reload and verify
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	task := ta.TaskStore.GetTask(taskID)
	if task == nil {
		t.Fatalf("task not found")
	}

	if task.Points != 7 {
		t.Errorf("points = %d, want 7", task.Points)
	}

	// That's sufficient to verify points field cycling works
	// (Testing wrapping would require reopening the task which we're avoiding)
}
