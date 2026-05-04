package taskdetail

import (
	"strings"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TaskEditView renders a task in full edit mode with all fields editable.
type TaskEditView struct {
	Base // Embed shared state

	registry *controller.ActionRegistry
	viewID   model.ViewID

	// Edit state
	focusedField     model.EditField
	validationErrors []string
	metadataBox      *tview.Frame
	descOnly         bool
	tagsOnly         bool

	// All field editors
	titleInput         *tview.InputField
	titleEditing       bool
	descTextArea       *tview.TextArea
	descEditing        bool
	statusSelectList   *component.EditSelectList
	typeSelectList     *component.EditSelectList
	prioritySelectList *component.EditSelectList
	assigneeSelectList *component.EditSelectList
	pointsInput        *component.IntEditSelect
	dueInput           *component.DateEdit
	recurrenceInput    *component.RecurrenceEdit
	tagsTextArea       *tview.TextArea
	tagsEditing        bool

	// All callbacks
	onTitleSave         func(string)
	onTitleChange       func(string)
	onTitleCancel       func()
	onDescSave          func(string)
	onDescCancel        func()
	onStatusSave        func(string)
	onTypeSave          func(string)
	onPrioritySave      func(int)
	onAssigneeSave      func(string)
	onPointsSave        func(int)
	onDueSave           func(string)
	onRecurrenceSave    func(string)
	onTagsSave          func(string)
	onTagsCancel        func()
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

// NewTaskEditView creates a task edit view
func NewTaskEditView(taskStore store.Store, taskID string, imageManager *navtview.ImageManager) *TaskEditView {
	ev := &TaskEditView{
		Base: Base{
			taskStore:    taskStore,
			taskID:       taskID,
			imageManager: imageManager,
		},
		registry:     controller.GetActionsForField(model.EditFieldTitle),
		viewID:       model.TaskEditViewID,
		focusedField: model.EditFieldTitle,
		titleEditing: true,
		descEditing:  true,
	}

	ev.build()

	// Eagerly create all edit field widgets to ensure they exist before focus management
	tk := ev.GetTiki()
	if tk != nil {
		ev.ensureTitleInput(tk)
		ev.ensureDescTextArea(tk)
		ev.ensureStatusSelectList(tk)
		ev.ensureTypeSelectList(tk)
		ev.ensurePrioritySelectList(tk)
		ev.ensureAssigneeSelectList(tk)
		ev.ensurePointsInput(tk)
		ev.ensureDueInput(tk)
		ev.ensureRecurrenceInput(tk)
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
		metadataBox := ev.buildMetadataBox(tk, colors)
		ev.content.AddItem(metadataBox, 10, 0, false)
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

func (ev *TaskEditView) buildMetadataBox(tk *tikipkg.Tiki, colors *config.ColorConfig) *tview.Frame {
	mode := RenderModeEdit
	if ev.descOnly || ev.tagsOnly {
		mode = RenderModeView
	}
	ctx := FieldRenderContext{
		Mode:         mode,
		FocusedField: ev.focusedField,
		Colors:       colors,
	}
	titlePrimitive := ev.buildTitlePrimitive(tk, colors)
	col1, col2, col3 := ev.buildMetadataColumns(tk, ctx, colors)
	metadataBox := ev.assembleMetadataBox(tk, colors, titlePrimitive, col1, col2, col3, 1)
	ev.metadataBox = metadataBox
	return metadataBox
}

func (ev *TaskEditView) buildTitlePrimitive(tk *tikipkg.Tiki, colors *config.ColorConfig) tview.Primitive {
	if ev.descOnly || ev.tagsOnly {
		ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}
		return RenderTitleText(tk, ctx)
	}
	input := ev.ensureTitleInput(tk)
	focused := ev.focusedField == model.EditFieldTitle
	if focused {
		input.SetLabel(getFocusMarker(colors))
	} else {
		input.SetLabel("")
	}
	return input
}

func (ev *TaskEditView) buildMetadataColumns(tk *tikipkg.Tiki, ctx FieldRenderContext, colors *config.ColorConfig) (*tview.Flex, *tview.Flex, *tview.Flex) {
	// Column 1: Status, Type, Priority, Points
	col1 := tview.NewFlex().SetDirection(tview.FlexRow)
	col1.AddItem(ev.buildStatusField(tk, ctx), 1, 0, false)
	col1.AddItem(ev.buildTypeField(tk, ctx), 1, 0, false)
	col1.AddItem(ev.buildPriorityField(tk, ctx), 1, 0, false)
	col1.AddItem(ev.buildPointsField(tk, ctx), 1, 0, false)

	// Column 2: Assignee, Author, Created, Updated
	col2 := tview.NewFlex().SetDirection(tview.FlexRow)
	col2.AddItem(ev.buildAssigneeField(tk, ctx), 1, 0, false)
	col2.AddItem(RenderAuthorText(tk, colors), 1, 0, false)
	col2.AddItem(RenderCreatedText(tk, colors), 1, 0, false)
	col2.AddItem(RenderUpdatedText(tk, colors), 1, 0, false)

	// Column 3: Due, Recurrence
	col3 := tview.NewFlex().SetDirection(tview.FlexRow)
	col3.AddItem(ev.buildDueField(tk, ctx), 1, 0, false)
	col3.AddItem(ev.buildRecurrenceField(tk, ctx), 1, 0, false)

	return col1, col2, col3
}

func (ev *TaskEditView) buildStatusField(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if ctx.FocusedField == model.EditFieldStatus {
		return ev.ensureStatusSelectList(tk)
	}
	return RenderStatusText(tk, ctx)
}

func (ev *TaskEditView) buildTypeField(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if ctx.FocusedField == model.EditFieldType {
		return ev.ensureTypeSelectList(tk)
	}
	return RenderTypeText(tk, ctx)
}

func (ev *TaskEditView) buildPriorityField(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if ctx.FocusedField == model.EditFieldPriority {
		return ev.ensurePrioritySelectList(tk)
	}
	return RenderPriorityText(tk, ctx)
}

func (ev *TaskEditView) buildAssigneeField(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if ctx.FocusedField == model.EditFieldAssignee {
		return ev.ensureAssigneeSelectList(tk)
	}
	return RenderAssigneeText(tk, ctx)
}

func (ev *TaskEditView) buildPointsField(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if ctx.FocusedField == model.EditFieldPoints {
		return ev.ensurePointsInput(tk)
	}
	return RenderPointsText(tk, ctx)
}

func (ev *TaskEditView) buildDueField(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if ev.isDueReadOnly() {
		return RenderDueText(tk, ctx.Colors)
	}
	if ctx.FocusedField == model.EditFieldDue {
		return ev.ensureDueInput(tk)
	}
	return RenderDueText(tk, ctx.Colors)
}

// isDueReadOnly returns true when recurrence is set, making Due auto-computed.
func (ev *TaskEditView) isDueReadOnly() bool {
	tk := ev.GetTiki()
	if tk == nil {
		return false
	}
	recurrenceStr, _, _ := tk.StringField(tikipkg.FieldRecurrence)
	return recurrenceStr != "" && taskpkg.Recurrence(recurrenceStr) != taskpkg.RecurrenceNone
}

// syncDueFromTask updates the dueInput widget to reflect the auto-computed Due
// from the in-memory tiki. Called after recurrence changes.
func (ev *TaskEditView) syncDueFromTask() {
	if ev.dueInput == nil {
		return
	}
	tk := ev.GetTiki()
	if tk == nil {
		return
	}
	due, _, _ := tk.TimeField(tikipkg.FieldDue)
	var dateStr string
	if !due.IsZero() {
		dateStr = due.Format(taskpkg.DateFormat)
	}
	ev.dueInput.SetInitialValue(dateStr)
}

func (ev *TaskEditView) buildRecurrenceField(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if ctx.FocusedField == model.EditFieldRecurrence {
		return ev.ensureRecurrenceInput(tk)
	}
	return RenderRecurrenceText(tk, ctx.Colors)
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

// GetEditedTags returns the current tags from the tags editor, split by whitespace.
func (ev *TaskEditView) GetEditedTags() []string {
	if ev.tagsTextArea == nil {
		tk := ev.GetTiki()
		if tk != nil {
			tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
			return tags
		}
		return nil
	}
	return strings.Fields(ev.tagsTextArea.GetText())
}

// ShowTagsEditor displays the tags text area and returns the primitive to focus
func (ev *TaskEditView) ShowTagsEditor() tview.Primitive {
	tk := ev.GetTiki()
	if tk == nil {
		return nil
	}
	return ev.ensureTagsTextArea(tk)
}

// IsTagsTextAreaFocused returns whether the tags text area currently has focus
func (ev *TaskEditView) IsTagsTextAreaFocused() bool {
	return ev.tagsEditing && ev.tagsTextArea != nil && ev.tagsTextArea.HasFocus()
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

// buildTikiSnapshotFromWidgets creates a tiki snapshot from current widget values
// for validation purposes — overlaying live widget state onto the stored tiki.
func (ev *TaskEditView) buildTikiSnapshotFromWidgets() *tikipkg.Tiki {
	tk := ev.GetTiki()
	if tk == nil {
		return nil
	}

	snapshot := tk.Clone()

	if ev.titleInput != nil {
		snapshot.Title = ev.titleInput.GetText()
	}
	if ev.prioritySelectList != nil {
		p := taskpkg.PriorityFromDisplay(ev.prioritySelectList.GetText())
		if p != 0 {
			snapshot.Set(tikipkg.FieldPriority, p)
		} else {
			snapshot.Delete(tikipkg.FieldPriority)
		}
	}
	if ev.pointsInput != nil {
		pts := ev.pointsInput.GetValue()
		if pts != 0 {
			snapshot.Set(tikipkg.FieldPoints, pts)
		} else {
			snapshot.Delete(tikipkg.FieldPoints)
		}
	}
	if ev.dueInput != nil {
		if parsed, ok := taskpkg.ParseDueDate(ev.dueInput.GetCurrentText()); ok {
			if parsed.IsZero() {
				snapshot.Delete(tikipkg.FieldDue)
			} else {
				snapshot.Set(tikipkg.FieldDue, parsed)
			}
		}
	}
	if ev.recurrenceInput != nil {
		r := taskpkg.Recurrence(ev.recurrenceInput.GetValue())
		if r == taskpkg.RecurrenceNone || r == "" {
			snapshot.Delete(tikipkg.FieldRecurrence)
		} else {
			snapshot.Set(tikipkg.FieldRecurrence, string(r))
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
func (ev *TaskEditView) HideTitleEditor() {
	// No-op in edit mode
}

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
func (ev *TaskEditView) HideDescriptionEditor() {
	// No-op in edit mode
}

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

// SetPrioritySaveHandler sets the callback for when priority is saved
func (ev *TaskEditView) SetPrioritySaveHandler(handler func(int)) {
	ev.onPrioritySave = handler
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
