package controller

import (
	"fmt"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/plugin/filter"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

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
	pluginConfig.SetLaneLayout([]int{1, 1})
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 1)

	pc := NewPluginController(taskStore, pluginConfig, pluginDef, nil)
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
	pluginConfig.SetLaneLayout([]int{1, 1})
	pluginConfig.SetSelectedLane(1)
	pluginConfig.SetSelectedIndexForLane(1, 0)

	pc := NewPluginController(taskStore, pluginConfig, pluginDef, nil)
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
	pluginConfig.SetLaneLayout([]int{1, 1})
	pluginConfig.SetSelectedLane(1)
	pluginConfig.SetSelectedIndexForLane(1, 2)

	pc := NewPluginController(taskStore, pluginConfig, pluginDef, nil)
	pc.EnsureFirstNonEmptyLaneSelection()

	if pluginConfig.GetSelectedLane() != 1 {
		t.Fatalf("expected selected lane 1, got %d", pluginConfig.GetSelectedLane())
	}
	if pluginConfig.GetSelectedIndexForLane(1) != 2 {
		t.Fatalf("expected selected index 2, got %d", pluginConfig.GetSelectedIndexForLane(1))
	}
}

func TestLaneSwitchSelectsTopOfViewport(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	// Create tasks for two lanes
	for i := 1; i <= 10; i++ {
		status := task.StatusReady
		if i > 5 {
			status = task.StatusInProgress
		}
		if err := taskStore.CreateTask(&task.Task{
			ID:     fmt.Sprintf("T-%d", i),
			Title:  "Task",
			Status: status,
			Type:   task.TypeStory,
		}); err != nil {
			t.Fatalf("create task: %v", err)
		}
	}

	readyFilter, err := filter.ParseFilter("status = 'ready'")
	if err != nil {
		t.Fatalf("parse filter: %v", err)
	}
	inProgressFilter, err := filter.ParseFilter("status = 'in_progress'")
	if err != nil {
		t.Fatalf("parse filter: %v", err)
	}

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{
			Name: "TestPlugin",
		},
		Lanes: []plugin.TikiLane{
			{Name: "Ready", Columns: 1, Filter: readyFilter},
			{Name: "InProgress", Columns: 1, Filter: inProgressFilter},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1, 1})

	// Start in lane 0 (Ready), with selection at index 2
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, 2)

	// Simulate that lane 1 has been scrolled to offset 3
	pluginConfig.SetScrollOffsetForLane(1, 3)

	pc := NewPluginController(taskStore, pluginConfig, pluginDef, nil)

	// Navigate right to lane 1
	pc.HandleAction(ActionNavRight)

	// Should be in lane 1
	if pluginConfig.GetSelectedLane() != 1 {
		t.Fatalf("expected selected lane 1, got %d", pluginConfig.GetSelectedLane())
	}

	// Selection should be at scroll offset (top of viewport), not stale index
	if pluginConfig.GetSelectedIndexForLane(1) != 3 {
		t.Errorf("expected selection at scroll offset 3, got %d", pluginConfig.GetSelectedIndexForLane(1))
	}
}

func TestLaneSwitchClampsScrollOffsetToTaskCount(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	// Create 3 tasks in lane 1 only
	for i := 1; i <= 3; i++ {
		if err := taskStore.CreateTask(&task.Task{
			ID:     fmt.Sprintf("T-%d", i),
			Title:  "Task",
			Status: task.StatusInProgress,
			Type:   task.TypeStory,
		}); err != nil {
			t.Fatalf("create task: %v", err)
		}
	}

	// Lane 0 is empty, lane 1 has 3 tasks
	emptyFilter, err := filter.ParseFilter("status = 'ready'")
	if err != nil {
		t.Fatalf("parse filter: %v", err)
	}
	inProgressFilter, err := filter.ParseFilter("status = 'in_progress'")
	if err != nil {
		t.Fatalf("parse filter: %v", err)
	}

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{
			Name: "TestPlugin",
		},
		Lanes: []plugin.TikiLane{
			{Name: "Empty", Columns: 1, Filter: emptyFilter},
			{Name: "InProgress", Columns: 1, Filter: inProgressFilter},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1, 1})

	// Start in lane 1
	pluginConfig.SetSelectedLane(1)
	pluginConfig.SetSelectedIndexForLane(1, 0)

	// Set a stale scroll offset that exceeds the task count
	pluginConfig.SetScrollOffsetForLane(1, 10)

	pc := NewPluginController(taskStore, pluginConfig, pluginDef, nil)

	// Navigate left (to empty lane, will skip to... well, nowhere)
	// Then try to go right from a fresh setup
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetScrollOffsetForLane(1, 10) // stale offset > task count

	pc.HandleAction(ActionNavRight)

	// Should be in lane 1
	if pluginConfig.GetSelectedLane() != 1 {
		t.Fatalf("expected selected lane 1, got %d", pluginConfig.GetSelectedLane())
	}

	// Selection should be clamped to last valid index (2, since 3 tasks)
	selectedIdx := pluginConfig.GetSelectedIndexForLane(1)
	if selectedIdx < 0 || selectedIdx >= 3 {
		t.Errorf("expected selection clamped to valid range [0,2], got %d", selectedIdx)
	}
}
