package plugin

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/task"
)

func TestPluginWithInFilter(t *testing.T) {
	// Test loading a plugin definition with IN filter
	pluginYAML := `
name: UI Tasks
foreground: "#ffffff"
background: "#0000ff"
key: U
panes:
  - name: UI
    filter: tags IN ['ui', 'ux', 'design']
`

	def, err := parsePluginYAML([]byte(pluginYAML), "test")
	if err != nil {
		t.Fatalf("Failed to parse plugin: %v", err)
	}

	if def.GetName() != "UI Tasks" {
		t.Errorf("Expected name 'UI Tasks', got '%s'", def.GetName())
	}

	tp, ok := def.(*TikiPlugin)
	if !ok {
		t.Fatalf("Expected TikiPlugin, got %T", def)
	}

	if len(tp.Panes) != 1 || tp.Panes[0].Filter == nil {
		t.Fatal("Expected pane filter to be parsed")
	}

	// Test filter evaluation with matching tasks
	matchingTask := &task.Task{
		ID:     "TIKI-1",
		Title:  "Design mockups",
		Tags:   []string{"ui", "design"},
		Status: task.StatusReady,
	}

	if !tp.Panes[0].Filter.Evaluate(matchingTask, time.Now(), "testuser") {
		t.Error("Expected filter to match task with 'ui' and 'design' tags")
	}

	// Test filter evaluation with non-matching tasks
	nonMatchingTask := &task.Task{
		ID:     "TIKI-2",
		Title:  "Backend API",
		Tags:   []string{"backend", "api"},
		Status: task.StatusReady,
	}

	if tp.Panes[0].Filter.Evaluate(nonMatchingTask, time.Now(), "testuser") {
		t.Error("Expected filter to NOT match task with 'backend' and 'api' tags")
	}

	// Test with task that has one matching tag
	partialMatchTask := &task.Task{
		ID:     "TIKI-3",
		Title:  "UX Research",
		Tags:   []string{"ux", "research"},
		Status: task.StatusReady,
	}

	if !tp.Panes[0].Filter.Evaluate(partialMatchTask, time.Now(), "testuser") {
		t.Error("Expected filter to match task with 'ux' tag")
	}
}

func TestPluginWithComplexInFilter(t *testing.T) {
	// Test plugin with combined filters
	pluginYAML := `
name: Active Work
key: A
panes:
  - name: Active
    filter: tags IN ['ui', 'backend'] AND status NOT IN ['done', 'cancelled']
`

	def, err := parsePluginYAML([]byte(pluginYAML), "test")
	if err != nil {
		t.Fatalf("Failed to parse plugin: %v", err)
	}

	tp, ok := def.(*TikiPlugin)
	if !ok {
		t.Fatalf("Expected TikiPlugin, got %T", def)
	}

	// Should match: has 'ui' tag and status is 'ready' (not done)
	matchingTask := &task.Task{
		ID:     "TIKI-1",
		Tags:   []string{"ui", "frontend"},
		Status: task.StatusReady,
	}

	if !tp.Panes[0].Filter.Evaluate(matchingTask, time.Now(), "testuser") {
		t.Error("Expected filter to match active UI task")
	}

	// Should NOT match: has 'ui' tag but status is 'done'
	doneTask := &task.Task{
		ID:     "TIKI-2",
		Tags:   []string{"ui"},
		Status: task.StatusDone,
	}

	if tp.Panes[0].Filter.Evaluate(doneTask, time.Now(), "testuser") {
		t.Error("Expected filter to NOT match done UI task")
	}

	// Should NOT match: status is active but no matching tags
	noTagsTask := &task.Task{
		ID:     "TIKI-3",
		Tags:   []string{"docs", "testing"},
		Status: task.StatusInProgress,
	}

	if tp.Panes[0].Filter.Evaluate(noTagsTask, time.Now(), "testuser") {
		t.Error("Expected filter to NOT match task without matching tags")
	}
}

func TestPluginWithStatusInFilter(t *testing.T) {
	// Test plugin filtering by status
	pluginYAML := `
name: In Progress Work
key: W
panes:
  - name: Active
    filter: status IN ['ready', 'in_progress', 'in_progress']
`

	def, err := parsePluginYAML([]byte(pluginYAML), "test")
	if err != nil {
		t.Fatalf("Failed to parse plugin: %v", err)
	}

	tp, ok := def.(*TikiPlugin)
	if !ok {
		t.Fatalf("Expected TikiPlugin, got %T", def)
	}

	testCases := []struct {
		name   string
		status task.Status
		expect bool
	}{
		{"todo status", task.StatusReady, true},
		{"in_progress status", task.StatusInProgress, true},
		{"blocked status", task.StatusInProgress, true},
		{"done status", task.StatusDone, false},
		{"review status", task.StatusReview, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			task := &task.Task{
				ID:     "TIKI-1",
				Status: tc.status,
			}

			result := tp.Panes[0].Filter.Evaluate(task, time.Now(), "testuser")
			if result != tc.expect {
				t.Errorf("Expected %v for status %s, got %v", tc.expect, tc.status, result)
			}
		})
	}
}
