package tikidetail

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/view/markdown"
	"github.com/boolean-maybe/tiki/workflow"

	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/rivo/tview"
)

// ConfigurableDetailView renders a tiki using a configured field list
// (`metadata:` from workflow.yaml) plus the description section.
//
// The metadata box height derives from spec.Rows (the grid's row count)
// plus metadataBoxOverhead (borders + padding). Title is a regular grid
// field — it only renders if declared in the metadata layout (e.g.
// `<highlight>title`). Field anchors render as value-only primitives,
// literal cells render as static text captions. Multi-row fields (tags
// via WordList wrapping, depends-on via TikiList) extend downward within
// their own column thanks to per-column natural-height packing in
// gridContainer.rebuild. Heights are clamped by the solver's
// maxRowHeight (6) per row.
//
// In-place edit mode toggles a per-field editor for editable metadata
// fields. Editors come from the field registry (status / type / priority /
// points / assignee / due / recurrence / tags). Read-only descriptors and
// fields without an implemented editor render their read-only primitive
// even in edit mode, matching the "render but skip in traversal" rule.
type ConfigurableDetailView struct {
	Base

	registry     *controller.ActionRegistry
	viewRegistry *controller.ActionRegistry // saved view-mode registry
	editRegistry *controller.ActionRegistry // edit-mode registry (built lazily)
	viewID       model.ViewID
	pluginName   string

	spec        gridlayout.GridSpec // parsed metadata grid (layout)
	metadata    []string            // flat anchor names in declaration order (edit traversal)
	navMarkdown *markdown.NavigableMarkdown
	listenerID  int

	// edit-mode state
	editMode          bool
	focusedIdx        int                          // position in metadata (-1 = not editing)
	editors           map[string]FieldEditorWidget // current editor widgets by field name
	onEditFieldChange map[string]func(string)      // per-field save callbacks set by controller
	onEditModeChange  func(bool)                   // controller-side notifier (registry derivation)
	editTikiSource    func() *tikipkg.Tiki         // returns editing copy during edit mode

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
// metadata field list. tikiID may be empty when the view is opened without a
// selection (the require:["selection:one"] gate normally prevents this; the
// view falls back to a placeholder for safety).
func NewConfigurableDetailView(
	tikiStore store.Store,
	tikiID string,
	pluginName string,
	spec gridlayout.GridSpec,
	registry *controller.ActionRegistry,
	imageManager *navtview.ImageManager,
	mermaidOpts *nav.MermaidOptions,
) *ConfigurableDetailView {
	cv := &ConfigurableDetailView{
		Base: Base{
			tikiStore:    tikiStore,
			tikiID:       tikiID,
			imageManager: imageManager,
			mermaidOpts:  mermaidOpts,
		},
		registry:          registry,
		viewRegistry:      registry,
		viewID:            model.MakePluginViewID(pluginName),
		pluginName:        pluginName,
		spec:              spec,
		metadata:          spec.AnchorNames(),
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
	return cv.tikiID
}

// SetSelectedID lets callers inject a selection after construction. Detail
// views do not surface an in-view selection gesture; this is primarily a
// SelectableView contract fill.
func (cv *ConfigurableDetailView) SetSelectedID(id string) {
	cv.tikiID = id
	cv.refresh()
}

// OnFocus subscribes to store updates so external mutations re-render the view.
func (cv *ConfigurableDetailView) OnFocus() {
	cv.listenerID = cv.tikiStore.AddListener(func() {
		cv.refresh()
	})
	cv.refresh()
}

// OnBlur unsubscribes and tears down the markdown viewer.
func (cv *ConfigurableDetailView) OnBlur() {
	if cv.listenerID != 0 {
		cv.tikiStore.RemoveListener(cv.listenerID)
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

// getTiki returns the tiki to render. In edit mode with an editing source
// installed, the in-memory editing copy is preferred so that in-flight
// changes (cycled enum, typed assignee) survive Tab-driven refreshes.
func (cv *ConfigurableDetailView) getTiki() *tikipkg.Tiki {
	if cv.editMode && cv.editTikiSource != nil {
		if tk := cv.editTikiSource(); tk != nil {
			return tk
		}
	}
	return cv.GetTiki()
}

// refresh re-renders the view contents.
func (cv *ConfigurableDetailView) refresh() {
	cv.content.Clear()
	cv.descView = nil
	if cv.navMarkdown != nil {
		cv.navMarkdown.Close()
		cv.navMarkdown = nil
	}

	tk := cv.getTiki()
	if tk == nil {
		notFound := tview.NewTextView().SetText("(no tiki selected)")
		cv.content.AddItem(notFound, 0, 1, false)
		return
	}

	roles := theme.Roles()

	if !cv.fullscreen {
		metadataBox := cv.buildMetadataBox(tk, roles)
		cv.content.AddItem(metadataBox, cv.spec.Rows+metadataBoxOverhead, 0, false)
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

// metadataBoxOverhead is the fixed vertical cost beyond the grid body:
// 1 top border + 1 top padding + 1 spacer + 1 bottom border.
const metadataBoxOverhead = 4

// buildMetadataBox assembles the configured metadata fields into a framed
// box. Title renders only if declared in the metadata grid (as a regular
// field cell). Configured fields flow into the grid body via the
// gridContainer primitive, which honors the layout grid declared in
// workflow.yaml (column/row spans, stretchers, preferred widths).
//
// In edit mode, fields whose registry advertises EditorImplemented receive
// a focusable editor primitive (cached on cv.editors so user input
// survives a refresh); other fields render their read-only primitive even
// in edit mode.
func (cv *ConfigurableDetailView) buildMetadataBox(tk *tikipkg.Tiki, roles *theme.Theme) *tview.Frame {
	mode := RenderModeView
	if cv.editMode {
		mode = RenderModeEdit
	}
	ctx := FieldRenderContext{Mode: mode, Roles: roles, Store: cv.tikiStore}

	primitives := cv.buildAnchorPrimitives(tk, ctx)
	heightOf := func(name string, w int) int { return FieldHeight(name, tk, w) }

	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.AddItem(newGridContainer(cv.spec, primitives, heightOf), cv.spec.Rows, 0, false)
	container.AddItem(tview.NewBox(), 1, 0, false)

	frame := tview.NewFrame(container).SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true).SetTitle(
		fmt.Sprintf(" %s ", renderTikiIDGradient(tk.ID, roles)),
	).SetBorderColor(roles.BorderIdle().TCell())
	frame.SetBorderPadding(1, 0, 2, 2)
	return frame
}

// buildAnchorPrimitives produces one tview.Primitive per anchor in the
// metadata grid, indexed by anchor position. Two cases:
//
//  1. Literal anchor: renders as a static text view carrying the caption
//     text declared by the layout author.
//  2. Field anchor: read-only renderer (with optional role color for text
//     fields), or (in edit mode + focused + editable) a cached editor widget.
func (cv *ConfigurableDetailView) buildAnchorPrimitives(tk *tikipkg.Tiki, ctx FieldRenderContext) []tview.Primitive {
	focusedName := cv.GetFocusedFieldName()
	primitives := make([]tview.Primitive, len(cv.spec.Anchors))
	for i, a := range cv.spec.Anchors {
		switch a.Kind {
		case gridlayout.AnchorLiteral:
			primitives[i] = renderLiteralCaption(a.Text, ctx.Roles)
		case gridlayout.AnchorComposite:
			primitives[i] = cv.buildCompositePrimitive(a, tk, ctx, focusedName)
		default:
			primitives[i] = cv.buildFieldPrimitive(a, tk, ctx, focusedName)
		}
	}
	return primitives
}

// buildCompositeText is the text-building half of renderCompositePrimitive.
// Split out so tests can pin the exact byte output without invoking tview.
// The bare-role and modifier branches differ in tag emission shape (see
// renderCompositePrimitive's contract for the reasoning), so the test that
// locks this output guards against silent drift between the two paths.
func buildCompositeText(a gridlayout.Anchor, tk *tikipkg.Tiki, ctx FieldRenderContext) string {
	var buf strings.Builder
	tag := ctx.Roles.TextValue().Tag()
	buf.WriteString(tag)
	for _, seg := range a.Segments {
		// build segment content first so the modifier path (which emits a
		// trailing [-] reset) wraps a complete string. The bare-role path
		// preserves the legacy "open tag then write text" emission so
		// existing visual snapshots stay byte-identical.
		var content string
		switch seg.Kind {
		case gridlayout.SegmentLiteral:
			content = seg.Text
		case gridlayout.SegmentField:
			segCtx := ctx
			segCtx.FieldName = seg.Name
			segCtx.Display = seg.Display
			switch {
			case seg.Name == "title":
				content = expandFieldText(tk.Title, ctx.Roles)
			default:
				if wfd, ok := workflow.Field(seg.Name); ok {
					content = genericFieldValueString(wfd, tk, segCtx)
				} else {
					content = "—"
				}
			}
		}

		switch {
		case seg.Role == "":
			buf.WriteString(content)
		case seg.Modifier != "" && ctx.Roles != nil:
			if paint, ok := ctx.Roles.PaintResolver()(seg.Role, seg.Modifier); ok {
				buf.WriteString(paint.PaintString(content))
				// re-establish the composite's default value color after the
				// Paint's trailing [-] reset so the next bare segment renders
				// in the value tag, not in tview's default style.
				buf.WriteString(tag)
				continue
			}
			// resolver miss — degrade to the bare-role path so the segment
			// still picks up its role color.
			if r, ok := ctx.Roles.ResolveByName(seg.Role); ok {
				buf.WriteString(r.Tag())
			}
			buf.WriteString(content)
		default:
			if r, ok := ctx.Roles.ResolveByName(seg.Role); ok {
				buf.WriteString(r.Tag())
			}
			buf.WriteString(content)
		}
	}
	return buf.String()
}

func renderCompositePrimitive(a gridlayout.Anchor, tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	tv := tview.NewTextView().SetDynamicColors(true).SetText(buildCompositeText(a, tk, ctx))
	tv.SetBorderPadding(0, 0, 0, 0)
	return tv
}

// renderLiteralCaption produces the read-only text primitive for a literal
// anchor. Uses the dim-label color so explicit captions visually match the
// dim "Status:" appearance of the legacy in-renderer labels.
//
// Caption text may contain `<role>` markup (e.g. `<danger>!!!`) drawn from
// workflow.ValidRoles; literal `<` is escaped as `<<`. Markup is parsed by
// workflow.ExpandVisual and resolved against the active theme. Captions
// originate in workflow yaml (controlled by the workflow author), so the
// raw text is not pre-escaped against `[...]` tview tags. Unknown roles
// fail closed to the plain caption text — the workflow loader's
// validateDetailMetadata gate already rejects bad role names at startup.
func renderLiteralCaption(text string, roles *theme.Theme) tview.Primitive {
	tag := roles.TextLabel().Tag()
	expanded, err := workflow.ExpandVisual(text, roles.PaintResolver())
	if err != nil {
		expanded = text
	}
	tv := tview.NewTextView().SetDynamicColors(true).SetText(tag + expanded)
	tv.SetBorderPadding(0, 0, 0, 0)
	return tv
}

// buildFieldPrimitive returns the widget for a metadata field. In view mode
// (or for fields with no editor) this is the field-registry's read-only
// renderer. In edit mode for the focused field, it returns a cached or
// freshly-created editor widget. Focus is matched by field name (not
// positional index) so literal and composite anchors don't shift the
// mapping. Editor widgets are cached per-field for the lifetime of edit
// mode so user input isn't lost when other rows trigger a refresh.
func (cv *ConfigurableDetailView) buildFieldPrimitive(a gridlayout.Anchor, tk *tikipkg.Tiki, ctx FieldRenderContext, focusedName string) tview.Primitive {
	name := a.Name
	ctx.Display = a.Display
	renderField := func() tview.Primitive {
		if name == "title" {
			return RenderTitleText(tk, ctx, a.Role, a.Modifier)
		}
		return renderConfiguredField(name, tk, ctx)
	}
	if !cv.editMode {
		return renderField()
	}
	if !FieldHasEditor(name) {
		return renderField()
	}
	if name != focusedName {
		return renderField()
	}
	return cv.ensureEditor(name, tk, ctx)
}

// buildCompositePrimitive returns the widget for a composite anchor. In
// view mode (or when not focused) this is the read-only concatenation. In
// edit mode, single-field composites delegate to the field's editor so the
// user can cycle through values. Multi-field composites stay read-only.
func (cv *ConfigurableDetailView) buildCompositePrimitive(a gridlayout.Anchor, tk *tikipkg.Tiki, ctx FieldRenderContext, focusedName string) tview.Primitive {
	if !cv.editMode || a.Name == "" || a.Name != focusedName {
		return renderCompositePrimitive(a, tk, ctx)
	}
	if !FieldHasEditor(a.Name) {
		return renderCompositePrimitive(a, tk, ctx)
	}
	return cv.ensureEditor(a.Name, tk, ctx)
}

// ensureEditor returns a cached or freshly-created editor widget for the
// named field. Shared by buildFieldPrimitive and buildCompositePrimitive.
func (cv *ConfigurableDetailView) ensureEditor(name string, tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if w, ok := cv.editors[name]; ok && w != nil {
		return w
	}
	rowCtx := ctx
	if fd, ok := LookupField(name); ok {
		rowCtx.FocusedField = fd.EditField
	} else {
		rowCtx.FocusedField = model.EditField(name)
	}
	w := buildFieldEditor(name, tk, rowCtx, cv.onEditFieldChange[name])
	if w == nil {
		return renderConfiguredField(name, tk, ctx)
	}
	cv.editors[name] = w
	return w
}

// buildDescription renders the always-present description section. Mirrors
// the legacy TikiDetailView's description path so wikilink rewriting and
// image resolution stay identical.
func (cv *ConfigurableDetailView) buildDescription(tk *tikipkg.Tiki) tview.Primitive {
	desc := defaultString(tk.Body, "(No description)")
	tikiSourcePath := tikiSourcePathFor(tk)

	searchRoots := []string{config.GetDocDir()}
	if tikiSourcePath != "" {
		searchRoots = append([]string{filepath.Dir(tikiSourcePath)}, searchRoots...)
	}

	resolver := &markdown.StoreResolver{Store: cv.tikiStore}
	wrapped := markdown.NewWikilinkProvider(
		newTikiDescriptionProvider(cv.tikiStore, searchRoots),
		resolver,
	)
	cv.navMarkdown = markdown.NewNavigableMarkdown(markdown.NavigableMarkdownConfig{
		Provider:       wrapped,
		SearchRoots:    searchRoots,
		ImageManager:   cv.imageManager,
		MermaidOptions: cv.mermaidOpts,
	})
	desc = markdown.RewriteWikilinks(desc, resolver)
	cv.navMarkdown.SetMarkdownWithSource(desc, tikiSourcePath, false)
	cv.navMarkdown.Viewer().SetBorderPadding(1, 1, 2, 2)
	cv.descView = cv.navMarkdown.Viewer()
	return cv.navMarkdown.Viewer()
}

// EnterFullscreen and ExitFullscreen mirror the legacy tiki detail view so
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

// SetEditTikiSource installs a function that returns the in-memory editing
// copy of the tiki. When set and edit mode is active, refresh reads from
// this source instead of the store so in-flight field changes (status
// cycled, assignee typed) survive a Tab-driven re-render.
func (cv *ConfigurableDetailView) SetEditTikiSource(fn func() *tikipkg.Tiki) {
	cv.editTikiSource = fn
}

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

// Metadata returns the configured metadata field list. Exposed so the
// input router can copy it into TikiEditParams when the user opens the
// edit view from this detail view, preserving the same field set across
// the view-edit transition.
func (cv *ConfigurableDetailView) Metadata() []string {
	return cv.metadata
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

// FlushFocusedEditor invokes the per-field onChange handler for every
// cached editor, passing each widget's live text. Despite the "Focused"
// in the name (kept for the original interface), the implementation
// flushes *all* visited editors — not just the one currently holding
// focus — because some widgets (notably the tags textarea) buffer their
// input internally and only push it on Ctrl+S. If the user edits tags,
// tabs to another field, then presses Ctrl+S, the tags widget is no
// longer focused but its cached buffer must still flow into the edit
// session before commit, otherwise the in-flight edit is dropped.
//
// Flush order matters: SaveRecurrence writes Due as a side effect (when
// recurrence is non-empty), so a stale Due flush after SaveRecurrence
// would overwrite the auto-computed Due with the user's pre-recurrence
// text. Iterate cv.metadata in declaration order, then flush recurrence
// last — so any side-effect-producing field always wins over stale
// per-field cache entries.
//
// Each editor's onChange is idempotent — cyclable editors already emit
// on every step, so re-emitting their current value is a no-op for the
// edit session. The cost is a per-editor handler call, bounded by the
// number of fields the user actually visited in this edit session.
func (cv *ConfigurableDetailView) FlushFocusedEditor() {
	if !cv.editMode {
		return
	}
	flush := func(name string) {
		w, ok := cv.editors[name]
		if !ok || w == nil {
			return
		}
		handler, ok := cv.onEditFieldChange[name]
		if !ok || handler == nil {
			return
		}
		handler(w.GetText())
	}
	// First pass: every cached metadata field except recurrence, in
	// declaration order. This is deterministic so map-iteration races
	// can't reorder due/recurrence/etc.
	for _, name := range cv.metadata {
		if name == tikipkg.FieldRecurrence {
			continue
		}
		flush(name)
	}
	// Second pass: recurrence last, so its Due side-effect overwrites
	// any stale Due that flushed in the first pass.
	flush(tikipkg.FieldRecurrence)
}

// isEditableMetadataField returns true when the named field has an
// implemented editor and is traversable. Built-in fields are gated by
// their static descriptor (ReadOnly/EditTraversable); workflow-declared
// fields without a static descriptor are editable when the field
// registry's FieldHasEditor reports an editor — currently only TypeEnum
// fields qualify, since they're the only workflow-only fields that
// route through a generic editor.
func (cv *ConfigurableDetailView) isEditableMetadataField(name string) bool {
	if fd, ok := LookupField(name); ok {
		if fd.ReadOnly || !fd.EditTraversable {
			return false
		}
		return FieldHasEditor(name)
	}
	// Workflow-only fields: defer entirely to FieldHasEditor, which
	// already excludes non-editable types.
	return FieldHasEditor(name)
}
