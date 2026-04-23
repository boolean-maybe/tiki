package view

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/view/header"
	"github.com/boolean-maybe/tiki/view/statusline"

	"github.com/rivo/tview"
)

// RootLayout is a container view managing a persistent header, swappable content area, and statusline.
// It observes LayoutModel for content changes, HeaderConfig for header visibility,
// and StatuslineConfig for statusline visibility.
type RootLayout struct {
	root        *tview.Flex
	header      *header.HeaderWidget
	contentArea *tview.Flex

	headerConfig *model.HeaderConfig
	layoutModel  *model.LayoutModel
	viewFactory  controller.ViewFactory
	taskStore    store.Store

	// statusline
	statuslineWidget *statusline.StatuslineWidget
	statuslineConfig *model.StatuslineConfig

	contentView   controller.View
	lastParamsKey string

	viewContext *model.ViewContext

	headerListenerID      int
	layoutListenerID      int
	storeListenerID       int
	statuslineListenerID  int
	lastHeaderVisible     bool
	lastStatuslineVisible bool
	app                   *tview.Application
	onViewActivated       func(controller.View)
}

// RootLayoutOpts groups all parameters for NewRootLayout
type RootLayoutOpts struct {
	Header           *header.HeaderWidget
	HeaderConfig     *model.HeaderConfig
	ViewContext      *model.ViewContext
	LayoutModel      *model.LayoutModel
	ViewFactory      controller.ViewFactory
	TaskStore        store.Store
	App              *tview.Application
	StatuslineWidget *statusline.StatuslineWidget
	StatuslineConfig *model.StatuslineConfig
}

// NewRootLayout creates a root layout that observes models and manages header/content/statusline
func NewRootLayout(opts RootLayoutOpts) *RootLayout {
	rl := &RootLayout{
		root:                  tview.NewFlex().SetDirection(tview.FlexRow),
		header:                opts.Header,
		contentArea:           tview.NewFlex().SetDirection(tview.FlexRow),
		headerConfig:          opts.HeaderConfig,
		viewContext:           opts.ViewContext,
		layoutModel:           opts.LayoutModel,
		viewFactory:           opts.ViewFactory,
		taskStore:             opts.TaskStore,
		statuslineWidget:      opts.StatuslineWidget,
		statuslineConfig:      opts.StatuslineConfig,
		lastHeaderVisible:     opts.HeaderConfig.IsVisible(),
		lastStatuslineVisible: opts.StatuslineConfig.IsVisible(),
		app:                   opts.App,
	}

	// Subscribe to layout model changes (content swapping)
	rl.layoutListenerID = opts.LayoutModel.AddListener(rl.onLayoutChange)

	// Subscribe to header config changes (visibility)
	rl.headerListenerID = opts.HeaderConfig.AddListener(rl.onHeaderConfigChange)

	// Subscribe to statusline config changes (visibility)
	rl.statuslineListenerID = opts.StatuslineConfig.AddListener(rl.onStatuslineConfigChange)

	// Subscribe to task store changes (stats updates)
	if opts.TaskStore != nil {
		rl.storeListenerID = opts.TaskStore.AddListener(rl.onStoreChange)
	}

	// Build initial layout
	rl.rebuildLayout()

	return rl
}

// SetOnViewActivated registers a callback that runs when any view becomes active.
// This is used to wire up focus setters and other view-specific setup.
func (rl *RootLayout) SetOnViewActivated(callback func(controller.View)) {
	rl.onViewActivated = callback
}

// onLayoutChange is called when LayoutModel changes (content view change or Touch)
func (rl *RootLayout) onLayoutChange() {
	viewID := rl.layoutModel.GetContentViewID()
	params := rl.layoutModel.GetContentParams()

	// Check if this is just a Touch (revision changed but not view/params)
	paramsKey, paramsKeyOK := stableParamsKey(params)
	if paramsKeyOK && rl.contentView != nil && rl.contentView.GetViewID() == viewID && paramsKey == rl.lastParamsKey {
		// Touch/update-only: keep the existing view instance, just recompute derived layout (header visibility)
		rl.recomputeHeaderVisibility(rl.contentView)
		return
	}

	// Blur current view if exists
	if rl.contentView != nil {
		rl.contentView.OnBlur()
	}

	// RootLayout creates the view (View layer responsibility)
	newView := rl.viewFactory.CreateView(viewID, params)
	if newView == nil {
		slog.Error("failed to create view", "viewID", viewID)
		return
	}
	if paramsKeyOK {
		rl.lastParamsKey = paramsKey
	} else {
		// If we couldn't fingerprint params (invalid/non-scalar), disable the optimization
		rl.lastParamsKey = ""
	}

	rl.recomputeHeaderVisibility(newView)

	// Swap content
	rl.contentArea.Clear()
	rl.contentArea.AddItem(newView.GetPrimitive(), 0, 1, true)
	rl.contentView = newView

	// Sync view context (writes to both ViewContext and HeaderConfig for header actions)
	rl.syncViewContextFromView(newView)

	// Update statusline stats from the view
	rl.updateStatuslineViewStats(newView)

	// Run view activated callback (for focus setters, etc.)
	if rl.onViewActivated != nil {
		rl.onViewActivated(newView)
	}

	// Wire up fullscreen change notifications
	if notifier, ok := newView.(controller.FullscreenChangeNotifier); ok {
		notifier.SetFullscreenChangeHandler(func(_ bool) {
			rl.recomputeHeaderVisibility(newView)
		})
	}

	// Wire up action change notifications (registry or enablement changes on the same view)
	if notifier, ok := newView.(controller.ActionChangeNotifier); ok {
		notifier.SetActionChangeHandler(func() {
			rl.syncViewContextFromView(newView)
		})
	}

	// Focus the view
	newView.OnFocus()
	if newView.GetViewID() == model.TaskEditViewID {
		// in desc-only mode, focus the description textarea instead of title
		if tagsOnlyView, ok := newView.(interface{ IsTagsOnly() bool }); ok && tagsOnlyView.IsTagsOnly() {
			if tagsView, ok := newView.(controller.TagsEditableView); ok {
				if tags := tagsView.ShowTagsEditor(); tags != nil {
					rl.app.SetFocus(tags)
					return
				}
			}
		}
		if descOnlyView, ok := newView.(interface{ IsDescOnly() bool }); ok && descOnlyView.IsDescOnly() {
			if descView, ok := newView.(controller.DescriptionEditableView); ok {
				if desc := descView.ShowDescriptionEditor(); desc != nil {
					rl.app.SetFocus(desc)
					return
				}
			}
		}
		if titleView, ok := newView.(controller.TitleEditableView); ok {
			if title := titleView.ShowTitleEditor(); title != nil {
				rl.app.SetFocus(title)
				return
			}
		}
	}
	rl.app.SetFocus(newView.GetPrimitive())
}

// syncViewContextFromView computes view name/description, view actions, and plugin actions
// from the active view, then writes to both ViewContext (single atomic update) and HeaderConfig
// (for backward-compatible header display during the transition period).
func (rl *RootLayout) syncViewContextFromView(v controller.View) {
	if v == nil {
		return
	}

	currentView := &controller.ViewEntry{
		ViewID: rl.layoutModel.GetContentViewID(),
		Params: rl.layoutModel.GetContentParams(),
	}
	ctx := controller.BuildAppContext(currentView, v)

	globalActions := controller.DefaultGlobalActions().ToHeaderActionsForContext(ctx)
	viewActions := v.GetActionRegistry().ToHeaderActionsForContext(ctx)

	var pluginActions []model.HeaderAction
	if np, ok := v.(controller.NavigationProvider); ok && np.ShowNavigation() {
		pluginActions = controller.GetPluginActions().ToHeaderActionsForContext(ctx)
	}

	var viewName, viewDesc string
	if vip, ok := v.(controller.ViewInfoProvider); ok {
		viewName = vip.GetViewName()
		viewDesc = vip.GetViewDescription()
	}

	if rl.viewContext != nil {
		rl.viewContext.SetFromView(v.GetViewID(), viewName, viewDesc, globalActions, viewActions, pluginActions)
	}
}

// recomputeHeaderVisibility computes header visibility based on view requirements and user preference
func (rl *RootLayout) recomputeHeaderVisibility(v controller.View) {
	// Start from user preference
	visible := rl.headerConfig.GetUserPreference()

	// Force-hide if view requires header hidden (static requirement)
	if hv, ok := v.(interface{ RequiresHeaderHidden() bool }); ok && hv.RequiresHeaderHidden() {
		visible = false
	}

	// Force-hide if view is currently fullscreen (dynamic state)
	if fv, ok := v.(controller.FullscreenView); ok && fv.IsFullscreen() {
		visible = false
	}

	rl.headerConfig.SetVisible(visible)
}

// onHeaderConfigChange is called when HeaderConfig changes
func (rl *RootLayout) onHeaderConfigChange() {
	currentVisible := rl.headerConfig.IsVisible()
	if currentVisible != rl.lastHeaderVisible {
		rl.lastHeaderVisible = currentVisible
		rl.rebuildLayout()
	}
}

// onStatuslineConfigChange is called when StatuslineConfig changes
func (rl *RootLayout) onStatuslineConfigChange() {
	currentVisible := rl.statuslineConfig.IsVisible()
	if currentVisible != rl.lastStatuslineVisible {
		rl.lastStatuslineVisible = currentVisible
		rl.rebuildLayout()
	}
}

// rebuildLayout rebuilds the root flex layout based on current header/statusline visibility
func (rl *RootLayout) rebuildLayout() {
	rl.root.Clear()

	if rl.headerConfig.IsVisible() {
		rl.root.AddItem(rl.header, header.HeaderHeight, 0, false)
		rl.root.AddItem(tview.NewBox(), 1, 0, false) // spacer
	}

	rl.root.AddItem(rl.contentArea, 0, 1, true)

	if rl.statuslineConfig.IsVisible() {
		rl.root.AddItem(rl.statuslineWidget, 1, 0, false)
	}
}

// GetPrimitive returns the root tview primitive for app.SetRoot()
func (rl *RootLayout) GetPrimitive() tview.Primitive {
	return rl.root
}

// GetActionRegistry delegates to the content view
func (rl *RootLayout) GetActionRegistry() *controller.ActionRegistry {
	if rl.contentView != nil {
		return rl.contentView.GetActionRegistry()
	}
	return controller.NewActionRegistry()
}

// GetViewID delegates to the content view
func (rl *RootLayout) GetViewID() model.ViewID {
	if rl.contentView != nil {
		return rl.contentView.GetViewID()
	}
	return ""
}

// GetContentView returns the current content view
func (rl *RootLayout) GetContentView() controller.View {
	return rl.contentView
}

// OnFocus delegates to the content view
func (rl *RootLayout) OnFocus() {
	if rl.contentView != nil {
		rl.contentView.OnFocus()
	}
}

// OnBlur delegates to the content view
func (rl *RootLayout) OnBlur() {
	if rl.contentView != nil {
		rl.contentView.OnBlur()
	}
}

// Cleanup removes all listeners
func (rl *RootLayout) Cleanup() {
	rl.layoutModel.RemoveListener(rl.layoutListenerID)
	rl.headerConfig.RemoveListener(rl.headerListenerID)
	rl.statuslineConfig.RemoveListener(rl.statuslineListenerID)
	rl.statuslineWidget.Cleanup()
	if rl.taskStore != nil {
		rl.taskStore.RemoveListener(rl.storeListenerID)
	}
}

// onStoreChange is called when the task store changes (task created/updated/deleted)
func (rl *RootLayout) onStoreChange() {
	if rl.contentView != nil {
		rl.updateStatuslineViewStats(rl.contentView)
	}
}

// updateStatuslineViewStats reads stats from the view and updates the statusline right section.
// Reuses StatsProvider — no separate interface needed until header and statusline stats diverge.
func (rl *RootLayout) updateStatuslineViewStats(v controller.View) {
	stats := make(map[string]model.StatValue)
	if sp, ok := v.(controller.StatsProvider); ok {
		for _, stat := range sp.GetStats() {
			stats[stat.Name] = model.StatValue{Value: stat.Value, Priority: stat.Order}
		}
	}
	rl.statuslineConfig.SetRightViewStats(stats)
}

// stableParamsKey produces a deterministic, collision-safe fingerprint for params
func stableParamsKey(params map[string]any) (string, bool) {
	if len(params) == 0 {
		return "", true
	}

	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build tuples of [key, value]
	tuples := make([][2]any, 0, len(keys))
	for _, k := range keys {
		tuples = append(tuples, [2]any{k, stableJSONValue(params[k])})
	}

	b, err := json.Marshal(tuples)
	if err != nil {
		// Do not silently ignore marshal errors: treat them as invalid params and disable caching
		return "", false
	}
	return string(b), true
}

// stableJSONValue converts a value to a stable JSON-encodable representation
func stableJSONValue(v any) any {
	switch x := v.(type) {
	case nil, string, bool, float64:
		return x
	case int:
		return x
	case int64:
		return x
	case uint64:
		// JSON doesn't have uint; encode as string to preserve meaning
		return map[string]string{"type": "uint64", "value": strconv.FormatUint(x, 10)}
	default:
		// Keep params scalar in navigation. For anything else, include a type tag.
		return map[string]string{"type": fmt.Sprintf("%T", v), "value": fmt.Sprintf("%v", v)}
	}
}
