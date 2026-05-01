package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/ruki"
)

func TestParsePluginConfig_FullyInline(t *testing.T) {
	schema := testSchema()

	cfg := pluginFileConfig{
		Name:       "Inline Test",
		Kind:       "board",
		Foreground: "#ffffff",
		Background: "#000000",
		Key:        "I",
		Lanes: []PluginLaneConfig{
			{Name: "Todo", Filter: `select where status = "ready"`},
		},
		Mode:    "expanded",
		Default: true,
	}

	def, err := parsePluginConfig(cfg, "test", schema, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	tp, ok := def.(*TikiPlugin)
	if !ok {
		t.Fatalf("Expected TikiPlugin, got %T", def)
		return
	}

	if tp.Name != "Inline Test" {
		t.Errorf("Expected name 'Inline Test', got '%s'", tp.Name)
	}

	if tp.Rune != 'I' {
		t.Errorf("Expected rune 'I', got '%c'", tp.Rune)
	}

	if tp.Mode != "expanded" {
		t.Errorf("Expected view mode 'expanded', got '%s'", tp.Mode)
	}

	if len(tp.Lanes) != 1 || tp.Lanes[0].Filter == nil {
		t.Fatal("Expected lane filter to be parsed")
	}

	if !tp.Lanes[0].Filter.IsSelect() {
		t.Error("Expected lane filter to be a SELECT statement")
	}

	if !tp.IsDefault() {
		t.Error("Expected IsDefault() to return true")
	}
}

func TestParsePluginConfig_Minimal(t *testing.T) {
	cfg := pluginFileConfig{
		Name: "Minimal",
		Kind: "board",
		Lanes: []PluginLaneConfig{
			{Name: "Bugs", Filter: `select where type = "bug"`},
		},
	}

	def, err := parsePluginConfig(cfg, "test", testSchema(), nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	tp, ok := def.(*TikiPlugin)
	if !ok {
		t.Fatalf("Expected TikiPlugin, got %T", def)
		return
	}

	if tp.Name != "Minimal" {
		t.Errorf("Expected name 'Minimal', got '%s'", tp.Name)
	}

	if len(tp.Lanes) != 1 || tp.Lanes[0].Filter == nil {
		t.Error("Expected lane filter to be parsed")
	}
}

func TestParsePluginConfig_NoName(t *testing.T) {
	cfg := pluginFileConfig{
		Lanes: []PluginLaneConfig{
			{Name: "Todo", Filter: `select where status = "ready"`},
		},
	}

	_, err := parsePluginConfig(cfg, "test", testSchema(), nil)
	if err == nil {
		t.Fatal("Expected error for plugin without name")
	}
}

// TestWikiKindExplicit asserts a wiki view parses to DokiPlugin with kind wiki.
func TestWikiKindExplicit(t *testing.T) {
	cfg := pluginFileConfig{
		Name: "Wiki Test",
		Kind: "wiki",
		Path: "index.md",
	}

	def, err := parsePluginConfig(cfg, "test", testSchema(), nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if def.GetKind() != KindWiki {
		t.Errorf("Expected kind wiki, got %q", def.GetKind())
	}
	if _, ok := def.(*DokiPlugin); !ok {
		t.Errorf("Expected DokiPlugin type assertion to succeed")
	}
}

func TestLoadPluginsFromFile_WorkflowFile(t *testing.T) {
	// create a temp directory with a workflow.yaml
	tmpDir := t.TempDir()
	workflowContent := `views:
  - name: TestBoard
    kind: board
    default: true
    key: "F5"
    lanes:
      - name: Ready
        filter: select where status = "ready"
  - name: TestDocs
    kind: wiki
    path: "hello.md"
    key: "D"
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("Failed to write workflow.yaml: %v", err)
	}

	plugins, _, errs := loadPluginsFromFile(workflowPath, testSchema())
	if len(errs) != 0 {
		t.Fatalf("Expected no errors, got: %v", errs)
	}
	if len(plugins) != 2 {
		t.Fatalf("Expected 2 plugins, got %d", len(plugins))
	}

	if plugins[0].GetName() != "TestBoard" {
		t.Errorf("Expected first plugin 'TestBoard', got '%s'", plugins[0].GetName())
	}
	if plugins[1].GetName() != "TestDocs" {
		t.Errorf("Expected second plugin 'TestDocs', got '%s'", plugins[1].GetName())
	}

	// verify config indices
	if plugins[0].GetConfigIndex() != 0 {
		t.Errorf("Expected config index 0, got %d", plugins[0].GetConfigIndex())
	}
	if plugins[1].GetConfigIndex() != 1 {
		t.Errorf("Expected config index 1, got %d", plugins[1].GetConfigIndex())
	}

	// verify default flag
	if !plugins[0].IsDefault() {
		t.Error("Expected TestBoard to be default")
	}
	if plugins[1].IsDefault() {
		t.Error("Expected TestDocs to not be default")
	}
}

func TestLoadPluginsFromFile_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	plugins, _, errs := loadPluginsFromFile(filepath.Join(tmpDir, "workflow.yaml"), testSchema())
	if plugins != nil {
		t.Errorf("Expected nil plugins when no workflow.yaml, got %d", len(plugins))
	}
	if len(errs) != 1 {
		t.Errorf("Expected 1 error for missing file, got %d", len(errs))
	}
}

func TestLoadPluginsFromFile_InvalidPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	workflowContent := `views:
  - name: Valid
    kind: board
    key: "V"
    lanes:
      - name: Todo
        filter: select where status = "ready"
  - name: Invalid
    kind: galaxy
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("Failed to write workflow.yaml: %v", err)
	}

	// should load valid plugin and skip invalid one
	plugins, _, errs := loadPluginsFromFile(workflowPath, testSchema())
	if len(plugins) != 1 {
		t.Fatalf("Expected 1 valid plugin (invalid skipped), got %d", len(plugins))
	}
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error for invalid plugin, got %d: %v", len(errs), errs)
	}

	if plugins[0].GetName() != "Valid" {
		t.Errorf("Expected plugin 'Valid', got '%s'", plugins[0].GetName())
	}
}

// TestLoadPluginsFromFile_FailClosedOnAnyError asserts the PUBLIC entry point
// refuses to boot when the workflow contains any parse error, even if at
// least one view parses cleanly. Partial workflows diverge from the user's
// declared intent and must not silently succeed.
func TestLoadPluginsFromFile_FailClosedOnAnyError(t *testing.T) {
	tmpDir := t.TempDir()
	workflowContent := `views:
  - name: Valid
    kind: board
    key: "V"
    lanes:
      - name: Todo
        filter: select where status = "ready"
  - name: Invalid
    kind: galaxy
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	_, err := LoadPluginsFromFile(workflowPath, testSchema())
	if err == nil {
		t.Fatal("expected fail-closed error when any view fails to parse; got nil")
	}
	if !strings.Contains(err.Error(), "did not load cleanly") {
		t.Errorf("expected fail-closed wrapper error, got: %v", err)
	}
}

// TestLoadPluginsFromFile_FailsOnDuplicateViewName asserts duplicate view
// names (now reported as a non-empty errs slice by the internal loader) cause
// the public entry point to fail the load.
func TestLoadPluginsFromFile_FailsOnDuplicateViewName(t *testing.T) {
	tmpDir := t.TempDir()
	workflowContent := `views:
  - name: Board
    kind: board
    key: "V"
    lanes:
      - name: Todo
        filter: select where status = "ready"
  - name: Board
    kind: board
    key: "W"
    lanes:
      - name: Done
        filter: select where status = "done"
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	_, err := LoadPluginsFromFile(workflowPath, testSchema())
	if err == nil {
		t.Fatal("expected failure on duplicate view name; got nil")
	}
	if !strings.Contains(err.Error(), "duplicate view name") {
		t.Errorf("expected duplicate-view-name error, got: %v", err)
	}
}

func TestDefaultPlugin_ExplicitDefault(t *testing.T) {
	plugins := []Plugin{
		&TikiPlugin{BasePlugin: BasePlugin{Name: "First"}},
		&TikiPlugin{BasePlugin: BasePlugin{Name: "Second", Default: true}},
		&TikiPlugin{BasePlugin: BasePlugin{Name: "Third"}},
	}
	got := DefaultPlugin(plugins)
	if got.GetName() != "Second" {
		t.Errorf("Expected 'Second' (marked default), got %q", got.GetName())
	}
}

func TestDefaultPlugin_NoDefault(t *testing.T) {
	plugins := []Plugin{
		&TikiPlugin{BasePlugin: BasePlugin{Name: "Alpha"}},
		&TikiPlugin{BasePlugin: BasePlugin{Name: "Beta"}},
	}
	got := DefaultPlugin(plugins)
	if got.GetName() != "Alpha" {
		t.Errorf("Expected first plugin 'Alpha' as fallback, got %q", got.GetName())
	}
}

// TestLoadPluginsFromFile_LegacySortRejected asserts that `sort:` — a
// pre-Phase-6 field — is rejected with a clear error and no plugins load.
func TestLoadPluginsFromFile_LegacySortRejected(t *testing.T) {
	tmpDir := t.TempDir()
	workflowContent := `views:
  - name: Board
    kind: board
    key: "F5"
    sort: Priority
    lanes:
      - name: Ready
        filter: select where status = "ready"
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	plugins, _, errs := loadPluginsFromFile(workflowPath, testSchema())
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "sort") || !strings.Contains(errs[0], "order by") {
		t.Errorf("expected error to mention sort/order by, got %q", errs[0])
	}
}

func TestLoadPluginsFromFile_UnnamedPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	workflowContent := `views:
  - name: Valid
    kind: board
    key: "V"
    lanes:
      - name: Todo
        filter: select where status = "ready"
  - kind: board
    lanes:
      - name: Bad
        filter: select where status = "done"
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	plugins, _, errs := loadPluginsFromFile(workflowPath, testSchema())
	// unnamed plugin should be skipped, valid one should load
	if len(plugins) != 1 {
		t.Fatalf("expected 1 valid plugin, got %d", len(plugins))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for unnamed plugin, got %d: %v", len(errs), errs)
	}
	if plugins[0].GetName() != "Valid" {
		t.Errorf("expected plugin 'Valid', got %q", plugins[0].GetName())
	}
}

func TestLoadPluginsFromFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	plugins, _, errs := loadPluginsFromFile(workflowPath, testSchema())
	if plugins != nil {
		t.Error("expected nil plugins for invalid YAML")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for invalid YAML, got %d: %v", len(errs), errs)
	}
}

func TestLoadPluginsFromFile_EmptyViews(t *testing.T) {
	tmpDir := t.TempDir()
	workflowContent := `views: []
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	plugins, _, errs := loadPluginsFromFile(workflowPath, testSchema())
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins for empty views, got %d", len(plugins))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors for empty views, got %d", len(errs))
	}
}

func TestLoadPluginsFromFile_DokiConfigIndex(t *testing.T) {
	tmpDir := t.TempDir()
	workflowContent := `views:
  - name: Board
    kind: board
    key: "B"
    lanes:
      - name: Todo
        filter: select where status = "ready"
  - name: Docs
    kind: wiki
    key: "D"
    path: "index.md"
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	plugins, _, errs := loadPluginsFromFile(workflowPath, testSchema())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}

	// verify DokiPlugin has correct ConfigIndex
	dp, ok := plugins[1].(*DokiPlugin)
	if !ok {
		t.Fatalf("expected DokiPlugin, got %T", plugins[1])
		return
	}
	if dp.ConfigIndex != 1 {
		t.Errorf("expected DokiPlugin ConfigIndex 1, got %d", dp.ConfigIndex)
	}
}

func TestLoadPluginsFromFile_GlobalActions(t *testing.T) {
	tmpDir := t.TempDir()
	workflowContent := `actions:
  - key: "a"
    label: "Assign to me"
    action: update where id = id() set assignee=user()
views:
  - name: Board
    kind: board
    key: "B"
    lanes:
      - name: Todo
        filter: select where status = "ready"
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	plugins, globalActions, errs := loadPluginsFromFile(workflowPath, testSchema())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if len(globalActions) != 1 {
		t.Fatalf("expected 1 global action, got %d", len(globalActions))
	}
	if globalActions[0].Rune != 'a' {
		t.Errorf("expected rune 'a', got %q", globalActions[0].Rune)
	}
	if globalActions[0].Label != "Assign to me" {
		t.Errorf("expected label 'Assign to me', got %q", globalActions[0].Label)
	}
}

// TestLoadPluginsFromFile_RejectsViewsPluginsWrapper asserts that the old
// `views.plugins:` map shape is rejected with a clear error pointing at the
// new top-level `views: [...]` list.
func TestLoadPluginsFromFile_RejectsViewsPluginsWrapper(t *testing.T) {
	tmpDir := t.TempDir()
	workflowContent := `views:
  plugins:
    - name: Board
      kind: board
      key: "B"
      lanes:
        - name: Todo
          filter: select where status = "ready"
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	plugins, _, errs := loadPluginsFromFile(workflowPath, testSchema())
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "views.plugins") {
		t.Errorf("expected error to mention views.plugins, got %q", errs[0])
	}
}

func TestMergeGlobalActionsIntoPlugins(t *testing.T) {
	stmt := mustParseAction(t, `update where id = id() set status="ready"`)

	plugins := []Plugin{
		&TikiPlugin{
			BasePlugin: BasePlugin{Name: "Board"},
			Actions:    []PluginAction{{Rune: 'b', KeyStr: "b", Label: "Board action", Action: stmt}},
		},
		&TikiPlugin{
			BasePlugin: BasePlugin{Name: "Backlog"},
			Actions:    nil,
		},
		&DokiPlugin{
			BasePlugin: BasePlugin{Name: "Help"},
		},
	}

	globals := []PluginAction{
		{Rune: 'a', KeyStr: "a", Label: "Assign", Action: stmt},
		{Rune: 'b', KeyStr: "b", Label: "Global board", Action: stmt}, // conflicts with Board's 'b'
	}

	mergeGlobalActionsIntoPlugins(plugins, globals)

	// Board: should have 'b' (local) + 'a' (global) — 'b' global skipped
	board, ok := plugins[0].(*TikiPlugin)
	if !ok {
		t.Fatalf("Board: expected *TikiPlugin, got %T", plugins[0])
	}
	if len(board.Actions) != 2 {
		t.Fatalf("Board: expected 2 actions, got %d", len(board.Actions))
	}
	if board.Actions[0].Label != "Board action" {
		t.Errorf("Board: first action should be local 'Board action', got %q", board.Actions[0].Label)
	}
	if board.Actions[1].Label != "Assign" {
		t.Errorf("Board: second action should be global 'Assign', got %q", board.Actions[1].Label)
	}

	// Backlog: should have both globals ('a' and 'b')
	backlog, ok := plugins[1].(*TikiPlugin)
	if !ok {
		t.Fatalf("Backlog: expected *TikiPlugin, got %T", plugins[1])
	}
	if len(backlog.Actions) != 2 {
		t.Fatalf("Backlog: expected 2 actions, got %d", len(backlog.Actions))
	}

	// Help (DokiPlugin): should have no actions (skipped)
	// DokiPlugin has no Actions field — nothing to check
}

func TestMergeGlobalActionsIntoPlugins_ShiftedRuneAlias(t *testing.T) {
	stmt := mustParseAction(t, `update where id = id() set status="ready"`)

	plugins := []Plugin{
		&TikiPlugin{
			BasePlugin: BasePlugin{Name: "Board"},
			Actions:    []PluginAction{{Rune: 'X', KeyStr: "X", Label: "Local X", Action: stmt}},
		},
	}

	globals := []PluginAction{
		{Rune: 'X', KeyStr: "X", Label: "Global X", Action: stmt},
		{Rune: 'x', KeyStr: "x", Label: "Global x", Action: stmt},
	}

	mergeGlobalActionsIntoPlugins(plugins, globals)

	board, ok := plugins[0].(*TikiPlugin)
	if !ok {
		t.Fatal("expected *TikiPlugin")
	}
	if len(board.Actions) != 2 {
		t.Fatalf("expected 2 actions (local X + global x), got %d", len(board.Actions))
	}
	if board.Actions[0].Label != "Local X" {
		t.Errorf("expected local X first, got %q", board.Actions[0].Label)
	}
	if board.Actions[1].Label != "Global x" {
		t.Errorf("expected global x second, got %q", board.Actions[1].Label)
	}
}

func mustParseAction(t *testing.T, input string) *ruki.ValidatedStatement { //nolint:unparam // test helper designed for varied inputs
	t.Helper()
	parser := testParser()
	stmt, err := parser.ParseAndValidateStatement(input, ruki.ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("parse ruki statement %q: %v", input, err)
	}
	return stmt
}

func TestDefaultPlugin_MultipleDefaults(t *testing.T) {
	plugins := []Plugin{
		&TikiPlugin{BasePlugin: BasePlugin{Name: "A"}},
		&TikiPlugin{BasePlugin: BasePlugin{Name: "B", Default: true}},
		&TikiPlugin{BasePlugin: BasePlugin{Name: "C", Default: true}},
	}
	got := DefaultPlugin(plugins)
	if got.GetName() != "B" {
		t.Errorf("Expected first default 'B', got %q", got.GetName())
	}
}
