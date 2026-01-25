package taskdetail

import (
	"fmt"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/view/renderer"

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
	headerFrame      *tview.Frame

	// All field editors
	titleInput         *tview.InputField
	titleEditing       bool
	descTextArea       *tview.TextArea
	descEditing        bool
	isEditing          bool
	statusSelectList   *component.EditSelectList
	typeSelectList     *component.EditSelectList
	priorityInput      *component.IntEditSelect
	assigneeSelectList *component.EditSelectList
	pointsInput        *component.IntEditSelect

	// All callbacks
	onTitleSave    func(string)
	onTitleChange  func(string)
	onTitleCancel  func()
	onDescSave     func(string)
	onDescCancel   func()
	onStatusSave   func(string)
	onTypeSave     func(string)
	onPrioritySave func(int)
	onAssigneeSave func(string)
	onPointsSave   func(int)
}

// Compile-time interface checks
var (
	_ controller.View          = (*TaskEditView)(nil)
	_ controller.FocusSettable = (*TaskEditView)(nil)
)

// NewTaskEditView creates a task edit view
func NewTaskEditView(taskStore store.Store, taskID string, renderer renderer.MarkdownRenderer) *TaskEditView {
	ev := &TaskEditView{
		Base: Base{
			taskStore: taskStore,
			taskID:    taskID,
			renderer:  renderer,
		},
		registry:     controller.TaskEditViewActions(),
		viewID:       model.TaskEditViewID,
		focusedField: model.EditFieldTitle,
		titleEditing: true,
		descEditing:  true,
		isEditing:    true,
	}

	ev.build()

	// Eagerly create all edit field widgets to ensure they exist before focus management
	task := ev.GetTask()
	if task != nil {
		ev.ensureTitleInput(task)
		ev.ensureDescTextArea(task)
		ev.ensureStatusSelectList(task)
		ev.ensureTypeSelectList(task)
		ev.ensurePriorityInput(task)
		ev.ensureAssigneeSelectList(task)
		ev.ensurePointsInput(task)
	}

	ev.refresh()

	return ev
}

// GetTask returns the appropriate task based on mode (prioritizes editing copy)
func (ev *TaskEditView) GetTask() *taskpkg.Task {
	if ev.taskController != nil {
		if draftTask := ev.taskController.GetDraftTask(); draftTask != nil {
			return draftTask
		}
		if editingTask := ev.taskController.GetEditingTask(); editingTask != nil {
			return editingTask
		}
	}

	task := ev.taskStore.GetTask(ev.taskID)
	if task == nil && ev.fallbackTask != nil && ev.fallbackTask.ID == ev.taskID {
		task = ev.fallbackTask
	}
	return task
}

// GetActionRegistry returns the view's action registry
func (ev *TaskEditView) GetActionRegistry() *controller.ActionRegistry {
	return ev.registry
}

// GetViewID returns the view identifier
func (ev *TaskEditView) GetViewID() model.ViewID {
	return ev.viewID
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

	task := ev.GetTask()
	if task == nil {
		notFound := tview.NewTextView().SetText("Task not found")
		ev.content.AddItem(notFound, 0, 1, false)
		return
	}

	colors := config.GetColors()

	if !ev.fullscreen {
		headerFrame := ev.buildHeader(task, colors)
		ev.content.AddItem(headerFrame, 9, 0, false)
	}

	descPrimitive := ev.buildDescription(task)
	ev.content.AddItem(descPrimitive, 0, 1, true)

	ev.updateValidationState()
}

func (ev *TaskEditView) buildHeader(task *taskpkg.Task, colors *config.ColorConfig) *tview.Frame {
	headerContainer := tview.NewFlex().SetDirection(tview.FlexRow)

	leftSide := tview.NewFlex().SetDirection(tview.FlexRow)

	titlePrimitive := ev.buildTitlePrimitive(task, colors)
	leftSide.AddItem(titlePrimitive, 1, 0, true)
	leftSide.AddItem(tview.NewBox(), 1, 0, false)

	ctx := FieldRenderContext{
		Mode:         RenderModeEdit,
		FocusedField: ev.focusedField,
		Colors:       colors,
	}
	col1, col2 := ev.buildMetadataColumns(task, ctx)
	col3 := RenderMetadataColumn3(task, colors)

	metadataRow := tview.NewFlex().SetDirection(tview.FlexColumn)
	metadataRow.AddItem(col1, 30, 0, false)
	metadataRow.AddItem(tview.NewBox(), 2, 0, false)
	metadataRow.AddItem(col2, 30, 0, false)
	metadataRow.AddItem(tview.NewBox(), 2, 0, false)
	metadataRow.AddItem(col3, 30, 0, false)
	leftSide.AddItem(metadataRow, 4, 0, false)

	rightSide := tview.NewFlex().SetDirection(tview.FlexRow)
	rightSide.AddItem(tview.NewBox(), 2, 0, false)
	tagsCol := RenderTagsColumn(task)
	rightSide.AddItem(tagsCol, 0, 1, false)

	mainRow := tview.NewFlex().SetDirection(tview.FlexColumn)
	mainRow.AddItem(leftSide, 0, 3, true)
	mainRow.AddItem(rightSide, 0, 1, false)

	headerContainer.AddItem(mainRow, 0, 1, false)

	headerFrame := tview.NewFrame(headerContainer).SetBorders(0, 0, 0, 0, 0, 0)
	headerFrame.SetBorder(true).SetTitle(fmt.Sprintf(" %s ", renderGradientText(task.ID, colors.TaskDetailIDColor))).SetBorderColor(colors.TaskBoxUnselectedBorder)
	headerFrame.SetBorderPadding(1, 0, 2, 2)

	ev.headerFrame = headerFrame

	return headerFrame
}

func (ev *TaskEditView) buildTitlePrimitive(task *taskpkg.Task, colors *config.ColorConfig) tview.Primitive {
	input := ev.ensureTitleInput(task)
	focused := ev.focusedField == model.EditFieldTitle
	if focused {
		input.SetLabel(getFocusMarker(colors))
	} else {
		input.SetLabel("")
	}
	return input
}

func (ev *TaskEditView) buildMetadataColumns(task *taskpkg.Task, ctx FieldRenderContext) (*tview.Flex, *tview.Flex) {
	// Column 1: Status, Type, Priority
	col1 := tview.NewFlex().SetDirection(tview.FlexRow)
	col1.AddItem(ev.buildStatusField(task, ctx), 1, 0, false)
	col1.AddItem(ev.buildTypeField(task, ctx), 1, 0, false)
	col1.AddItem(ev.buildPriorityField(task, ctx), 1, 0, false)

	// Column 2: Assignee, Points
	col2 := tview.NewFlex().SetDirection(tview.FlexRow)
	col2.AddItem(ev.buildAssigneeField(task, ctx), 1, 0, false)
	col2.AddItem(ev.buildPointsField(task, ctx), 1, 0, false)
	col2.AddItem(tview.NewBox(), 1, 0, false)

	return col1, col2
}

func (ev *TaskEditView) buildStatusField(task *taskpkg.Task, ctx FieldRenderContext) tview.Primitive {
	if ctx.FocusedField == model.EditFieldStatus {
		return ev.ensureStatusSelectList(task)
	}
	return RenderStatusText(task, ctx)
}

func (ev *TaskEditView) buildTypeField(task *taskpkg.Task, ctx FieldRenderContext) tview.Primitive {
	if ctx.FocusedField == model.EditFieldType {
		return ev.ensureTypeSelectList(task)
	}
	return RenderTypeText(task, ctx)
}

func (ev *TaskEditView) buildPriorityField(task *taskpkg.Task, ctx FieldRenderContext) tview.Primitive {
	if ctx.FocusedField == model.EditFieldPriority {
		return ev.ensurePriorityInput(task)
	}
	return RenderPriorityText(task, ctx)
}

func (ev *TaskEditView) buildAssigneeField(task *taskpkg.Task, ctx FieldRenderContext) tview.Primitive {
	if ctx.FocusedField == model.EditFieldAssignee {
		return ev.ensureAssigneeSelectList(task)
	}
	return RenderAssigneeText(task, ctx)
}

func (ev *TaskEditView) buildPointsField(task *taskpkg.Task, ctx FieldRenderContext) tview.Primitive {
	if ctx.FocusedField == model.EditFieldPoints {
		return ev.ensurePointsInput(task)
	}
	return RenderPointsText(task, ctx)
}

func (ev *TaskEditView) buildDescription(task *taskpkg.Task) tview.Primitive {
	textArea := ev.ensureDescTextArea(task)
	return textArea
}

func (ev *TaskEditView) ensureDescTextArea(task *taskpkg.Task) *tview.TextArea {
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

		ev.descTextArea.SetText(task.Description, false)
	} else if !ev.descEditing {
		ev.descTextArea.SetText(task.Description, false)
	}

	ev.descEditing = true
	return ev.descTextArea
}

func (ev *TaskEditView) ensureTitleInput(task *taskpkg.Task) *tview.InputField {
	if ev.titleInput == nil {
		colors := config.GetColors()
		ev.titleInput = tview.NewInputField()
		ev.titleInput.SetFieldBackgroundColor(config.GetContentBackgroundColor())
		ev.titleInput.SetFieldTextColor(colors.InputFieldTextColor)
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

		ev.titleInput.SetText(task.Title)
	} else if !ev.titleEditing {
		ev.titleInput.SetText(task.Title)
	}

	ev.titleEditing = true
	return ev.titleInput
}

// updateValidationState runs validation checks and updates the border color
func (ev *TaskEditView) updateValidationState() {
	// Get current task state from widgets
	task := ev.buildTaskFromWidgets()
	if task == nil {
		return
	}

	// Run validation framework
	errors := task.Validate()

	// Convert ValidationErrors to string messages for UI display
	ev.validationErrors = nil
	for _, err := range errors {
		ev.validationErrors = append(ev.validationErrors, err.Message)
	}

	// Update border color based on validation
	if ev.headerFrame != nil {
		colors := config.DefaultColors()
		if len(ev.validationErrors) > 0 {
			ev.headerFrame.SetBorderColor(colors.TaskBoxSelectedBorder)
		} else {
			ev.headerFrame.SetBorderColor(colors.TaskBoxUnselectedBorder)
		}
	}
}

// buildTaskFromWidgets creates a task snapshot from current widget values
func (ev *TaskEditView) buildTaskFromWidgets() *taskpkg.Task {
	task := ev.GetTask()
	if task == nil {
		return nil
	}

	// Create snapshot with current widget values
	snapshot := task.Clone()

	if ev.titleInput != nil {
		snapshot.Title = ev.titleInput.GetText()
	}
	if ev.priorityInput != nil {
		snapshot.Priority = ev.priorityInput.GetValue()
	}
	if ev.pointsInput != nil {
		snapshot.Points = ev.pointsInput.GetValue()
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
	if ev.focusSetter != nil && ev.descView != nil {
		ev.focusSetter(ev.descView)
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
	if ev.focusSetter != nil && ev.descView != nil {
		ev.focusSetter(ev.descView)
	}
	if ev.onFullscreenChange != nil {
		ev.onFullscreenChange(false)
	}
}

// SetEditing sets the editing state
func (ev *TaskEditView) SetEditing(editing bool) {
	ev.isEditing = editing
}

// IsEditing returns whether the view is currently in edit mode
func (ev *TaskEditView) IsEditing() bool {
	return ev.isEditing
}

// ShowTitleEditor displays the title input field
func (ev *TaskEditView) ShowTitleEditor() tview.Primitive {
	task := ev.GetTask()
	if task == nil {
		return nil
	}
	return ev.ensureTitleInput(task)
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
	task := ev.GetTask()
	if task == nil {
		return nil
	}
	return ev.ensureDescTextArea(task)
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

	task := ev.GetTask()
	if task == nil {
		return ""
	}

	return task.Title
}

// GetEditedDescription returns the current description text in the editor
func (ev *TaskEditView) GetEditedDescription() string {
	if ev.descTextArea != nil {
		return ev.descTextArea.GetText()
	}

	task := ev.GetTask()
	if task == nil {
		return ""
	}

	return task.Description
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
