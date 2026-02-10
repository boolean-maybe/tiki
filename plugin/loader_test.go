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
		Sort: "Priority DESC",
		View: "expanded",
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

func TestLoadConfiguredPlugins_WorkflowFile(t *testing.T) {
	// create a temp directory with a workflow.yaml
	tmpDir := t.TempDir()
	workflowContent := `plugins:
  - name: TestBoard
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

	// change to temp dir so FindWorkflowFile() finds it in cwd
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}

	plugins := loadConfiguredPlugins()
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
}

func TestLoadConfiguredPlugins_NoWorkflowFile(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}

	plugins := loadConfiguredPlugins()
	if plugins != nil {
		t.Errorf("Expected nil plugins when no workflow.yaml, got %d", len(plugins))
	}
}

func TestLoadConfiguredPlugins_InvalidPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	workflowContent := `plugins:
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

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}

	// should load valid plugin and skip invalid one
	plugins := loadConfiguredPlugins()
	if len(plugins) != 1 {
		t.Fatalf("Expected 1 valid plugin (invalid skipped), got %d", len(plugins))
	}

	if plugins[0].GetName() != "Valid" {
		t.Errorf("Expected plugin 'Valid', got '%s'", plugins[0].GetName())
	}
}
