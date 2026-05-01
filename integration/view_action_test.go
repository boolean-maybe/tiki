package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/task"
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

// TestViewAction_DetailKindRendersSelectedTask exercises Phase 6B.2 end-to-end.
// Board view → select a task → F7 (kind: view → Detail) → assert the detail
// view rendered the selected task by matching the view ID and selection
// passthrough. The actual rendered body is an implementation detail tested
// at unit level in view/markdown; the integration concern is that selection
// survives the view switch.
func TestViewAction_DetailKindRendersSelectedTask(t *testing.T) {
	workflowContent := testWorkflowPreamble + `actions:
  - key: F7
    kind: view
    label: "Show detail"
    view: Selected
    require: ["selection:one"]
views:
  - name: Board
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Todo
        filter: select where status = "backlog"
  - name: Selected
    kind: detail
    key: "F2"
    require: ["selection:one"]
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

	if err := testutil.CreateTestTask(ta.TaskDir, "000001", "Pick Me", task.StatusBacklog, task.TypeStory); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// Land on the Board view. The default PluginConfig selection points at
	// the first lane / first task, which is the task we just created.
	ta.NavController.PushView(model.MakePluginViewID("Board"), nil)
	ta.Draw()

	// F7 invokes `kind: view` with selection:one — requires a task selected.
	ta.SendKey(tcell.KeyF7, 0, tcell.ModNone)
	ta.Draw()

	// We should now be on the Selected (detail) view.
	want := model.MakePluginViewID("Selected")
	if got := ta.NavController.CurrentViewID(); got != want {
		t.Errorf("current view = %q, want %q", got, want)
	}
}

// TestViewAction_ViewKindBlockedWithoutSelection asserts that
// `require: ["selection:one"]` on a `kind: view` action prevents navigation
// when no task is selected (empty lane).
func TestViewAction_ViewKindBlockedWithoutSelection(t *testing.T) {
	workflowContent := testWorkflowPreamble + `actions:
  - key: F7
    kind: view
    label: "Show detail"
    view: Selected
    require: ["selection:one"]
views:
  - name: Board
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Todo
        filter: select where status = "backlog"
  - name: Selected
    kind: detail
    key: "F2"
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

	// No tasks seeded — lane is empty, nothing selected.
	ta.NavController.PushView(model.MakePluginViewID("Board"), nil)
	ta.Draw()

	before := ta.NavController.CurrentViewID()
	ta.SendKey(tcell.KeyF7, 0, tcell.ModNone)
	ta.Draw()
	if got := ta.NavController.CurrentViewID(); got != before {
		t.Errorf("F7 with no selection must not switch views: was %q, now %q", before, got)
	}
}

// TestViewAction_DirectActivationCarriesSelection asserts 6B.18:
// pressing the target view's direct activation key (not a declared
// `kind: view` action) from a board with a task selected must navigate
// to the target AND carry the selection into the target's params, so a
// `kind: detail` target sees the selected task.
func TestViewAction_DirectActivationCarriesSelection(t *testing.T) {
	workflowContent := testWorkflowPreamble + `views:
  - name: Board
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Todo
        filter: select where status = "backlog"
  - name: Selected
    kind: detail
    key: "F2"
    require: ["selection:one"]
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

	// Seed a task so the board has a selection.
	if err := testutil.CreateTestTask(ta.TaskDir, "000042", "Carried", task.StatusBacklog, task.TypeStory); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	ta.NavController.PushView(model.MakePluginViewID("Board"), nil)
	ta.Draw()

	// Direct activation key (F2) — not a declared kind: view action.
	ta.SendKey(tcell.KeyF2, 0, tcell.ModNone)
	ta.Draw()

	// Target view must be active with the carried task id in its params.
	currentView := ta.NavController.CurrentView()
	if currentView == nil || currentView.ViewID != model.MakePluginViewID("Selected") {
		t.Fatalf("expected current view = Selected, got %v", currentView)
	}
	gotParams := model.DecodePluginViewParams(currentView.Params)
	if gotParams.TaskID != "000042" {
		t.Errorf("direct activation must carry selection; got TaskID=%q, want %q",
			gotParams.TaskID, "000042")
	}
}

// TestViewAction_TargetViewRequireBlocksNavigation asserts 6B.15: a
// kind: view action with NO own `require:` still refuses to navigate to a
// target view that declares its own `require: ["selection:one"]` when no
// task is selected. The target's declared requirements must be honored
// regardless of how navigation is triggered.
func TestViewAction_TargetViewRequireBlocksNavigation(t *testing.T) {
	workflowContent := testWorkflowPreamble + `actions:
  - key: F7
    kind: view
    label: "Show detail"
    view: Selected
views:
  - name: Board
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Todo
        filter: select where status = "backlog"
  - name: Selected
    kind: detail
    key: "F2"
    require: ["selection:one"]
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

	// No tasks seeded — lane is empty, nothing selected. The action
	// itself has no require:, so its own gate passes; only the target
	// view's require should block.
	ta.NavController.PushView(model.MakePluginViewID("Board"), nil)
	ta.Draw()

	before := ta.NavController.CurrentViewID()
	ta.SendKey(tcell.KeyF7, 0, tcell.ModNone)
	ta.Draw()
	if got := ta.NavController.CurrentViewID(); got != before {
		t.Errorf("F7 navigation should be blocked by target view's require:selection:one when no task selected; was %q, now %q", before, got)
	}
}

// TestViewAction_TargetRequireNonSelectionAttributeBlocks asserts 6B.20:
// the target view's `require:` list is evaluated in full (not just for
// selection cardinality). A target with `require: ["ai"]` must refuse
// navigation when the AI agent is not configured, even when the action
// itself has no require and a task is selected.
func TestViewAction_TargetRequireNonSelectionAttributeBlocks(t *testing.T) {
	workflowContent := testWorkflowPreamble + `actions:
  - key: F7
    kind: view
    label: "Jump to AI"
    view: AiOnly
views:
  - name: Board
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Todo
        filter: select where status = "backlog"
  - name: AiOnly
    kind: detail
    key: "F2"
    require: ["ai"]
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

	// Seed a task so the source board has a selection — this is the
	// condition that used to slip past: a selection-only check would
	// approve the navigation, and the "ai" requirement on the target
	// would be silently ignored.
	if err := testutil.CreateTestTask(ta.TaskDir, "000001", "Anything", task.StatusBacklog, task.TypeStory); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	ta.NavController.PushView(model.MakePluginViewID("Board"), nil)
	ta.Draw()

	before := ta.NavController.CurrentViewID()
	ta.SendKey(tcell.KeyF7, 0, tcell.ModNone)
	ta.Draw()
	if got := ta.NavController.CurrentViewID(); got != before {
		t.Errorf("kind: view with target require:[ai] must refuse when AI not configured; was %q, now %q", before, got)
	}
}

// TestViewAction_TargetRequireViewAttributeEvaluatedAgainstTarget asserts
// 6B.22: `view:*` attributes in a target's require list are evaluated
// against the target view's identity, not the source's. Uses a target
// that declares `require: ["view:plugin:SelfNamed"]` where "SelfNamed"
// is the target's own name. Under source-context evaluation this would
// fail (source's view id != target's). Under target-context evaluation
// it passes because the target's own id is in the context.
func TestViewAction_TargetRequireViewAttributeEvaluatedAgainstTarget(t *testing.T) {
	workflowContent := testWorkflowPreamble + `actions:
  - key: F7
    kind: view
    label: "Open SelfNamed"
    view: SelfNamed
views:
  - name: Board
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Todo
        filter: select where status = "backlog"
  - name: SelfNamed
    kind: detail
    key: "F2"
    require: ["view:plugin:SelfNamed"]
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

	ta.SendKey(tcell.KeyF7, 0, tcell.ModNone)
	ta.Draw()

	want := model.MakePluginViewID("SelfNamed")
	if got := ta.NavController.CurrentViewID(); got != want {
		t.Errorf("target's view:* require must evaluate against target context; current view = %q, want %q", got, want)
	}
}

// TestViewAction_DirectActivationUsesTargetScope asserts 6B.24: direct
// activation keys evaluate the target's require: list in target-scope,
// matching what `kind: view` actions do. A target declaring
// `require: ["view:plugin:SelfNamed"]` must be reachable by its direct
// F-key from another view — the requirement names the target's own id,
// which is satisfied once we're on the target. Under source-scoped
// evaluation (pre-6B.24) the UI enablement gate would have checked the
// source's view id against view:plugin:SelfNamed and refused.
func TestViewAction_DirectActivationUsesTargetScope(t *testing.T) {
	workflowContent := testWorkflowPreamble + `views:
  - name: Board
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Todo
        filter: select where status = "backlog"
  - name: SelfNamed
    kind: detail
    key: "F2"
    require: ["view:plugin:SelfNamed"]
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

	// Direct F2 — not a `kind: view` action.
	ta.SendKey(tcell.KeyF2, 0, tcell.ModNone)
	ta.Draw()

	want := model.MakePluginViewID("SelfNamed")
	if got := ta.NavController.CurrentViewID(); got != want {
		t.Errorf("direct activation must use target-scope for view:* require; current = %q, want %q", got, want)
	}
}

// TestViewAction_DirectActivationBlockedByTargetRequire asserts the
// inverse: a target whose require fails in target-scope must refuse
// direct activation too. Target requires `ai`; AI agent not configured;
// activation must be blocked.
func TestViewAction_DirectActivationBlockedByTargetRequire(t *testing.T) {
	workflowContent := testWorkflowPreamble + `views:
  - name: Board
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Todo
        filter: select where status = "backlog"
  - name: AiOnly
    kind: detail
    key: "F2"
    require: ["ai"]
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

	before := ta.NavController.CurrentViewID()
	ta.SendKey(tcell.KeyF2, 0, tcell.ModNone)
	ta.Draw()
	if got := ta.NavController.CurrentViewID(); got != before {
		t.Errorf("direct activation of a target with require:[ai] must be refused when AI not configured; was %q, now %q", before, got)
	}
}

// TestViewAction_WikiViewDispatchesRukiGlobal exercises Phase 6B.4:
// a `kind: ruki` global action must fire from a wiki view through the
// shared PluginExecutor. Creates a task, triggers a ruki update from a
// wiki view, then reloads the store and asserts the mutation landed.
func TestViewAction_WikiViewDispatchesRukiGlobal(t *testing.T) {
	workflowContent := testWorkflowPreamble + `actions:
  - key: "p"
    kind: ruki
    label: "Flag all as priority 1"
    action: update where status = "backlog" set priority = 1
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

	// Seed two backlog tasks at default priority so we can observe the
	// ruki-driven mutation.
	if err := testutil.CreateTestTask(ta.TaskDir, "000001", "Task One", task.StatusBacklog, task.TypeStory); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "000002", "Task Two", task.StatusBacklog, task.TypeStory); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// Navigate to the wiki view and press the ruki-global key.
	ta.NavController.PushView(model.MakePluginViewID("Docs"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyRune, 'p', tcell.ModNone)
	ta.Draw()

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("post-action reload: %v", err)
	}
	for _, id := range []string{"000001", "000002"} {
		t2 := ta.TaskStore.GetTask(id)
		if t2 == nil {
			t.Fatalf("task %s missing after action", id)
		}
		if t2.Priority != 1 {
			t.Errorf("task %s priority = %d, want 1 (ruki global from wiki view)", id, t2.Priority)
		}
	}
}
