package integration

import (
	"fmt"
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

	if err := testutil.CreateTestTask(ta.TaskDir, "000001", "My Story", task.StatusBacklog, task.TypeStory); err != nil {
		t.Fatalf("failed to create story task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "000002", "My Epic", task.StatusBacklog, task.TypeEpic); err != nil {
		t.Fatalf("failed to create epic task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "000003", "Another Epic", task.StatusBacklog, task.TypeEpic); err != nil {
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
	updated := ta.TaskStore.GetTask("000001")
	if updated == nil {
		t.Fatal("task not found")
	}
	// should be one of the epic task IDs
	if updated.Assignee != "000002" && updated.Assignee != "000003" {
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
	updated := ta.TaskStore.GetTask("000001")
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
	updated := ta.TaskStore.GetTask("000001")
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
	updated := ta.TaskStore.GetTask("000001")
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

var filteredEpicWorkflow = testWorkflowPreamble + `views:
  plugins:
    - name: FilteredEpicTest
      key: "F4"
      lanes:
        - name: All
          columns: 1
          filter: select where status = "backlog" order by id
      actions:
        - key: "e"
          label: "Link to epic"
          action: update where id = choose(select where type = "epic" and id() not in dependsOn) set dependsOn = dependsOn + id()
`

func setupFilteredEpicTest(t *testing.T) *testutil.TestApp {
	t.Helper()
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(filteredEpicWorkflow), 0644); err != nil {
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

	if err := testutil.CreateTestTask(ta.TaskDir, "000001", "My Story", task.StatusBacklog, task.TypeStory); err != nil {
		t.Fatalf("failed to create story task: %v", err)
	}
	if err := testutil.CreateTestTaskWithDeps(ta.TaskDir, "000002", "Linked Epic", task.StatusBacklog, task.TypeEpic, []string{"000001"}); err != nil {
		t.Fatalf("failed to create linked epic task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "000003", "Available Epic", task.StatusBacklog, task.TypeEpic); err != nil {
		t.Fatalf("failed to create available epic task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	ta.NavController.PushView(model.MakePluginViewID("FilteredEpicTest"), nil)
	ta.Draw()

	return ta
}

func TestChooseAction_FiltersOutAlreadyLinkedEpics(t *testing.T) {
	ta := setupFilteredEpicTest(t)
	defer ta.Cleanup()

	ta.SendKey(tcell.KeyRune, 'e', tcell.ModNone)

	qsc := ta.GetQuickSelectConfig()
	if !qsc.IsVisible() {
		t.Fatal("QuickSelect should be visible")
	}

	qsX, qsW := 50, 30
	_, _, screenH := ta.Screen.GetContents()
	if ta.FindTextInRegion("000002", qsX, 0, qsW, screenH) {
		ta.DumpScreen()
		t.Fatal("linked epic should not appear in QuickSelect")
	}
	if !ta.FindTextInRegion("000003", qsX, 0, qsW, screenH) {
		ta.DumpScreen()
		t.Fatal("available epic should appear in QuickSelect")
	}

	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("000003")
	if updated == nil {
		t.Fatal("epic not found")
	}
	found := false
	for _, dep := range updated.DependsOn {
		if dep == "000001" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected TIKI-000003 to depend on TIKI-000001, got %v", updated.DependsOn)
	}
}

var scrollTestWorkflow = testWorkflowPreamble + `views:
  plugins:
    - name: ScrollTest
      key: "F4"
      lanes:
        - name: All
          columns: 1
          filter: select where status = "backlog" order by id
      actions:
        - key: "p"
          label: "Pick"
          action: update where id = id() set assignee = choose(select)
`

func setupScrollTest(t *testing.T) *testutil.TestApp {
	t.Helper()
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(scrollTestWorkflow), 0644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	ta := testutil.NewTestApp(t)
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("load plugins: %v", err)
	}

	for i := 0; i < 40; i++ {
		id := fmt.Sprintf("TIKI-%06d", i)
		title := fmt.Sprintf("ScrollTask%02d", i)
		if err := testutil.CreateTestTask(ta.TaskDir, id, title, task.StatusBacklog, task.TypeStory); err != nil {
			t.Fatalf("create task %s: %v", id, err)
		}
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	ta.NavController.PushView(model.MakePluginViewID("ScrollTest"), nil)
	ta.Draw()
	return ta
}

func TestChooseAction_ScrollKeepsSelectionVisible(t *testing.T) {
	ta := setupScrollTest(t)
	defer ta.Cleanup()

	ta.SendKey(tcell.KeyRune, 'p', tcell.ModNone)
	qsc := ta.GetQuickSelectConfig()
	if !qsc.IsVisible() {
		t.Fatal("QuickSelect should be visible")
	}

	// screen is 80x40; quick-select overlay is PaletteMinWidth (30) on the right
	// region: x=50, y=0, w=30, h=40
	qsX, qsW := 50, 30
	_, _, screenH := ta.Screen.GetContents()

	// arrow down 35 times: index 0 → index 35 (36th task, first beyond 35-visible viewport)
	for i := 0; i < 35; i++ {
		ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)
	}

	if !ta.FindTextInRegion("ScrollTask35", qsX, 0, qsW, screenH) {
		t.Fatal("after scrolling down 35 times, ScrollTask35 should be visible in the picker")
	}
	if ta.FindTextInRegion("ScrollTask00", qsX, 0, qsW, screenH) {
		t.Fatal("after scrolling down 35 times, ScrollTask00 should have scrolled out of the picker")
	}

	// wrap from last to first: press down until we wrap
	for i := 35; i < 40; i++ {
		ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)
	}
	// now at index 0 (wrapped)
	if !ta.FindTextInRegion("ScrollTask00", qsX, 0, qsW, screenH) {
		t.Fatal("after wrapping to first, ScrollTask00 should be visible in the picker")
	}

	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}
