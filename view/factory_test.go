package view

import (
	"testing"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
)

// newWikiTestFactory builds a ViewFactory with a single registered wiki
// plugin "wikitest" whose static DocumentPath is "static.md".
func newWikiTestFactory(t *testing.T) *ViewFactory {
	t.Helper()
	f := NewViewFactory(store.NewInMemoryStore())
	def := &plugin.WikiPlugin{
		BasePlugin:   plugin.BasePlugin{Name: "wikitest", Kind: plugin.KindWiki},
		DocumentPath: "static.md",
	}
	f.SetPlugins(
		map[string]*model.PluginConfig{},
		map[string]plugin.Plugin{"wikitest": def},
		nil,
		nil,
	)
	return f
}

func TestCreateView_WikiParamPathOverridesDef(t *testing.T) {
	f := newWikiTestFactory(t)

	params := model.EncodePluginViewParams(model.PluginViewParams{DocumentPath: "chosen/file.md"})
	v := f.CreateView(model.MakePluginViewID("wikitest"), params)
	wv, ok := v.(*WikiView)
	if !ok {
		t.Fatalf("CreateView returned %T, want *WikiView", v)
	}
	if got := wv.DocumentPath(); got != "chosen/file.md" {
		t.Fatalf("effective DocumentPath = %q, want chosen/file.md", got)
	}
}

func TestWikiView_MarkdownViewerTitleIsFileName(t *testing.T) {
	f := NewViewFactory(store.NewInMemoryStore())
	def := &plugin.WikiPlugin{
		BasePlugin: plugin.BasePlugin{Name: controller.MarkdownFileViewerPlugin, Kind: plugin.KindWiki},
	}
	f.SetPlugins(
		map[string]*model.PluginConfig{},
		map[string]plugin.Plugin{controller.MarkdownFileViewerPlugin: def},
		nil,
		nil,
	)

	params := model.EncodePluginViewParams(model.PluginViewParams{DocumentPath: "api/rest-api.md"})
	v := f.CreateView(model.MakePluginViewID(controller.MarkdownFileViewerPlugin), params)
	wv, ok := v.(*WikiView)
	if !ok {
		t.Fatalf("CreateView returned %T, want *WikiView", v)
	}
	if got := wv.GetViewName(); got != "rest-api.md" {
		t.Fatalf("GetViewName = %q, want rest-api.md (base of DocumentPath, not the internal plugin id)", got)
	}
}

func TestCreateView_WikiNoParamUsesDefPath(t *testing.T) {
	f := newWikiTestFactory(t)

	v := f.CreateView(model.MakePluginViewID("wikitest"), nil)
	wv, ok := v.(*WikiView)
	if !ok {
		t.Fatalf("CreateView returned %T, want *WikiView", v)
	}
	if got := wv.DocumentPath(); got != "static.md" {
		t.Fatalf("effective DocumentPath = %q, want static.md (def path)", got)
	}
}
