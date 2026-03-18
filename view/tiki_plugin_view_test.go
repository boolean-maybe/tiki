package view

import (
	"fmt"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

func TestPluginViewRefreshResetsNonSelectedLaneScrollOffset(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1, 1})

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{
			Name: "TestPlugin",
		},
		Lanes: []plugin.TikiLane{
			{Name: "Lane0", Columns: 1},
			{Name: "Lane1", Columns: 1},
		},
	}

	tasks := make([]*task.Task, 10)
	for i := range tasks {
		tasks[i] = &task.Task{
			ID:     fmt.Sprintf("T-%d", i),
			Title:  fmt.Sprintf("Task %d", i),
			Status: task.StatusReady,
			Type:   task.TypeStory,
		}
	}

	pv := NewPluginView(taskStore, pluginConfig, pluginDef, func(lane int) []*task.Task {
		return tasks
	}, nil, controller.PluginViewActions(), true)

	itemHeight := config.TaskBoxHeight
	for _, lb := range pv.laneBoxes {
		lb.SetRect(0, 0, 80, itemHeight*5)
	}

	// select last task in lane 0 to force scroll offset
	pluginConfig.SetSelectedLane(0)
	pluginConfig.SetSelectedIndexForLane(0, len(tasks)-1)
	pv.refresh()

	if pv.laneBoxes[0].scrollOffset == 0 {
		t.Fatalf("expected lane 0 scroll offset > 0 after selecting last item")
	}

	// non-selected lane 1 must have scroll offset 0
	if pv.laneBoxes[1].scrollOffset != 0 {
		t.Fatalf("expected non-selected lane 1 scroll offset 0, got %d", pv.laneBoxes[1].scrollOffset)
	}

	// switch selection to lane 1, scroll it, then verify lane 0 resets
	pluginConfig.SetSelectedLane(1)
	pluginConfig.SetSelectedIndexForLane(1, len(tasks)-1)
	pv.refresh()

	if pv.laneBoxes[1].scrollOffset == 0 {
		t.Fatalf("expected lane 1 scroll offset > 0 after selecting last item")
	}
	if pv.laneBoxes[0].scrollOffset != 0 {
		t.Fatalf("expected non-selected lane 0 scroll offset 0, got %d", pv.laneBoxes[0].scrollOffset)
	}
}

func TestPluginViewRefreshPreservesScrollOffset(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pluginConfig.SetLaneLayout([]int{1})

	pluginDef := &plugin.TikiPlugin{
		BasePlugin: plugin.BasePlugin{
			Name: "TestPlugin",
		},
		Lanes: []plugin.TikiLane{
			{Name: "Lane", Columns: 1},
		},
	}

	tasks := make([]*task.Task, 10)
	for i := range tasks {
		tasks[i] = &task.Task{
			ID:     fmt.Sprintf("T-%d", i),
			Title:  fmt.Sprintf("Task %d", i),
			Status: task.StatusReady,
			Type:   task.TypeStory,
		}
	}

	pv := NewPluginView(taskStore, pluginConfig, pluginDef, func(lane int) []*task.Task {
		return tasks
	}, nil, controller.PluginViewActions(), true)

	if len(pv.laneBoxes) != 1 {
		t.Fatalf("expected 1 lane box, got %d", len(pv.laneBoxes))
	}

	lane := pv.laneBoxes[0]
	itemHeight := config.TaskBoxHeight
	lane.SetRect(0, 0, 80, itemHeight*5)

	pluginConfig.SetSelectedIndexForLane(0, len(tasks)-1)
	pv.refresh()

	expectedScrollOffset := len(tasks) - 5
	if lane.scrollOffset != expectedScrollOffset {
		t.Fatalf("expected scrollOffset %d, got %d", expectedScrollOffset, lane.scrollOffset)
	}

	laneBefore := lane
	pluginConfig.SetSelectedIndexForLane(0, len(tasks)-2)
	pv.refresh()

	if pv.laneBoxes[0] != laneBefore {
		t.Fatalf("expected lane list to be reused across refresh")
	}

	if lane.scrollOffset != expectedScrollOffset {
		t.Fatalf("expected scrollOffset to remain %d, got %d", expectedScrollOffset, lane.scrollOffset)
	}
}
