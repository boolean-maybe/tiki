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
// consistent.
//
// Phase 2 added in-place edit mode: the same view is reused by toggling
// editMode, which swaps the read-only metadata renderer for editor widgets
// driven by the field-registry. Tab traversal walks editable metadata
// fields in `metadata:` order (skipping stubs and read-only fields).
type ConfigurableDetailView struct {
	Base

	registry     *controller.ActionRegistry
	viewRegistry *controller.ActionRegistry // saved view-mode registry
	editRegistry *controller.ActionRegistry // edit-mode registry (built lazily)
	viewID       model.ViewID
	pluginName   string

	metadata    []string
	navMarkdown *markdown.NavigableMarkdown
	listenerID  int

	// edit-mode state
	editMode          bool
	focusedIdx        int                          // position in metadata (-1 = not editing)
	editors           map[string]FieldEditorWidget // current editor widgets by field name
	onEditFieldChange map[string]func(string)      // per-field save callbacks set by controller
	onEditModeChange  func(bool)                   // controller-side notifier (registry derivation)

	// actionChangeHandler is RootLayout's ActionChangeNotifier hook — fired
	// whenever the view's installed registry mutates (currently: on
	// EnterEditMode / ExitEditMode). Without this, the header bar and
	// palette would keep showing read-only actions while edit mode is
	// active, even though the keyboard dispatch is already correct.
	actionChangeHandler func()
}

// Compile-time interface check: the view must satisfy ActionChangeNotifier
// so RootLayout can subscribe to registry swaps without re-activating.
var _ controller.ActionChangeNotifier = (*ConfigurableDetailView)(nil)

// SetActionChangeHandler implements controller.ActionChangeNotifier. The
// handler is invoked after edit-mode toggles so the header/palette
// re-read the active registry from this view.
func (cv *ConfigurableDetailView) SetActionChangeHandler(handler func()) {
	cv.actionChangeHandler = handler
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
		registry:          registry,
		viewRegistry:      registry,
		viewID:            model.MakePluginViewID(pluginName),
		pluginName:        pluginName,
		metadata:          metadata,
		focusedIdx:        -1,
		editors:           make(map[string]FieldEditorWidget),
		onEditFieldChange: make(map[string]func(string)),
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

	if cv.editMode {
		cv.focusActiveEditor()
		return
	}
	if cv.focusSetter != nil {
		cv.focusSetter(descPrimitive)
	}
}

// focusActiveEditor pushes focus to the currently focused editor widget
// while in edit mode. If the focused field has no editor (e.g. focusedIdx
// landed on a stub field) the focus falls back to the description so the
// view remains usable.
func (cv *ConfigurableDetailView) focusActiveEditor() {
	if cv.focusSetter == nil {
		return
	}
	if cv.focusedIdx < 0 || cv.focusedIdx >= len(cv.metadata) {
		return
	}
	name := cv.metadata[cv.focusedIdx]
	if w, ok := cv.editors[name]; ok && w != nil {
		cv.focusSetter(w)
		return
	}
	if cv.descView != nil {
		cv.focusSetter(cv.descView)
	}
}

// buildMetadataBox assembles the title row and configured metadata fields
// into a framed box. Phase 1 stacks fields vertically — a future pass may
// reuse the responsive multi-column layout once we know which fields the
// user actually configured.
//
// When editMode is true, each metadata row is rendered through the field
// registry's editor factory if one exists; the focused row receives focus
// via focusActiveEditor. Stub or read-only fields render the standard
// read-only row even in edit mode, matching the plan's "render but skip in
// traversal" requirement.
func (cv *ConfigurableDetailView) buildMetadataBox(tk *tikipkg.Tiki, colors *config.ColorConfig) *tview.Frame {
	mode := RenderModeView
	if cv.editMode {
		mode = RenderModeEdit
	}
	ctx := FieldRenderContext{Mode: mode, Colors: colors}

	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.AddItem(RenderTitleText(tk, ctx), 1, 0, false)
	container.AddItem(tview.NewBox(), 1, 0, false)

	for i, name := range cv.metadata {
		row := cv.buildMetadataRow(i, name, tk, ctx)
		container.AddItem(row, 1, 0, false)
	}

	frame := tview.NewFrame(container).SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true).SetTitle(
		fmt.Sprintf(" %s ", gradient.RenderAdaptiveGradientText(tk.ID, colors.TaskDetailIDColor, colors.FallbackTaskIDColor)),
	).SetBorderColor(colors.TaskBoxUnselectedBorder.TCell())
	frame.SetBorderPadding(1, 0, 2, 2)
	return frame
}

// buildMetadataRow returns the widget for a metadata field. In view mode
// (or for fields with no editor) this is the field-registry's read-only
// renderer. In edit mode for fields with implemented editors it returns a
// cached or freshly-created editor widget. Editor widgets are cached
// per-field for the lifetime of edit mode so user input isn't lost when
// other rows trigger a refresh.
func (cv *ConfigurableDetailView) buildMetadataRow(idx int, name string, tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if !cv.editMode {
		return renderConfiguredField(name, tk, ctx)
	}
	if !FieldHasEditor(name) {
		return renderConfiguredField(name, tk, ctx)
	}
	rowCtx := ctx
	if fd, ok := LookupField(name); ok {
		rowCtx.FocusedField = fd.EditField
	}
	if w, ok := cv.editors[name]; ok && w != nil {
		return w
	}
	w := buildFieldEditor(name, tk, rowCtx, cv.onEditFieldChange[name])
	if w == nil {
		return renderConfiguredField(name, tk, ctx)
	}
	cv.editors[name] = w
	_ = idx
	return w
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

// --- Phase 2: edit-mode API ---

// IsEditMode reports whether the view is currently in in-place edit mode.
func (cv *ConfigurableDetailView) IsEditMode() bool { return cv.editMode }

// SetEditModeRegistry installs the action registry to surface while in edit
// mode. Called by the controller during construction so the view can
// swap registries on mode change without depending on the controller
// directly. Passing nil keeps the view-mode registry on toggle.
func (cv *ConfigurableDetailView) SetEditModeRegistry(r *controller.ActionRegistry) {
	cv.editRegistry = r
}

// SetEditModeChangeHandler registers a notifier invoked when edit mode
// toggles. The controller uses this to refresh action surfacing.
func (cv *ConfigurableDetailView) SetEditModeChangeHandler(h func(bool)) {
	cv.onEditModeChange = h
}

// SetEditFieldChangeHandler registers the per-field save callback. The
// callback receives the editor's display value (e.g. "Bug 💥") whenever
// the user changes the value while in edit mode. Calling with a nil
// handler clears the registration.
func (cv *ConfigurableDetailView) SetEditFieldChangeHandler(fieldName string, h func(string)) {
	if cv.onEditFieldChange == nil {
		cv.onEditFieldChange = make(map[string]func(string))
	}
	if h == nil {
		delete(cv.onEditFieldChange, fieldName)
		return
	}
	cv.onEditFieldChange[fieldName] = h
}

// EnterEditMode flips the view into edit mode and focuses the first
// editable metadata field. No-op if no field has an implemented editor —
// the view stays in view mode and the controller can surface a
// "nothing to edit" hint.
func (cv *ConfigurableDetailView) EnterEditMode() bool {
	if cv.editMode {
		return true
	}
	first := cv.firstEditableIndex()
	if first < 0 {
		return false
	}
	cv.editMode = true
	cv.focusedIdx = first
	cv.editors = make(map[string]FieldEditorWidget)
	if cv.editRegistry != nil {
		cv.registry = cv.editRegistry
	}
	cv.refresh()
	if cv.onEditModeChange != nil {
		cv.onEditModeChange(true)
	}
	if cv.actionChangeHandler != nil {
		cv.actionChangeHandler()
	}
	return true
}

// ExitEditMode flips the view back to read-only and discards any cached
// editor widgets. The controller is responsible for committing or
// cancelling the underlying edit session before calling this.
func (cv *ConfigurableDetailView) ExitEditMode() {
	if !cv.editMode {
		return
	}
	cv.editMode = false
	cv.focusedIdx = -1
	cv.editors = make(map[string]FieldEditorWidget)
	cv.registry = cv.viewRegistry
	cv.refresh()
	if cv.onEditModeChange != nil {
		cv.onEditModeChange(false)
	}
	if cv.actionChangeHandler != nil {
		cv.actionChangeHandler()
	}
}

// FocusNextField moves focus to the next editable metadata field,
// skipping stubs and read-only descriptors. Returns false if the view
// is not in edit mode or no later editable field exists.
func (cv *ConfigurableDetailView) FocusNextField() bool {
	if !cv.editMode {
		return false
	}
	next := cv.nextEditableIndex(cv.focusedIdx)
	if next < 0 || next == cv.focusedIdx {
		return false
	}
	cv.focusedIdx = next
	cv.refresh()
	return true
}

// FocusPrevField moves focus to the previous editable metadata field,
// skipping stubs and read-only descriptors. Returns false if the view
// is not in edit mode or no earlier editable field exists.
func (cv *ConfigurableDetailView) FocusPrevField() bool {
	if !cv.editMode {
		return false
	}
	prev := cv.prevEditableIndex(cv.focusedIdx)
	if prev < 0 || prev == cv.focusedIdx {
		return false
	}
	cv.focusedIdx = prev
	cv.refresh()
	return true
}

// GetFocusedFieldName returns the metadata name of the currently focused
// editable field, or empty string when not in edit mode.
func (cv *ConfigurableDetailView) GetFocusedFieldName() string {
	if !cv.editMode || cv.focusedIdx < 0 || cv.focusedIdx >= len(cv.metadata) {
		return ""
	}
	return cv.metadata[cv.focusedIdx]
}

// IsEditFieldFocused reports whether any of the current edit-mode editor
// widgets currently holds tview focus. Used by the input router to route
// keys to the editor (Tab, arrows) before falling through to the action
// registry.
func (cv *ConfigurableDetailView) IsEditFieldFocused() bool {
	if !cv.editMode {
		return false
	}
	for _, w := range cv.editors {
		if w != nil && w.HasFocus() {
			return true
		}
	}
	return false
}

// firstEditableIndex returns the metadata position of the first field
// with an implemented editor and a traversable descriptor, or -1.
func (cv *ConfigurableDetailView) firstEditableIndex() int {
	for i, name := range cv.metadata {
		if cv.isEditableMetadataField(name) {
			return i
		}
	}
	return -1
}

// nextEditableIndex returns the next metadata position (after current)
// that is editable, or -1.
func (cv *ConfigurableDetailView) nextEditableIndex(current int) int {
	for i := current + 1; i < len(cv.metadata); i++ {
		if cv.isEditableMetadataField(cv.metadata[i]) {
			return i
		}
	}
	return -1
}

// prevEditableIndex returns the previous metadata position (before
// current) that is editable, or -1.
func (cv *ConfigurableDetailView) prevEditableIndex(current int) int {
	for i := current - 1; i >= 0; i-- {
		if cv.isEditableMetadataField(cv.metadata[i]) {
			return i
		}
	}
	return -1
}

// isEditableMetadataField returns true when the named field has an
// implemented editor and a traversable, non-read-only descriptor.
func (cv *ConfigurableDetailView) isEditableMetadataField(name string) bool {
	fd, ok := LookupField(name)
	if !ok {
		return false
	}
	if fd.ReadOnly || !fd.EditTraversable {
		return false
	}
	return FieldHasEditor(name)
}
