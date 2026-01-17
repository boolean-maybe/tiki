package controller

import (
	"github.com/boolean-maybe/tiki/model"

	"github.com/gdamore/tcell/v2"
)

// TaskEditCoordinator owns task edit lifecycle: preparing the view, wiring handlers,
// and implementing commit/cancel and field navigation policy.
type TaskEditCoordinator struct {
	navController  *NavigationController
	taskController *TaskController

	preparedView View
}

func NewTaskEditCoordinator(navController *NavigationController, taskController *TaskController) *TaskEditCoordinator {
	return &TaskEditCoordinator{
		navController:  navController,
		taskController: taskController,
	}
}

// Prepare wires handlers and starts an edit session for the provided view instance.
// It is safe to call repeatedly; preparation is cached per active view instance.
func (c *TaskEditCoordinator) Prepare(activeView View, params model.TaskEditParams) {
	if activeView == nil {
		return
	}
	if c.preparedView == activeView {
		return
	}

	if params.TaskID != "" {
		c.taskController.SetCurrentTask(params.TaskID)
	}
	if params.Draft != nil {
		c.taskController.SetDraft(params.Draft)
	} else {
		c.taskController.ClearDraft()
	}

	c.prepareView(activeView, params.Focus)
	c.preparedView = activeView
}

func (c *TaskEditCoordinator) HandleKey(activeView View, event *tcell.EventKey) bool {
	switch event.Key() {
	case tcell.KeyCtrlS:
		return c.CommitAndClose(activeView)
	case tcell.KeyTab:
		return c.FocusNextField(activeView)
	case tcell.KeyBacktab:
		return c.FocusPrevField(activeView)
	case tcell.KeyEscape:
		return c.CancelAndClose()
	case tcell.KeyUp:
		return c.CycleFieldValueUp(activeView)
	case tcell.KeyDown:
		return c.CycleFieldValueDown(activeView)
	default:
		return false
	}
}

func (c *TaskEditCoordinator) FocusNextField(activeView View) bool {
	fieldFocusable, ok := activeView.(FieldFocusableView)
	if !ok {
		return false
	}
	return fieldFocusable.FocusNextField()
}

func (c *TaskEditCoordinator) FocusPrevField(activeView View) bool {
	fieldFocusable, ok := activeView.(FieldFocusableView)
	if !ok {
		return false
	}
	return fieldFocusable.FocusPrevField()
}

func (c *TaskEditCoordinator) CycleFieldValueUp(activeView View) bool {
	if cyclable, ok := activeView.(ValueCyclableView); ok {
		return cyclable.CycleFieldValueUp()
	}
	return false
}

func (c *TaskEditCoordinator) CycleFieldValueDown(activeView View) bool {
	if cyclable, ok := activeView.(ValueCyclableView); ok {
		return cyclable.CycleFieldValueDown()
	}
	return false
}

func (c *TaskEditCoordinator) CommitAndClose(activeView View) bool {
	if !c.commit(activeView) {
		return false
	}
	c.navController.HandleBack()
	return true
}

func (c *TaskEditCoordinator) CommitNoClose(activeView View) bool {
	if !c.commit(activeView) {
		return false
	}

	// Re-start edit session with newly saved task
	taskID := c.taskController.currentTaskID
	if editingTask := c.taskController.StartEditSession(taskID); editingTask != nil {
		// Refresh view with new editing copy
		if refreshable, ok := activeView.(interface{ Refresh() }); ok {
			refreshable.Refresh()
		}
	}
	return true
}

func (c *TaskEditCoordinator) CancelAndClose() bool {
	// Cancel edit session (discards changes) and clear any draft.
	c.taskController.CancelEditSession()
	c.taskController.ClearDraft()
	c.navController.HandleBack()
	return true
}

func (c *TaskEditCoordinator) commit(activeView View) bool {
	editorView, ok := activeView.(TaskEditView)
	if !ok {
		return false
	}

	// Check validation state - do not save if invalid
	if validator, ok := activeView.(interface{ IsValid() bool }); ok {
		if !validator.IsValid() {
			return false // save is disabled when validation fails
		}
	}

	// Update in-memory editing copy with latest widget values
	c.taskController.SaveTitle(editorView.GetEditedTitle())
	c.taskController.SaveDescription(editorView.GetEditedDescription())

	// Commit the edit session (writes to disk)
	if err := c.taskController.CommitEditSession(); err != nil {
		return false
	}
	return true
}

func (c *TaskEditCoordinator) prepareView(activeView View, focus model.EditField) {
	app := c.navController.GetApp()

	// Start edit session for existing tasks (creates in-memory copy)
	// Draft tasks already have an in-memory copy via draftTask
	if _, ok := activeView.(TaskEditView); ok {
		if taskView, hasController := activeView.(interface{ SetTaskController(*TaskController) }); hasController {
			taskView.SetTaskController(c.taskController)
		}

		// Only start edit session for non-draft tasks
		if c.taskController.draftTask == nil {
			taskID := c.taskController.currentTaskID
			if taskID != "" {
				c.taskController.StartEditSession(taskID)
			}
		}
	}

	if titleEditableView, ok := activeView.(TitleEditableView); ok {
		// Explicit save on Enter (commits and closes)
		titleEditableView.SetTitleSaveHandler(func(_ string) {
			c.CommitAndClose(activeView)
		})
		titleEditableView.SetTitleCancelHandler(func() {
			c.CancelAndClose()
		})
	}

	if descEditableView, ok := activeView.(DescriptionEditableView); ok {
		descEditableView.SetDescriptionSaveHandler(func(_ string) {
			c.CommitNoClose(activeView)
		})
		descEditableView.SetDescriptionCancelHandler(func() {
			c.CancelAndClose()
		})
	}

	if statusEditableView, ok := activeView.(StatusEditableView); ok {
		statusEditableView.SetStatusSaveHandler(func(statusDisplay string) {
			c.taskController.SaveStatus(statusDisplay)
		})
	}

	if typeEditableView, ok := activeView.(TypeEditableView); ok {
		typeEditableView.SetTypeSaveHandler(func(typeDisplay string) {
			c.taskController.SaveType(typeDisplay)
		})
	}

	if priorityEditableView, ok := activeView.(PriorityEditableView); ok {
		priorityEditableView.SetPrioritySaveHandler(func(priority int) {
			c.taskController.SavePriority(priority)
		})
	}

	if assigneeEditableView, ok := activeView.(AssigneeEditableView); ok {
		assigneeEditableView.SetAssigneeSaveHandler(func(assignee string) {
			c.taskController.SaveAssignee(assignee)
		})
	}

	if pointsEditableView, ok := activeView.(PointsEditableView); ok {
		pointsEditableView.SetPointsSaveHandler(func(points int) {
			c.taskController.SavePoints(points)
		})
	}

	// Initialize with title field focused by default (or specified focus field)
	if fieldFocusable, ok := activeView.(FieldFocusableView); ok {
		fieldFocusable.SetFocusedField(model.EditFieldTitle)
	}

	if focus == model.EditFieldDescription {
		if fieldFocusable, ok := activeView.(FieldFocusableView); ok {
			fieldFocusable.SetFocusedField(model.EditFieldDescription)
		}
		if descEditableView, ok := activeView.(DescriptionEditableView); ok {
			if desc := descEditableView.ShowDescriptionEditor(); desc != nil {
				app.SetFocus(desc)
				return
			}
		}
	}

	if titleEditableView, ok := activeView.(TitleEditableView); ok {
		if title := titleEditableView.ShowTitleEditor(); title != nil {
			app.SetFocus(title)
			return
		}
	}

	// fallback to next field focus cycle
	_ = c.FocusNextField(activeView)
}
