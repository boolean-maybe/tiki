package store

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store/internal/git"
	"github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	collectionutil "github.com/boolean-maybe/tiki/util/collections"
	"github.com/boolean-maybe/tiki/workflow"
)

// InMemoryStore is an in-memory implementation of Store.
// Useful for testing and as a reference implementation.

// InMemoryStore is an in-memory task repository
type InMemoryStore struct {
	mu             sync.RWMutex
	tikis          map[string]*tikipkg.Tiki
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
		tikis:          make(map[string]*tikipkg.Tiki),
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

// GetTiki retrieves a tiki by ID. Returns nil when not found.
func (s *InMemoryStore) GetTiki(id string) *tikipkg.Tiki {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tikis[normalizeTaskID(id)]
}

// GetAllTikis returns every loaded tiki, including plain docs.
func (s *InMemoryStore) GetAllTikis() []*tikipkg.Tiki {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*tikipkg.Tiki, 0, len(s.tikis))
	for _, tk := range s.tikis {
		out = append(out, tk)
	}
	return out
}

// CreateTiki adds a new tiki to the store.
func (s *InMemoryStore) CreateTiki(tk *tikipkg.Tiki) error {
	return s.storeNewTikiLocked(tk)
}

// UpdateTiki updates an existing tiki. The incoming tiki's field set is
// authoritative (exact presence): fields absent in tk are not carried forward
// from the stored tiki, so native callers that delete a field see it removed.
func (s *InMemoryStore) UpdateTiki(tk *tikipkg.Tiki) error {
	return s.updateTikiLocked(tk, false)
}

// DeleteTiki removes a tiki from the store.
func (s *InMemoryStore) DeleteTiki(id string) {
	s.mu.Lock()
	delete(s.tikis, normalizeTaskID(id))
	s.mu.Unlock()
	s.notifyListeners()
}

// NewTikiTemplate returns a new tiki populated with creation defaults.
func (s *InMemoryStore) NewTikiTemplate() (*tikipkg.Tiki, error) {
	t, err := s.NewTaskTemplate()
	if err != nil {
		return nil, err
	}
	return tikipkg.FromTask(t), nil
}

// storeNewTikiLocked is the shared create path. The caller owns the
// workflow-presence decision.
func (s *InMemoryStore) storeNewTikiLocked(tk *tikipkg.Tiki) error {
	s.mu.Lock()
	now := time.Now()
	tk.CreatedAt = now
	tk.UpdatedAt = now
	tk.ID = normalizeTaskID(tk.ID)
	s.tikis[tk.ID] = tk
	s.mu.Unlock()
	s.notifyListeners()
	return nil
}

// updateTikiLocked is the shared update path. carryWorkflow controls whether
// a stored tiki with workflow fields should force workflow fields onto an
// incoming tiki that has none — true for task-shaped callers (protective),
// false for document-first callers (explicit demotion works).
func (s *InMemoryStore) updateTikiLocked(tk *tikipkg.Tiki, carryWorkflow bool) error {
	s.mu.Lock()

	tk.ID = normalizeTaskID(tk.ID)
	old, exists := s.tikis[tk.ID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("task not found: %s", tk.ID)
	}

	// protective carry-forward: if the stored tiki has workflow fields and
	// the incoming tiki has none, carry the stored workflow fields forward
	// so callers that only update one field don't accidentally demote.
	if carryWorkflow && !hasAnyWorkflowFieldMem(tk) && hasAnyWorkflowFieldMem(old) {
		for _, k := range tikipkg.SchemaKnownFields {
			if !tk.Has(k) {
				if v, ok := old.Fields[k]; ok {
					tk.Set(k, v)
				}
			}
		}
	}

	tk.UpdatedAt = time.Now()
	s.tikis[tk.ID] = tk
	s.mu.Unlock()
	s.notifyListeners()
	return nil
}

// hasAnyWorkflowFieldMem is the InMemoryStore equivalent of tikistore.hasAnyWorkflowField.
func hasAnyWorkflowFieldMem(tk *tikipkg.Tiki) bool {
	if tk == nil {
		return false
	}
	for _, f := range tikipkg.SchemaKnownFields {
		if tk.Has(f) {
			return true
		}
	}
	return false
}

// CreateTask adds a new task to the store. InMemoryStore is a test fixture
// and deliberately keeps the pre-Phase-7 "always workflow" semantics so that
// ~150 existing call sites don't need to set IsWorkflow explicitly. The
// production TikiStore (tikistore.TikiStore.CreateTask) honors the caller's
// IsWorkflow so piped/ruki capture against workflows with no default status
// produces plain documents. Tests that specifically need plain-doc
// semantics should call CreateDocument instead.
//
// The original task struct is updated in-place with the normalized ID and
// timestamps so callers that inspect the task after creation see the
// canonical values (matching the previous map[string]*task.Task behavior).
func (s *InMemoryStore) CreateTask(newTask *task.Task) error {
	newTask.IsWorkflow = true
	task.NormalizeCollectionFields(newTask)
	tk := tikipkg.FromTask(newTask)
	if err := s.storeNewTikiLocked(tk); err != nil {
		return err
	}
	// carry normalized values back to the caller's task struct
	newTask.ID = tk.ID
	newTask.CreatedAt = tk.CreatedAt
	newTask.UpdatedAt = tk.UpdatedAt
	return nil
}

// storeNewDocumentLocked is kept for document-first callers (CreateDocument).
// The caller owns the IsWorkflow decision.
func (s *InMemoryStore) storeNewDocumentLocked(tk *tikipkg.Tiki) error {
	return s.storeNewTikiLocked(tk)
}

// GetTask retrieves a task by ID. Phase 5 compatibility adapter over GetTiki.
func (s *InMemoryStore) GetTask(id string) *task.Task {
	return tikipkg.ToTask(s.GetTiki(id))
}

// UpdateTask updates an existing task. Protects against silent workflow
// demotion (see TikiStore.UpdateTask): callers that pass IsWorkflow=false
// over a workflow-flagged stored task have the flag carried forward.
// Document-first callers that need explicit demotion use UpdateDocument.
// The original task struct is updated in-place with the new UpdatedAt
// timestamp so callers that inspect the task after update see the canonical value.
func (s *InMemoryStore) UpdateTask(updatedTask *task.Task) error {
	if storedTiki := s.GetTiki(updatedTask.ID); storedTiki != nil && hasAnyWorkflowFieldMem(storedTiki) {
		oldTask := tikipkg.ToTask(storedTiki)
		// Grow WorkflowFrontmatter to include every stored workflow field so
		// FromTask's applyPresenceMap emits all stored fields, not just the
		// subset the caller happened to set. See TikiStore.UpdateTask for the
		// full rationale.
		if updatedTask.WorkflowFrontmatter == nil {
			updatedTask.WorkflowFrontmatter = make(map[string]interface{}, len(storedTiki.Fields))
		}
		for _, k := range tikipkg.SchemaKnownFields {
			if storedTiki.Has(k) {
				if _, already := updatedTask.WorkflowFrontmatter[k]; !already {
					updatedTask.WorkflowFrontmatter[k] = struct{}{}
				}
			}
		}
		task.CarryZeroTypedFields(updatedTask, oldTask)
		// promote before merging so a fresh compat caller with IsWorkflow=false
		// still gets newly non-zero fields merged into WorkflowFrontmatter.
		updatedTask.IsWorkflow = true
		task.MergeTypedWorkflowDeltas(updatedTask, oldTask)
	}
	tk := tikipkg.FromTask(updatedTask)
	if err := s.updateTikiLocked(tk, true); err != nil {
		return err
	}
	updatedTask.UpdatedAt = tk.UpdatedAt
	return nil
}

// updateLocked is kept as a document-first update path.
// carryWorkflow=false means no protective carry-forward, so explicit demotion works.
func (s *InMemoryStore) updateLocked(tk *tikipkg.Tiki, carryWorkflow bool) error {
	return s.updateTikiLocked(tk, carryWorkflow)
}

// DeleteTask removes a task from the store
func (s *InMemoryStore) DeleteTask(id string) {
	s.DeleteTiki(id)
}

// GetAllTasks returns all tikis projected to tasks. Plain docs are included;
// callers that want only workflow-capable items should filter by
// hasAnyWorkflowFieldMem or use a ruki query with has(status).
func (s *InMemoryStore) GetAllTasks() []*task.Task {
	tikis := s.GetAllTikis()
	tasks := make([]*task.Task, 0, len(tikis))
	for _, tk := range tikis {
		tasks = append(tasks, tikipkg.ToTask(tk))
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

	for _, tk := range s.tikis {
		t := tikipkg.ToTask(tk)
		if filterFunc != nil {
			if !filterFunc(t) {
				continue
			}
		} else if !hasAnyWorkflowFieldMem(tk) {
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

// SearchTikis searches all tikis (including plain docs) with an optional
// tiki-native filter. query matches against id, title, and body.
// filter is applied before the text match; nil means no pre-filter.
// Results are sorted by title then id.
func (s *InMemoryStore) SearchTikis(query string, filter func(*tikipkg.Tiki) bool) []*tikipkg.Tiki {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryLower := strings.ToLower(strings.TrimSpace(query))
	var results []*tikipkg.Tiki
	for _, tk := range s.tikis {
		if filter != nil && !filter(tk) {
			continue
		}
		if queryLower != "" && !matchesTikiQueryMem(tk, queryLower) {
			continue
		}
		results = append(results, tk)
	}
	sort.Slice(results, func(i, j int) bool {
		ti, tj := strings.ToLower(results[i].Title), strings.ToLower(results[j].Title)
		if ti != tj {
			return ti < tj
		}
		return results[i].ID < results[j].ID
	})
	return results
}

func matchesTikiQueryMem(tk *tikipkg.Tiki, queryLower string) bool {
	return strings.Contains(strings.ToLower(tk.ID), queryLower) ||
		strings.Contains(strings.ToLower(tk.Title), queryLower) ||
		strings.Contains(strings.ToLower(tk.Body), queryLower)
}

// AddComment adds a comment to a task
func (s *InMemoryStore) AddComment(taskID string, comment task.Comment) bool {
	s.mu.Lock()

	taskID = normalizeTaskID(taskID)
	tk, exists := s.tikis[taskID]
	if !exists {
		s.mu.Unlock()
		return false
	}

	// project to task, append comment, project back
	t := tikipkg.ToTask(tk)
	comment.CreatedAt = time.Now()
	t.Comments = append(t.Comments, comment)
	t.UpdatedAt = time.Now()
	updated := tikipkg.FromTask(t)
	updated.Path = tk.Path
	updated.LoadedMtime = tk.LoadedMtime
	s.tikis[taskID] = updated
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

// PathForID returns the in-memory tiki's Path if set, or the empty
// string. InMemoryStore has no filesystem backing, so production callers
// should not rely on this returning a meaningful value — it exists to
// satisfy the ReadStore interface. Tests that want the real path resolver
// use the TikiStore backing implementation.
func (s *InMemoryStore) PathForID(id string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if tk, ok := s.tikis[normalizeTaskID(id)]; ok {
		return tk.Path
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
		if _, exists := s.tikis[taskID]; !exists {
			break
		}
		taskID = "" // mark as failed so we can detect exhaustion
	}
	if taskID == "" {
		return nil, fmt.Errorf("failed to generate unique task ID after %d attempts", maxIDAttempts)
	}

	defaultStatus := task.DefaultStatus()
	t := &task.Task{
		ID:        taskID,
		CreatedAt: time.Now(),
		CreatedBy: "memory-user",
	}

	if defaultStatus != "" {
		t.Status = defaultStatus
		t.Type = task.DefaultType()
		t.Priority = 3
		t.Points = 1
		t.Tags = []string{"idea"}
		t.IsWorkflow = true
		t.CustomFields = buildMemoryFieldDefaults()
	}

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
