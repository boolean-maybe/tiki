package store

import (
	"github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// ReadStore is the read-only subset of Store.
// Consumers that only need to query tasks should depend on this interface.
type ReadStore interface {
	// GetTask retrieves a task by ID
	GetTask(id string) *task.Task

	// GetAllTasks returns all tikis projected to tasks. Plain docs are included;
	// callers that want only workflow-capable items should use GetAllTikis and
	// filter by has(status) / hasAnyWorkflowField as appropriate.
	GetAllTasks() []*task.Task

	// GetTiki retrieves a tiki by ID. Returns nil when not found.
	GetTiki(id string) *tikipkg.Tiki

	// GetAllTikis returns every loaded tiki, including plain docs.
	GetAllTikis() []*tikipkg.Tiki

	// NewTikiTemplate returns a new tiki populated with creation defaults.
	NewTikiTemplate() (*tikipkg.Tiki, error)

	// Search searches workflow tasks with optional filter function.
	// query: case-insensitive search term (searches task IDs, titles, descriptions, and tags)
	// filterFunc: optional filter function to pre-filter tasks (nil = workflow tasks only)
	// Returns matching tasks sorted by priority then title with relevance scores.
	Search(query string, filterFunc func(*task.Task) bool) []task.SearchResult

	// SearchTikis searches all tikis (including plain docs) with an optional
	// tiki-native filter. query matches against id, title, and body.
	// filter is applied before the text match; nil means no pre-filter.
	// Results are sorted by title then id.
	SearchTikis(query string, filter func(*tikipkg.Tiki) bool) []*tikipkg.Tiki

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
