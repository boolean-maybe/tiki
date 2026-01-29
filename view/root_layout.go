package view

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/view/header"

	"github.com/rivo/tview"
)

// RootLayout is a container view managing a persistent header and swappable content area.
// It observes LayoutModel for content changes and HeaderConfig for visibility changes.
type RootLayout struct {
	root        *tview.Flex
	header      *header.HeaderWidget
	contentArea *tview.Flex

	headerConfig *model.HeaderConfig
	layoutModel  *model.LayoutModel
	viewFactory  controller.ViewFactory
	taskStore    store.Store

	contentView   controller.View
	lastParamsKey string

	headerListenerID  int
	layoutListenerID  int
	storeListenerID   int
	lastHeaderVisible bool
	app               *tview.Application
	onViewActivated   func(controller.View)
}

// NewRootLayout creates a root layout that observes models and manages header/content
func NewRootLayout(
	hdr *header.HeaderWidget,
	headerConfig *model.HeaderConfig,
	layoutModel *model.LayoutModel,
	viewFactory controller.ViewFactory,
	taskStore store.Store,
	app *tview.Application,
) *RootLayout {
	rl := &RootLayout{
		root:              tview.NewFlex().SetDirection(tview.FlexRow),
		header:            hdr,
		contentArea:       tview.NewFlex().SetDirection(tview.FlexRow),
		headerConfig:      headerConfig,
		layoutModel:       layoutModel,
		viewFactory:       viewFactory,
		taskStore:         taskStore,
		lastHeaderVisible: headerConfig.IsVisible(),
		app:               app,
	}

	// Subscribe to layout model changes (content swapping)
	rl.layoutListenerID = layoutModel.AddListener(rl.onLayoutChange)

	// Subscribe to header config changes (visibility)
	rl.headerListenerID = headerConfig.AddListener(rl.onHeaderConfigChange)

	// Subscribe to task store changes (stats updates)
	if taskStore != nil {
		rl.storeListenerID = taskStore.AddListener(rl.onStoreChange)
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

	// Update header with new view's actions
	rl.headerConfig.SetViewActions(convertActionRegistry(newView.GetActionRegistry()))

	// Clear plugin actions for non-plugin views (task detail, task edit)
	// Plugin navigation keys only work in plugin views
	if model.IsPluginViewID(viewID) {
		// Restore plugin actions for plugin views
		rl.headerConfig.SetPluginActions(convertActionRegistry(controller.GetPluginActions()))
	} else {
		// Clear plugin actions for task detail/edit views
		rl.headerConfig.SetPluginActions(nil)
	}

	// Apply view-specific stats from the view
	rl.updateViewStats(newView)

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

	// Focus the view
	newView.OnFocus()
	if newView.GetViewID() == model.TaskEditViewID {
		if titleView, ok := newView.(controller.TitleEditableView); ok {
			if title := titleView.ShowTitleEditor(); title != nil {
				rl.app.SetFocus(title)
				return
			}
		}
	}
	rl.app.SetFocus(newView.GetPrimitive())
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

// rebuildLayout rebuilds the root flex layout based on current header visibility
func (rl *RootLayout) rebuildLayout() {
	rl.root.Clear()

	if rl.headerConfig.IsVisible() {
		rl.root.AddItem(rl.header, header.HeaderHeight, 0, false)
		rl.root.AddItem(tview.NewBox(), 1, 0, false) // spacer
	}

	rl.root.AddItem(rl.contentArea, 0, 1, true)
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

// RecomputeHeaderVisibility recomputes header visibility based on current view state.
// Call this when fullscreen state changes on a view not managed by RootLayout.
func (rl *RootLayout) RecomputeHeaderVisibility() {
	if rl.contentView != nil {
		rl.recomputeHeaderVisibility(rl.contentView)
	}
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
	if rl.taskStore != nil {
		rl.taskStore.RemoveListener(rl.storeListenerID)
	}
}

// onStoreChange is called when the task store changes (task created/updated/deleted)
func (rl *RootLayout) onStoreChange() {
	if rl.contentView != nil {
		rl.updateViewStats(rl.contentView)
	}
}

// updateViewStats reads stats from the view and updates the header
func (rl *RootLayout) updateViewStats(v controller.View) {
	rl.headerConfig.ClearViewStats()
	if sp, ok := v.(controller.StatsProvider); ok {
		for _, stat := range sp.GetStats() {
			rl.headerConfig.SetViewStat(stat.Name, stat.Value, stat.Order)
		}
	}
}

// convertActionRegistry converts controller.ActionRegistry to []model.HeaderAction
// This avoids import cycles between model and controller packages.
func convertActionRegistry(registry *controller.ActionRegistry) []model.HeaderAction {
	if registry == nil {
		return nil
	}

	actions := registry.GetHeaderActions()
	result := make([]model.HeaderAction, len(actions))
	for i, a := range actions {
		result[i] = model.HeaderAction{
			ID:           string(a.ID),
			Key:          a.Key,
			Rune:         a.Rune,
			Label:        a.Label,
			Modifier:     a.Modifier,
			ShowInHeader: a.ShowInHeader,
		}
	}
	return result
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
