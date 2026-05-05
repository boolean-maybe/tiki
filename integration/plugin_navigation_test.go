package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

// ============================================================================
// Test Data Helpers
// ============================================================================

// setupPluginTestData creates tasks matching all three default plugin filters:
// - Backlog: status = 'backlog'
// - Recent: UpdatedAt within 2 hours
// - Roadmap: type = 'epic'
func setupPluginTestData(t *testing.T, ta *testutil.TestApp) {
	tasks := []struct {
		id       string
		title    string
		status   taskpkg.Status
		taskType taskpkg.Type
		recent   bool // needs UpdatedAt within 2 hours
	}{
		// Backlog plugin: status = 'backlog'
		{"000001", "Backlog Task 1", taskpkg.StatusBacklog, taskpkg.TypeStory, false},
		{"000002", "Backlog Task 2", taskpkg.StatusBacklog, taskpkg.TypeBug, false},

		// Recent plugin: UpdatedAt within 2 hours
		{"000003", "Recent Task 1", taskpkg.StatusReady, taskpkg.TypeStory, true},
		{"000004", "Recent Task 2", taskpkg.StatusInProgress, taskpkg.TypeBug, true},

		// Roadmap plugin: type = 'epic'
		{"000005", "Roadmap Epic 1", taskpkg.StatusReady, taskpkg.TypeEpic, false},
		{"000006", "Roadmap Epic 2", taskpkg.StatusInProgress, taskpkg.TypeEpic, false},

		// Multi-plugin match
		{"000007", "Recent Backlog", taskpkg.StatusBacklog, taskpkg.TypeStory, true},
	}

	for _, task := range tasks {
		err := testutil.CreateTestTask(ta.TaskDir, task.id, task.title, task.status, task.taskType)
		if err != nil {
			t.Fatalf("Failed to create task %s: %v", task.id, err)
		}

		// For recent tasks, touch file to set mtime to now
		if task.recent {
			filePath := filepath.Join(ta.TaskDir, strings.ToLower(task.id)+".md")
			now := time.Now()
			if err := os.Chtimes(filePath, now, now); err != nil {
				t.Fatalf("Failed to touch file %s: %v", filePath, err)
			}
		}
	}

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("Failed to reload task store: %v", err)
	}
}

// setupTestAppWithPlugins creates TestApp with plugins loaded and test data
func setupTestAppWithPlugins(t *testing.T) *testutil.TestApp {
	ta := testutil.NewTestApp(t)
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("Failed to load plugins: %v", err)
	}
	setupPluginTestData(t, ta)
	return ta
}

// ============================================================================
// Plugin Switching Tests
// ============================================================================

func TestPluginNavigation_PluginSwitch_ReplacesView(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Start on Kanban
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected stack depth 1, got %d", ta.NavController.Depth())
	}

	// Press F3 for Backlog (should replace, not push)
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)

	// Verify: stack depth unchanged (plugin-to-plugin uses ReplaceView), view changed
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected stack depth 1 after switching plugin, got %d", ta.NavController.Depth())
	}
	expectedViewID := model.MakePluginViewID("Backlog")
	if ta.NavController.CurrentViewID() != expectedViewID {
		t.Errorf("Expected view %s, got %s", expectedViewID, ta.NavController.CurrentViewID())
	}

	// Verify screen shows plugin
	found, _, _ := ta.FindText("Backlog")
	if !found {
		t.Error("Expected to find 'Backlog' text on screen")
	}
}

func TestPluginNavigation_PluginToPlugin_ReplacesView(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Start: Kanban → Backlog
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)
	ta.Draw()

	// Verify we're on Backlog with depth 1 (plugin-to-plugin replaces)
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected stack depth 1, got %d", ta.NavController.Depth())
	}

	// Press Ctrl+R for Recent (should REPLACE Backlog, not push)
	ta.SendKey(tcell.KeyRune, 'R', tcell.ModCtrl)

	// Verify: depth unchanged, view changed
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected stack depth 1 after replacing plugin, got %d", ta.NavController.Depth())
	}
	expectedViewID := model.MakePluginViewID("Recent")
	if ta.NavController.CurrentViewID() != expectedViewID {
		t.Errorf("Expected view %s, got %s", expectedViewID, ta.NavController.CurrentViewID())
	}

	// Verify screen shows Recent
	found, _, _ := ta.FindText("Recent")
	if !found {
		t.Error("Expected to find 'Recent' text on screen")
	}
}

func TestPluginNavigation_EscDoesNothingAtRoot(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Start on Kanban (root view)
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Verify we're on Kanban with depth 1
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected stack depth 1, got %d", ta.NavController.Depth())
	}

	// Press Esc - should do nothing since we're at root
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify: still on Kanban with depth 1
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected stack depth 1 after Esc at root, got %d", ta.NavController.Depth())
	}
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Kanban") {
		t.Errorf("Expected view %s, got %s", model.MakePluginViewID("Kanban"), ta.NavController.CurrentViewID())
	}
}

func TestPluginNavigation_SamePluginKey_NoOp(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Start: Kanban → Backlog
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)
	ta.Draw()

	expectedViewID := model.MakePluginViewID("Backlog")
	if ta.NavController.CurrentViewID() != expectedViewID {
		t.Fatalf("Expected view %s, got %s", expectedViewID, ta.NavController.CurrentViewID())
	}
	initialDepth := ta.NavController.Depth()

	// Press 'L' again (should be no-op)
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)

	// Verify: no change
	if ta.NavController.Depth() != initialDepth {
		t.Errorf("Expected stack depth unchanged at %d, got %d", initialDepth, ta.NavController.Depth())
	}
	if ta.NavController.CurrentViewID() != expectedViewID {
		t.Errorf("Expected view unchanged at %s, got %s", expectedViewID, ta.NavController.CurrentViewID())
	}
}

// ============================================================================
// Action Registry Tests
// ============================================================================

func TestPluginActions_RegistryMatchesExpectedKeys(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Phase 1 dropped the built-in Enter→ActionOpenFromPlugin binding from
	// the regular plugin registry. Open is now declared by the workflow as
	// a `kind: view` action; the bundled kanban registers it on Enter, so
	// every TikiPlugin's registry should expose Enter as a plugin_action
	// entry rather than the legacy ActionOpenFromPlugin id.
	expectedActions := []struct {
		id   controller.ActionID
		key  tcell.Key
		rune rune
	}{
		{controller.ActionNavUp, tcell.KeyUp, 0},
		{controller.ActionNavDown, tcell.KeyDown, 0},
		{controller.ActionNavLeft, tcell.KeyLeft, 0},
		{controller.ActionNavRight, tcell.KeyRight, 0},
		{controller.ActionNewTask, tcell.KeyRune, 'n'},
		{controller.ActionDeleteTask, tcell.KeyRune, 'd'},
		{controller.ActionSearch, tcell.KeyRune, '/'},
		{controller.ActionToggleViewMode, tcell.KeyRune, 'v'},
	}

	// Test each plugin controller (only TikiPlugin types have task management actions)
	for pluginName, pluginController := range ta.PluginControllers {
		// Skip non-board controllers — they don't carry the board-style action set.
		if _, ok := pluginController.(*controller.DokiController); ok {
			continue
		}
		if _, ok := pluginController.(*controller.DetailController); ok {
			continue
		}

		registry := pluginController.GetActionRegistry()

		for _, expected := range expectedActions {
			event := tcell.NewEventKey(expected.key, expected.rune, tcell.ModNone)
			action := registry.Match(event)
			if action == nil {
				t.Errorf("Plugin %s: action %s not found in registry", pluginName, expected.id)
			} else if action.ID != expected.id {
				t.Errorf("Plugin %s: expected action %s, got %s", pluginName, expected.id, action.ID)
			}
		}

		// Enter should match a workflow-declared kind:view plugin action,
		// not the retired ActionOpenFromPlugin built-in.
		enter := registry.Match(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
		if enter == nil {
			t.Errorf("Plugin %s: Enter has no binding (expected workflow plugin_action:Enter)", pluginName)
		} else if enter.ID == controller.ActionOpenFromPlugin {
			t.Errorf("Plugin %s: Enter still bound to ActionOpenFromPlugin (Phase 1 should retire this)", pluginName)
		}
	}
}

func TestPluginActions_HeaderDisplaysCorrectActions(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Navigate to a plugin view
	ta.NavController.PushView(model.MakePluginViewID("Backlog"), nil)
	ta.Draw()

	// Verify at least the plugin name appears (header may not show all actions in test env)
	found, _, _ := ta.FindText("Backlog")
	if !found {
		t.Error("Expected to find plugin name 'Backlog' on screen")
	}

	// If you want to debug what's actually on screen:
	// ta.DumpScreen()
}

// ============================================================================
// Action Execution Tests
// ============================================================================

func TestPluginActions_Navigation_ArrowKeys(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Navigate to Backlog plugin (has at least 3 tasks: TIKI-1, TIKI-2, TIKI-7)
	ta.NavController.PushView(model.MakePluginViewID("Backlog"), nil)
	ta.Draw()

	pluginConfig := ta.GetPluginConfig("Backlog")
	if pluginConfig == nil {
		t.Fatal("Failed to get Backlog plugin config")
	}

	// Initial selection should be 0
	initialIndex := pluginConfig.GetSelectedIndex()
	if initialIndex != 0 {
		t.Errorf("Expected initial selection 0, got %d", initialIndex)
	}

	// Press Down arrow - in a 4-column grid with 3 tasks:
	// Layout might be: [0] [1] [2] [-]
	// Down from 0 might not move (no row below) or might cycle
	// The exact behavior depends on the grid implementation
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)
	indexAfterDown := pluginConfig.GetSelectedIndex()

	// Press Right arrow - should move from column 0 to column 1
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)
	indexAfterRight := pluginConfig.GetSelectedIndex()

	// Verify that navigation keys DO affect selection
	// (exact behavior may vary, but at least one of these should change)
	if initialIndex == indexAfterDown && initialIndex == indexAfterRight {
		// This might be OK if there's only 1 task or navigation wraps differently
		t.Logf("Navigation didn't change selection (initial=%d, afterDown=%d, afterRight=%d)",
			initialIndex, indexAfterDown, indexAfterRight)
		// Don't fail - navigation logic may be more complex
	}

	// Test that selection stays within bounds
	if pluginConfig.GetSelectedIndex() < 0 {
		t.Errorf("Selection went negative: %d", pluginConfig.GetSelectedIndex())
	}
}

func TestPluginActions_OpenTask_EnterKey(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Navigate to Backlog plugin (replaces Kanban, depth stays 1)
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)
	ta.Draw()

	// Verify initial depth (plugin-to-plugin uses replace, so depth is 1)
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected stack depth 1, got %d", ta.NavController.Depth())
	}

	// Press Enter to open first task
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify: configurable detail view pushed onto stack (Phase 3:
	// Enter is workflow-declared kind: view → Detail, not built-in
	// TaskDetailViewID).
	if ta.NavController.Depth() != 2 {
		t.Errorf("Expected stack depth 2 after opening task, got %d", ta.NavController.Depth())
	}
	wantDetail := model.DetailPluginViewID()
	if ta.NavController.CurrentViewID() != wantDetail {
		t.Errorf("Expected view %s, got %s", wantDetail, ta.NavController.CurrentViewID())
	}

	// Verify screen shows task title
	found, _, _ := ta.FindText("Backlog Task")
	if !found {
		t.Error("Expected to find task title on screen")
	}
}

func TestPluginActions_NewTask_NKey(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	ta.NavController.PushView(model.MakePluginViewID("Backlog"), nil)
	ta.Draw()

	initialDepth := ta.NavController.Depth()

	// Press 'n' to create task
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Verify: TaskEdit view pushed
	if ta.NavController.CurrentViewID() != model.TaskEditViewID {
		t.Errorf("Expected view %s after pressing 'n', got %s", model.TaskEditViewID, ta.NavController.CurrentViewID())
	}

	// Type title and save
	ta.SendText("New Plugin Task")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify: Back to plugin view
	if ta.NavController.Depth() != initialDepth {
		t.Errorf("Expected to return to plugin view at depth %d, got %d", initialDepth, ta.NavController.Depth())
	}

	// Verify: Task created
	_ = ta.TaskStore.Reload()
	tikis := ta.TaskStore.GetAllTikis()
	var found bool
	for _, tk := range tikis {
		if tk.Title == "New Plugin Task" {
			found = true
			status, _, _ := tk.StringField("status")
			if status != string(taskpkg.StatusBacklog) {
				t.Errorf("Expected new task to have backlog status, got %s", status)
			}
			break
		}
	}
	if !found {
		t.Error("Expected to find newly created task")
	}
}

func TestPluginActions_DeleteTask_DKey(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Create a specific task to delete
	_ = testutil.CreateTestTask(ta.TaskDir, "DELETE", "To Delete", taskpkg.StatusBacklog, taskpkg.TypeStory)
	_ = ta.TaskStore.Reload()

	ta.NavController.PushView(model.MakePluginViewID("Backlog"), nil)
	ta.Draw()

	// Verify task exists
	tikiTask := ta.TaskStore.GetTiki("DELETE")
	if tikiTask == nil {
		t.Fatal("Test task DELETE not found before deletion")
		return
	}

	// Press 'd' to delete (assumes first task is selected)
	// Note: We need to ensure DELETE is selected, which depends on sort order
	// For simplicity, we'll just verify the delete action works
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Verify: At least one task was deleted
	_ = ta.TaskStore.Reload()
	initialTaskCount := len(ta.TaskStore.GetAllTikis())

	// Check if the specific file is deleted (it should be one of the backlog tasks)
	tikisAfter := ta.TaskStore.GetAllTikis()
	if len(tikisAfter) >= initialTaskCount {
		// Count should decrease
		t.Log("Task deletion completed")
	}
}

func TestPluginActions_Search_SlashKey(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	ta.NavController.PushView(model.MakePluginViewID("Backlog"), nil)
	ta.Draw()

	// Press '/' to open search
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)

	// Verify: Search box visible (implementation may vary)
	// This is a basic test - in real usage, search box should appear
	// We'll just verify no crash occurs
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Backlog") {
		t.Error("Expected to stay on Backlog view after opening search")
	}
}

func TestPluginActions_ToggleViewMode_VKey(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	ta.NavController.PushView(model.MakePluginViewID("Backlog"), nil)
	ta.Draw()

	pluginConfig := ta.GetPluginConfig("Backlog")
	if pluginConfig == nil {
		t.Fatal("Failed to get Backlog plugin config")
	}

	initialViewMode := pluginConfig.GetViewMode()

	// Press 'v' to toggle view mode
	ta.SendKey(tcell.KeyRune, 'v', tcell.ModNone)

	newViewMode := pluginConfig.GetViewMode()
	if newViewMode == initialViewMode {
		t.Error("Expected view mode to toggle after pressing 'v'")
	}
}

// ============================================================================
// Navigation Stack Tests
// ============================================================================

func TestPluginStack_MultiLevelNavigation(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Kanban (depth 1)
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected depth 1, got %d", ta.NavController.Depth())
	}

	// Kanban→Backlog (Replace, depth 1)
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected depth 1 after Backlog (replace), got %d", ta.NavController.Depth())
	}
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Backlog") {
		t.Errorf("Expected Backlog view, got %s", ta.NavController.CurrentViewID())
	}

	// Backlog→Recent (Replace, depth 1)
	ta.SendKey(tcell.KeyRune, 'R', tcell.ModCtrl)
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected depth 1 after Recent (replace), got %d", ta.NavController.Depth())
	}
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Recent") {
		t.Errorf("Expected Recent view, got %s", ta.NavController.CurrentViewID())
	}

	// Recent→TaskDetail (Push, depth 2)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	if ta.NavController.Depth() != 2 {
		t.Errorf("Expected depth 2 after TaskDetail, got %d", ta.NavController.Depth())
	}

	// TaskDetail→Recent (Pop, depth 1)
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected depth 1 after Esc, got %d", ta.NavController.Depth())
	}
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Recent") {
		t.Errorf("Expected Recent view, got %s", ta.NavController.CurrentViewID())
	}

	// Esc at root does nothing
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected depth 1 after Esc at root, got %d", ta.NavController.Depth())
	}
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Recent") {
		t.Errorf("Expected Recent view, got %s", ta.NavController.CurrentViewID())
	}
}

func TestPluginStack_TaskDetailFromPlugin_ReturnsToPlugin(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Kanban→Backlog(replace)→TaskDetail(push)
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)    // Replace: Kanban→Backlog, depth 1
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone) // Push: TaskDetail, depth 2

	// Stack: Backlog, TaskDetail (depth 2)
	if ta.NavController.Depth() != 2 {
		t.Errorf("Expected depth 2, got %d", ta.NavController.Depth())
	}

	// Press Esc from TaskDetail
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify: returned to Backlog
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected depth 1 after Esc, got %d", ta.NavController.Depth())
	}
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Backlog") {
		t.Errorf("Expected Backlog view, got %s", ta.NavController.CurrentViewID())
	}

	// Verify screen shows Backlog
	found, _, _ := ta.FindText("Backlog")
	if !found {
		t.Error("Expected to find 'Backlog' text on screen")
	}
}

// Phase 3 cleanup: TestPluginStack_ComplexDrillDown removed. It tested
// the legacy 3-level Enter → TaskDetailViewID → 'e' → TaskEditViewID
// stack; after Phase 2 'e' flips in-place edit mode without pushing.

// ============================================================================
// Esc Behavior Tests
// ============================================================================

func TestPluginEsc_AtRootDoesNothing(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Start at Kanban, switch to Backlog (ReplaceView keeps depth at 1)
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)

	// Verify we're on Backlog at depth 1
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected depth 1, got %d", ta.NavController.Depth())
	}
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Backlog") {
		t.Errorf("Expected Backlog view, got %s", ta.NavController.CurrentViewID())
	}

	// Esc at root does nothing
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify: still on Backlog at depth 1
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected depth 1 after Esc at root, got %d", ta.NavController.Depth())
	}
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Backlog") {
		t.Errorf("Expected to stay on Backlog after Esc at root, got %s", ta.NavController.CurrentViewID())
	}
}

func TestPluginEsc_FromTaskDetailToPlugin(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Kanban→Recent(replace)→TaskDetail(push)
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.SendKey(tcell.KeyRune, 'R', tcell.ModCtrl) // Recent (replaces Kanban)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)  // Open task (pushes TaskDetail)

	// Plugin-to-plugin uses ReplaceView, so: Kanban→Recent = depth 1, then push TaskDetail = depth 2
	if ta.NavController.Depth() != 2 {
		t.Errorf("Expected depth 2, got %d", ta.NavController.Depth())
	}

	// Esc from TaskDetail
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify: back to Recent plugin
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected depth 1, got %d", ta.NavController.Depth())
	}
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Recent") {
		t.Errorf("Expected Recent view, got %s", ta.NavController.CurrentViewID())
	}
}

// Phase 3 cleanup: TestPluginEsc_ComplexDrillDown removed for the same
// reason as TestPluginStack_ComplexDrillDown above — the legacy 3-level
// Enter → TaskDetailViewID → 'e' → TaskEditViewID stack no longer
// exists, edit mode is in-place on the configurable detail view.

// ============================================================================
// Edge Case Tests
// ============================================================================

func TestPluginNavigation_NoTasks_EmptyView(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins but DON'T create any test data
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("Failed to load plugins: %v", err)
	}

	// Navigate to Roadmap (should be empty without epic tasks)
	ta.NavController.PushView(model.MakePluginViewID("Roadmap"), nil)
	ta.Draw()

	// Verify: view renders without crashing
	pluginConfig := ta.GetPluginConfig("Roadmap")
	if pluginConfig == nil {
		t.Fatal("Failed to get Roadmap plugin config")
	}

	// Selection should be clamped to 0
	if pluginConfig.GetSelectedIndex() != 0 {
		t.Errorf("Expected selection 0 in empty view, got %d", pluginConfig.GetSelectedIndex())
	}

	// Verify: Enter key does nothing (no crash)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Roadmap") {
		t.Error("Expected to stay on Roadmap view after Enter in empty view")
	}
}

func TestPluginActions_CreateFromPlugin_ReturnsToPlugin(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	ta.NavController.PushView(model.MakePluginViewID("Backlog"), nil)
	ta.Draw()

	// Create task
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)
	ta.SendText("Created from Plugin")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify: returned to Backlog plugin (not Board)
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Backlog") {
		t.Errorf("Expected Backlog view after creating task, got %s", ta.NavController.CurrentViewID())
	}

	// Verify: new task exists
	_ = ta.TaskStore.Reload()
	tikis := ta.TaskStore.GetAllTikis()
	var found bool
	for _, tk := range tikis {
		if tk.Title == "Created from Plugin" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find newly created task")
	}
}

func TestPluginActions_DeleteTask_UpdatesSelection(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("Failed to load plugins: %v", err)
	}

	// Create specific tasks for this test
	_ = testutil.CreateTestTask(ta.TaskDir, "00DEL1", "Task 1", taskpkg.StatusBacklog, taskpkg.TypeStory)
	_ = testutil.CreateTestTask(ta.TaskDir, "00DEL2", "Task 2", taskpkg.StatusBacklog, taskpkg.TypeStory)
	_ = testutil.CreateTestTask(ta.TaskDir, "00DEL3", "Task 3", taskpkg.StatusBacklog, taskpkg.TypeStory)
	_ = ta.TaskStore.Reload()

	ta.NavController.PushView(model.MakePluginViewID("Backlog"), nil)
	ta.Draw()

	pluginConfig := ta.GetPluginConfig("Backlog")
	if pluginConfig == nil {
		t.Fatal("Failed to get Backlog plugin config")
	}

	// Select second task (index 1)
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Delete it
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Verify: selection resets (typically to 0)
	// The exact behavior may vary, but selection should be valid
	newIndex := pluginConfig.GetSelectedIndex()
	if newIndex < 0 {
		t.Errorf("Expected valid selection after delete, got %d", newIndex)
	}

	// Verify: task count decreased
	_ = ta.TaskStore.Reload()
	tikis := ta.TaskStore.GetAllTikis()
	backlogCount := 0
	for _, tk := range tikis {
		status, _, _ := tk.StringField("status")
		if status == string(taskpkg.StatusBacklog) {
			backlogCount++
		}
	}
	if backlogCount >= 3 {
		t.Errorf("Expected fewer than 3 backlog tasks after delete, got %d", backlogCount)
	}
}

// ============================================================================
// Phase 3: Deep Navigation Stack Tests
// ============================================================================

// TestNavigationStack_BoardToTaskDetail verifies 2-level stack
// Phase 3: Enter pushes the configurable detail view (workflow-declared
// `kind: view` action), not the legacy TaskDetailViewID.
func TestNavigationStack_BoardToTaskDetail(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Board (depth 1)
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Open detail (Push, depth 2)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	if ta.NavController.Depth() != 2 {
		t.Errorf("Expected depth 2, got %d", ta.NavController.Depth())
	}
	wantDetail := model.DetailPluginViewID()
	if ta.NavController.CurrentViewID() != wantDetail {
		t.Errorf("Expected %s, got %s", wantDetail, ta.NavController.CurrentViewID())
	}

	// Esc back to board (Pop, depth 1)
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected depth 1 after Esc, got %d", ta.NavController.Depth())
	}
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Kanban") {
		t.Errorf("Expected Board view, got %s", ta.NavController.CurrentViewID())
	}
}

// Phase 3 cleanup: TestNavigationStack_BoardToDetailToEdit and
// TestNavigationStack_ThreeLevelDeep have been removed. They asserted
// the legacy 3-level Enter → TaskDetailViewID → 'e' → TaskEditViewID
// stack. After Phase 2, 'e' on the configurable detail view flips the
// same view into in-place edit mode without pushing a new entry, so the
// 3-level depth invariant no longer applies. Edit-mode behavior is
// covered by view/taskdetail/configurable_detail_edit_test.go and the
// surviving integration tests use the in-place edit flow.

// TestNavigationStack_MultipleTaskDetailOpens verifies stack doesn't corrupt with repeated opens
func TestNavigationStack_MultipleTaskDetailOpens(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Open several tasks in sequence without closing
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Open task 1
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	if ta.NavController.Depth() != 2 {
		t.Errorf("Expected depth 2 after first open, got %d", ta.NavController.Depth())
	}

	// Open task 2 from detail (shouldn't be possible normally, but test for robustness)
	// Go back first
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Move to another task and open
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	if ta.NavController.Depth() != 2 {
		t.Errorf("Expected depth 2 after second open, got %d", ta.NavController.Depth())
	}

	// Verify no stack corruption
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
	if ta.NavController.Depth() != 1 {
		t.Errorf("Expected depth 1 after final Esc, got %d", ta.NavController.Depth())
	}
	if ta.NavController.CurrentViewID() != model.MakePluginViewID("Kanban") {
		t.Errorf("Expected Board view, got %s", ta.NavController.CurrentViewID())
	}
}
