package controller

import (
	"fmt"
	"slices"
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

const (
	testCtxID  = "AACTX0"
	testBlkID  = "AABLK0"
	testDepID  = "AADEP0"
	testFreeID = "AAFRE0"
)

// newWorkflowTiki creates a workflow tiki with the given id/title/dependsOn for test seeding.
func newWorkflowTiki(id, title string, dependsOn []string) *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.ID = id
	tk.Title = title
	tk.Set("status", "ready")
	tk.Set("type", "story")
	tk.Set("priority", "medium")
	if len(dependsOn) > 0 {
		tk.Set(tikipkg.FieldDependsOn, dependsOn)
	}
	return tk
}

// newDepsTestEnv sets up a deps editor test environment with:
// - contextTiki whose dependsOn contains testDepID
// - blockerTiki whose dependsOn contains testCtxID
// - dependsTiki with no deps
// - freeTiki with no dependency relationship
func newDepsTestEnv(t *testing.T) (*DepsController, store.Store) {
	t.Helper()
	tikiStore := store.NewInMemoryStore()

	tikis := []*tikipkg.Tiki{
		newWorkflowTiki(testCtxID, "Context", []string{testDepID}),
		newWorkflowTiki(testBlkID, "Blocker", []string{testCtxID}),
		newWorkflowTiki(testDepID, "Depends", nil),
		newWorkflowTiki(testFreeID, "Free", nil),
	}
	for _, tk := range tikis {
		if err := tikiStore.CreateTiki(tk); err != nil {
			t.Fatalf("create tiki %s: %v", tk.ID, err)
		}
	}

	pluginDef := &plugin.WorkflowPlugin{
		BasePlugin: plugin.BasePlugin{Name: "Dependency:" + testCtxID, ConfigIndex: -1, Kind: plugin.KindBoard},
		TikiID:     testCtxID,
		Lanes:      []plugin.TikiLane{{Name: "Blocks"}, {Name: "All"}, {Name: "Depends"}},
	}
	pluginConfig := model.NewPluginConfig("Dependency")
	pluginConfig.SetLaneLayout([]int{1, 2, 1}, nil)

	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)

	nav := newMockNavigationController()
	// Tests in this file that exercise the Open action assert the
	// pushed view id, so the resolver returns the bundled default
	// detail view name. Tests that don't dispatch Open still get a
	// resolver — passing nil would refuse the open and obscure other
	// failures.
	resolver := func() string { return model.DetailPluginName }
	dc := NewDepsController(tikiStore, gate, pluginConfig, pluginDef, nav, nil, rukiRuntime.NewSchema(), resolver)
	return dc, tikiStore
}

func tikiIDs(tikis []*tikipkg.Tiki) []string {
	ids := make([]string, len(tikis))
	for i, tk := range tikis {
		ids[i] = tk.ID
	}
	return ids
}

func TestDepsController_GetFilteredTikisForLane(t *testing.T) {
	dc, _ := newDepsTestEnv(t)

	t.Run("all lane excludes context, blocks, and depends", func(t *testing.T) {
		all := dc.GetFilteredTikisForLane(depsLaneAll)
		ids := tikiIDs(all)
		if slices.Contains(ids, testCtxID) {
			t.Error("all lane should not contain context tiki")
		}
		if slices.Contains(ids, testBlkID) {
			t.Error("all lane should not contain blocker tiki")
		}
		if slices.Contains(ids, testDepID) {
			t.Error("all lane should not contain depends tiki")
		}
		if !slices.Contains(ids, testFreeID) {
			t.Error("all lane should contain free tiki")
		}
	})

	t.Run("blocks lane contains tikis that depend on context", func(t *testing.T) {
		blocks := dc.GetFilteredTikisForLane(depsLaneBlocks)
		ids := tikiIDs(blocks)
		if !slices.Contains(ids, testBlkID) {
			t.Error("blocks lane should contain blocker tiki")
		}
		if len(ids) != 1 {
			t.Errorf("blocks lane should have exactly 1 tiki, got %d: %v", len(ids), ids)
		}
	})

	t.Run("depends lane contains context tiki dependencies", func(t *testing.T) {
		depends := dc.GetFilteredTikisForLane(depsLaneDepends)
		ids := tikiIDs(depends)
		if !slices.Contains(ids, testDepID) {
			t.Error("depends lane should contain depends tiki")
		}
		if len(ids) != 1 {
			t.Errorf("depends lane should have exactly 1 tiki, got %d: %v", len(ids), ids)
		}
	})

	t.Run("invalid lane returns nil", func(t *testing.T) {
		if dc.GetFilteredTikisForLane(-1) != nil {
			t.Error("invalid lane should return nil")
		}
		if dc.GetFilteredTikisForLane(3) != nil {
			t.Error("out of range lane should return nil")
		}
	})
}

func TestDepsController_MoveTiki_AllToBlocks(t *testing.T) {
	dc, tikiStore := newDepsTestEnv(t)

	dc.pluginConfig.SetSelectedLane(depsLaneAll)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)

	if !dc.handleMoveTiki(-1) {
		t.Fatal("move should succeed")
	}

	// free tiki should now have context tiki in its dependsOn
	free := tikiStore.GetTiki(testFreeID)
	freeDeps, _, _ := free.StringSliceField(tikipkg.FieldDependsOn)
	if !slices.Contains(freeDeps, testCtxID) {
		t.Errorf("free.DependsOn should contain %s, got %v", testCtxID, freeDeps)
	}
}

func TestDepsController_MoveTiki_AllToDepends(t *testing.T) {
	dc, tikiStore := newDepsTestEnv(t)

	dc.pluginConfig.SetSelectedLane(depsLaneAll)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)

	if !dc.handleMoveTiki(1) {
		t.Fatal("move should succeed")
	}

	// context tiki should now have free tiki in its dependsOn
	ctx := tikiStore.GetTiki(testCtxID)
	ctxDeps, _, _ := ctx.StringSliceField(tikipkg.FieldDependsOn)
	if !slices.Contains(ctxDeps, testFreeID) {
		t.Errorf("ctx.DependsOn should contain %s, got %v", testFreeID, ctxDeps)
	}
}

func TestDepsController_MoveTiki_BlocksToAll(t *testing.T) {
	dc, tikiStore := newDepsTestEnv(t)

	dc.pluginConfig.SetSelectedLane(depsLaneBlocks)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneBlocks, 0)

	if !dc.handleMoveTiki(1) {
		t.Fatal("move should succeed")
	}

	blk := tikiStore.GetTiki(testBlkID)
	blkDeps, _, _ := blk.StringSliceField(tikipkg.FieldDependsOn)
	if slices.Contains(blkDeps, testCtxID) {
		t.Errorf("blk.DependsOn should not contain %s after move, got %v", testCtxID, blkDeps)
	}
}

func TestDepsController_MoveTiki_DependsToAll(t *testing.T) {
	dc, tikiStore := newDepsTestEnv(t)

	dc.pluginConfig.SetSelectedLane(depsLaneDepends)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneDepends, 0)

	if !dc.handleMoveTiki(-1) {
		t.Fatal("move should succeed")
	}

	ctx := tikiStore.GetTiki(testCtxID)
	ctxDeps, _, _ := ctx.StringSliceField(tikipkg.FieldDependsOn)
	if slices.Contains(ctxDeps, testDepID) {
		t.Errorf("ctx.DependsOn should not contain %s after move, got %v", testDepID, ctxDeps)
	}
}

func TestDepsController_MoveTiki_OutOfBounds(t *testing.T) {
	dc, _ := newDepsTestEnv(t)

	dc.pluginConfig.SetSelectedLane(depsLaneBlocks)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneBlocks, 0)

	if dc.handleMoveTiki(-1) {
		t.Error("move left from lane 0 should fail")
	}

	dc.pluginConfig.SetSelectedLane(depsLaneDepends)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneDepends, 0)

	if dc.handleMoveTiki(1) {
		t.Error("move right from lane 2 should fail")
	}
}

func TestDepsController_MoveTiki_RejectsMultiLaneJump(t *testing.T) {
	dc, _ := newDepsTestEnv(t)

	dc.pluginConfig.SetSelectedLane(depsLaneBlocks)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneBlocks, 0)

	if dc.handleMoveTiki(2) {
		t.Error("offset=2 should be rejected")
	}
	if dc.handleMoveTiki(-2) {
		t.Error("offset=-2 should be rejected")
	}
}

func TestDepsController_HandleSearch(t *testing.T) {
	t.Run("empty query is no-op", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.HandleSearch("")
		if dc.pluginConfig.GetSearchResults() != nil {
			t.Error("empty query should not set search results")
		}
	})

	t.Run("matching query", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.HandleSearch("Free")
		results := dc.pluginConfig.GetSearchResults()
		if results == nil {
			t.Fatal("expected search results, got nil")
		}
		found := false
		for _, sr := range results {
			if sr.ID == testFreeID {
				found = true
			}
		}
		if !found {
			t.Errorf("expected search results to contain %s", testFreeID)
		}
	})

	t.Run("non-matching query", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.HandleSearch("zzzzz")
		results := dc.pluginConfig.GetSearchResults()
		if results == nil {
			t.Fatal("expected empty search results slice, got nil")
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

func TestDepsController_HandleAction(t *testing.T) {
	t.Run("nav down changes index", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)
		// All lane has only free tiki, so nav down should return false (can't go past end)
		dc.HandleAction(ActionNavDown)
		// just verify it doesn't panic and returns a bool
	})

	t.Run("nav left from All switches to Blocks", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)
		result := dc.HandleAction(ActionNavLeft)
		if !result {
			t.Error("nav left from All should succeed (Blocks has tikis)")
		}
		if dc.pluginConfig.GetSelectedLane() != depsLaneBlocks {
			t.Errorf("expected lane %d, got %d", depsLaneBlocks, dc.pluginConfig.GetSelectedLane())
		}
	})

	t.Run("nav right from All switches to Depends", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)
		result := dc.HandleAction(ActionNavRight)
		if !result {
			t.Error("nav right from All should succeed (Depends has tikis)")
		}
		if dc.pluginConfig.GetSelectedLane() != depsLaneDepends {
			t.Errorf("expected lane %d, got %d", depsLaneDepends, dc.pluginConfig.GetSelectedLane())
		}
	})

	t.Run("open tiki pushes configurable detail view", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)
		result := dc.HandleAction(ActionOpenFromPlugin)
		if !result {
			t.Error("open should succeed when a tiki is selected")
		}
		top := dc.navController.navState.currentView()
		if top == nil || top.ViewID != model.DetailPluginViewID() {
			t.Errorf("expected configurable detail view %q to be pushed, got %v",
				model.DetailPluginViewID(), top)
		}
	})

	t.Run("open tiki uses resolver-returned name (handles renamed detail view)", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		// override the resolver to a custom workflow-renamed view name
		dc.detailViewResolver = func() string { return "MyCustomDetail" }
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)

		if !dc.HandleAction(ActionOpenFromPlugin) {
			t.Fatal("open should succeed when resolver returns a non-empty name")
		}
		top := dc.navController.navState.currentView()
		want := model.MakePluginViewID("MyCustomDetail")
		if top == nil || top.ViewID != want {
			t.Errorf("expected resolver-returned view %q to be pushed, got %v", want, top)
		}
	})

	t.Run("open tiki refuses when resolver returns empty (no detail view loaded)", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.detailViewResolver = func() string { return "" }
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)
		startDepth := dc.navController.navState.depth()

		if dc.HandleAction(ActionOpenFromPlugin) {
			t.Error("open should refuse when the resolver returns the empty string")
		}
		if dc.navController.navState.depth() != startDepth {
			t.Error("nav stack should not have grown after refused open")
		}
	})

	t.Run("open tiki refuses when resolver is nil", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.detailViewResolver = nil
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)
		startDepth := dc.navController.navState.depth()

		if dc.HandleAction(ActionOpenFromPlugin) {
			t.Error("open should refuse when no resolver is configured")
		}
		if dc.navController.navState.depth() != startDepth {
			t.Error("nav stack should not have grown after refused open")
		}
	})

	t.Run("new tiki pushes edit view", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		// handleNewTiki bails when no detail spec is registered; install a
		// minimal stub so the success path runs.
		stubSpec := gridlayout.GridSpec{
			Rows: 1, Cols: 1,
			Anchors:   []gridlayout.Anchor{{Name: "title", Row: 0, Col: 0, RowSpan: 1, ColSpan: 1}},
			Stretcher: []bool{false},
			Cells:     [][]gridlayout.Cell{{gridlayout.FieldCell{Name: "title"}}},
		}
		SetDetailSpecSource(func() (gridlayout.GridSpec, bool) { return stubSpec, true })
		defer SetDetailSpecSource(nil)

		result := dc.HandleAction(ActionNewTiki)
		if !result {
			t.Error("new tiki should succeed")
		}
		top := dc.navController.navState.currentView()
		if top == nil || top.ViewID != model.TikiEditViewID {
			t.Error("expected TikiEditViewID to be pushed")
		}
	})

	t.Run("delete tiki removes from store", func(t *testing.T) {
		dc, tikiStore := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)

		// free tiki should be in the All lane
		allTikis := dc.GetFilteredTikisForLane(depsLaneAll)
		if len(allTikis) == 0 {
			t.Fatal("expected at least one tiki in All lane")
		}
		deletedID := allTikis[0].ID

		result := dc.HandleAction(ActionDeleteTiki)
		if !result {
			t.Error("delete should succeed when a tiki is selected")
		}
		if tikiStore.GetTiki(deletedID) != nil {
			t.Errorf("tiki %s should have been deleted", deletedID)
		}
	})

	t.Run("invalid action returns false", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		if dc.HandleAction("nonexistent_action") {
			t.Error("unknown action should return false")
		}
	})
}

func TestDepsController_HandleLaneSwitch(t *testing.T) {
	t.Run("right from Blocks lands on All", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneBlocks)
		result := dc.handleLaneSwitch("right", dc.GetFilteredTikisForLane)
		if !result {
			t.Error("should succeed — All lane has tikis")
		}
		if dc.pluginConfig.GetSelectedLane() != depsLaneAll {
			t.Errorf("expected lane %d, got %d", depsLaneAll, dc.pluginConfig.GetSelectedLane())
		}
	})

	t.Run("left from All lands on Blocks", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		result := dc.handleLaneSwitch("left", dc.GetFilteredTikisForLane)
		if !result {
			t.Error("should succeed — Blocks lane has tikis")
		}
		if dc.pluginConfig.GetSelectedLane() != depsLaneBlocks {
			t.Errorf("expected lane %d, got %d", depsLaneBlocks, dc.pluginConfig.GetSelectedLane())
		}
	})

	t.Run("left from Blocks returns false (boundary)", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneBlocks)
		if dc.handleLaneSwitch("left", dc.GetFilteredTikisForLane) {
			t.Error("should fail — no lane to the left of Blocks")
		}
	})

	t.Run("right from Depends returns false (boundary)", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneDepends)
		if dc.handleLaneSwitch("right", dc.GetFilteredTikisForLane) {
			t.Error("should fail — no lane to the right of Depends")
		}
	})
}

func TestDepsController_EnsureFirstNonEmptyLaneSelection(t *testing.T) {
	t.Run("current lane has tikis — no change", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)
		if dc.EnsureFirstNonEmptyLaneSelection() {
			t.Error("should return false when current lane has tikis")
		}
		if dc.pluginConfig.GetSelectedLane() != depsLaneAll {
			t.Error("lane should not change")
		}
	})

	t.Run("current lane empty — switches to first non-empty", func(t *testing.T) {
		dc, tikiStore := newDepsTestEnv(t)
		// move free tiki into depends so All lane becomes empty
		free := tikiStore.GetTiki(testFreeID).Clone()
		free.Set(tikipkg.FieldDependsOn, []string{testCtxID})
		if err := tikiStore.UpdateTiki(free); err != nil {
			t.Fatal(err)
		}

		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		result := dc.EnsureFirstNonEmptyLaneSelection()
		if !result {
			t.Error("should return true when lane was empty and switch occurred")
		}
		newLane := dc.pluginConfig.GetSelectedLane()
		if newLane == depsLaneAll {
			t.Error("should have switched away from empty All lane")
		}
	})
}

func TestDepsController_DeleteTiki_GateError(t *testing.T) {
	// when gate rejects delete, handleDeleteTiki should return false
	tikiStore := store.NewInMemoryStore()

	for _, tk := range []*tikipkg.Tiki{
		newWorkflowTiki(testCtxID, "Context", []string{testDepID}),
		newWorkflowTiki(testBlkID, "Blocker", []string{testCtxID}),
		newWorkflowTiki(testDepID, "Depends", nil),
		newWorkflowTiki(testFreeID, "Free", nil),
	} {
		if err := tikiStore.CreateTiki(tk); err != nil {
			t.Fatalf("create tiki %s: %v", tk.ID, err)
		}
	}

	pluginDef := &plugin.WorkflowPlugin{
		BasePlugin: plugin.BasePlugin{Name: "Dependency:" + testCtxID, ConfigIndex: -1, Kind: plugin.KindBoard},
		TikiID:     testCtxID,
		Lanes:      []plugin.TikiLane{{Name: "Blocks"}, {Name: "All"}, {Name: "Depends"}},
	}
	pluginConfig := model.NewPluginConfig("Dependency")
	pluginConfig.SetLaneLayout([]int{1, 2, 1}, nil)

	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	// register a before-delete validator that blocks all deletes
	gate.OnDelete(func(old, new *tikipkg.Tiki, allTikis []*tikipkg.Tiki) *service.Rejection {
		return &service.Rejection{Reason: "deletes blocked for test"}
	})

	nav := newMockNavigationController()
	statusline := model.NewStatuslineConfig()
	dc := NewDepsController(tikiStore, gate, pluginConfig, pluginDef, nav, statusline, rukiRuntime.NewSchema(), nil)

	// select free tiki in All lane
	dc.pluginConfig.SetSelectedLane(depsLaneAll)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)

	result := dc.HandleAction(ActionDeleteTiki)
	if result {
		t.Error("expected delete to fail when gate rejects")
	}
}

func TestDepsController_MoveTiki_UpdateError(t *testing.T) {
	// when gate rejects the update, statusline should receive the error
	tikiStore := store.NewInMemoryStore()

	for _, tk := range []*tikipkg.Tiki{
		newWorkflowTiki(testCtxID, "Context", []string{testDepID}),
		newWorkflowTiki(testBlkID, "Blocker", []string{testCtxID}),
		newWorkflowTiki(testDepID, "Depends", nil),
		newWorkflowTiki(testFreeID, "Free", nil),
	} {
		if err := tikiStore.CreateTiki(tk); err != nil {
			t.Fatalf("create tiki %s: %v", tk.ID, err)
		}
	}

	pluginDef := &plugin.WorkflowPlugin{
		BasePlugin: plugin.BasePlugin{Name: "Dependency:" + testCtxID, ConfigIndex: -1, Kind: plugin.KindBoard},
		TikiID:     testCtxID,
		Lanes:      []plugin.TikiLane{{Name: "Blocks"}, {Name: "All"}, {Name: "Depends"}},
	}
	pluginConfig := model.NewPluginConfig("Dependency")
	pluginConfig.SetLaneLayout([]int{1, 2, 1}, nil)

	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	// register a validator that blocks all updates
	gate.OnUpdate(func(old, new *tikipkg.Tiki, allTikis []*tikipkg.Tiki) *service.Rejection {
		return &service.Rejection{Reason: "updates blocked for test"}
	})

	nav := newMockNavigationController()
	statusline := model.NewStatuslineConfig()
	dc := NewDepsController(tikiStore, gate, pluginConfig, pluginDef, nav, statusline, rukiRuntime.NewSchema(), nil)

	// select free tiki in All lane, move left → Blocks
	dc.pluginConfig.SetSelectedLane(depsLaneAll)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)

	result := dc.handleMoveTiki(-1)
	if result {
		t.Error("expected move to fail when gate rejects update")
	}

	// statusline should have received the error
	msg, _, _ := statusline.GetMessage()
	if msg == "" {
		t.Error("expected statusline to have error message")
	}
}

func TestDepsViewActions(t *testing.T) {
	registry := DepsViewActions()
	actions := registry.GetActions()

	required := map[ActionID]bool{
		ActionNavUp:          false,
		ActionNavDown:        false,
		ActionMoveTikiLeft:   false,
		ActionMoveTikiRight:  false,
		ActionOpenFromPlugin: false,
		ActionNewTiki:        false,
		ActionDeleteTiki:     false,
		ActionSearch:         false,
	}
	for _, a := range actions {
		if _, ok := required[a.ID]; ok {
			required[a.ID] = true
		}
	}
	for id, found := range required {
		if !found {
			t.Errorf("DepsViewActions missing required action %s", id)
		}
	}
}

func newDepsNavEnv(t *testing.T, blockers int, allTikis int, depends int, laneColumns []int) *DepsController {
	t.Helper()

	tikiStore := store.NewInMemoryStore()
	contextID := "CTXNAV0"
	contextDepends := make([]string, 0, depends)
	for i := 0; i < depends; i++ {
		contextDepends = append(contextDepends, fmt.Sprintf("DEP%04d", i))
	}
	if err := tikiStore.CreateTiki(newWorkflowTiki(contextID, "Context", contextDepends)); err != nil {
		t.Fatalf("create context: %v", err)
	}

	for i := 0; i < depends; i++ {
		if err := tikiStore.CreateTiki(newWorkflowTiki(fmt.Sprintf("DEP%04d", i), "Depends", nil)); err != nil {
			t.Fatalf("create depends tiki: %v", err)
		}
	}

	for i := 0; i < blockers; i++ {
		if err := tikiStore.CreateTiki(newWorkflowTiki(fmt.Sprintf("BLK%04d", i), "Blocker", []string{contextID})); err != nil {
			t.Fatalf("create blocker tiki: %v", err)
		}
	}

	for i := 0; i < allTikis; i++ {
		if err := tikiStore.CreateTiki(newWorkflowTiki(fmt.Sprintf("ALL%04d", i), "All", nil)); err != nil {
			t.Fatalf("create all lane tiki: %v", err)
		}
	}

	pluginDef := &plugin.WorkflowPlugin{
		BasePlugin: plugin.BasePlugin{Name: "Dependency:" + contextID, ConfigIndex: -1, Kind: plugin.KindBoard},
		TikiID:     contextID,
		Lanes:      []plugin.TikiLane{{Name: "Blocks"}, {Name: "All"}, {Name: "Depends"}},
	}
	pluginConfig := model.NewPluginConfig("Dependency")
	pluginConfig.SetLaneLayout(laneColumns, nil)

	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)

	nav := newMockNavigationController()
	return NewDepsController(tikiStore, gate, pluginConfig, pluginDef, nav, nil, rukiRuntime.NewSchema(), nil)
}

func TestDepsController_NavRightAdjacentNonEmptyPreservesRow(t *testing.T) {
	dc := newDepsNavEnv(t, 2, 4, 3, []int{1, 2, 1})
	dc.pluginConfig.SetSelectedLane(depsLaneAll)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 3) // row 1, col 1
	dc.pluginConfig.SetScrollOffsetForLane(depsLaneAll, 1)  // row offset in viewport = 0
	dc.pluginConfig.SetScrollOffsetForLane(depsLaneDepends, 1)

	if !dc.HandleAction(ActionNavRight) {
		t.Fatal("expected nav right to succeed")
	}
	if got := dc.pluginConfig.GetSelectedLane(); got != depsLaneDepends {
		t.Fatalf("expected lane %d, got %d", depsLaneDepends, got)
	}
	if got := dc.pluginConfig.GetSelectedIndexForLane(depsLaneDepends); got != 1 {
		t.Fatalf("expected selected index 1, got %d", got)
	}
}

func TestDepsController_NavLeftAdjacentNonEmptyLandsRightmostPartial(t *testing.T) {
	dc := newDepsNavEnv(t, 6, 4, 2, []int{4, 2, 1})
	dc.pluginConfig.SetSelectedLane(depsLaneAll)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 2) // row 1, col 0
	dc.pluginConfig.SetScrollOffsetForLane(depsLaneAll, 1)  // row offset in viewport = 0
	dc.pluginConfig.SetScrollOffsetForLane(depsLaneBlocks, 1)

	if !dc.HandleAction(ActionNavLeft) {
		t.Fatal("expected nav left to succeed")
	}
	if got := dc.pluginConfig.GetSelectedLane(); got != depsLaneBlocks {
		t.Fatalf("expected lane %d, got %d", depsLaneBlocks, got)
	}
	// lane 0 has 6 tikis with 4 columns; row 1 is partial => index 5
	if got := dc.pluginConfig.GetSelectedIndexForLane(depsLaneBlocks); got != 5 {
		t.Fatalf("expected selected index 5, got %d", got)
	}
}

func TestDepsController_NavSkipEmptyKeepsTraversalAndLandsByTargetViewport(t *testing.T) {
	dc := newDepsNavEnv(t, 3, 0, 2, []int{1, 2, 1})
	dc.pluginConfig.SetSelectedLane(depsLaneBlocks)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneBlocks, 2)
	dc.pluginConfig.SetScrollOffsetForLane(depsLaneDepends, 1)

	if !dc.HandleAction(ActionNavRight) {
		t.Fatal("expected nav right to skip empty all lane and succeed")
	}
	if got := dc.pluginConfig.GetSelectedLane(); got != depsLaneDepends {
		t.Fatalf("expected lane %d, got %d", depsLaneDepends, got)
	}
	if got := dc.pluginConfig.GetSelectedIndexForLane(depsLaneDepends); got != 1 {
		t.Fatalf("expected selected index 1 from depends viewport row, got %d", got)
	}
}

func TestDepsController_VerticalStaleIndexRecoveryIsShared(t *testing.T) {
	dc := newDepsNavEnv(t, 1, 1, 1, []int{1, 2, 1})
	dc.pluginConfig.SetSelectedLane(depsLaneAll)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 99)

	callbacks := 0
	listenerID := dc.pluginConfig.AddSelectionListener(func() { callbacks++ })
	defer dc.pluginConfig.RemoveSelectionListener(listenerID)

	if !dc.HandleAction(ActionNavDown) {
		t.Fatal("expected stale vertical action to heal selection")
	}
	if got := dc.pluginConfig.GetSelectedIndexForLane(depsLaneAll); got != 0 {
		t.Fatalf("expected healed index 0, got %d", got)
	}
	if callbacks != 1 {
		t.Fatalf("expected 1 selection callback, got %d", callbacks)
	}
}

func TestDepsController_SuccessfulSwitchPersistsClampedTargetScroll(t *testing.T) {
	dc := newDepsNavEnv(t, 2, 3, 1, []int{1, 2, 1})
	dc.pluginConfig.SetSelectedLane(depsLaneBlocks)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneBlocks, 0)
	dc.pluginConfig.SetScrollOffsetForLane(depsLaneAll, 99)

	if !dc.HandleAction(ActionNavRight) {
		t.Fatal("expected nav right to succeed")
	}
	if got := dc.pluginConfig.GetSelectedLane(); got != depsLaneAll {
		t.Fatalf("expected lane %d, got %d", depsLaneAll, got)
	}
	// all lane has 3 tikis with 2 columns => max row 1, row-start index 2
	if got := dc.pluginConfig.GetSelectedIndexForLane(depsLaneAll); got != 2 {
		t.Fatalf("expected selected index 2, got %d", got)
	}
	if got := dc.pluginConfig.GetScrollOffsetForLane(depsLaneAll); got != 1 {
		t.Fatalf("expected clamped scroll offset 1, got %d", got)
	}
}

func TestDepsController_ShowNavigation(t *testing.T) {
	dc, _ := newDepsTestEnv(t)
	if dc.ShowNavigation() {
		t.Error("DepsController.ShowNavigation() should return false")
	}
}

func TestDepsController_GetFilteredTikisForLane_WithSearch(t *testing.T) {
	dc, tikiStore := newDepsTestEnv(t)

	// set search results to only include the free tiki
	free := tikiStore.GetTiki(testFreeID)
	dc.pluginConfig.SetSearchResults([]*tikipkg.Tiki{free}, "Free")

	// All lane should now only show the free tiki (matching search)
	allTikis := dc.GetFilteredTikisForLane(depsLaneAll)
	if len(allTikis) != 1 {
		t.Fatalf("expected 1 tiki with search narrowing, got %d", len(allTikis))
	}
	if allTikis[0].ID != testFreeID {
		t.Errorf("expected %s, got %s", testFreeID, allTikis[0].ID)
	}

	// Blocks lane should be empty (no matching search results)
	blocksTikis := dc.GetFilteredTikisForLane(depsLaneBlocks)
	if len(blocksTikis) != 0 {
		t.Errorf("expected 0 blocks tikis with search narrowing, got %d", len(blocksTikis))
	}
}

func TestDepsController_GetFilteredTikisForLane_MissingContextTiki(t *testing.T) {
	dc, tikiStore := newDepsTestEnv(t)

	// delete the context tiki
	tikiStore.DeleteTiki(testCtxID)

	// all lanes should return nil when context tiki is missing
	if dc.GetFilteredTikisForLane(depsLaneAll) != nil {
		t.Error("expected nil when context tiki is missing")
	}
	if dc.GetFilteredTikisForLane(depsLaneBlocks) != nil {
		t.Error("expected nil when context tiki is missing")
	}
	if dc.GetFilteredTikisForLane(depsLaneDepends) != nil {
		t.Error("expected nil when context tiki is missing")
	}
}

func TestDepsController_MoveTiki_EmptySelection(t *testing.T) {
	dc, _ := newDepsTestEnv(t)

	// set selected lane to blocks but with an index beyond the tiki list
	dc.pluginConfig.SetSelectedLane(depsLaneBlocks)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneBlocks, 99) // beyond available tikis

	// getSelectedTikiID should return "" for an index beyond the tiki list,
	// so handleMoveTiki should return false
	if dc.handleMoveTiki(1) {
		t.Error("expected false when no tiki is selected (index out of range)")
	}
}
