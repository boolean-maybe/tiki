package view

import (
	_ "embed"
	"fmt"
	"log/slog"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/view/renderer"

	"github.com/boolean-maybe/navidown/loaders"
	nav "github.com/boolean-maybe/navidown/navidown"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

//go:embed help/help.md
var helpMd string

//go:embed help/tiki.md
var tikiMd string

//go:embed help/custom.md
var customMd string

// DokiView renders a documentation plugin (navigable markdown)
type DokiView struct {
	root      *tview.Flex
	titleBar  tview.Primitive
	markdown  *NavigableMarkdown
	pluginDef *plugin.DokiPlugin
	registry  *controller.ActionRegistry
	renderer  renderer.MarkdownRenderer
}

// NewDokiView creates a doki view
func NewDokiView(
	pluginDef *plugin.DokiPlugin,
	mdRenderer renderer.MarkdownRenderer,
) *DokiView {
	dv := &DokiView{
		pluginDef: pluginDef,
		registry:  controller.NewActionRegistry(),
		renderer:  mdRenderer,
	}

	dv.build()
	return dv
}

func (dv *DokiView) build() {
	// title bar with gradient background using plugin color
	textColor := tcell.ColorDefault
	if dv.pluginDef.Foreground != tcell.ColorDefault {
		textColor = dv.pluginDef.Foreground
	}
	dv.titleBar = NewGradientCaptionRow([]string{dv.pluginDef.Name}, dv.pluginDef.Background, textColor)

	// Fetch initial content and create NavigableMarkdown with appropriate provider
	var content string
	var sourcePath string
	var err error

	switch dv.pluginDef.Fetcher {
	case "file":
		searchRoots := []string{config.GetDokiDir()}
		provider := &loaders.FileHTTP{SearchRoots: searchRoots}

		content, err = provider.FetchContent(nav.NavElement{URL: dv.pluginDef.URL})

		// Resolve initial source path for stable relative navigation
		sourcePath, _ = nav.ResolveMarkdownPath(dv.pluginDef.URL, "", searchRoots)
		if sourcePath == "" {
			sourcePath = dv.pluginDef.URL
		}

		dv.markdown = NewNavigableMarkdown(NavigableMarkdownConfig{
			Provider:      provider,
			SearchRoots:   searchRoots,
			OnStateChange: dv.UpdateNavigationActions,
		})

	case "internal":
		cnt := map[string]string{
			"Help":      helpMd,
			"tiki":      tikiMd,
			"customize": customMd,
		}
		provider := &internalDokiProvider{content: cnt}
		content, err = provider.FetchContent(nav.NavElement{Text: dv.pluginDef.Text})

		dv.markdown = NewNavigableMarkdown(NavigableMarkdownConfig{
			Provider:      provider,
			OnStateChange: dv.UpdateNavigationActions,
		})

	default:
		content = "Error: Unknown fetcher type"
		dv.markdown = NewNavigableMarkdown(NavigableMarkdownConfig{
			OnStateChange: dv.UpdateNavigationActions,
		})
	}

	if err != nil {
		slog.Error("failed to fetch doki content", "plugin", dv.pluginDef.Name, "error", err)
		content = fmt.Sprintf("Error loading content: %v", err)
	}

	// Display initial content (don't push to history - this is the first page)
	if sourcePath != "" {
		dv.markdown.SetMarkdownWithSource(content, sourcePath, false)
	} else {
		dv.markdown.SetMarkdown(content)
	}

	// root layout
	dv.root = tview.NewFlex().SetDirection(tview.FlexRow)
	dv.rebuildLayout()
}

func (dv *DokiView) rebuildLayout() {
	dv.root.Clear()
	dv.root.AddItem(dv.titleBar, 1, 0, false)
	dv.root.AddItem(dv.markdown.Viewer(), 0, 1, true)
}

func (dv *DokiView) GetPrimitive() tview.Primitive {
	return dv.root
}

func (dv *DokiView) GetActionRegistry() *controller.ActionRegistry {
	return dv.registry
}

func (dv *DokiView) GetViewID() model.ViewID {
	return model.MakePluginViewID(dv.pluginDef.Name)
}

func (dv *DokiView) OnFocus() {
	// Focus behavior
}

func (dv *DokiView) OnBlur() {
	// No cleanup needed yet
}

// UpdateNavigationActions updates the registry to reflect current navigation state
func (dv *DokiView) UpdateNavigationActions() {
	// Clear and rebuild the registry
	dv.registry = controller.NewActionRegistry()

	// Always show Tab/Shift+Tab for link navigation
	dv.registry.Register(controller.Action{
		ID:           "navigate_next_link",
		Key:          tcell.KeyTab,
		Label:        "Next Link",
		ShowInHeader: true,
	})
	dv.registry.Register(controller.Action{
		ID:           "navigate_prev_link",
		Key:          tcell.KeyBacktab,
		Label:        "Prev Link",
		ShowInHeader: true,
	})

	// Add back action if available
	// Note: navidown supports both plain Left/Right and Alt+Left/Right for navigation
	// We register plain arrows since they're simpler and work in all terminals
	if dv.markdown.CanGoBack() {
		dv.registry.Register(controller.Action{
			ID:           controller.ActionNavigateBack,
			Key:          tcell.KeyLeft,
			Label:        "← Back",
			ShowInHeader: true,
		})
	}

	// Add forward action if available
	if dv.markdown.CanGoForward() {
		dv.registry.Register(controller.Action{
			ID:           controller.ActionNavigateForward,
			Key:          tcell.KeyRight,
			Label:        "Forward →",
			ShowInHeader: true,
		})
	}
}

// internalDokiProvider implements navidown.ContentProvider for embedded/internal docs.
// It treats elem.URL as the lookup key, falling back to elem.Text for initial loads.
type internalDokiProvider struct {
	content map[string]string
}

func (p *internalDokiProvider) FetchContent(elem nav.NavElement) (string, error) {
	if p == nil {
		return "", nil
	}
	// Internal docs use text as the key, never URL
	return p.content[elem.Text], nil
}
