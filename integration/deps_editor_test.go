package integration

import (
	"slices"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

// openDepsEditor navigates: Kanban → Enter (task detail) → Ctrl+D (deps editor)
// The task selected on the Kanban board becomes the context task.
func openDepsEditor(ta *testutil.TestApp) {
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	ta.Draw()
	ta.SendKey(tcell.KeyCtrlD, 0, tcell.ModCtrl)
	ta.Draw()
}

// TestDepsEditor_OpenFromTaskDetail verifies Ctrl+D on task detail pushes the deps plugin view.
func TestDepsEditor_OpenFromTaskDetail(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	contextID := "TIKI-CTXA01"
	if err := testutil.CreateTestTask(ta.TaskDir, contextID, "Context Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create context task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-FREE01", "Free Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create free task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	openDepsEditor(ta)

	wantViewID := model.MakePluginViewID("deps:" + contextID)
	current := ta.NavController.CurrentView()
	if current.ViewID != wantViewID {
		ta.DumpScreen()
		t.Fatalf("current view = %v, want %v", current.ViewID, wantViewID)
	}

	for _, label := range []string{"Blocks", "All", "Depends"} {
		if found, _, _ := ta.FindText(label); !found {
			ta.DumpScreen()
			t.Errorf("lane label %q not found on screen", label)
		}
	}
}

// TestDepsEditor_LanesShowCorrectTasks verifies each lane contains the expected tasks.
func TestDepsEditor_LanesShowCorrectTasks(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	contextID := "TIKI-CTXA02"
	depID := "TIKI-DEP002"
	blockerID := "TIKI-BLK002"
	freeID := "TIKI-FRE002"

	// context depends on dep; blocker depends on context
	if err := testutil.CreateTestTaskWithDeps(ta.TaskDir, contextID, "Context Task", taskpkg.StatusReady, taskpkg.TypeStory, []string{depID}); err != nil {
		t.Fatalf("create context: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, depID, "Dep Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create dep: %v", err)
	}
	if err := testutil.CreateTestTaskWithDeps(ta.TaskDir, blockerID, "Blocker Task", taskpkg.StatusReady, taskpkg.TypeStory, []string{contextID}); err != nil {
		t.Fatalf("create blocker: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, freeID, "Free Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create free: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// navigate to context task then open deps editor
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// context task should be first in the Ready lane — press Enter to open detail
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	ta.Draw()

	// confirm we're on the right task detail
	if found, _, _ := ta.FindText(contextID); !found {
		ta.DumpScreen()
		t.Fatalf("context task %s not selected on board", contextID)
	}

	ta.SendKey(tcell.KeyCtrlD, 0, tcell.ModCtrl)
	ta.Draw()

	// Blocker task belongs in Blocks lane (it depends on context)
	if found, _, _ := ta.FindText("Blocker Task"); !found {
		ta.DumpScreen()
		t.Errorf("Blocker Task not visible (expected in Blocks lane)")
	}

	// Dep task belongs in Depends lane (context depends on it)
	if found, _, _ := ta.FindText("Dep Task"); !found {
		ta.DumpScreen()
		t.Errorf("Dep Task not visible (expected in Depends lane)")
	}

	// Free task belongs in All lane
	if found, _, _ := ta.FindText("Free Task"); !found {
		ta.DumpScreen()
		t.Errorf("Free Task not visible (expected in All lane)")
	}

	// Context task must not appear anywhere in the deps view
	if found, _, _ := ta.FindText("Context Task"); found {
		ta.DumpScreen()
		t.Errorf("Context Task should not be visible in deps editor")
	}
}

// TestDepsEditor_MoveTask_AllToDepends_PersistsOnDisk verifies moving a task from All → Depends
// updates DependsOn in memory and persists on disk after reload.
func TestDepsEditor_MoveTask_AllToDepends_PersistsOnDisk(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	contextID := "TIKI-CTXA03"
	freeID := "TIKI-FRE003"

	if err := testutil.CreateTestTask(ta.TaskDir, contextID, "Context Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create context: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, freeID, "Free Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create free: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	openDepsEditor(ta)

	// verify we're in the deps editor
	wantViewID := model.MakePluginViewID("deps:" + contextID)
	if ta.NavController.CurrentView().ViewID != wantViewID {
		ta.DumpScreen()
		t.Fatalf("not in deps editor, got %v", ta.NavController.CurrentView().ViewID)
	}

	// Blocks lane is empty, so selection should land on All lane automatically.
	// Shift+Right moves selected task from All → Depends.
	ta.SendKey(tcell.KeyRight, 0, tcell.ModShift)
	ta.Draw()

	// verify in-memory state
	updated := ta.TaskStore.GetTask(contextID)
	if updated == nil {
		t.Fatalf("context task not found in store")
	}
	if !slices.Contains(updated.DependsOn, freeID) {
		t.Errorf("DependsOn = %v, want it to contain %s", updated.DependsOn, freeID)
	}

	// verify persisted to disk
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload after move: %v", err)
	}
	reloaded := ta.TaskStore.GetTask(contextID)
	if reloaded == nil {
		t.Fatalf("context task not found after reload")
	}
	if !slices.Contains(reloaded.DependsOn, freeID) {
		t.Errorf("after reload: DependsOn = %v, want it to contain %s", reloaded.DependsOn, freeID)
	}
}

// TestDepsEditor_MoveTask_DependsToAll_RemovesDep verifies moving a task from Depends → All
// removes it from DependsOn in memory and on disk.
func TestDepsEditor_MoveTask_DependsToAll_RemovesDep(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	contextID := "TIKI-CTXA04"
	depID := "TIKI-DEP004"
	freeID := "TIKI-FRE004"

	if err := testutil.CreateTestTaskWithDeps(ta.TaskDir, contextID, "Context Task", taskpkg.StatusReady, taskpkg.TypeStory, []string{depID}); err != nil {
		t.Fatalf("create context: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, depID, "Dep Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create dep: %v", err)
	}
	// a free task is needed so All lane is non-empty — handleLaneSwitch skips empty lanes,
	// so without it Shift+H from Depends has nowhere to land and becomes a no-op.
	if err := testutil.CreateTestTask(ta.TaskDir, freeID, "Free Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create free: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	openDepsEditor(ta)

	wantViewID := model.MakePluginViewID("deps:" + contextID)
	if ta.NavController.CurrentView().ViewID != wantViewID {
		ta.DumpScreen()
		t.Fatalf("not in deps editor, got %v", ta.NavController.CurrentView().ViewID)
	}

	// EnsureFirstNonEmptyLaneSelection picks the first non-empty lane: Blocks is empty,
	// All has the free task, so selection starts on All (lane 1).
	// Navigate right once to reach Depends lane.
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)
	ta.Draw()

	// Shift+Left moves selected task from Depends → All
	ta.SendKey(tcell.KeyLeft, 0, tcell.ModShift)
	ta.Draw()

	// verify in-memory state
	updated := ta.TaskStore.GetTask(contextID)
	if updated == nil {
		t.Fatalf("context task not found in store")
	}
	if slices.Contains(updated.DependsOn, depID) {
		t.Errorf("DependsOn = %v, should not contain %s after removal", updated.DependsOn, depID)
	}

	// verify persisted to disk
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload after move: %v", err)
	}
	reloaded := ta.TaskStore.GetTask(contextID)
	if reloaded == nil {
		t.Fatalf("context task not found after reload")
	}
	if slices.Contains(reloaded.DependsOn, depID) {
		t.Errorf("after reload: DependsOn = %v, should not contain %s", reloaded.DependsOn, depID)
	}
}

// TestDepsEditor_ReopenIsSameView verifies that opening the deps editor for the same task
// a second time reuses the existing plugin entry (idempotency).
func TestDepsEditor_ReopenIsSameView(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	contextID := "TIKI-CTXA05"
	if err := testutil.CreateTestTask(ta.TaskDir, contextID, "Context Task", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create context: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	wantViewID := model.MakePluginViewID("deps:" + contextID)

	// first open
	openDepsEditor(ta)
	if ta.NavController.CurrentView().ViewID != wantViewID {
		ta.DumpScreen()
		t.Fatalf("first open: not in deps editor, got %v", ta.NavController.CurrentView().ViewID)
	}

	// go back to task detail
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
	ta.Draw()
	if ta.NavController.CurrentView().ViewID != model.TaskDetailViewID {
		ta.DumpScreen()
		t.Fatalf("expected task detail after Esc, got %v", ta.NavController.CurrentView().ViewID)
	}

	// second open — should reuse existing plugin, not create a duplicate
	ta.SendKey(tcell.KeyCtrlD, 0, tcell.ModCtrl)
	ta.Draw()

	if ta.NavController.CurrentView().ViewID != wantViewID {
		ta.DumpScreen()
		t.Fatalf("second open: not in deps editor, got %v", ta.NavController.CurrentView().ViewID)
	}

	// verify the deps view still renders correctly (plugin wiring intact)
	if found, _, _ := ta.FindText("All"); !found {
		ta.DumpScreen()
		t.Errorf("lane label 'All' not found on second open")
	}
}
