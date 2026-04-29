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

var inputActionWorkflow = testWorkflowPreamble + `views:
  plugins:
    - name: InputTest
      key: "F4"
      lanes:
        - name: All
          columns: 1
          filter: select where status = "backlog" order by id
      actions:
        - key: "A"
          label: "Assign to..."
          action: update where id = id() set assignee=input()
          input: string
        - key: "t"
          label: "Add tag"
          action: update where id = id() set tags=tags+[input()]
          input: string
        - key: "p"
          label: "Set points"
          action: update where id = id() set points=input()
          input: int
        - key: "b"
          label: "Add to board"
          action: update where id = id() set status="ready"
`

func setupInputActionTest(t *testing.T) *testutil.TestApp {
	t.Helper()
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(inputActionWorkflow), 0644); err != nil {
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

	if err := testutil.CreateTestTask(ta.TaskDir, "000001", "Test Task", task.StatusBacklog, task.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	ta.NavController.PushView(model.MakePluginViewID("InputTest"), nil)
	ta.Draw()

	return ta
}

func getActiveInputableView(ta *testutil.TestApp) controller.InputableView {
	v := ta.NavController.GetActiveView()
	iv, _ := v.(controller.InputableView)
	return iv
}

func TestInputAction_KeyOpensPrompt(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	iv := getActiveInputableView(ta)
	if iv == nil {
		t.Fatal("active view does not implement InputableView")
	}
	if iv.IsInputBoxVisible() {
		t.Fatal("input box should not be visible initially")
	}

	ta.SendKey(tcell.KeyRune, 'A', tcell.ModNone)

	if !iv.IsInputBoxVisible() {
		t.Fatal("input box should be visible after pressing 'A'")
	}
	if !iv.IsInputBoxFocused() {
		t.Fatal("input box should be focused after pressing 'A'")
	}
}

func TestInputAction_EnterAppliesMutation(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	ta.SendKey(tcell.KeyRune, 'A', tcell.ModNone)
	ta.SendText("alice")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	iv := getActiveInputableView(ta)
	if iv.IsInputBoxVisible() {
		t.Fatal("input box should be hidden after valid submit")
	}

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("000001")
	if updated == nil {
		t.Fatal("task not found")
	}
	if updated.Assignee != "alice" {
		t.Fatalf("expected assignee=alice, got %q", updated.Assignee)
	}
}

func TestInputAction_EscCancelsWithoutMutation(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	ta.SendKey(tcell.KeyRune, 'A', tcell.ModNone)
	ta.SendText("bob")
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	iv := getActiveInputableView(ta)
	if iv.IsInputBoxVisible() {
		t.Fatal("input box should be hidden after Esc")
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

func TestInputAction_NonInputActionStillWorks(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	// 'b' is a non-input action — should execute immediately without prompt
	ta.SendKey(tcell.KeyRune, 'b', tcell.ModNone)

	iv := getActiveInputableView(ta)
	if iv.IsInputBoxVisible() {
		t.Fatal("non-input action should not open input box")
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

func TestInputAction_ModalBlocksOtherActions(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	// open action-input prompt
	ta.SendKey(tcell.KeyRune, 'A', tcell.ModNone)
	iv := getActiveInputableView(ta)
	if !iv.IsInputBoxFocused() {
		t.Fatal("input box should be focused")
	}

	// while modal, 'b' should NOT execute the non-input action
	ta.SendKey(tcell.KeyRune, 'b', tcell.ModNone)

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("000001")
	if updated.Status != task.StatusBacklog {
		t.Fatalf("expected status backlog (action should be blocked while modal), got %v", updated.Status)
	}

	// cancel and verify box closes
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
	if iv.IsInputBoxVisible() {
		t.Fatal("input box should be hidden after Esc")
	}
}

func TestInputAction_SearchPassiveBlocksNewSearch(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	iv := getActiveInputableView(ta)

	// open search
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	if !iv.IsInputBoxFocused() {
		t.Fatal("search box should be focused after '/'")
	}

	// type and submit search
	ta.SendText("Test")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// should be in passive mode: visible but not focused
	if !iv.IsInputBoxVisible() {
		t.Fatal("search box should remain visible in passive mode")
	}
	if iv.IsInputBoxFocused() {
		t.Fatal("search box should not be focused in passive mode")
	}
	if !iv.IsSearchPassive() {
		t.Fatal("expected search-passive state")
	}

	// pressing '/' again should NOT re-enter search editing
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	if iv.IsInputBoxFocused() {
		t.Fatal("'/' should be blocked while search is passive — user must Esc first")
	}

	// Esc clears search and closes
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
	if iv.IsInputBoxVisible() {
		t.Fatal("input box should be hidden after Esc from passive mode")
	}
}

func TestInputAction_PassiveSearchReplacedByActionInput(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	iv := getActiveInputableView(ta)

	// set up passive search
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	ta.SendText("Test")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	if !iv.IsSearchPassive() {
		t.Fatal("expected search-passive state")
	}

	// action-input should temporarily replace the passive search
	ta.SendKey(tcell.KeyRune, 'A', tcell.ModNone)
	if !iv.IsInputBoxFocused() {
		t.Fatal("action-input should be focused, replacing passive search")
	}
	if iv.IsSearchPassive() {
		t.Fatal("should no longer be in search-passive while action-input is active")
	}

	// submit action input
	ta.SendText("carol")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// should restore passive search
	if !iv.IsInputBoxVisible() {
		t.Fatal("passive search should be restored after action-input closes")
	}
	if !iv.IsSearchPassive() {
		t.Fatal("should be back in search-passive mode")
	}

	// verify the mutation happened
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("000001")
	if updated == nil {
		t.Fatal("task not found")
	}
	if updated.Assignee != "carol" {
		t.Fatalf("expected assignee=carol, got %q", updated.Assignee)
	}
}

func TestInputAction_ActionInputEscRestoresPassiveSearch(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	iv := getActiveInputableView(ta)

	// set up passive search
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	ta.SendText("Test")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// open action-input
	ta.SendKey(tcell.KeyRune, 'A', tcell.ModNone)
	ta.SendText("dave")

	// Esc should restore passive search, not clear it
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	if !iv.IsSearchPassive() {
		t.Fatal("Esc from action-input should restore passive search")
	}

	// verify no mutation
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("000001")
	if updated.Assignee != "" {
		t.Fatalf("expected empty assignee after cancel, got %q", updated.Assignee)
	}
}

func TestInputAction_SearchEditingBlocksPluginActions(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	iv := getActiveInputableView(ta)

	// open search
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	if !iv.IsInputBoxFocused() {
		t.Fatal("search box should be focused")
	}

	// while search editing is active, 'b' (non-input action) should be blocked
	ta.SendKey(tcell.KeyRune, 'b', tcell.ModNone)

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("000001")
	if updated.Status != task.StatusBacklog {
		t.Fatalf("expected status backlog (action blocked during search editing), got %v", updated.Status)
	}

	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}

func TestInputAction_EmptySearchEnterIsNoOp(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	iv := getActiveInputableView(ta)

	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	if !iv.IsInputBoxFocused() {
		t.Fatal("search box should be focused")
	}

	// Enter on empty text should keep editing open
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	if !iv.IsInputBoxFocused() {
		t.Fatal("empty search Enter should keep box focused (no-op)")
	}
	if iv.IsSearchPassive() {
		t.Fatal("empty search Enter should not transition to passive")
	}

	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}

func TestInputAction_PaletteOpensDuringModal(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	// open action-input
	ta.SendKey(tcell.KeyRune, 'A', tcell.ModNone)
	iv := getActiveInputableView(ta)
	if !iv.IsInputBoxFocused() {
		t.Fatal("input box should be focused")
	}

	// Ctrl+A should open the palette even while input box is focused
	ta.SendKey(tcell.KeyCtrlA, 0, tcell.ModCtrl)
	if !ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should open when Ctrl+A is pressed with input box focused")
	}

	// clean up
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}

func TestInputAction_PaletteDispatchOpensPrompt(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	iv := getActiveInputableView(ta)

	// simulate palette dispatch: call HandleAction directly with the input-backed action ID
	actionID := controller.ActionID("plugin_action:A")
	ta.InputRouter.HandleAction(actionID, ta.NavController.CurrentView())
	ta.Draw()

	if !iv.IsInputBoxVisible() {
		t.Fatal("palette-dispatched input action should open the prompt")
	}
	if !iv.IsInputBoxFocused() {
		t.Fatal("prompt should be focused after palette dispatch")
	}

	// type and submit
	ta.SendText("eve")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	if iv.IsInputBoxVisible() {
		t.Fatal("prompt should close after valid submit")
	}

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("000001")
	if updated.Assignee != "eve" {
		t.Fatalf("expected assignee=eve via palette dispatch, got %q", updated.Assignee)
	}
}

func TestInputAction_InvalidInputKeepsPromptOpen(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	iv := getActiveInputableView(ta)

	originalTask := ta.TaskStore.GetTask("000001")
	originalPoints := originalTask.Points

	// open int input (points)
	ta.SendKey(tcell.KeyRune, 'p', tcell.ModNone)
	if !iv.IsInputBoxFocused() {
		t.Fatal("prompt should be focused")
	}

	// type non-numeric text and submit
	ta.SendText("abc")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// prompt should stay open — invalid input
	if !iv.IsInputBoxFocused() {
		t.Fatal("prompt should remain focused after invalid input")
	}
	if !iv.IsInputBoxVisible() {
		t.Fatal("prompt should remain visible after invalid input")
	}

	// verify no mutation
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("000001")
	if updated.Points != originalPoints {
		t.Fatalf("expected points=%d (unchanged), got %d", originalPoints, updated.Points)
	}

	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}

func TestInputAction_PreflightNoTaskSelected_NoPrompt(t *testing.T) {
	tmpDir := t.TempDir()

	// workflow with a lane that will match no tasks (review lane, but test task is backlog)
	workflow := testWorkflowPreamble + `views:
  plugins:
    - name: EmptyTest
      key: "F4"
      lanes:
        - name: Empty
          columns: 1
          filter: select where status = "review" order by id
      actions:
        - key: "A"
          label: "Assign to..."
          action: update where id = id() set assignee=input()
          input: string
`
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflow), 0644); err != nil {
		t.Fatalf("failed to write workflow.yaml: %v", err)
	}
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	ta := testutil.NewTestApp(t)
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}
	defer ta.Cleanup()

	// create a task, but it won't match the filter
	if err := testutil.CreateTestTask(ta.TaskDir, "000001", "Test", task.StatusBacklog, task.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	ta.NavController.PushView(model.MakePluginViewID("EmptyTest"), nil)
	ta.Draw()

	iv := getActiveInputableView(ta)

	// press 'A' — no task selected, preflight should fail, no prompt
	ta.SendKey(tcell.KeyRune, 'A', tcell.ModNone)
	if iv != nil && iv.IsInputBoxVisible() {
		t.Fatal("input prompt should not open when no task is selected")
	}
}

func TestInputAction_DraftSearchSurvivesRefresh(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	iv := getActiveInputableView(ta)

	// open search and type (but don't submit)
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	ta.SendText("draft")

	if !iv.IsInputBoxFocused() {
		t.Fatal("search box should be focused")
	}

	// simulate a store refresh (which triggers view rebuild)
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	ta.Draw()

	// search box should still be visible after refresh
	if !iv.IsInputBoxVisible() {
		t.Fatal("draft search should survive store refresh/rebuild")
	}

	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}

func TestInputAction_AddTagMutation(t *testing.T) {
	ta := setupInputActionTest(t)
	defer ta.Cleanup()

	ta.SendKey(tcell.KeyRune, 't', tcell.ModNone)
	ta.SendText("urgent")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("000001")
	if updated == nil {
		t.Fatal("task not found")
	}
	found := false
	for _, tag := range updated.Tags {
		if tag == "urgent" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'urgent' tag, got %v", updated.Tags)
	}
}

var compositeKeyWorkflow = testWorkflowPreamble + `views:
  plugins:
    - name: CompositeTest
      key: "F4"
      lanes:
        - name: All
          columns: 1
          filter: select where status = "backlog" order by id
      actions:
        - key: "Ctrl-U"
          label: "Unblock"
          action: update where id = id() set status="ready"
`

func TestInputAction_CompositeKeyPluginAction(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(compositeKeyWorkflow), 0644); err != nil {
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

	if err := testutil.CreateTestTask(ta.TaskDir, "000001", "Blocked Task", task.StatusBacklog, task.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	ta.NavController.PushView(model.MakePluginViewID("CompositeTest"), nil)
	ta.Draw()

	// send Ctrl-U keypress through the real EventKey → Match → HandleAction path
	ta.SendKey(tcell.KeyCtrlU, 0, tcell.ModCtrl)

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("000001")
	if updated.Status != task.StatusReady {
		t.Fatalf("expected status 'ready' after Ctrl-U action, got %q", updated.Status)
	}
}
