package controller

import (
	"fmt"
	"slices"
	"testing"

	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

const (
	testCtxID  = "AACTX0"
	testBlkID  = "AABLK0"
	testDepID  = "AADEP0"
	testFreeID = "AAFRE0"
)

// newDepsTestEnv sets up a deps editor test environment with:
// - contextTask whose dependsOn contains testDepID
// - blockerTask whose dependsOn contains testCtxID
// - dependsTask with no deps
// - freeTask with no dependency relationship
func newDepsTestEnv(t *testing.T) (*DepsController, store.Store) {
	t.Helper()
	taskStore := store.NewInMemoryStore()

	tasks := []*task.Task{
		{ID: testCtxID, Title: "Context", Status: task.StatusReady, Type: task.TypeStory, Priority: 3, DependsOn: []string{testDepID}, IsWorkflow: true},
		{ID: testBlkID, Title: "Blocker", Status: task.StatusReady, Type: task.TypeStory, Priority: 3, DependsOn: []string{testCtxID}, IsWorkflow: true},
		{ID: testDepID, Title: "Depends", Status: task.StatusReady, Type: task.TypeStory, Priority: 3, IsWorkflow: true},
		{ID: testFreeID, Title: "Free", Status: task.StatusReady, Type: task.TypeStory, Priority: 3, IsWorkflow: true},
	}
	for _, tt := range tasks {
		if err := taskStore.CreateTask(tt); err != nil {
			t.Fatalf("create task %s: %v", tt.ID, err)
		}
	}

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "Dependency:" + testCtxID, ConfigIndex: -1, Kind: plugin.KindBoard},
		TaskID:     testCtxID,
		Lanes:      []plugin.TikiLane{{Name: "Blocks"}, {Name: "All"}, {Name: "Depends"}},
	}
	pluginConfig := model.NewPluginConfig("Dependency")
	pluginConfig.SetLaneLayout([]int{1, 2, 1}, nil)

	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)

	nav := newMockNavigationController()
	dc := NewDepsController(taskStore, gate, pluginConfig, pluginDef, nav, nil, rukiRuntime.NewSchema())
	return dc, taskStore
}

func taskIDs(tasks []*task.Task) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}

func TestDepsController_GetFilteredTasksForLane(t *testing.T) {
	dc, _ := newDepsTestEnv(t)

	t.Run("all lane excludes context, blocks, and depends", func(t *testing.T) {
		all := dc.GetFilteredTasksForLane(depsLaneAll)
		ids := taskIDs(all)
		if slices.Contains(ids, testCtxID) {
			t.Error("all lane should not contain context task")
		}
		if slices.Contains(ids, testBlkID) {
			t.Error("all lane should not contain blocker task")
		}
		if slices.Contains(ids, testDepID) {
			t.Error("all lane should not contain depends task")
		}
		if !slices.Contains(ids, testFreeID) {
			t.Error("all lane should contain free task")
		}
	})

	t.Run("blocks lane contains tasks that depend on context", func(t *testing.T) {
		blocks := dc.GetFilteredTasksForLane(depsLaneBlocks)
		ids := taskIDs(blocks)
		if !slices.Contains(ids, testBlkID) {
			t.Error("blocks lane should contain blocker task")
		}
		if len(ids) != 1 {
			t.Errorf("blocks lane should have exactly 1 task, got %d: %v", len(ids), ids)
		}
	})

	t.Run("depends lane contains context task dependencies", func(t *testing.T) {
		depends := dc.GetFilteredTasksForLane(depsLaneDepends)
		ids := taskIDs(depends)
		if !slices.Contains(ids, testDepID) {
			t.Error("depends lane should contain depends task")
		}
		if len(ids) != 1 {
			t.Errorf("depends lane should have exactly 1 task, got %d: %v", len(ids), ids)
		}
	})

	t.Run("invalid lane returns nil", func(t *testing.T) {
		if dc.GetFilteredTasksForLane(-1) != nil {
			t.Error("invalid lane should return nil")
		}
		if dc.GetFilteredTasksForLane(3) != nil {
			t.Error("out of range lane should return nil")
		}
	})
}

func TestDepsController_MoveTask_AllToBlocks(t *testing.T) {
	dc, taskStore := newDepsTestEnv(t)

	dc.pluginConfig.SetSelectedLane(depsLaneAll)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)

	if !dc.handleMoveTask(-1) {
		t.Fatal("move should succeed")
	}

	// free task should now have context task in its dependsOn
	free := taskStore.GetTask(testFreeID)
	if !slices.Contains(free.DependsOn, testCtxID) {
		t.Errorf("free.DependsOn should contain %s, got %v", testCtxID, free.DependsOn)
	}
}

func TestDepsController_MoveTask_AllToDepends(t *testing.T) {
	dc, taskStore := newDepsTestEnv(t)

	dc.pluginConfig.SetSelectedLane(depsLaneAll)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)

	if !dc.handleMoveTask(1) {
		t.Fatal("move should succeed")
	}

	// context task should now have free task in its dependsOn
	ctx := taskStore.GetTask(testCtxID)
	if !slices.Contains(ctx.DependsOn, testFreeID) {
		t.Errorf("ctx.DependsOn should contain %s, got %v", testFreeID, ctx.DependsOn)
	}
}

func TestDepsController_MoveTask_BlocksToAll(t *testing.T) {
	dc, taskStore := newDepsTestEnv(t)

	dc.pluginConfig.SetSelectedLane(depsLaneBlocks)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneBlocks, 0)

	if !dc.handleMoveTask(1) {
		t.Fatal("move should succeed")
	}

	blk := taskStore.GetTask(testBlkID)
	if slices.Contains(blk.DependsOn, testCtxID) {
		t.Errorf("blk.DependsOn should not contain %s after move, got %v", testCtxID, blk.DependsOn)
	}
}

func TestDepsController_MoveTask_DependsToAll(t *testing.T) {
	dc, taskStore := newDepsTestEnv(t)

	dc.pluginConfig.SetSelectedLane(depsLaneDepends)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneDepends, 0)

	if !dc.handleMoveTask(-1) {
		t.Fatal("move should succeed")
	}

	ctx := taskStore.GetTask(testCtxID)
	if slices.Contains(ctx.DependsOn, testDepID) {
		t.Errorf("ctx.DependsOn should not contain %s after move, got %v", testDepID, ctx.DependsOn)
	}
}

func TestDepsController_MoveTask_OutOfBounds(t *testing.T) {
	dc, _ := newDepsTestEnv(t)

	dc.pluginConfig.SetSelectedLane(depsLaneBlocks)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneBlocks, 0)

	if dc.handleMoveTask(-1) {
		t.Error("move left from lane 0 should fail")
	}

	dc.pluginConfig.SetSelectedLane(depsLaneDepends)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneDepends, 0)

	if dc.handleMoveTask(1) {
		t.Error("move right from lane 2 should fail")
	}
}

func TestDepsController_MoveTask_RejectsMultiLaneJump(t *testing.T) {
	dc, _ := newDepsTestEnv(t)

	dc.pluginConfig.SetSelectedLane(depsLaneBlocks)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneBlocks, 0)

	if dc.handleMoveTask(2) {
		t.Error("offset=2 should be rejected")
	}
	if dc.handleMoveTask(-2) {
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
			if sr.Task.ID == testFreeID {
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
		// All lane has only free task, so nav down should return false (can't go past end)
		dc.HandleAction(ActionNavDown)
		// just verify it doesn't panic and returns a bool
	})

	t.Run("nav left from All switches to Blocks", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)
		result := dc.HandleAction(ActionNavLeft)
		if !result {
			t.Error("nav left from All should succeed (Blocks has tasks)")
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
			t.Error("nav right from All should succeed (Depends has tasks)")
		}
		if dc.pluginConfig.GetSelectedLane() != depsLaneDepends {
			t.Errorf("expected lane %d, got %d", depsLaneDepends, dc.pluginConfig.GetSelectedLane())
		}
	})

	t.Run("toggle view mode", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		before := dc.pluginConfig.GetViewMode()
		result := dc.HandleAction(ActionToggleViewMode)
		if !result {
			t.Error("toggle view mode should return true")
		}
		after := dc.pluginConfig.GetViewMode()
		if before == after {
			t.Error("view mode should change after toggle")
		}
	})

	t.Run("open task pushes detail view", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)
		result := dc.HandleAction(ActionOpenFromPlugin)
		if !result {
			t.Error("open should succeed when a task is selected")
		}
		top := dc.navController.navState.currentView()
		if top == nil || top.ViewID != model.TaskDetailViewID {
			t.Error("expected TaskDetailViewID to be pushed")
		}
	})

	t.Run("new task pushes edit view", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		result := dc.HandleAction(ActionNewTask)
		if !result {
			t.Error("new task should succeed")
		}
		top := dc.navController.navState.currentView()
		if top == nil || top.ViewID != model.TaskEditViewID {
			t.Error("expected TaskEditViewID to be pushed")
		}
	})

	t.Run("delete task removes from store", func(t *testing.T) {
		dc, taskStore := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)

		// free task should be in the All lane
		allTasks := dc.GetFilteredTasksForLane(depsLaneAll)
		if len(allTasks) == 0 {
			t.Fatal("expected at least one task in All lane")
		}
		deletedID := allTasks[0].ID

		result := dc.HandleAction(ActionDeleteTask)
		if !result {
			t.Error("delete should succeed when a task is selected")
		}
		if taskStore.GetTask(deletedID) != nil {
			t.Errorf("task %s should have been deleted", deletedID)
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
		result := dc.handleLaneSwitch("right", dc.GetFilteredTasksForLane)
		if !result {
			t.Error("should succeed — All lane has tasks")
		}
		if dc.pluginConfig.GetSelectedLane() != depsLaneAll {
			t.Errorf("expected lane %d, got %d", depsLaneAll, dc.pluginConfig.GetSelectedLane())
		}
	})

	t.Run("left from All lands on Blocks", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		result := dc.handleLaneSwitch("left", dc.GetFilteredTasksForLane)
		if !result {
			t.Error("should succeed — Blocks lane has tasks")
		}
		if dc.pluginConfig.GetSelectedLane() != depsLaneBlocks {
			t.Errorf("expected lane %d, got %d", depsLaneBlocks, dc.pluginConfig.GetSelectedLane())
		}
	})

	t.Run("left from Blocks returns false (boundary)", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneBlocks)
		if dc.handleLaneSwitch("left", dc.GetFilteredTasksForLane) {
			t.Error("should fail — no lane to the left of Blocks")
		}
	})

	t.Run("right from Depends returns false (boundary)", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneDepends)
		if dc.handleLaneSwitch("right", dc.GetFilteredTasksForLane) {
			t.Error("should fail — no lane to the right of Depends")
		}
	})
}

func TestDepsController_EnsureFirstNonEmptyLaneSelection(t *testing.T) {
	t.Run("current lane has tasks — no change", func(t *testing.T) {
		dc, _ := newDepsTestEnv(t)
		dc.pluginConfig.SetSelectedLane(depsLaneAll)
		dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)
		if dc.EnsureFirstNonEmptyLaneSelection() {
			t.Error("should return false when current lane has tasks")
		}
		if dc.pluginConfig.GetSelectedLane() != depsLaneAll {
			t.Error("lane should not change")
		}
	})

	t.Run("current lane empty — switches to first non-empty", func(t *testing.T) {
		dc, taskStore := newDepsTestEnv(t)
		// move free task into depends so All lane becomes empty
		free := taskStore.GetTask(testFreeID)
		free.DependsOn = []string{testCtxID}
		if err := taskStore.UpdateTask(free); err != nil {
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

func TestDepsController_DeleteTask_GateError(t *testing.T) {
	// when gate rejects delete, handleDeleteTask should return false
	taskStore := store.NewInMemoryStore()

	tasks := []*task.Task{
		{ID: testCtxID, Title: "Context", Status: task.StatusReady, Type: task.TypeStory, Priority: 3, DependsOn: []string{testDepID}, IsWorkflow: true},
		{ID: testBlkID, Title: "Blocker", Status: task.StatusReady, Type: task.TypeStory, Priority: 3, DependsOn: []string{testCtxID}, IsWorkflow: true},
		{ID: testDepID, Title: "Depends", Status: task.StatusReady, Type: task.TypeStory, Priority: 3, IsWorkflow: true},
		{ID: testFreeID, Title: "Free", Status: task.StatusReady, Type: task.TypeStory, Priority: 3, IsWorkflow: true},
	}
	for _, tt := range tasks {
		if err := taskStore.CreateTask(tt); err != nil {
			t.Fatalf("create task %s: %v", tt.ID, err)
		}
	}

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "Dependency:" + testCtxID, ConfigIndex: -1, Kind: plugin.KindBoard},
		TaskID:     testCtxID,
		Lanes:      []plugin.TikiLane{{Name: "Blocks"}, {Name: "All"}, {Name: "Depends"}},
	}
	pluginConfig := model.NewPluginConfig("Dependency")
	pluginConfig.SetLaneLayout([]int{1, 2, 1}, nil)

	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	// register a before-delete validator that blocks all deletes
	gate.OnDelete(func(old, new *task.Task, allTasks []*task.Task) *service.Rejection {
		return &service.Rejection{Reason: "deletes blocked for test"}
	})

	nav := newMockNavigationController()
	statusline := model.NewStatuslineConfig()
	dc := NewDepsController(taskStore, gate, pluginConfig, pluginDef, nav, statusline, rukiRuntime.NewSchema())

	// select free task in All lane
	dc.pluginConfig.SetSelectedLane(depsLaneAll)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)

	result := dc.HandleAction(ActionDeleteTask)
	if result {
		t.Error("expected delete to fail when gate rejects")
	}
}

func TestDepsController_MoveTask_UpdateError(t *testing.T) {
	// when gate rejects the update, statusline should receive the error
	taskStore := store.NewInMemoryStore()

	tasks := []*task.Task{
		{ID: testCtxID, Title: "Context", Status: task.StatusReady, Type: task.TypeStory, Priority: 3, DependsOn: []string{testDepID}, IsWorkflow: true},
		{ID: testBlkID, Title: "Blocker", Status: task.StatusReady, Type: task.TypeStory, Priority: 3, DependsOn: []string{testCtxID}, IsWorkflow: true},
		{ID: testDepID, Title: "Depends", Status: task.StatusReady, Type: task.TypeStory, Priority: 3, IsWorkflow: true},
		{ID: testFreeID, Title: "Free", Status: task.StatusReady, Type: task.TypeStory, Priority: 3, IsWorkflow: true},
	}
	for _, tt := range tasks {
		if err := taskStore.CreateTask(tt); err != nil {
			t.Fatalf("create task %s: %v", tt.ID, err)
		}
	}

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "Dependency:" + testCtxID, ConfigIndex: -1, Kind: plugin.KindBoard},
		TaskID:     testCtxID,
		Lanes:      []plugin.TikiLane{{Name: "Blocks"}, {Name: "All"}, {Name: "Depends"}},
	}
	pluginConfig := model.NewPluginConfig("Dependency")
	pluginConfig.SetLaneLayout([]int{1, 2, 1}, nil)

	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	// register a validator that blocks all updates
	gate.OnUpdate(func(old, new *task.Task, allTasks []*task.Task) *service.Rejection {
		return &service.Rejection{Reason: "updates blocked for test"}
	})

	nav := newMockNavigationController()
	statusline := model.NewStatuslineConfig()
	dc := NewDepsController(taskStore, gate, pluginConfig, pluginDef, nav, statusline, rukiRuntime.NewSchema())

	// select free task in All lane, move left → Blocks
	dc.pluginConfig.SetSelectedLane(depsLaneAll)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneAll, 0)

	result := dc.handleMoveTask(-1)
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
		ActionMoveTaskLeft:   false,
		ActionMoveTaskRight:  false,
		ActionOpenFromPlugin: false,
		ActionNewTask:        false,
		ActionDeleteTask:     false,
		ActionToggleViewMode: false,
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

func newDepsNavEnv(t *testing.T, blockers int, allTasks int, depends int, laneColumns []int) *DepsController {
	t.Helper()

	taskStore := store.NewInMemoryStore()
	contextID := "CTXNAV0"
	contextDepends := make([]string, 0, depends)
	for i := 0; i < depends; i++ {
		contextDepends = append(contextDepends, fmt.Sprintf("TIKI-DEP%03d", i))
	}
	if err := taskStore.CreateTask(&task.Task{
		ID:         contextID,
		Title:      "Context",
		Status:     task.StatusReady,
		Type:       task.TypeStory,
		Priority:   3,
		DependsOn:  contextDepends,
		IsWorkflow: true,
	}); err != nil {
		t.Fatalf("create context: %v", err)
	}

	for i := 0; i < depends; i++ {
		if err := taskStore.CreateTask(&task.Task{
			ID:         fmt.Sprintf("TIKI-DEP%03d", i),
			Title:      "Depends",
			Status:     task.StatusReady,
			Type:       task.TypeStory,
			Priority:   3,
			IsWorkflow: true,
		}); err != nil {
			t.Fatalf("create depends task: %v", err)
		}
	}

	for i := 0; i < blockers; i++ {
		if err := taskStore.CreateTask(&task.Task{
			ID:         fmt.Sprintf("TIKI-BLK%03d", i),
			Title:      "Blocker",
			Status:     task.StatusReady,
			Type:       task.TypeStory,
			Priority:   3,
			DependsOn:  []string{contextID},
			IsWorkflow: true,
		}); err != nil {
			t.Fatalf("create blocker task: %v", err)
		}
	}

	for i := 0; i < allTasks; i++ {
		if err := taskStore.CreateTask(&task.Task{
			ID:         fmt.Sprintf("TIKI-ALL%03d", i),
			Title:      "All",
			Status:     task.StatusReady,
			Type:       task.TypeStory,
			Priority:   3,
			IsWorkflow: true,
		}); err != nil {
			t.Fatalf("create all lane task: %v", err)
		}
	}

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "Dependency:" + contextID, ConfigIndex: -1, Kind: plugin.KindBoard},
		TaskID:     contextID,
		Lanes:      []plugin.TikiLane{{Name: "Blocks"}, {Name: "All"}, {Name: "Depends"}},
	}
	pluginConfig := model.NewPluginConfig("Dependency")
	pluginConfig.SetLaneLayout(laneColumns, nil)

	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)

	nav := newMockNavigationController()
	return NewDepsController(taskStore, gate, pluginConfig, pluginDef, nav, nil, rukiRuntime.NewSchema())
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
	// lane 0 has 6 tasks with 4 columns; row 1 is partial => index 5
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
	// all lane has 3 tasks with 2 columns => max row 1, row-start index 2
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

func TestDepsController_GetFilteredTasksForLane_WithSearch(t *testing.T) {
	dc, taskStore := newDepsTestEnv(t)

	// set search results to only include the free task
	free := taskStore.GetTask(testFreeID)
	dc.pluginConfig.SetSearchResults([]task.SearchResult{{Task: free, Score: 1.0}}, "Free")

	// All lane should now only show the free task (matching search)
	allTasks := dc.GetFilteredTasksForLane(depsLaneAll)
	if len(allTasks) != 1 {
		t.Fatalf("expected 1 task with search narrowing, got %d", len(allTasks))
	}
	if allTasks[0].ID != testFreeID {
		t.Errorf("expected %s, got %s", testFreeID, allTasks[0].ID)
	}

	// Blocks lane should be empty (no matching search results)
	blocksTasks := dc.GetFilteredTasksForLane(depsLaneBlocks)
	if len(blocksTasks) != 0 {
		t.Errorf("expected 0 blocks tasks with search narrowing, got %d", len(blocksTasks))
	}
}

func TestDepsController_GetFilteredTasksForLane_MissingContextTask(t *testing.T) {
	dc, taskStore := newDepsTestEnv(t)

	// delete the context task
	taskStore.DeleteTask(testCtxID)

	// all lanes should return nil when context task is missing
	if dc.GetFilteredTasksForLane(depsLaneAll) != nil {
		t.Error("expected nil when context task is missing")
	}
	if dc.GetFilteredTasksForLane(depsLaneBlocks) != nil {
		t.Error("expected nil when context task is missing")
	}
	if dc.GetFilteredTasksForLane(depsLaneDepends) != nil {
		t.Error("expected nil when context task is missing")
	}
}

func TestDepsController_MoveTask_EmptySelection(t *testing.T) {
	dc, _ := newDepsTestEnv(t)

	// set selected lane to blocks but with an index beyond the task list
	dc.pluginConfig.SetSelectedLane(depsLaneBlocks)
	dc.pluginConfig.SetSelectedIndexForLane(depsLaneBlocks, 99) // beyond available tasks

	// getSelectedTaskID should return "" for an index beyond the task list,
	// so handleMoveTask should return false
	if dc.handleMoveTask(1) {
		t.Error("expected false when no task is selected (index out of range)")
	}
}
