package tikistore

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"

	"gopkg.in/yaml.v3"
)

const templateSource = "<template>"

// templateFrontmatter represents the YAML frontmatter in template files
type templateFrontmatter struct {
	Title      string   `yaml:"title"`
	Type       string   `yaml:"type"`
	Status     string   `yaml:"status"`
	Tags       []string `yaml:"tags"`
	DependsOn  []string `yaml:"dependsOn"`
	Due        string   `yaml:"due"`
	Recurrence string   `yaml:"recurrence"`
	Assignee   string   `yaml:"assignee"`
	Priority   int      `yaml:"priority"`
	Points     int      `yaml:"points"`
}

// loadTemplateTask reads new.md from the highest-priority location
// (cwd > .doc/ > user config), or falls back to the embedded template.
func loadTemplateTask() (*taskpkg.Task, error) {
	templatePath := config.FindTemplateFile()

	if templatePath == "" {
		slog.Debug("no new.md found in any search path, using embedded template")
		return loadEmbeddedTemplate()
	}

	data, err := os.ReadFile(templatePath)
	if err != nil {
		slog.Warn("failed to read new.md template", "path", templatePath, "error", err)
		return loadEmbeddedTemplate()
	}

	slog.Debug("loaded new.md template", "path", templatePath)
	return parseTaskTemplate(data)
}

// loadEmbeddedTemplate loads the embedded config/new.md template
func loadEmbeddedTemplate() (*taskpkg.Task, error) {
	templateStr := config.GetDefaultNewTaskTemplate()
	if templateStr == "" {
		return nil, nil
	}
	return parseTaskTemplate([]byte(templateStr))
}

// parseTaskTemplate parses task template data from markdown with YAML frontmatter
func parseTaskTemplate(data []byte) (*taskpkg.Task, error) {
	content := strings.TrimSpace(string(data))
	if !strings.HasPrefix(content, "---") {
		return nil, nil
	}

	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return nil, nil
	}

	frontmatter := strings.TrimSpace(rest[:idx])
	body := strings.TrimSpace(strings.TrimPrefix(rest[idx+4:], "\n"))

	var fm templateFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return nil, nil
	}

	// Parse due date if provided
	var dueTime time.Time
	if fm.Due != "" {
		parsed, ok := taskpkg.ParseDueDate(fm.Due)
		if ok {
			dueTime = parsed
		}
	}

	// Parse recurrence if provided
	var recurrence taskpkg.Recurrence
	if fm.Recurrence != "" {
		if parsed, ok := taskpkg.ParseRecurrence(fm.Recurrence); ok {
			recurrence = parsed
		}
	}

	// second pass: extract custom fields from frontmatter map
	var fmMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(frontmatter), &fmMap); err != nil {
		return nil, nil
	}
	customFields, _, err := extractCustomFields(fmMap, templateSource)
	if err != nil {
		return nil, fmt.Errorf("template custom fields: %w", err)
	}

	return &taskpkg.Task{
		Title:        fm.Title,
		Description:  body,
		Type:         taskpkg.NormalizeType(fm.Type),
		Status:       taskpkg.NormalizeStatus(fm.Status),
		Tags:         fm.Tags,
		DependsOn:    fm.DependsOn,
		Due:          dueTime,
		Recurrence:   recurrence,
		Assignee:     fm.Assignee,
		Priority:     fm.Priority,
		Points:       fm.Points,
		CustomFields: customFields,
	}, nil
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

	taskID = normalizeTaskID(taskID)

	// Load template (with defaults)
	template, err := loadTemplateTask()
	if err != nil {
		return nil, fmt.Errorf("loading template: %w", err)
	}

	// Create base task with defaults
	task := &taskpkg.Task{
		ID:          taskID,
		Title:       "",
		Description: "",
		Status:      taskpkg.DefaultStatus(), // default fallback
		Type:        taskpkg.TypeStory,       // default fallback
		Priority:    3,                       // default: medium priority (1-5 scale)
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
		task.DependsOn = template.DependsOn
		task.Due = template.Due
		task.Recurrence = template.Recurrence
		task.Assignee = template.Assignee
		task.Status = template.Status
	}

	if template != nil && template.CustomFields != nil {
		task.CustomFields = make(map[string]interface{}, len(template.CustomFields))
		for k, v := range template.CustomFields {
			if ss, ok := v.([]string); ok {
				cp := make([]string, len(ss))
				copy(cp, ss)
				task.CustomFields[k] = cp
			} else {
				task.CustomFields[k] = v
			}
		}
	}

	// Ensure type has a value (fallback if template didn't provide)
	if task.Type == "" {
		task.Type = taskpkg.TypeStory
	}

	// Ensure status has a value
	if task.Status == "" {
		task.Status = taskpkg.DefaultStatus()
	}

	// Set git author
	s.setAuthorFromGit(task)

	return task, nil
}
