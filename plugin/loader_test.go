package plugin

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

func TestLoadPluginFromRef_FullyInline(t *testing.T) {
	ref := PluginRef{
		Name:       "Inline Test",
		Foreground: "#ffffff",
		Background: "#000000",
		Key:        "I",
		Panes: []PluginPaneConfig{
			{Name: "Todo", Filter: "status = 'ready'"},
		},
		Sort: "Priority DESC",
		View: "expanded",
	}

	def, err := loadPluginFromRef(ref)
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

	if len(tp.Panes) != 1 || tp.Panes[0].Filter == nil {
		t.Fatal("Expected pane filter to be parsed")
	}

	if len(tp.Sort) != 1 || tp.Sort[0].Field != "priority" || !tp.Sort[0].Descending {
		t.Errorf("Expected sort 'Priority DESC', got %+v", tp.Sort)
	}

	// Test filter evaluation
	task := &taskpkg.Task{
		ID:     "TIKI-1",
		Status: taskpkg.StatusReady,
	}

	if !tp.Panes[0].Filter.Evaluate(task, time.Now(), "testuser") {
		t.Error("Expected filter to match todo task")
	}
}

func TestLoadPluginFromRef_InlineMinimal(t *testing.T) {
	ref := PluginRef{
		Name: "Minimal",
		Panes: []PluginPaneConfig{
			{Name: "Bugs", Filter: "type = 'bug'"},
		},
	}

	def, err := loadPluginFromRef(ref)
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

	if len(tp.Panes) != 1 || tp.Panes[0].Filter == nil {
		t.Error("Expected pane filter to be parsed")
	}
}

func TestLoadPluginFromRef_FileBased(t *testing.T) {
	// Create temp plugin file
	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "test-plugin.yaml")
	content := `name: Test Plugin
foreground: "#ff0000"
background: "#0000ff"
key: T
panes:
  - name: In Progress
    filter: status = 'in_progress'
sort: Priority, UpdatedAt DESC
view: compact
`
	if err := os.WriteFile(pluginFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write plugin file: %v", err)
	}

	ref := PluginRef{
		File: pluginFile, // Use absolute path
	}

	def, err := loadPluginFromRef(ref)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	tp, ok := def.(*TikiPlugin)
	if !ok {
		t.Fatalf("Expected TikiPlugin, got %T", def)
	}

	if tp.Name != "Test Plugin" {
		t.Errorf("Expected name 'Test Plugin', got '%s'", tp.Name)
	}

	if tp.Rune != 'T' {
		t.Errorf("Expected rune 'T', got '%c'", tp.Rune)
	}

	if tp.ViewMode != "compact" {
		t.Errorf("Expected view mode 'compact', got '%s'", tp.ViewMode)
	}
}

func TestLoadPluginFromRef_Hybrid(t *testing.T) {
	// Create temp plugin file with base config
	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "base-plugin.yaml")
	content := `name: Base Plugin
foreground: "#ff0000"
background: "#0000ff"
key: L
panes:
  - name: Todo
    filter: status = 'ready'
sort: Priority
view: compact
`
	if err := os.WriteFile(pluginFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write plugin file: %v", err)
	}

	// Override view and key
	ref := PluginRef{
		File: pluginFile, // Use absolute path
		View: "expanded",
		Key:  "H",
	}

	def, err := loadPluginFromRef(ref)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	tp, ok := def.(*TikiPlugin)
	if !ok {
		t.Fatalf("Expected TikiPlugin, got %T", def)
	}

	// Base fields should be from file
	if tp.Name != "Base Plugin" {
		t.Errorf("Expected name 'Base Plugin', got '%s'", tp.Name)
	}

	if len(tp.Panes) != 1 || tp.Panes[0].Filter == nil {
		t.Error("Expected pane filter from file")
	}

	// Overridden fields should be from inline
	if tp.Rune != 'H' {
		t.Errorf("Expected rune 'H' (overridden), got '%c'", tp.Rune)
	}

	if tp.ViewMode != "expanded" {
		t.Errorf("Expected view mode 'expanded' (overridden), got '%s'", tp.ViewMode)
	}
}

func TestLoadPluginFromRef_HybridMultipleOverrides(t *testing.T) {
	// Create temp plugin file
	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "multi-plugin.yaml")
	content := `name: Multi Plugin
foreground: "#ffffff"
background: "#000000"
key: M
panes:
  - name: Todo
    filter: status = 'ready'
sort: Priority
view: compact
`
	if err := os.WriteFile(pluginFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write plugin file: %v", err)
	}

	// Override multiple fields
	ref := PluginRef{
		File: pluginFile, // Use absolute path
		Key:  "X",
		Panes: []PluginPaneConfig{
			{Name: "In Progress", Filter: "status = 'in_progress'"},
		},
		Sort:       "UpdatedAt DESC",
		View:       "expanded",
		Foreground: "#00ff00",
	}

	def, err := loadPluginFromRef(ref)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	tp, ok := def.(*TikiPlugin)
	if !ok {
		t.Fatalf("Expected TikiPlugin, got %T", def)
	}

	// Check overridden values
	if tp.Rune != 'X' {
		t.Errorf("Expected rune 'X', got '%c'", tp.Rune)
	}

	if tp.ViewMode != "expanded" {
		t.Errorf("Expected view 'expanded', got '%s'", tp.ViewMode)
	}

	// Verify filter override
	task := &taskpkg.Task{
		ID:     "TIKI-1",
		Status: taskpkg.StatusInProgress,
	}
	if len(tp.Panes) != 1 || tp.Panes[0].Filter == nil {
		t.Fatal("Expected overridden pane filter")
	}
	if !tp.Panes[0].Filter.Evaluate(task, time.Now(), "testuser") {
		t.Error("Expected overridden filter to match in_progress task")
	}

	todoTask := &taskpkg.Task{
		ID:     "TIKI-2",
		Status: taskpkg.StatusReady,
	}
	if tp.Panes[0].Filter.Evaluate(todoTask, time.Now(), "testuser") {
		t.Error("Expected overridden filter to NOT match todo task")
	}
}

func TestLoadPluginFromRef_MissingFile(t *testing.T) {
	ref := PluginRef{
		File: "nonexistent.yaml",
	}

	_, err := loadPluginFromRef(ref)
	if err == nil {
		t.Fatal("Expected error for missing file")
	}

	if err.Error() != "plugin file not found: nonexistent.yaml" {
		t.Errorf("Expected 'file not found' error, got: %v", err)
	}
}

func TestLoadPluginFromRef_NoName(t *testing.T) {
	// Inline plugin without name
	ref := PluginRef{
		Panes: []PluginPaneConfig{
			{Name: "Todo", Filter: "status = 'ready'"},
		},
	}

	_, err := loadPluginFromRef(ref)
	if err == nil {
		t.Fatal("Expected error for plugin without name")
	}

	if err.Error() != "plugin must have a name" {
		t.Errorf("Expected 'must have a name' error, got: %v", err)
	}
}

// Tests for merger functions moved to merger_test.go

func TestPluginTypeExplicit(t *testing.T) {
	// 1. Inline plugin with type doki
	ref := PluginRef{
		Name:    "Type Doki Test",
		Type:    "doki",
		Fetcher: "internal",
		Text:    "some text",
	}

	def, err := loadPluginFromRef(ref)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if def.GetType() != "doki" {
		t.Errorf("Expected type 'doki', got '%s'", def.GetType())
	}

	if _, ok := def.(*DokiPlugin); !ok {
		t.Errorf("Expected DokiPlugin type assertion to succeed")
	}

	// 2. File-based plugin with type doki
	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "type-doki.yaml")
	content := `name: File Type Doki
type: doki
fetcher: file
url: http://example.com/resource
`
	if err := os.WriteFile(pluginFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write plugin file: %v", err)
	}

	refFile := PluginRef{
		File: pluginFile, // Use absolute path
	}

	defFile, err := loadPluginFromRef(refFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if defFile.GetType() != "doki" {
		t.Errorf("Expected type 'doki' for file plugin, got '%s'", defFile.GetType())
	}
}

func TestPluginTypeOverride(t *testing.T) {
	// File specifies tiki, override specifies doki
	// This scenario tests if we can override an embedded/file plugin type.
	// Current mergePluginDefinitions only merges Tiki->Tiki.
	// If types mismatch, it returns the override.

	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "type-override.yaml")
	content := `name: Type Override
type: tiki
`
	if err := os.WriteFile(pluginFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write plugin file: %v", err)
	}

	ref := PluginRef{
		File:    pluginFile, // Use absolute path
		Type:    "doki",
		Fetcher: "internal",
		Text:    "override text",
	}

	// loadPluginFromRef calls mergePluginConfigs but NOT mergePluginDefinitions.
	// mergePluginConfigs updates the config struct.
	// parsePluginConfig then creates the struct.
	// So this test checks mergePluginConfigs logic + parsing logic.

	def, err := loadPluginFromRef(ref)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if def.GetType() != "doki" {
		t.Errorf("Expected type 'doki' (overridden), got '%s'", def.GetType())
	}

	if _, ok := def.(*DokiPlugin); !ok {
		t.Errorf("Expected DokiPlugin type assertion to succeed")
	}
}
