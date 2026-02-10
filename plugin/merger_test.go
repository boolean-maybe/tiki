package plugin

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/boolean-maybe/tiki/plugin/filter"
)

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
		Lanes: []TikiLane{
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
		Lanes: []TikiLane{
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
	if len(resultTiki.Lanes) != 1 || resultTiki.Lanes[0].Filter == nil {
		t.Error("Expected lane filter to be overridden")
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
		Lanes: []TikiLane{
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
