package plugin

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/boolean-maybe/tiki/config"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/ruki"
)

// mustParseFilter is a test helper that parses a ruki select statement or panics.
func mustParseFilter(t *testing.T, expr string) *ruki.ValidatedStatement {
	t.Helper()
	schema := rukiRuntime.NewSchema()
	parser := ruki.NewParser(schema)
	stmt, err := parser.ParseAndValidateStatement(expr, ruki.ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("failed to parse ruki statement %q: %v", expr, err)
	}
	return stmt
}

func TestMergePluginDefinitions_TikiToTiki(t *testing.T) {
	baseFilter := mustParseFilter(t, `select where status = "ready"`)

	base := &TikiPlugin{
		BasePlugin: BasePlugin{
			Name:       "Base",
			Key:        tcell.KeyRune,
			Rune:       'B',
			Modifier:   0,
			Foreground: config.NewColor(tcell.ColorRed),
			Background: config.NewColor(tcell.ColorBlue),
			Type:       "tiki",
		},
		Lanes: []TikiLane{
			{Name: "Todo", Columns: 1, Filter: baseFilter},
		},
		ViewMode: "compact",
	}

	overrideFilter := mustParseFilter(t, `select where type = "bug"`)
	override := &TikiPlugin{
		BasePlugin: BasePlugin{
			Name:        "Base",
			Key:         tcell.KeyRune,
			Rune:        'O',
			Modifier:    tcell.ModAlt,
			Foreground:  config.NewColor(tcell.ColorGreen),
			Background:  config.DefaultColor(),
			FilePath:    "override.yaml",
			ConfigIndex: 1,
			Type:        "tiki",
		},
		Lanes: []TikiLane{
			{Name: "Bugs", Columns: 1, Filter: overrideFilter},
		},
		ViewMode: "expanded",
	}

	result := mergePluginDefinitions(base, override)
	resultTiki, ok := result.(*TikiPlugin)
	if !ok {
		t.Fatal("expected result to be *TikiPlugin")
	}

	// check overridden values
	if resultTiki.Rune != 'O' {
		t.Errorf("expected rune 'O', got %q", resultTiki.Rune)
	}
	if resultTiki.Modifier != tcell.ModAlt {
		t.Errorf("expected ModAlt, got %v", resultTiki.Modifier)
	}
	if resultTiki.Foreground.TCell() != tcell.ColorGreen {
		t.Errorf("expected green foreground, got %v", resultTiki.Foreground)
	}
	if resultTiki.ViewMode != "expanded" {
		t.Errorf("expected expanded view, got %q", resultTiki.ViewMode)
	}
	if len(resultTiki.Lanes) != 1 || resultTiki.Lanes[0].Filter == nil {
		t.Error("expected lane filter to be overridden")
	}
}

func TestMergePluginDefinitions_PreservesModifier(t *testing.T) {
	baseFilter := mustParseFilter(t, `select where status = "ready"`)

	base := &TikiPlugin{
		BasePlugin: BasePlugin{
			Name:       "Base",
			Key:        tcell.KeyRune,
			Rune:       'M',
			Modifier:   tcell.ModAlt, // this should be preserved
			Foreground: config.NewColor(tcell.ColorWhite),
			Background: config.DefaultColor(),
			Type:       "tiki",
		},
		Lanes: []TikiLane{
			{Name: "Todo", Columns: 1, Filter: baseFilter},
		},
	}

	// override with no modifier change (Modifier: 0)
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
		t.Fatal("expected result to be *TikiPlugin")
	}

	// the Modifier from base should be preserved
	if resultTiki.Modifier != tcell.ModAlt {
		t.Errorf("expected ModAlt to be preserved from base, got %v", resultTiki.Modifier)
	}
	if resultTiki.Rune != 'M' {
		t.Errorf("expected rune 'M' to be preserved from base, got %q", resultTiki.Rune)
	}
}
