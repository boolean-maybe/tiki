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

func TestEnsureFirstNonEmptyPaneSelectionSelectsFirstTask(t *testing.T) {
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
		Panes: []plugin.TikiPane{
			{Name: "Empty", Columns: 1, Filter: emptyFilter},
			{Name: "Todo", Columns: 1, Filter: todoFilter},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetPaneLayout([]int{1, 1})
	pluginConfig.SetSelectedPane(0)
	pluginConfig.SetSelectedIndexForPane(0, 1)

	pc := NewPluginController(taskStore, pluginConfig, pluginDef, nil)
	pc.EnsureFirstNonEmptyPaneSelection()

	if pluginConfig.GetSelectedPane() != 1 {
		t.Fatalf("expected selected pane 1, got %d", pluginConfig.GetSelectedPane())
	}
	if pluginConfig.GetSelectedIndexForPane(1) != 0 {
		t.Fatalf("expected selected index 0, got %d", pluginConfig.GetSelectedIndexForPane(1))
	}
}

func TestEnsureFirstNonEmptyPaneSelectionKeepsCurrentPane(t *testing.T) {
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
		Panes: []plugin.TikiPane{
			{Name: "First", Columns: 1, Filter: todoFilter},
			{Name: "Second", Columns: 1, Filter: todoFilter},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetPaneLayout([]int{1, 1})
	pluginConfig.SetSelectedPane(1)
	pluginConfig.SetSelectedIndexForPane(1, 0)

	pc := NewPluginController(taskStore, pluginConfig, pluginDef, nil)
	pc.EnsureFirstNonEmptyPaneSelection()

	if pluginConfig.GetSelectedPane() != 1 {
		t.Fatalf("expected selected pane 1, got %d", pluginConfig.GetSelectedPane())
	}
	if pluginConfig.GetSelectedIndexForPane(1) != 0 {
		t.Fatalf("expected selected index 0, got %d", pluginConfig.GetSelectedIndexForPane(1))
	}
}

func TestEnsureFirstNonEmptyPaneSelectionNoTasks(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	emptyFilter, err := filter.ParseFilter("status = 'done'")
	if err != nil {
		t.Fatalf("parse filter: %v", err)
	}

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{
			Name: "TestPlugin",
		},
		Panes: []plugin.TikiPane{
			{Name: "Empty", Columns: 1, Filter: emptyFilter},
			{Name: "StillEmpty", Columns: 1, Filter: emptyFilter},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetPaneLayout([]int{1, 1})
	pluginConfig.SetSelectedPane(1)
	pluginConfig.SetSelectedIndexForPane(1, 2)

	pc := NewPluginController(taskStore, pluginConfig, pluginDef, nil)
	pc.EnsureFirstNonEmptyPaneSelection()

	if pluginConfig.GetSelectedPane() != 1 {
		t.Fatalf("expected selected pane 1, got %d", pluginConfig.GetSelectedPane())
	}
	if pluginConfig.GetSelectedIndexForPane(1) != 2 {
		t.Fatalf("expected selected index 2, got %d", pluginConfig.GetSelectedIndexForPane(1))
	}
}

func TestPaneSwitchSelectsTopOfViewport(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	// Create tasks for two panes
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
		Panes: []plugin.TikiPane{
			{Name: "Ready", Columns: 1, Filter: readyFilter},
			{Name: "InProgress", Columns: 1, Filter: inProgressFilter},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetPaneLayout([]int{1, 1})

	// Start in pane 0 (Ready), with selection at index 2
	pluginConfig.SetSelectedPane(0)
	pluginConfig.SetSelectedIndexForPane(0, 2)

	// Simulate that pane 1 has been scrolled to offset 3
	pluginConfig.SetScrollOffsetForPane(1, 3)

	pc := NewPluginController(taskStore, pluginConfig, pluginDef, nil)

	// Navigate right to pane 1
	pc.HandleAction(ActionNavRight)

	// Should be in pane 1
	if pluginConfig.GetSelectedPane() != 1 {
		t.Fatalf("expected selected pane 1, got %d", pluginConfig.GetSelectedPane())
	}

	// Selection should be at scroll offset (top of viewport), not stale index
	if pluginConfig.GetSelectedIndexForPane(1) != 3 {
		t.Errorf("expected selection at scroll offset 3, got %d", pluginConfig.GetSelectedIndexForPane(1))
	}
}

func TestPaneSwitchClampsScrollOffsetToTaskCount(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	// Create 3 tasks in pane 1 only
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

	// Pane 0 is empty, pane 1 has 3 tasks
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
		Panes: []plugin.TikiPane{
			{Name: "Empty", Columns: 1, Filter: emptyFilter},
			{Name: "InProgress", Columns: 1, Filter: inProgressFilter},
		},
	}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetPaneLayout([]int{1, 1})

	// Start in pane 1
	pluginConfig.SetSelectedPane(1)
	pluginConfig.SetSelectedIndexForPane(1, 0)

	// Set a stale scroll offset that exceeds the task count
	pluginConfig.SetScrollOffsetForPane(1, 10)

	pc := NewPluginController(taskStore, pluginConfig, pluginDef, nil)

	// Navigate left (to empty pane, will skip to... well, nowhere)
	// Then try to go right from a fresh setup
	pluginConfig.SetSelectedPane(0)
	pluginConfig.SetScrollOffsetForPane(1, 10) // stale offset > task count

	pc.HandleAction(ActionNavRight)

	// Should be in pane 1
	if pluginConfig.GetSelectedPane() != 1 {
		t.Fatalf("expected selected pane 1, got %d", pluginConfig.GetSelectedPane())
	}

	// Selection should be clamped to last valid index (2, since 3 tasks)
	selectedIdx := pluginConfig.GetSelectedIndexForPane(1)
	if selectedIdx < 0 || selectedIdx >= 3 {
		t.Errorf("expected selection clamped to valid range [0,2], got %d", selectedIdx)
	}
}
