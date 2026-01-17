package taskdetail

import (
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/view/renderer"

	"github.com/rivo/tview"
)

// Base contains shared state and methods for task detail/edit views.
// Both TaskDetailView and TaskEditView embed this struct to share common functionality.
type Base struct {
	// Layout
	root    *tview.Flex
	content *tview.Flex

	// Dependencies
	taskStore store.Store
	taskID    string
	renderer  renderer.MarkdownRenderer
	descView  *tview.TextView

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

// GetTaskID returns the task being displayed
func (b *Base) GetTaskID() string {
	return b.taskID
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

// defaultString returns def if s is empty, otherwise s
func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
