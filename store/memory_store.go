package store

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store/internal/git"
	"github.com/boolean-maybe/tiki/task"
	collectionutil "github.com/boolean-maybe/tiki/util/collections"
	"github.com/boolean-maybe/tiki/workflow"
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

// CreateTask adds a new task to the store. CreateTask is a workflow
// creation path, so the resulting task is always workflow-capable. The
// presence-aware document-first entry point is CreateDocument, which calls
// storeNewDocumentLocked directly.
func (s *InMemoryStore) CreateTask(newTask *task.Task) error {
	newTask.IsWorkflow = true
	return s.storeNewDocumentLocked(newTask)
}

// storeNewDocumentLocked is the shared create path for CreateTask (workflow
// only) and CreateDocument (workflow or plain). The caller owns the
// IsWorkflow decision.
func (s *InMemoryStore) storeNewDocumentLocked(newTask *task.Task) error {
	s.mu.Lock()
	task.NormalizeCollectionFields(newTask)
	now := time.Now()
	newTask.CreatedAt = now
	newTask.UpdatedAt = now
	newTask.ID = normalizeTaskID(newTask.ID)
	s.tasks[newTask.ID] = newTask
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

// UpdateTask updates an existing task. Protects against silent workflow
// demotion (see TikiStore.UpdateTask): callers that pass IsWorkflow=false
// over a workflow-flagged stored task have the flag carried forward.
// Document-first callers that need explicit demotion use UpdateDocument.
func (s *InMemoryStore) UpdateTask(updatedTask *task.Task) error {
	return s.updateLocked(updatedTask, true)
}

// updateLocked is the shared update path. carryWorkflow controls whether a
// caller passing IsWorkflow=false over a workflow-flagged stored task has
// their value forced back to true — true for task-shaped callers
// (protective), false for document-first callers (explicit demotion works).
func (s *InMemoryStore) updateLocked(updatedTask *task.Task, carryWorkflow bool) error {
	s.mu.Lock()

	task.NormalizeCollectionFields(updatedTask)
	updatedTask.ID = normalizeTaskID(updatedTask.ID)
	oldTask, exists := s.tasks[updatedTask.ID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("task not found: %s", updatedTask.ID)
	}

	if carryWorkflow && !updatedTask.IsWorkflow && oldTask.IsWorkflow {
		updatedTask.IsWorkflow = true
	}

	updatedTask.UpdatedAt = time.Now()
	s.tasks[updatedTask.ID] = updatedTask
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

// GetAllTasks returns workflow-capable tasks. Plain docs (IsWorkflow=false)
// are filtered out so board/list/burndown surfaces and any other consumer of
// the task-shaped compatibility API see only workflow items — matching
// TikiStore.GetAllTasks. Document-first callers that want the unfiltered
// view should use GetAllDocuments.
func (s *InMemoryStore) GetAllTasks() []*task.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*task.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		if !t.IsWorkflow {
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks
}

// Search searches workflow tasks with an optional caller-supplied filter.
// Mirrors TikiStore.Search semantics: nil filter means "workflow tasks
// only"; a non-nil filter is trusted as-is. See TikiStore.Search for the
// presence-aware rationale.
func (s *InMemoryStore) Search(query string, filterFunc func(*task.Task) bool) []task.SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.TrimSpace(query)
	queryLower := strings.ToLower(query)
	var results []task.SearchResult

	for _, t := range s.tasks {
		if filterFunc != nil {
			if !filterFunc(t) {
				continue
			}
		} else if !t.IsWorkflow {
			// nil filter → workflow tasks only, matching GetAllTasks.
			continue
		}

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

// PathForID returns the in-memory task's FilePath if set, or the empty
// string. InMemoryStore has no filesystem backing, so production callers
// should not rely on this returning a meaningful value — it exists to
// satisfy the ReadStore interface. Tests that want the real path resolver
// use the TikiStore backing implementation.
func (s *InMemoryStore) PathForID(id string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if t, ok := s.tasks[normalizeTaskID(id)]; ok {
		return t.FilePath
	}
	return ""
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
		// Post-Phase-1: bare uppercase IDs; normalize for safety in case a
		// test injects a generator that returns a non-canonical string.
		taskID = normalizeTaskID(s.idGenerator())
		if _, exists := s.tasks[taskID]; !exists {
			break
		}
		taskID = "" // mark as failed so we can detect exhaustion
	}
	if taskID == "" {
		return nil, fmt.Errorf("failed to generate unique task ID after %d attempts", maxIDAttempts)
	}

	t := &task.Task{
		ID:         taskID,
		Type:       task.DefaultType(),
		Status:     task.DefaultStatus(),
		Priority:   3,
		Points:     1,
		Tags:       []string{"idea"},
		CreatedAt:  time.Now(),
		CreatedBy:  "memory-user",
		IsWorkflow: true,
	}

	t.CustomFields = buildMemoryFieldDefaults()

	return t, nil
}

// buildMemoryFieldDefaults collects DefaultValue from custom field defs.
func buildMemoryFieldDefaults() map[string]interface{} {
	var defaults map[string]interface{}
	for _, fd := range workflow.Fields() {
		if !fd.Custom || fd.DefaultValue == nil {
			continue
		}
		if defaults == nil {
			defaults = make(map[string]interface{})
		}
		if ss, ok := fd.DefaultValue.([]string); ok {
			switch fd.Type {
			case workflow.TypeListRef:
				defaults[fd.Name] = collectionutil.NormalizeRefSet(ss)
			case workflow.TypeListString:
				defaults[fd.Name] = collectionutil.NormalizeStringSet(ss)
			default:
				cp := make([]string, len(ss))
				copy(cp, ss)
				defaults[fd.Name] = cp
			}
		} else {
			defaults[fd.Name] = fd.DefaultValue
		}
	}
	return defaults
}

// ensure InMemoryStore implements Store
var _ Store = (*InMemoryStore)(nil)
