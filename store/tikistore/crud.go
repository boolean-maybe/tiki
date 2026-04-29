package tikistore

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
)

// CreateTask adds a new task and saves it to a file
func (s *TikiStore) CreateTask(task *taskpkg.Task) error {
	if err := s.createTaskLocked(task); err != nil {
		return err
	}
	slog.Info("task created", "task_id", task.ID, "status", task.Status)
	s.notifyListeners()
	return nil
}

func (s *TikiStore) createTaskLocked(task *taskpkg.Task) error {
	// CreateTask is a workflow creation path by definition — everything that
	// arrives here is a workflow item, even if the caller left IsWorkflow
	// zero-valued. The presence-aware document-first entry point is
	// CreateDocument, which calls storeNewDocumentLocked directly and keeps
	// plain docs out of the workflow surface.
	task.IsWorkflow = true
	return s.storeNewDocumentLocked(task)
}

// storeNewDocumentLocked is the shared create path for CreateTask (workflow
// only) and CreateDocument (workflow or plain). It allocates an id when
// missing, validates dependsOn, and saves the file. The caller owns the
// IsWorkflow decision — this function never flips it — so the serialized
// frontmatter (workflow schema vs plain schema) matches the caller's intent
// and survives a reload.
func (s *TikiStore) storeNewDocumentLocked(task *taskpkg.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// generate ID if not provided. Identity uniqueness is checked against the
	// in-memory index (s.tasks), not the filesystem — a task loaded from a
	// renamed file occupies an id without occupying <taskdir>/<id>.md, so an
	// os.Stat probe would falsely report the id free.
	if task.ID == "" {
		for i := 0; ; i++ {
			candidate := normalizeTaskID(config.GenerateRandomID())
			if _, taken := s.tasks[candidate]; !taken {
				task.ID = candidate
				break
			}
			if i > maxGenerateRetries {
				return fmt.Errorf("failed to generate unique task id after %d attempts", maxGenerateRetries)
			}
			slog.Debug("ID collision detected in index, regenerating", "id", candidate)
		}
	} else {
		task.ID = normalizeTaskID(task.ID)
	}

	taskpkg.NormalizeCollectionFields(task)

	if err := s.validateDependsOnLocked(task); err != nil {
		return err
	}

	s.tasks[task.ID] = task
	if err := s.saveTask(task); err != nil {
		delete(s.tasks, task.ID)
		slog.Error("failed to save new document after creation", "task_id", task.ID, "error", err)
		return fmt.Errorf("failed to save task: %w", err)
	}
	return nil
}

// maxGenerateRetries caps the id-generation retry loop so a pathological
// index (e.g. someone wedged the whole 36^6 space) surfaces an error
// instead of looping forever.
const maxGenerateRetries = 100

// GetTask retrieves a task by ID
func (s *TikiStore) GetTask(id string) *taskpkg.Task {
	slog.Debug("retrieving task", "task_id", id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[normalizeTaskID(id)]
}

// UpdateTask updates an existing task and saves it.
//
// UpdateTask treats the task as workflow-capable: if the caller passes a
// Task with IsWorkflow=false but the stored task is workflow, the workflow
// flag is carried forward. This protects ruki/UI paths that rebuild a Task
// from a subset of fields and forget to set IsWorkflow — a silent demotion
// would hide the task from board/search views until the next reload.
// Callers that need to explicitly demote (e.g. UpdateDocument stripping all
// workflow frontmatter) must use the document-first API.
func (s *TikiStore) UpdateTask(task *taskpkg.Task) error {
	if err := s.updateTaskLocked(task, true); err != nil {
		return err
	}
	slog.Info("task updated", "task_id", task.ID, "status", task.Status)
	s.notifyListeners()
	return nil
}

// updateTaskLocked is the shared update path. carryWorkflow controls whether
// a caller passing IsWorkflow=false over a workflow-flagged stored task has
// their value forced back to true. Task-shaped callers pass true (protective
// carry-forward); document-first callers pass false so explicit demotion
// works.
func (s *TikiStore) updateTaskLocked(task *taskpkg.Task, carryWorkflow bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task.ID = normalizeTaskID(task.ID)
	taskpkg.NormalizeCollectionFields(task)
	oldTask, exists := s.tasks[task.ID]
	if !exists {
		return fmt.Errorf("task not found: %s", task.ID)
	}

	// Carry forward identity-bound state from the loaded task if the caller
	// passed a fresh Task value. Without this, a caller that constructs a
	// Task from scratch would:
	//   • lose the loaded path, so save would target <id>.md instead of the
	//     renamed file the user is editing — the high finding's escape hatch;
	//   • silently bypass the optimistic-locking mtime check, defeating
	//     external-edit detection;
	//   • silently demote a workflow document to a plain doc by leaving
	//     IsWorkflow at its zero value, which hides the task from
	//     GetAllTasks / search / board views until the next reload.
	// Callers that *want* to reset these can set them explicitly.
	if task.FilePath == "" {
		task.FilePath = oldTask.FilePath
	}
	if task.LoadedMtime.IsZero() {
		task.LoadedMtime = oldTask.LoadedMtime
	}
	if carryWorkflow && !task.IsWorkflow && oldTask.IsWorkflow {
		task.IsWorkflow = true
	}

	if err := s.validateDependsOnLocked(task); err != nil {
		return err
	}

	// Seed the preserved presence set from the stored task when the caller
	// passed a fresh Task that doesn't carry one. Without this, a task-API
	// caller (ruki/UI) who builds a Task from typed fields only would wipe
	// the presence set and cause saveTask to fall back to full-schema
	// synthesis — undoing the sparse-preservation fix.
	if task.IsWorkflow && task.WorkflowFrontmatter == nil && oldTask.WorkflowFrontmatter != nil {
		task.WorkflowFrontmatter = cloneWorkflowFrontmatter(oldTask.WorkflowFrontmatter)
	}

	// Carry forward typed workflow field values from the stored task for any
	// field whose incoming value is zero. Rationale is the same as the
	// FilePath/LoadedMtime/IsWorkflow carry-forward above: task-shaped
	// compatibility callers routinely build a fresh Task carrying only the
	// fields they care about, and every other typed field is zero because
	// the caller simply did not populate them. Without this carry-forward
	// the sparse save would emit the zero values, effectively clearing
	// workflow fields the caller never touched (the root cause the reviewer
	// caught). Callers that intend to CLEAR a field should use the document-
	// first API and remove the key from doc.Frontmatter.
	if task.IsWorkflow {
		carryTypedWorkflowValues(task, oldTask)
		taskpkg.MergeTypedWorkflowDeltas(task, oldTask)
	}

	s.tasks[task.ID] = task
	if err := s.saveTask(task); err != nil {
		s.tasks[task.ID] = oldTask
		slog.Error("failed to save updated task", "task_id", task.ID, "error", err)
		return fmt.Errorf("failed to save task: %w", err)
	}
	return nil
}

// carryTypedWorkflowValues copies typed workflow field values from src into
// dst for any field whose dst value is zero. dst's non-zero fields win
// because they represent an explicit caller edit. Slice fields use a
// non-aliasing copy so subsequent mutations on dst don't reach src.
//
// This is the field-by-field analog of the FilePath/LoadedMtime carry-
// forward in updateTaskLocked. It runs only on the workflow-task path
// because plain docs have no typed workflow fields to preserve.
func carryTypedWorkflowValues(dst, src *taskpkg.Task) {
	if dst == nil || src == nil {
		return
	}
	if dst.Status == "" {
		dst.Status = src.Status
	}
	if dst.Type == "" {
		dst.Type = src.Type
	}
	if dst.Tags == nil && src.Tags != nil {
		dst.Tags = append([]string(nil), src.Tags...)
	}
	if dst.DependsOn == nil && src.DependsOn != nil {
		dst.DependsOn = append([]string(nil), src.DependsOn...)
	}
	if dst.Due.IsZero() {
		dst.Due = src.Due
	}
	if dst.Recurrence == "" {
		dst.Recurrence = src.Recurrence
	}
	if dst.Assignee == "" {
		dst.Assignee = src.Assignee
	}
	if dst.Priority == 0 {
		dst.Priority = src.Priority
	}
	if dst.Points == 0 {
		dst.Points = src.Points
	}
}

// cloneWorkflowFrontmatter returns a shallow copy of a preserved workflow
// frontmatter map. Keys are strings and values are the YAML-decoded scalar
// or slice shapes that do not need deep copies to stay isolated from
// subsequent typed-field mutations.
func cloneWorkflowFrontmatter(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		if ss, ok := v.([]string); ok {
			cp := make([]string, len(ss))
			copy(cp, ss)
			out[k] = cp
		} else {
			out[k] = v
		}
	}
	return out
}

// DeleteTask removes a task and its file
func (s *TikiStore) DeleteTask(id string) {
	normalizedID := normalizeTaskID(id)
	if !s.deleteTaskLocked(normalizedID) {
		return
	}
	slog.Info("task deleted", "task_id", normalizedID)
	s.notifyListeners()
}

// deleteTaskLocked removes the task file and in-memory entry.
// Returns true if the task was deleted.
func (s *TikiStore) deleteTaskLocked(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.tasks[id]
	if !exists {
		return false
	}

	// use the loaded file path so a renamed/moved file still gets cleaned up.
	path := s.pathForTask(existing)

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

	delete(s.tasks, id)
	return true
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
	task, exists := s.tasks[taskID]
	if !exists {
		return false
	}

	task.Comments = append(task.Comments, comment)
	return true
}

// validateDependsOnLocked checks that all dependsOn IDs reference existing tasks.
// Caller must hold s.mu lock.
func (s *TikiStore) validateDependsOnLocked(task *taskpkg.Task) error {
	for _, depID := range task.DependsOn {
		normalized := normalizeTaskID(depID)
		if _, exists := s.tasks[normalized]; !exists {
			return fmt.Errorf("dependsOn references non-existent tiki: %s", normalized)
		}
	}
	return nil
}
