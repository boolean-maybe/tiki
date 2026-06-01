package plugin

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"gopkg.in/yaml.v3"

	"github.com/boolean-maybe/ruki"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
)

func testSchema() ruki.Schema {
	return rukiRuntime.NewSchema()
}

func testParser() *ruki.Parser {
	return ruki.NewParser(testSchema())
}

// minimalBoardLayout returns the smallest valid `layout:` value for a
// board/list view. Reused by tests that don't otherwise care about the
// layout shape — they just need the parser to accept the config.
func minimalBoardLayout() string {
	return "id"
}

func TestWikiValidation(t *testing.T) {
	schema := testSchema()
	cases := []struct {
		name      string
		cfg       pluginFileConfig
		wantError string
	}{
		{"missing path", pluginFileConfig{Name: "Docs", Kind: "wiki"}, "requires `path:`"},
		{"document not implemented", pluginFileConfig{Name: "Docs", Kind: "wiki", Document: "ABC123"}, "ID-based resolution) is not yet implemented"},
		{"lanes rejected", pluginFileConfig{Name: "Docs", Kind: "wiki", Path: "x.md", Lanes: []PluginLaneConfig{{Name: "x", Filter: "select"}}}, "`lanes:` only valid on board"},
		{"per-view actions rejected", pluginFileConfig{Name: "Docs", Kind: "wiki", Path: "x.md", Actions: []PluginActionConfig{{Key: "x", Label: "x", Action: `update where id = id() set status="ready"`}}}, "cannot have per-view `actions:`"},
		{"valid with path", pluginFileConfig{Name: "Docs", Kind: "wiki", Path: "x.md"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePluginConfig(tc.cfg, "test", schema, nil)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantError)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("expected error containing %q, got %q", tc.wantError, err.Error())
			}
		})
	}
}

// TestLegacyFieldRejection asserts each pre-Phase-6 field produces a clear
// error that points at the new syntax.
func TestLegacyFieldRejection(t *testing.T) {
	schema := testSchema()
	cases := []struct {
		name      string
		cfg       pluginFileConfig
		wantError string
	}{
		{"type: tiki", pluginFileConfig{Name: "X", Type: "tiki"}, "use `kind: board`"},
		{"type: doki", pluginFileConfig{Name: "X", Type: "doki"}, "use `kind: wiki`"},
		{"view field", pluginFileConfig{Name: "X", View: "compact"}, "`view:` as a display mode"},
		{"fetcher field", pluginFileConfig{Name: "X", Fetcher: "file"}, "use `document:` or `path:`"},
		{"text field", pluginFileConfig{Name: "X", Text: "t"}, "use `document:` or `path:`"},
		{"url field", pluginFileConfig{Name: "X", URL: "u"}, "use `document:` or `path:`"},
		{"sort field", pluginFileConfig{Name: "X", Sort: "priority"}, "use `order by`"},
		{"mode field", pluginFileConfig{Name: "X", Mode: "expanded"}, "use `layout:`"},
		{"metadata field", pluginFileConfig{Name: "X", Metadata: [][]string{{"title"}}}, "renamed to `layout:`"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePluginConfig(tc.cfg, "test", schema, nil)
			if err == nil {
				t.Fatalf("expected rejection, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("expected error containing %q, got %q", tc.wantError, err.Error())
			}
		})
	}
}

func TestUnknownAndReservedKinds(t *testing.T) {
	schema := testSchema()
	cases := []struct {
		name      string
		cfg       pluginFileConfig
		wantError string
	}{
		{"missing kind", pluginFileConfig{Name: "X"}, "missing `kind:`"},
		{"unknown kind", pluginFileConfig{Name: "X", Kind: "galaxy"}, "unknown view kind"},
		{"timeline reserved", pluginFileConfig{Name: "X", Kind: "timeline"}, "reserved but not yet implemented"},
		{"search reserved", pluginFileConfig{Name: "X", Kind: "search"}, "kind: search` is reserved but not yet implemented"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePluginConfig(tc.cfg, "test", schema, nil)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("expected error containing %q, got %q", tc.wantError, err.Error())
			}
		})
	}
}

func TestParsePluginConfig_InvalidKey(t *testing.T) {
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "InvalidKey",
		Kind: "board",
	}

	_, err := parsePluginConfig(cfg, "test.yaml", testSchema(), nil)
	if err == nil {
		t.Fatal("Expected error for invalid key format")
	}

	if !strings.Contains(err.Error(), "parsing key") {
		t.Errorf("Expected 'parsing key' error, got: %v", err)
	}
}

func TestParsePluginConfig_ActivationKeyNormalization(t *testing.T) {
	schema := testSchema()

	tests := []struct {
		name    string
		keyStr  string
		wantKey tcell.Key
		wantR   rune
		wantMod tcell.ModMask
	}{
		{"plain rune", "T", tcell.KeyRune, 'T', 0},
		{"Ctrl-U", "Ctrl-U", tcell.KeyCtrlU, 0, tcell.ModCtrl},
		{"ctrl-u lowercase", "ctrl-u", tcell.KeyCtrlU, 0, tcell.ModCtrl},
		{"Alt-M", "Alt-M", tcell.KeyRune, 'M', tcell.ModAlt},
		{"F5", "F5", tcell.KeyF5, 0, 0},
		{"Shift-x normalizes to X", "Shift-x", tcell.KeyRune, 'X', 0},
		{"Shift-X normalizes to X", "Shift-X", tcell.KeyRune, 'X', 0},
		{"Shift-F3", "Shift-F3", tcell.KeyF3, 0, tcell.ModShift},
		{"empty key is valid", "", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := pluginFileConfig{
				Name:   "Test",
				Key:    tt.keyStr,
				Kind:   "board",
				Layout: minimalBoardLayout(),
				Lanes: []PluginLaneConfig{
					{Name: "Todo", Filter: `select where status = "ready"`},
				},
			}
			p, err := parsePluginConfig(cfg, "test.yaml", schema, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotKey, gotR, gotMod := p.GetActivationKey()
			if gotKey != tt.wantKey || gotR != tt.wantR || gotMod != tt.wantMod {
				t.Errorf("activation key for %q: got (%v, %q, %v), want (%v, %q, %v)",
					tt.keyStr, gotKey, gotR, gotMod, tt.wantKey, tt.wantR, tt.wantMod)
			}
		})
	}
}

// TestParsePluginConfig_BoardKindExplicit asserts kind: board builds a WorkflowPlugin.
func TestParsePluginConfig_BoardKindExplicit(t *testing.T) {
	cfg := pluginFileConfig{
		Name:   "Test",
		Key:    "T",
		Kind:   "board",
		Layout: "id\n<highlight>title",
		Lanes: []PluginLaneConfig{
			{Name: "Todo", Filter: `select where status = "ready"`},
		},
	}
	p, err := parsePluginConfig(cfg, "test.yaml", testSchema(), nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	wp, ok := p.(*WorkflowPlugin)
	if !ok {
		t.Fatalf("Expected WorkflowPlugin for kind: board, got %T", p)
	}
	if wp.Layout.Rows != 2 {
		t.Errorf("Layout.Rows = %d, want 2", wp.Layout.Rows)
	}
}

// TestParsePluginConfig_BoardMissingLayout asserts that a board view without
// a layout: field is rejected.
func TestParsePluginConfig_BoardMissingLayout(t *testing.T) {
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Kind: "board",
		Lanes: []PluginLaneConfig{
			{Name: "Todo", Filter: `select where status = "ready"`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", testSchema(), nil)
	if err == nil || !strings.Contains(err.Error(), "layout") {
		t.Fatalf("expected missing-layout error, got %v", err)
	}
}

// TestParsePluginConfig_BoardEmptyLayout asserts that an empty layout block
// is rejected.
func TestParsePluginConfig_BoardEmptyLayout(t *testing.T) {
	cfg := pluginFileConfig{
		Name:   "Test",
		Key:    "T",
		Kind:   "board",
		Layout: "",
		Lanes: []PluginLaneConfig{
			{Name: "Todo", Filter: `select where status = "ready"`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", testSchema(), nil)
	if err == nil || !strings.Contains(err.Error(), "layout") {
		t.Fatalf("expected empty-layout error, got %v", err)
	}
}

func TestParsePluginYAML_InvalidYAML(t *testing.T) {
	invalidYAML := []byte("invalid: yaml: content:")

	_, err := parsePluginYAML(invalidYAML, "test.yaml", testSchema())
	if err == nil {
		t.Fatal("Expected error for invalid YAML")
	}

	if !strings.Contains(err.Error(), "parsing yaml") {
		t.Errorf("Expected 'parsing yaml' error, got: %v", err)
	}
}

func TestParsePluginYAML_ValidTiki(t *testing.T) {
	validYAML := []byte(`
name: Test Plugin
key: T
kind: board
lanes:
  - name: Todo
    columns: 4
    filter: select where status = "ready"
layout: |
  id
  <highlight>title
foreground: "#ff0000"
background: "#0000ff"
`)

	plugin, err := parsePluginYAML(validYAML, "test.yaml", testSchema())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	tikiPlugin, ok := plugin.(*WorkflowPlugin)
	if !ok {
		t.Fatalf("Expected WorkflowPlugin, got %T", plugin)
		return
	}

	if tikiPlugin.GetName() != "Test Plugin" {
		t.Errorf("Expected name 'Test Plugin', got %q", tikiPlugin.GetName())
	}

	if tikiPlugin.Layout.Rows != 2 {
		t.Errorf("Layout.Rows = %d, want 2", tikiPlugin.Layout.Rows)
	}

	if len(tikiPlugin.Lanes) != 1 {
		t.Fatalf("Expected 1 lane, got %d", len(tikiPlugin.Lanes))
	}

	if tikiPlugin.Lanes[0].Columns != 4 {
		t.Errorf("Expected lane columns 4, got %d", tikiPlugin.Lanes[0].Columns)
	}
}

func TestParsePluginActions_Valid(t *testing.T) {
	parser := testParser()

	configs := []PluginActionConfig{
		{Key: "b", Label: "Add to board", Action: `update where id = id() set status="ready"`},
		{Key: "a", Label: "Assign to me", Action: `update where id = id() set assignee=user()`},
	}

	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	if actions[0].Rune != 'b' {
		t.Errorf("expected rune 'b', got %q", actions[0].Rune)
	}
	if actions[0].KeyStr != "b" {
		t.Errorf("expected KeyStr 'b', got %q", actions[0].KeyStr)
	}
	if actions[0].Label != "Add to board" {
		t.Errorf("expected label 'Add to board', got %q", actions[0].Label)
	}
	if actions[0].Action == nil {
		t.Fatal("expected non-nil action")
	}
	if !actions[0].Action.IsUpdate() {
		t.Error("expected action to be an UPDATE statement")
	}

	if actions[1].Rune != 'a' {
		t.Errorf("expected rune 'a', got %q", actions[1].Rune)
	}
	if actions[1].Action == nil {
		t.Fatal("expected non-nil action for 'assign to me'")
	}
	if !actions[1].Action.IsUpdate() {
		t.Error("expected 'assign to me' action to be an UPDATE statement")
	}
}

func TestParsePluginActions_Empty(t *testing.T) {
	parser := testParser()

	actions, err := parsePluginActions(nil, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actions != nil {
		t.Errorf("expected nil, got %v", actions)
	}
}

func TestParsePluginActions_Errors(t *testing.T) {
	parser := testParser()

	tests := []struct {
		name    string
		configs []PluginActionConfig
		wantErr string
	}{
		{
			name:    "missing key",
			configs: []PluginActionConfig{{Key: "", Label: "Test", Action: `update where id = id() set status="ready"`}},
			wantErr: "missing 'key'",
		},
		{
			name:    "multi-character key",
			configs: []PluginActionConfig{{Key: "ab", Label: "Test", Action: `update where id = id() set status="ready"`}},
			wantErr: "invalid key",
		},
		{
			name:    "missing label",
			configs: []PluginActionConfig{{Key: "b", Label: "", Action: `update where id = id() set status="ready"`}},
			wantErr: "missing 'label'",
		},
		{
			name:    "missing action and view",
			configs: []PluginActionConfig{{Key: "b", Label: "Test", Action: ""}},
			wantErr: "must set either `action:` (ruki) or `view:`",
		},
		{
			name:    "invalid action expression",
			configs: []PluginActionConfig{{Key: "b", Label: "Test", Action: `update where id = id() set owner="me"`}},
			wantErr: "parsing action",
		},
		{
			name: "duplicate key",
			configs: []PluginActionConfig{
				{Key: "b", Label: "First", Action: `update where id = id() set status="ready"`},
				{Key: "b", Label: "Second", Action: `update where id = id() set status="done"`},
			},
			wantErr: "duplicate action key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePluginActions(tc.configs, parser, nil, false)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestParsePluginYAML_TikiWithActions(t *testing.T) {
	yamlData := []byte(`
name: Test
key: T
kind: board
layout: |
  id
lanes:
  - name: Backlog
    filter: select where status = "inbox"
actions:
  - key: "b"
    label: "Add to board"
    action: update where id = id() set status = "ready"
`)

	p, err := parsePluginYAML(yamlData, "test.yaml", testSchema())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tiki, ok := p.(*WorkflowPlugin)
	if !ok {
		t.Fatalf("expected WorkflowPlugin, got %T", p)
	}

	if len(tiki.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(tiki.Actions))
	}

	if tiki.Actions[0].Rune != 'b' {
		t.Errorf("expected rune 'b', got %q", tiki.Actions[0].Rune)
	}
	if tiki.Actions[0].Label != "Add to board" {
		t.Errorf("expected label 'Add to board', got %q", tiki.Actions[0].Label)
	}
}

func TestParsePluginConfig_LaneFilterMustBeSelect(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Kind: "board",
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Filter: `update where id = id() set status = "ready"`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema, nil)
	if err == nil {
		t.Fatal("expected error for non-SELECT filter")
	}
	if !strings.Contains(err.Error(), "filter must be a SELECT") {
		t.Errorf("expected 'filter must be a SELECT' error, got: %v", err)
	}
}

// Lane filters run on every render without a selection payload. target. would
// fail every render against the exactly-one contract; targets. would silently
// project zero. Reject both at parse time with a clear message. (Lane actions
// receive the moved tiki as a single selection, so they remain valid.)
func TestParsePluginConfig_LaneFilterRejectsTargetQualifier(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Kind: "board",
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Filter: `select where id = target.id`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema, nil)
	if err == nil {
		t.Fatal("expected rejection of target. in lane filter")
	}
	if !strings.Contains(err.Error(), "target.") {
		t.Fatalf("expected error to mention target., got: %v", err)
	}
}

func TestParsePluginConfig_LaneFilterRejectsTargetsQualifier(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Kind: "board",
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Filter: `select where id in targets.id`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema, nil)
	if err == nil {
		t.Fatal("expected rejection of targets. in lane filter")
	}
	if !strings.Contains(err.Error(), "targets.") {
		t.Fatalf("expected error to mention targets., got: %v", err)
	}
}

// Lane actions fire on a specific moved tiki and receive it as a single
// selection. target. and targets. must remain usable there.
func TestParsePluginConfig_LaneActionAllowsTargetQualifier(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:   "Test",
		Key:    "T",
		Kind:   "board",
		Layout: minimalBoardLayout(),
		Lanes: []PluginLaneConfig{
			{
				Name:   "OK",
				Filter: `select where status = "ready"`,
				Action: `update where id = target.id set status = "ready"`,
			},
		},
	}
	if _, err := parsePluginConfig(cfg, "test.yaml", schema, nil); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestParsePluginConfig_LaneActionMustBeUpdate(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Kind: "board",
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Filter: `select where status = "ready"`, Action: `select where status = "done"`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema, nil)
	if err == nil {
		t.Fatal("expected error for non-UPDATE action")
	}
	if !strings.Contains(err.Error(), "action must be an UPDATE") {
		t.Errorf("expected 'action must be an UPDATE' error, got: %v", err)
	}
}

func TestParsePluginActions_SelectAllowedAsAction(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "s", Label: "Search", Action: `select where status = "ready"`},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if !actions[0].Action.IsSelect() {
		t.Error("expected action to be a SELECT statement")
	}
}

func TestParsePluginActions_PipeAcceptedAsAction(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "c", Label: "Copy ID", Action: `select id where id = id() | run("echo $1")`},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("expected pipe action to be accepted, got error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if !actions[0].Action.IsPipe() {
		t.Error("expected IsPipe() = true for pipe action")
	}
}

func TestParsePluginActions_RejectsExpressionStatement(t *testing.T) {
	parser := testParser()

	tests := []struct {
		name   string
		action string
	}{
		{"count top-level", `count(select)`},
		{"count with where", `count(select where status = "done")`},
		{"exists top-level", `exists(select where priority = "high")`},
		{"now top-level", `now()`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs := []PluginActionConfig{{Key: "x", Label: "Scalar", Action: tt.action}}
			_, err := parsePluginActions(configs, parser, nil, false)
			if err == nil {
				t.Fatal("expected error for expression statement as plugin action")
			}
			// message should steer the reader toward the supported action shapes
			if !strings.Contains(err.Error(), "expression statement") {
				t.Errorf("expected 'expression statement' in error, got: %v", err)
			}
		})
	}
}

func TestParsePluginConfig_LaneFilterParseError(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Kind: "board",
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Filter: "totally invalid @@@"},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema, nil)
	if err == nil {
		t.Fatal("expected error for invalid filter expression")
	}
	if !strings.Contains(err.Error(), "parsing filter") {
		t.Errorf("expected 'parsing filter' error, got: %v", err)
	}
}

func TestParsePluginConfig_LaneActionParseError(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Kind: "board",
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Filter: `select where status = "ready"`, Action: "totally invalid @@@"},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema, nil)
	if err == nil {
		t.Fatal("expected error for invalid action expression")
	}
	if !strings.Contains(err.Error(), "parsing action") {
		t.Errorf("expected 'parsing action' error, got: %v", err)
	}
}

func TestParsePluginConfig_LaneMissingName(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Kind: "board",
		Lanes: []PluginLaneConfig{
			{Name: "", Filter: `select where status = "ready"`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema, nil)
	if err == nil {
		t.Fatal("expected error for lane missing name")
	}
	if !strings.Contains(err.Error(), "missing name") {
		t.Errorf("expected 'missing name' error, got: %v", err)
	}
}

func TestParsePluginConfig_LaneInvalidColumns(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Kind: "board",
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Columns: -1, Filter: `select where status = "ready"`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema, nil)
	if err == nil {
		t.Fatal("expected error for invalid columns")
	}
	if !strings.Contains(err.Error(), "invalid columns") {
		t.Errorf("expected 'invalid columns' error, got: %v", err)
	}
}

func TestParsePluginConfig_LaneInvalidWidth(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Kind: "board",
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Width: 101, Filter: `select where status = "ready"`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema, nil)
	if err == nil {
		t.Fatal("expected error for invalid width")
	}
	if !strings.Contains(err.Error(), "invalid width") {
		t.Errorf("expected 'invalid width' error, got: %v", err)
	}
}

func TestParsePluginConfig_TooManyLanes(t *testing.T) {
	schema := testSchema()
	lanes := make([]PluginLaneConfig, 11)
	for i := range lanes {
		lanes[i] = PluginLaneConfig{Name: "Lane", Filter: `select where status = "ready"`}
	}
	cfg := pluginFileConfig{
		Name:  "Test",
		Key:   "T",
		Kind:  "board",
		Lanes: lanes,
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema, nil)
	if err == nil {
		t.Fatal("expected error for too many lanes")
	}
	if !strings.Contains(err.Error(), "too many lanes") {
		t.Errorf("expected 'too many lanes' error, got: %v", err)
	}
}

func TestParsePluginConfig_NoLanes(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:  "Test",
		Key:   "T",
		Kind:  "board",
		Lanes: []PluginLaneConfig{},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema, nil)
	if err == nil {
		t.Fatal("expected error for no lanes")
	}
	if !strings.Contains(err.Error(), "requires 'lanes'") {
		t.Errorf("expected 'requires lanes' error, got: %v", err)
	}
}

func TestParsePluginConfig_PluginActionsError(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Kind: "board",
		Lanes: []PluginLaneConfig{
			{Name: "Todo", Filter: `select where status = "ready"`},
		},
		Actions: []PluginActionConfig{
			{Key: "b", Label: "Bad", Action: "totally invalid @@@"},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema, nil)
	if err == nil {
		t.Fatal("expected error for invalid plugin action")
	}
}

func TestParsePluginActions_NonPrintableKey(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "\x01", Label: "Test", Action: `update where id = id() set status="ready"`},
	}
	_, err := parsePluginActions(configs, parser, nil, false)
	if err == nil {
		t.Fatal("expected error for non-printable key")
	}
	if !strings.Contains(err.Error(), "printable character") {
		t.Errorf("expected 'printable character' error, got: %v", err)
	}
}

func TestParsePluginActions_CompositeKeys(t *testing.T) {
	parser := testParser()

	t.Run("Ctrl-U as action key", func(t *testing.T) {
		configs := []PluginActionConfig{
			{Key: "Ctrl-U", Label: "Undo", Action: `update where id = id() set status="ready"`},
		}
		actions, err := parsePluginActions(configs, parser, nil, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].KeyStr != "Ctrl-U" {
			t.Errorf("expected KeyStr 'Ctrl-U', got %q", actions[0].KeyStr)
		}
		if actions[0].Modifier != tcell.ModCtrl {
			t.Errorf("expected ModCtrl, got %v", actions[0].Modifier)
		}
	})

	t.Run("Alt-M as action key", func(t *testing.T) {
		configs := []PluginActionConfig{
			{Key: "Alt-M", Label: "Mark", Action: `update where id = id() set status="ready"`},
		}
		actions, err := parsePluginActions(configs, parser, nil, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if actions[0].KeyStr != "Alt-M" {
			t.Errorf("expected KeyStr 'Alt-M', got %q", actions[0].KeyStr)
		}
	})

	t.Run("F5 as action key", func(t *testing.T) {
		configs := []PluginActionConfig{
			{Key: "F5", Label: "Reload", Action: `update where id = id() set status="ready"`},
		}
		actions, err := parsePluginActions(configs, parser, nil, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if actions[0].KeyStr != "F5" {
			t.Errorf("expected KeyStr 'F5', got %q", actions[0].KeyStr)
		}
	})

	t.Run("Shift-X normalizes to X", func(t *testing.T) {
		configs := []PluginActionConfig{
			{Key: "Shift-X", Label: "eXtra", Action: `update where id = id() set status="ready"`},
		}
		actions, err := parsePluginActions(configs, parser, nil, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if actions[0].KeyStr != "X" {
			t.Errorf("expected KeyStr 'X', got %q", actions[0].KeyStr)
		}
		if actions[0].Modifier != 0 {
			t.Errorf("expected no modifier after normalization, got %v", actions[0].Modifier)
		}
	})

	t.Run("duplicate Shift-x vs X", func(t *testing.T) {
		configs := []PluginActionConfig{
			{Key: "Shift-x", Label: "First", Action: `update where id = id() set status="ready"`},
			{Key: "X", Label: "Second", Action: `update where id = id() set status="done"`},
		}
		_, err := parsePluginActions(configs, parser, nil, false)
		if err == nil {
			t.Fatal("expected duplicate error for Shift-x vs X")
		}
		if !strings.Contains(err.Error(), "duplicate") {
			t.Errorf("expected 'duplicate' error, got: %v", err)
		}
	})

	t.Run("x and X are distinct", func(t *testing.T) {
		configs := []PluginActionConfig{
			{Key: "x", Label: "lowercase", Action: `update where id = id() set status="ready"`},
			{Key: "X", Label: "uppercase", Action: `update where id = id() set status="done"`},
		}
		actions, err := parsePluginActions(configs, parser, nil, false)
		if err != nil {
			t.Fatalf("expected no error for x vs X, got: %v", err)
		}
		if len(actions) != 2 {
			t.Fatalf("expected 2 actions, got %d", len(actions))
		}
	})

	t.Run("differently cased Ctrl spellings are duplicates", func(t *testing.T) {
		configs := []PluginActionConfig{
			{Key: "Ctrl-U", Label: "First", Action: `update where id = id() set status="ready"`},
			{Key: "ctrl-u", Label: "Second", Action: `update where id = id() set status="done"`},
		}
		_, err := parsePluginActions(configs, parser, nil, false)
		if err == nil {
			t.Fatal("expected duplicate error for differently-cased Ctrl spellings")
		}
		if !strings.Contains(err.Error(), "duplicate") {
			t.Errorf("expected 'duplicate' error, got: %v", err)
		}
	})
}

func TestParsePluginYAML_ValidWiki(t *testing.T) {
	validYAML := []byte(`
name: Doc Plugin
key: D
kind: wiki
path: index.md
foreground: "#00ff00"
`)

	p, err := parsePluginYAML(validYAML, "test.yaml", testSchema())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	wiki, ok := p.(*WikiPlugin)
	if !ok {
		t.Fatalf("Expected WikiPlugin, got %T", p)
	}

	if wiki.GetName() != "Doc Plugin" {
		t.Errorf("Expected name 'Doc Plugin', got %q", wiki.GetName())
	}
	if wiki.GetKind() != KindWiki {
		t.Errorf("Expected kind wiki, got %q", wiki.GetKind())
	}
	if wiki.DocumentPath != "index.md" {
		t.Errorf("Expected path 'index.md', got %q", wiki.DocumentPath)
	}
}

func TestParsePluginActions_HotDefault(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "b", Label: "Board", Action: `update where id = id() set status="ready"`},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !actions[0].ShowInHeader {
		t.Error("absent hot should default to ShowInHeader=true")
	}
}

func TestParsePluginActions_HotExplicitFalse(t *testing.T) {
	parser := testParser()
	hotFalse := false
	configs := []PluginActionConfig{
		{Key: "b", Label: "Board", Action: `update where id = id() set status="ready"`, Hot: &hotFalse},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actions[0].ShowInHeader {
		t.Error("hot: false should set ShowInHeader=false")
	}
}

func TestParsePluginActions_HotExplicitTrue(t *testing.T) {
	parser := testParser()
	hotTrue := true
	configs := []PluginActionConfig{
		{Key: "b", Label: "Board", Action: `update where id = id() set status="ready"`, Hot: &hotTrue},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !actions[0].ShowInHeader {
		t.Error("hot: true should set ShowInHeader=true")
	}
}

func TestParsePluginYAML_HotFlagFromYAML(t *testing.T) {
	yamlData := []byte(`
name: Test
key: T
kind: board
layout: |
  id
lanes:
  - name: Backlog
    filter: select where status = "inbox"
actions:
  - key: "b"
    label: "Board"
    action: update where id = id() set status = "ready"
    hot: false
  - key: "a"
    label: "Assign"
    action: update where id = id() set assignee = user()
`)

	p, err := parsePluginYAML(yamlData, "test.yaml", testSchema())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tiki, ok := p.(*WorkflowPlugin)
	if !ok {
		t.Fatalf("expected WorkflowPlugin, got %T", p)
	}

	if tiki.Actions[0].ShowInHeader {
		t.Error("action with hot: false should have ShowInHeader=false")
	}
	if !tiki.Actions[1].ShowInHeader {
		t.Error("action without hot should default to ShowInHeader=true")
	}
}

func TestParsePluginActions_InputValid(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Assign to", Action: `update where id = id() set assignee=input()`, Input: "string"},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if !actions[0].HasInput {
		t.Error("expected HasInput=true")
	}
	if actions[0].InputType != ruki.ValueString {
		t.Errorf("expected InputType=ValueString, got %d", actions[0].InputType)
	}
}

func TestParsePluginActions_InputIntValid(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "p", Label: "Set escalations", Action: `update where id = id() set escalations=input()`, Input: "int"},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !actions[0].HasInput {
		t.Error("expected HasInput=true")
	}
	if actions[0].InputType != ruki.ValueInt {
		t.Errorf("expected InputType=ValueInt, got %d", actions[0].InputType)
	}
}

func TestParsePluginActions_InputTypeMismatch(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Assign to", Action: `update where id = id() set assignee=input()`, Input: "int"},
	}
	_, err := parsePluginActions(configs, parser, nil, false)
	if err == nil {
		t.Fatal("expected error for input type mismatch (int into string field)")
	}
}

func TestParsePluginActions_InputWithoutInputFunc(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Ready", Action: `update where id = id() set status="ready"`, Input: "string"},
	}
	_, err := parsePluginActions(configs, parser, nil, false)
	if err == nil {
		t.Fatal("expected error: input: declared but input() not used")
	}
	if !strings.Contains(err.Error(), "does not use input()") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestParsePluginActions_InputUnsupportedType(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Assign to", Action: `update where id = id() set assignee=input()`, Input: "enum"},
	}
	_, err := parsePluginActions(configs, parser, nil, false)
	if err == nil {
		t.Fatal("expected error for unsupported input type")
	}
}

func TestParsePluginActions_NoInputField_NoHasInput(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Ready", Action: `update where id = id() set status="ready"`},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actions[0].HasInput {
		t.Error("expected HasInput=false for action without input: field")
	}
}

func TestParsePluginActions_RequirePreserved(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Ready", Action: `update where id = id() set status="ready"`, Require: []string{"id"}},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions[0].Require) != 1 || actions[0].Require[0] != "id" {
		t.Errorf("expected require=[id], got %v", actions[0].Require)
	}
}

func TestParsePluginActions_RequireAutoInferID(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Ready", Action: `update where id = id() set status="ready"`},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, r := range actions[0].Require {
		if r == "id" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected auto-inferred 'id' requirement, got %v", actions[0].Require)
	}
}

func TestParsePluginActions_RequireNoAutoInferWithoutID(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Bulk", Action: `delete where status = "done"`},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions[0].Require) > 0 {
		t.Errorf("expected no requirements for action without id(), got %v", actions[0].Require)
	}
}

// target.<field> must auto-infer the same "id" requirement as id(), so plugin
// actions using it stay disabled until exactly one tiki is selected.
func TestParsePluginActions_RequireAutoInferIDFromTargetQualifier(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Copy assignee", Action: `update where type = "bug" set assignee = target.assignee`},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, r := range actions[0].Require {
		if r == "id" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected auto-inferred 'id' requirement from target., got %v", actions[0].Require)
	}
}

// targets.<field> must auto-infer "selection:any" like ids(), so the action
// stays disabled when nothing is selected.
func TestParsePluginActions_RequireAutoInferSelectionAnyFromTargetsQualifier(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "b", Label: "Show blockers", Action: `select where id in targets.dependsOn`},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, r := range actions[0].Require {
		if r == "selection:any" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected auto-inferred 'selection:any' from targets., got %v", actions[0].Require)
	}
}

func TestParsePluginActions_RequireAutoInferSelectionAnyFromIDs(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Done all", Action: `update where id in ids() set status = "done"`},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, r := range actions[0].Require {
		if r == "selection:any" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected auto-inferred 'selection:any' requirement from ids(), got %v", actions[0].Require)
	}
}

// selected_count() must NOT auto-infer selection:any: ruki statements like
// `where selected_count() = 0` need to remain reachable from the preflight.
func TestParsePluginActions_RequireNoAutoInferFromSelectedCount(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Cardinality branch", Action: `select where selected_count() >= 0`},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range actions[0].Require {
		if r == "selection:any" || r == "id" || r == "selection:one" || r == "selection:many" {
			t.Errorf("selected_count() should not auto-infer selection requirement, got %v", actions[0].Require)
		}
	}
}

// filepath() must auto-infer the same "id" requirement as id() — both are
// scalar selection builtins requiring exactly one selected tiki at execute
// time, so the action stays disabled until that holds.
func TestParsePluginActions_RequireAutoInferIDFromFilepath(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "e", Label: "Edit", Action: `update where filepath = filepath() set status = "done"`},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, r := range actions[0].Require {
		if r == "id" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected auto-inferred 'id' requirement from filepath(), got %v", actions[0].Require)
	}
}

// filepaths() must auto-infer "selection:any" like ids(), so the action stays
// disabled when nothing is selected.
func TestParsePluginActions_RequireAutoInferSelectionAnyFromFilepaths(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "b", Label: "Bulk", Action: `select where filepath in filepaths()`},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, r := range actions[0].Require {
		if r == "selection:any" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected auto-inferred 'selection:any' from filepaths(), got %v", actions[0].Require)
	}
}

// A negated selection requirement is still a cardinality constraint and must
// suppress auto-inference from ids(); otherwise an explicit "!selection:many"
// (meaning "0 or 1") would be silently augmented with "selection:any"
// (meaning "at least 1"), narrowing the author's intent to "exactly 1".
func TestParsePluginActions_RequireNegatedSelectionSuppressesAutoInfer(t *testing.T) {
	cases := []struct {
		name     string
		negated  string
		notSeen  string
		wantKept string
	}{
		{"negated selection:many", "!selection:many", "selection:any", "!selection:many"},
		{"negated id", "!id", "selection:any", "!id"},
		{"negated selection:one", "!selection:one", "selection:any", "!selection:one"},
		{"negated selection:any", "!selection:any", "selection:any", "!selection:any"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parser := testParser()
			configs := []PluginActionConfig{{
				Key:     "a",
				Label:   "Bulk",
				Action:  `update where id in ids() set status = "done"`,
				Require: []string{tc.negated},
			}}
			actions, err := parsePluginActions(configs, parser, nil, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotKept, gotInferred := false, false
			for _, r := range actions[0].Require {
				if r == tc.wantKept {
					gotKept = true
				}
				if r == tc.notSeen {
					gotInferred = true
				}
			}
			if !gotKept {
				t.Errorf("expected %q preserved, got %v", tc.wantKept, actions[0].Require)
			}
			if gotInferred {
				t.Errorf("expected auto-inference suppressed, but found %q in %v", tc.notSeen, actions[0].Require)
			}
		})
	}
}

func TestParsePluginActions_RequireAIPlusAutoID(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "AI Ready", Action: `update where id = id() set status="ready"`, Require: []string{"ai"}},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasAI, hasID := false, false
	for _, r := range actions[0].Require {
		if r == "ai" {
			hasAI = true
		}
		if r == "id" {
			hasID = true
		}
	}
	if !hasAI || !hasID {
		t.Errorf("expected [ai, id], got %v", actions[0].Require)
	}
}

func TestParsePluginActions_RequireCustomPreserved(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Custom", Action: `select where status = "done"`, Require: []string{"foo"}},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions[0].Require) != 1 || actions[0].Require[0] != "foo" {
		t.Errorf("expected [foo], got %v", actions[0].Require)
	}
}

func TestParsePluginActions_RequireNegatedPreserved(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Not KB", Action: `select where status = "done"`, Require: []string{"!view:plugin:Kanban"}},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions[0].Require) != 1 || actions[0].Require[0] != "!view:plugin:Kanban" {
		t.Errorf("expected [!view:plugin:Kanban], got %v", actions[0].Require)
	}
}

func TestParsePluginActions_RequireEmptyRejected(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Bad", Action: `select where status = "done"`, Require: []string{""}},
	}
	_, err := parsePluginActions(configs, parser, nil, false)
	if err == nil {
		t.Fatal("expected error for empty requirement")
	}
}

func TestParsePluginActions_RequireBareExclamationRejected(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Bad", Action: `select where status = "done"`, Require: []string{"!"}},
	}
	_, err := parsePluginActions(configs, parser, nil, false)
	if err == nil {
		t.Fatal("expected error for bare '!' requirement")
	}
}

func TestParsePluginActions_RequireDoubleNegationRejected(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Bad", Action: `select where status = "done"`, Require: []string{"!!view:plugin:Kanban"}},
	}
	_, err := parsePluginActions(configs, parser, nil, false)
	if err == nil {
		t.Fatal("expected error for double-negation requirement")
	}
}

func TestParsePluginActions_RequireDedup(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Dup", Action: `update where id = id() set status="ready"`, Require: []string{"id"}},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := 0
	for _, r := range actions[0].Require {
		if r == "id" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one 'id', got %d in %v", count, actions[0].Require)
	}
}

func TestParsePluginActions_BulkActionExplicitRequireID(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "a", Label: "Selective Bulk", Action: `delete where status = "done"`, Require: []string{"id"}},
	}
	actions, err := parsePluginActions(configs, parser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions[0].Require) != 1 || actions[0].Require[0] != "id" {
		t.Errorf("expected explicit [id] preserved, got %v", actions[0].Require)
	}
}

func TestParsePluginYAML_RequireQuotedNegation(t *testing.T) {
	yaml := `
name: Test
key: "1"
kind: board
layout: |
  id
lanes:
  - name: All
    filter: 'select'
actions:
  - key: a
    label: "Not here"
    action: 'select where status = "done"'
    require: ["!view:plugin:Kanban"]
`
	p, err := parsePluginYAML([]byte(yaml), "test.yaml", testSchema())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tp, ok := p.(*WorkflowPlugin)
	if !ok {
		t.Fatal("expected *WorkflowPlugin")
	}
	if len(tp.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(tp.Actions))
	}
	if len(tp.Actions[0].Require) != 1 || tp.Actions[0].Require[0] != "!view:plugin:Kanban" {
		t.Errorf("expected [!view:plugin:Kanban], got %v", tp.Actions[0].Require)
	}
}

// --- ActionKindView tests --------------------------------------------------
//
// Covers the `kind: view` code path end-to-end at the parser level: shape
// validation, inference when `kind:` is omitted, cross-view name resolution,
// and the "action: and view: both set" / "neither set" error branches.

// TestParsePluginActions_ViewKindExplicit parses a `kind: view` action and
// asserts the PluginAction carries the right target view.
func TestParsePluginActions_ViewKindExplicit(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "F11", Kind: "view", Label: "Open kanban", View: "Kanban"},
	}
	viewNames := map[string]ViewKind{"Kanban": KindBoard}

	actions, err := parsePluginActions(configs, parser, viewNames, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Kind != ActionKindView {
		t.Errorf("expected ActionKindView, got %q", actions[0].Kind)
	}
	if actions[0].TargetView != "Kanban" {
		t.Errorf("expected TargetView=Kanban, got %q", actions[0].TargetView)
	}
	if actions[0].Action != nil {
		t.Errorf("view-kind actions must not carry a ruki statement, got %v", actions[0].Action)
	}
}

// TestParsePluginActions_ViewKindInferred asserts the parser infers
// ActionKindView when `view:` is present but `kind:` is omitted.
func TestParsePluginActions_ViewKindInferred(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "F11", Label: "Open kanban", View: "Kanban"},
	}
	viewNames := map[string]ViewKind{"Kanban": KindBoard}

	actions, err := parsePluginActions(configs, parser, viewNames, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actions[0].Kind != ActionKindView {
		t.Errorf("expected inferred ActionKindView, got %q", actions[0].Kind)
	}
}

// TestParsePluginActions_ViewKindErrors covers the four error branches:
// missing `view:` target, `action:` set on a view-kind action, both set, and
// `view:` referencing an unknown view name.
func TestParsePluginActions_ViewKindErrors(t *testing.T) {
	parser := testParser()
	knownViews := map[string]ViewKind{"Kanban": KindBoard}

	cases := []struct {
		name      string
		cfg       PluginActionConfig
		viewNames map[string]ViewKind
		wantError string
	}{
		{
			name:      "missing view target",
			cfg:       PluginActionConfig{Key: "F11", Kind: "view", Label: "Open"},
			viewNames: knownViews,
			wantError: "kind: view requires `view:`",
		},
		{
			name:      "both action and view set",
			cfg:       PluginActionConfig{Key: "F11", Kind: "view", Label: "Open", Action: `select where status = "done"`, View: "Kanban"},
			viewNames: knownViews,
			wantError: "kind: view must not set `action:`",
		},
		{
			name:      "unknown view name",
			cfg:       PluginActionConfig{Key: "F11", Kind: "view", Label: "Open", View: "NotAView"},
			viewNames: knownViews,
			wantError: `references unknown view "NotAView"`,
		},
		{
			name:      "input field rejected on view kind",
			cfg:       PluginActionConfig{Key: "F11", Kind: "view", Label: "Open", View: "Kanban", Input: "string"},
			viewNames: knownViews,
			wantError: "does not support `input:`",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePluginActions([]PluginActionConfig{tc.cfg}, parser, tc.viewNames, false)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantError)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("expected error containing %q, got %q", tc.wantError, err.Error())
			}
		})
	}
}

// TestParsePluginActions_AmbiguousKindInference asserts that when neither
// `action:` nor `view:` is set, the parser refuses rather than silently
// guessing a kind.
func TestParsePluginActions_AmbiguousKindInference(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "x", Label: "Ambiguous"},
	}
	_, err := parsePluginActions(configs, parser, nil, false)
	if err == nil {
		t.Fatal("expected error when neither action: nor view: is set")
	}
	if !strings.Contains(err.Error(), "must set either `action:` (ruki) or `view:`") {
		t.Errorf("expected neither-set error, got %q", err.Error())
	}
}

// TestParsePluginActions_BothActionAndViewSet asserts the combination
// `action: + view:` without explicit kind is an error (the disambiguation
// message must point at `kind:`).
func TestParsePluginActions_BothActionAndViewSet(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "x", Label: "Both", Action: `select where status = "done"`, View: "Kanban"},
	}
	viewNames := map[string]ViewKind{"Kanban": KindBoard}
	_, err := parsePluginActions(configs, parser, viewNames, false)
	if err == nil {
		t.Fatal("expected error when both action: and view: are set without an explicit kind:")
	}
	if !strings.Contains(err.Error(), "use `kind:` to disambiguate") {
		t.Errorf("expected disambiguate error, got %q", err.Error())
	}
}

// TestParseViewAction_ModeRequiresDetailTarget asserts that `mode:` is rejected
// when targeting a non-detail view.
func TestParseViewAction_ModeRequiresDetailTarget(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:   "Detail",
		Kind:   "detail",
		Layout: "status",
		Actions: []PluginActionConfig{
			{Key: "e", Label: "Edit", Kind: "view", View: "Board", Mode: "edit"},
		},
	}
	// simulate the viewNames map that would be built during workflow parsing
	viewNames := map[string]ViewKind{"Board": KindBoard, "Detail": KindDetail}
	_, err := parsePluginConfig(cfg, "test", schema, viewNames)
	if err == nil {
		t.Fatal("expected parse error for mode: edit targeting Board (kind: board, not detail)")
	}
	if !strings.Contains(err.Error(), "mode: only valid when targeting a kind: detail view") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseViewAction_ModeAcceptedOnDetail(t *testing.T) {
	schema := testSchema()
	yamlSrc := `
version: "2"
views:
  - name: Detail
    key: d
    kind: detail
    layout: status
actions:
  - key: Enter
    label: Open
    kind: view
    view: Detail
  - key: e
    label: Edit
    kind: view
    view: Detail
    mode: edit
  - key: n
    label: New
    kind: view
    view: Detail
    mode: new
  - key: Ctrl-D
    label: Edit description
    kind: view
    view: Detail
    mode: edit-desc
  - key: Ctrl-T
    label: Edit tags
    kind: view
    view: Detail
    mode: edit-tags
`
	plugins, actions, errs := loadPluginsFromYAML(yamlSrc, schema)
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if len(plugins) != 1 {
		t.Fatalf("got %d plugins, want 1", len(plugins))
	}
	if len(actions) != 5 {
		t.Fatalf("got %d actions, want 5", len(actions))
	}
	wantModes := []DetailMode{
		DetailModeView, DetailModeEdit, DetailModeNew,
		DetailModeEditDesc, DetailModeEditTags,
	}
	for i, want := range wantModes {
		if got := actions[i].Mode; got != want {
			t.Errorf("action %d: mode = %q, want %q", i, got, want)
		}
	}
}

func TestParseViewAction_ModeDefaultsToView(t *testing.T) {
	schema := testSchema()
	yamlSrc := `
version: "2"
views:
  - name: Detail
    key: d
    kind: detail
    layout: status
actions:
  - key: Enter
    label: Open
    kind: view
    view: Detail
`
	_, actions, errs := loadPluginsFromYAML(yamlSrc, schema)
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if len(actions) != 1 {
		t.Fatalf("got %d actions, want 1", len(actions))
	}
	if actions[0].Mode != DetailModeView {
		t.Errorf("default Mode = %q, want %q", actions[0].Mode, DetailModeView)
	}
}

func TestParseViewAction_ModeUnknownValue(t *testing.T) {
	schema := testSchema()
	yamlSrc := `
version: "2"
views:
  - name: Detail
    key: d
    kind: detail
    layout: status
actions:
  - key: e
    label: Edit
    kind: view
    view: Detail
    mode: garbage
`
	_, _, errs := loadPluginsFromYAML(yamlSrc, schema)
	if len(errs) == 0 {
		t.Fatal("expected parse error for unknown mode value")
	}
	found := false
	for _, err := range errs {
		if strings.Contains(err, "mode must be one of view, edit, new, edit-desc, edit-tags") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestParseViewAction_ModeRejectedOnRukiKind(t *testing.T) {
	schema := testSchema()
	yamlSrc := `
version: "2"
views:
  - name: Detail
    key: d
    kind: detail
    layout: status
actions:
  - key: x
    label: Bad
    kind: ruki
    action: update set status = "done"
    mode: edit
`
	_, _, errs := loadPluginsFromYAML(yamlSrc, schema)
	if len(errs) == 0 {
		t.Fatal("expected parse error for mode on kind: ruki")
	}
	found := false
	for _, err := range errs {
		if strings.Contains(err, "mode: only valid on kind: view actions") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestParseViewAction_ModeNewRejectedOnDetailViewActions(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:   "Detail",
		Kind:   "detail",
		Layout: "status",
		Actions: []PluginActionConfig{
			{Key: "n", Label: "New", Kind: "view", View: "Detail", Mode: "new"},
		},
	}
	viewNames := map[string]ViewKind{"Detail": KindDetail}
	_, err := parsePluginConfig(cfg, "test", schema, viewNames)
	if err == nil {
		t.Fatal("expected parse error for mode: new on a detail view's own actions")
	}
	if !strings.Contains(err.Error(), "mode: new not valid on a detail view's own actions") {
		t.Errorf("unexpected error: %v", err)
	}
}

// loadPluginsFromYAML is a test helper that parses a complete workflow YAML
// snippet and returns plugins, global actions, and any errors.
func loadPluginsFromYAML(yamlSrc string, schema ruki.Schema) ([]Plugin, []PluginAction, []string) {
	var wf WorkflowFile
	if err := yaml.Unmarshal([]byte(yamlSrc), &wf); err != nil {
		return nil, nil, []string{err.Error()}
	}

	viewNames, firstPassErrs := collectViewNames(wf.Views, "test")
	if len(firstPassErrs) > 0 {
		return nil, nil, firstPassErrs
	}

	var plugins []Plugin
	var errs []string
	for _, cfg := range wf.Views {
		if cfg.Name == "" {
			continue
		}
		p, err := parsePluginConfig(cfg, "test", schema, viewNames)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		plugins = append(plugins, p)
	}

	var actions []PluginAction
	if len(wf.Actions) > 0 {
		parser := ruki.NewParser(schema)
		parsedActions, err := parsePluginActions(wf.Actions, parser, viewNames, false)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			actions = parsedActions
		}
	}

	return plugins, actions, errs
}
