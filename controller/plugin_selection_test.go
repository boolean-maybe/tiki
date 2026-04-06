package controller

import (
	"fmt"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/plugin/filter"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

type navHarness struct {
	pb      *pluginBase
	config  *model.PluginConfig
	byLane  map[int][]*task.Task
	getLane func(int) []*task.Task
}

func newNavHarness(columns []int, counts []int) *navHarness {
	lanes := make([]plugin.TikiLane, len(columns))
	for i := range columns {
		lanes[i] = plugin.TikiLane{Name: fmt.Sprintf("Lane-%d", i)}
	}

	config := model.NewPluginConfig("TestPlugin")
	config.SetLaneLayout(columns, nil)

	byLane := make(map[int][]*task.Task, len(counts))
	for lane, count := range counts {
		tasks := make([]*task.Task, count)
		for i := 0; i < count; i++ {
			tasks[i] = &task.Task{
				ID:     fmt.Sprintf("T-%d-%d", lane, i),
				Title:  "Task",
				Status: task.StatusReady,
				Type:   task.TypeStory,
			}
		}
		byLane[lane] = tasks
	}

	pb := &pluginBase{
		pluginConfig: config,
		pluginDef: &plugin.TikiPlugin{
			BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
			Lanes:      lanes,
		},
	}

	return &navHarness{
		pb:     pb,
		config: config,
		byLane: byLane,
		getLane: func(lane int) []*task.Task {
			return byLane[lane]
		},
	}
}

func TestEnsureFirstNonEmptyLaneSelectionSelectsFirstTask(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	if err := taskStore.CreateTask(&task.Task{
		ID:     "T-1",
		Title:  "Task 1",
		Status: task.StatusReady,
		Type:   task.TypeStory,
	}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := taskStore.CreateTask(&task.Task{
		ID:     "T-2",
		Title:  "Task 2",
		Status: task.StatusReady,
		Type:   task.TypeStory,
	}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	emptyFilter, err := filter.ParseFilter("status = 'done'")
	if err != nil {
		t.Fatalf("parse filter: %v", err)
	}
	todoFilter, err := filter.ParseFilter("status = 'ready'")
	if err != nil {
		t.Fatalf("parse filter: %v", err)
	}

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{
			Name: "TestPlugin",
		},
		Lanes: []plugin.TikiLane{
			{Name: "Empty", Columns: 1, Filter: emptyFilter},
			{Name: "Todo", Columns: 1, Filter: todoFilter},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1, 1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 1)

	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil)
	pc.EnsureFirstNonEmptyLaneSelection()

	if pluginConfig.GetSelectedLane() != 1 {
		t.Fatalf("expected selected lane 1, got %d", pluginConfig.GetSelectedLane())
	}
	if pluginConfig.GetSelectedIndexForLane(1) != 0 {
		t.Fatalf("expected selected index 0, got %d", pluginConfig.GetSelectedIndexForLane(1))
	}
}

func TestEnsureFirstNonEmptyLaneSelectionKeepsCurrentLane(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	if err := taskStore.CreateTask(&task.Task{
		ID:     "T-1",
		Title:  "Task 1",
		Status: task.StatusReady,
		Type:   task.TypeStory,
	}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	todoFilter, err := filter.ParseFilter("status = 'ready'")
	if err != nil {
		t.Fatalf("parse filter: %v", err)
	}

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{
			Name: "TestPlugin",
		},
		Lanes: []plugin.TikiLane{
			{Name: "First", Columns: 1, Filter: todoFilter},
			{Name: "Second", Columns: 1, Filter: todoFilter},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1, 1}, nil)
	pluginConfig.SetSelectedLane(1)
	pluginConfig.SetSelectedIndexForLane(1, 0)

	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil)
	pc.EnsureFirstNonEmptyLaneSelection()

	if pluginConfig.GetSelectedLane() != 1 {
		t.Fatalf("expected selected lane 1, got %d", pluginConfig.GetSelectedLane())
	}
	if pluginConfig.GetSelectedIndexForLane(1) != 0 {
		t.Fatalf("expected selected index 0, got %d", pluginConfig.GetSelectedIndexForLane(1))
	}
}

func TestEnsureFirstNonEmptyLaneSelectionNoTasks(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	emptyFilter, err := filter.ParseFilter("status = 'done'")
	if err != nil {
		t.Fatalf("parse filter: %v", err)
	}

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{
			Name: "TestPlugin",
		},
		Lanes: []plugin.TikiLane{
			{Name: "Empty", Columns: 1, Filter: emptyFilter},
			{Name: "StillEmpty", Columns: 1, Filter: emptyFilter},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1, 1}, nil)
	pluginConfig.SetSelectedLane(1)
	pluginConfig.SetSelectedIndexForLane(1, 2)

	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil)
	pc.EnsureFirstNonEmptyLaneSelection()

	if pluginConfig.GetSelectedLane() != 1 {
		t.Fatalf("expected selected lane 1, got %d", pluginConfig.GetSelectedLane())
	}
	if pluginConfig.GetSelectedIndexForLane(1) != 2 {
		t.Fatalf("expected selected index 2, got %d", pluginConfig.GetSelectedIndexForLane(1))
	}
}

func TestLaneSwitchAdjacentNonEmptyPreservesViewportRow_RightLandsLeftmost(t *testing.T) {
	h := newNavHarness([]int{2, 3}, []int{8, 12})
	h.config.SetSelectedLane(0)
	h.config.SetSelectedIndexForLane(0, 5) // row 2, col 1 (right edge)
	h.config.SetScrollOffsetForLane(0, 1)  // source row offset in viewport = 1
	h.config.SetScrollOffsetForLane(1, 2)

	if !h.pb.handleNav("right", h.getLane) {
		t.Fatal("expected right lane switch to succeed")
	}

	if got := h.config.GetSelectedLane(); got != 1 {
		t.Fatalf("expected selected lane 1, got %d", got)
	}

	// target row: 2(scroll) + 1(offset) = 3, moving right lands at row start
	if got := h.config.GetSelectedIndexForLane(1); got != 9 {
		t.Fatalf("expected selected index 9, got %d", got)
	}
}

func TestLaneSwitchAdjacentNonEmptyPreservesViewportRow_LeftLandsRightmostPopulated(t *testing.T) {
	h := newNavHarness([]int{4, 3}, []int{6, 8})
	h.config.SetSelectedLane(1)
	h.config.SetSelectedIndexForLane(1, 3) // row 1, col 0 (left edge)
	h.config.SetScrollOffsetForLane(1, 1)  // source row offset in viewport = 0
	h.config.SetScrollOffsetForLane(0, 1)

	if !h.pb.handleNav("left", h.getLane) {
		t.Fatal("expected left lane switch to succeed")
	}

	if got := h.config.GetSelectedLane(); got != 0 {
		t.Fatalf("expected selected lane 0, got %d", got)
	}

	// target row 1 in a partial row (indices 4..5), moving left lands on 5
	if got := h.config.GetSelectedIndexForLane(0); got != 5 {
		t.Fatalf("expected selected index 5, got %d", got)
	}
}

func TestLaneSwitchSkipEmptyDoesNotCarrySourceRowOffset_Right(t *testing.T) {
	h := newNavHarness([]int{2, 2, 3}, []int{8, 0, 9})
	h.config.SetSelectedLane(0)
	h.config.SetSelectedIndexForLane(0, 7) // row 3, col 1
	h.config.SetScrollOffsetForLane(0, 0)  // large source row offset should be ignored
	h.config.SetScrollOffsetForLane(2, 1)

	if !h.pb.handleNav("right", h.getLane) {
		t.Fatal("expected skip-empty right lane switch to succeed")
	}

	if got := h.config.GetSelectedLane(); got != 2 {
		t.Fatalf("expected selected lane 2, got %d", got)
	}

	// skip-empty landing uses target viewport row only: row 1 start in 3 columns => index 3
	if got := h.config.GetSelectedIndexForLane(2); got != 3 {
		t.Fatalf("expected selected index 3, got %d", got)
	}
}

func TestLaneSwitchSkipEmptyDoesNotCarrySourceRowOffset_Left(t *testing.T) {
	h := newNavHarness([]int{4, 1, 2}, []int{6, 0, 5})
	h.config.SetSelectedLane(2)
	h.config.SetSelectedIndexForLane(2, 4) // row 2, col 0
	h.config.SetScrollOffsetForLane(2, 0)  // source row offset should be ignored
	h.config.SetScrollOffsetForLane(0, 1)

	if !h.pb.handleNav("left", h.getLane) {
		t.Fatal("expected skip-empty left lane switch to succeed")
	}

	if got := h.config.GetSelectedLane(); got != 0 {
		t.Fatalf("expected selected lane 0, got %d", got)
	}

	// row 1 in lane 0 is partial (indices 4..5), moving left lands on index 5
	if got := h.config.GetSelectedIndexForLane(0); got != 5 {
		t.Fatalf("expected selected index 5, got %d", got)
	}
}

func TestLaneSwitchMultiEmptyChainPreservesTraversalOrder(t *testing.T) {
	h := newNavHarness([]int{1, 1, 1, 2}, []int{3, 0, 0, 4})
	h.config.SetSelectedLane(0)
	h.config.SetSelectedIndexForLane(0, 2)
	h.config.SetScrollOffsetForLane(3, 1)

	if !h.pb.handleNav("right", h.getLane) {
		t.Fatal("expected right lane switch to succeed across empty chain")
	}

	if got := h.config.GetSelectedLane(); got != 3 {
		t.Fatalf("expected selected lane 3, got %d", got)
	}

	// skip-empty landing uses lane 3 viewport row 1, direction right => row start index 2
	if got := h.config.GetSelectedIndexForLane(3); got != 2 {
		t.Fatalf("expected selected index 2, got %d", got)
	}
}

func TestLaneSwitchNoReachableTargetIsStrictNoOp(t *testing.T) {
	h := newNavHarness([]int{2, 2, 2}, []int{5, 0, 0})
	h.config.SetSelectedLane(0)
	h.config.SetSelectedIndexForLane(0, 4)
	h.config.SetScrollOffsetForLane(0, 3)
	h.config.SetScrollOffsetForLane(1, 7)
	h.config.SetScrollOffsetForLane(2, 8)

	callbacks := 0
	listenerID := h.config.AddSelectionListener(func() { callbacks++ })
	defer h.config.RemoveSelectionListener(listenerID)

	if h.pb.handleNav("right", h.getLane) {
		t.Fatal("expected right action to be a no-op with no reachable lane")
	}

	if got := h.config.GetSelectedLane(); got != 0 {
		t.Fatalf("expected lane 0, got %d", got)
	}
	if got := h.config.GetSelectedIndexForLane(0); got != 4 {
		t.Fatalf("expected selected index to remain 4, got %d", got)
	}
	if got := h.config.GetScrollOffsetForLane(0); got != 3 {
		t.Fatalf("expected lane 0 scroll offset 3, got %d", got)
	}
	if got := h.config.GetScrollOffsetForLane(1); got != 7 {
		t.Fatalf("expected lane 1 scroll offset 7, got %d", got)
	}
	if got := h.config.GetScrollOffsetForLane(2); got != 8 {
		t.Fatalf("expected lane 2 scroll offset 8, got %d", got)
	}
	if callbacks != 0 {
		t.Fatalf("expected 0 selection callbacks, got %d", callbacks)
	}
}

func TestHandleNavVerticalStaleIndexRecoveryNotifiesOnce(t *testing.T) {
	t.Run("stale index at down boundary is healed", func(t *testing.T) {
		h := newNavHarness([]int{2}, []int{6})
		h.config.SetSelectedLane(0)
		h.config.SetSelectedIndexForLane(0, 99)

		callbacks := 0
		listenerID := h.config.AddSelectionListener(func() { callbacks++ })
		defer h.config.RemoveSelectionListener(listenerID)

		if !h.pb.handleNav("down", h.getLane) {
			t.Fatal("expected stale down action to heal selection")
		}
		if got := h.config.GetSelectedIndexForLane(0); got != 5 {
			t.Fatalf("expected healed index 5, got %d", got)
		}
		if callbacks != 1 {
			t.Fatalf("expected 1 selection callback, got %d", callbacks)
		}
	})

	t.Run("stale negative index at up boundary is healed", func(t *testing.T) {
		h := newNavHarness([]int{2}, []int{6})
		h.config.SetSelectedLane(0)
		h.config.SetSelectedIndexForLane(0, -5)

		callbacks := 0
		listenerID := h.config.AddSelectionListener(func() { callbacks++ })
		defer h.config.RemoveSelectionListener(listenerID)

		if !h.pb.handleNav("up", h.getLane) {
			t.Fatal("expected stale up action to heal selection")
		}
		if got := h.config.GetSelectedIndexForLane(0); got != 0 {
			t.Fatalf("expected healed index 0, got %d", got)
		}
		if callbacks != 1 {
			t.Fatalf("expected 1 selection callback, got %d", callbacks)
		}
	})
}

func TestHandleNavVerticalInRangeBoundaryNoOpHasNoNotification(t *testing.T) {
	h := newNavHarness([]int{2}, []int{6})
	h.config.SetSelectedLane(0)
	h.config.SetSelectedIndexForLane(0, 0)

	callbacks := 0
	listenerID := h.config.AddSelectionListener(func() { callbacks++ })
	defer h.config.RemoveSelectionListener(listenerID)

	if h.pb.handleNav("up", h.getLane) {
		t.Fatal("expected up at top boundary to return false")
	}
	if got := h.config.GetSelectedIndexForLane(0); got != 0 {
		t.Fatalf("expected index 0, got %d", got)
	}
	if callbacks != 0 {
		t.Fatalf("expected 0 callbacks, got %d", callbacks)
	}
}

func TestHandleNavHorizontalStaleIndexNoTargetDoesNotPersistNormalization(t *testing.T) {
	h := newNavHarness([]int{2}, []int{3})
	h.config.SetSelectedLane(0)
	h.config.SetSelectedIndexForLane(0, 99)

	callbacks := 0
	listenerID := h.config.AddSelectionListener(func() { callbacks++ })
	defer h.config.RemoveSelectionListener(listenerID)

	if h.pb.handleNav("right", h.getLane) {
		t.Fatal("expected right with no reachable target to return false")
	}
	if got := h.config.GetSelectedIndexForLane(0); got != 99 {
		t.Fatalf("expected stale index to remain 99 on strict no-op, got %d", got)
	}
	if callbacks != 0 {
		t.Fatalf("expected 0 callbacks, got %d", callbacks)
	}
}

func TestHandleNavHorizontalStaleIndexInLaneMovePersistsFinalIndex(t *testing.T) {
	h := newNavHarness([]int{3}, []int{5})
	h.config.SetSelectedLane(0)
	h.config.SetSelectedIndexForLane(0, 99)

	callbacks := 0
	listenerID := h.config.AddSelectionListener(func() { callbacks++ })
	defer h.config.RemoveSelectionListener(listenerID)

	if !h.pb.handleNav("left", h.getLane) {
		t.Fatal("expected left move from stale index to succeed in-lane")
	}
	if got := h.config.GetSelectedIndexForLane(0); got != 3 {
		t.Fatalf("expected index 3 after clamped in-lane move, got %d", got)
	}
	if callbacks != 1 {
		t.Fatalf("expected 1 callback, got %d", callbacks)
	}
}

func TestLaneSwitchClampsTargetScrollAndPersistsClampedValue(t *testing.T) {
	h := newNavHarness([]int{1, 2}, []int{1, 3})
	h.config.SetSelectedLane(0)
	h.config.SetSelectedIndexForLane(0, 0)
	h.config.SetScrollOffsetForLane(1, 10)

	if !h.pb.handleNav("right", h.getLane) {
		t.Fatal("expected right lane switch to succeed")
	}

	if got := h.config.GetSelectedLane(); got != 1 {
		t.Fatalf("expected lane 1, got %d", got)
	}
	// lane 1 has max row 1, moving right lands at row-start index 2
	if got := h.config.GetSelectedIndexForLane(1); got != 2 {
		t.Fatalf("expected selected index 2, got %d", got)
	}
	if got := h.config.GetScrollOffsetForLane(1); got != 1 {
		t.Fatalf("expected clamped and persisted scroll offset 1, got %d", got)
	}
}

func TestLaneSwitchClampsStaleSourceScrollBeforeRowMath(t *testing.T) {
	h := newNavHarness([]int{2, 2}, []int{6, 6})
	h.config.SetSelectedLane(0)
	h.config.SetSelectedIndexForLane(0, 3) // row 1, col 1 (right edge)
	h.config.SetScrollOffsetForLane(0, -5) // stale source scroll should clamp to 0
	h.config.SetScrollOffsetForLane(1, 0)

	if !h.pb.handleNav("right", h.getLane) {
		t.Fatal("expected right lane switch to succeed")
	}
	if got := h.config.GetSelectedLane(); got != 1 {
		t.Fatalf("expected lane 1, got %d", got)
	}
	// source row 1 with clamped source scroll 0 => row offset 1 => target row 1 => index 2
	if got := h.config.GetSelectedIndexForLane(1); got != 2 {
		t.Fatalf("expected selected index 2, got %d", got)
	}
}

func TestLaneSwitchSuccessfulActionKeepsUnrelatedLaneScrollOffsets(t *testing.T) {
	h := newNavHarness([]int{1, 1, 1}, []int{1, 1, 2})
	h.config.SetSelectedLane(0)
	h.config.SetSelectedIndexForLane(0, 0)
	h.config.SetScrollOffsetForLane(2, 5)

	if !h.pb.handleNav("right", h.getLane) {
		t.Fatal("expected right lane switch to succeed")
	}
	if got := h.config.GetSelectedLane(); got != 1 {
		t.Fatalf("expected lane 1, got %d", got)
	}
	if got := h.config.GetScrollOffsetForLane(2); got != 5 {
		t.Fatalf("expected unrelated lane scroll offset 5, got %d", got)
	}
}

func TestLaneSwitchFromEmptySourceUsesTopViewportContext(t *testing.T) {
	h := newNavHarness([]int{2, 2}, []int{0, 6})
	h.config.SetSelectedLane(0)
	h.config.SetSelectedIndexForLane(0, 42)
	h.config.SetScrollOffsetForLane(1, 2)

	if !h.pb.handleNav("right", h.getLane) {
		t.Fatal("expected right lane switch from empty source to succeed")
	}

	if got := h.config.GetSelectedLane(); got != 1 {
		t.Fatalf("expected lane 1, got %d", got)
	}
	// empty source forces row offset 0, so landed row is target scroll row (2)
	if got := h.config.GetSelectedIndexForLane(1); got != 4 {
		t.Fatalf("expected selected index 4, got %d", got)
	}
}

func TestPluginController_HandleOpenTask(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory,
	})

	todoFilter, _ := filter.ParseFilter("status = 'ready'")
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Todo", Columns: 1, Filter: todoFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	navController := newMockNavigationController()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, navController, nil)

	if !pc.HandleAction(ActionOpenFromPlugin) {
		t.Error("expected HandleAction(open) to return true when task is selected")
	}

	// verify navigation was pushed
	if navController.navState.depth() == 0 {
		t.Error("expected navigation push for open task")
	}
}

func TestPluginController_HandleOpenTask_Empty(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	emptyFilter, _ := filter.ParseFilter("status = 'done'")
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Empty", Columns: 1, Filter: emptyFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, newMockNavigationController(), nil)

	if pc.HandleAction(ActionOpenFromPlugin) {
		t.Error("expected false when no task is selected")
	}
}

func TestPluginController_HandleDeleteTask(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	todoFilter, _ := filter.ParseFilter("status = 'ready'")
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Todo", Columns: 1, Filter: todoFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, newMockNavigationController(), nil)

	if !pc.HandleAction(ActionDeleteTask) {
		t.Error("expected HandleAction(delete) to return true")
	}

	if taskStore.GetTask("T-1") != nil {
		t.Error("task should have been deleted")
	}
}

func TestPluginController_HandleDeleteTask_Empty(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	emptyFilter, _ := filter.ParseFilter("status = 'done'")
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Empty", Columns: 1, Filter: emptyFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, newMockNavigationController(), nil)

	if pc.HandleAction(ActionDeleteTask) {
		t.Error("expected false when no task is selected")
	}
}

func TestPluginController_HandleDeleteTask_Rejected(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	todoFilter, _ := filter.ParseFilter("status = 'ready'")
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Todo", Columns: 1, Filter: todoFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	statusline := model.NewStatuslineConfig()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	gate.OnDelete(func(_, _ *task.Task, _ []*task.Task) *service.Rejection {
		return &service.Rejection{Reason: "cannot delete"}
	})
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, newMockNavigationController(), statusline)

	if pc.HandleAction(ActionDeleteTask) {
		t.Error("expected false when delete is rejected")
	}

	// task should still exist
	if taskStore.GetTask("T-1") == nil {
		t.Error("task should not have been deleted")
	}
}

func TestPluginController_GetNameAndRegistry(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	todoFilter, _ := filter.ParseFilter("status = 'ready'")
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "MyPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Todo", Columns: 1, Filter: todoFilter}},
	}
	pluginConfig := model.NewPluginConfig("MyPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil)

	if pc.GetPluginName() != "MyPlugin" {
		t.Errorf("GetPluginName() = %q, want %q", pc.GetPluginName(), "MyPlugin")
	}
	if pc.GetActionRegistry() == nil {
		t.Error("GetActionRegistry() should not be nil")
	}
}

func TestPluginController_HandleMoveTask_Rejected(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	readyFilter, _ := filter.ParseFilter("status = 'ready'")
	inProgressFilter, _ := filter.ParseFilter("status = 'in_progress'")
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes: []plugin.TikiLane{
			{Name: "Ready", Columns: 1, Filter: readyFilter},
			{
				Name: "InProgress", Columns: 1, Filter: inProgressFilter,
				Action: plugin.LaneAction{
					Ops: []plugin.LaneActionOp{
						{Field: plugin.ActionFieldStatus, Operator: plugin.ActionOperatorAssign, StrValue: "in_progress"},
					},
				},
			},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1, 1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	statusline := model.NewStatuslineConfig()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	gate.OnUpdate(func(_, _ *task.Task, _ []*task.Task) *service.Rejection {
		return &service.Rejection{Reason: "updates blocked"}
	})
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, statusline)

	if pc.HandleAction(ActionMoveTaskRight) {
		t.Error("expected false when move is rejected by gate")
	}

	// task should still have original status
	tk := taskStore.GetTask("T-1")
	if tk.Status != task.StatusReady {
		t.Errorf("expected status ready, got %s", tk.Status)
	}
}

func TestPluginController_HandlePluginAction_Success(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	readyFilter, _ := filter.ParseFilter("status = 'ready'")
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
		Actions: []plugin.PluginAction{
			{
				Rune:  'd',
				Label: "Mark Done",
				Action: plugin.LaneAction{
					Ops: []plugin.LaneActionOp{
						{Field: plugin.ActionFieldStatus, Operator: plugin.ActionOperatorAssign, StrValue: "done"},
					},
				},
			},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil)

	if !pc.HandleAction(pluginActionID('d')) {
		t.Error("expected true for successful plugin action")
	}

	tk := taskStore.GetTask("T-1")
	if tk.Status != "done" {
		t.Errorf("expected status done, got %s", tk.Status)
	}
}

func TestPluginController_HandlePluginAction_Rejected(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	readyFilter, _ := filter.ParseFilter("status = 'ready'")
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
		Actions: []plugin.PluginAction{
			{
				Rune:  'd',
				Label: "Mark Done",
				Action: plugin.LaneAction{
					Ops: []plugin.LaneActionOp{
						{Field: plugin.ActionFieldStatus, Operator: plugin.ActionOperatorAssign, StrValue: "done"},
					},
				},
			},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	statusline := model.NewStatuslineConfig()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	gate.OnUpdate(func(_, _ *task.Task, _ []*task.Task) *service.Rejection {
		return &service.Rejection{Reason: "updates blocked"}
	})
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, statusline)

	if pc.HandleAction(pluginActionID('d')) {
		t.Error("expected false when plugin action is rejected by gate")
	}

	// task should still have original status
	tk := taskStore.GetTask("T-1")
	if tk.Status != task.StatusReady {
		t.Errorf("expected status ready, got %s", tk.Status)
	}
}
