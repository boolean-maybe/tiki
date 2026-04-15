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
	"github.com/boolean-maybe/tiki/workflow"

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
			if idStr, ok := rawID.(string); ok && idStr != "" && !strings.EqualFold(idStr, taskID) {
				slog.Warn("ignoring frontmatter ID mismatch, using filename",
					"file", path,
					"frontmatter_id", idStr,
					"filename_id", taskID)
			}
		}
	}

	// extract custom fields from frontmatter map
	customFields, unknownFields, err := extractCustomFields(fmMap, path)
	if err != nil {
		return nil, err
	}

	task := &taskpkg.Task{
		ID:            taskID,
		Title:         fm.Title,
		Description:   strings.TrimSpace(body),
		Type:          taskpkg.NormalizeType(fm.Type),
		Status:        taskpkg.MapStatus(fm.Status),
		Tags:          fm.Tags.ToStringSlice(),
		DependsOn:     fm.DependsOn.ToStringSlice(),
		Due:           fm.Due.ToTime(),
		Recurrence:    fm.Recurrence.ToRecurrence(),
		Assignee:      fm.Assignee,
		Priority:      int(fm.Priority),
		Points:        fm.Points,
		CustomFields:  customFields,
		UnknownFields: unknownFields,
		LoadedMtime:   info.ModTime(),
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

	s.upgrader.UpgradeTask(task)

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
	normalizedID := normalizeTaskID(taskID)
	slog.Debug("reloading single task", "task_id", normalizedID)

	// Construct file path
	filename := strings.ToLower(normalizedID) + ".md"
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
	slog.Debug("task reloaded successfully", "task_id", task.ID)
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
		Title:      task.Title,
		Type:       string(task.Type),
		Status:     taskpkg.StatusToString(task.Status),
		Tags:       task.Tags,
		DependsOn:  task.DependsOn,
		Due:        taskpkg.DueValue{Time: task.Due},
		Recurrence: taskpkg.RecurrenceValue{Value: task.Recurrence},
		Assignee:   task.Assignee,
		Priority:   taskpkg.PriorityValue(task.Priority),
		Points:     task.Points,
	}

	// sort tags for consistent output
	if len(fm.Tags) > 0 {
		sort.Strings(fm.Tags)
	}

	// sort dependsOn for consistent output
	if len(fm.DependsOn) > 0 {
		sort.Strings(fm.DependsOn)
	}

	yamlBytes, err := yaml.Marshal(fm)
	if err != nil {
		slog.Error("failed to marshal frontmatter for task", "task_id", task.ID, "error", err)
		return fmt.Errorf("marshaling frontmatter: %w", err)
	}

	var content strings.Builder
	content.WriteString("---\n")
	content.Write(yamlBytes)
	// append custom fields
	if len(task.CustomFields) > 0 {
		if err := appendCustomFields(&content, task.CustomFields); err != nil {
			return fmt.Errorf("marshaling custom fields: %w", err)
		}
	}
	// append unknown fields so they survive round-trips
	if len(task.UnknownFields) > 0 {
		if err := appendUnknownFields(&content, task.UnknownFields); err != nil {
			return fmt.Errorf("marshaling unknown fields: %w", err)
		}
	}
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

// builtInFrontmatterKeys lists keys handled by the taskFrontmatter struct.
var builtInFrontmatterKeys = map[string]bool{
	"id": true, "title": true, "type": true, "status": true,
	"tags": true, "dependsOn": true, "due": true, "recurrence": true,
	"assignee": true, "priority": true, "points": true,
}

// extractCustomFields reads custom field values from a raw frontmatter map.
// Built-in keys are skipped. Registered custom fields are coerced and returned
// in the first map. Unrecognised non-builtin keys are preserved verbatim in
// the second map so they survive a load→save round-trip.
func extractCustomFields(fmMap map[string]interface{}, path string) (customFields, unknownFields map[string]interface{}, err error) {
	if fmMap == nil {
		return nil, nil, nil
	}

	registryChecked := false
	for key, raw := range fmMap {
		if builtInFrontmatterKeys[key] {
			continue
		}
		// defer registry check until we actually encounter a non-builtin key
		if !registryChecked {
			if err := config.RequireWorkflowRegistriesLoaded(); err != nil {
				return nil, nil, fmt.Errorf("extractCustomFields for %s: %w", path, err)
			}
			registryChecked = true
		}
		fd, ok := workflow.Field(key)
		if !ok || !fd.Custom {
			slog.Debug("preserving unknown field in frontmatter", "field", key, "file", path)
			if unknownFields == nil {
				unknownFields = make(map[string]interface{})
			}
			unknownFields[key] = raw
			continue
		}
		val, err := coerceCustomValue(fd, raw)
		if err != nil {
			// stale value (e.g. removed enum option): demote to unknown so
			// the task still loads and the value survives for repair
			slog.Warn("demoting stale custom field value to unknown",
				"field", key, "file", path, "error", err)
			if unknownFields == nil {
				unknownFields = make(map[string]interface{})
			}
			unknownFields[key] = raw
			continue
		}
		if customFields == nil {
			customFields = make(map[string]interface{})
		}
		customFields[key] = val
	}
	return customFields, unknownFields, nil
}

// coerceCustomValue converts a raw YAML value to the Go type expected by FieldDef.
func coerceCustomValue(fd workflow.FieldDef, raw interface{}) (interface{}, error) {
	switch fd.Type {
	case workflow.TypeString:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", raw)
		}
		return s, nil

	case workflow.TypeEnum:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("expected string for enum, got %T", raw)
		}
		for _, av := range fd.AllowedValues {
			if strings.EqualFold(s, av) {
				return av, nil // canonical casing
			}
		}
		return nil, fmt.Errorf("value %q not in allowed values %v", s, fd.AllowedValues)

	case workflow.TypeInt:
		switch v := raw.(type) {
		case int:
			return v, nil
		case float64:
			if v != float64(int(v)) {
				return nil, fmt.Errorf("value %g is not a whole number", v)
			}
			return int(v), nil
		default:
			return nil, fmt.Errorf("expected int, got %T", raw)
		}

	case workflow.TypeBool:
		b, ok := raw.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool, got %T", raw)
		}
		return b, nil

	case workflow.TypeTimestamp:
		switch v := raw.(type) {
		case time.Time:
			return v, nil
		case string:
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t, nil
			}
			if t, err := time.Parse("2006-01-02", v); err == nil {
				return t, nil
			}
			return nil, fmt.Errorf("cannot parse timestamp %q", v)
		default:
			return nil, fmt.Errorf("expected time or string for timestamp, got %T", raw)
		}

	case workflow.TypeListString:
		return coerceStringList(raw)

	case workflow.TypeListRef:
		ss, err := coerceStringList(raw)
		if err != nil {
			return nil, err
		}
		for i, s := range ss {
			ss[i] = strings.ToUpper(strings.TrimSpace(s))
		}
		// drop empties
		filtered := ss[:0]
		for _, s := range ss {
			if s != "" {
				filtered = append(filtered, s)
			}
		}
		return filtered, nil

	default:
		return raw, nil
	}
}

// coerceStringList converts a raw YAML value ([]interface{}) to []string.
func coerceStringList(raw interface{}) ([]string, error) {
	switch v := raw.(type) {
	case []interface{}:
		ss := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("list item: expected string, got %T", item)
			}
			ss = append(ss, s)
		}
		return ss, nil
	case []string:
		return v, nil
	default:
		return nil, fmt.Errorf("expected list, got %T", raw)
	}
}

// appendCustomFields validates and marshals custom fields into the content builder.
// Keys are written in sorted order after the struct YAML output.
// Uses yaml.Marshal per field so that ambiguous string values (e.g. "true",
// "2026-05-15") are properly quoted and round-trip without type corruption.
func appendCustomFields(w *strings.Builder, fields map[string]interface{}) error {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		fd, ok := workflow.Field(k)
		if !ok || !fd.Custom {
			return fmt.Errorf("unknown custom field %q", k)
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		out, err := yaml.Marshal(map[string]interface{}{k: fields[k]})
		if err != nil {
			return fmt.Errorf("marshaling field %q: %w", k, err)
		}
		w.Write(out)
	}
	return nil
}

// appendUnknownFields writes preserved unknown frontmatter keys back in sorted
// order. These are keys that were present in the file but don't match any
// currently registered custom field — preserved so they survive round-trips.
func appendUnknownFields(w *strings.Builder, fields map[string]interface{}) error {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		out, err := yaml.Marshal(map[string]interface{}{k: fields[k]})
		if err != nil {
			return fmt.Errorf("marshaling unknown field %q: %w", k, err)
		}
		w.Write(out)
	}
	return nil
}
