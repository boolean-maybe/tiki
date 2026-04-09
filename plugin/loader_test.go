package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePluginConfig_FullyInline(t *testing.T) {
	schema := testSchema()

	cfg := pluginFileConfig{
		Name:       "Inline Test",
		Foreground: "#ffffff",
		Background: "#000000",
		Key:        "I",
		Lanes: []PluginLaneConfig{
			{Name: "Todo", Filter: `select where status = "ready"`},
		},
		View:    "expanded",
		Default: true,
	}

	def, err := parsePluginConfig(cfg, "test", schema)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	tp, ok := def.(*TikiPlugin)
	if !ok {
		t.Fatalf("Expected TikiPlugin, got %T", def)
	}

	if tp.Name != "Inline Test" {
		t.Errorf("Expected name 'Inline Test', got '%s'", tp.Name)
	}

	if tp.Rune != 'I' {
		t.Errorf("Expected rune 'I', got '%c'", tp.Rune)
	}

	if tp.ViewMode != "expanded" {
		t.Errorf("Expected view mode 'expanded', got '%s'", tp.ViewMode)
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
		Lanes: []PluginLaneConfig{
			{Name: "Bugs", Filter: `select where type = "bug"`},
		},
	}

	def, err := parsePluginConfig(cfg, "test", testSchema())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	tp, ok := def.(*TikiPlugin)
	if !ok {
		t.Fatalf("Expected TikiPlugin, got %T", def)
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

	_, err := parsePluginConfig(cfg, "test", testSchema())
	if err == nil {
		t.Fatal("Expected error for plugin without name")
	}
}

func TestPluginTypeExplicit(t *testing.T) {
	// inline plugin with type doki
	cfg := pluginFileConfig{
		Name:    "Type Doki Test",
		Type:    "doki",
		Fetcher: "internal",
		Text:    "some text",
	}

	def, err := parsePluginConfig(cfg, "test", testSchema())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if def.GetType() != "doki" {
		t.Errorf("Expected type 'doki', got '%s'", def.GetType())
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
    default: true
    key: "F5"
    lanes:
      - name: Ready
        filter: select where status = "ready"
  - name: TestDocs
    type: doki
    fetcher: internal
    text: "hello"
    key: "D"
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("Failed to write workflow.yaml: %v", err)
	}

	plugins, errs := loadPluginsFromFile(workflowPath, testSchema())
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
	plugins, errs := loadPluginsFromFile(filepath.Join(tmpDir, "workflow.yaml"), testSchema())
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
    key: "V"
    lanes:
      - name: Todo
        filter: select where status = "ready"
  - name: Invalid
    type: unknown
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("Failed to write workflow.yaml: %v", err)
	}

	// should load valid plugin and skip invalid one
	plugins, errs := loadPluginsFromFile(workflowPath, testSchema())
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

func TestLoadPluginsFromFile_LegacyConversion(t *testing.T) {
	tmpDir := t.TempDir()
	// workflow with legacy filter expressions that need conversion
	workflowContent := `views:
  - name: Board
    key: "F5"
    sort: Priority
    lanes:
      - name: Ready
        filter: status = 'ready'
        action: status = 'inProgress'
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	plugins, errs := loadPluginsFromFile(workflowPath, testSchema())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	tp, ok := plugins[0].(*TikiPlugin)
	if !ok {
		t.Fatalf("expected TikiPlugin, got %T", plugins[0])
	}

	// filter should have been converted and parsed (with order by from sort)
	if tp.Lanes[0].Filter == nil {
		t.Fatal("expected filter to be parsed after legacy conversion")
	}
	if !tp.Lanes[0].Filter.IsSelect() {
		t.Error("expected SELECT filter after conversion")
	}
	if tp.Lanes[0].Action == nil {
		t.Fatal("expected action to be parsed after legacy conversion")
	}
	if !tp.Lanes[0].Action.IsUpdate() {
		t.Error("expected UPDATE action after conversion")
	}
}

func TestLoadPluginsFromFile_UnnamedPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	workflowContent := `views:
  - name: Valid
    key: "V"
    lanes:
      - name: Todo
        filter: select where status = "ready"
  - lanes:
      - name: Bad
        filter: select where status = "done"
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	plugins, errs := loadPluginsFromFile(workflowPath, testSchema())
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

	plugins, errs := loadPluginsFromFile(workflowPath, testSchema())
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

	plugins, errs := loadPluginsFromFile(workflowPath, testSchema())
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
    key: "B"
    lanes:
      - name: Todo
        filter: select where status = "ready"
  - name: Docs
    key: "D"
    type: doki
    fetcher: internal
    text: "hello"
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}

	plugins, errs := loadPluginsFromFile(workflowPath, testSchema())
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
	}
	if dp.ConfigIndex != 1 {
		t.Errorf("expected DokiPlugin ConfigIndex 1, got %d", dp.ConfigIndex)
	}
}

func TestMergePluginLists(t *testing.T) {
	base := []Plugin{
		&TikiPlugin{BasePlugin: BasePlugin{Name: "Board", FilePath: "base.yaml"}},
		&TikiPlugin{BasePlugin: BasePlugin{Name: "Bugs", FilePath: "base.yaml"}},
	}
	overrides := []Plugin{
		&TikiPlugin{BasePlugin: BasePlugin{Name: "Board", FilePath: "override.yaml"}},
		&TikiPlugin{BasePlugin: BasePlugin{Name: "NewView", FilePath: "override.yaml"}},
	}

	result := mergePluginLists(base, overrides)

	// Bugs (non-overridden) + Board (merged) + NewView (new)
	if len(result) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(result))
	}

	names := make([]string, len(result))
	for i, p := range result {
		names[i] = p.GetName()
	}

	// Bugs should come first (non-overridden base), then Board (merged), then NewView (new)
	if names[0] != "Bugs" {
		t.Errorf("expected first plugin 'Bugs', got %q", names[0])
	}
	if names[1] != "Board" {
		t.Errorf("expected second plugin 'Board', got %q", names[1])
	}
	if names[2] != "NewView" {
		t.Errorf("expected third plugin 'NewView', got %q", names[2])
	}
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
