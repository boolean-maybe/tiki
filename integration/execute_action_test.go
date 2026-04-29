package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

// initGitRepo initializes a minimal git repo so the runtime can resolve user().
// rukiRuntime.RunQuery calls readStore.GetCurrentUser(), which returns an error
// when the task directory isn't a git repo.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init", "-q", dir},
		{"git", "-C", dir, "config", "user.name", "test"},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...) //nolint:gosec // G204: hardcoded git args for test setup
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v (%s)", c, err, out)
		}
	}
}

var executeActionWorkflow = testWorkflowPreamble + `views:
  plugins:
    - name: ExecuteTest
      key: "F4"
      lanes:
        - name: All
          columns: 1
          filter: select where status = "backlog" order by id
`

func setupExecuteActionTest(t *testing.T) *testutil.TestApp {
	t.Helper()
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(executeActionWorkflow), 0644); err != nil {
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

	ta.NavController.PushView(model.MakePluginViewID("ExecuteTest"), nil)
	ta.Draw()

	return ta
}

func TestExecuteAction_BangOpensPrompt(t *testing.T) {
	ta := setupExecuteActionTest(t)
	defer ta.Cleanup()

	iv := getActiveInputableView(ta)
	if iv == nil {
		t.Fatal("active view does not implement InputableView")
	}
	if iv.IsInputBoxVisible() {
		t.Fatal("input box should not be visible initially")
	}

	ta.SendKey(tcell.KeyRune, '!', tcell.ModNone)

	if !iv.IsInputBoxVisible() {
		t.Fatal("input box should be visible after pressing '!'")
	}
	if !iv.IsInputBoxFocused() {
		t.Fatal("input box should be focused after pressing '!'")
	}
}

func TestExecuteAction_EnterRunsRukiAndCloses(t *testing.T) {
	ta := setupExecuteActionTest(t)
	defer ta.Cleanup()

	ta.SendKey(tcell.KeyRune, '!', tcell.ModNone)
	ta.SendText(`update where id = "000001" set assignee="alice"`)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	iv := getActiveInputableView(ta)
	if iv.IsInputBoxVisible() {
		t.Fatal("input box should be hidden after successful execute")
	}

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("000001")
	if updated == nil {
		t.Fatal("task not found")
	}
	if updated.Assignee != "alice" {
		t.Fatalf("expected assignee=alice after execute, got %q", updated.Assignee)
	}
}

func TestExecuteAction_EscCancelsWithoutMutation(t *testing.T) {
	ta := setupExecuteActionTest(t)
	defer ta.Cleanup()

	ta.SendKey(tcell.KeyRune, '!', tcell.ModNone)
	ta.SendText(`update where id = "000001" set assignee="bob"`)
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
		t.Fatalf("expected empty assignee after Esc, got %q", updated.Assignee)
	}
}

func TestExecuteAction_InvalidRukiKeepsPromptOpen(t *testing.T) {
	ta := setupExecuteActionTest(t)
	defer ta.Cleanup()

	iv := getActiveInputableView(ta)

	ta.SendKey(tcell.KeyRune, '!', tcell.ModNone)
	if !iv.IsInputBoxFocused() {
		t.Fatal("prompt should be focused after '!'")
	}

	// intentionally malformed ruki — parser should reject it
	ta.SendText("not a valid statement")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	if !iv.IsInputBoxVisible() {
		t.Fatal("prompt should stay visible after invalid ruki")
	}
	if !iv.IsInputBoxFocused() {
		t.Fatal("prompt should remain focused after invalid ruki")
	}

	// statusline should carry an error message
	sl := ta.GetStatuslineConfig()
	msg, level, _ := sl.GetMessage()
	if msg == "" {
		t.Fatal("expected statusline error message after invalid ruki")
	}
	if level != model.MessageLevelError {
		t.Fatalf("expected MessageLevelError, got %q", level)
	}

	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}

func TestExecuteAction_PaletteDispatchOpensPrompt(t *testing.T) {
	ta := setupExecuteActionTest(t)
	defer ta.Cleanup()

	iv := getActiveInputableView(ta)

	ta.InputRouter.HandleAction(controller.ActionExecute, ta.NavController.CurrentView())
	ta.Draw()

	if !iv.IsInputBoxVisible() {
		t.Fatal("palette-dispatched Execute should open the prompt")
	}
	if !iv.IsInputBoxFocused() {
		t.Fatal("prompt should be focused after palette dispatch")
	}

	ta.SendText(`update where id = "000001" set assignee="eve"`)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	if iv.IsInputBoxVisible() {
		t.Fatal("prompt should close after valid execute")
	}

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	updated := ta.TaskStore.GetTask("000001")
	if updated.Assignee != "eve" {
		t.Fatalf("expected assignee=eve via palette dispatch, got %q", updated.Assignee)
	}
}

func TestExecuteAction_RunPipeSuccessMessage(t *testing.T) {
	ta := setupExecuteActionTest(t)
	defer ta.Cleanup()

	ta.SendKey(tcell.KeyRune, '!', tcell.ModNone)
	ta.SendText(`select id where status = "backlog" | run("echo $1")`)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	iv := getActiveInputableView(ta)
	if iv.IsInputBoxVisible() {
		t.Fatal("prompt should close after successful pipe")
	}

	msg, level, _ := ta.GetStatuslineConfig().GetMessage()
	if level == model.MessageLevelError {
		t.Fatalf("unexpected error on statusline after pipe: %q", msg)
	}
	if !strings.Contains(msg, "ran command on 1 rows") {
		t.Fatalf("expected run-pipe success summary, got %q", msg)
	}
}

func TestExecuteAction_ClipboardPipeSuccessMessage(t *testing.T) {
	ta := setupExecuteActionTest(t)
	defer ta.Cleanup()

	var captured [][]string
	ta.InputRouter.SetClipboardWriter(func(rows [][]string) error {
		captured = rows
		return nil
	})

	ta.SendKey(tcell.KeyRune, '!', tcell.ModNone)
	ta.SendText(`select id where status = "backlog" | clipboard()`)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	iv := getActiveInputableView(ta)
	if iv.IsInputBoxVisible() {
		t.Fatal("prompt should close after successful clipboard pipe")
	}

	msg, level, _ := ta.GetStatuslineConfig().GetMessage()
	if level == model.MessageLevelError {
		t.Fatalf("unexpected error on statusline after clipboard: %q", msg)
	}
	if !strings.Contains(msg, "copied 1 rows to clipboard") {
		t.Fatalf("expected clipboard success summary, got %q", msg)
	}
	if len(captured) != 1 || len(captured[0]) != 1 || captured[0][0] != "000001" {
		t.Fatalf("expected clipboard to receive [[TIKI-1]], got %v", captured)
	}
}

func TestExecuteAction_RegisteredInPluginViewActions(t *testing.T) {
	r := controller.PluginViewActions()

	action := r.GetByID(controller.ActionExecute)
	if action == nil {
		t.Fatal("ActionExecute should be registered in PluginViewActions")
	}
	if action.Rune != '!' {
		t.Fatalf("expected '!' rune for Execute, got %q", action.Rune)
	}
	if !action.ShowInHeader {
		t.Fatal("Execute should be shown in header")
	}
	if action.HideFromPalette {
		t.Fatal("Execute should be visible in palette")
	}
}
