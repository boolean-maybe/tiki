package view

import (
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

// DokiView renders a documentation plugin (navigable markdown).
// globalViewActions carries the workflow-level `kind: view` actions so the
// header and action palette (which read this view's registry) show them
// alongside the built-in navigation actions.
type DokiView struct {
	root                *tview.Flex
	titleBar            tview.Primitive
	md                  *markdown.NavigableMarkdown
	pluginDef           *plugin.DokiPlugin
	registry            *controller.ActionRegistry
	imageManager        *navtview.ImageManager
	mermaidOpts         *nav.MermaidOptions
	globalViewActions   []plugin.PluginAction
	actionChangeHandler func()
}

// NewDokiView creates a doki view. globalActions is the workflow's top-level
// actions list; only `kind: view` entries are surfaced in the view registry
// (the controller mirrors this for keyboard dispatch).
func NewDokiView(
	pluginDef *plugin.DokiPlugin,
	imageManager *navtview.ImageManager,
	mermaidOpts *nav.MermaidOptions,
	globalActions []plugin.PluginAction,
) *DokiView {
	dv := &DokiView{
		pluginDef:         pluginDef,
		registry:          controller.NewActionRegistry(),
		imageManager:      imageManager,
		mermaidOpts:       mermaidOpts,
		globalViewActions: filterViewKindActions(globalActions),
	}

	dv.build()
	return dv
}

// filterViewKindActions keeps only `kind: view` actions — the ones the view
// registry should surface for non-board views at Phase 6A. `kind: ruki`
// globals are handled only on board views today; surfacing them in doki
// view registries would be misleading because they would not actually fire.
func filterViewKindActions(globals []plugin.PluginAction) []plugin.PluginAction {
	if len(globals) == 0 {
		return nil
	}
	out := make([]plugin.PluginAction, 0, len(globals))
	for _, ga := range globals {
		if ga.Kind == plugin.ActionKindView {
			out = append(out, ga)
		}
	}
	return out
}

// pluginRequirementsToController converts the plugin-layer []string require
// list into the controller-layer []Requirement slice the ActionRegistry and
// enablement pipeline expect. Kept local to avoid pushing a view-layer
// dependency into plugin or controller.
func pluginRequirementsToController(raw []string) []controller.Requirement {
	if len(raw) == 0 {
		return nil
	}
	out := make([]controller.Requirement, 0, len(raw))
	for _, r := range raw {
		out = append(out, controller.Requirement(r))
	}
	return out
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

	// Fetch initial content and create NavigableMarkdown with appropriate provider.
	// Phase 6A supports `path:` only for wiki views; `document:` (ID-based
	// resolution) is rejected at parse time until Phase 6B. Detail views carry
	// no bound document and render a placeholder until selection binding
	// lands in Phase 6B.
	var content string
	var sourcePath string
	var err error

	// legacy doki root stays first so older configs still resolve; the
	// unified `.doc/` root handles documents anywhere under the new layout.
	searchRoots := []string{config.GetDokiDir(), config.GetDocDir()}
	provider := &loaders.FileHTTP{SearchRoots: searchRoots}

	if target := dv.pluginDef.DocumentPath; target != "" {
		content, err = provider.FetchContent(nav.NavElement{URL: target})
		sourcePath, _ = nav.ResolveMarkdownPath(target, "", searchRoots)
		if sourcePath == "" {
			sourcePath = target
		}
	} else {
		// kind: detail — selection-driven rendering lands in 6B.
		content = "(select a document)"
	}

	dv.md = markdown.NewNavigableMarkdown(markdown.NavigableMarkdownConfig{
		Provider:       provider,
		SearchRoots:    searchRoots,
		OnStateChange:  dv.UpdateNavigationActions,
		ImageManager:   dv.imageManager,
		MermaidOptions: dv.mermaidOpts,
	})

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

	// Surface workflow-level `kind: view` actions so the header and action
	// palette show them alongside the built-in navigation actions. Without
	// this, globals would fire on keystroke (via the controller) but stay
	// invisible in every UI affordance that reads this registry.
	//
	// `Require` must be propagated: header/palette enablement and palette
	// dispatch consult the registry entry's Require list to decide whether
	// to grey out / block an action. Dropping it here would let the palette
	// fire a `selection:one` action on a view with no selection, which the
	// controller would then silently refuse.
	for _, ga := range dv.globalViewActions {
		dv.registry.Register(controller.Action{
			ID:           controller.ActionID("plugin_action:" + ga.KeyStr),
			Key:          ga.Key,
			Rune:         ga.Rune,
			Modifier:     ga.Modifier,
			Label:        ga.Label,
			ShowInHeader: ga.ShowInHeader,
			Require:      pluginRequirementsToController(ga.Require),
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
