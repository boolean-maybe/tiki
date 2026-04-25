package plugin

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
	"gopkg.in/yaml.v3"
)

func TestConvertLegacyFilter(t *testing.T) {
	tr := NewLegacyConfigTransformer()

	tests := []struct {
		name string
		old  string
		want string
	}{
		{
			name: "simple comparison",
			old:  "status = 'ready'",
			want: `select where status = "ready"`,
		},
		{
			name: "multi-condition and",
			old:  "status = 'ready' AND type != 'epic'",
			want: `select where status = "ready" and type != "epic"`,
		},
		{
			name: "time expression with NOW",
			old:  "NOW - UpdatedAt < 24hours",
			want: `select where now() - updatedAt < 24hour`,
		},
		{
			name: "duration plural weeks",
			old:  "NOW - CreatedAt < 1weeks",
			want: `select where now() - createdAt < 1week`,
		},
		{
			name: "tags IN array expansion",
			old:  "tags IN ['ui', 'charts']",
			want: `select where ("ui" in tags or "charts" in tags)`,
		},
		{
			name: "tags IN single element",
			old:  "tags IN ['ui']",
			want: `select where "ui" in tags`,
		},
		{
			name: "tags NOT IN array expansion",
			old:  "tags NOT IN ['ui', 'old']",
			want: `select where ("ui" not in tags and "old" not in tags)`,
		},
		{
			name: "status IN scalar",
			old:  "status IN ['ready', 'inProgress']",
			want: `select where status in ["ready", "inProgress"]`,
		},
		{
			name: "status NOT IN scalar",
			old:  "status NOT IN ['done']",
			want: `select where status not in ["done"]`,
		},
		{
			name: "NOT with parens",
			old:  "NOT (status = 'done')",
			want: `select where not (status = "done")`,
		},
		{
			name: "tag singular alias equality",
			old:  "tag = 'ui'",
			want: `select where "ui" in tags`,
		},
		{
			name: "tag singular alias IN",
			old:  "tag IN ['ui', 'charts']",
			want: `select where ("ui" in tags or "charts" in tags)`,
		},
		{
			name: "tag singular alias NOT IN",
			old:  "tag NOT IN ['ui', 'old']",
			want: `select where ("ui" not in tags and "old" not in tags)`,
		},
		{
			name: "CURRENT_USER",
			old:  "assignee = CURRENT_USER",
			want: `select where assignee = user()`,
		},
		{
			name: "double equals",
			old:  "priority == 3",
			want: `select where priority = 3`,
		},
		{
			name: "mixed case keywords",
			old:  "type = 'epic' And status = 'ready'",
			want: `select where type = "epic" and status = "ready"`,
		},
		{
			name: "numeric comparison",
			old:  "priority > 2",
			want: `select where priority > 2`,
		},
		{
			name: "empty string",
			old:  "",
			want: "",
		},
		{
			name: "passthrough already ruki",
			old:  "select where status = \"ready\"",
			want: "select where status = \"ready\"",
		},
		{
			name: "whitespace variations",
			old:  "  status  =  'ready'  ",
			want: `select where status  =  "ready"`,
		},
		{
			name: "field name normalization CreatedAt",
			old:  "CreatedAt > 0",
			want: `select where createdAt > 0`,
		},
		{
			name: "field name normalization Priority",
			old:  "Priority > 2",
			want: `select where priority > 2`,
		},
		{
			name: "dependsOn field normalization",
			old:  "DependsOn IN ['TIKI-ABC123']",
			want: `select where "TIKI-ABC123" in dependsOn`,
		},
		{
			name: "case variations Or",
			old:  "status = 'ready' Or status = 'done'",
			want: `select where status = "ready" or status = "done"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// passthrough check
			if tt.old != "" && isRukiFilter(tt.old) {
				if tt.old != tt.want {
					t.Errorf("passthrough mismatch: got %q, want %q", tt.old, tt.want)
				}
				return
			}
			got := tr.ConvertFilter(tt.old)
			if got != tt.want {
				t.Errorf("ConvertFilter(%q)\n  got:  %q\n  want: %q", tt.old, got, tt.want)
			}
		})
	}
}

func TestConvertLegacySort(t *testing.T) {
	tr := NewLegacyConfigTransformer()

	tests := []struct {
		name string
		old  string
		want string
	}{
		{
			name: "single field",
			old:  "Priority",
			want: "order by priority",
		},
		{
			name: "multi field",
			old:  "Priority, CreatedAt",
			want: "order by priority, createdAt",
		},
		{
			name: "DESC",
			old:  "UpdatedAt DESC",
			want: "order by updatedAt desc",
		},
		{
			name: "mixed",
			old:  "Priority, Points DESC",
			want: "order by priority, points desc",
		},
		{
			name: "empty",
			old:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tr.ConvertSort(tt.old)
			if got != tt.want {
				t.Errorf("ConvertSort(%q)\n  got:  %q\n  want: %q", tt.old, got, tt.want)
			}
		})
	}
}

func TestConvertLegacySortMerging(t *testing.T) {
	tests := []struct {
		name       string
		sort       string
		lanes      []PluginLaneConfig
		wantFilter []string // expected filter for each lane after merge
	}{
		{
			name: "sort + non-empty lane filter",
			sort: "Priority",
			lanes: []PluginLaneConfig{
				{Name: "ready", Filter: "status = 'ready'"},
			},
			wantFilter: []string{`select where status = "ready" order by priority`},
		},
		{
			name: "sort + empty lane filter",
			sort: "Priority",
			lanes: []PluginLaneConfig{
				{Name: "all", Filter: ""},
			},
			wantFilter: []string{"select order by priority"},
		},
		{
			name: "sort + lane already has order by",
			sort: "Priority",
			lanes: []PluginLaneConfig{
				{Name: "custom", Filter: "select where status = \"ready\" order by updatedAt desc"},
			},
			wantFilter: []string{`select where status = "ready" order by updatedAt desc`},
		},
		{
			name: "no sort field",
			sort: "",
			lanes: []PluginLaneConfig{
				{Name: "ready", Filter: "status = 'ready'"},
			},
			wantFilter: []string{`select where status = "ready"`},
		},
	}

	tr := NewLegacyConfigTransformer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &pluginFileConfig{
				Name:  "test",
				Sort:  tt.sort,
				Lanes: tt.lanes,
			}
			tr.ConvertPluginConfig(cfg)

			if len(cfg.Lanes) != len(tt.wantFilter) {
				t.Fatalf("lane count mismatch: got %d, want %d", len(cfg.Lanes), len(tt.wantFilter))
			}
			for i, want := range tt.wantFilter {
				if cfg.Lanes[i].Filter != want {
					t.Errorf("lane %d filter:\n  got:  %q\n  want: %q", i, cfg.Lanes[i].Filter, want)
				}
			}
			if tt.sort != "" && cfg.Sort != "" {
				t.Error("Sort field was not cleared after merging")
			}
		})
	}
}

func TestConvertLegacyAction(t *testing.T) {
	tr := NewLegacyConfigTransformer()

	tests := []struct {
		name    string
		old     string
		want    string
		wantErr bool
	}{
		{
			name: "simple status",
			old:  "status = 'ready'",
			want: `update where id = id() set status="ready"`,
		},
		{
			name: "multiple assignments",
			old:  "status = 'backlog', priority = 1",
			want: `update where id = id() set status="backlog" priority=1`,
		},
		{
			name: "tags add",
			old:  "tags+=[frontend, 'needs review']",
			want: `update where id = id() set tags=tags+["frontend", "needs review"]`,
		},
		{
			name: "tags remove",
			old:  "tags-=[old]",
			want: `update where id = id() set tags=tags-["old"]`,
		},
		{
			name: "dependsOn add",
			old:  "dependsOn+=[TIKI-ABC123]",
			want: `update where id = id() set dependsOn=dependsOn+["TIKI-ABC123"]`,
		},
		{
			name: "CURRENT_USER",
			old:  "assignee=CURRENT_USER",
			want: `update where id = id() set assignee=user()`,
		},
		{
			name: "unquoted string value",
			old:  "status=done",
			want: `update where id = id() set status="done"`,
		},
		{
			name: "bare identifiers in brackets",
			old:  "tags+=[frontend, backend]",
			want: `update where id = id() set tags=tags+["frontend", "backend"]`,
		},
		{
			name: "integer value",
			old:  "priority=2",
			want: `update where id = id() set priority=2`,
		},
		{
			name: "multiple mixed",
			old:  "status=done, type=bug, priority=2, points=3, assignee='Alice', tags+=[frontend], dependsOn+=[TIKI-ABC123]",
			want: `update where id = id() set status="done" type="bug" priority=2 points=3 assignee="Alice" tags=tags+["frontend"] dependsOn=dependsOn+["TIKI-ABC123"]`,
		},
		{
			name: "empty",
			old:  "",
			want: "",
		},
		{
			name: "passthrough already ruki",
			old:  `update where id = id() set status="ready"`,
			want: `update where id = id() set status="ready"`,
		},
		{
			name:    "malformed brackets",
			old:     "tags+=[frontend",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.old != "" && isRukiAction(tt.old) {
				// passthrough — already ruki
				if tt.old != tt.want {
					t.Errorf("passthrough mismatch: got %q, want %q", tt.old, tt.want)
				}
				return
			}
			got, err := tr.ConvertAction(tt.old)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ConvertAction(%q) expected error, got %q", tt.old, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ConvertAction(%q) unexpected error: %v", tt.old, err)
			}
			if got != tt.want {
				t.Errorf("ConvertAction(%q)\n  got:  %q\n  want: %q", tt.old, got, tt.want)
			}
		})
	}
}

func TestConvertLegacyPluginActions(t *testing.T) {
	tr := NewLegacyConfigTransformer()

	cfg := &pluginFileConfig{
		Name: "test",
		Lanes: []PluginLaneConfig{
			{Name: "all", Filter: ""},
		},
		Actions: []PluginActionConfig{
			{Key: "b", Label: "Ready", Action: "status = 'ready'"},
			{Key: "c", Label: "Done", Action: `update where id = id() set status="done"`},
		},
	}

	tr.ConvertPluginConfig(cfg)

	if cfg.Actions[0].Action != `update where id = id() set status="ready"` {
		t.Errorf("action 0 not converted: %q", cfg.Actions[0].Action)
	}
	if cfg.Actions[1].Action != `update where id = id() set status="done"` {
		t.Errorf("action 1 was modified: %q", cfg.Actions[1].Action)
	}
}

func TestConvertLegacyMixedFormats(t *testing.T) {
	tr := NewLegacyConfigTransformer()

	cfg := &pluginFileConfig{
		Name: "test",
		Lanes: []PluginLaneConfig{
			{Name: "old", Filter: "status = 'ready'", Action: "status = 'inProgress'"},
			{Name: "new", Filter: `select where status = "done"`, Action: `update where id = id() set status="done"`},
		},
	}

	tr.ConvertPluginConfig(cfg)

	// old lane should be converted
	if cfg.Lanes[0].Filter != `select where status = "ready"` {
		t.Errorf("old lane filter not converted: %q", cfg.Lanes[0].Filter)
	}
	if cfg.Lanes[0].Action != `update where id = id() set status="inProgress"` {
		t.Errorf("old lane action not converted: %q", cfg.Lanes[0].Action)
	}

	// new lane should be unchanged
	if cfg.Lanes[1].Filter != `select where status = "done"` {
		t.Errorf("new lane filter was modified: %q", cfg.Lanes[1].Filter)
	}
	if cfg.Lanes[1].Action != `update where id = id() set status="done"` {
		t.Errorf("new lane action was modified: %q", cfg.Lanes[1].Action)
	}
}

func TestConvertLegacyActionPassthroughPrefixes(t *testing.T) {
	// create and delete prefixes must not be re-converted
	tests := []struct {
		name   string
		action string
	}{
		{name: "create prefix", action: `create where type = "bug" set status="ready"`},
		{name: "delete prefix", action: `delete where id = id()`},
		{name: "update prefix", action: `update where id = id() set status="done"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !isRukiAction(tt.action) {
				t.Errorf("isRukiAction(%q) returned false, expected true", tt.action)
			}
		})
	}
}

func TestConvertLegacyEdgeCases(t *testing.T) {
	tr := NewLegacyConfigTransformer()

	t.Run("tag vs tags word boundary", func(t *testing.T) {
		// "tags IN" should not trigger the singular "tag" alias path
		got := tr.ConvertFilter("tags IN ['ui', 'charts']")
		want := `select where ("ui" in tags or "charts" in tags)`
		if got != want {
			t.Errorf("got:  %q\nwant: %q", got, want)
		}
	})

	t.Run("tag NOT IN treated as tags NOT IN", func(t *testing.T) {
		got := tr.ConvertFilter("tag NOT IN ['ui', 'old']")
		want := `select where ("ui" not in tags and "old" not in tags)`
		if got != want {
			t.Errorf("got:  %q\nwant: %q", got, want)
		}
	})

	t.Run("array NOT IN single element no parens", func(t *testing.T) {
		got := tr.ConvertFilter("tags NOT IN ['old']")
		want := `select where "old" not in tags`
		if got != want {
			t.Errorf("got:  %q\nwant: %q", got, want)
		}
	})

	t.Run("array IN single element no parens", func(t *testing.T) {
		got := tr.ConvertFilter("tags IN ['ui']")
		want := `select where "ui" in tags`
		if got != want {
			t.Errorf("got:  %q\nwant: %q", got, want)
		}
	})

	t.Run("values with spaces in quotes", func(t *testing.T) {
		got := tr.ConvertFilter("status = 'in progress'")
		want := `select where status = "in progress"`
		if got != want {
			t.Errorf("got:  %q\nwant: %q", got, want)
		}
	})

	t.Run("already double-quoted values", func(t *testing.T) {
		got := tr.ConvertFilter(`status = "ready"`)
		want := `select where status = "ready"`
		if got != want {
			t.Errorf("got:  %q\nwant: %q", got, want)
		}
	})

	t.Run("dependsOn single element", func(t *testing.T) {
		got := tr.ConvertFilter("DependsOn IN ['TIKI-ABC123']")
		want := `select where "TIKI-ABC123" in dependsOn`
		if got != want {
			t.Errorf("got:  %q\nwant: %q", got, want)
		}
	})

	t.Run("field name inside quotes not normalized", func(t *testing.T) {
		// "Type" as a string value must not be lowercased to "type"
		got := tr.ConvertFilter("title = 'Type'")
		want := `select where title = "Type"`
		if got != want {
			t.Errorf("got:  %q\nwant: %q", got, want)
		}
	})

	t.Run("field name inside double quotes not normalized", func(t *testing.T) {
		got := tr.ConvertFilter(`assignee = "Status"`)
		want := `select where assignee = "Status"`
		if got != want {
			t.Errorf("got:  %q\nwant: %q", got, want)
		}
	})

	t.Run("field name in bracket values not normalized", func(t *testing.T) {
		got := tr.ConvertFilter("status IN ['Priority', 'Type']")
		want := `select where status in ["Priority", "Type"]`
		if got != want {
			t.Errorf("got:  %q\nwant: %q", got, want)
		}
	})
}

func TestConvertLegacyFullConfig(t *testing.T) {
	tr := NewLegacyConfigTransformer()

	cfg := &pluginFileConfig{
		Name: "board",
		Sort: "Priority, CreatedAt",
		Lanes: []PluginLaneConfig{
			{Name: "Backlog", Filter: "status = 'backlog'", Action: "status = 'backlog'"},
			{Name: "Ready", Filter: "status = 'ready'", Action: "status = 'ready'"},
			{Name: "In Progress", Filter: "status = 'inProgress'", Action: "status = 'inProgress'"},
			{Name: "Done", Filter: "status = 'done'", Action: "status = 'done'"},
		},
		Actions: []PluginActionConfig{
			{Key: "b", Label: "Backlog", Action: "status = 'backlog'"},
		},
	}

	count := tr.ConvertPluginConfig(cfg)

	// 4 filters + 4 actions + 1 plugin action + 1 sort = 10
	if count != 10 {
		t.Errorf("expected 10 conversions, got %d", count)
	}

	// verify sort was cleared
	if cfg.Sort != "" {
		t.Error("Sort field was not cleared")
	}

	// verify all converted expressions parse through ruki
	schema := testSchema()
	parser := ruki.NewParser(schema)

	for _, lane := range cfg.Lanes {
		if lane.Filter != "" {
			_, err := parser.ParseAndValidateStatement(lane.Filter, ruki.ExecutorRuntimePlugin)
			if err != nil {
				t.Errorf("lane %q filter failed ruki parse: %v\n  filter: %s", lane.Name, err, lane.Filter)
			}
		}
		if lane.Action != "" {
			_, err := parser.ParseAndValidateStatement(lane.Action, ruki.ExecutorRuntimePlugin)
			if err != nil {
				t.Errorf("lane %q action failed ruki parse: %v\n  action: %s", lane.Name, err, lane.Action)
			}
		}
	}

	for _, action := range cfg.Actions {
		if action.Action != "" {
			_, err := parser.ParseAndValidateStatement(action.Action, ruki.ExecutorRuntimePlugin)
			if err != nil {
				t.Errorf("plugin action %q failed ruki parse: %v\n  action: %s", action.Key, err, action.Action)
			}
		}
	}
}

// TestLegacyWorkflowEndToEnd tests the full pipeline from legacy YAML string
// through conversion and parsing to plugin creation and execution.
func TestLegacyWorkflowEndToEnd(t *testing.T) {
	legacyYAML := `views:
  - name: Board
    default: true
    key: "F1"
    sort: Priority, CreatedAt
    lanes:
      - name: Backlog
        filter: status = 'backlog' AND tags NOT IN ['blocked']
        action: status = 'backlog'
      - name: Ready
        filter: status = 'ready' AND assignee = CURRENT_USER
        action: status = 'ready', tags+=[reviewed]
      - name: Done
        filter: status = 'done'
        action: status = 'done'
    actions:
      - key: b
        label: Bug
        action: type = 'bug', priority = 1
`

	// convert legacy views format (list → map) before unmarshaling
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(legacyYAML), &raw); err != nil {
		t.Fatalf("failed to unmarshal raw YAML: %v", err)
	}
	transformer := NewLegacyConfigTransformer()
	transformer.ConvertViewsFormat(raw)
	normalizedData, err := yaml.Marshal(raw)
	if err != nil {
		t.Fatalf("failed to re-marshal: %v", err)
	}

	var wf WorkflowFile
	if err := yaml.Unmarshal(normalizedData, &wf); err != nil {
		t.Fatalf("failed to unmarshal workflow YAML: %v", err)
	}

	// convert legacy expressions
	for i := range wf.Views.Plugins {
		transformer.ConvertPluginConfig(&wf.Views.Plugins[i])
	}

	// parse into plugin — this validates ruki parsing succeeds
	schema := testSchema()
	p, err := parsePluginConfig(wf.Views.Plugins[0], "test", schema)
	if err != nil {
		t.Fatalf("parsePluginConfig failed: %v", err)
	}

	tp, ok := p.(*TikiPlugin)
	if !ok {
		t.Fatalf("expected TikiPlugin, got %T", p)
		return
	}

	if tp.Name != "Board" {
		t.Errorf("expected name Board, got %s", tp.Name)
	}
	if !tp.Default {
		t.Error("expected default=true")
	}
	if len(tp.Lanes) != 3 {
		t.Fatalf("expected 3 lanes, got %d", len(tp.Lanes))
	}
	if len(tp.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(tp.Actions))
	}

	// verify all lanes have parsed filter and action statements
	for i, lane := range tp.Lanes {
		if lane.Filter == nil {
			t.Errorf("lane %d (%s): expected non-nil filter", i, lane.Name)
		}
		if !lane.Filter.IsSelect() {
			t.Errorf("lane %d (%s): expected select statement", i, lane.Name)
		}
		if lane.Action == nil {
			t.Errorf("lane %d (%s): expected non-nil action", i, lane.Name)
		}
		if !lane.Action.IsUpdate() {
			t.Errorf("lane %d (%s): expected update statement", i, lane.Name)
		}
	}

	// execute filters against test tasks to verify they actually work
	executor := newTestExecutor()

	allTasks := []*task.Task{
		{ID: "TIKI-000001", Status: task.StatusBacklog, Priority: 3, Tags: []string{}, Assignee: "testuser"},
		{ID: "TIKI-000002", Status: task.StatusBacklog, Priority: 1, Tags: []string{"blocked"}, Assignee: "testuser"},
		{ID: "TIKI-000003", Status: task.StatusReady, Priority: 2, Assignee: "testuser"},
		{ID: "TIKI-000004", Status: task.StatusReady, Priority: 1, Assignee: "other"},
		{ID: "TIKI-000005", Status: task.StatusDone, Priority: 5, Assignee: "testuser"},
	}

	// backlog lane: status='backlog' AND tags NOT IN ['blocked']
	result, err := executor.Execute(tp.Lanes[0].Filter, allTasks)
	if err != nil {
		t.Fatalf("backlog filter execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 || result.Select.Tasks[0].ID != "TIKI-000001" {
		t.Errorf("backlog lane: expected [TIKI-000001], got %v", taskIDs(result.Select.Tasks))
	}

	// ready lane: status='ready' AND assignee = CURRENT_USER
	result, err = executor.Execute(tp.Lanes[1].Filter, allTasks)
	if err != nil {
		t.Fatalf("ready filter execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 || result.Select.Tasks[0].ID != "TIKI-000003" {
		t.Errorf("ready lane: expected [TIKI-000003], got %v", taskIDs(result.Select.Tasks))
	}

	// done lane: status='done'
	result, err = executor.Execute(tp.Lanes[2].Filter, allTasks)
	if err != nil {
		t.Fatalf("done filter execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 || result.Select.Tasks[0].ID != "TIKI-000005" {
		t.Errorf("done lane: expected [TIKI-000005], got %v", taskIDs(result.Select.Tasks))
	}

	// verify sort was merged: backlog filter should have order by
	backlogFilter := wf.Views.Plugins[0].Lanes[0].Filter
	if !strings.Contains(backlogFilter, "order by") {
		t.Errorf("expected sort merged into backlog filter, got: %s", backlogFilter)
	}

	// execute ready lane action to verify tag append works
	actionResult, err := executor.Execute(tp.Lanes[1].Action, []*task.Task{
		{ID: "TIKI-000003", Status: task.StatusReady, Tags: []string{}},
	}, ruki.NewSingleSelectionInput("TIKI-000003"))
	if err != nil {
		t.Fatalf("ready action execute: %v", err)
	}
	updated := actionResult.Update.Updated
	if len(updated) != 1 {
		t.Fatalf("expected 1 updated task, got %d", len(updated))
	}
	if updated[0].Status != task.StatusReady {
		t.Errorf("expected status ready, got %v", updated[0].Status)
	}
	if !containsString(updated[0].Tags, "reviewed") {
		t.Errorf("expected 'reviewed' tag after action, got %v", updated[0].Tags)
	}
}

func taskIDs(tasks []*task.Task) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}

func containsString(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

func TestConvertLegacyConversionCount(t *testing.T) {
	tr := NewLegacyConfigTransformer()

	// config with no legacy expressions
	cfg := &pluginFileConfig{
		Name: "test",
		Lanes: []PluginLaneConfig{
			{Name: "all", Filter: `select where status = "ready"`},
		},
	}
	count := tr.ConvertPluginConfig(cfg)
	if count != 0 {
		t.Errorf("expected 0 conversions for already-ruki config, got %d", count)
	}
}

func TestSplitTopLevelCommas(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "simple",
			input: "a, b, c",
			want:  []string{"a", " b", " c"},
		},
		{
			name:  "with brackets",
			input: "tags+=[a, b], status=done",
			want:  []string{"tags+=[a, b]", " status=done"},
		},
		{
			name:  "with quotes",
			input: "status='a, b', type=bug",
			want:  []string{"status='a, b'", " type=bug"},
		},
		{
			name:    "unmatched bracket",
			input:   "tags+=[a, b",
			wantErr: true,
		},
		{
			name:    "extra close bracket",
			input:   "a]",
			wantErr: true,
		},
		{
			name:    "unclosed quote",
			input:   "status='open",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitTopLevelCommas(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("length mismatch: got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("segment %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestQuoteIfBareIdentifier(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"done", `"done"`},
		{"ready", `"ready"`},
		{"42", "42"},
		{"3.14", "3.14"},
		{"-7", "-7"},
		{"-3.5", "-3.5"},
		{"now()", "now()"},
		{"user()", "user()"},
		{`"already"`, `"already"`},
		{"", ""},
		{"TIKI-ABC123", `"TIKI-ABC123"`},
		// not a bare identifier (contains space) — returned unchanged
		{"hello world", "hello world"},
		// starts with digit — not bare identifier, not numeric
		{"0xDEAD", "0xDEAD"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := quoteIfBareIdentifier(tt.input)
			if got != tt.want {
				t.Errorf("quoteIfBareIdentifier(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitTopLevelCommas_UnclosedDoubleQuote(t *testing.T) {
	_, err := splitTopLevelCommas(`status="open, tags+=[a]`)
	if err == nil {
		t.Fatal("expected error for unclosed double quote")
	}
	if !strings.Contains(err.Error(), "unclosed quote") {
		t.Errorf("expected 'unclosed quote' error, got: %v", err)
	}
}

func TestConvertBracketValues_NonBracketEnclosed(t *testing.T) {
	// bare identifier without brackets should be quoted
	got := convertBracketValues("frontend")
	if got != `"frontend"` {
		t.Errorf("expected bare identifier to be quoted, got %q", got)
	}

	// single-quoted value without brackets
	got = convertBracketValues("'needs review'")
	if got != `"needs review"` {
		t.Errorf("expected single-quoted value to be converted, got %q", got)
	}
}

func TestConvertActionSegment_DoubleEquals(t *testing.T) {
	got, err := convertActionSegment("status=='done'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// == should be handled: first = is found, then value starts with = and is stripped
	if got != `status="done"` {
		t.Errorf("got %q, want %q", got, `status="done"`)
	}
}

func TestConvertAction_NoAssignmentOperator(t *testing.T) {
	tr := NewLegacyConfigTransformer()
	_, err := tr.ConvertAction("garbage_without_equals")
	if err == nil {
		t.Fatal("expected error for no assignment operator")
	}
	if !strings.Contains(err.Error(), "no assignment operator") {
		t.Errorf("expected 'no assignment operator' error, got: %v", err)
	}
}

func TestConvertPluginConfig_ActionConvertError(t *testing.T) {
	tr := NewLegacyConfigTransformer()
	cfg := &pluginFileConfig{
		Name: "test",
		Lanes: []PluginLaneConfig{
			{Name: "all", Filter: ""},
			// lane action with malformed brackets should be skipped with a warning
			{Name: "bad", Filter: "", Action: "tags+=[unclosed"},
		},
	}
	count := tr.ConvertPluginConfig(cfg)
	// the malformed action should be skipped (not counted), but lane filter is empty (not counted)
	if count != 0 {
		t.Errorf("expected 0 conversions for malformed action, got %d", count)
	}
	// the action should remain unchanged due to conversion failure
	if cfg.Lanes[1].Action != "tags+=[unclosed" {
		t.Errorf("malformed action should be passed through unchanged, got %q", cfg.Lanes[1].Action)
	}
}

func TestConvertPluginConfig_PluginActionConvertError(t *testing.T) {
	tr := NewLegacyConfigTransformer()
	cfg := &pluginFileConfig{
		Name: "test",
		Lanes: []PluginLaneConfig{
			{Name: "all", Filter: ""},
		},
		Actions: []PluginActionConfig{
			{Key: "b", Label: "Bad", Action: "tags+=[unclosed"},
		},
	}
	count := tr.ConvertPluginConfig(cfg)
	if count != 0 {
		t.Errorf("expected 0 conversions for malformed plugin action, got %d", count)
	}
	if cfg.Actions[0].Action != "tags+=[unclosed" {
		t.Errorf("malformed plugin action should be passed through unchanged, got %q", cfg.Actions[0].Action)
	}
}

func TestConvertViewsFormat_ListToMap(t *testing.T) {
	tr := NewLegacyConfigTransformer()

	raw := map[string]interface{}{
		"views": []interface{}{
			map[string]interface{}{"name": "Kanban"},
			map[string]interface{}{"name": "Backlog"},
		},
	}
	tr.ConvertViewsFormat(raw)

	views, ok := raw["views"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected views to be a map after conversion, got %T", raw["views"])
	}
	plugins, ok := views["plugins"].([]interface{})
	if !ok {
		t.Fatalf("expected views.plugins to be a list, got %T", views["plugins"])
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}
}

func TestConvertViewsFormat_AlreadyMap(t *testing.T) {
	tr := NewLegacyConfigTransformer()

	raw := map[string]interface{}{
		"views": map[string]interface{}{
			"plugins": []interface{}{
				map[string]interface{}{"name": "Kanban"},
			},
			"actions": []interface{}{},
		},
	}
	tr.ConvertViewsFormat(raw)

	// should be unchanged
	views, ok := raw["views"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected views to remain a map, got %T", raw["views"])
	}
	plugins, ok := views["plugins"].([]interface{})
	if !ok {
		t.Fatalf("expected views.plugins to be a list, got %T", views["plugins"])
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
}

func TestConvertViewsFormat_NoViewsKey(t *testing.T) {
	tr := NewLegacyConfigTransformer()

	raw := map[string]interface{}{
		"statuses": []interface{}{},
	}
	tr.ConvertViewsFormat(raw)

	if _, ok := raw["views"]; ok {
		t.Fatal("should not create views key when it doesn't exist")
	}
}

func TestLegacyConvert_BoolLiteralBare(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "flag", Type: workflow.TypeBool, Custom: true},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	tr := NewLegacyConfigTransformer()

	// true/false values are emitted bare for TypeBool fields
	got, err := tr.ConvertAction("flag=true")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `update where id = id() set flag=true`
	if got != want {
		t.Errorf("ConvertAction(flag=true)\n  got:  %q\n  want: %q", got, want)
	}

	got, err = tr.ConvertAction("flag=false")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want = `update where id = id() set flag=false`
	if got != want {
		t.Errorf("ConvertAction(flag=false)\n  got:  %q\n  want: %q", got, want)
	}
}

func TestLegacyConvert_BoolInList(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "flag", Type: workflow.TypeBool, Custom: true},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	tr := NewLegacyConfigTransformer()

	// bool values in lists are emitted bare for TypeBool fields
	got := tr.ConvertFilter("flag IN [true, false]")
	want := `select where flag in [true, false]`
	if got != want {
		t.Errorf("ConvertFilter(flag IN [true, false])\n  got:  %q\n  want: %q", got, want)
	}
}

func TestLegacyConvert_TypeAwareQuoting(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, Custom: true, AllowedValues: []string{"low", "high", "true"}},
		{Name: "notes", Type: workflow.TypeString, Custom: true},
		{Name: "active", Type: workflow.TypeBool, Custom: true},
		{Name: "score", Type: workflow.TypeInt, Custom: true},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	tr := NewLegacyConfigTransformer()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "enum field with bool-like value is quoted",
			input: "severity=true",
			want:  `update where id = id() set severity="true"`,
		},
		{
			name:  "string field with numeric value is quoted",
			input: "notes=42",
			want:  `update where id = id() set notes="42"`,
		},
		{
			name:  "bool field with true stays bare",
			input: "active=true",
			want:  `update where id = id() set active=true`,
		},
		{
			name:  "int field with number stays bare",
			input: "score=42",
			want:  `update where id = id() set score=42`,
		},
		{
			name:  "string field with bool-like value is quoted",
			input: "notes=false",
			want:  `update where id = id() set notes="false"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tr.ConvertAction(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ConvertAction(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLegacyConvert_TypeAwareListQuoting(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "labels", Type: workflow.TypeListString, Custom: true},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	tr := NewLegacyConfigTransformer()

	// list<string> field: all elements must be quoted, even bool-like and numeric
	got, err := tr.ConvertAction("labels+=[true, 42, hello]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `update where id = id() set labels=labels+["true", "42", "hello"]`
	if got != want {
		t.Errorf("ConvertAction(labels+=[true, 42, hello])\n  got:  %q\n  want: %q", got, want)
	}
}

func TestLegacyConvert_UnknownFieldDefaultsToQuoting(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	tr := NewLegacyConfigTransformer()

	// unknown field: "true" should be quoted as safe fallback
	got, err := tr.ConvertAction("unknown_field=true")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `update where id = id() set unknown_field="true"`
	if got != want {
		t.Errorf("ConvertAction(unknown_field=true)\n  got:  %q\n  want: %q", got, want)
	}
}
