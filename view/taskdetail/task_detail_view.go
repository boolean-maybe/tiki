package taskdetail

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"

	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	navutil "github.com/boolean-maybe/navidown/util"
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

// NewTaskDetailView creates a task detail view.
// When readOnly is true, only fullscreen is available — no editing actions.
func NewTaskDetailView(taskStore store.Store, taskID string, readOnly bool, imageManager *navtview.ImageManager, mermaidOpts *nav.MermaidOptions) *TaskDetailView {
	registry := controller.TaskDetailViewActions()
	if readOnly {
		registry = controller.ReadonlyTaskDetailViewActions()
	}
	tv := &TaskDetailView{
		Base: Base{
			taskStore:    taskStore,
			taskID:       taskID,
			imageManager: imageManager,
			mermaidOpts:  mermaidOpts,
		},
		registry: registry,
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
		tv.content.AddItem(metadataBox, 10, 0, false)
	}

	descPrimitive := tv.buildDescription(task)
	tv.content.AddItem(descPrimitive, 0, 1, true)

	// Ensure focus is restored to description after refresh
	if tv.focusSetter != nil {
		tv.focusSetter(descPrimitive)
	}
}

func (tv *TaskDetailView) buildMetadataBox(task *taskpkg.Task, colors *config.ColorConfig) *tview.Frame {
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}
	titlePrimitive := tv.buildTitlePrimitive(task, colors)
	col1, col2, col3 := tv.buildMetadataColumns(task, ctx, colors)
	return tv.assembleMetadataBox(task, colors, titlePrimitive, col1, col2, col3, 1)
}

func (tv *TaskDetailView) buildTitlePrimitive(task *taskpkg.Task, colors *config.ColorConfig) tview.Primitive {
	// View mode rendering
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}
	return RenderTitleText(task, ctx)
}

func (tv *TaskDetailView) buildMetadataColumns(task *taskpkg.Task, ctx FieldRenderContext, colors *config.ColorConfig) (*tview.Flex, *tview.Flex, *tview.Flex) {
	// Column 1: Status, Type, Priority, Points
	col1 := tview.NewFlex().SetDirection(tview.FlexRow)
	col1.AddItem(RenderStatusText(task, ctx), 1, 0, false)
	col1.AddItem(RenderTypeText(task, ctx), 1, 0, false)
	col1.AddItem(RenderPriorityText(task, ctx), 1, 0, false)
	col1.AddItem(RenderPointsText(task, ctx), 1, 0, false)

	// Column 2: Assignee, Author, Created, Updated
	col2 := tview.NewFlex().SetDirection(tview.FlexRow)
	col2.AddItem(RenderAssigneeText(task, ctx), 1, 0, false)
	col2.AddItem(RenderAuthorText(task, colors), 1, 0, false)
	col2.AddItem(RenderCreatedText(task, colors), 1, 0, false)
	col2.AddItem(RenderUpdatedText(task, colors), 1, 0, false)

	// Column 3: Due, Recurrence
	col3 := tview.NewFlex().SetDirection(tview.FlexRow)
	col3.AddItem(RenderDueText(task, colors), 1, 0, false)
	col3.AddItem(RenderRecurrenceText(task, colors), 1, 0, false)

	return col1, col2, col3
}

func (tv *TaskDetailView) buildDescription(task *taskpkg.Task) tview.Primitive {
	desc := defaultString(task.Description, "(No description)")

	// Get the source file path for the task to enable relative image resolution
	taskSourcePath := getTaskFilePath(task.ID)

	viewer := navtview.NewTextView()
	viewer.SetAnsiConverter(navutil.NewAnsiConverter(true))
	renderer := nav.NewANSIRendererWithStyle(config.GetEffectiveTheme())
	if t := config.GetCodeBlockTheme(); t != "" {
		renderer = renderer.WithCodeTheme(t)
	}
	if bg := config.GetCodeBlockBackground(); bg != "" {
		renderer = renderer.WithCodeBackground(bg)
	}
	if b := config.GetCodeBlockBorder(); b != "" {
		renderer = renderer.WithCodeBorder(b)
	}
	viewer.SetRenderer(renderer)
	viewer.SetBackgroundColor(config.GetContentBackgroundColor())
	if tv.imageManager != nil && tv.imageManager.Supported() {
		viewer.SetImageManager(tv.imageManager)
	}
	if tv.mermaidOpts != nil {
		viewer.Core().SetMermaidOptions(tv.mermaidOpts)
	}
	// Use SetMarkdownWithSource to provide the source file path for relative image resolution
	viewer.SetMarkdownWithSource(desc, taskSourcePath, false)
	viewer.SetBorderPadding(1, 1, 2, 2)
	tv.descView = viewer
	return viewer
}

// getTaskFilePath constructs the file path for a task based on its ID
// This enables relative image path resolution in markdown content
func getTaskFilePath(taskID string) string {
	// Task files are named like "tiki-z53pc9.md" (lowercase) in the task directory
	// but the task ID in the struct is "TIKI-z53pc9" (uppercase prefix)
	// Convert to lowercase for the filename
	taskFilename := fmt.Sprintf("%s.md", strings.ToLower(taskID))
	return filepath.Join(config.GetTaskDir(), taskFilename)
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
