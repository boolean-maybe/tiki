package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	"github.com/gdamore/tcell/v2"
)

func TestPluginView_MoveTaskAppliesLaneAction(t *testing.T) {
	// create a temp workflow.yaml with the test plugin
	tmpDir := t.TempDir()
	workflowContent := testWorkflowPreamble + `views:
  - name: ActionTest
    kind: board
    key: "F4"
    lanes:
      - name: Backlog
        columns: 1
        filter: select where status = "backlog"
        action: update where id = id() set status="backlog" tags=tags-["moved"]
      - name: Done
        columns: 1
        filter: select where status = "done"
        action: update where id = id() set status="done" tags=tags+["moved"]
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

	if err := testutil.CreateTestTask(ta.TaskDir, "000001", "Backlog Task", task.StatusBacklog, task.TypeStory); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "000002", "Done Task", task.StatusDone, task.TypeStory); err != nil {
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
	updated := ta.TaskStore.GetTiki("000001")
	if updated == nil {
		t.Fatalf("expected task TIKI-1 to exist")
		return
	}
	status, _, _ := updated.StringField("status")
	tags, _, _ := updated.StringSliceField(tikipkg.FieldTags)
	if status != string(task.StatusDone) {
		t.Fatalf("expected status done, got %v", status)
	}
	if !containsTag(tags, "moved") {
		t.Fatalf("expected moved tag, got %v", tags)
	}

	ta.SendKey(tcell.KeyLeft, 0, tcell.ModShift)

	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	updated = ta.TaskStore.GetTiki("000001")
	if updated == nil {
		t.Fatalf("expected task TIKI-1 to exist")
		return
	}
	status2, _, _ := updated.StringField("status")
	tags2, _, _ := updated.StringSliceField(tikipkg.FieldTags)
	if status2 != string(task.StatusBacklog) {
		t.Fatalf("expected status backlog, got %v", status2)
	}
	if containsTag(tags2, "moved") {
		t.Fatalf("expected moved tag removed, got %v", tags2)
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
