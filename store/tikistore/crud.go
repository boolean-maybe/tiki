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
	return s.tasks[id]
}

// UpdateTask updates an existing task and saves it
func (s *TikiStore) UpdateTask(task *taskpkg.Task) error {
	s.mu.Lock()

	oldTask, exists := s.tasks[task.ID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("task not found: %s", task.ID)
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

// UpdateStatus changes a task's status
func (s *TikiStore) UpdateStatus(taskID string, newStatus taskpkg.Status) bool {
	s.mu.Lock()

	task, exists := s.tasks[taskID]
	if !exists {
		s.mu.Unlock()
		return false
	}

	oldStatus := task.Status // Capture old status for logging

	if task.Status == newStatus {
		s.mu.Unlock()
		slog.Debug("task status already matches new status, no update needed", "task_id", taskID, "status", newStatus)
		return false
	}

	task.Status = newStatus
	if err := s.saveTask(task); err != nil {
		slog.Error("failed to save task after status update", "task_id", taskID, "old_status", oldStatus, "new_status", newStatus, "error", err)
		// Consider reverting task.Status if save fails
		s.mu.Unlock()
		return false
	}
	s.mu.Unlock()
	slog.Info("task status updated", "task_id", taskID, "old_status", oldStatus, "new_status", newStatus)
	// notify outside lock to prevent deadlock when listeners call back into store
	s.notifyListeners()
	return true
}

// DeleteTask removes a task and its file
func (s *TikiStore) DeleteTask(id string) {
	s.mu.Lock()

	if _, exists := s.tasks[id]; !exists {
		s.mu.Unlock()
		return
	}

	path := s.taskFilePath(id)

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
	delete(s.tasks, id)
	s.mu.Unlock()
	slog.Info("task deleted", "task_id", id)
	s.notifyListeners()
}

// AddComment adds a comment to a task
// note: comments are stored in memory only for TikiStore
// (could be extended to store in file or separate files)
func (s *TikiStore) AddComment(taskID string, comment taskpkg.Comment) bool {
	s.mu.Lock()

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
