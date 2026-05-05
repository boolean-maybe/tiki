package taskdetail

import (
	"fmt"
	"path/filepath"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/util/gradient"
	"github.com/boolean-maybe/tiki/view/markdown"

	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/rivo/tview"
)

// ConfigurableDetailView renders a tiki using a configured field list
// (`metadata:` from workflow.yaml) plus the always-present title and
// description sections.
//
// This view is the Phase 1 replacement for the old "kind: detail = render
// selected markdown" behavior. It shares the markdown description renderer
// with the legacy TaskDetailView so wikilink and image handling stay
// consistent. Edit-mode (Phase 2) will reuse this same view by switching
// the field-registry RenderMode.
type ConfigurableDetailView struct {
	Base

	registry   *controller.ActionRegistry
	viewID     model.ViewID
	pluginName string

	metadata    []string
	navMarkdown *markdown.NavigableMarkdown
	listenerID  int
}

// NewConfigurableDetailView builds a detail view bound to the configured
// metadata field list. taskID may be empty when the view is opened without a
// selection (the require:["selection:one"] gate normally prevents this; the
// view falls back to a placeholder for safety).
func NewConfigurableDetailView(
	taskStore store.Store,
	taskID string,
	pluginName string,
	metadata []string,
	registry *controller.ActionRegistry,
	imageManager *navtview.ImageManager,
	mermaidOpts *nav.MermaidOptions,
) *ConfigurableDetailView {
	cv := &ConfigurableDetailView{
		Base: Base{
			taskStore:    taskStore,
			taskID:       taskID,
			imageManager: imageManager,
			mermaidOpts:  mermaidOpts,
		},
		registry:   registry,
		viewID:     model.MakePluginViewID(pluginName),
		pluginName: pluginName,
		metadata:   metadata,
	}

	cv.build()
	cv.refresh()

	return cv
}

// GetActionRegistry returns the view's action registry.
func (cv *ConfigurableDetailView) GetActionRegistry() *controller.ActionRegistry {
	return cv.registry
}

// GetViewID returns the plugin-namespaced view id used by the navigation stack.
func (cv *ConfigurableDetailView) GetViewID() model.ViewID {
	return cv.viewID
}

// GetViewName returns the plugin name for the header info section.
func (cv *ConfigurableDetailView) GetViewName() string { return cv.pluginName }

// GetViewDescription returns a short description for the header info section.
func (cv *ConfigurableDetailView) GetViewDescription() string {
	return "configured detail view"
}

// GetSelectedID implements controller.SelectableView so target-scoped
// require: gates and outbound `kind: view` actions see the carried selection.
func (cv *ConfigurableDetailView) GetSelectedID() string {
	return cv.taskID
}

// SetSelectedID lets callers inject a selection after construction. Detail
// views do not surface an in-view selection gesture; this is primarily a
// SelectableView contract fill.
func (cv *ConfigurableDetailView) SetSelectedID(id string) {
	cv.taskID = id
	cv.refresh()
}

// OnFocus subscribes to store updates so external mutations re-render the view.
func (cv *ConfigurableDetailView) OnFocus() {
	cv.listenerID = cv.taskStore.AddListener(func() {
		cv.refresh()
	})
	cv.refresh()
}

// OnBlur unsubscribes and tears down the markdown viewer.
func (cv *ConfigurableDetailView) OnBlur() {
	if cv.listenerID != 0 {
		cv.taskStore.RemoveListener(cv.listenerID)
		cv.listenerID = 0
	}
	if cv.navMarkdown != nil {
		cv.navMarkdown.Close()
		cv.navMarkdown = nil
	}
}

// RestoreFocus pushes focus back to the description viewer after the palette
// or other transient overlays close.
func (cv *ConfigurableDetailView) RestoreFocus() bool {
	if cv.descView != nil && cv.focusSetter != nil {
		cv.focusSetter(cv.descView)
		return true
	}
	return false
}

// refresh re-renders the view contents.
func (cv *ConfigurableDetailView) refresh() {
	cv.content.Clear()
	cv.descView = nil
	if cv.navMarkdown != nil {
		cv.navMarkdown.Close()
		cv.navMarkdown = nil
	}

	tk := cv.GetTiki()
	if tk == nil {
		notFound := tview.NewTextView().SetText("(no tiki selected)")
		cv.content.AddItem(notFound, 0, 1, false)
		return
	}

	colors := config.GetColors()

	if !cv.fullscreen {
		metadataBox := cv.buildMetadataBox(tk, colors)
		// Compute a height tall enough to hold title + spacer + each
		// configured row at minimum 1 line each, plus border padding.
		// Phase 1 uses a fixed minimum to keep layout predictable; future
		// passes can compute dynamic heights from descriptors.
		height := 4 + len(cv.metadata)
		if height < 6 {
			height = 6
		}
		cv.content.AddItem(metadataBox, height, 0, false)
	}

	descPrimitive := cv.buildDescription(tk)
	cv.content.AddItem(descPrimitive, 0, 1, true)

	if cv.focusSetter != nil {
		cv.focusSetter(descPrimitive)
	}
}

// buildMetadataBox assembles the title row and configured metadata fields
// into a framed box. Phase 1 stacks fields vertically — a future pass may
// reuse the responsive multi-column layout once we know which fields the
// user actually configured.
func (cv *ConfigurableDetailView) buildMetadataBox(tk *tikipkg.Tiki, colors *config.ColorConfig) *tview.Frame {
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}

	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.AddItem(RenderTitleText(tk, ctx), 1, 0, false)
	container.AddItem(tview.NewBox(), 1, 0, false)

	for _, name := range cv.metadata {
		row := renderConfiguredField(name, tk, ctx)
		container.AddItem(row, 1, 0, false)
	}

	frame := tview.NewFrame(container).SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true).SetTitle(
		fmt.Sprintf(" %s ", gradient.RenderAdaptiveGradientText(tk.ID, colors.TaskDetailIDColor, colors.FallbackTaskIDColor)),
	).SetBorderColor(colors.TaskBoxUnselectedBorder.TCell())
	frame.SetBorderPadding(1, 0, 2, 2)
	return frame
}

// buildDescription renders the always-present description section. Mirrors
// the legacy TaskDetailView's description path so wikilink rewriting and
// image resolution stay identical.
func (cv *ConfigurableDetailView) buildDescription(tk *tikipkg.Tiki) tview.Primitive {
	desc := defaultString(tk.Body, "(No description)")
	taskSourcePath := taskSourcePathFor(tk)

	searchRoots := []string{config.GetDocDir(), config.GetTaskDir()}
	if taskSourcePath != "" {
		searchRoots = append([]string{filepath.Dir(taskSourcePath)}, searchRoots...)
	}

	resolver := &markdown.StoreResolver{Store: cv.taskStore}
	wrapped := markdown.NewWikilinkProvider(
		newTaskDescriptionProvider(cv.taskStore, searchRoots),
		resolver,
	)
	cv.navMarkdown = markdown.NewNavigableMarkdown(markdown.NavigableMarkdownConfig{
		Provider:       wrapped,
		SearchRoots:    searchRoots,
		ImageManager:   cv.imageManager,
		MermaidOptions: cv.mermaidOpts,
	})
	desc = markdown.RewriteWikilinks(desc, resolver)
	cv.navMarkdown.SetMarkdownWithSource(desc, taskSourcePath, false)
	cv.navMarkdown.Viewer().SetBorderPadding(1, 1, 2, 2)
	cv.descView = cv.navMarkdown.Viewer()
	return cv.navMarkdown.Viewer()
}

// EnterFullscreen and ExitFullscreen mirror the legacy task detail view so
// the existing Fullscreen action keeps working through this view.
func (cv *ConfigurableDetailView) EnterFullscreen() {
	if cv.fullscreen {
		return
	}
	cv.fullscreen = true
	cv.refresh()
	if cv.focusSetter != nil && cv.descView != nil {
		cv.focusSetter(cv.descView)
	}
	if cv.onFullscreenChange != nil {
		cv.onFullscreenChange(true)
	}
}

func (cv *ConfigurableDetailView) ExitFullscreen() {
	if !cv.fullscreen {
		return
	}
	cv.fullscreen = false
	cv.refresh()
	if cv.focusSetter != nil && cv.descView != nil {
		cv.focusSetter(cv.descView)
	}
	if cv.onFullscreenChange != nil {
		cv.onFullscreenChange(false)
	}
}
