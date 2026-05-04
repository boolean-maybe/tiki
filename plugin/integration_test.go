package plugin

import (
	"testing"

	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/tiki"
)

func newTestExecutor() *ruki.Executor {
	schema := rukiRuntime.NewSchema()
	return ruki.NewExecutor(schema, func() string { return "testuser" },
		ruki.ExecutorRuntime{Mode: ruki.ExecutorRuntimePlugin})
}

// tasksAsTikis converts task fixtures for ruki execution and marks them as
// workflow-declaring so derived presence emits the schema fields these
// tests query against.
func tasksAsTikis(tasks []*task.Task) []*tiki.Tiki {
	out := make([]*tiki.Tiki, 0, len(tasks))
	for _, t := range tasks {
		if !t.IsWorkflow {
			c := t.Clone()
			c.IsWorkflow = true
			t = c
		}
		if tk := tiki.FromTask(t); tk != nil {
			out = append(out, tk)
		}
	}
	return out
}

// tikisAsTasks unwraps tikis to task.Task for assertions against legacy fields.
func tikisAsTasks(tks []*tiki.Tiki) []*task.Task {
	out := make([]*task.Task, 0, len(tks))
	for _, tk := range tks {
		if t := tiki.ToTask(tk); t != nil {
			out = append(out, t)
		}
	}
	return out
}

func TestPluginWithTagFilter(t *testing.T) {
	schema := rukiRuntime.NewSchema()
	pluginYAML := `
name: UI Tasks
foreground: "#ffffff"
background: "#0000ff"
key: U
kind: board
lanes:
  - name: UI
    filter: select where "ui" in tags or "ux" in tags or "design" in tags
`

	def, err := parsePluginYAML([]byte(pluginYAML), "test", schema)
	if err != nil {
		t.Fatalf("failed to parse plugin: %v", err)
	}

	if def.GetName() != "UI Tasks" {
		t.Errorf("expected name 'UI Tasks', got '%s'", def.GetName())
	}

	tp, ok := def.(*TikiPlugin)
	if !ok {
		t.Fatalf("expected TikiPlugin, got %T", def)
	}

	if len(tp.Lanes) != 1 || tp.Lanes[0].Filter == nil {
		t.Fatal("expected lane filter to be parsed")
	}

	allTasks := []*task.Task{
		{ID: "TIKI-000001", Title: "Design mockups", Tags: []string{"ui", "design"}, Status: task.StatusReady},
		{ID: "TIKI-000002", Title: "Backend API", Tags: []string{"backend", "api"}, Status: task.StatusReady},
		{ID: "TIKI-000003", Title: "UX Research", Tags: []string{"ux", "research"}, Status: task.StatusReady},
	}

	executor := newTestExecutor()
	result, err := executor.Execute(tp.Lanes[0].Filter, tasksAsTikis(allTasks))
	if err != nil {
		t.Fatalf("executor error: %v", err)
	}

	filtered := tikisAsTasks(result.Select.Tikis)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 matching tasks, got %d", len(filtered))
	}

	// task with "ui"+"design" and task with "ux" should match; "backend"+"api" should not
	ids := map[string]bool{}
	for _, tk := range filtered {
		ids[tk.ID] = true
	}
	if !ids["TIKI-000001"] {
		t.Error("expected task TIKI-000001 (ui, design tags) to match")
	}
	if ids["TIKI-000002"] {
		t.Error("expected task TIKI-000002 (backend, api tags) to NOT match")
	}
	if !ids["TIKI-000003"] {
		t.Error("expected task TIKI-000003 (ux tag) to match")
	}
}

func TestPluginWithComplexTagAndStatusFilter(t *testing.T) {
	schema := rukiRuntime.NewSchema()
	pluginYAML := `
name: Active Work
key: A
kind: board
lanes:
  - name: Active
    filter: select where ("ui" in tags or "backend" in tags) and status != "done" and status != "backlog"
`

	def, err := parsePluginYAML([]byte(pluginYAML), "test", schema)
	if err != nil {
		t.Fatalf("failed to parse plugin: %v", err)
	}

	tp, ok := def.(*TikiPlugin)
	if !ok {
		t.Fatalf("expected TikiPlugin, got %T", def)
	}

	allTasks := []*task.Task{
		{ID: "TIKI-000001", Tags: []string{"ui", "frontend"}, Status: task.StatusReady},
		{ID: "TIKI-000002", Tags: []string{"ui"}, Status: task.StatusDone},
		{ID: "TIKI-000003", Tags: []string{"docs", "testing"}, Status: task.StatusInProgress},
	}

	executor := newTestExecutor()
	result, err := executor.Execute(tp.Lanes[0].Filter, tasksAsTikis(allTasks))
	if err != nil {
		t.Fatalf("executor error: %v", err)
	}

	filtered := tikisAsTasks(result.Select.Tikis)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 matching task, got %d", len(filtered))
	}
	if filtered[0].ID != "TIKI-000001" {
		t.Errorf("expected TIKI-000001, got %s", filtered[0].ID)
	}
}

func TestPluginWithStatusFilter(t *testing.T) {
	schema := rukiRuntime.NewSchema()
	pluginYAML := `
name: In Progress Work
key: W
kind: board
lanes:
  - name: Active
    filter: select where status = "ready" or status = "inProgress"
`

	def, err := parsePluginYAML([]byte(pluginYAML), "test", schema)
	if err != nil {
		t.Fatalf("failed to parse plugin: %v", err)
	}

	tp, ok := def.(*TikiPlugin)
	if !ok {
		t.Fatalf("expected TikiPlugin, got %T", def)
	}

	testCases := []struct {
		name   string
		status task.Status
		expect bool
	}{
		{"ready status", task.StatusReady, true},
		{"inProgress status", task.StatusInProgress, true},
		{"done status", task.StatusDone, false},
		{"review status", task.StatusReview, false},
	}

	executor := newTestExecutor()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allTasks := []*task.Task{
				{ID: "TIKI-000001", Status: tc.status},
			}

			result, err := executor.Execute(tp.Lanes[0].Filter, tasksAsTikis(allTasks))
			if err != nil {
				t.Fatalf("executor error: %v", err)
			}

			got := len(tikisAsTasks(result.Select.Tikis)) > 0
			if got != tc.expect {
				t.Errorf("expected match=%v for status %s, got %v", tc.expect, tc.status, got)
			}
		})
	}
}
