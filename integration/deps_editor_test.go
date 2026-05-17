package integration

import (
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/testutil"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	"github.com/gdamore/tcell/v2"
)

// openDepsEditor navigates: Kanban → Enter (tiki detail) → Ctrl+D (deps editor)
// The tiki selected on the Kanban board becomes the context tiki.
func openDepsEditor(ta *testutil.TestApp) {
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	ta.Draw()
	ta.SendKey(tcell.KeyCtrlD, 0, tcell.ModCtrl)
	ta.Draw()
}

// TestDepsEditor_OpenFromTikiDetail verifies Ctrl+D on tiki detail pushes the deps plugin view.
func TestDepsEditor_OpenFromTikiDetail(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	contextID := "CTXA01"
	if err := testutil.CreateTestTiki(ta.TikiDir, contextID, "Context Tiki", "ready", "story"); err != nil {
		t.Fatalf("create context tiki: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, "FREE01", "Free Tiki", "ready", "story"); err != nil {
		t.Fatalf("create free tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	openDepsEditor(ta)

	wantViewID := model.MakePluginViewID("Dependency:" + contextID)
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

// TestDepsEditor_LanesShowCorrectTikis verifies each lane contains the expected tikis.
func TestDepsEditor_LanesShowCorrectTikis(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	contextID := "CTXA02"
	depID := "DEP002"
	blockerID := "BLK002"
	freeID := "FRE002"

	// context depends on dep; blocker depends on context
	if err := testutil.CreateTestTikiWithDeps(ta.TikiDir, contextID, "Context Tiki", "ready", "story", []string{depID}); err != nil {
		t.Fatalf("create context: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, depID, "Dep Tiki", "ready", "story"); err != nil {
		t.Fatalf("create dep: %v", err)
	}
	if err := testutil.CreateTestTikiWithDeps(ta.TikiDir, blockerID, "Blocker Tiki", "ready", "story", []string{contextID}); err != nil {
		t.Fatalf("create blocker: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, freeID, "Free Tiki", "ready", "story"); err != nil {
		t.Fatalf("create free: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// navigate to context tiki then open deps editor
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// navigate to the context tiki regardless of sort order
	if !ta.NavigateToTiki(contextID, 10) {
		ta.DumpScreen()
		t.Fatalf("context tiki %s not found on board", contextID)
	}

	ta.SendKey(tcell.KeyCtrlD, 0, tcell.ModCtrl)
	ta.Draw()

	// Blocker tiki belongs in Blocks lane (it depends on context)
	// Search by ID — titles may be truncated in narrow lanes
	if found, _, _ := ta.FindText(blockerID); !found {
		ta.DumpScreen()
		t.Errorf("Blocker tiki %s not visible (expected in Blocks lane)", blockerID)
	}

	// Dep tiki belongs in Depends lane (context depends on it)
	if found, _, _ := ta.FindText(depID); !found {
		ta.DumpScreen()
		t.Errorf("Dep tiki %s not visible (expected in Depends lane)", depID)
	}

	// Free tiki belongs in All lane
	if found, _, _ := ta.FindText(freeID); !found {
		ta.DumpScreen()
		t.Errorf("Free tiki %s not visible (expected in All lane)", freeID)
	}

	// Context tiki must not appear anywhere in the deps view
	if found, _, _ := ta.FindText("Context Tiki"); found {
		ta.DumpScreen()
		t.Errorf("Context Tiki should not be visible in deps editor")
	}
}

// TestDepsEditor_MoveTiki_AllToDepends_PersistsOnDisk verifies moving a tiki from All → Depends
// updates DependsOn in memory and persists on disk after reload.
func TestDepsEditor_MoveTiki_AllToDepends_PersistsOnDisk(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	contextID := "CTXA03"
	freeID := "FRE003"

	if err := testutil.CreateTestTiki(ta.TikiDir, contextID, "Context Tiki", "ready", "story"); err != nil {
		t.Fatalf("create context: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, freeID, "Free Tiki", "ready", "story"); err != nil {
		t.Fatalf("create free: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	openDepsEditor(ta)

	// verify we're in the deps editor
	wantViewID := model.MakePluginViewID("Dependency:" + contextID)
	if ta.NavController.CurrentView().ViewID != wantViewID {
		ta.DumpScreen()
		t.Fatalf("not in deps editor, got %v", ta.NavController.CurrentView().ViewID)
	}

	// Blocks lane is empty, so selection should land on All lane automatically.
	// Shift+Right moves selected tiki from All → Depends.
	ta.SendKey(tcell.KeyRight, 0, tcell.ModShift)
	ta.Draw()

	// verify in-memory state
	updated := ta.TikiStore.GetTiki(contextID)
	if updated == nil {
		t.Fatalf("context tiki not found in store")
		return
	}
	updatedDeps, _, _ := updated.StringSliceField(tikipkg.FieldDependsOn)
	found := false
	for _, d := range updatedDeps {
		if d == freeID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DependsOn = %v, want it to contain %s", updatedDeps, freeID)
	}

	// verify persisted to disk
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("reload after move: %v", err)
	}
	reloaded := ta.TikiStore.GetTiki(contextID)
	if reloaded == nil {
		t.Fatalf("context tiki not found after reload")
		return
	}
	reloadedDeps, _, _ := reloaded.StringSliceField(tikipkg.FieldDependsOn)
	foundAfterReload := false
	for _, d := range reloadedDeps {
		if d == freeID {
			foundAfterReload = true
			break
		}
	}
	if !foundAfterReload {
		t.Errorf("after reload: DependsOn = %v, want it to contain %s", reloadedDeps, freeID)
	}
}

// TestDepsEditor_MoveTiki_DependsToAll_RemovesDep verifies moving a tiki from Depends → All
// removes it from DependsOn in memory and on disk.
func TestDepsEditor_MoveTiki_DependsToAll_RemovesDep(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	contextID := "CTXA04"
	depID := "DEP004"
	freeID := "FRE004"

	if err := testutil.CreateTestTikiWithDeps(ta.TikiDir, contextID, "Context Tiki", "ready", "story", []string{depID}); err != nil {
		t.Fatalf("create context: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, depID, "Dep Tiki", "ready", "story"); err != nil {
		t.Fatalf("create dep: %v", err)
	}
	// a free tiki is needed so All lane is non-empty — handleLaneSwitch skips empty lanes,
	// so without it Shift+H from Depends has nowhere to land and becomes a no-op.
	if err := testutil.CreateTestTiki(ta.TikiDir, freeID, "Free Tiki", "ready", "story"); err != nil {
		t.Fatalf("create free: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	openDepsEditor(ta)

	wantViewID := model.MakePluginViewID("Dependency:" + contextID)
	if ta.NavController.CurrentView().ViewID != wantViewID {
		ta.DumpScreen()
		t.Fatalf("not in deps editor, got %v", ta.NavController.CurrentView().ViewID)
	}

	// EnsureFirstNonEmptyLaneSelection picks the first non-empty lane: Blocks is empty,
	// All has the free tiki, so selection starts on All (lane 1).
	// Navigate right once to reach Depends lane.
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)
	ta.Draw()

	// Shift+Left moves selected tiki from Depends → All
	ta.SendKey(tcell.KeyLeft, 0, tcell.ModShift)
	ta.Draw()

	// verify in-memory state
	updated := ta.TikiStore.GetTiki(contextID)
	if updated == nil {
		t.Fatalf("context tiki not found in store")
		return
	}
	updatedDeps, _, _ := updated.StringSliceField(tikipkg.FieldDependsOn)
	for _, d := range updatedDeps {
		if d == depID {
			t.Errorf("DependsOn = %v, should not contain %s after removal", updatedDeps, depID)
			break
		}
	}

	// verify persisted to disk
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("reload after move: %v", err)
	}
	reloaded := ta.TikiStore.GetTiki(contextID)
	if reloaded == nil {
		t.Fatalf("context tiki not found after reload")
		return
	}
	reloadedDeps, _, _ := reloaded.StringSliceField(tikipkg.FieldDependsOn)
	for _, d := range reloadedDeps {
		if d == depID {
			t.Errorf("after reload: DependsOn = %v, should not contain %s", reloadedDeps, depID)
			break
		}
	}
}

// TestDepsEditor_ReopenIsSameView verifies that opening the deps editor for the same tiki
// a second time reuses the existing plugin entry (idempotency).
func TestDepsEditor_ReopenIsSameView(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	contextID := "CTXA05"
	if err := testutil.CreateTestTiki(ta.TikiDir, contextID, "Context Tiki", "ready", "story"); err != nil {
		t.Fatalf("create context: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	wantViewID := model.MakePluginViewID("Dependency:" + contextID)

	// first open
	openDepsEditor(ta)
	if ta.NavController.CurrentView().ViewID != wantViewID {
		ta.DumpScreen()
		t.Fatalf("first open: not in deps editor, got %v", ta.NavController.CurrentView().ViewID)
	}

	// go back to detail view (Phase 3: configurable detail, not legacy TikiDetailViewID)
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
	ta.Draw()
	wantDetailID := model.DetailPluginViewID()
	if ta.NavController.CurrentView().ViewID != wantDetailID {
		ta.DumpScreen()
		t.Fatalf("expected configurable detail view %q after Esc, got %v",
			wantDetailID, ta.NavController.CurrentView().ViewID)
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
