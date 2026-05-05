package integration

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"
)

// TestConfigurableDetail_EnterOpensDetailView verifies the Phase 1 contract:
// pressing Enter on a board navigates to the workflow-declared Detail view
// (kind: detail) instead of the retired built-in TaskDetail. Sister assertions
// covering the legacy Enter→TaskDetail path live in tests Phase 3 will retire.
func TestConfigurableDetail_EnterOpensDetailView(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	// Land on the Backlog board so Enter has a selection-eligible context.
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)
	ta.Draw()

	startDepth := ta.NavController.Depth()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	ta.Draw()

	if ta.NavController.Depth() <= startDepth {
		t.Fatalf("Enter did not push a new view (depth %d → %d)", startDepth, ta.NavController.Depth())
	}
	want := model.MakePluginViewID("Detail")
	if got := ta.NavController.CurrentViewID(); got != want {
		t.Errorf("current view = %q, want %q", got, want)
	}
}

// TestConfigurableDetail_EnterOnDetailDoesNotRecurse asserts the bug fix
// for the self-targeting Enter global. Once the user is already on the
// Detail view, pressing Enter must not push another Detail copy and must
// not change the navigation depth. The merge-time loader filter plus the
// dispatch-time guard in DetailController together cover this.
func TestConfigurableDetail_EnterOnDetailDoesNotRecurse(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	taskID := "DETL02"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Detail Recurse", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	params := model.EncodePluginViewParams(model.PluginViewParams{TaskID: taskID})
	ta.NavController.PushView(model.MakePluginViewID("Detail"), params)
	ta.Draw()

	startDepth := ta.NavController.Depth()
	startView := ta.NavController.CurrentViewID()

	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	ta.Draw()

	if got := ta.NavController.Depth(); got != startDepth {
		t.Errorf("depth changed after Enter on Detail: %d → %d (expected no-op)", startDepth, got)
	}
	if got := ta.NavController.CurrentViewID(); got != startView {
		t.Errorf("view changed after Enter on Detail: %q → %q", startView, got)
	}
}

// TestConfigurableDetail_FreshControllerPerNavigation guards against the
// shared-singleton bug where a second Detail navigation would overwrite the
// first view's selectedTaskID. Pushing Detail twice for two different tikis
// must leave both controllers carrying their own selection.
func TestConfigurableDetail_FreshControllerPerNavigation(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	if err := testutil.CreateTestTask(ta.TaskDir, "FRSH01", "First", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create first: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "FRSH02", "Second", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create second: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// First navigation
	ta.NavController.PushView(model.MakePluginViewID("Detail"),
		model.EncodePluginViewParams(model.PluginViewParams{TaskID: "FRSH01"}))
	ta.Draw()
	firstView := ta.NavController.GetActiveView()

	// Second navigation — different task. Without fresh-per-navigation, this
	// would mutate the shared controller and the firstView's selection would
	// silently flip to FRSH02.
	ta.NavController.PushView(model.MakePluginViewID("Detail"),
		model.EncodePluginViewParams(model.PluginViewParams{TaskID: "FRSH02"}))
	ta.Draw()
	secondView := ta.NavController.GetActiveView()

	type selector interface{ GetSelectedID() string }
	first, ok1 := firstView.(selector)
	second, ok2 := secondView.(selector)
	if !ok1 || !ok2 {
		t.Fatalf("views do not implement SelectableView (first=%v second=%v)", ok1, ok2)
	}
	if first.GetSelectedID() != "FRSH01" {
		t.Errorf("first view selection = %q, want %q (shared-singleton regression)", first.GetSelectedID(), "FRSH01")
	}
	if second.GetSelectedID() != "FRSH02" {
		t.Errorf("second view selection = %q, want %q", second.GetSelectedID(), "FRSH02")
	}
}

// TestConfigurableDetail_RendersConfiguredMetadata draws the Detail view and
// asserts the configured Status/Type/Priority labels appear on screen, plus
// the title from the selected tiki. Catches accidental renderer drift.
func TestConfigurableDetail_RendersConfiguredMetadata(t *testing.T) {
	ta := setupTestAppWithPlugins(t)
	defer ta.Cleanup()

	taskID := "DETL01"
	if err := testutil.CreateTestTask(ta.TaskDir, taskID, "Detail Title", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// Push directly to Detail with the task carried as PluginViewParams,
	// matching what Enter dispatch would do via selection passthrough.
	params := model.EncodePluginViewParams(model.PluginViewParams{TaskID: taskID})
	ta.NavController.PushView(model.MakePluginViewID("Detail"), params)
	ta.Draw()

	for _, want := range []string{"Detail Title", "Status:", "Type:", "Priority:"} {
		found, _, _ := ta.FindText(want)
		if !found {
			ta.DumpScreen()
			t.Errorf("expected %q on screen", want)
		}
	}
}
