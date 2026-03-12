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
	s.mu.Lock()

	// generate ID if not provided
	if task.ID == "" {
		// Generate random ID with collision check
		for {
			randomID := config.GenerateRandomID()
			task.ID = fmt.Sprintf("TIKI-%s", randomID)

			// Check if file already exists (collision check)
			path := s.taskFilePath(task.ID)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				break // No collision, use this ID
			}
			slog.Debug("ID collision detected, regenerating", "id", task.ID)
		}
	}

	task.ID = normalizeTaskID(task.ID)

	// Validate dependsOn references exist
	if err := s.validateDependsOnLocked(task); err != nil {
		s.mu.Unlock()
		return err
	}

	s.tasks[task.ID] = task
	if err := s.saveTask(task); err != nil {
		// Rollback on failure
		delete(s.tasks, task.ID)
		s.mu.Unlock()
		slog.Error("failed to save new task after creation", "task_id", task.ID, "error", err)
		return fmt.Errorf("failed to save task: %w", err)
	}
	s.mu.Unlock()

	slog.Info("task created", "task_id", task.ID, "status", task.Status)
	s.notifyListeners()
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
	s.mu.Lock()

	task.ID = normalizeTaskID(task.ID)
	oldTask, exists := s.tasks[task.ID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("task not found: %s", task.ID)
	}

	// Validate dependsOn references exist
	if err := s.validateDependsOnLocked(task); err != nil {
		s.mu.Unlock()
		return err
	}

	s.tasks[task.ID] = task
	if err := s.saveTask(task); err != nil {
		// Rollback on failure
		s.tasks[task.ID] = oldTask
		s.mu.Unlock()
		slog.Error("failed to save updated task", "task_id", task.ID, "error", err)
		return fmt.Errorf("failed to save task: %w", err)
	}
	s.mu.Unlock()

	slog.Info("task updated", "task_id", task.ID, "status", task.Status)
	s.notifyListeners()
	return nil
}

// DeleteTask removes a task and its file
func (s *TikiStore) DeleteTask(id string) {
	s.mu.Lock()

	normalizedID := normalizeTaskID(id)
	if _, exists := s.tasks[normalizedID]; !exists {
		s.mu.Unlock()
		return
	}

	path := s.taskFilePath(normalizedID)

	// Try git rm first if git is available
	removed := false
	if s.gitUtil != nil {
		if err := s.gitUtil.Remove(path); err == nil {
			removed = true
		} else {
			slog.Debug("failed to git remove task file, falling back to os.Remove", "task_id", id, "path", path, "error", err)
		}
	}

	// Fall back to os.Remove if git rm failed or unavailable
	if !removed {
		if err := os.Remove(path); err != nil {
			slog.Error("file deletion failed, task preserved in memory", "task_id", id, "path", path, "error", err)
			s.mu.Unlock()
			return // Don't modify in-memory state if file deletion failed
		}
	}

	// Only delete from memory after successful file deletion
	delete(s.tasks, normalizedID)
	s.mu.Unlock()
	slog.Info("task deleted", "task_id", normalizedID)
	s.notifyListeners()
}

// AddComment adds a comment to a task
// note: comments are stored in memory only for TikiStore
// (could be extended to store in file or separate files)
func (s *TikiStore) AddComment(taskID string, comment taskpkg.Comment) bool {
	s.mu.Lock()

	taskID = normalizeTaskID(taskID)
	task, exists := s.tasks[taskID]
	if !exists {
		s.mu.Unlock()
		return false
	}

	task.Comments = append(task.Comments, comment)
	s.mu.Unlock()
	s.notifyListeners()
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
