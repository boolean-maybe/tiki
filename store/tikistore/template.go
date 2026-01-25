package tikistore

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"

	"gopkg.in/yaml.v3"
)

// templateFrontmatter represents the YAML frontmatter in template files
type templateFrontmatter struct {
	Title    string   `yaml:"title"`
	Type     string   `yaml:"type"`
	Status   string   `yaml:"status"`
	Tags     []string `yaml:"tags"`
	Assignee string   `yaml:"assignee"`
	Priority int      `yaml:"priority"`
	Points   int      `yaml:"points"`
}

// loadTemplateTask reads new.md next to the executable, or falls back to embedded template.
func loadTemplateTask() *taskpkg.Task {
	// Try to load from binary directory first
	exePath, err := os.Executable()
	if err != nil {
		slog.Warn("failed to get executable path for template", "error", err)
		return loadEmbeddedTemplate()
	}

	binaryDir := filepath.Dir(exePath)
	templatePath := filepath.Join(binaryDir, "new.md")

	data, err := os.ReadFile(templatePath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("new.md not found in binary dir, using embedded template")
			return loadEmbeddedTemplate()
		}
		slog.Warn("failed to read new.md template", "path", templatePath, "error", err)
		return loadEmbeddedTemplate()
	}

	return parseTaskTemplate(data)
}

// loadEmbeddedTemplate loads the embedded config/new.md template
func loadEmbeddedTemplate() *taskpkg.Task {
	templateStr := config.GetDefaultNewTaskTemplate()
	if templateStr == "" {
		return nil
	}
	return parseTaskTemplate([]byte(templateStr))
}

// parseTaskTemplate parses task template data from markdown with YAML frontmatter
func parseTaskTemplate(data []byte) *taskpkg.Task {
	content := strings.TrimSpace(string(data))
	if !strings.HasPrefix(content, "---") {
		return nil
	}

	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return nil
	}

	frontmatter := strings.TrimSpace(rest[:idx])
	body := strings.TrimSpace(strings.TrimPrefix(rest[idx+4:], "\n"))

	var fm templateFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return nil
	}

	return &taskpkg.Task{
		Title:       fm.Title,
		Description: body,
		Type:        taskpkg.NormalizeType(fm.Type),
		Status:      taskpkg.NormalizeStatus(fm.Status),
		Tags:        fm.Tags,
		Assignee:    fm.Assignee,
		Priority:    fm.Priority,
		Points:      fm.Points,
	}
}

// setAuthorFromGit best-effort populates CreatedBy using current git user.
func (s *TikiStore) setAuthorFromGit(task *taskpkg.Task) {
	if task == nil || task.CreatedBy != "" {
		return
	}

	name, email, err := s.GetCurrentUser()
	if err != nil {
		return
	}

	switch {
	case name != "" && email != "":
		task.CreatedBy = fmt.Sprintf("%s <%s>", name, email)
	case name != "":
		task.CreatedBy = name
	case email != "":
		task.CreatedBy = email
	}
}

// NewTaskTemplate returns a new task populated with template defaults.
// The task will have all fields from the template (priority, type, tags, etc.)
// plus generated ID and git author.
func (s *TikiStore) NewTaskTemplate() (*taskpkg.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate random ID with collision check
	var taskID string
	for {
		randomID := config.GenerateRandomID()
		taskID = fmt.Sprintf("TIKI-%s", randomID)

		// Check if file already exists (collision check)
		path := s.taskFilePath(taskID)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break // No collision, use this ID
		}
		slog.Debug("ID collision detected during template creation, regenerating", "id", taskID)
	}

	// Load template (with defaults)
	template := loadTemplateTask()

	// Create base task with defaults
	task := &taskpkg.Task{
		ID:          taskID,
		Title:       "",
		Description: "",
		Status:      taskpkg.StatusBacklog, // default fallback
		Type:        taskpkg.TypeStory,     // default fallback
		Priority:    3,                     // default: medium priority (1-5 scale)
		Points:      0,
		CreatedAt:   time.Now(),
	}

	// Apply template values if available
	if template != nil {
		task.Title = template.Title
		task.Description = template.Description
		task.Type = template.Type
		task.Priority = template.Priority
		task.Points = template.Points
		task.Tags = template.Tags
		task.Assignee = template.Assignee
		task.Status = template.Status
	}

	// Ensure type has a value (fallback if template didn't provide)
	if task.Type == "" {
		task.Type = taskpkg.TypeStory
	}

	// Ensure status has a value
	if task.Status == "" {
		task.Status = taskpkg.StatusBacklog
	}

	// Set git author
	s.setAuthorFromGit(task)

	return task, nil
}
