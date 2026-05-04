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
	s.mu.RLock()
	defer s.mu.RUnlock()

	var taskID string
	for range maxIDAttempts {
		taskID = normalizeTaskID(s.idGenerator())
		if _, exists := s.tikis[taskID]; !exists {
			break
		}
		taskID = "" // mark as failed so we can detect exhaustion
	}
	if taskID == "" {
		return nil, fmt.Errorf("failed to generate unique task ID after %d attempts", maxIDAttempts)
	}

	tk := tikipkg.New()
	tk.ID = taskID
	tk.CreatedAt = time.Now()

	if defaultStatus := task.DefaultStatus(); defaultStatus != "" {
		tk.Set(tikipkg.FieldStatus, string(defaultStatus))
		tk.Set(tikipkg.FieldType, string(task.DefaultType()))
		tk.Set(tikipkg.FieldPriority, 3)
		tk.Set(tikipkg.FieldPoints, 1)
		tk.Set(tikipkg.FieldTags, []string{"idea"})
		for k, v := range buildMemoryFieldDefaults() {
			tk.Set(k, v)
		}
	}

	if name, email, err := s.GetCurrentUser(); err == nil {
		switch {
		case name != "" && email != "":
			tk.Set("createdBy", fmt.Sprintf("%s <%s>", name, email))
		case name != "":
			tk.Set("createdBy", name)
		case email != "":
			tk.Set("createdBy", email)
		}
	}

	return tk, nil
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

// updateTikiLocked is the shared update path. carrySchemaFields controls
// whether stored schema-known fields should be carried forward onto an
// incoming tiki that omits them — true for task-shaped callers (protective
// against accidental field loss), false for tiki-native callers that rely
// on exact-presence semantics.
func (s *InMemoryStore) updateTikiLocked(tk *tikipkg.Tiki, carrySchemaFields bool) error {
	s.mu.Lock()

	tk.ID = normalizeTaskID(tk.ID)
	old, exists := s.tikis[tk.ID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("task not found: %s", tk.ID)
	}

	// protective carry-forward: if the stored tiki carries schema-known
	// fields and the incoming tiki has none, carry the stored values
	// forward so callers that only update one field don't accidentally
	// drop the rest.
	if carrySchemaFields && !hasAnySchemaFieldMem(tk) && hasAnySchemaFieldMem(old) {
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

// hasAnySchemaFieldMem is the InMemoryStore equivalent of tikistore.hasAnySchemaField.
func hasAnySchemaFieldMem(tk *tikipkg.Tiki) bool {
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
	if strings.Contains(strings.ToLower(tk.ID), queryLower) ||
		strings.Contains(strings.ToLower(tk.Title), queryLower) ||
		strings.Contains(strings.ToLower(tk.Body), queryLower) {
		return true
	}
	tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), queryLower) {
			return true
		}
	}
	return false
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
