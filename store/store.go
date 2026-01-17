package store

import (
	"github.com/boolean-maybe/tiki/task"
)

// Store is the interface for task storage engines.
// Implementations must be thread-safe and notify listeners on changes.
type Store interface {
	// AddListener registers a callback for change notifications.
	// returns a listener ID that can be used to remove the listener.
	AddListener(listener ChangeListener) int

	// RemoveListener removes a previously registered listener by ID
	RemoveListener(id int)

	// CreateTask adds a new task to the store.
	// Returns error if save fails (IO error, ErrConflict).
	CreateTask(task *task.Task) error

	// GetTask retrieves a task by ID
	GetTask(id string) *task.Task

	// UpdateTask updates an existing task.
	// Returns error if save fails (IO error, ErrConflict).
	UpdateTask(task *task.Task) error

	// UpdateStatus changes a task's status (with validation)
	UpdateStatus(taskID string, newStatus task.Status) bool

	// DeleteTask removes a task from the store
	DeleteTask(id string)

	// GetAllTasks returns all tasks
	GetAllTasks() []*task.Task

	// GetTasksByStatus returns tasks filtered by status
	GetTasksByStatus(status task.Status) []*task.Task

	// GetBacklogTasks returns tasks with backlog status
	GetBacklogTasks() []*task.Task

	// SearchBacklog searches backlog tasks by title (case-insensitive).
	// Returns results in relevance order (Score for future use).
	// Deprecated: Use Search() instead for more flexible searching.
	SearchBacklog(query string) []task.SearchResult

	// Search searches tasks with optional filter function.
	// query: case-insensitive search term (searches task titles)
	// filterFunc: optional filter function to pre-filter tasks (nil = all tasks)
	// Returns matching tasks sorted by ID with relevance scores.
	Search(query string, filterFunc func(*task.Task) bool) []task.SearchResult

	// AddComment adds a comment to a task
	AddComment(taskID string, comment task.Comment) bool

	// Reload reloads all data from the backing store
	Reload() error

	// ReloadTask reloads a single task from disk by ID
	ReloadTask(taskID string) error

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
}

// ChangeListener is called when the store's data changes
type ChangeListener func()

// Stat represents a statistic to be displayed in the header
type Stat struct {
	Name  string
	Value string
	Order int
}
