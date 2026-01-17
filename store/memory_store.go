package store

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/boolean-maybe/tiki/store/internal/git"
	"github.com/boolean-maybe/tiki/task"
)

// InMemoryStore is an in-memory implementation of Store.
// Useful for testing and as a reference implementation.

// InMemoryStore is an in-memory task repository
type InMemoryStore struct {
	mu             sync.RWMutex
	tasks          map[string]*task.Task
	listeners      map[int]ChangeListener
	nextListenerID int
}

// NewInMemoryStore creates a new in-memory task store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		tasks:          make(map[string]*task.Task),
		listeners:      make(map[int]ChangeListener),
		nextListenerID: 1, // Start at 1 to avoid conflict with zero-value sentinel
	}
}

// AddListener registers a callback for change notifications.
// returns a listener ID that can be used to remove the listener.
func (s *InMemoryStore) AddListener(listener ChangeListener) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextListenerID
	s.nextListenerID++
	s.listeners[id] = listener
	return id
}

// RemoveListener removes a previously registered listener by ID
func (s *InMemoryStore) RemoveListener(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.listeners, id)
}

// notifyListeners calls all registered listeners
func (s *InMemoryStore) notifyListeners() {
	s.mu.RLock()
	listeners := make([]ChangeListener, 0, len(s.listeners))
	for _, l := range s.listeners {
		listeners = append(listeners, l)
	}
	s.mu.RUnlock()

	for _, l := range listeners {
		l()
	}
}

// CreateTask adds a new task to the store
func (s *InMemoryStore) CreateTask(task *task.Task) error {
	s.mu.Lock()

	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now
	s.tasks[task.ID] = task
	s.mu.Unlock()
	s.notifyListeners()
	return nil
}

// GetTask retrieves a task by ID
func (s *InMemoryStore) GetTask(id string) *task.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[id]
}

// UpdateTask updates an existing task
func (s *InMemoryStore) UpdateTask(task *task.Task) error {
	s.mu.Lock()

	if _, exists := s.tasks[task.ID]; !exists {
		s.mu.Unlock()
		return fmt.Errorf("task not found: %s", task.ID)
	}

	task.UpdatedAt = time.Now()
	s.tasks[task.ID] = task
	s.mu.Unlock()
	s.notifyListeners()
	return nil
}

// UpdateStatus changes a task's status (with validation)
func (s *InMemoryStore) UpdateStatus(taskID string, newStatus task.Status) bool {
	s.mu.Lock()

	task, exists := s.tasks[taskID]
	if !exists {
		s.mu.Unlock()
		return false
	}

	// validate transition (could add more rules here)
	if !isValidTransition(task.Status, newStatus) {
		s.mu.Unlock()
		return false
	}

	task.Status = newStatus
	task.UpdatedAt = time.Now()
	s.mu.Unlock()
	s.notifyListeners()
	return true
}

// isValidTransition checks if a status transition is allowed
func isValidTransition(from, to task.Status) bool {
	// for now, allow all transitions
	// can add business rules here (e.g., can't go from done to backlog)
	return from != to
}

// DeleteTask removes a task from the store
func (s *InMemoryStore) DeleteTask(id string) {
	s.mu.Lock()
	delete(s.tasks, id)
	s.mu.Unlock()
	s.notifyListeners()
}

// GetAllTasks returns all tasks
func (s *InMemoryStore) GetAllTasks() []*task.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*task.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// GetTasksByStatus returns tasks filtered by status
func (s *InMemoryStore) GetTasksByStatus(status task.Status) []*task.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	targetColumn := task.StatusColumn(status)

	var tasks []*task.Task
	for _, t := range s.tasks {
		if task.StatusColumn(t.Status) == targetColumn {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

// GetBacklogTasks returns tasks with backlog status
func (s *InMemoryStore) GetBacklogTasks() []*task.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []*task.Task
	for _, t := range s.tasks {
		if task.StatusColumn(t.Status) == task.StatusBacklog {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

// SearchBacklog searches backlog tasks by title (case-insensitive).
// Returns results with Score for relevance (currently all 1.0).
func (s *InMemoryStore) SearchBacklog(query string) []task.SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.TrimSpace(query)
	var results []task.SearchResult

	for _, t := range s.tasks {
		if task.StatusColumn(t.Status) == task.StatusBacklog {
			if query == "" || strings.Contains(strings.ToLower(t.Title), strings.ToLower(query)) {
				results = append(results, task.SearchResult{Task: t, Score: 1.0})
			}
		}
	}
	return results
}

// Search searches tasks with optional filter function (simplified in-memory version)
func (s *InMemoryStore) Search(query string, filterFunc func(*task.Task) bool) []task.SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.TrimSpace(query)
	queryLower := strings.ToLower(query)
	var results []task.SearchResult

	for _, t := range s.tasks {
		// Apply filter function (or include all if nil)
		if filterFunc != nil && !filterFunc(t) {
			continue
		}

		// Apply query filter
		if queryLower == "" || strings.Contains(strings.ToLower(t.Title), queryLower) {
			results = append(results, task.SearchResult{Task: t, Score: 1.0})
		}
	}

	return results
}

// AddComment adds a comment to a task
func (s *InMemoryStore) AddComment(taskID string, comment task.Comment) bool {
	s.mu.Lock()

	task, exists := s.tasks[taskID]
	if !exists {
		s.mu.Unlock()
		return false
	}

	comment.CreatedAt = time.Now()
	task.Comments = append(task.Comments, comment)
	task.UpdatedAt = time.Now()
	s.mu.Unlock()
	s.notifyListeners()
	return true
}

// Reload is a no-op for in-memory store (no disk backing)
func (s *InMemoryStore) Reload() error {
	s.notifyListeners()
	return nil
}

// ReloadTask reloads a single task (no-op for memory store)
func (s *InMemoryStore) ReloadTask(taskID string) error {
	// In-memory store doesn't have external storage to reload from
	s.notifyListeners()
	return nil
}

// GetCurrentUser returns a placeholder user (MemoryStore has no git integration)
func (s *InMemoryStore) GetCurrentUser() (name string, email string, err error) {
	return "memory-user", "", nil
}

// GetStats returns placeholder statistics for the header
func (s *InMemoryStore) GetStats() []Stat {
	return []Stat{
		{Name: "User", Value: "memory-user", Order: 3},
		{Name: "Branch", Value: "memory", Order: 4},
	}
}

// GetBurndown returns nil for MemoryStore (no history tracking)
func (s *InMemoryStore) GetBurndown() []BurndownPoint {
	return nil
}

// GetAllUsers returns a placeholder user list for MemoryStore
func (s *InMemoryStore) GetAllUsers() ([]string, error) {
	return []string{"memory-user"}, nil
}

// GetGitOps returns nil for in-memory store (no git operations)
func (s *InMemoryStore) GetGitOps() git.GitOps {
	return nil
}

// NewTaskTemplate returns a new task with hardcoded defaults.
// MemoryStore doesn't load templates from files.
func (s *InMemoryStore) NewTaskTemplate() (*task.Task, error) {
	task := &task.Task{
		ID:          "", // Caller must set ID
		Title:       "",
		Description: "",
		Type:        task.TypeStory,
		Status:      task.StatusBacklog,
		Priority:    7, // Match embedded template default
		Points:      1,
		Tags:        []string{"idea"},
		CreatedAt:   time.Now(),
		CreatedBy:   "memory-user",
	}
	return task, nil
}

// ensure InMemoryStore implements Store
var _ Store = (*InMemoryStore)(nil)
