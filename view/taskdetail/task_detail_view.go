package taskdetail

import (
	"path/filepath"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/view/markdown"

	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/rivo/tview"
)

// TaskDetailView renders a full task with all details in read-only mode.
type TaskDetailView struct {
	Base // Embed shared state

	registry *controller.ActionRegistry
	viewID   model.ViewID

	// View-mode specific
	storeListenerID int
	navMarkdown     *markdown.NavigableMarkdown
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

// GetViewName returns the view name for the header info section
func (tv *TaskDetailView) GetViewName() string { return model.TaskDetailViewName }

// GetViewDescription returns the view description for the header info section
func (tv *TaskDetailView) GetViewDescription() string { return model.TaskDetailViewDesc }

// OnFocus is called when the view becomes active
func (tv *TaskDetailView) OnFocus() {
	// Register listener for live updates
	tv.storeListenerID = tv.taskStore.AddListener(func() {
		tv.refresh()
	})
	tv.refresh()
}

// RestoreFocus sets focus to the current description viewer (which may have been
// rebuilt by a store-driven refresh while the palette was open).
func (tv *TaskDetailView) RestoreFocus() bool {
	if tv.descView != nil && tv.focusSetter != nil {
		tv.focusSetter(tv.descView)
		return true
	}
	return false
}

// OnBlur is called when the view becomes inactive
func (tv *TaskDetailView) OnBlur() {
	if tv.storeListenerID != 0 {
		tv.taskStore.RemoveListener(tv.storeListenerID)
		tv.storeListenerID = 0
	}
	if tv.navMarkdown != nil {
		tv.navMarkdown.Close()
		tv.navMarkdown = nil
	}
}

// refresh re-renders the view
func (tv *TaskDetailView) refresh() {
	tv.content.Clear()
	tv.descView = nil
	if tv.navMarkdown != nil {
		tv.navMarkdown.Close()
		tv.navMarkdown = nil
	}

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
	taskSourcePath := taskSourcePathFor(task)

	// Image / relative-link search roots. The document's own directory wins
	// for sibling assets (`![[image.png]]` next to the doc); the unified
	// .doc/ root is the fallback so tasks with image paths relative to the
	// root (e.g. `.doc/assets/foo.png`) resolve too. Keeping .doc/tiki in
	// the list protects docs still sitting under the legacy layout.
	searchRoots := []string{config.GetDocDir(), config.GetTaskDir()}
	if taskSourcePath != "" {
		searchRoots = append([]string{filepath.Dir(taskSourcePath)}, searchRoots...)
	}

	// Phase 6B.11: wrap the description provider so every FetchContent call
	// — both the initial description render AND every navigated-through
	// click — runs its body through wikilink rewriting. The earlier
	// implementation only rewrote the initial description and left click
	// targets raw, so nested `[[ID]]` links stopped resolving after the
	// first navigation.
	resolver := &markdown.StoreResolver{Store: tv.taskStore}
	wrapped := markdown.NewWikilinkProvider(
		newTaskDescriptionProvider(tv.taskStore, searchRoots),
		resolver,
	)
	tv.navMarkdown = markdown.NewNavigableMarkdown(markdown.NavigableMarkdownConfig{
		Provider:       wrapped,
		SearchRoots:    searchRoots,
		ImageManager:   tv.imageManager,
		MermaidOptions: tv.mermaidOpts,
	})
	// Rewrite the initial description the same way the provider will
	// rewrite subsequent fetches — SetMarkdownWithSource does not round-trip
	// through FetchContent, so this body needs explicit treatment.
	desc = markdown.RewriteWikilinks(desc, resolver)
	tv.navMarkdown.SetMarkdownWithSource(desc, taskSourcePath, false)
	tv.navMarkdown.Viewer().SetBorderPadding(1, 1, 2, 2)
	tv.descView = tv.navMarkdown.Viewer()
	return tv.navMarkdown.Viewer()
}

// taskSourcePathFor returns the on-disk path of the task's markdown file for
// use as the markdown view's source path (image root, relative-link base).
// Honors task.FilePath when the task was loaded from disk — so renames and
// nested layouts resolve correctly — and falls back to the id-derived
// default under the document root for tasks that have no path yet.
func taskSourcePathFor(task *taskpkg.Task) string {
	if task == nil {
		return ""
	}
	if task.FilePath != "" {
		return task.FilePath
	}
	return filepath.Join(config.GetDocDir(), task.ID+".md")
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
