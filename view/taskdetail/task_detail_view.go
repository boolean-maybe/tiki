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

	"github.com/rivo/tview"
)

// TaskDetailView renders a full task with all details in read-only mode.
type TaskDetailView struct {
	Base // Embed shared state

	registry *controller.ActionRegistry
	viewID   model.ViewID

	// View-mode specific
	storeListenerID int
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
	// Register listener for live updates
	tv.storeListenerID = tv.taskStore.AddListener(func() {
		tv.refresh()
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
		metadataBox := tv.buildMetadataBox(task, colors)
		tv.content.AddItem(metadataBox, 9, 0, false)
	}

	descPrimitive := tv.buildDescription(task)
	tv.content.AddItem(descPrimitive, 0, 1, true)

	// Ensure focus is restored to description after refresh
	if tv.focusSetter != nil {
		tv.focusSetter(descPrimitive)
	}
}

func (tv *TaskDetailView) buildMetadataBox(task *taskpkg.Task, colors *config.ColorConfig) *tview.Frame {
	metadataContainer := tview.NewFlex().SetDirection(tview.FlexRow)

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

	metadataContainer.AddItem(mainRow, 0, 1, false)

	metadataBox := tview.NewFrame(metadataContainer).SetBorders(0, 0, 0, 0, 0, 0)
	metadataBox.SetBorder(true).SetTitle(fmt.Sprintf(" %s ", gradient.RenderAdaptiveGradientText(task.ID, colors.TaskDetailIDColor, config.FallbackTaskIDColor))).SetBorderColor(colors.TaskBoxUnselectedBorder)
	metadataBox.SetBorderPadding(1, 0, 2, 2)

	return metadataBox
}

func (tv *TaskDetailView) buildTitlePrimitive(task *taskpkg.Task, colors *config.ColorConfig) tview.Primitive {
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
