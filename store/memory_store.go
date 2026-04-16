package store

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/boolean-maybe/tiki/config"
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
	idGenerator    func() string // injectable for testing; defaults to config.GenerateRandomID
}

func normalizeTaskID(id string) string {
	return strings.ToUpper(strings.TrimSpace(id))
}

// NewInMemoryStore creates a new in-memory task store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		tasks:          make(map[string]*task.Task),
		listeners:      make(map[int]ChangeListener),
		nextListenerID: 1, // Start at 1 to avoid conflict with zero-value sentinel
		idGenerator:    config.GenerateRandomID,
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
	task.ID = normalizeTaskID(task.ID)
	s.tasks[task.ID] = task
	s.mu.Unlock()
	s.notifyListeners()
	return nil
}

// GetTask retrieves a task by ID
func (s *InMemoryStore) GetTask(id string) *task.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[normalizeTaskID(id)]
}

// UpdateTask updates an existing task
func (s *InMemoryStore) UpdateTask(task *task.Task) error {
	s.mu.Lock()

	task.ID = normalizeTaskID(task.ID)
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

// DeleteTask removes a task from the store
func (s *InMemoryStore) DeleteTask(id string) {
	s.mu.Lock()
	delete(s.tasks, normalizeTaskID(id))
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
		if queryLower == "" || matchesQueryInMemory(t, queryLower) {
			results = append(results, task.SearchResult{Task: t, Score: 1.0})
		}
	}

	return results
}

func matchesQueryInMemory(t *task.Task, queryLower string) bool {
	if strings.Contains(strings.ToLower(t.ID), queryLower) ||
		strings.Contains(strings.ToLower(t.Title), queryLower) ||
		strings.Contains(strings.ToLower(t.Description), queryLower) {
		return true
	}
	for _, tag := range t.Tags {
		if strings.Contains(strings.ToLower(tag), queryLower) {
			return true
		}
	}
	return false
}

// AddComment adds a comment to a task
func (s *InMemoryStore) AddComment(taskID string, comment task.Comment) bool {
	s.mu.Lock()

	taskID = normalizeTaskID(taskID)
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

const maxIDAttempts = 100

// NewTaskTemplate returns a new task with hardcoded defaults and an auto-generated ID.
func (s *InMemoryStore) NewTaskTemplate() (*task.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var taskID string
	for range maxIDAttempts {
		taskID = normalizeTaskID(fmt.Sprintf("TIKI-%s", s.idGenerator()))
		if _, exists := s.tasks[taskID]; !exists {
			break
		}
		taskID = "" // mark as failed so we can detect exhaustion
	}
	if taskID == "" {
		return nil, fmt.Errorf("failed to generate unique task ID after %d attempts", maxIDAttempts)
	}

	t := &task.Task{
		ID:           taskID,
		Title:        "",
		Description:  "",
		Type:         task.DefaultType(),
		Status:       task.DefaultStatus(),
		Priority:     3,
		Points:       1,
		Tags:         []string{"idea"},
		CustomFields: make(map[string]interface{}),
		CreatedAt:    time.Now(),
		CreatedBy:    "memory-user",
	}
	return t, nil
}

// ensure InMemoryStore implements Store
var _ Store = (*InMemoryStore)(nil)
