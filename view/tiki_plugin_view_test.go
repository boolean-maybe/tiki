package view

import (
	"fmt"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

func TestPluginViewRefreshPreservesScrollOffset(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetPaneLayout([]int{1})

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{
			Name: "TestPlugin",
		},
		Panes: []plugin.TikiPane{
			{Name: "Pane", Columns: 1},
		},
	}

	tasks := make([]*task.Task, 10)
	for i := range tasks {
		tasks[i] = &task.Task{
			ID:     fmt.Sprintf("T-%d", i),
			Title:  fmt.Sprintf("Task %d", i),
			Status: task.StatusTodo,
			Type:   task.TypeStory,
		}
	}

	pv := NewPluginView(taskStore, pluginConfig, pluginDef, func(pane int) []*task.Task {
		return tasks
	}, nil)

	if len(pv.paneBoxes) != 1 {
		t.Fatalf("expected 1 pane box, got %d", len(pv.paneBoxes))
	}

	pane := pv.paneBoxes[0]
	itemHeight := config.TaskBoxHeight
	pane.SetRect(0, 0, 80, itemHeight*5)

	pluginConfig.SetSelectedIndexForPane(0, len(tasks)-1)
	pv.refresh()

	expectedScrollOffset := len(tasks) - 5
	if pane.scrollOffset != expectedScrollOffset {
		t.Fatalf("expected scrollOffset %d, got %d", expectedScrollOffset, pane.scrollOffset)
	}

	paneBefore := pane
	pluginConfig.SetSelectedIndexForPane(0, len(tasks)-2)
	pv.refresh()

	if pv.paneBoxes[0] != paneBefore {
		t.Fatalf("expected pane list to be reused across refresh")
	}

	if pane.scrollOffset != expectedScrollOffset {
		t.Fatalf("expected scrollOffset to remain %d, got %d", expectedScrollOffset, pane.scrollOffset)
	}
}
