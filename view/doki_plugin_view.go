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
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	navutil "github.com/boolean-maybe/navidown/util"
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
	root        *tview.Flex
	titleBar    tview.Primitive
	contentView *navtview.TextViewViewer
	pluginDef   *plugin.DokiPlugin
	registry    *controller.ActionRegistry
	renderer    renderer.MarkdownRenderer
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
	titleGradient := pluginCaptionGradient(dv.pluginDef.Background, config.GetColors().BoardPaneTitleGradient)
	dv.titleBar = NewGradientCaptionRow([]string{dv.pluginDef.Name}, titleGradient, textColor)

	// content view (Navigable Markdown)
	dv.contentView = navtview.NewTextView()
	dv.contentView.SetAnsiConverter(navutil.NewAnsiConverter(true))
	dv.contentView.SetRenderer(nav.NewANSIRendererWithStyle(config.GetEffectiveTheme()))
	dv.contentView.SetBackgroundColor(config.GetContentBackgroundColor())

	// Set up state change handler to update navigation actions
	dv.contentView.SetStateChangedHandler(func(_ *navtview.TextViewViewer) {
		dv.UpdateNavigationActions()
	})

	// Fetch initial content using component fetchers
	var content string
	var err error

	switch dv.pluginDef.Fetcher {
	case "file":
		searchRoots := []string{config.GetDokiDir()}
		provider := &loaders.FileHTTP{SearchRoots: searchRoots}

		// Fetch initial content (no source context yet; rely on searchRoots)
		content, err = provider.FetchContent(nav.NavElement{URL: dv.pluginDef.URL})

		// Set up link navigation for file-based docs
		dv.contentView.SetSelectHandler(func(v *navtview.TextViewViewer, elem nav.NavElement) {
			if elem.Type != nav.NavElementURL {
				return
			}
			content, err := provider.FetchContent(elem)
			if err != nil {
				errorContent := "# Error\n\nFailed to load `" + elem.URL + "`:\n\n```\n" + err.Error() + "\n```"
				v.SetMarkdown(errorContent)
				return
			}
			if content == "" {
				return
			}
			// Resolve path for source context
			newSourcePath := elem.URL
			if elem.SourceFilePath != "" {
				resolved, rerr := nav.ResolveMarkdownPath(elem.URL, elem.SourceFilePath, searchRoots)
				if rerr == nil && resolved != "" {
					newSourcePath = resolved
				}
			}
			v.SetMarkdownWithSource(content, newSourcePath, true)
		})

	case "internal":
		cnt := map[string]string{
			"Help":      helpMd,
			"tiki":      tikiMd,
			"customize": customMd,
		}
		provider := &internalDokiProvider{content: cnt}
		content, err = provider.FetchContent(nav.NavElement{Text: dv.pluginDef.Text})

		// Set up link navigation (internal docs use text as source path for history)
		dv.contentView.SetSelectHandler(func(v *navtview.TextViewViewer, elem nav.NavElement) {
			if elem.Type != nav.NavElementURL {
				return
			}
			content, err := provider.FetchContent(elem)
			if err != nil {
				errorContent := "# Error\n\nFailed to load content:\n\n```\n" + err.Error() + "\n```"
				v.SetMarkdown(errorContent)
				return
			}
			if content == "" {
				return
			}
			// Use elem.Text as source path for history tracking
			v.SetMarkdownWithSource(content, elem.Text, true)
		})

	default:
		content = "Error: Unknown fetcher type"
	}

	if err != nil {
		slog.Error("failed to fetch doki content", "plugin", dv.pluginDef.Name, "error", err)
		content = fmt.Sprintf("Error loading content: %v", err)
	}

	// Display initial content with source context (don't push to history - this is the first page)
	if dv.pluginDef.Fetcher == "file" {
		// Try to resolve the initial URL so subsequent relative navigation has a stable source path.
		sourcePath, rerr := nav.ResolveMarkdownPath(dv.pluginDef.URL, "", []string{config.GetDokiDir()})
		if rerr != nil || sourcePath == "" {
			sourcePath = dv.pluginDef.URL
		}
		dv.contentView.SetMarkdownWithSource(content, sourcePath, false)
	} else {
		dv.contentView.SetMarkdown(content)
	}

	// root layout
	dv.root = tview.NewFlex().SetDirection(tview.FlexRow)
	dv.rebuildLayout()
}

func (dv *DokiView) rebuildLayout() {
	dv.root.Clear()
	dv.root.AddItem(dv.titleBar, 1, 0, false)
	dv.root.AddItem(dv.contentView, 0, 1, true)
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
	if dv.contentView.Core().CanGoBack() {
		dv.registry.Register(controller.Action{
			ID:           controller.ActionNavigateBack,
			Key:          tcell.KeyLeft,
			Label:        "← Back",
			ShowInHeader: true,
		})
	}

	// Add forward action if available
	if dv.contentView.Core().CanGoForward() {
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
