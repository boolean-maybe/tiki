package taskdetail

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/util/gradient"
	"github.com/boolean-maybe/tiki/workflow"

	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DefaultEditMetadata is the canonical field list TaskEditView shows when
// no workflow-specific list is supplied. Title is first so the edit view
// always starts focused on it. Single source of truth so the factory's
// metadata-precedence resolver can fall back to it without hardcoding the
// slice in two places.
var DefaultEditMetadata = []string{
	"title",
	tikipkg.FieldStatus,
	tikipkg.FieldType,
	tikipkg.FieldPriority,
	tikipkg.FieldPoints,
	tikipkg.FieldAssignee,
	tikipkg.FieldDue,
	tikipkg.FieldRecurrence,
	tikipkg.FieldTags,
}

// TaskEditView renders a task in full edit mode using the same fixed-shape
// metadata grid as ConfigurableDetailView. The metadata field list is
// supplied at construction time by the factory's precedence resolver
// (TaskEditParams.Metadata → workflow Detail-plugin lookup →
// DefaultEditMetadata).
//
// Title is a regular grid field — its position in the Tab order derives
// from its position in the metadata list. Description is appended as the
// last Tab stop after all metadata fields.
type TaskEditView struct {
	Base

	registry *controller.ActionRegistry
	viewID   model.ViewID

	metadata         []string
	editFieldOrder   []model.EditField
	focusedField     model.EditField
	validationErrors []string
	metadataBox      *tview.Frame
	descOnly         bool
	tagsOnly         bool

	titleInput   *tview.InputField
	titleEditing bool
	descTextArea *tview.TextArea
	descEditing  bool
	tagsTextArea *tview.TextArea
	tagsEditing  bool

	// editors caches the per-field FieldEditorWidget so user input survives
	// across refreshes. Keyed by canonical field name (matches the registry).
	editors map[string]FieldEditorWidget

	// typed save callbacks wired by the coordinator (one per editable
	// semantic). The adapter layer in adapterForField translates the
	// registry's string-shaped onChange into these typed signatures.
	onTitleSave      func(string)
	onTitleChange    func(string)
	onTitleCancel    func()
	onDescSave       func(string)
	onDescCancel     func()
	onStatusSave     func(string)
	onTypeSave       func(string)
	onPrioritySave   func(string)
	onAssigneeSave   func(string)
	onPointsSave     func(int)
	onDueSave        func(string)
	onRecurrenceSave func(string)
	onTagsSave       func(string)
	onTagsCancel     func()
	// onWorkflowEnumSave receives (fieldName, canonicalKey) for workflow-
	// declared enum fields that lack a dedicated typed save handler. The
	// coordinator installs this once and it dispatches by field name —
	// this is how custom enums (severity, environment, etc.) flow through
	// the full TaskEditView edit path.
	onWorkflowEnumSave  func(name, canonicalKey string)
	actionChangeHandler func()
}

// Compile-time interface checks
var (
	_ controller.View                 = (*TaskEditView)(nil)
	_ controller.FocusSettable        = (*TaskEditView)(nil)
	_ controller.ActionChangeNotifier = (*TaskEditView)(nil)
)

func (ev *TaskEditView) SetActionChangeHandler(handler func()) {
	ev.actionChangeHandler = handler
}

// NewTaskEditView creates a task edit view bound to the given metadata list.
// The factory resolves the metadata source per the precedence rules and
// passes the result here; an empty slice falls back to DefaultEditMetadata
// so the view always has a non-empty navigation order.
func NewTaskEditView(taskStore store.Store, taskID string, imageManager *navtview.ImageManager, metadata []string) *TaskEditView {
	if len(metadata) == 0 {
		metadata = DefaultEditMetadata
	}
	order := model.MetadataToEditFieldOrder(metadata)
	order = append(order, model.EditFieldDescription)

	ev := &TaskEditView{
		Base: Base{
			taskStore:    taskStore,
			taskID:       taskID,
			imageManager: imageManager,
		},
		registry:       controller.GetActionsForField(model.EditFieldTitle),
		viewID:         model.TaskEditViewID,
		metadata:       metadata,
		editFieldOrder: order,
		focusedField:   order[0],
		titleEditing:   true,
		descEditing:    true,
		editors:        make(map[string]FieldEditorWidget),
	}
	ev.build()

	tk := ev.GetTiki()
	if tk != nil {
		ev.ensureTitleInput(tk)
		ev.ensureDescTextArea(tk)
	}
	ev.refresh()
	return ev
}

// GetTiki returns the appropriate tiki based on mode (prioritizes editing copy)
func (ev *TaskEditView) GetTiki() *tikipkg.Tiki {
	if ev.taskController != nil {
		if draftTiki := ev.taskController.GetDraftTiki(); draftTiki != nil {
			return draftTiki
		}
		if editingTiki := ev.taskController.GetEditingTiki(); editingTiki != nil {
			return editingTiki
		}
	}

	tk := ev.taskStore.GetTiki(ev.taskID)
	if tk == nil && ev.fallbackTiki != nil && ev.fallbackTiki.ID == ev.taskID {
		tk = ev.fallbackTiki
	}
	return tk
}

// GetActionRegistry returns the view's action registry
func (ev *TaskEditView) GetActionRegistry() *controller.ActionRegistry {
	return ev.registry
}

// GetViewID returns the view identifier
func (ev *TaskEditView) GetViewID() model.ViewID {
	return ev.viewID
}

// GetViewName returns the view name for the header info section
func (ev *TaskEditView) GetViewName() string { return model.TaskEditViewName }

// GetViewDescription returns the view description for the header info section
func (ev *TaskEditView) GetViewDescription() string { return model.TaskEditViewDesc }

// Metadata returns the configured metadata field list. Used by callers that
// want to forward the same metadata shape into a downstream view (e.g. the
// configurable detail view exposes this so the input router can copy it
// into TaskEditParams when opening the edit view from a detail context).
func (ev *TaskEditView) Metadata() []string {
	return ev.metadata
}

// SetDescOnly enables description-only edit mode where metadata is read-only.
func (ev *TaskEditView) SetDescOnly(descOnly bool) {
	ev.descOnly = descOnly
	if descOnly {
		ev.focusedField = model.EditFieldDescription
		ev.registry = controller.DescOnlyEditActions()
		ev.refresh()
		if ev.actionChangeHandler != nil {
			ev.actionChangeHandler()
		}
	}
}

// IsDescOnly returns whether the view is in description-only edit mode.
func (ev *TaskEditView) IsDescOnly() bool {
	return ev.descOnly
}

// SetTagsOnly enables tags-only edit mode where metadata is read-only and
// the description area is replaced with a tags textarea.
func (ev *TaskEditView) SetTagsOnly(tagsOnly bool) {
	ev.tagsOnly = tagsOnly
	if tagsOnly {
		ev.registry = controller.TagsOnlyEditActions()
		ev.refresh()
		if ev.actionChangeHandler != nil {
			ev.actionChangeHandler()
		}
	}
}

// IsTagsOnly returns whether the view is in tags-only edit mode.
func (ev *TaskEditView) IsTagsOnly() bool {
	return ev.tagsOnly
}

// OnFocus is called when the view becomes active
func (ev *TaskEditView) OnFocus() {
	ev.refresh()
}

// OnBlur is called when the view becomes inactive
func (ev *TaskEditView) OnBlur() {
	// No listener to clean up in edit mode
}

// refresh re-renders the view
func (ev *TaskEditView) refresh() {
	ev.content.Clear()
	ev.descView = nil

	tk := ev.GetTiki()
	if tk == nil {
		notFound := tview.NewTextView().SetText("Task not found")
		ev.content.AddItem(notFound, 0, 1, false)
		return
	}

	colors := config.GetColors()

	if !ev.fullscreen {
		metadataBox, boxHeight := ev.buildMetadataBox(tk, colors)
		ev.content.AddItem(metadataBox, boxHeight, 0, false)
	}

	if ev.tagsOnly {
		tagsTextArea := ev.ensureTagsTextArea(tk)
		ev.content.AddItem(tagsTextArea, 0, 1, true)
	} else {
		descPrimitive := ev.buildDescription(tk)
		ev.content.AddItem(descPrimitive, 0, 1, true)
	}

	ev.updateValidationState()
}

// buildMetadataBox builds the framed metadata box with the configurable
// grid. Mirrors ConfigurableDetailView's shape — gridded fields turn into
// editors when focused (including title as an InputField).
func (ev *TaskEditView) buildMetadataBox(tk *tikipkg.Tiki, colors *config.ColorConfig) (*tview.Frame, int) {
	mode := RenderModeEdit
	if ev.descOnly || ev.tagsOnly {
		mode = RenderModeView
	}
	ctx := FieldRenderContext{
		Mode:         mode,
		FocusedField: ev.focusedField,
		Colors:       colors,
		Store:        ev.taskStore,
	}

	spec, primitives := ev.buildGridSpecAndPrimitives(tk, ctx)
	heightOf := func(name string, w int) int { return FieldHeight(name, tk, w) }

	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.AddItem(newGridContainer(spec, primitives, heightOf), spec.Rows, 0, false)
	container.AddItem(tview.NewBox(), 1, 0, false)

	frame := tview.NewFrame(container).SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true).SetTitle(
		fmt.Sprintf(" %s ", gradient.RenderAdaptiveGradientText(tk.ID, colors.TaskDetailIDColor, colors.FallbackTaskIDColor)),
	).SetBorderColor(colors.TaskBoxUnselectedBorder.TCell())
	frame.SetBorderPadding(1, 0, 2, 2)
	ev.metadataBox = frame
	return frame, spec.Rows + metadataBoxOverhead
}

// buildGridSpecAndPrimitives mirrors the configurable view's helper but
// returns editor primitives for the focused field (when focus is on a
// metadata field) instead of the read-only render. Editors are cached on
// ev.editors so user input survives across refreshes.
//
// TaskEditView has a flat metadata list rather than a parsed grid; it
// packs the list into columns of rowsPerPackedColumn rows so the
// shared grid container can lay them out without overflowing the
// fixed-height metadata box.
func (ev *TaskEditView) buildGridSpecAndPrimitives(tk *tikipkg.Tiki, ctx FieldRenderContext) (gridlayout.GridSpec, []tview.Primitive) {
	spec := greedyPackedSpec(ev.metadata)
	primitives := make([]tview.Primitive, len(spec.Anchors))
	for i, a := range spec.Anchors {
		primitives[i] = ev.gridFieldPrimitive(a.Name, tk, ctx)
	}
	return spec, primitives
}

// gridFieldPrimitive returns the editor for the focused field when the view
// is in normal edit mode, else the read-only renderer. Editors come from
// the field registry; their string-shaped onChange is bridged to the typed
// save callbacks via adapterForField.
func (ev *TaskEditView) gridFieldPrimitive(name string, tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if name == "title" {
		return ev.titleGridPrimitive(tk, ctx)
	}
	if ev.descOnly || ev.tagsOnly {
		return renderConfiguredField(name, tk, ctx)
	}
	if !FieldHasEditor(name) {
		return renderConfiguredField(name, tk, ctx)
	}
	// Resolve the EditField identity for focus-matching. Built-in fields
	// pull from the static FieldDescriptor; workflow-only fields use the
	// field name directly (the EditField type is a string, and the
	// MetadataToEditFieldOrder helper emits the same shape).
	editField := editFieldFor(name)
	if editField == "" {
		return renderConfiguredField(name, tk, ctx)
	}
	if ev.focusedField != editField {
		return renderConfiguredField(name, tk, ctx)
	}
	if w, ok := ev.editors[name]; ok && w != nil {
		return w
	}
	editorCtx := ctx
	editorCtx.FocusedField = editField
	w := buildFieldEditor(name, tk, editorCtx, ev.adapterForField(name))
	if w == nil {
		return renderConfiguredField(name, tk, ctx)
	}
	ev.editors[name] = w
	return w
}

// editFieldFor returns the EditField identity for a metadata field. Built-in
// fields use their static descriptor; workflow-only TypeEnum fields use
// their field name directly (the EditField type is a string alias, and
// MetadataToEditFieldOrder emits the same shape so focus-matching lines up).
func editFieldFor(name string) model.EditField {
	if fd, ok := LookupField(name); ok {
		return fd.EditField
	}
	if wfd, ok := workflow.Field(name); ok && wfd.Type == workflow.TypeEnum {
		return model.EditField(name)
	}
	return ""
}

func (ev *TaskEditView) titleGridPrimitive(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if ev.descOnly || ev.tagsOnly {
		return RenderTitleText(tk, ctx, "")
	}
	input := ev.ensureTitleInput(tk)
	if ev.focusedField == model.EditFieldTitle {
		input.SetLabel(getFocusMarker(ctx.Colors))
	} else {
		input.SetLabel("")
	}
	return input
}

// adapterForField returns the registry's onChange(string) callback that
// parses the string back to the typed value and invokes TaskEditView's
// typed save handler. Lives here (not on the registry) so the registry's
// editor factories can stay view-agnostic; the adapter encodes the
// view-specific typed-handler chain.
func (ev *TaskEditView) adapterForField(name string) func(string) {
	switch name {
	case tikipkg.FieldStatus:
		return func(d string) {
			if ev.onStatusSave != nil {
				ev.onStatusSave(d)
			}
			ev.updateValidationState()
		}
	case tikipkg.FieldType:
		return func(d string) {
			if ev.onTypeSave != nil {
				ev.onTypeSave(d)
			}
			ev.updateValidationState()
		}
	case tikipkg.FieldPriority:
		return func(d string) {
			if ev.onPrioritySave != nil {
				// SemanticEnum editor emits the canonical key directly.
				// No display→key conversion needed at this boundary.
				ev.onPrioritySave(d)
			}
			ev.updateValidationState()
		}
	case tikipkg.FieldPoints:
		return func(d string) {
			n, err := strconv.Atoi(d)
			if err != nil {
				return
			}
			ev.updateValidationState()
			if ev.onPointsSave != nil {
				ev.onPointsSave(n)
			}
		}
	case tikipkg.FieldAssignee:
		return func(d string) {
			if ev.onAssigneeSave != nil {
				ev.onAssigneeSave(d)
			}
			ev.updateValidationState()
		}
	case tikipkg.FieldDue:
		return func(d string) {
			ev.updateValidationState()
			if ev.onDueSave != nil {
				ev.onDueSave(d)
			}
		}
	case tikipkg.FieldRecurrence:
		return func(d string) {
			if ev.onRecurrenceSave != nil {
				ev.onRecurrenceSave(d)
			}
			// preserve recurrence-affects-due side effect: refresh due editor
			// and re-validate.
			ev.syncDueFromTask()
			ev.refresh()
			ev.updateValidationState()
		}
	case tikipkg.FieldTags:
		return func(d string) {
			if ev.onTagsSave != nil {
				ev.onTagsSave(d)
			}
		}
	}
	// Fallback for workflow-declared enum fields (severity, environment, ...)
	// that don't have a dedicated typed save handler. The coordinator
	// wires onWorkflowEnumSave to dispatch by field name.
	if wfd, ok := workflow.Field(name); ok && wfd.Type == workflow.TypeEnum {
		field := name // capture for closure
		return func(canonicalKey string) {
			if ev.onWorkflowEnumSave != nil {
				ev.onWorkflowEnumSave(field, canonicalKey)
			}
			ev.updateValidationState()
		}
	}
	return nil
}

// SaveTagsFromTextArea reads the focused tags grid editor and fires its
// wired onTagsSave handler. Called by the coordinator's handleSaveKey when
// Ctrl+S lands on a focused tags field in non-tagsOnly mode — drives the
// CommitNoClose path the coordinator wired in Prepare.
func (ev *TaskEditView) SaveTagsFromTextArea() {
	w, ok := ev.editors[tikipkg.FieldTags]
	if !ok || w == nil {
		return
	}
	if ev.onTagsSave != nil {
		ev.onTagsSave(w.GetText())
	}
}

// SaveDescriptionFromTextArea reads the description textarea and fires the
// wired onDescSave. Counterpart to SaveTagsFromTextArea for descriptions.
func (ev *TaskEditView) SaveDescriptionFromTextArea() {
	if ev.descTextArea == nil {
		return
	}
	if ev.onDescSave != nil {
		ev.onDescSave(ev.descTextArea.GetText())
	}
}

// isDueReadOnly returns true when recurrence is set, making Due auto-computed.
func (ev *TaskEditView) isDueReadOnly() bool {
	tk := ev.GetTiki()
	if tk == nil {
		return false
	}
	recurrenceStr, _, _ := tk.StringField(tikipkg.FieldRecurrence)
	return recurrenceStr != "" && recurrenceStr != "none"
}

// syncDueFromTask updates the cached due editor (if any) to reflect the
// auto-computed Due from the in-memory tiki. Called after recurrence
// changes.
func (ev *TaskEditView) syncDueFromTask() {
	w, ok := ev.editors[tikipkg.FieldDue]
	if !ok || w == nil {
		return
	}
	// the underlying editor is dateEditAdapter; invalidate it so a fresh
	// initial value is read on next render.
	delete(ev.editors, tikipkg.FieldDue)
}

func (ev *TaskEditView) buildDescription(tk *tikipkg.Tiki) tview.Primitive {
	textArea := ev.ensureDescTextArea(tk)
	return textArea
}

func (ev *TaskEditView) ensureDescTextArea(tk *tikipkg.Tiki) *tview.TextArea {
	if ev.descTextArea == nil {
		ev.descTextArea = tview.NewTextArea()
		ev.descTextArea.SetBorder(false)
		ev.descTextArea.SetBorderPadding(1, 1, 2, 2)

		ev.descTextArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			key := event.Key()
			if key == tcell.KeyCtrlS {
				if ev.onDescSave != nil {
					ev.onDescSave(ev.descTextArea.GetText())
				}
				return nil
			}
			if key == tcell.KeyEscape {
				if ev.onDescCancel != nil {
					ev.onDescCancel()
				}
				return nil
			}
			return event
		})

		ev.descTextArea.SetText(tk.Body, false)
	} else if !ev.descEditing {
		ev.descTextArea.SetText(tk.Body, false)
	}

	ev.descEditing = true
	return ev.descTextArea
}

func (ev *TaskEditView) ensureTagsTextArea(tk *tikipkg.Tiki) *tview.TextArea {
	if ev.tagsTextArea == nil {
		ev.tagsTextArea = tview.NewTextArea()
		ev.tagsTextArea.SetBorder(false)
		ev.tagsTextArea.SetBorderPadding(1, 1, 2, 2)
		ev.tagsTextArea.SetPlaceholder("Enter tags separated by spaces")
		ev.tagsTextArea.SetPlaceholderStyle(tcell.StyleDefault.Foreground(config.GetColors().TaskDetailPlaceholderColor.TCell()))

		ev.tagsTextArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyCtrlS {
				if ev.onTagsSave != nil {
					ev.onTagsSave(ev.tagsTextArea.GetText())
				}
				return nil
			}
			if event.Key() == tcell.KeyEscape {
				if ev.onTagsCancel != nil {
					ev.onTagsCancel()
				}
				return nil
			}
			return event
		})

		tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
		ev.tagsTextArea.SetText(strings.Join(tags, " "), false)
	} else if !ev.tagsEditing {
		tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
		ev.tagsTextArea.SetText(strings.Join(tags, " "), false)
	}

	ev.tagsEditing = true
	return ev.tagsTextArea
}

// GetEditedTags returns the current tags from the active tags editor.
// In tags-only mode the source is the dedicated tagsTextArea; in grid mode
// the source is the cached registry editor (whitespace-joined string).
func (ev *TaskEditView) GetEditedTags() []string {
	if ev.tagsOnly && ev.tagsTextArea != nil {
		return strings.Fields(ev.tagsTextArea.GetText())
	}
	if w, ok := ev.editors[tikipkg.FieldTags]; ok && w != nil {
		return strings.Fields(w.GetText())
	}
	tk := ev.GetTiki()
	if tk != nil {
		tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
		return tags
	}
	return nil
}

// ShowTagsEditor displays the tags text area (tags-only mode) and returns
// the primitive to focus.
func (ev *TaskEditView) ShowTagsEditor() tview.Primitive {
	tk := ev.GetTiki()
	if tk == nil {
		return nil
	}
	return ev.ensureTagsTextArea(tk)
}

// IsTagsTextAreaFocused returns whether the tags text area currently has focus.
// Used by the input router to route Ctrl+S/Esc through the coordinator. In
// tags-only mode, the source is the dedicated tagsTextArea; in grid mode,
// the source is the cached registry editor under tikipkg.FieldTags.
func (ev *TaskEditView) IsTagsTextAreaFocused() bool {
	if ev.tagsOnly && ev.tagsTextArea != nil && ev.tagsTextArea.HasFocus() {
		return true
	}
	if w, ok := ev.editors[tikipkg.FieldTags]; ok && w != nil && w.HasFocus() {
		return true
	}
	return false
}

// SetTagsSaveHandler sets the callback for when tags are saved
func (ev *TaskEditView) SetTagsSaveHandler(handler func(string)) {
	ev.onTagsSave = handler
}

// SetTagsCancelHandler sets the callback for when tags editing is cancelled
func (ev *TaskEditView) SetTagsCancelHandler(handler func()) {
	ev.onTagsCancel = handler
}

func (ev *TaskEditView) ensureTitleInput(tk *tikipkg.Tiki) *tview.InputField {
	if ev.titleInput == nil {
		colors := config.GetColors()
		ev.titleInput = tview.NewInputField()
		ev.titleInput.SetFieldBackgroundColor(colors.ContentBackgroundColor.TCell())
		ev.titleInput.SetFieldTextColor(colors.InputFieldTextColor.TCell())
		ev.titleInput.SetBorder(false)

		ev.titleInput.SetChangedFunc(func(text string) {
			if ev.onTitleChange != nil {
				ev.onTitleChange(text)
			}
			ev.updateValidationState()
		})

		ev.titleInput.SetDoneFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyEnter:
				if ev.onTitleSave != nil {
					ev.onTitleSave(ev.titleInput.GetText())
				}
			case tcell.KeyEscape:
				if ev.onTitleCancel != nil {
					ev.onTitleCancel()
				}
			}
		})

		ev.titleInput.SetText(tk.Title)
	} else if !ev.titleEditing {
		ev.titleInput.SetText(tk.Title)
	}

	ev.titleEditing = true
	return ev.titleInput
}

// updateValidationState runs validation checks and updates the border color.
// Builds a tiki snapshot from current widget values and validates each field.
func (ev *TaskEditView) updateValidationState() {
	snapshot := ev.buildTikiSnapshotFromWidgets()
	if snapshot == nil {
		return
	}

	ev.validationErrors = nil
	for _, fn := range service.AllTikiValidators() {
		if msg := fn(snapshot); msg != "" {
			ev.validationErrors = append(ev.validationErrors, msg)
		}
	}

	if ev.metadataBox != nil {
		colors := config.GetColors()
		if len(ev.validationErrors) > 0 {
			ev.metadataBox.SetBorderColor(colors.TaskBoxSelectedBorder.TCell())
		} else {
			ev.metadataBox.SetBorderColor(colors.TaskBoxUnselectedBorder.TCell())
		}
	}
}

// buildTikiSnapshotFromWidgets creates a tiki snapshot from current widget
// values for validation purposes — overlaying live widget state onto the
// stored tiki. Reads from registry editors (cached on ev.editors) where
// available, falls back to the persisted values otherwise.
func (ev *TaskEditView) buildTikiSnapshotFromWidgets() *tikipkg.Tiki {
	tk := ev.GetTiki()
	if tk == nil {
		return nil
	}
	snapshot := tk.Clone()

	if ev.titleInput != nil {
		snapshot.Title = ev.titleInput.GetText()
	}
	if w, ok := ev.editors[tikipkg.FieldPriority]; ok && w != nil {
		// enumSelectAdapter.GetText() returns the canonical key (e.g.
		// "high"), not the display string. Empty means the editor's
		// reverse-lookup couldn't match — drop the field rather than
		// writing junk into the validated snapshot.
		key := w.GetText()
		if key != "" {
			if fd, ok := workflow.Field(tikipkg.FieldPriority); ok && fd.IsValidEnum(key) {
				snapshot.Set(tikipkg.FieldPriority, key)
			} else {
				snapshot.Delete(tikipkg.FieldPriority)
			}
		} else {
			snapshot.Delete(tikipkg.FieldPriority)
		}
	}
	if w, ok := ev.editors[tikipkg.FieldPoints]; ok && w != nil {
		// Points is a workflow enum (declared in kanban.yaml with values
		// like "11"/"7"/"3"/"1"). enumSelectAdapter.GetText() returns the
		// canonical key after reverse-lookup; an empty key means the user
		// cleared the field or the lookup failed.
		key := w.GetText()
		if key != "" {
			if fd, ok := workflow.Field(tikipkg.FieldPoints); ok && fd.IsValidEnum(key) {
				snapshot.Set(tikipkg.FieldPoints, key)
			} else {
				snapshot.Delete(tikipkg.FieldPoints)
			}
		} else {
			snapshot.Delete(tikipkg.FieldPoints)
		}
	}
	if w, ok := ev.editors[tikipkg.FieldDue]; ok && w != nil {
		if parsed, ok := parseDueDateText(w.GetText()); ok {
			if parsed.IsZero() {
				snapshot.Delete(tikipkg.FieldDue)
			} else {
				snapshot.Set(tikipkg.FieldDue, parsed)
			}
		}
	}
	if w, ok := ev.editors[tikipkg.FieldRecurrence]; ok && w != nil {
		r := w.GetText()
		if r == "" || r == "none" {
			snapshot.Delete(tikipkg.FieldRecurrence)
		} else {
			snapshot.Set(tikipkg.FieldRecurrence, r)
		}
	}
	return snapshot
}

// EnterFullscreen switches the view to fullscreen mode
func (ev *TaskEditView) EnterFullscreen() {
	if ev.fullscreen {
		return
	}
	ev.fullscreen = true
	ev.refresh()
	if ev.focusSetter != nil && ev.descTextArea != nil {
		ev.focusSetter(ev.descTextArea)
	}
	if ev.onFullscreenChange != nil {
		ev.onFullscreenChange(true)
	}
}

// ExitFullscreen restores the regular task detail layout
func (ev *TaskEditView) ExitFullscreen() {
	if !ev.fullscreen {
		return
	}
	ev.fullscreen = false
	ev.refresh()
	if ev.focusSetter != nil && ev.descTextArea != nil {
		ev.focusSetter(ev.descTextArea)
	}
	if ev.onFullscreenChange != nil {
		ev.onFullscreenChange(false)
	}
}

// ShowTitleEditor displays the title input field
func (ev *TaskEditView) ShowTitleEditor() tview.Primitive {
	tk := ev.GetTiki()
	if tk == nil {
		return nil
	}
	return ev.ensureTitleInput(tk)
}

// HideTitleEditor is a no-op in edit mode (title always visible)
func (ev *TaskEditView) HideTitleEditor() {}

// IsTitleEditing returns whether the title is being edited (always true in edit mode)
func (ev *TaskEditView) IsTitleEditing() bool {
	return ev.titleEditing
}

// IsTitleInputFocused returns whether the title input has focus
func (ev *TaskEditView) IsTitleInputFocused() bool {
	return ev.titleEditing && ev.titleInput != nil && ev.titleInput.HasFocus()
}

// SetTitleSaveHandler sets the callback for when title is saved
func (ev *TaskEditView) SetTitleSaveHandler(handler func(string)) {
	ev.onTitleSave = handler
}

// SetTitleChangeHandler sets the callback for when title changes
func (ev *TaskEditView) SetTitleChangeHandler(handler func(string)) {
	ev.onTitleChange = handler
}

// SetTitleCancelHandler sets the callback for when title editing is cancelled
func (ev *TaskEditView) SetTitleCancelHandler(handler func()) {
	ev.onTitleCancel = handler
}

// ShowDescriptionEditor displays the description text area
func (ev *TaskEditView) ShowDescriptionEditor() tview.Primitive {
	tk := ev.GetTiki()
	if tk == nil {
		return nil
	}
	return ev.ensureDescTextArea(tk)
}

// HideDescriptionEditor is a no-op in edit mode
func (ev *TaskEditView) HideDescriptionEditor() {}

// IsDescriptionEditing returns whether the description is being edited
func (ev *TaskEditView) IsDescriptionEditing() bool {
	return ev.descEditing
}

// IsDescriptionTextAreaFocused returns whether the description text area has focus
func (ev *TaskEditView) IsDescriptionTextAreaFocused() bool {
	return ev.descEditing && ev.descTextArea != nil && ev.descTextArea.HasFocus()
}

// SetDescriptionSaveHandler sets the callback for when description is saved
func (ev *TaskEditView) SetDescriptionSaveHandler(handler func(string)) {
	ev.onDescSave = handler
}

// SetDescriptionCancelHandler sets the callback for when description editing is cancelled
func (ev *TaskEditView) SetDescriptionCancelHandler(handler func()) {
	ev.onDescCancel = handler
}

// GetEditedTitle returns the current title in the editor
func (ev *TaskEditView) GetEditedTitle() string {
	if ev.titleInput != nil {
		return ev.titleInput.GetText()
	}
	tk := ev.GetTiki()
	if tk == nil {
		return ""
	}
	return tk.Title
}

// GetEditedDescription returns the current description text in the editor
func (ev *TaskEditView) GetEditedDescription() string {
	if ev.descTextArea != nil {
		return ev.descTextArea.GetText()
	}
	tk := ev.GetTiki()
	if tk == nil {
		return ""
	}
	return tk.Body
}

// SetStatusSaveHandler sets the callback for when status is saved
func (ev *TaskEditView) SetStatusSaveHandler(handler func(string)) {
	ev.onStatusSave = handler
}

// SetTypeSaveHandler sets the callback for when type is saved
func (ev *TaskEditView) SetTypeSaveHandler(handler func(string)) {
	ev.onTypeSave = handler
}

// SetPrioritySaveHandler sets the callback for when priority is saved.
// The handler receives the canonical enum key (or empty string for delete).
func (ev *TaskEditView) SetPrioritySaveHandler(handler func(string)) {
	ev.onPrioritySave = handler
}

// SetWorkflowEnumSaveHandler installs the dispatcher for custom workflow
// enum saves — fields like severity / environment that don't have their
// own typed save handler. The handler receives (fieldName, canonicalKey).
func (ev *TaskEditView) SetWorkflowEnumSaveHandler(handler func(name, canonicalKey string)) {
	ev.onWorkflowEnumSave = handler
}

// SetAssigneeSaveHandler sets the callback for when assignee is saved
func (ev *TaskEditView) SetAssigneeSaveHandler(handler func(string)) {
	ev.onAssigneeSave = handler
}

// SetPointsSaveHandler sets the callback for when story points is saved
func (ev *TaskEditView) SetPointsSaveHandler(handler func(int)) {
	ev.onPointsSave = handler
}

// SetDueSaveHandler sets the callback for when due date is saved
func (ev *TaskEditView) SetDueSaveHandler(handler func(string)) {
	ev.onDueSave = handler
}

// SetRecurrenceSaveHandler sets the callback for when recurrence is saved
func (ev *TaskEditView) SetRecurrenceSaveHandler(handler func(string)) {
	ev.onRecurrenceSave = handler
}
