package plugin

import (
	"testing"

	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/ruki"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

func newTestExecutor() *ruki.Executor {
	schema := rukiRuntime.NewSchema()
	return ruki.NewExecutor(schema, func() string { return "testuser" },
		ruki.ExecutorRuntime{Mode: ruki.ExecutorRuntimePlugin})
}

func newWFTiki(id, status string, tags []string) *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.ID = id
	if status != "" {
		tk.Set(tikipkg.FieldStatus, status)
	}
	if len(tags) > 0 {
		tk.Set(tikipkg.FieldTags, tags)
	}
	return tk
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

	allTikis := []*tikipkg.Tiki{
		newWFTiki("T00001", "ready", []string{"ui", "design"}),
		newWFTiki("T00002", "ready", []string{"backend", "api"}),
		newWFTiki("T00003", "ready", []string{"ux", "research"}),
	}

	executor := newTestExecutor()
	result, err := executor.Execute(tp.Lanes[0].Filter, allTikis)
	if err != nil {
		t.Fatalf("executor error: %v", err)
	}

	filtered := result.Select.Tikis
	if len(filtered) != 2 {
		t.Fatalf("expected 2 matching tasks, got %d", len(filtered))
	}

	// task with "ui"+"design" and task with "ux" should match; "backend"+"api" should not
	ids := map[string]bool{}
	for _, tk := range filtered {
		ids[tk.ID] = true
	}
	if !ids["T00001"] {
		t.Error("expected T00001 (ui, design tags) to match")
	}
	if ids["T00002"] {
		t.Error("expected T00002 (backend, api tags) to NOT match")
	}
	if !ids["T00003"] {
		t.Error("expected T00003 (ux tag) to match")
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

	allTikis := []*tikipkg.Tiki{
		newWFTiki("T00001", "ready", []string{"ui", "frontend"}),
		newWFTiki("T00002", "done", []string{"ui"}),
		newWFTiki("T00003", "inProgress", []string{"docs", "testing"}),
	}

	executor := newTestExecutor()
	result, err := executor.Execute(tp.Lanes[0].Filter, allTikis)
	if err != nil {
		t.Fatalf("executor error: %v", err)
	}

	filtered := result.Select.Tikis
	if len(filtered) != 1 {
		t.Fatalf("expected 1 matching task, got %d", len(filtered))
	}
	if filtered[0].ID != "T00001" {
		t.Errorf("expected T00001, got %s", filtered[0].ID)
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
		status string
		expect bool
	}{
		{"ready status", "ready", true},
		{"inProgress status", "inProgress", true},
		{"done status", "done", false},
		{"review status", "review", false},
	}

	executor := newTestExecutor()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tikis := []*tikipkg.Tiki{newWFTiki("T00001", tc.status, nil)}

			result, err := executor.Execute(tp.Lanes[0].Filter, tikis)
			if err != nil {
				t.Fatalf("executor error: %v", err)
			}

			got := len(result.Select.Tikis) > 0
			if got != tc.expect {
				t.Errorf("expected match=%v for status %s, got %v", tc.expect, tc.status, got)
			}
		})
	}
}
