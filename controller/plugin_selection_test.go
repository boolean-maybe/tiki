package controller

import (
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
