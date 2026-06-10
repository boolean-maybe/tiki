package tikidetail

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/view/gridbox"
	"github.com/boolean-maybe/tiki/view/markdown"
	"github.com/boolean-maybe/tiki/workflow"

	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/rivo/tview"
)

// ConfigurableDetailView renders a tiki using a configured field list
// (`layout:` from workflow.yaml) plus the description section.
//
// The layout box height derives from spec.Rows (the grid's row count)
// plus gridbox.DetailBoxOverhead (borders + padding + spacer). Title is a regular
// grid field — it only renders if declared in the layout (e.g.
// `<highlight>title`). Field anchors render as value-only primitives,
// literal cells render as static text captions. Multi-row fields (tags
// via WordList wrapping, depends-on via TikiList) extend downward within
// their own column thanks to per-column natural-height packing in
// gridbox.Container.rebuild. Heights are clamped by the solver's
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

	pluginDef *plugin.DetailPlugin // workflow-yaml-derived definition; source of name/description/layout

	spec   gridlayout.GridSpec // parsed layout grid (alias of pluginDef.Layout, kept for hot-path reads)
	layout []string            // flat anchor names in column-major order (edit traversal)

	// layoutDisplays is positionally aligned with layout: layoutDisplays[i] is
	// the DisplayMode of the anchor that produced layout[i]. Display-only anchors
	// (DisplayCaption, DisplayCount) carry a field name but render the field's
	// label / item count, not its editable value — they must never be
	// edit-traversal stops. Without this, a field with both such a cell and a
	// value cell (the bundled Detail layout has a `.caption` per field) appears
	// twice in layout, and Tab stops on the display-only cell first, requiring a
	// second press to reach the value.
	layoutDisplays []gridlayout.DisplayMode

	navMarkdown *markdown.NavigableMarkdown
	listenerID  int

	// edit-mode state
	editMode          bool
	focusedIdx        int                          // position in layout (-1 = not editing)
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

	// onFieldFocusChange fires whenever the focused edit-mode field
	// changes (entering edit mode, Tab/Shift-Tab, exiting). Empty value
	// signals "no editable field is focused" and lets consumers clear
	// any field-specific state.
	onFieldFocusChange func(model.EditField)
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

// SetFieldFocusChangeHandler installs a callback fired after every
// edit-mode focus change. Implements controller.FieldFocusChangeNotifier
// so the controller can refresh per-field statusline hints.
func (cv *ConfigurableDetailView) SetFieldFocusChangeHandler(handler func(model.EditField)) {
	cv.onFieldFocusChange = handler
}

// setFocusedIdx is the single funnel for focused-field assignment. All
// edit-mode focus transitions route through this so the focus-change
// notifier sees every move (including the initial focus on
// EnterEditMode and the -1 reset on ExitEditMode).
func (cv *ConfigurableDetailView) setFocusedIdx(idx int) {
	cv.focusedIdx = idx
	if cv.onFieldFocusChange == nil {
		return
	}
	var name string
	switch {
	case idx >= 0 && idx < len(cv.layout):
		name = cv.layout[idx]
	case idx == len(cv.layout):
		name = descriptionFieldName
	}
	cv.onFieldFocusChange(model.EditField(name))
}

// NewConfigurableDetailView builds a detail view bound to a workflow-declared
// detail plugin. tikiID may be empty when the view is opened without a
// selection (the require:["selection:one"] gate normally prevents this; the
// view falls back to a placeholder for safety).
func NewConfigurableDetailView(
	tikiStore store.Store,
	tikiID string,
	pluginDef *plugin.DetailPlugin,
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
		pluginDef:         pluginDef,
		spec:              pluginDef.Layout,
		layout:            pluginDef.Layout.AnchorNamesColumnMajor(),
		layoutDisplays:    pluginDef.Layout.AnchorDisplaysColumnMajor(),
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
	return model.MakePluginViewID(cv.pluginDef.GetName())
}

// GetViewName returns the plugin name for the header info section.
func (cv *ConfigurableDetailView) GetViewName() string { return cv.pluginDef.GetName() }

// GetViewDescription returns the workflow-yaml description for the header info section.
func (cv *ConfigurableDetailView) GetViewDescription() string {
	return cv.pluginDef.GetDescription()
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
		cv.content.AddItem(metadataBox, cv.spec.Rows+gridbox.DetailBoxOverhead, 0, false)
	}

	descPrimitive := cv.buildDescriptionSection(tk)
	cv.content.AddItem(descPrimitive, 0, 1, true)

	if cv.editMode {
		cv.focusActiveEditor()
		return
	}
	if cv.focusSetter != nil {
		cv.focusSetter(descPrimitive)
	}
}

// buildDescriptionSection returns either the read-only markdown viewer or
// the inline description TextArea editor, depending on whether edit mode
// is active and description has focus. Caching the editor on cv.editors
// (keyed by descriptionFieldName) means typed content survives
// inter-field refreshes — the same contract metadata editors rely on.
func (cv *ConfigurableDetailView) buildDescriptionSection(tk *tikipkg.Tiki) tview.Primitive {
	if cv.editMode && cv.GetFocusedFieldName() == descriptionFieldName {
		return cv.ensureDescriptionEditor(tk)
	}
	return cv.buildDescription(tk)
}

// ensureDescriptionEditor returns the cached description TextArea editor,
// constructing it on first focus. The widget seeds from tk.Body and
// pushes every change through cv.onEditFieldChange[descriptionFieldName]
// (wired by the controller to TikiEditSession.SaveDescription) so the
// editing tiki stays current and the title-style commit-on-Enter contract
// doesn't apply (a description body needs newlines).
func (cv *ConfigurableDetailView) ensureDescriptionEditor(tk *tikipkg.Tiki) tview.Primitive {
	if w, ok := cv.editors[descriptionFieldName]; ok && w != nil {
		return w
	}
	textArea := tview.NewTextArea()
	textArea.SetText(tk.Body(), false)
	// Match the read-only markdown viewer's framing: no border, equivalent
	// padding so the cursor lines up with the rendered prose. The metadata
	// box above already provides a visual seam between the two sections.
	textArea.SetBorder(false)
	textArea.SetBorderPadding(1, 1, 2, 2)
	handler := cv.onEditFieldChange[descriptionFieldName]
	if handler != nil {
		textArea.SetChangedFunc(func() {
			handler(textArea.GetText())
		})
	}
	adapter := &descriptionEditAdapter{TextArea: textArea}
	cv.editors[descriptionFieldName] = adapter
	return adapter
}

// GetPreferredFocus implements controller.PreferredFocusProvider. When the
// view is in edit mode, returns the active editor primitive so RootLayout's
// post-activation focus call lands on the editor instead of the view root.
// The view factory enters edit mode during construction (via ApplyDetailMode);
// without this hook, RootLayout's app.SetFocus(root) would override any focus
// the view tried to set during OnFocus and the user would be unable to type.
func (cv *ConfigurableDetailView) GetPreferredFocus() tview.Primitive {
	if !cv.editMode {
		return nil
	}
	name := cv.GetFocusedFieldName()
	if name == "" {
		return nil
	}
	if w, ok := cv.editors[name]; ok && w != nil {
		return w
	}
	return nil
}

// focusActiveEditor pushes focus to the currently focused editor widget
// while in edit mode. The description pseudo-index (len(layout)) points
// at the inline description TextArea editor; metadata indices point at
// per-anchor editors cached on cv.editors. If the focused field has no
// editor (e.g. focusedIdx landed on a stub field) the focus falls back
// to the description viewer so the view remains usable.
func (cv *ConfigurableDetailView) focusActiveEditor() {
	if cv.focusSetter == nil {
		return
	}
	if cv.focusedIdx < 0 {
		return
	}
	name := cv.GetFocusedFieldName()
	if name == "" {
		return
	}
	if w, ok := cv.editors[name]; ok && w != nil {
		cv.focusSetter(w)
		return
	}
	if cv.descView != nil {
		cv.focusSetter(cv.descView)
	}
}

// fieldHasValue reports whether tk carries a non-empty value for the named
// field. Emptiness is defined by the same value-formatting path the view
// renders (genericFieldValueString), so a field hidden via the `?` suffix is
// hidden exactly when it would otherwise render its empty placeholder. Unknown
// fields are treated as having no value.
func fieldHasValue(tk *tikipkg.Tiki, name string) bool {
	fd, ok := workflow.Field(name)
	if !ok {
		return false
	}
	// "—" is the empty placeholder genericFieldValueString emits for every type.
	return genericFieldValueString(fd, tk, FieldRenderContext{}) != "—"
}

// specForTiki returns the layout spec to render for tk: the parsed template
// with `?`-marked empty fields (and their field.caption cells) hidden. The
// HideWhenEmpty trait drives hiding, so any field type may opt in — replacing
// the former hardcoded list-type special-case.
func (cv *ConfigurableDetailView) specForTiki(tk *tikipkg.Tiki) gridlayout.GridSpec {
	has := func(name string) bool { return fieldHasValue(tk, name) }
	hidden := gridlayout.EmptyFieldNames(cv.spec, has)
	return gridlayout.HideFields(cv.spec, hidden)
}

// buildMetadataBox assembles the configured layout fields into a framed
// box. Title renders only if declared in the layout grid (as a regular
// field cell). Configured fields flow into the grid body via the
// gridbox.Container primitive, which honors the layout grid declared in
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

	spec := cv.specForTiki(tk)
	primitives := cv.buildAnchorPrimitivesForSpec(spec, tk, ctx)
	heightOf := func(a gridlayout.Anchor, w int) int { return FieldHeight(a.Name, tk, w) }
	measure := func(a gridlayout.Anchor) int { return MeasureAnchor(a, tk, ctx) }

	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.AddItem(gridbox.NewContainer(spec, primitives, measure, heightOf), spec.Rows, 0, false)
	container.AddItem(tview.NewBox(), 1, 0, false)

	frame := tview.NewFrame(container).SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true).SetTitle(
		fmt.Sprintf(" %s ", renderTikiIDGradient(tk.ID(), roles)),
	).SetBorderColor(roles.BorderIdle().TCell())
	frame.SetBorderPadding(1, 0, 2, 2)
	return frame
}

// buildAnchorPrimitivesForSpec produces one tview.Primitive per anchor in the
// given spec, indexed by anchor position. Two cases:
//
//  1. Literal anchor: renders as a static text view carrying the caption
//     text declared by the layout author.
//  2. Field anchor: read-only renderer (with optional role color for text
//     fields), or (in edit mode + focused + editable) a cached editor widget.
func (cv *ConfigurableDetailView) buildAnchorPrimitivesForSpec(spec gridlayout.GridSpec, tk *tikipkg.Tiki, ctx FieldRenderContext) []tview.Primitive {
	focusedName := cv.GetFocusedFieldName()
	primitives := make([]tview.Primitive, len(spec.Anchors))
	for i, a := range spec.Anchors {
		switch a.Kind {
		case gridlayout.AnchorLiteral:
			primitives[i] = renderLiteralAnchor(a, ctx.Roles)
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
		content := compositeSegmentContent(seg, tk, ctx)

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

// compositeSegmentContent returns the plain (untagged) rendered text of a
// single composite segment: the literal text, or the field's rendered value.
// Shared by buildCompositeText (which then wraps each piece in color tags) and
// compositePlainText (which concatenates the pieces for width measurement), so
// the value-resolution logic lives in one place.
func compositeSegmentContent(seg gridlayout.Segment, tk *tikipkg.Tiki, ctx FieldRenderContext) string {
	if seg.Kind == gridlayout.SegmentLiteral {
		return seg.Text
	}
	segCtx := ctx
	segCtx.FieldName = seg.Name
	segCtx.Display = seg.Display
	switch seg.Name {
	case "title":
		return expandFieldText(tk.Title(), ctx.Roles)
	case "id":
		return renderTikiIDGradient(tk.ID(), ctx.Roles)
	}
	if wfd, ok := workflow.Field(seg.Name); ok {
		// inside a composite, an empty/absent list field contributes no value
		// text (not the standalone "—" dash), so a `"label: " + listField` cell
		// reads "label:" rather than "label: —" when the list is empty. The dash
		// is the right placeholder only for a standalone value cell. `.count`
		// already has its own non-dash path in genericFieldValueString.
		if wfd.Type.IsList() && seg.Display != gridlayout.DisplayCount {
			if vals, _, _ := tk.StringSliceField(seg.Name); len(vals) == 0 {
				return ""
			}
		}
		return genericFieldValueString(wfd, tk, segCtx)
	}
	return "—"
}

// compositePlainText concatenates a composite's segment contents with no color
// tags — the wrap-measure source for MeasureAnchor's row-spanned-composite
// branch. Tags would inflate word widths (a leading [#hex] glues onto the
// first word), so measurement runs against this untagged form.
func compositePlainText(a gridlayout.Anchor, tk *tikipkg.Tiki, ctx FieldRenderContext) string {
	var buf strings.Builder
	for _, seg := range a.Segments {
		buf.WriteString(compositeSegmentContent(seg, tk, ctx))
	}
	return buf.String()
}

func renderCompositePrimitive(a gridlayout.Anchor, tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	text := buildCompositeText(a, tk, ctx)
	// row-spanned composites become prose blocks: enable word-wrap so a
	// multi-segment paragraph (e.g. coloured `<Ctrl-Q>` highlight inside
	// surrounding muted prose) flows across the rows it occupies, mirroring
	// the renderLiteralAnchor RowSpan>1 path. Such a block wraps rather than
	// truncates, so it uses a plain TextView, not the single-line truncator.
	if a.RowSpan > 1 {
		tv := tview.NewTextView().SetDynamicColors(true).SetText(text)
		tv.SetBorderPadding(0, 0, 0, 0)
		tv.SetWrap(true)
		tv.SetWordWrap(true)
		return tv
	}
	// single-row composite: truncate with an ellipsis when it overflows its
	// column instead of hard-clipping mid-token (e.g. "tags: backend, sec…").
	return gridbox.NewTruncatingTextView().SetText(text)
}

// RenderViewModeAnchor produces the read-only primitive for a single
// layout anchor. Used by the tiki box (board/list cards) and by any
// non-edit-mode caller that doesn't need cv's per-instance editor cache.
// Edit-mode rendering stays in ConfigurableDetailView.buildAnchorPrimitivesForSpec,
// which adds focus and editor wiring on top of the view-mode primitive.
//
// Literal anchors are colored via Anchor.Role / Anchor.Modifier (set from the
// layout-string `<role>` prefix). When Role is empty, literals fall back to
// text.primary. There is no inline `<role>...` markup parsing — multi-color
// text is composed via composite ` + ` segments.
func RenderViewModeAnchor(a gridlayout.Anchor, tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	switch a.Kind {
	case gridlayout.AnchorLiteral:
		return renderLiteralAnchor(a, ctx.Roles)
	case gridlayout.AnchorComposite:
		return renderCompositePrimitive(a, tk, ctx)
	}
	if a.Display == gridlayout.DisplayCaption {
		return renderCaptionAnchor(a, ctx.Roles)
	}
	ctx.Display = a.Display
	if a.Name == "title" {
		return RenderTitleText(tk, ctx, a.Role, a.Modifier)
	}
	return renderConfiguredField(a.Name, tk, ctx)
}

// paintLiteralText returns the literal text wrapped in the tview tags that
// realize the requested role + modifier. When role is empty, it falls back to
// text.primary so unmarked literals render in the same color as ordinary body
// text. Unknown roles also fall back to text.primary — the workflow loader's
// role validation gate is the place to reject bad names; rendering should
// never fail closed to a wrong color.
//
// Modified roles route through theme.Paint.PaintString to support gradient
// implementations; bare roles emit a single opening color tag (no trailing
// reset, since the cell holds the color until end of line).
func paintLiteralText(text, role, modifier string, roles *theme.Theme) string {
	if role == "" {
		return roles.TextPrimary().Tag() + text
	}
	if modifier != "" {
		if paint, ok := roles.PaintResolver()(role, modifier); ok {
			return paint.PaintString(text)
		}
	}
	if r, ok := roles.ResolveByName(role); ok {
		return r.Tag() + text
	}
	return roles.TextPrimary().Tag() + text
}

// renderLiteralCaption produces a single-line read-only text primitive for a
// literal anchor. Role/modifier come from the layout-string `<role>` prefix;
// when absent, the caption renders in text.primary. There is no inline role
// markup parsing — authors compose multi-color text via composite ` + `
// segments.
func renderLiteralCaption(text, role, modifier string, roles *theme.Theme) tview.Primitive {
	return gridbox.NewTruncatingTextView().SetText(paintLiteralText(text, role, modifier, roles))
}

// fieldCaptionText resolves the caption text for a `.caption` field anchor.
// Resolution order: an explicit workflow `caption:`, then the typed registry
// Label, then the bare field name. The registry Label is preferred over the
// workflow field's name-fallback so system fields without a declared caption
// (createdBy/createdAt/updatedAt) render their human labels ("Author"/
// "Created"/"Updated") instead of the raw field name. Shared by the renderer
// and the solver's content measurer so a caption column is sized to the text
// it actually shows.
func fieldCaptionText(name string) string {
	if wfd, ok := workflow.Field(name); ok && wfd.Caption != "" {
		return wfd.Caption
	}
	if fd, ok := LookupField(name); ok && fd.Label != "" {
		return fd.Label
	}
	if wfd, ok := workflow.Field(name); ok {
		return wfd.DisplayCaption()
	}
	return name
}

// renderCaptionAnchor renders a field anchor's caption text as a static label —
// the field-owned equivalent of a literal caption cell.
func renderCaptionAnchor(a gridlayout.Anchor, roles *theme.Theme) tview.Primitive {
	return renderLiteralCaption(fieldCaptionText(a.Name), a.Role, a.Modifier, roles)
}

// renderLiteralAnchor dispatches a literal anchor to the appropriate
// primitive: a single-line caption for row-span <= 1, or a word-wrapping
// prose block for row-span > 1. Both paths honor the anchor's Role/Modifier
// with text.primary fallback.
func renderLiteralAnchor(a gridlayout.Anchor, roles *theme.Theme) tview.Primitive {
	if a.RowSpan <= 1 {
		return renderLiteralCaption(a.Text, a.Role, a.Modifier, roles)
	}
	if strings.TrimSpace(a.Text) == "" {
		return renderLiteralCaption(a.Text, a.Role, a.Modifier, roles)
	}
	tv := tview.NewTextView().SetDynamicColors(true).SetText(paintLiteralText(a.Text, a.Role, a.Modifier, roles))
	tv.SetWrap(true)
	tv.SetWordWrap(true)
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
	if a.Display == gridlayout.DisplayCaption {
		return renderCaptionAnchor(a, ctx.Roles)
	}
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
	desc := defaultString(tk.Body(), "(No description)")
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

// descriptionPseudoIdx is the focus index used for the inline description
// editor. It sits one past the last metadata anchor (cv.layout) so the
// existing forward/backward traversal arithmetic ("editable indices in
// 0..len(layout)") extends naturally — len(layout) means "description".
const descriptionFieldName = "description"

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
	cv.setFocusedIdx(first)
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

// EnterEditModeWithFocus enters edit mode and focuses the named field.
// When focusField is empty, behaves identically to EnterEditMode (focus
// lands on the first editable layout field). When the requested field
// is not present in the layout or has no implemented editor, focus
// falls back to the EnterEditMode default.
func (cv *ConfigurableDetailView) EnterEditModeWithFocus(focusField model.EditField) bool {
	if !cv.EnterEditMode() {
		return false
	}
	if focusField == "" {
		return true
	}
	idx := cv.indexOfEditableField(string(focusField))
	if idx >= 0 {
		cv.setFocusedIdx(idx)
		cv.refresh()
	}
	return true
}

// indexOfEditableField returns the layout position of the named field
// when it is present and editable, or -1. The synthetic "description"
// field lives at len(layout) so callers can land focus on the inline
// description editor via the same focus index machinery used for
// metadata-grid anchors.
func (cv *ConfigurableDetailView) indexOfEditableField(name string) int {
	if name == descriptionFieldName {
		return len(cv.layout)
	}
	for i, layoutName := range cv.layout {
		if layoutName == name && cv.isEditableLayoutPosition(i) {
			return i
		}
	}
	return -1
}

// ExitEditMode flips the view back to read-only and discards any cached
// editor widgets. The controller is responsible for committing or
// cancelling the underlying edit session before calling this.
func (cv *ConfigurableDetailView) ExitEditMode() {
	if !cv.editMode {
		return
	}
	cv.editMode = false
	cv.setFocusedIdx(-1)
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
	cv.setFocusedIdx(next)
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
	cv.setFocusedIdx(prev)
	cv.refresh()
	return true
}

// GetFocusedFieldName returns the metadata name of the currently focused
// editable field, the synthetic "description" name when the inline
// description editor holds focus, or empty string when not in edit mode.
func (cv *ConfigurableDetailView) GetFocusedFieldName() string {
	if !cv.editMode || cv.focusedIdx < 0 {
		return ""
	}
	if cv.focusedIdx == len(cv.layout) {
		return descriptionFieldName
	}
	if cv.focusedIdx >= len(cv.layout) {
		return ""
	}
	return cv.layout[cv.focusedIdx]
}

// Layout returns the configured layout anchor list. Exposed so callers
// that need to introspect the metadata field set (e.g. edit-mode field
// traversal) can read it without re-parsing the workflow.
func (cv *ConfigurableDetailView) Layout() []string {
	return cv.layout
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

// isEditableLayoutPosition reports whether the layout position i is an
// edit-traversal stop. It is the position-aware companion to
// isEditableLayoutField: in addition to the field being editable, the anchor
// at i must render the field's value, not its caption or item count.
// Display-only anchors (DisplayCaption, DisplayCount) carry a field name but
// are read-only, so a field that has both such a cell and a value cell appears
// twice in layout — only the value position is a stop.
func (cv *ConfigurableDetailView) isEditableLayoutPosition(i int) bool {
	if i < 0 || i >= len(cv.layout) {
		return false
	}
	if i < len(cv.layoutDisplays) && cv.layoutDisplays[i].IsSingleLineDisplay() {
		return false
	}
	return cv.isEditableLayoutField(cv.layout[i])
}

// firstEditableIndex returns the metadata position of the first field
// with an implemented editor and a traversable descriptor, or -1.
func (cv *ConfigurableDetailView) firstEditableIndex() int {
	for i := range cv.layout {
		if cv.isEditableLayoutPosition(i) {
			return i
		}
	}
	return -1
}

// nextEditableIndex returns the next editable position (after current),
// or -1. The traversal walks metadata anchors in column-major order and
// then lands on the synthetic description editor at len(layout) before
// reporting "no further field" — so Tab from the last metadata field
// takes the user into the description body, matching the contract in
// CLAUDE.md ("Tab cycles through all metadata fields → Description").
func (cv *ConfigurableDetailView) nextEditableIndex(current int) int {
	for i := current + 1; i < len(cv.layout); i++ {
		if cv.isEditableLayoutPosition(i) {
			return i
		}
	}
	if current < len(cv.layout) {
		return len(cv.layout)
	}
	return -1
}

// prevEditableIndex returns the previous editable position (before
// current), or -1. From the description pseudo-index it returns the last
// editable metadata field; from a metadata field it walks backwards.
func (cv *ConfigurableDetailView) prevEditableIndex(current int) int {
	if current > len(cv.layout) {
		current = len(cv.layout)
	}
	for i := current - 1; i >= 0; i-- {
		if cv.isEditableLayoutPosition(i) {
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
// text. Iterate cv.layout (column-major order), then flush recurrence
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
	for _, name := range cv.layout {
		if name == tikipkg.FieldRecurrence {
			continue
		}
		flush(name)
	}
	// Second pass: recurrence last, so its Due side-effect overwrites
	// any stale Due that flushed in the first pass.
	flush(tikipkg.FieldRecurrence)
	// Description lives outside the metadata layout (it's the body section
	// edited inline when Tab lands past the last metadata field), so it's
	// not covered by the layout walk above. Flush it here so a Tab-away
	// from the description editor pushes the typed body into the edit
	// session before any commit or save.
	flush(descriptionFieldName)
}

// MoveRecurrencePartLeft moves the recurrence editor's part cursor to
// the frequency part. Returns true when the focused field is recurrence,
// the editor exists, and is a recurrenceEditAdapter; false otherwise.
func (cv *ConfigurableDetailView) MoveRecurrencePartLeft() bool {
	re, ok := cv.focusedRecurrenceEditor()
	if !ok {
		return false
	}
	re.MovePartLeft()
	return true
}

// MoveRecurrencePartRight moves the recurrence editor's part cursor to
// the value part. Returns true when the focused field is recurrence,
// the editor exists, and is a recurrenceEditAdapter; false otherwise.
func (cv *ConfigurableDetailView) MoveRecurrencePartRight() bool {
	re, ok := cv.focusedRecurrenceEditor()
	if !ok {
		return false
	}
	re.MovePartRight()
	return true
}

// IsRecurrenceValueFocused reports whether the recurrence field's value
// part is currently active. Returns false when the focused field is not
// recurrence or the editor isn't a recurrenceEditAdapter.
func (cv *ConfigurableDetailView) IsRecurrenceValueFocused() bool {
	re, ok := cv.focusedRecurrenceEditor()
	if !ok {
		return false
	}
	return re.IsValueFocused()
}

// focusedRecurrenceEditor returns the recurrence editor adapter when the
// recurrence field is focused and its editor is cached as the expected
// adapter type. Centralizes the focus-and-cast guards used by the three
// RecurrencePartNavigable methods.
func (cv *ConfigurableDetailView) focusedRecurrenceEditor() (*recurrenceEditAdapter, bool) {
	if cv.GetFocusedFieldName() != tikipkg.FieldRecurrence {
		return nil, false
	}
	w, ok := cv.editors[tikipkg.FieldRecurrence]
	if !ok || w == nil {
		return nil, false
	}
	re, ok := w.(*recurrenceEditAdapter)
	if !ok {
		return nil, false
	}
	return re, true
}

// isEditableLayoutField returns true when the named field has an
// implemented editor and is traversable. Built-in fields are gated by
// their static descriptor (ReadOnly/EditTraversable); workflow-declared
// fields without a static descriptor are editable when the field
// registry's FieldHasEditor reports an editor — currently only TypeEnum
// fields qualify, since they're the only workflow-only fields that
// route through a generic editor.
func (cv *ConfigurableDetailView) isEditableLayoutField(name string) bool {
	if cv.isHiddenForCurrentTiki(name) {
		return false
	}
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

// isHiddenForCurrentTiki reports whether name is hidden from the metadata grid
// for the tiki being viewed (a `?`-marked field with no value). Hidden fields
// are skipped in edit-mode traversal too — a field skipped in view mode is
// skipped in edit mode. The tiki source mirrors what specForTiki/buildMetadataBox
// actually render (getTiki prefers the in-flight editing copy in edit mode).
func (cv *ConfigurableDetailView) isHiddenForCurrentTiki(name string) bool {
	tk := cv.getTiki()
	if tk == nil {
		return false
	}
	has := func(n string) bool { return fieldHasValue(tk, n) }
	for _, hidden := range gridlayout.EmptyFieldNames(cv.spec, has) {
		if hidden == name {
			return true
		}
	}
	return false
}
