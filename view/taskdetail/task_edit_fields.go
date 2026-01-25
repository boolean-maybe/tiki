package taskdetail

import (
	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
)

// This file contains the edit field component creation methods for TaskEditView.

func (ev *TaskEditView) ensureStatusSelectList(task *taskpkg.Task) *component.EditSelectList {
	if ev.statusSelectList == nil {
		statusOptions := []string{
			taskpkg.StatusDisplay(taskpkg.StatusBacklog),
			taskpkg.StatusDisplay(taskpkg.StatusReady),
			taskpkg.StatusDisplay(taskpkg.StatusInProgress),
			taskpkg.StatusDisplay(taskpkg.StatusReview),
			taskpkg.StatusDisplay(taskpkg.StatusDone),
		}

		colors := config.GetColors()
		ev.statusSelectList = component.NewEditSelectList(statusOptions, false)
		ev.statusSelectList.SetLabel(getFocusMarker(colors) + "Status:   ")
		ev.statusSelectList.SetInitialValue(taskpkg.StatusDisplay(task.Status))

		ev.statusSelectList.SetSubmitHandler(func(text string) {
			if ev.onStatusSave != nil {
				ev.onStatusSave(text)
			}
			ev.updateValidationState()
		})
	}
	return ev.statusSelectList
}

func (ev *TaskEditView) ensureTypeSelectList(task *taskpkg.Task) *component.EditSelectList {
	if ev.typeSelectList == nil {
		typeOptions := []string{
			taskpkg.TypeDisplay(taskpkg.TypeStory),
			taskpkg.TypeDisplay(taskpkg.TypeBug),
			taskpkg.TypeDisplay(taskpkg.TypeSpike),
			taskpkg.TypeDisplay(taskpkg.TypeEpic),
		}

		colors := config.GetColors()
		ev.typeSelectList = component.NewEditSelectList(typeOptions, false)
		ev.typeSelectList.SetLabel(getFocusMarker(colors) + "Type:     ")
		ev.typeSelectList.SetInitialValue(taskpkg.TypeDisplay(task.Type))

		ev.typeSelectList.SetSubmitHandler(func(text string) {
			if ev.onTypeSave != nil {
				ev.onTypeSave(text)
			}
			ev.updateValidationState()
		})
	}
	return ev.typeSelectList
}

func (ev *TaskEditView) ensurePriorityInput(task *taskpkg.Task) *component.IntEditSelect {
	if ev.priorityInput == nil {
		colors := config.GetColors()
		ev.priorityInput = component.NewIntEditSelect(1, 5, false)
		ev.priorityInput.SetLabel(getFocusMarker(colors) + "Priority: ")

		ev.priorityInput.SetChangeHandler(func(value int) {
			ev.updateValidationState()

			if ev.onPrioritySave != nil {
				ev.onPrioritySave(value)
			}
		})

		ev.priorityInput.SetValue(task.Priority)
	}
	// Don't reset value if widget already exists - preserve user edits

	return ev.priorityInput
}

func (ev *TaskEditView) ensurePointsInput(task *taskpkg.Task) *component.IntEditSelect {
	if ev.pointsInput == nil {
		colors := config.GetColors()
		ev.pointsInput = component.NewIntEditSelect(1, config.GetMaxPoints(), false)
		ev.pointsInput.SetLabel(getFocusMarker(colors) + "Points:  ")

		ev.pointsInput.SetChangeHandler(func(value int) {
			ev.updateValidationState()

			if ev.onPointsSave != nil {
				ev.onPointsSave(value)
			}
		})

		ev.pointsInput.SetValue(task.Points)
	}
	// Don't reset value if widget already exists - preserve user edits

	return ev.pointsInput
}

func (ev *TaskEditView) ensureAssigneeSelectList(task *taskpkg.Task) *component.EditSelectList {
	if ev.assigneeSelectList == nil {
		var assigneeOptions []string
		if users, err := ev.taskStore.GetAllUsers(); err == nil {
			assigneeOptions = append(assigneeOptions, users...)
		}

		if len(assigneeOptions) == 0 {
			assigneeOptions = []string{"Unassigned"}
		}

		colors := config.GetColors()
		ev.assigneeSelectList = component.NewEditSelectList(assigneeOptions, true)
		ev.assigneeSelectList.SetLabel(getFocusMarker(colors) + "Assignee: ")

		initialValue := task.Assignee
		if initialValue == "" {
			initialValue = "Unassigned"
		}
		ev.assigneeSelectList.SetInitialValue(initialValue)

		ev.assigneeSelectList.SetSubmitHandler(func(text string) {
			if ev.onAssigneeSave != nil {
				ev.onAssigneeSave(text)
			}
			ev.updateValidationState()
		})
	}
	return ev.assigneeSelectList
}
