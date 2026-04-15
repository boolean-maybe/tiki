package plugin

import (
	"strings"
	"testing"

	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/ruki"
)

func testSchema() ruki.Schema {
	return rukiRuntime.NewSchema()
}

func testParser() *ruki.Parser {
	return ruki.NewParser(testSchema())
}

func TestDokiValidation(t *testing.T) {
	schema := testSchema()

	tests := []struct {
		name      string
		cfg       pluginFileConfig
		wantError string
	}{
		{
			name: "Missing Fetcher",
			cfg: pluginFileConfig{
				Name: "Invalid Doki",
				Type: "doki",
			},
			wantError: "doki plugin fetcher must be 'file' or 'internal'",
		},
		{
			name: "Invalid Fetcher",
			cfg: pluginFileConfig{
				Name:    "Invalid Fetcher",
				Type:    "doki",
				Fetcher: "http",
			},
			wantError: "doki plugin fetcher must be 'file' or 'internal'",
		},
		{
			name: "File Fetcher Missing URL",
			cfg: pluginFileConfig{
				Name:    "File No URL",
				Type:    "doki",
				Fetcher: "file",
			},
			wantError: "doki plugin with file fetcher requires 'url'",
		},
		{
			name: "Internal Fetcher Missing Text",
			cfg: pluginFileConfig{
				Name:    "Internal No Text",
				Type:    "doki",
				Fetcher: "internal",
			},
			wantError: "doki plugin with internal fetcher requires 'text'",
		},
		{
			name: "Valid File Fetcher",
			cfg: pluginFileConfig{
				Name:    "Valid File",
				Type:    "doki",
				Fetcher: "file",
				URL:     "http://example.com",
			},
			wantError: "",
		},
		{
			name: "Valid Internal Fetcher",
			cfg: pluginFileConfig{
				Name:    "Valid Internal",
				Type:    "doki",
				Fetcher: "internal",
				Text:    "content",
			},
			wantError: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePluginConfig(tc.cfg, "test", schema)
			if tc.wantError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tc.wantError)
				} else if !strings.Contains(err.Error(), tc.wantError) {
					t.Errorf("Expected error containing '%s', got '%v'", tc.wantError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got '%v'", err)
				}
			}
		})
	}
}

func TestTikiValidation(t *testing.T) {
	schema := testSchema()

	tests := []struct {
		name      string
		cfg       pluginFileConfig
		wantError string
	}{
		{
			name: "Tiki with Doki fields (Fetcher)",
			cfg: pluginFileConfig{
				Name:    "Tiki with Fetcher",
				Type:    "tiki",
				Fetcher: "file",
				Lanes: []PluginLaneConfig{
					{Name: "Todo", Filter: `select where status = "ready"`},
				},
			},
			wantError: "tiki plugin cannot have 'fetcher'",
		},
		{
			name: "Tiki with Doki fields (Text)",
			cfg: pluginFileConfig{
				Name: "Tiki with Text",
				Type: "tiki",
				Text: "text",
				Lanes: []PluginLaneConfig{
					{Name: "Todo", Filter: `select where status = "ready"`},
				},
			},
			wantError: "tiki plugin cannot have 'text'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePluginConfig(tc.cfg, "test", schema)
			if tc.wantError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tc.wantError)
				} else if !strings.Contains(err.Error(), tc.wantError) {
					t.Errorf("Expected error containing '%s', got '%v'", tc.wantError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got '%v'", err)
				}
			}
		})
	}
}

func TestParsePluginConfig_InvalidKey(t *testing.T) {
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "InvalidKey",
		Type: "tiki",
	}

	_, err := parsePluginConfig(cfg, "test.yaml", testSchema())
	if err == nil {
		t.Fatal("Expected error for invalid key format")
	}

	if !strings.Contains(err.Error(), "parsing key") {
		t.Errorf("Expected 'parsing key' error, got: %v", err)
	}
}

func TestParsePluginConfig_DefaultTikiType(t *testing.T) {
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Lanes: []PluginLaneConfig{
			{Name: "Todo", Filter: `select where status = "ready"`},
		},
		// Type not specified, should default to "tiki"
	}

	plugin, err := parsePluginConfig(cfg, "test.yaml", testSchema())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if _, ok := plugin.(*TikiPlugin); !ok {
		t.Errorf("Expected TikiPlugin when type not specified, got %T", plugin)
	}
}

func TestParsePluginConfig_UnknownType(t *testing.T) {
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Type: "unknown",
	}

	_, err := parsePluginConfig(cfg, "test.yaml", testSchema())
	if err == nil {
		t.Fatal("Expected error for unknown plugin type")
	}

	expected := "unknown plugin type: unknown"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got: %v", expected, err)
	}
}

// TestParsePluginConfig_TikiWithInvalidSort removed - the sort parser is very lenient
// and accepts most field names. Invalid syntax would be caught by ParseSort internally.

func TestParsePluginConfig_DokiWithView(t *testing.T) {
	cfg := pluginFileConfig{
		Name:    "Test",
		Key:     "T",
		Type:    "doki",
		Fetcher: "internal",
		Text:    "content",
		View:    "expanded", // Doki shouldn't have view
	}

	_, err := parsePluginConfig(cfg, "test.yaml", testSchema())
	if err == nil {
		t.Fatal("Expected error for doki with view field")
	}

	if !strings.Contains(err.Error(), "doki plugin cannot have 'view'") {
		t.Errorf("Expected 'cannot have view' error, got: %v", err)
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
type: tiki
lanes:
  - name: Todo
    columns: 4
    filter: select where status = "ready"
view: expanded
foreground: "#ff0000"
background: "#0000ff"
`)

	plugin, err := parsePluginYAML(validYAML, "test.yaml", testSchema())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	tikiPlugin, ok := plugin.(*TikiPlugin)
	if !ok {
		t.Fatalf("Expected TikiPlugin, got %T", plugin)
		return
	}

	if tikiPlugin.GetName() != "Test Plugin" {
		t.Errorf("Expected name 'Test Plugin', got %q", tikiPlugin.GetName())
	}

	if tikiPlugin.ViewMode != "expanded" {
		t.Errorf("Expected view mode 'expanded', got %q", tikiPlugin.ViewMode)
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

	actions, err := parsePluginActions(configs, parser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	if actions[0].Rune != 'b' {
		t.Errorf("expected rune 'b', got %q", actions[0].Rune)
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

	actions, err := parsePluginActions(nil, parser)
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
			wantErr: "single character",
		},
		{
			name:    "missing label",
			configs: []PluginActionConfig{{Key: "b", Label: "", Action: `update where id = id() set status="ready"`}},
			wantErr: "missing 'label'",
		},
		{
			name:    "missing action",
			configs: []PluginActionConfig{{Key: "b", Label: "Test", Action: ""}},
			wantErr: "missing 'action'",
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
		{
			name: "too many actions",
			configs: func() []PluginActionConfig {
				configs := make([]PluginActionConfig, 11)
				for i := range configs {
					configs[i] = PluginActionConfig{Key: string(rune('a' + i)), Label: "Test", Action: `update where id = id() set status="ready"`}
				}
				return configs
			}(),
			wantErr: "too many actions",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePluginActions(tc.configs, parser)
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
lanes:
  - name: Backlog
    filter: select where status = "backlog"
actions:
  - key: "b"
    label: "Add to board"
    action: update where id = id() set status = "ready"
`)

	p, err := parsePluginYAML(yamlData, "test.yaml", testSchema())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tiki, ok := p.(*TikiPlugin)
	if !ok {
		t.Fatalf("expected TikiPlugin, got %T", p)
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

func TestParsePluginConfig_DokiWithActions(t *testing.T) {
	cfg := pluginFileConfig{
		Name:    "Test",
		Key:     "T",
		Type:    "doki",
		Fetcher: "internal",
		Text:    "content",
		Actions: []PluginActionConfig{
			{Key: "b", Label: "Test", Action: `update where id = id() set status="ready"`},
		},
	}

	_, err := parsePluginConfig(cfg, "test.yaml", testSchema())
	if err == nil {
		t.Fatal("expected error for doki with actions")
	}
	if !strings.Contains(err.Error(), "doki plugin cannot have 'actions'") {
		t.Errorf("expected 'cannot have actions' error, got: %v", err)
	}
}

func TestParsePluginConfig_LaneFilterMustBeSelect(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Filter: `update where id = id() set status = "ready"`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema)
	if err == nil {
		t.Fatal("expected error for non-SELECT filter")
	}
	if !strings.Contains(err.Error(), "filter must be a SELECT") {
		t.Errorf("expected 'filter must be a SELECT' error, got: %v", err)
	}
}

func TestParsePluginConfig_LaneActionMustBeUpdate(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Filter: `select where status = "ready"`, Action: `select where status = "done"`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema)
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
	actions, err := parsePluginActions(configs, parser)
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
	actions, err := parsePluginActions(configs, parser)
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

func TestParsePluginConfig_LaneFilterParseError(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Filter: "totally invalid @@@"},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema)
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
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Filter: `select where status = "ready"`, Action: "totally invalid @@@"},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema)
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
		Lanes: []PluginLaneConfig{
			{Name: "", Filter: `select where status = "ready"`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema)
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
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Columns: -1, Filter: `select where status = "ready"`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema)
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
		Lanes: []PluginLaneConfig{
			{Name: "Bad", Width: 101, Filter: `select where status = "ready"`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema)
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
		Lanes: lanes,
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema)
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
		Type:  "tiki",
		Lanes: []PluginLaneConfig{},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema)
	if err == nil {
		t.Fatal("expected error for no lanes")
	}
	if !strings.Contains(err.Error(), "requires 'lanes'") {
		t.Errorf("expected 'requires lanes' error, got: %v", err)
	}
}

func TestParsePluginConfig_TikiWithURL(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Type: "tiki",
		URL:  "http://example.com",
		Lanes: []PluginLaneConfig{
			{Name: "Todo", Filter: `select where status = "ready"`},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema)
	if err == nil {
		t.Fatal("expected error for tiki with url")
	}
	if !strings.Contains(err.Error(), "tiki plugin cannot have 'url'") {
		t.Errorf("expected 'cannot have url' error, got: %v", err)
	}
}

func TestParsePluginConfig_DokiWithLanes(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:    "Test",
		Key:     "T",
		Type:    "doki",
		Fetcher: "internal",
		Text:    "content",
		Lanes:   []PluginLaneConfig{{Name: "Bad"}},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema)
	if err == nil {
		t.Fatal("expected error for doki with lanes")
	}
	if !strings.Contains(err.Error(), "doki plugin cannot have 'lanes'") {
		t.Errorf("expected 'cannot have lanes' error, got: %v", err)
	}
}

func TestParsePluginConfig_PluginActionsError(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Test",
		Key:  "T",
		Lanes: []PluginLaneConfig{
			{Name: "Todo", Filter: `select where status = "ready"`},
		},
		Actions: []PluginActionConfig{
			{Key: "b", Label: "Bad", Action: "totally invalid @@@"},
		},
	}
	_, err := parsePluginConfig(cfg, "test.yaml", schema)
	if err == nil {
		t.Fatal("expected error for invalid plugin action")
	}
}

func TestParsePluginActions_NonPrintableKey(t *testing.T) {
	parser := testParser()
	configs := []PluginActionConfig{
		{Key: "\x01", Label: "Test", Action: `update where id = id() set status="ready"`},
	}
	_, err := parsePluginActions(configs, parser)
	if err == nil {
		t.Fatal("expected error for non-printable key")
	}
	if !strings.Contains(err.Error(), "printable character") {
		t.Errorf("expected 'printable character' error, got: %v", err)
	}
}

func TestParsePluginYAML_ValidDoki(t *testing.T) {
	validYAML := []byte(`
name: Doc Plugin
key: D
type: doki
fetcher: file
url: http://example.com/doc
foreground: "#00ff00"
`)

	plugin, err := parsePluginYAML(validYAML, "test.yaml", testSchema())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	dokiPlugin, ok := plugin.(*DokiPlugin)
	if !ok {
		t.Fatalf("Expected DokiPlugin, got %T", plugin)
		return
	}

	if dokiPlugin.GetName() != "Doc Plugin" {
		t.Errorf("Expected name 'Doc Plugin', got %q", dokiPlugin.GetName())
	}

	if dokiPlugin.Fetcher != "file" {
		t.Errorf("Expected fetcher 'file', got %q", dokiPlugin.Fetcher)
	}

	if dokiPlugin.URL != "http://example.com/doc" {
		t.Errorf("Expected URL, got %q", dokiPlugin.URL)
	}
}
