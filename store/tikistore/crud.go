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

	// CreateTask is a workflow creation path by definition — everything that
	// arrives here is a workflow item, even if the caller left IsWorkflow
	// zero-valued. Plain docs are created by other means (editing files on
	// disk without workflow fields).
	task.IsWorkflow = true

	s.tasks[task.ID] = task
	if err := s.saveTask(task); err != nil {
		delete(s.tasks, task.ID)
		slog.Error("failed to save new task after creation", "task_id", task.ID, "error", err)
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

// UpdateTask updates an existing task and saves it
func (s *TikiStore) UpdateTask(task *taskpkg.Task) error {
	if err := s.updateTaskLocked(task); err != nil {
		return err
	}
	slog.Info("task updated", "task_id", task.ID, "status", task.Status)
	s.notifyListeners()
	return nil
}

func (s *TikiStore) updateTaskLocked(task *taskpkg.Task) error {
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
	if !task.IsWorkflow && oldTask.IsWorkflow {
		task.IsWorkflow = true
	}

	if err := s.validateDependsOnLocked(task); err != nil {
		return err
	}

	s.tasks[task.ID] = task
	if err := s.saveTask(task); err != nil {
		s.tasks[task.ID] = oldTask
		slog.Error("failed to save updated task", "task_id", task.ID, "error", err)
		return fmt.Errorf("failed to save task: %w", err)
	}
	return nil
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
