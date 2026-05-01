package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

// TestViewAction_BoardPressesKeyAndSwitchesView exercises a global `kind: view`
// action declared at the workflow top level. Pressing the bound key from a
// board view must push the target plugin view ID onto the nav stack.
func TestViewAction_BoardPressesKeyAndSwitchesView(t *testing.T) {
	workflowContent := testWorkflowPreamble + `actions:
  - key: F6
    kind: view
    label: "Open Docs"
    view: Docs
views:
  - name: Board
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Todo
        filter: select where status = "backlog"
  - name: Docs
    kind: wiki
    key: "F2"
    path: "index.md"
`
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflowContent), 0644); err != nil {
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
		t.Fatalf("LoadPlugins: %v", err)
	}
	defer ta.Cleanup()

	ta.NavController.PushView(model.MakePluginViewID("Board"), nil)
	ta.Draw()

	ta.SendKey(tcell.KeyF6, 0, tcell.ModNone)
	ta.Draw()

	want := model.MakePluginViewID("Docs")
	if got := ta.NavController.CurrentViewID(); got != want {
		t.Errorf("after F6: current view = %q, want %q", got, want)
	}
}

// TestViewAction_WikiViewDispatchesGlobal exercises the same dispatch path
// from a non-board view. `DokiController.handleGlobalAction` should switch
// to the target view.
func TestViewAction_WikiViewDispatchesGlobal(t *testing.T) {
	workflowContent := testWorkflowPreamble + `actions:
  - key: F7
    kind: view
    label: "Back to board"
    view: Board
views:
  - name: Board
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Todo
        filter: select where status = "backlog"
  - name: Docs
    kind: wiki
    key: "F2"
    path: "index.md"
`
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflowContent), 0644); err != nil {
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
		t.Fatalf("LoadPlugins: %v", err)
	}
	defer ta.Cleanup()

	ta.NavController.PushView(model.MakePluginViewID("Docs"), nil)
	ta.Draw()

	ta.SendKey(tcell.KeyF7, 0, tcell.ModNone)
	ta.Draw()

	want := model.MakePluginViewID("Board")
	if got := ta.NavController.CurrentViewID(); got != want {
		t.Errorf("after F7: current view = %q, want %q", got, want)
	}
}

// Rejection of a `kind: view` action that points at an unknown view name is
// covered at the loader unit-test level in plugin/loader_test.go —
// TestLoadPluginsFromFile_FailClosedOnAnyError and
// TestParsePluginActions_ViewKindErrors/unknown_view_name. An integration-level
// test would need to bootstrap the process-wide workflow/status registry,
// which obscures what's being tested.
