package store

import (
	"github.com/boolean-maybe/tiki/task"
)

// ReadStore is the read-only subset of Store.
// Consumers that only need to query tasks should depend on this interface.
type ReadStore interface {
	// GetTask retrieves a task by ID
	GetTask(id string) *task.Task

	// GetAllTasks returns all tasks
	GetAllTasks() []*task.Task

	// Search searches tasks with optional filter function.
	// query: case-insensitive search term (searches task IDs, titles, descriptions, and tags)
	// filterFunc: optional filter function to pre-filter tasks (nil = all tasks)
	// Returns matching tasks sorted by ID with relevance scores.
	Search(query string, filterFunc func(*task.Task) bool) []task.SearchResult

	// GetCurrentUser returns the current git user name and email
	GetCurrentUser() (name string, email string, err error)

	// GetStats returns statistics for the header (user, branch, etc.)
	GetStats() []Stat

	// GetBurndown returns the burndown chart data
	GetBurndown() []BurndownPoint

	// GetAllUsers returns list of all git users for assignee selection
	GetAllUsers() ([]string, error)

	// NewTaskTemplate returns a new task populated with template defaults from new.md.
	// The task will have an auto-generated ID, git author, and all fields from the template.
	NewTaskTemplate() (*task.Task, error)

	// AddListener registers a callback for change notifications.
	// returns a listener ID that can be used to remove the listener.
	AddListener(listener ChangeListener) int

	// RemoveListener removes a previously registered listener by ID
	RemoveListener(id int)

	// Reload reloads all data from the backing store
	Reload() error

	// ReloadTask reloads a single task from disk by ID
	ReloadTask(taskID string) error
}
