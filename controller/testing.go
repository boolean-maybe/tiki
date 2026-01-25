package controller

import (
	"github.com/boolean-maybe/tiki/task"
)

// Test utilities for controller unit tests

// newMockNavigationController creates a new mock navigation controller
func newMockNavigationController() *NavigationController {
	// Create a real NavigationController but we won't use most of its methods in tests
	// The key is that TaskController only calls SuspendAndEdit which we can ignore in tests
	return &NavigationController{
		// minimal initialization - only used to satisfy type checking
		app: nil, // Unit tests don't need the tview.Application
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
