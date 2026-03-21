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

	// generate ID if not provided
	if task.ID == "" {
		for {
			randomID := config.GenerateRandomID()
			task.ID = fmt.Sprintf("TIKI-%s", randomID)

			path := s.taskFilePath(task.ID)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				break
			}
			slog.Debug("ID collision detected, regenerating", "id", task.ID)
		}
	}

	task.ID = normalizeTaskID(task.ID)

	if err := s.validateDependsOnLocked(task); err != nil {
		return err
	}

	s.tasks[task.ID] = task
	if err := s.saveTask(task); err != nil {
		delete(s.tasks, task.ID)
		slog.Error("failed to save new task after creation", "task_id", task.ID, "error", err)
		return fmt.Errorf("failed to save task: %w", err)
	}
	return nil
}

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
	oldTask, exists := s.tasks[task.ID]
	if !exists {
		return fmt.Errorf("task not found: %s", task.ID)
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

	if _, exists := s.tasks[id]; !exists {
		return false
	}

	path := s.taskFilePath(id)

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
