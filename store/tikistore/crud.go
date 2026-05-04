package tikistore

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/document"
	taskpkg "github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// GetTiki retrieves a tiki by ID. Returns nil when not found.
func (s *TikiStore) GetTiki(id string) *tikipkg.Tiki {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tikis[normalizeTaskID(id)]
}

// GetTask retrieves a task by ID. Phase 5 compatibility adapter over GetTiki.
func (s *TikiStore) GetTask(id string) *taskpkg.Task {
	slog.Debug("retrieving task", "task_id", id)
	return tikipkg.ToTask(s.GetTiki(id))
}

// CreateTiki adds a new tiki and saves it to a file.
func (s *TikiStore) CreateTiki(tk *tikipkg.Tiki) error {
	if err := s.createTikiLocked(tk); err != nil {
		return err
	}
	slog.Info("task created", "task_id", tk.ID)
	s.notifyListeners()
	return nil
}

func (s *TikiStore) createTikiLocked(tk *tikipkg.Tiki) error {
	return s.storeNewDocumentLocked(tk)
}

// storeNewDocumentLocked is the shared create path for CreateTask (workflow
// only), CreateTiki, and CreateDocument (workflow or plain). The caller owns
// the workflow-presence decision — this function never adds workflow fields.
func (s *TikiStore) storeNewDocumentLocked(tk *tikipkg.Tiki) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// generate ID if not provided. Identity uniqueness is checked against the
	// in-memory index (s.tikis), not the filesystem — a tiki loaded from a
	// renamed file occupies an id without occupying <dir>/<id>.md, so an
	// os.Stat probe would falsely report the id free.
	if tk.ID == "" {
		for i := 0; ; i++ {
			candidate := normalizeTaskID(config.GenerateRandomID())
			if _, taken := s.tikis[candidate]; !taken {
				tk.ID = candidate
				break
			}
			if i > maxGenerateRetries {
				return fmt.Errorf("failed to generate unique task id after %d attempts", maxGenerateRetries)
			}
			slog.Debug("ID collision detected in index, regenerating", "id", candidate)
		}
	} else {
		tk.ID = normalizeTaskID(tk.ID)
	}

	if err := s.validateDependsOnLocked(tk); err != nil {
		return err
	}

	s.tikis[tk.ID] = tk
	if err := s.saveTiki(tk); err != nil {
		delete(s.tikis, tk.ID)
		slog.Error("failed to save new document after creation", "task_id", tk.ID, "error", err)
		return fmt.Errorf("failed to save task: %w", err)
	}
	return nil
}

// maxGenerateRetries caps the id-generation retry loop so a pathological
// index (e.g. someone wedged the whole 36^6 space) surfaces an error
// instead of looping forever.
const maxGenerateRetries = 100

// CreateTask adds a new task. Phase 5 compatibility adapter over CreateTiki.
func (s *TikiStore) CreateTask(task *taskpkg.Task) error {
	// Honor the caller's IsWorkflow. Phase 7: when a workflow has no default
	// status, NewTaskTemplate returns a plain template (IsWorkflow=false) and
	// piped/ruki capture flows through CreateTask as a plain document.

	// Validate custom fields at the task-shaped boundary.
	if err := validateCustomFields(task.CustomFields); err != nil {
		slog.Error("custom-field validation failed on create", "task_id", task.ID, "error", err)
		return err
	}

	tk := tikipkg.FromTask(task)
	// dependsOn is a referential-integrity constraint validated at store
	// boundary for every document, regardless of IsWorkflow. If the caller
	// passed DependsOn but IsWorkflow=false, FromTask skips it — copy it
	// explicitly so validateDependsOnLocked can enforce the check.
	if len(task.DependsOn) > 0 && !tk.Has("dependsOn") {
		tk.Set("dependsOn", append([]string(nil), task.DependsOn...))
	}
	if err := s.CreateTiki(tk); err != nil {
		return err
	}
	// carry back the generated ID and path
	task.ID = tk.ID
	task.FilePath = tk.Path
	slog.Info("task created", "task_id", task.ID, "status", task.Status)
	return nil
}

// GetAllTikis returns every loaded tiki, including plain docs.
func (s *TikiStore) GetAllTikis() []*tikipkg.Tiki {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*tikipkg.Tiki, 0, len(s.tikis))
	for _, tk := range s.tikis {
		out = append(out, tk)
	}
	return out
}

// UpdateTiki updates an existing tiki and saves it. The incoming tiki's field
// set is authoritative (exact presence): fields absent in tk are not carried
// forward from the stored tiki, so a native or ruki caller that deletes a
// field (e.g. due, assignee) sees that deletion land on disk.
//
// Task-shaped callers that need carry-forward semantics (i.e. a partial Task
// that only sets a few fields) go through UpdateTask, which performs the
// carry-forward merge before projecting to a Tiki and calling UpdateTiki.
func (s *TikiStore) UpdateTiki(tk *tikipkg.Tiki) error {
	if err := s.updateTikiCore(tk, false); err != nil {
		return err
	}
	slog.Info("task updated", "task_id", tk.ID)
	s.notifyListeners()
	return nil
}

// updateTikiCore persists an updated tiki. Path and LoadedMtime are carried
// forward from the stored tiki when absent on the incoming tiki (same as the
// pre-Phase-5 FilePath/LoadedMtime carry-forward). The field map is used as-is:
// no schema-known fields are injected or merged from the stored tiki.
func (s *TikiStore) updateTikiCore(tk *tikipkg.Tiki, _ bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tk.ID = normalizeTaskID(tk.ID)
	old, exists := s.tikis[tk.ID]
	if !exists {
		return fmt.Errorf("task not found: %s", tk.ID)
	}

	if tk.Path == "" {
		tk.Path = old.Path
	}
	if tk.LoadedMtime.IsZero() {
		tk.LoadedMtime = old.LoadedMtime
	}

	if err := s.validateDependsOnLocked(tk); err != nil {
		return err
	}

	s.tikis[tk.ID] = tk
	if err := s.saveTiki(tk); err != nil {
		s.tikis[tk.ID] = old
		slog.Error("failed to save updated task", "task_id", tk.ID, "error", err)
		return fmt.Errorf("failed to save task: %w", err)
	}
	return nil
}

// UpdateTask updates an existing task. Phase 5 compatibility adapter over UpdateTiki.
//
// UpdateTask treats the task as workflow-capable: if the stored tiki has any
// workflow field, those fields are carried forward into the outgoing tiki so a
// caller that rebuilds a Task from only a few fields (title, ID) does not
// silently wipe the rest. Callers that need explicit demotion (removing a
// workflow key) must use UpdateDocument.
func (s *TikiStore) UpdateTask(task *taskpkg.Task) error {
	// Validate custom fields before the tiki projection so the task-shaped
	// contract from pre-Phase-5 (unregistered keys in CustomFields are errors)
	// is preserved. This mirrors the validateCustomFields call that lived in
	// saveTask pre-Phase-5.
	if err := validateCustomFields(task.CustomFields); err != nil {
		slog.Error("custom-field validation failed", "task_id", task.ID, "error", err)
		return err
	}

	s.mu.RLock()
	storedTiki, exists := s.tikis[normalizeTaskID(task.ID)]
	s.mu.RUnlock()

	if exists && hasAnyWorkflowField(storedTiki) {
		// Project the stored tiki to a Task so we have typed values to carry.
		oldTask := tikipkg.ToTask(storedTiki)

		// Grow WorkflowFrontmatter to include every stored workflow field.
		// This ensures FromTask's applyPresenceMap emits all stored fields,
		// not just the subset the caller touched. Without this, a ruki action
		// that only sets status would produce a tiki with only status in
		// Fields, and UpdateTiki's exact-presence save would drop type,
		// priority, and all other stored fields.
		//
		// Keys already in the caller's map are left alone — they represent
		// explicit caller intent (e.g. a newly added field via ruki). New keys
		// added from the stored set are also carried through CarryZeroTypedFields
		// below so the typed Task fields agree with the presence map.
		if task.WorkflowFrontmatter == nil {
			task.WorkflowFrontmatter = make(map[string]interface{}, len(storedTiki.Fields))
		}
		for _, k := range tikipkg.SchemaKnownFields {
			if storedTiki.Has(k) {
				if _, already := task.WorkflowFrontmatter[k]; !already {
					task.WorkflowFrontmatter[k] = struct{}{}
				}
			}
		}

		// Carry stored typed values for any field the caller left at zero
		// that is now declared in the grown WorkflowFrontmatter.
		taskpkg.CarryZeroTypedFields(task, oldTask)

		// Promote before merging so a fresh compat caller (IsWorkflow=false) that
		// sets a new non-zero field (e.g. Priority: 1) still gets it merged into
		// WorkflowFrontmatter. MergeTypedWorkflowDeltas is grow-only and never
		// removes stored keys.
		task.IsWorkflow = true
		taskpkg.MergeTypedWorkflowDeltas(task, oldTask)
	}

	tk := tikipkg.FromTask(task)

	if err := s.UpdateTiki(tk); err != nil {
		return err
	}
	// carry back path for callers that inspect task.FilePath after update
	task.FilePath = tk.Path
	slog.Info("task updated", "task_id", task.ID, "status", task.Status)
	return nil
}

// DeleteTiki removes a tiki and its file.
func (s *TikiStore) DeleteTiki(id string) {
	normalizedID := normalizeTaskID(id)
	if !s.deleteTikiLocked(normalizedID) {
		return
	}
	slog.Info("task deleted", "task_id", normalizedID)
	s.notifyListeners()
}

// deleteTikiLocked removes the tiki file and in-memory entry.
// Returns true if the tiki was deleted.
func (s *TikiStore) deleteTikiLocked(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.tikis[id]
	if !exists {
		return false
	}

	// use the loaded file path so a renamed/moved file still gets cleaned up.
	path := s.pathForTiki(existing)

	// try git rm first if git is available
	removed := false
	if s.gitUtil != nil {
		if err := s.gitUtil.Remove(path); err == nil {
			removed = true
		} else {
			slog.Debug("failed to git remove task file, falling back to os.Remove", "task_id", id, "path", path, "error", err)
		}
	}

	// fall back to os.Remove if git rm failed or unavailable
	if !removed {
		if err := os.Remove(path); err != nil {
			slog.Error("file deletion failed, task preserved in memory", "task_id", id, "path", path, "error", err)
			return false
		}
	}

	delete(s.tikis, id)
	return true
}

// DeleteTask removes a task and its file. Phase 5 compatibility adapter over DeleteTiki.
func (s *TikiStore) DeleteTask(id string) {
	s.DeleteTiki(id)
}

// AddComment adds a comment to a task.
// Comments are stored in memory only for TikiStore.
func (s *TikiStore) AddComment(taskID string, comment taskpkg.Comment) bool {
	if !s.addCommentLocked(taskID, comment) {
		return false
	}
	s.notifyListeners()
	return true
}

func (s *TikiStore) addCommentLocked(taskID string, comment taskpkg.Comment) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskID = normalizeTaskID(taskID)
	tk, exists := s.tikis[taskID]
	if !exists {
		return false
	}

	// comments are stored in the tiki's Fields under a "comments" key.
	// For backward compatibility the comment list lives on the Task, so
	// we project the tiki to a task, append the comment, and update via
	// the task adapter path. This avoids duplicating comment serialization
	// logic; Phase 6 can lift this once comments move to Fields.
	task := tikipkg.ToTask(tk)
	if task == nil {
		return false
	}
	task.Comments = append(task.Comments, comment)
	updated := tikipkg.FromTask(task)
	updated.Path = tk.Path
	updated.LoadedMtime = tk.LoadedMtime
	s.tikis[taskID] = updated
	return true
}

// validateDependsOnLocked checks that all dependsOn IDs reference existing
// documents in the store. References do not need to be workflow-capable —
// any loaded document (plain or workflow) is a valid target per Phase 5
// semantics. IDs must be bare (^[A-Z0-9]{6}$); legacy TIKI-prefixed IDs are
// rejected.
// Caller must hold s.mu lock.
func (s *TikiStore) validateDependsOnLocked(tk *tikipkg.Tiki) error {
	deps, _, _ := tk.StringSliceField("dependsOn")
	for _, depID := range deps {
		normalized := normalizeTaskID(depID)
		if !document.IsValidID(normalized) {
			return fmt.Errorf("dependsOn reference %q is not a bare document id (expected %d uppercase alphanumeric chars)", normalized, document.IDLength)
		}
		if _, exists := s.tikis[normalized]; !exists {
			return fmt.Errorf("dependsOn references non-existent document: %s", normalized)
		}
	}
	return nil
}
