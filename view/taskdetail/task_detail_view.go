package taskdetail

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/util/gradient"
	"github.com/boolean-maybe/tiki/view/renderer"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TaskDetailView renders a full task with all details in read-only mode.
// It supports inline editing of title and description.
type TaskDetailView struct {
	Base // Embed shared state

	registry *controller.ActionRegistry
	viewID   model.ViewID

	// View-mode specific
	storeListenerID int

	// Inline editing (title/description only)
	titleInput   *tview.InputField
	titleEditing bool
	descTextArea *tview.TextArea
	descEditing  bool
	isEditing    bool // prevents refresh while editing

	// Callbacks
	onTitleSave   func(string)
	onTitleChange func(string)
	onTitleCancel func()
	onDescSave    func(string)
	onDescCancel  func()
}

// NewTaskDetailView creates a task detail view in read-only mode
func NewTaskDetailView(taskStore store.Store, taskID string, renderer renderer.MarkdownRenderer) *TaskDetailView {
	tv := &TaskDetailView{
		Base: Base{
			taskStore: taskStore,
			taskID:    taskID,
			renderer:  renderer,
		},
		registry: controller.TaskDetailViewActions(),
		viewID:   model.TaskDetailViewID,
	}

	tv.build()
	tv.refresh()

	return tv
}

// GetActionRegistry returns the view's action registry
func (tv *TaskDetailView) GetActionRegistry() *controller.ActionRegistry {
	return tv.registry
}

// GetViewID returns the view identifier
func (tv *TaskDetailView) GetViewID() model.ViewID {
	return tv.viewID
}

// OnFocus is called when the view becomes active
func (tv *TaskDetailView) OnFocus() {
	// Register listener for live updates (respects isEditing flag)
	tv.storeListenerID = tv.taskStore.AddListener(func() {
		if !tv.isEditing {
			tv.refresh()
		}
	})
	tv.refresh()
}

// OnBlur is called when the view becomes inactive
func (tv *TaskDetailView) OnBlur() {
	if tv.storeListenerID != 0 {
		tv.taskStore.RemoveListener(tv.storeListenerID)
		tv.storeListenerID = 0
	}
}

// refresh re-renders the view
func (tv *TaskDetailView) refresh() {
	tv.content.Clear()
	tv.descView = nil

	task := tv.GetTask()
	if task == nil {
		notFound := tview.NewTextView().SetText("Task not found")
		tv.content.AddItem(notFound, 0, 1, false)
		return
	}

	colors := config.GetColors()

	if !tv.fullscreen {
		headerFrame := tv.buildHeader(task, colors)
		tv.content.AddItem(headerFrame, 9, 0, false)
	}

	descPrimitive := tv.buildDescription(task)
	tv.content.AddItem(descPrimitive, 0, 1, true)

	// keep editing flag in sync
	tv.isEditing = tv.titleEditing || tv.descEditing

	// Ensure focus is restored to description after refresh
	if tv.focusSetter != nil {
		tv.focusSetter(descPrimitive)
	}
}

func (tv *TaskDetailView) buildHeader(task *taskpkg.Task, colors *config.ColorConfig) *tview.Frame {
	headerContainer := tview.NewFlex().SetDirection(tview.FlexRow)

	leftSide := tview.NewFlex().SetDirection(tview.FlexRow)

	titlePrimitive := tv.buildTitlePrimitive(task, colors)
	leftSide.AddItem(titlePrimitive, 1, 0, false)
	leftSide.AddItem(tview.NewBox(), 1, 0, false) // blank line

	// build metadata columns
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}
	col1, col2 := tv.buildMetadataColumns(task, ctx)
	col3 := RenderMetadataColumn3(task, colors)

	metadataRow := tview.NewFlex().SetDirection(tview.FlexColumn)
	metadataRow.AddItem(col1, 30, 0, false)
	metadataRow.AddItem(tview.NewBox(), 2, 0, false)
	metadataRow.AddItem(col2, 30, 0, false)
	metadataRow.AddItem(tview.NewBox(), 2, 0, false)
	metadataRow.AddItem(col3, 30, 0, false)
	leftSide.AddItem(metadataRow, 4, 0, false)

	// Build right side (tags)
	rightSide := tview.NewFlex().SetDirection(tview.FlexRow)
	rightSide.AddItem(tview.NewBox(), 2, 0, false)
	tagsCol := RenderTagsColumn(task)
	rightSide.AddItem(tagsCol, 0, 1, false)

	mainRow := tview.NewFlex().SetDirection(tview.FlexColumn)
	mainRow.AddItem(leftSide, 0, 3, false)
	mainRow.AddItem(rightSide, 0, 1, false)

	headerContainer.AddItem(mainRow, 0, 1, false)

	headerFrame := tview.NewFrame(headerContainer).SetBorders(0, 0, 0, 0, 0, 0)
	headerFrame.SetBorder(true).SetTitle(fmt.Sprintf(" %s ", gradient.RenderGradientText(task.ID, colors.TaskDetailIDColor))).SetBorderColor(colors.TaskBoxUnselectedBorder)
	headerFrame.SetBorderPadding(1, 0, 2, 2)

	return headerFrame
}

func (tv *TaskDetailView) buildTitlePrimitive(task *taskpkg.Task, colors *config.ColorConfig) tview.Primitive {
	if tv.titleEditing {
		input := tv.ensureTitleInput(task)
		return input
	}

	// View mode rendering
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}
	return RenderTitleText(task, ctx)
}

func (tv *TaskDetailView) buildMetadataColumns(task *taskpkg.Task, ctx FieldRenderContext) (*tview.Flex, *tview.Flex) {
	// Column 1: Status, Type, Priority
	col1 := tview.NewFlex().SetDirection(tview.FlexRow)
	col1.AddItem(RenderStatusText(task, ctx), 1, 0, false)
	col1.AddItem(RenderTypeText(task, ctx), 1, 0, false)
	col1.AddItem(RenderPriorityText(task, ctx), 1, 0, false)

	// Column 2: Assignee, Points
	col2 := tview.NewFlex().SetDirection(tview.FlexRow)
	col2.AddItem(RenderAssigneeText(task, ctx), 1, 0, false)
	col2.AddItem(RenderPointsText(task, ctx), 1, 0, false)
	col2.AddItem(tview.NewBox(), 1, 0, false) // Spacer

	return col1, col2
}

func (tv *TaskDetailView) buildDescription(task *taskpkg.Task) tview.Primitive {
	if tv.descEditing {
		textArea := tv.ensureDescTextArea(task)
		return textArea
	}

	desc := defaultString(task.Description, "(No description)")

	renderedDesc, err := tv.renderer.Render(desc)
	if err != nil {
		renderedDesc = desc
	}

	descBox := tview.NewTextView().
		SetDynamicColors(true).
		SetText(renderedDesc).
		SetScrollable(true)

	descBox.SetBorderPadding(1, 1, 2, 2)
	tv.descView = descBox
	return descBox
}

func (tv *TaskDetailView) ensureDescTextArea(task *taskpkg.Task) *tview.TextArea {
	if tv.descTextArea == nil {
		tv.descTextArea = tview.NewTextArea()
		tv.descTextArea.SetBorder(false)
		tv.descTextArea.SetBorderPadding(1, 1, 2, 2)

		tv.descTextArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			key := event.Key()

			if key == tcell.KeyCtrlS {
				if tv.onDescSave != nil {
					tv.onDescSave(tv.descTextArea.GetText())
				}
				return nil
			}

			if key == tcell.KeyEscape {
				if tv.onDescCancel != nil {
					tv.onDescCancel()
				}
				return nil
			}

			return event
		})

		tv.descTextArea.SetText(task.Description, false)
	} else if !tv.descEditing {
		tv.descTextArea.SetText(task.Description, false)
	}

	tv.descEditing = true
	return tv.descTextArea
}

func (tv *TaskDetailView) ensureTitleInput(task *taskpkg.Task) *tview.InputField {
	if tv.titleInput == nil {
		colors := config.GetColors()
		tv.titleInput = tview.NewInputField()
		tv.titleInput.SetFieldBackgroundColor(config.GetContentBackgroundColor())
		tv.titleInput.SetFieldTextColor(colors.InputFieldTextColor)
		tv.titleInput.SetBorder(false)

		tv.titleInput.SetChangedFunc(func(text string) {
			if tv.onTitleChange != nil {
				tv.onTitleChange(text)
			}
		})

		tv.titleInput.SetDoneFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyEnter:
				if tv.onTitleSave != nil {
					tv.onTitleSave(tv.titleInput.GetText())
				}
			case tcell.KeyEscape:
				if tv.onTitleCancel != nil {
					tv.onTitleCancel()
				}
			}
		})

		tv.titleInput.SetText(task.Title)
	} else if !tv.titleEditing {
		tv.titleInput.SetText(task.Title)
	}

	tv.titleEditing = true
	return tv.titleInput
}

// EnterFullscreen switches the view to fullscreen mode (description only)
func (tv *TaskDetailView) EnterFullscreen() {
	if tv.fullscreen {
		return
	}
	tv.fullscreen = true
	tv.refresh()
	if tv.focusSetter != nil && tv.descView != nil {
		tv.focusSetter(tv.descView)
	}
	if tv.onFullscreenChange != nil {
		tv.onFullscreenChange(true)
	}
}

// ExitFullscreen restores the regular task detail layout
func (tv *TaskDetailView) ExitFullscreen() {
	if !tv.fullscreen {
		return
	}
	tv.fullscreen = false
	tv.refresh()
	if tv.focusSetter != nil && tv.descView != nil {
		tv.focusSetter(tv.descView)
	}
	if tv.onFullscreenChange != nil {
		tv.onFullscreenChange(false)
	}
}

// SetEditing sets the editing state (prevents refresh during edit)
func (tv *TaskDetailView) SetEditing(editing bool) {
	tv.isEditing = editing
}

// IsEditing returns whether the view is currently in edit mode
func (tv *TaskDetailView) IsEditing() bool {
	return tv.isEditing
}

// ShowTitleEditor displays the title input field and returns the primitive to focus
func (tv *TaskDetailView) ShowTitleEditor() tview.Primitive {
	task := tv.GetTask()
	if task == nil {
		task = tv.fallbackTask
	}
	if task == nil {
		return nil
	}

	input := tv.ensureTitleInput(task)
	tv.refresh()

	return input
}

// HideTitleEditor hides the title input field and returns to display mode
func (tv *TaskDetailView) HideTitleEditor() {
	if !tv.titleEditing {
		return
	}

	tv.titleEditing = false
	tv.isEditing = tv.descEditing
	tv.refresh()

	if tv.focusSetter != nil {
		tv.focusSetter(tv.content)
	}
}

// IsTitleEditing returns whether the title is currently being edited
func (tv *TaskDetailView) IsTitleEditing() bool {
	return tv.titleEditing
}

// IsTitleInputFocused returns whether the title input currently has focus
func (tv *TaskDetailView) IsTitleInputFocused() bool {
	return tv.titleEditing && tv.titleInput != nil && tv.titleInput.HasFocus()
}

// SetTitleSaveHandler sets the callback for when title is saved
func (tv *TaskDetailView) SetTitleSaveHandler(handler func(string)) {
	tv.onTitleSave = handler
}

// SetTitleChangeHandler sets the callback for when title changes
func (tv *TaskDetailView) SetTitleChangeHandler(handler func(string)) {
	tv.onTitleChange = handler
}

// SetTitleCancelHandler sets the callback for when title editing is cancelled
func (tv *TaskDetailView) SetTitleCancelHandler(handler func()) {
	tv.onTitleCancel = handler
}

// ShowDescriptionEditor displays the description text area and returns the primitive to focus
func (tv *TaskDetailView) ShowDescriptionEditor() tview.Primitive {
	task := tv.GetTask()
	if task == nil {
		return nil
	}

	desc := tv.ensureDescTextArea(task)
	tv.refresh()

	return desc
}

// HideDescriptionEditor hides the description text area and returns to display mode
func (tv *TaskDetailView) HideDescriptionEditor() {
	if !tv.descEditing {
		return
	}

	tv.descEditing = false
	tv.isEditing = tv.titleEditing
	tv.refresh()

	if tv.focusSetter != nil {
		tv.focusSetter(tv.content)
	}
}

// IsDescriptionEditing returns whether the description is currently being edited
func (tv *TaskDetailView) IsDescriptionEditing() bool {
	return tv.descEditing
}

// IsDescriptionTextAreaFocused returns whether the description text area currently has focus
func (tv *TaskDetailView) IsDescriptionTextAreaFocused() bool {
	return tv.descEditing && tv.descTextArea != nil && tv.descTextArea.HasFocus()
}

// SetDescriptionSaveHandler sets the callback for when description is saved
func (tv *TaskDetailView) SetDescriptionSaveHandler(handler func(string)) {
	tv.onDescSave = handler
}

// SetDescriptionCancelHandler sets the callback for when description editing is cancelled
func (tv *TaskDetailView) SetDescriptionCancelHandler(handler func()) {
	tv.onDescCancel = handler
}

// GetEditedTitle returns the current title in the editor
func (tv *TaskDetailView) GetEditedTitle() string {
	if tv.titleInput != nil {
		return tv.titleInput.GetText()
	}

	task := tv.GetTask()
	if task == nil {
		return ""
	}

	return task.Title
}

// GetEditedDescription returns the current description text in the editor
func (tv *TaskDetailView) GetEditedDescription() string {
	if tv.descTextArea != nil {
		return tv.descTextArea.GetText()
	}

	task := tv.GetTask()
	if task == nil {
		return ""
	}

	return task.Description
}
