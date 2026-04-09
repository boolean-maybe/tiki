package taskdetail

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/util/gradient"

	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/rivo/tview"
)

// Base contains shared state and methods for task detail/edit views.
// Both TaskDetailView and TaskEditView embed this struct to share common functionality.
type Base struct {
	// Layout
	root    *tview.Flex
	content *tview.Flex

	// Dependencies
	taskStore    store.Store
	taskID       string
	imageManager *navtview.ImageManager
	mermaidOpts  *nav.MermaidOptions
	descView     tview.Primitive

	// Task data
	fallbackTask   *taskpkg.Task
	taskController *controller.TaskController

	// Shared state
	fullscreen         bool
	focusSetter        func(tview.Primitive)
	onFullscreenChange func(bool)
}

// build initializes the root and content flex layouts
func (b *Base) build() {
	b.content = tview.NewFlex().SetDirection(tview.FlexRow)
	b.root = tview.NewFlex().SetDirection(tview.FlexRow)
	b.root.AddItem(b.content, 0, 1, true)
}

// GetTask returns the task from the store or the fallback task
func (b *Base) GetTask() *taskpkg.Task {
	task := b.taskStore.GetTask(b.taskID)
	if task == nil && b.fallbackTask != nil && b.fallbackTask.ID == b.taskID {
		task = b.fallbackTask
	}
	return task
}

// GetPrimitive returns the root tview primitive
func (b *Base) GetPrimitive() tview.Primitive {
	return b.root
}

// SetFallbackTask sets a task to render when it does not yet exist in the store (draft mode)
func (b *Base) SetFallbackTask(task *taskpkg.Task) {
	b.fallbackTask = task
}

// SetTaskController sets the task controller for edit session management
func (b *Base) SetTaskController(tc *controller.TaskController) {
	b.taskController = tc
}

// SetFocusSetter sets the callback for requesting focus changes
func (b *Base) SetFocusSetter(setter func(tview.Primitive)) {
	b.focusSetter = setter
}

// SetFullscreenChangeHandler sets the callback for when fullscreen state changes
func (b *Base) SetFullscreenChangeHandler(handler func(isFullscreen bool)) {
	b.onFullscreenChange = handler
}

// IsFullscreen reports whether the view is currently in fullscreen mode
func (b *Base) IsFullscreen() bool {
	return b.fullscreen
}

// assembleMetadataBox builds the framed metadata box from pre-built title and column
// primitives. It computes blocked tasks once and passes them to both BuildSectionInputs
// and RenderBlocksColumn, avoiding a double-scan.
func (b *Base) assembleMetadataBox(
	task *taskpkg.Task,
	colors *config.ColorConfig,
	titlePrimitive tview.Primitive,
	col1, col2, col3 *tview.Flex,
	titleHeight int,
) *tview.Frame {
	metadataContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	metadataContainer.AddItem(titlePrimitive, titleHeight, 0, titleHeight > 0)
	metadataContainer.AddItem(tview.NewBox(), 1, 0, false)

	blocked := taskpkg.FindBlockedTasks(b.taskStore.GetAllTasks(), task.ID)
	primitives := map[SectionID]tview.Primitive{
		SectionStatusGroup: col1,
		SectionPeopleGroup: col2,
		SectionDueGroup:    col3,
		SectionTags:        RenderTagsColumn(task),
		SectionDependsOn:   RenderDependsOnColumn(task, b.taskStore),
		SectionBlocks:      RenderBlocksColumn(blocked),
	}

	inputs := BuildSectionInputs(task, len(blocked) > 0)
	metadataRow := newResponsiveMetadataRow(inputs, primitives)
	metadataContainer.AddItem(metadataRow, 0, 1, false)

	metadataBox := tview.NewFrame(metadataContainer).SetBorders(0, 0, 0, 0, 0, 0)
	metadataBox.SetBorder(true).SetTitle(
		fmt.Sprintf(" %s ", gradient.RenderAdaptiveGradientText(task.ID, colors.TaskDetailIDColor, colors.FallbackTaskIDColor)),
	).SetBorderColor(colors.TaskBoxUnselectedBorder)
	metadataBox.SetBorderPadding(1, 0, 2, 2)

	return metadataBox
}

// defaultString returns def if s is empty, otherwise s
func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
