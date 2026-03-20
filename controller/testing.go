package controller

import (
	"github.com/boolean-maybe/tiki/task"
)

// Test utilities for controller unit tests

// newMockNavigationController creates a new mock navigation controller
func newMockNavigationController() *NavigationController {
	return &NavigationController{
		app:      nil, // unit tests don't need the tview.Application
		navState: newViewStack(),
	}
}

// Test fixtures

// newTestTask creates a test task with default values
func newTestTask() *task.Task {
	return &task.Task{
		ID:       "TIKI-1",
		Title:    "Test Task",
		Status:   task.StatusReady,
		Type:     task.TypeStory,
		Priority: 3,
		Points:   5,
	}
}

// newTestTaskWithID creates a test task with ID "DRAFT-1"
func newTestTaskWithID() *task.Task {
	t := newTestTask()
	t.ID = "DRAFT-1"
	return t
}
