package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

var chooseActionWorkflow = testWorkflowPreamble + `views:
  plugins:
    - name: ChooseTest
      key: "F4"
      lanes:
        - name: All
          columns: 1
          filter: select where status = "backlog" order by id
      actions:
        - key: "e"
          label: "Set Assignee from Epic"
          action: update where id = id() set assignee = choose(select where type = "epic")
        - key: "p"
          label: "Set Assignee from All"
          action: update where id = id() set assignee = choose(select)
        - key: "b"
          label: "Add to board"
          action: update where id = id() set status="ready"
`

func setupChooseActionTest(t *testing.T) *testutil.TestApp {
	t.Helper()
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(chooseActionWorkflow), 0644); err != nil {
		t.Fatalf("failed to write workflow.yaml: %v", err)
	}
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	ta := testutil.NewTestApp(t)
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "My Story", task.StatusBacklog, task.TypeStory); err != nil {
		t.Fatalf("failed to create story task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "My Epic", task.StatusBacklog, task.TypeEpic); err != nil {
		t.Fatalf("failed to create epic task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-3", "Another Epic", task.StatusBacklog, task.TypeEpic); err != nil {
		t.Fatalf("failed to create second epic task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	ta.NavController.PushView(model.MakePluginViewID("ChooseTest"), nil)
	ta.Draw()

	return ta
}

func TestChooseAction_KeyOpensQuickSelect(t *testing.T) {
	ta := setupChooseActionTest(t)
	defer ta.Cleanup()

	qsc := ta.GetQuickSelectConfig()
	if qsc.IsVisible() {
		t.Fatal("QuickSelect should not be visible initially")
	}

	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	if !qsc.IsVisible() {
		t.Fatal("QuickSelect should be visible after pressing 'e'")
	}
}

func TestChooseAction_EnterAppliesMutation(t *testing.T) {
	ta := setupChooseActionTest(t)
	defer ta.Cleanup()

	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	qsc := ta.GetQuickSelectConfig()
	if !qsc.IsVisible() {
		t.Fatal("QuickSelect should be visible")
	}

	// the picker shows only epics (filtered by type = "epic")
	// press Enter to select the first one
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	if qsc.IsVisible() {
		t.Fatal("QuickSelect should be hidden after selection")
	}

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("TIKI-1")
	if updated == nil {
		t.Fatal("task not found")
	}
	// should be one of the epic task IDs
	if updated.Assignee != "TIKI-2" && updated.Assignee != "TIKI-3" {
		t.Fatalf("expected assignee to be TIKI-2 or TIKI-3, got %q", updated.Assignee)
	}
}

func TestChooseAction_EscCancelsWithoutMutation(t *testing.T) {
	ta := setupChooseActionTest(t)
	defer ta.Cleanup()

	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	qsc := ta.GetQuickSelectConfig()
	if !qsc.IsVisible() {
		t.Fatal("QuickSelect should be visible")
	}

	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	if qsc.IsVisible() {
		t.Fatal("QuickSelect should be hidden after Esc")
	}

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("TIKI-1")
	if updated == nil {
		t.Fatal("task not found")
	}
	if updated.Assignee != "" {
		t.Fatalf("expected empty assignee after cancel, got %q", updated.Assignee)
	}
}

func TestChooseAction_NonChooseActionStillWorks(t *testing.T) {
	ta := setupChooseActionTest(t)
	defer ta.Cleanup()

	// 'b' is a non-choose action — should execute immediately
	ta.SendKey(tcell.KeyRune, 'b', tcell.ModNone)

	qsc := ta.GetQuickSelectConfig()
	if qsc.IsVisible() {
		t.Fatal("non-choose action should not open QuickSelect")
	}

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("TIKI-1")
	if updated == nil {
		t.Fatal("task not found")
	}
	if updated.Status != task.StatusReady {
		t.Fatalf("expected status ready, got %v", updated.Status)
	}
}

func TestChooseAction_ModalBlocksOtherActions(t *testing.T) {
	ta := setupChooseActionTest(t)
	defer ta.Cleanup()

	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	qsc := ta.GetQuickSelectConfig()
	if !qsc.IsVisible() {
		t.Fatal("QuickSelect should be visible")
	}

	// while modal, 'b' should NOT execute
	ta.SendKey(tcell.KeyRune, 'b', tcell.ModNone)

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("TIKI-1")
	if updated.Status != task.StatusBacklog {
		t.Fatalf("expected status backlog (action blocked while modal), got %v", updated.Status)
	}

	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}

func TestChooseAction_PaletteDispatchOpensQuickSelect(t *testing.T) {
	ta := setupChooseActionTest(t)
	defer ta.Cleanup()

	actionID := controller.ActionID("plugin_action:e")
	ta.InputRouter.HandleAction(actionID, ta.NavController.CurrentView())
	ta.Draw()

	qsc := ta.GetQuickSelectConfig()
	if !qsc.IsVisible() {
		t.Fatal("palette-dispatched choose action should open QuickSelect")
	}

	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}

func TestChooseAction_BareSelectShowsAllTasks(t *testing.T) {
	ta := setupChooseActionTest(t)
	defer ta.Cleanup()

	// 'p' uses choose(select) with no filter — all tasks should be candidates
	ta.SendKey(tcell.KeyRune, 'p', tcell.ModNone)

	qsc := ta.GetQuickSelectConfig()
	if !qsc.IsVisible() {
		t.Fatal("QuickSelect should be visible for bare select")
	}

	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}
