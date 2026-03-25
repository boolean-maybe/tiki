package taskdetail

import (
	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
)

// This file contains the edit field component creation methods for TaskEditView.

func (ev *TaskEditView) ensureStatusSelectList(task *taskpkg.Task) *component.EditSelectList {
	if ev.statusSelectList == nil {
		allStatuses := taskpkg.AllStatuses()
		statusOptions := make([]string, len(allStatuses))
		for i, s := range allStatuses {
			statusOptions[i] = taskpkg.StatusDisplay(s)
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

func (ev *TaskEditView) ensurePrioritySelectList(task *taskpkg.Task) *component.EditSelectList {
	if ev.prioritySelectList == nil {
		priorityOptions := taskpkg.AllPriorityDisplayValues()

		colors := config.GetColors()
		ev.prioritySelectList = component.NewEditSelectList(priorityOptions, false)
		ev.prioritySelectList.SetLabel(getFocusMarker(colors) + "Priority: ")
		ev.prioritySelectList.SetInitialValue(taskpkg.PriorityDisplay(task.Priority))

		ev.prioritySelectList.SetSubmitHandler(func(text string) {
			if ev.onPrioritySave != nil {
				ev.onPrioritySave(taskpkg.PriorityFromDisplay(text))
			}
			ev.updateValidationState()
		})
	}
	return ev.prioritySelectList
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

func (ev *TaskEditView) ensureDueInput(task *taskpkg.Task) *component.DateEdit {
	if ev.dueInput == nil {
		colors := config.GetColors()
		ev.dueInput = component.NewDateEdit()
		ev.dueInput.SetLabel(getFocusMarker(colors) + "Due:        ")

		ev.dueInput.SetChangeHandler(func(value string) {
			ev.updateValidationState()

			if ev.onDueSave != nil {
				ev.onDueSave(value)
			}
		})

		var initialValue string
		if !task.Due.IsZero() {
			initialValue = task.Due.Format(taskpkg.DateFormat)
		}
		ev.dueInput.SetInitialValue(initialValue)
	}

	return ev.dueInput
}

func (ev *TaskEditView) ensureRecurrenceInput(task *taskpkg.Task) *component.RecurrenceEdit {
	if ev.recurrenceInput == nil {
		colors := config.GetColors()
		ev.recurrenceInput = component.NewRecurrenceEdit()
		ev.recurrenceInput.SetLabel(getFocusMarker(colors) + "Recurrence: ")

		ev.recurrenceInput.SetChangeHandler(func(value string) {
			if ev.onRecurrenceSave != nil {
				ev.onRecurrenceSave(value)
			}

			// sync due widget with auto-computed value from the updated in-memory task
			ev.syncDueFromTask()

			// full refresh needed: tview can't swap a single primitive in a flex layout,
			// so we rebuild to toggle Due between input and read-only text
			ev.refresh()
			ev.updateValidationState()
		})

		ev.recurrenceInput.SetInitialValue(string(task.Recurrence))
	}

	return ev.recurrenceInput
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
