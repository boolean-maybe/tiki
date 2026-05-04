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

	// GetCurrentUser returns the current Tiki identity (name and email).
	// Sourced from configured `identity.*` → git user → OS user.
	GetCurrentUser() (name string, email string, err error)

	// GetStats returns statistics for the header (user, branch, etc.)
	GetStats() []Stat

	// GetAllUsers returns candidate identities for assignee selection.
	// Merges the configured identity with git commit authors when git is enabled;
	// otherwise returns the resolved identity (configured or OS user).
	GetAllUsers() ([]string, error)

	// NewTaskTemplate returns a new task populated with creation defaults
	// from workflow registries (type, status, custom field defaults).
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

	// PathForID returns the on-disk path of the document with the given id,
	// or the empty string when the id is unknown to the store.
	//
	// This is the authoritative resolver for any caller that needs to open,
	// edit, delete, or stage the file for a task: it honors moves and
	// nested layouts, unlike id-derived fallbacks that assume a fixed
	// `.doc/tiki/<id>.md` location. Phase 2 invariant: path is mutable,
	// id is identity — callers must not reconstruct paths themselves.
	PathForID(id string) string
}
