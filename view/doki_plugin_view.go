package view

import (
	"fmt"
	"log/slog"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/view/markdown"

	"github.com/boolean-maybe/navidown/loaders"
	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DokiView renders a documentation plugin (navigable markdown).
// surfacedGlobals carries the workflow-level global actions (both
// `kind: view` and `kind: ruki`) so the header and action palette (which
// read this view's registry) show them alongside the built-in navigation
// actions. Keyboard dispatch for both kinds is routed by DokiController.
// taskStore is used to resolve `[[ID]]` wikilinks against the document
// store and (for `kind: detail`) to render the selected document's
// markdown body.
type DokiView struct {
	root                *tview.Flex
	titleBar            tview.Primitive
	md                  *markdown.NavigableMarkdown
	pluginDef           *plugin.DokiPlugin
	registry            *controller.ActionRegistry
	imageManager        *navtview.ImageManager
	mermaidOpts         *nav.MermaidOptions
	taskStore           store.ReadStore
	surfacedGlobals     []plugin.PluginAction
	selectedTaskID      string // 6B.2: for kind: detail, the document to render
	actionChangeHandler func()
}

// NewDokiView creates a doki view. globalActions is the workflow's top-level
// actions list; both `kind: view` and `kind: ruki` entries are surfaced in
// the view registry (DokiController routes keyboard dispatch for both).
// taskStore may be nil for unit tests that don't exercise wikilink
// resolution. selectedTaskID is the document id to render for `kind: detail`
// views (ignored for wiki); pass empty to get the "no document selected"
// placeholder.
func NewDokiView(
	pluginDef *plugin.DokiPlugin,
	imageManager *navtview.ImageManager,
	mermaidOpts *nav.MermaidOptions,
	globalActions []plugin.PluginAction,
	taskStore store.ReadStore,
	selectedTaskID string,
) *DokiView {
	dv := &DokiView{
		pluginDef:       pluginDef,
		registry:        controller.NewActionRegistry(),
		imageManager:    imageManager,
		mermaidOpts:     mermaidOpts,
		taskStore:       taskStore,
		surfacedGlobals: surfacedGlobalActions(globalActions, pluginDef.GetName()),
		selectedTaskID:  selectedTaskID,
	}

	dv.build()
	return dv
}

// surfacedGlobalActions returns the global actions the view should surface
// in its registry (header + palette). Mirrors the filtering DokiController
// applies to its own registry so UI and keyboard dispatch agree:
//
//   - view-kind actions surface unconditionally, except a global pointing
//     at the host view itself (would no-op or recurse on activation).
//   - ruki-kind actions surface ONLY when non-interactive. `input:` and
//     `choose()` actions need the input/choose pipeline which DokiController
//     does not implement; showing them would lie about what pressing the
//     key would do (the controller refuses to fire them).
//
// Future work (phase6.md): implement GetActionInputSpec / GetActionChooseSpec
// on DokiController so interactive ruki globals can fire from non-board views
// too, at which point this filter can widen.
func surfacedGlobalActions(globals []plugin.PluginAction, hostViewName string) []plugin.PluginAction {
	if len(globals) == 0 {
		return nil
	}
	out := make([]plugin.PluginAction, 0, len(globals))
	for _, ga := range globals {
		switch ga.Kind {
		case plugin.ActionKindView:
			if ga.TargetView == hostViewName {
				continue
			}
			out = append(out, ga)
		case plugin.ActionKindRuki:
			if ga.HasInput || ga.HasChoose {
				continue
			}
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
	// Wiki views render the file at `path:`; `document: <ID>` binding is
	// rejected at parse time. Detail views render the task whose id was
	// carried in via PluginViewParams (6B.3), or a placeholder when no
	// selection was passed.
	var content string
	var sourcePath string
	var err error

	// legacy doki root stays first so older configs still resolve; the
	// unified `.doc/` root handles documents anywhere under the new layout.
	searchRoots := []string{config.GetDokiDir(), config.GetDocDir()}
	fileProvider := &loaders.FileHTTP{SearchRoots: searchRoots}

	// Wrap the file provider in a wikilink rewriter so `[[ID]]` spans inside
	// the fetched markdown resolve through the document store. The wrapper
	// applies to every FetchContent call — initial load and every navigated
	// click — so all markdown reaching the renderer has been processed.
	resolver := &markdown.StoreResolver{Store: dv.taskStore}
	provider := markdown.NewWikilinkProvider(fileProvider, resolver)

	switch {
	case dv.pluginDef.DocumentPath != "":
		target := dv.pluginDef.DocumentPath
		content, err = provider.FetchContent(nav.NavElement{URL: target})
		sourcePath, _ = nav.ResolveMarkdownPath(target, "", searchRoots)
		if sourcePath == "" {
			sourcePath = target
		}
	case dv.pluginDef.GetKind() == plugin.KindDetail && dv.selectedTaskID != "" && dv.taskStore != nil:
		// 6B.2: render the selected document. The provider's bare-id
		// lookup handles formatting the task body into markdown; wikilinks
		// inside are resolved by the same WikilinkProvider wrapper.
		content, err = provider.FetchContent(nav.NavElement{URL: dv.selectedTaskID})
		if path := dv.taskStore.PathForID(dv.selectedTaskID); path != "" {
			sourcePath = path
		}
	default:
		// kind: detail with no selection, or a kind without a content source
		content = "(no document selected)"
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

// GetSelectedID implements controller.SelectableView. Returns the task id
// this view was navigated to with (for kind: detail) or the empty string
// when the view has no carried selection (kind: wiki on its own, or a
// detail view opened without a source selection). Load-bearing for the
// InputRouter enablement gate: actions with `require: ["selection:one"]`
// read this to decide whether to dispatch from a doki view.
func (dv *DokiView) GetSelectedID() string {
	return dv.selectedTaskID
}

// SetSelectedID lets the harness inject a selection after construction.
// Doki views do not expose an in-view selection gesture today — selection
// arrives via PluginViewParams at navigation time — so this is primarily a
// contract fill for SelectableView.
func (dv *DokiView) SetSelectedID(id string) {
	dv.selectedTaskID = id
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
	for _, ga := range dv.surfacedGlobals {
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
