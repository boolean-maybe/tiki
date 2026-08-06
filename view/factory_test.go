package view

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
)

// newMarkdownViewer constructs the reserved Ctrl-O markdown-file viewer bound to
// docPath via the factory, mirroring the runtime PushView path.
func newMarkdownViewer(t *testing.T, docPath string) *WikiView {
	t.Helper()
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
	params := model.EncodePluginViewParams(model.PluginViewParams{DocumentPath: docPath})
	v := f.CreateView(model.MakePluginViewID(controller.MarkdownFileViewerPlugin), params)
	wv, ok := v.(*WikiView)
	if !ok {
		t.Fatalf("CreateView returned %T, want *WikiView", v)
	}
	return wv
}

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

// A missing/untitled document falls back to the base filename, never the
// internal plugin id.
func TestWikiView_MarkdownViewerTitleFallsBackToFileName(t *testing.T) {
	wv := newMarkdownViewer(t, "api/rest-api.md") // no such file → empty-state → filename
	if got := wv.GetViewName(); got != "rest-api.md" {
		t.Fatalf("GetViewName = %q, want rest-api.md (base of DocumentPath, not the internal plugin id)", got)
	}
}

// Reproduction for the reported bug: opening a tiki with a frontmatter title
// (and empty body) via Ctrl-O showed the filename in both caption and header,
// never the title. Both the caption (displayTitle) and the header
// (GetViewName) must surface the frontmatter title.
func TestWikiView_MarkdownViewerTitleFromFrontmatter(t *testing.T) {
	dir := t.TempDir()
	const wantTitle = "When done with customizing detail and task box remove has() from workflows"
	body := "---\nid: 18RS7D\ntitle: " + wantTitle + "\ntype: story\nstatus: ready\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "18RS7D.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir) // config.GetDocDir() resolves to cwd; must precede view construction
	config.ResetPathManager()

	wv := newMarkdownViewer(t, "18RS7D.md")

	if got := wv.GetViewName(); got != wantTitle {
		t.Errorf("GetViewName (header) = %q, want frontmatter title %q", got, wantTitle)
	}
	if got := wv.displayTitle(); got != wantTitle {
		t.Errorf("displayTitle (caption) = %q, want frontmatter title %q", got, wantTitle)
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
