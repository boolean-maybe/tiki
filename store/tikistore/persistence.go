package tikistore

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/internal/git"
	taskpkg "github.com/boolean-maybe/tiki/task"

	"gopkg.in/yaml.v3"
)

// loadLocked reads all task files from the directory.
// Caller must hold s.mu lock.
func (s *TikiStore) loadLocked() error {
	slog.Debug("loading tasks from directory", "dir", s.dir)
	// create directory if it doesn't exist
	//nolint:gosec // G301: 0755 is appropriate for task storage directory
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		slog.Error("failed to create task directory", "dir", s.dir, "error", err)
		return fmt.Errorf("creating directory: %w", err)
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		slog.Error("failed to read task directory", "dir", s.dir, "error", err)
		return fmt.Errorf("reading directory: %w", err)
	}

	// Pre-fetch all author info in one batch git operation
	var authorMap map[string]*git.AuthorInfo
	var lastCommitMap map[string]time.Time
	if s.gitUtil != nil {
		dirPattern := filepath.Join(s.dir, "*.md")
		if authors, err := s.gitUtil.AllAuthors(dirPattern); err == nil {
			authorMap = authors
		} else {
			slog.Warn("failed to batch fetch authors", "error", err)
		}

		if lastCommits, err := s.gitUtil.AllLastCommitTimes(dirPattern); err == nil {
			lastCommitMap = lastCommits
		} else {
			slog.Warn("failed to batch fetch last commit times", "error", err)
		}
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(s.dir, entry.Name())
		task, err := s.loadTaskFile(filePath, authorMap, lastCommitMap)
		if err != nil {
			slog.Error("failed to load task file", "file", filePath, "error", err)
			// log error but continue loading other files
			continue
		}

		s.tasks[task.ID] = task
		slog.Debug("loaded task", "task_id", task.ID, "file", filePath)
	}
	slog.Info("finished loading tasks", "num_tasks", len(s.tasks))
	return nil
}

// loadTaskFile parses a single markdown file into a Task
func (s *TikiStore) loadTaskFile(path string, authorMap map[string]*git.AuthorInfo, lastCommitMap map[string]time.Time) (*taskpkg.Task, error) {
	// Get file info for mtime (optimistic locking)
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	frontmatter, body, err := store.ParseFrontmatter(string(content))
	if err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	var fm taskFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}

	// Derive ID from filename: "tiki-abc123.md" -> "TIKI-ABC123"
	// IGNORE fm.ID even if present - filename is authoritative
	filename := filepath.Base(path)
	taskID := strings.ToUpper(strings.TrimSuffix(filename, ".md"))

	// Log warning if frontmatter has ID that differs from filename
	// Parse frontmatter as generic map to check for ID field
	var fmMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(frontmatter), &fmMap); err == nil {
		if rawID, ok := fmMap["id"]; ok {
			if idStr, ok := rawID.(string); ok && idStr != "" && idStr != taskID {
				slog.Warn("ignoring frontmatter ID mismatch, using filename",
					"file", path,
					"frontmatter_id", idStr,
					"filename_id", taskID)
			}
		}
	}

	task := &taskpkg.Task{
		ID:          taskID,
		Title:       fm.Title,
		Description: strings.TrimSpace(body),
		Type:        taskpkg.NormalizeType(fm.Type),
		Status:      taskpkg.MapStatus(fm.Status),
		Tags:        fm.Tags.ToStringSlice(),
		Assignee:    fm.Assignee,
		Priority:    int(fm.Priority),
		Points:      fm.Points,
		LoadedMtime: info.ModTime(),
	}

	// Validate and default Priority field (1-5 range)
	if task.Priority < taskpkg.MinPriority || task.Priority > taskpkg.MaxPriority {
		slog.Debug("invalid priority value, using default", "task_id", task.ID, "file", path, "invalid_value", task.Priority, "default", taskpkg.DefaultPriority)
		task.Priority = taskpkg.DefaultPriority
	}

	// Validate and default Points field
	maxPoints := config.GetMaxPoints()
	if task.Points < 1 || task.Points > maxPoints {
		task.Points = maxPoints / 2
		slog.Debug("invalid points value, using default", "task_id", task.ID, "file", path, "invalid_value", fm.Points, "default", task.Points)
	}

	// Compute UpdatedAt as max(file_mtime, last_git_commit_time)
	task.UpdatedAt = info.ModTime() // Start with file mtime
	if lastCommitMap != nil {
		// Convert to relative path for lookup (same pattern as authorMap)
		relPath := path
		if filepath.IsAbs(path) {
			if rel, err := filepath.Rel(s.dir, path); err == nil {
				relPath = filepath.Join(s.dir, rel)
			}
		}

		if lastCommit, exists := lastCommitMap[relPath]; exists {
			// Take the maximum of file mtime and git commit time
			if lastCommit.After(task.UpdatedAt) {
				task.UpdatedAt = lastCommit
			}
		}
	}

	// Populate CreatedBy from author map (already fetched in batch)
	if authorMap != nil {
		// Convert to relative path for lookup
		relPath := path
		if filepath.IsAbs(path) {
			if rel, err := filepath.Rel(s.dir, path); err == nil {
				relPath = filepath.Join(s.dir, rel)
			}
		}

		if author, exists := authorMap[relPath]; exists {
			// Use name if present, otherwise fall back to email
			if author.Name != "" {
				task.CreatedBy = author.Name
			} else if author.Email != "" {
				task.CreatedBy = author.Email
			}
			task.CreatedAt = author.Date
		}
	}

	// Fallback to file metadata when git history is not available.
	// This handles the case where files are staged or untracked.
	// Once the file is committed, git history will be used instead.
	if task.CreatedAt.IsZero() {
		// No git history for this file - use file modification time as fallback
		task.CreatedAt = info.ModTime()

		// Try to get current git user for CreatedBy
		if s.gitUtil != nil {
			if name, email, err := s.gitUtil.CurrentUser(); err == nil {
				// Prefer name, fall back to email
				if name != "" {
					task.CreatedBy = name
				} else if email != "" {
					task.CreatedBy = email
				}
			}
		}

		// If git user is not available, leave CreatedBy empty (will show "Unknown" in UI)
	}

	return task, nil
}

// Reload reloads all tasks from disk
func (s *TikiStore) Reload() error {
	slog.Info("reloading tasks from disk")
	start := time.Now()
	s.mu.Lock()
	s.tasks = make(map[string]*taskpkg.Task)

	if err := s.loadLocked(); err != nil {
		s.mu.Unlock()
		slog.Error("error reloading tasks from disk", "error", err)
		return err
	}
	s.mu.Unlock()

	slog.Info("tasks reloaded successfully", "duration", time.Since(start).Round(time.Millisecond))
	s.notifyListeners()
	return nil
}

// ReloadTask reloads a single task from disk by ID
func (s *TikiStore) ReloadTask(taskID string) error {
	slog.Debug("reloading single task", "task_id", taskID)

	// Construct file path
	filename := strings.ToLower(taskID) + ".md"
	filePath := filepath.Join(s.dir, filename)

	// Fetch git info for this single file
	var authorMap map[string]*git.AuthorInfo
	var lastCommitMap map[string]time.Time
	if s.gitUtil != nil {
		if authors, err := s.gitUtil.AllAuthors(filePath); err == nil {
			authorMap = authors
		}
		if lastCommits, err := s.gitUtil.AllLastCommitTimes(filePath); err == nil {
			lastCommitMap = lastCommits
		}
	}

	// Load the task file
	task, err := s.loadTaskFile(filePath, authorMap, lastCommitMap)
	if err != nil {
		return fmt.Errorf("loading task file %s: %w", filePath, err)
	}

	// Update the task in the map
	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()

	s.notifyListeners()
	slog.Debug("task reloaded successfully", "task_id", taskID)
	return nil
}

// saveTask writes a task to its markdown file
func (s *TikiStore) saveTask(task *taskpkg.Task) error {
	path := s.taskFilePath(task.ID)
	slog.Debug("attempting to save task", "task_id", task.ID, "path", path)

	// Check for external modification (optimistic locking)
	// Only check if task was previously loaded (LoadedMtime is not zero)
	if !task.LoadedMtime.IsZero() {
		if info, err := os.Stat(path); err == nil {
			if !info.ModTime().Equal(task.LoadedMtime) {
				slog.Warn("task modified externally, conflict detected", "task_id", task.ID, "path", path, "loaded_mtime", task.LoadedMtime, "file_mtime", info.ModTime())
				return ErrConflict
			}
		} else if !os.IsNotExist(err) {
			slog.Error("failed to stat file for optimistic locking", "task_id", task.ID, "path", path, "error", err)
			return fmt.Errorf("stat file for optimistic locking: %w", err)
		}
	}

	fm := taskFrontmatter{
		Title:    task.Title,
		Type:     string(task.Type),
		Status:   taskpkg.StatusToString(task.Status),
		Tags:     task.Tags,
		Assignee: task.Assignee,
		Priority: taskpkg.PriorityValue(task.Priority),
		Points:   task.Points,
	}

	// sort tags for consistent output
	if len(fm.Tags) > 0 {
		sort.Strings(fm.Tags)
	}

	yamlBytes, err := yaml.Marshal(fm)
	if err != nil {
		slog.Error("failed to marshal frontmatter for task", "task_id", task.ID, "error", err)
		return fmt.Errorf("marshaling frontmatter: %w", err)
	}

	var content strings.Builder
	content.WriteString("---\n")
	content.Write(yamlBytes)
	content.WriteString("---\n")
	if task.Description != "" {
		content.WriteString(task.Description)
		content.WriteString("\n")
	}

	if err := os.WriteFile(path, []byte(content.String()), 0644); err != nil {
		slog.Error("failed to write task file", "task_id", task.ID, "path", path, "error", err)
		return fmt.Errorf("writing file: %w", err)
	}

	// Update LoadedMtime after successful save
	if info, err := os.Stat(path); err == nil {
		task.LoadedMtime = info.ModTime()
		// Recompute UpdatedAt (computed field, not persisted)
		task.UpdatedAt = info.ModTime() // Start with new mtime
		if s.gitUtil != nil {
			if lastCommit, err := s.gitUtil.LastCommitTime(path); err == nil {
				if lastCommit.After(task.UpdatedAt) {
					task.UpdatedAt = lastCommit
				}
			}
		}
		slog.Debug("task file saved and timestamps computed", "task_id", task.ID, "path", path, "new_mtime", task.LoadedMtime, "updated_at", task.UpdatedAt)
	} else {
		slog.Error("failed to stat file after save for mtime computation", "task_id", task.ID, "path", path, "error", err)
	}

	// Git add the modified file (best effort)
	if s.gitUtil != nil {
		if err := s.gitUtil.Add(path); err != nil {
			slog.Warn("failed to git add task file", "task_id", task.ID, "path", path, "error", err)
		}
	}

	slog.Info("task saved successfully", "task_id", task.ID, "path", path)
	return nil
}

// taskFilePath returns the file path for a task ID
func (s *TikiStore) taskFilePath(id string) string {
	// convert ID to lowercase filename: TIKI-ABC123 -> tiki-abc123.md
	filename := strings.ToLower(id) + ".md"
	return filepath.Join(s.dir, filename)
}
