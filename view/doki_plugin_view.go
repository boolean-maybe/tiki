package view

import (
	_ "embed"
	"fmt"
	"log/slog"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/view/markdown"

	"github.com/boolean-maybe/navidown/loaders"
	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
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
	root                *tview.Flex
	titleBar            tview.Primitive
	md                  *markdown.NavigableMarkdown
	pluginDef           *plugin.DokiPlugin
	registry            *controller.ActionRegistry
	imageManager        *navtview.ImageManager
	mermaidOpts         *nav.MermaidOptions
	actionChangeHandler func()
}

// NewDokiView creates a doki view
func NewDokiView(
	pluginDef *plugin.DokiPlugin,
	imageManager *navtview.ImageManager,
	mermaidOpts *nav.MermaidOptions,
) *DokiView {
	dv := &DokiView{
		pluginDef:    pluginDef,
		registry:     controller.NewActionRegistry(),
		imageManager: imageManager,
		mermaidOpts:  mermaidOpts,
	}

	dv.build()
	return dv
}

func (dv *DokiView) build() {
	// title bar with gradient background using theme-derived caption colors
	colors := config.GetColors()
	pair := colors.CaptionColorForIndex(dv.pluginDef.ConfigIndex)
	bgColor := pair.Background
	textColor := pair.Foreground
	if dv.pluginDef.ConfigIndex < 0 {
		bgColor = dv.pluginDef.Background
		textColor = config.DefaultColor()
	}
	dv.titleBar = NewGradientCaptionRow([]string{dv.pluginDef.Name}, nil, bgColor, textColor)

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

		dv.md = markdown.NewNavigableMarkdown(markdown.NavigableMarkdownConfig{
			Provider:       provider,
			SearchRoots:    searchRoots,
			OnStateChange:  dv.UpdateNavigationActions,
			ImageManager:   dv.imageManager,
			MermaidOptions: dv.mermaidOpts,
		})

	case "internal":
		cnt := map[string]string{
			"Help":    helpMd,
			"tiki.md": tikiMd,
			"view.md": customMd,
		}
		provider := &internalDokiProvider{content: cnt}
		content, err = provider.FetchContent(nav.NavElement{Text: dv.pluginDef.Text})

		dv.md = markdown.NewNavigableMarkdown(markdown.NavigableMarkdownConfig{
			Provider:       provider,
			OnStateChange:  dv.UpdateNavigationActions,
			ImageManager:   dv.imageManager,
			MermaidOptions: dv.mermaidOpts,
		})

	default:
		content = "Error: Unknown fetcher type"
		dv.md = markdown.NewNavigableMarkdown(markdown.NavigableMarkdownConfig{
			OnStateChange:  dv.UpdateNavigationActions,
			ImageManager:   dv.imageManager,
			MermaidOptions: dv.mermaidOpts,
		})
	}

	if err != nil {
		slog.Error("failed to fetch doki content", "plugin", dv.pluginDef.Name, "error", err)
		content = fmt.Sprintf("Error loading content: %v", err)
	}

	// Display initial content (don't push to history - this is the first page)
	if sourcePath != "" {
		dv.md.SetMarkdownWithSource(content, sourcePath, false)
	} else {
		dv.md.SetMarkdown(content)
	}

	// root layout
	dv.root = tview.NewFlex().SetDirection(tview.FlexRow)
	dv.rebuildLayout()
}

func (dv *DokiView) rebuildLayout() {
	dv.root.Clear()
	dv.root.AddItem(dv.titleBar, 1, 0, false)
	dv.root.AddItem(dv.md.Viewer(), 0, 1, true)
}

// ShowNavigation returns true — doki views always show plugin navigation keys.
func (dv *DokiView) ShowNavigation() bool { return true }

// GetViewName returns the plugin name for the header info section
func (dv *DokiView) GetViewName() string { return dv.pluginDef.GetName() }

// GetViewDescription returns the plugin description for the header info section
func (dv *DokiView) GetViewDescription() string { return dv.pluginDef.GetDescription() }

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
	if dv.md != nil {
		dv.md.Close()
	}
}

func (dv *DokiView) SetActionChangeHandler(handler func()) {
	dv.actionChangeHandler = handler
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
	if dv.md.CanGoBack() {
		dv.registry.Register(controller.Action{
			ID:           controller.ActionNavigateBack,
			Key:          tcell.KeyLeft,
			Label:        "← Back",
			ShowInHeader: true,
		})
	}

	// Add forward action if available
	if dv.md.CanGoForward() {
		dv.registry.Register(controller.Action{
			ID:           controller.ActionNavigateForward,
			Key:          tcell.KeyRight,
			Label:        "Forward →",
			ShowInHeader: true,
		})
	}

	if dv.actionChangeHandler != nil {
		dv.actionChangeHandler()
	}
}

// HandlePaletteAction maps palette-dispatched actions to the markdown viewer's
// existing key-driven behavior by replaying synthetic key events.
func (dv *DokiView) HandlePaletteAction(id controller.ActionID) bool {
	if dv.md == nil {
		return false
	}
	var event *tcell.EventKey
	switch id {
	case "navigate_next_link":
		event = tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	case "navigate_prev_link":
		event = tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
	case controller.ActionNavigateBack:
		event = tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
	case controller.ActionNavigateForward:
		event = tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)
	default:
		return false
	}
	handler := dv.md.Viewer().InputHandler()
	if handler != nil {
		handler(event, nil)
		return true
	}
	return false
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
	// Use URL for link navigation, Text for initial load
	key := elem.URL
	if key == "" {
		key = elem.Text
	}
	return p.content[key], nil
}
