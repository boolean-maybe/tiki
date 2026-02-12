package plugin

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

func TestParsePluginConfig_FullyInline(t *testing.T) {
	cfg := pluginFileConfig{
		Name:       "Inline Test",
		Foreground: "#ffffff",
		Background: "#000000",
		Key:        "I",
		Lanes: []PluginLaneConfig{
			{Name: "Todo", Filter: "status = 'ready'"},
		},
		Sort:    "Priority DESC",
		View:    "expanded",
		Default: true,
	}

	def, err := parsePluginConfig(cfg, "test")
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

	if len(tp.Sort) != 1 || tp.Sort[0].Field != "priority" || !tp.Sort[0].Descending {
		t.Errorf("Expected sort 'Priority DESC', got %+v", tp.Sort)
	}

	if !tp.IsDefault() {
		t.Error("Expected IsDefault() to return true")
	}

	// test filter evaluation
	task := &taskpkg.Task{
		ID:     "TIKI-1",
		Status: taskpkg.StatusReady,
	}

	if !tp.Lanes[0].Filter.Evaluate(task, time.Now(), "testuser") {
		t.Error("Expected filter to match todo task")
	}
}

func TestParsePluginConfig_Minimal(t *testing.T) {
	cfg := pluginFileConfig{
		Name: "Minimal",
		Lanes: []PluginLaneConfig{
			{Name: "Bugs", Filter: "type = 'bug'"},
		},
	}

	def, err := parsePluginConfig(cfg, "test")
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
			{Name: "Todo", Filter: "status = 'ready'"},
		},
	}

	_, err := parsePluginConfig(cfg, "test")
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

	def, err := parsePluginConfig(cfg, "test")
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
        filter: status = 'ready'
    sort: Priority
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

	plugins, errs := loadPluginsFromFile(workflowPath)
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
	plugins, errs := loadPluginsFromFile(filepath.Join(tmpDir, "workflow.yaml"))
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
        filter: status = 'ready'
  - name: Invalid
    type: unknown
`
	workflowPath := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("Failed to write workflow.yaml: %v", err)
	}

	// should load valid plugin and skip invalid one
	plugins, errs := loadPluginsFromFile(workflowPath)
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
