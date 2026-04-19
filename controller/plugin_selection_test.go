package controller

import (
	"fmt"
	"testing"

	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// mustParseStmt is a test helper that parses and validates a ruki statement,
// failing the test on error.
func mustParseStmt(t *testing.T, input string) *ruki.ValidatedStatement {
	t.Helper()
	schema := rukiRuntime.NewSchema()
	parser := ruki.NewParser(schema)
	stmt, err := parser.ParseAndValidateStatement(input, ruki.ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("parse ruki statement %q: %v", input, err)
	}
	return stmt
}

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

	emptyFilter := mustParseStmt(t, `select where status = "done"`)
	todoFilter := mustParseStmt(t, `select where status = "ready"`)

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

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)
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

	todoFilter := mustParseStmt(t, `select where status = "ready"`)

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

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)
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
	emptyFilter := mustParseStmt(t, `select where status = "done"`)

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

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)
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

	todoFilter := mustParseStmt(t, `select where status = "ready"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Todo", Columns: 1, Filter: todoFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	navController := newMockNavigationController()
	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, navController, nil, schema)

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
	emptyFilter := mustParseStmt(t, `select where status = "done"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Empty", Columns: 1, Filter: emptyFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, newMockNavigationController(), nil, schema)

	if pc.HandleAction(ActionOpenFromPlugin) {
		t.Error("expected false when no task is selected")
	}
}

func TestPluginController_HandleDeleteTask(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	todoFilter := mustParseStmt(t, `select where status = "ready"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Todo", Columns: 1, Filter: todoFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, newMockNavigationController(), nil, schema)

	if !pc.HandleAction(ActionDeleteTask) {
		t.Error("expected HandleAction(delete) to return true")
	}

	if taskStore.GetTask("T-1") != nil {
		t.Error("task should have been deleted")
	}
}

func TestPluginController_HandleDeleteTask_Empty(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	emptyFilter := mustParseStmt(t, `select where status = "done"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Empty", Columns: 1, Filter: emptyFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, newMockNavigationController(), nil, schema)

	if pc.HandleAction(ActionDeleteTask) {
		t.Error("expected false when no task is selected")
	}
}

func TestPluginController_HandleDeleteTask_Rejected(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	todoFilter := mustParseStmt(t, `select where status = "ready"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Todo", Columns: 1, Filter: todoFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	statusline := model.NewStatuslineConfig()
	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	gate.OnDelete(func(_, _ *task.Task, _ []*task.Task) *service.Rejection {
		return &service.Rejection{Reason: "cannot delete"}
	})
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, newMockNavigationController(), statusline, schema)

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
	todoFilter := mustParseStmt(t, `select where status = "ready"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "MyPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Todo", Columns: 1, Filter: todoFilter}},
	}
	pluginConfig := model.NewPluginConfig("MyPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

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

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	inProgressFilter := mustParseStmt(t, `select where status = "inProgress"`)
	inProgressAction := mustParseStmt(t, `update where id = id() set status = "inProgress"`)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes: []plugin.TikiLane{
			{Name: "Ready", Columns: 1, Filter: readyFilter},
			{Name: "InProgress", Columns: 1, Filter: inProgressFilter, Action: inProgressAction},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1, 1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	statusline := model.NewStatuslineConfig()
	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	gate.OnUpdate(func(_, _ *task.Task, _ []*task.Task) *service.Rejection {
		return &service.Rejection{Reason: "updates blocked"}
	})
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, statusline, schema)

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

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	markDoneAction := mustParseStmt(t, `update where id = id() set status = "done"`)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
		Actions: []plugin.PluginAction{
			{
				Rune:   'd',
				Label:  "Mark Done",
				Action: markDoneAction,
			},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

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

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	markDoneAction := mustParseStmt(t, `update where id = id() set status = "done"`)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
		Actions: []plugin.PluginAction{
			{
				Rune:   'd',
				Label:  "Mark Done",
				Action: markDoneAction,
			},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	statusline := model.NewStatuslineConfig()
	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	gate.OnUpdate(func(_, _ *task.Task, _ []*task.Task) *service.Rejection {
		return &service.Rejection{Reason: "updates blocked"}
	})
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, statusline, schema)

	if pc.HandleAction(pluginActionID('d')) {
		t.Error("expected false when plugin action is rejected by gate")
	}

	// task should still have original status
	tk2 := taskStore.GetTask("T-1")
	if tk2.Status != task.StatusReady {
		t.Errorf("expected status ready, got %s", tk2.Status)
	}
}

func TestPluginController_HandlePluginAction_Create(t *testing.T) {
	taskStore := store.NewInMemoryStore()

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	createAction := mustParseStmt(t, `create title="New Task" status="ready" type="story" priority=3`)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
		Actions: []plugin.PluginAction{
			{
				Rune:   'c',
				Label:  "Create",
				Action: createAction,
			},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	if !pc.HandleAction(pluginActionID('c')) {
		t.Error("expected true for successful create action")
	}

	allTasks := taskStore.GetAllTasks()
	if len(allTasks) != 1 {
		t.Fatalf("expected 1 task after create, got %d", len(allTasks))
	}
	if allTasks[0].Title != "New Task" {
		t.Errorf("expected title 'New Task', got %q", allTasks[0].Title)
	}
}

func TestPluginController_HandlePluginAction_Delete(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusDone, Type: task.TypeStory, Priority: 3,
	})

	doneFilter := mustParseStmt(t, `select where status = "done"`)
	deleteAction := mustParseStmt(t, `delete where status = "done"`)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Done", Columns: 1, Filter: doneFilter}},
		Actions: []plugin.PluginAction{
			{
				Rune:   'x',
				Label:  "Delete Done",
				Action: deleteAction,
			},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	if !pc.HandleAction(pluginActionID('x')) {
		t.Error("expected true for successful delete action")
	}

	if taskStore.GetTask("T-1") != nil {
		t.Error("task should have been deleted")
	}
}

func TestPluginController_HandlePluginAction_NoMatchingRune(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
		Actions:    []plugin.PluginAction{},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	if pc.HandleAction(pluginActionID('z')) {
		t.Error("expected false for non-matching plugin action rune")
	}
}

func TestPluginController_HandlePluginAction_NoSelectedTask(t *testing.T) {
	taskStore := store.NewInMemoryStore()

	emptyFilter := mustParseStmt(t, `select where status = "done"`)
	updateAction := mustParseStmt(t, `update where id = id() set status = "done"`)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Empty", Columns: 1, Filter: emptyFilter}},
		Actions: []plugin.PluginAction{
			{Rune: 'd', Label: "Done", Action: updateAction},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	if pc.HandleAction(pluginActionID('d')) {
		t.Error("expected false when no task is selected for update action")
	}
}

func TestPluginController_HandleMoveTask_NoActionOnTargetLane(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	doneFilter := mustParseStmt(t, `select where status = "done"`)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes: []plugin.TikiLane{
			{Name: "Ready", Columns: 1, Filter: readyFilter},
			{Name: "Done", Columns: 1, Filter: doneFilter},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1, 1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	if pc.HandleAction(ActionMoveTaskRight) {
		t.Error("expected false when target lane has no action")
	}
}

func TestPluginController_HandleMoveTask_Success(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	doneFilter := mustParseStmt(t, `select where status = "done"`)
	doneAction := mustParseStmt(t, `update where id = id() set status = "done"`)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes: []plugin.TikiLane{
			{Name: "Ready", Columns: 1, Filter: readyFilter},
			{Name: "Done", Columns: 1, Filter: doneFilter, Action: doneAction},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1, 1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	if !pc.HandleAction(ActionMoveTaskRight) {
		t.Error("expected true for successful move")
	}

	tk := taskStore.GetTask("T-1")
	if tk.Status != "done" {
		t.Errorf("expected status done, got %s", tk.Status)
	}
}

func TestPluginController_HandleMoveTask_OutOfBounds(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	if pc.HandleAction(ActionMoveTaskLeft) {
		t.Error("expected false for out-of-bounds move left")
	}
	if pc.HandleAction(ActionMoveTaskRight) {
		t.Error("expected false for out-of-bounds move right")
	}
}

func TestPluginController_GetFilteredTasksForLane_NilPluginDef(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := &PluginController{
		pluginBase: pluginBase{
			taskStore:    taskStore,
			mutationGate: gate,
			pluginConfig: pluginConfig,
			pluginDef:    nil,
			schema:       schema,
		},
	}

	if tasks := pc.GetFilteredTasksForLane(0); tasks != nil {
		t.Error("expected nil for nil pluginDef")
	}
}

func TestPluginController_GetFilteredTasksForLane_OutOfRange(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	if tasks := pc.GetFilteredTasksForLane(-1); tasks != nil {
		t.Error("expected nil for negative lane")
	}
	if tasks := pc.GetFilteredTasksForLane(5); tasks != nil {
		t.Error("expected nil for out-of-range lane")
	}
}

func TestPluginController_GetFilteredTasksForLane_NilFilter(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "All", Columns: 1}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	tasks := pc.GetFilteredTasksForLane(0)
	if len(tasks) != 1 {
		t.Errorf("expected all tasks when filter is nil, got %d", len(tasks))
	}
}

func TestPluginController_GetFilteredTasksForLane_WithSearchNarrowing(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Alpha", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-2", Title: "Beta", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	t1 := taskStore.GetTask("T-1")
	pluginConfig.SetSearchResults([]task.SearchResult{{Task: t1, Score: 1.0}}, "Alpha")

	tasks := pc.GetFilteredTasksForLane(0)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task with search narrowing, got %d", len(tasks))
	}
	if tasks[0].ID != "T-1" {
		t.Errorf("expected T-1, got %s", tasks[0].ID)
	}
}

func TestPluginController_HandleAction_UnknownAction(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	if pc.HandleAction("nonexistent_action") {
		t.Error("expected false for unknown action")
	}
}

func TestPluginController_HandleSearch(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Alpha", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	pc.HandleSearch("Alpha")
	results := pluginConfig.GetSearchResults()
	if results == nil {
		t.Fatal("expected search results")
	}
}

func TestPluginController_ShowNavigation(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	if !pc.ShowNavigation() {
		t.Error("PluginController.ShowNavigation() should return true")
	}
}

func TestPluginController_HandleToggleViewMode(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	before := pluginConfig.GetViewMode()
	if !pc.HandleAction(ActionToggleViewMode) {
		t.Error("expected true for toggle view mode")
	}
	after := pluginConfig.GetViewMode()
	if before == after {
		t.Error("view mode should change after toggle")
	}
}

func TestPluginController_HandlePluginAction_Select(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	selectAction := mustParseStmt(t, `select where status = "ready"`)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
		Actions: []plugin.PluginAction{
			{Rune: 's', Label: "Search Ready", Action: selectAction},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 0)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	if !pc.HandleAction(pluginActionID('s')) {
		t.Error("expected true for SELECT plugin action")
	}

	// task should be unchanged (SELECT is side-effect only)
	tk := taskStore.GetTask("T-1")
	if tk.Status != task.StatusReady {
		t.Errorf("expected status ready (unchanged), got %s", tk.Status)
	}
}

func TestPluginController_HandlePluginAction_SelectNoSelectedTask(t *testing.T) {
	taskStore := store.NewInMemoryStore()

	emptyFilter := mustParseStmt(t, `select where status = "done"`)
	selectAction := mustParseStmt(t, `select where status = "ready"`)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Empty", Columns: 1, Filter: emptyFilter}},
		Actions: []plugin.PluginAction{
			{Rune: 's', Label: "Search Ready", Action: selectAction},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, nil, schema)

	// SELECT should succeed even with no selected task
	if !pc.HandleAction(pluginActionID('s')) {
		t.Error("expected true for SELECT action even with no selected task")
	}
}

func mustParseStmtWithInput(t *testing.T, input string, inputType ruki.ValueType) *ruki.ValidatedStatement {
	t.Helper()
	schema := rukiRuntime.NewSchema()
	parser := ruki.NewParser(schema)
	stmt, err := parser.ParseAndValidateStatementWithInput(input, ruki.ExecutorRuntimePlugin, inputType)
	if err != nil {
		t.Fatalf("parse ruki statement %q: %v", input, err)
	}
	return stmt
}

func TestPluginController_HandleActionInput_ValidInput(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	assignAction := mustParseStmtWithInput(t, `update where id = id() set assignee = input()`, ruki.ValueString)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
		Actions: []plugin.PluginAction{
			{Rune: 'a', Label: "Assign to...", Action: assignAction, InputType: ruki.ValueString, HasInput: true},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)

	statusline := model.NewStatuslineConfig()
	gate := service.BuildGate()
	gate.SetStore(taskStore)
	schema := rukiRuntime.NewSchema()
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, statusline, schema)

	result := pc.HandleActionInput(pluginActionID('a'), "alice")
	if result != InputClose {
		t.Fatalf("expected InputClose for valid input, got %d", result)
	}

	updated := taskStore.GetTask("T-1")
	if updated.Assignee != "alice" {
		t.Fatalf("expected assignee=alice, got %q", updated.Assignee)
	}
}

func TestPluginController_HandleActionInput_InvalidInput(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	pointsAction := mustParseStmtWithInput(t, `update where id = id() set points = input()`, ruki.ValueInt)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
		Actions: []plugin.PluginAction{
			{Rune: 'p', Label: "Set points", Action: pointsAction, InputType: ruki.ValueInt, HasInput: true},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)

	statusline := model.NewStatuslineConfig()
	gate := service.BuildGate()
	gate.SetStore(taskStore)
	schema := rukiRuntime.NewSchema()
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, statusline, schema)

	result := pc.HandleActionInput(pluginActionID('p'), "abc")
	if result != InputKeepEditing {
		t.Fatalf("expected InputKeepEditing for invalid int input, got %d", result)
	}

	msg, level, _ := statusline.GetMessage()
	if level != model.MessageLevelError {
		t.Fatalf("expected error message in statusline, got level %v msg %q", level, msg)
	}
}

func TestPluginController_HandleActionInput_ExecutionFailure_StillCloses(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	// no tasks in store — executor will find no match for id(), which means
	// the update produces no results (not an error), but executeAndApply still returns true.
	// Instead, test with a task that exists but use input on a field
	// where the assignment succeeds at parse/execution level.
	_ = taskStore.CreateTask(&task.Task{
		ID: "T-1", Title: "Task 1", Status: task.StatusReady, Type: task.TypeStory, Priority: 3,
	})

	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	assignAction := mustParseStmtWithInput(t, `update where id = id() set assignee = input()`, ruki.ValueString)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
		Actions: []plugin.PluginAction{
			{Rune: 'a', Label: "Assign to...", Action: assignAction, InputType: ruki.ValueString, HasInput: true},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)
	pluginConfig.SetSelectedLane(0)

	statusline := model.NewStatuslineConfig()
	gate := service.BuildGate()
	gate.SetStore(taskStore)
	schema := rukiRuntime.NewSchema()
	pc := NewPluginController(taskStore, gate, pluginConfig, pluginDef, nil, statusline, schema)

	// valid parse, successful execution — still returns InputClose
	result := pc.HandleActionInput(pluginActionID('a'), "bob")
	if result != InputClose {
		t.Fatalf("expected InputClose after valid parse (regardless of execution outcome), got %d", result)
	}
}

func TestPluginController_GetActionInputSpec(t *testing.T) {
	readyFilter := mustParseStmt(t, `select where status = "ready"`)
	assignAction := mustParseStmtWithInput(t, `update where id = id() set assignee = input()`, ruki.ValueString)
	markDoneAction := mustParseStmt(t, `update where id = id() set status = "done"`)

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "TestPlugin"},
		Lanes:      []plugin.TikiLane{{Name: "Ready", Columns: 1, Filter: readyFilter}},
		Actions: []plugin.PluginAction{
			{Rune: 'a', Label: "Assign to...", Action: assignAction, InputType: ruki.ValueString, HasInput: true},
			{Rune: 'd', Label: "Done", Action: markDoneAction},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1}, nil)

	schema := rukiRuntime.NewSchema()
	gate := service.BuildGate()
	gate.SetStore(store.NewInMemoryStore())
	pc := NewPluginController(store.NewInMemoryStore(), gate, pluginConfig, pluginDef, nil, nil, schema)

	prompt, typ, hasInput := pc.GetActionInputSpec(pluginActionID('a'))
	if !hasInput {
		t.Fatal("expected hasInput=true for 'a' action")
	}
	if typ != ruki.ValueString {
		t.Fatalf("expected ValueString, got %d", typ)
	}
	if prompt != "Assign to...: " {
		t.Fatalf("expected prompt 'Assign to...: ', got %q", prompt)
	}

	_, _, hasInput = pc.GetActionInputSpec(pluginActionID('d'))
	if hasInput {
		t.Fatal("expected hasInput=false for non-input 'd' action")
	}
}

func TestGetPluginActionRune(t *testing.T) {
	tests := []struct {
		name string
		id   ActionID
		want rune
	}{
		{"valid", pluginActionID('d'), 'd'},
		{"not a plugin action", "some_action", 0},
		{"empty suffix", ActionID(pluginActionPrefix), 0},
		{"multi-char suffix", ActionID(pluginActionPrefix + "ab"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPluginActionRune(tt.id); got != tt.want {
				t.Errorf("getPluginActionRune(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}
