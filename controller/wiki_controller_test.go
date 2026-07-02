package controller

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
)

// newWikiTestController builds a WikiController with a wired executor (store +
// gate + schema all non-nil) so ruki globals are eligible for surfacing.
func newWikiTestController(t *testing.T, globals []plugin.PluginAction) *WikiController {
	t.Helper()
	pluginDef := &plugin.WikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: "Docs", Kind: plugin.KindWiki},
	}
	statusline := &model.StatuslineConfig{}
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	schema := rukiRuntime.NewSchema()
	return NewWikiController(pluginDef, nil, statusline, globals, tikiStore, gate, schema)
}

func rukiGlobal(t *testing.T, key rune, label, stmt string) plugin.PluginAction {
	t.Helper()
	return plugin.PluginAction{
		Key:          tcell.KeyRune,
		Rune:         key,
		KeyStr:       string(key),
		Label:        label,
		Kind:         plugin.ActionKindRuki,
		Action:       mustParseStmt(t, stmt),
		ShowInHeader: true,
	}
}

// "Copy ID" uses id() — it must not surface on the wiki.
func TestWikiHidesIDBuiltinGlobal(t *testing.T) {
	dc := newWikiTestController(t, []plugin.PluginAction{
		rukiGlobal(t, 'y', "Copy ID", `select id where id = id() | clipboard()`),
	})
	if _, ok := dc.GetActionRegistry().LookupRune('y'); ok {
		t.Error("Copy ID (uses id()) should not surface on the wiki, but it did")
	}
}

// "Copy content" uses filepath() — it must not surface on the wiki either.
func TestWikiHidesFilepathBuiltinGlobal(t *testing.T) {
	dc := newWikiTestController(t, []plugin.PluginAction{
		rukiGlobal(t, 'Y', "Copy content",
			`select title, description where filepath = filepath() | clipboard()`),
	})
	if _, ok := dc.GetActionRegistry().LookupRune('Y'); ok {
		t.Error("Copy content (uses filepath()) should not surface on the wiki, but it did")
	}
}

// A ruki global with no selection builtin still surfaces (guard against over-hiding).
func TestWikiKeepsNonSelectionGlobal(t *testing.T) {
	dc := newWikiTestController(t, []plugin.PluginAction{
		rukiGlobal(t, 'z', "List all", `select id`),
	})
	if _, ok := dc.GetActionRegistry().LookupRune('z'); !ok {
		t.Error("a ruki global with no selection builtin should surface on the wiki, but it did not")
	}
}
