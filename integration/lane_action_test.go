package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

func TestPluginView_MoveTaskAppliesLaneAction(t *testing.T) {
	// create a temp workflow.yaml with the test plugin
	tmpDir := t.TempDir()
	workflowContent := `plugins:
  - name: ActionTest
    key: "F4"
    lanes:
      - name: Backlog
        columns: 1
        filter: status = 'backlog'
        action: status=backlog, tags-=[moved]
      - name: Done
        columns: 1
        filter: status = 'done'
        action: status=done, tags+=[moved]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflowContent), 0644); err != nil {
		t.Fatalf("failed to write workflow.yaml: %v", err)
	}

	// chdir so FindWorkflowFile() picks it up
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
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
