package plugin

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/boolean-maybe/tiki/plugin/filter"
)

func TestMergePluginConfigs(t *testing.T) {
	base := pluginFileConfig{
		Name:       "Base",
		Foreground: "#ff0000",
		Background: "#0000ff",
		Key:        "L",
		Panes: []PluginPaneConfig{
			{Name: "Todo", Filter: "status = 'ready'"},
		},
		Sort: "Priority",
		View: "compact",
	}

	overrides := PluginRef{
		View: "expanded",
		Key:  "O",
	}

	result := mergePluginConfigs(base, overrides)

	// Base fields should remain
	if result.Name != "Base" {
		t.Errorf("Expected name 'Base', got '%s'", result.Name)
	}
	if len(result.Panes) != 1 || result.Panes[0].Filter != "status = 'ready'" {
		t.Errorf("Expected panes from base, got %+v", result.Panes)
	}
	if result.Foreground != "#ff0000" {
		t.Errorf("Expected foreground from base, got '%s'", result.Foreground)
	}

	// Overridden fields
	if result.View != "expanded" {
		t.Errorf("Expected view 'expanded', got '%s'", result.View)
	}
	if result.Key != "O" {
		t.Errorf("Expected key 'O', got '%s'", result.Key)
	}
}

func TestMergePluginConfigs_AllOverrides(t *testing.T) {
	base := pluginFileConfig{
		Name:       "Base",
		Foreground: "#ff0000",
		Background: "#0000ff",
		Key:        "L",
		Panes: []PluginPaneConfig{
			{Name: "Todo", Filter: "status = 'ready'"},
		},
		Sort: "Priority",
		View: "compact",
	}

	overrides := PluginRef{
		Name:       "Overridden",
		Foreground: "#00ff00",
		Background: "#000000",
		Key:        "O",
		Panes: []PluginPaneConfig{
			{Name: "Done", Filter: "status = 'done'"},
		},
		Sort: "UpdatedAt DESC",
		View: "expanded",
	}

	result := mergePluginConfigs(base, overrides)

	// All fields should be overridden
	if result.Name != "Overridden" {
		t.Errorf("Expected name 'Overridden', got '%s'", result.Name)
	}
	if result.Foreground != "#00ff00" {
		t.Errorf("Expected foreground '#00ff00', got '%s'", result.Foreground)
	}
	if result.Background != "#000000" {
		t.Errorf("Expected background '#000000', got '%s'", result.Background)
	}
	if result.Key != "O" {
		t.Errorf("Expected key 'O', got '%s'", result.Key)
	}
	if len(result.Panes) != 1 || result.Panes[0].Filter != "status = 'done'" {
		t.Errorf("Expected pane filter 'status = 'done'', got %+v", result.Panes)
	}
	if result.Sort != "UpdatedAt DESC" {
		t.Errorf("Expected sort 'UpdatedAt DESC', got '%s'", result.Sort)
	}
	if result.View != "expanded" {
		t.Errorf("Expected view 'expanded', got '%s'", result.View)
	}
}

func TestValidatePluginRef_FileBased(t *testing.T) {
	ref := PluginRef{
		File: "plugin.yaml",
	}

	err := validatePluginRef(ref)
	if err != nil {
		t.Errorf("Expected no error for file-based plugin, got: %v", err)
	}
}

func TestValidatePluginRef_Hybrid(t *testing.T) {
	ref := PluginRef{
		File: "plugin.yaml",
		View: "expanded",
	}

	err := validatePluginRef(ref)
	if err != nil {
		t.Errorf("Expected no error for hybrid plugin, got: %v", err)
	}
}

func TestValidatePluginRef_InlineValid(t *testing.T) {
	ref := PluginRef{
		Name: "Test",
		Panes: []PluginPaneConfig{
			{Name: "Todo", Filter: "status = 'ready'"},
		},
	}

	err := validatePluginRef(ref)
	if err != nil {
		t.Errorf("Expected no error for valid inline plugin, got: %v", err)
	}
}

func TestValidatePluginRef_InlineNoName(t *testing.T) {
	ref := PluginRef{
		Panes: []PluginPaneConfig{
			{Name: "Todo", Filter: "status = 'ready'"},
		},
	}

	err := validatePluginRef(ref)
	if err == nil {
		t.Fatal("Expected error for inline plugin without name")
	}

	if err.Error() != "inline plugin must specify 'name' field" {
		t.Errorf("Expected 'must specify name' error, got: %v", err)
	}
}

func TestValidatePluginRef_InlineNoContent(t *testing.T) {
	ref := PluginRef{
		Name: "Empty",
	}

	err := validatePluginRef(ref)
	if err == nil {
		t.Fatal("Expected error for inline plugin with no content")
	}

	expected := "inline plugin 'Empty' has no configuration fields"
	if err.Error() != expected {
		t.Errorf("Expected '%s', got: %v", expected, err)
	}
}

func TestMergePluginDefinitions_TikiToTiki(t *testing.T) {
	baseFilter, _ := filter.ParseFilter("status = 'ready'")
	baseSort, _ := ParseSort("Priority")

	base := &TikiPlugin{
		BasePlugin: BasePlugin{
			Name:       "Base",
			Key:        tcell.KeyRune,
			Rune:       'B',
			Modifier:   0,
			Foreground: tcell.ColorRed,
			Background: tcell.ColorBlue,
			Type:       "tiki",
		},
		Panes: []TikiPane{
			{Name: "Todo", Columns: 1, Filter: baseFilter},
		},
		Sort:     baseSort,
		ViewMode: "compact",
	}

	overrideFilter, _ := filter.ParseFilter("type = 'bug'")
	override := &TikiPlugin{
		BasePlugin: BasePlugin{
			Name:        "Base",
			Key:         tcell.KeyRune,
			Rune:        'O',
			Modifier:    tcell.ModAlt,
			Foreground:  tcell.ColorGreen,
			Background:  tcell.ColorDefault,
			FilePath:    "override.yaml",
			ConfigIndex: 1,
			Type:        "tiki",
		},
		Panes: []TikiPane{
			{Name: "Bugs", Columns: 1, Filter: overrideFilter},
		},
		Sort:     nil,
		ViewMode: "expanded",
	}

	result := mergePluginDefinitions(base, override)
	resultTiki, ok := result.(*TikiPlugin)
	if !ok {
		t.Fatal("Expected result to be *TikiPlugin")
	}

	// Check overridden values
	if resultTiki.Rune != 'O' {
		t.Errorf("Expected rune 'O', got %q", resultTiki.Rune)
	}
	if resultTiki.Modifier != tcell.ModAlt {
		t.Errorf("Expected ModAlt, got %v", resultTiki.Modifier)
	}
	if resultTiki.Foreground != tcell.ColorGreen {
		t.Errorf("Expected green foreground, got %v", resultTiki.Foreground)
	}
	if resultTiki.ViewMode != "expanded" {
		t.Errorf("Expected expanded view, got %q", resultTiki.ViewMode)
	}
	if len(resultTiki.Panes) != 1 || resultTiki.Panes[0].Filter == nil {
		t.Error("Expected pane filter to be overridden")
	}

	// Check that base sort is kept when override has nil
	if resultTiki.Sort == nil {
		t.Error("Expected base sort to be retained")
	}
}

func TestMergePluginDefinitions_PreservesModifier(t *testing.T) {
	// This test verifies the bug fix where Modifier was not being copied from base
	baseFilter, _ := filter.ParseFilter("status = 'ready'")

	base := &TikiPlugin{
		BasePlugin: BasePlugin{
			Name:       "Base",
			Key:        tcell.KeyRune,
			Rune:       'M',
			Modifier:   tcell.ModAlt, // This should be preserved
			Foreground: tcell.ColorWhite,
			Background: tcell.ColorDefault,
			Type:       "tiki",
		},
		Panes: []TikiPane{
			{Name: "Todo", Columns: 1, Filter: baseFilter},
		},
	}

	// Override with no modifier change (Modifier: 0)
	override := &TikiPlugin{
		BasePlugin: BasePlugin{
			Name:        "Base",
			FilePath:    "config.yaml",
			ConfigIndex: 0,
			Type:        "tiki",
		},
	}

	result := mergePluginDefinitions(base, override)
	resultTiki, ok := result.(*TikiPlugin)
	if !ok {
		t.Fatal("Expected result to be *TikiPlugin")
	}

	// The Modifier from base should be preserved
	if resultTiki.Modifier != tcell.ModAlt {
		t.Errorf("Expected ModAlt to be preserved from base, got %v", resultTiki.Modifier)
	}
	if resultTiki.Rune != 'M' {
		t.Errorf("Expected rune 'M' to be preserved from base, got %q", resultTiki.Rune)
	}
}
