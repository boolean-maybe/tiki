package integration

import (
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
	"github.com/spf13/viper"
)

func TestPluginView_MoveTaskAppliesLaneAction(t *testing.T) {
	originalPlugins := viper.Get("plugins")
	viper.Set("plugins", []plugin.PluginRef{
		{
			Name: "ActionTest",
			Key:  "F4",
			Lanes: []plugin.PluginLaneConfig{
				{
					Name:    "Backlog",
					Columns: 1,
					Filter:  "status = 'backlog'",
					Action:  "status=backlog, tags-=[moved]",
				},
				{
					Name:    "Done",
					Columns: 1,
					Filter:  "status = 'done'",
					Action:  "status=done, tags+=[moved]",
				},
			},
		},
	})
	t.Cleanup(func() {
		viper.Set("plugins", originalPlugins)
	})

	ta := testutil.NewTestApp(t)
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}
	defer ta.Cleanup()

	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Backlog Task", task.StatusBacklog, task.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Done Task", task.StatusDone, task.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	ta.NavController.PushView(model.MakePluginViewID("ActionTest"), nil)
	ta.Draw()

	ta.SendKey(tcell.KeyRight, 0, tcell.ModShift)

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	updated := ta.TaskStore.GetTask("TIKI-1")
	if updated == nil {
		t.Fatalf("expected task TIKI-1 to exist")
	}
	if updated.Status != task.StatusDone {
		t.Fatalf("expected status done, got %v", updated.Status)
	}
	if !containsTag(updated.Tags, "moved") {
		t.Fatalf("expected moved tag, got %v", updated.Tags)
	}

	ta.SendKey(tcell.KeyLeft, 0, tcell.ModShift)

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	updated = ta.TaskStore.GetTask("TIKI-1")
	if updated == nil {
		t.Fatalf("expected task TIKI-1 to exist")
	}
	if updated.Status != task.StatusBacklog {
		t.Fatalf("expected status backlog, got %v", updated.Status)
	}
	if containsTag(updated.Tags, "moved") {
		t.Fatalf("expected moved tag removed, got %v", updated.Tags)
	}
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}
