package store

import (
	"github.com/boolean-maybe/tiki/task"
)

// Store is the interface for task storage engines.
// Implementations must be thread-safe and notify listeners on changes.
type Store interface {
	ReadStore

	// CreateTask adds a new task to the store.
	// Returns error if save fails (IO error, ErrConflict).
	CreateTask(task *task.Task) error

	// UpdateTask updates an existing task.
	// Returns error if save fails (IO error, ErrConflict).
	UpdateTask(task *task.Task) error

	// DeleteTask removes a task from the store
	DeleteTask(id string)

	// AddComment adds a comment to a task
	AddComment(taskID string, comment task.Comment) bool
}

// ChangeListener is called when the store's data changes
type ChangeListener func()

// Stat represents a statistic to be displayed in the header
type Stat struct {
	Name  string
	Value string
	Order int
}
