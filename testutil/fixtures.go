package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/task"
)

// CreateTestTask creates a markdown task file with YAML frontmatter
func CreateTestTask(dir, id, title string, status task.Status, taskType task.Type) error {
	// Task files are lowercase (e.g., tiki-1.md)
	filename := strings.ToLower(id) + ".md"
	filename = strings.ReplaceAll(filename, "-", "-") // already hyphenated
	filepath := filepath.Join(dir, filename)

	// Build YAML frontmatter
	content := fmt.Sprintf(`---
title: %s
type: %s
status: %s
priority: 3
points: 1
---
%s
`, title, taskType, status, title)

	return os.WriteFile(filepath, []byte(content), 0644)
}

// CreateBoardTasks creates sample tasks across all board panes
func CreateBoardTasks(dir string) error {
	tasks := []struct {
		id       string
		title    string
		status   task.Status
		taskType task.Type
	}{
		{"TIKI-1", "Todo Task", task.StatusReady, task.TypeStory},
		{"TIKI-2", "In Progress Task", task.StatusInProgress, task.TypeStory},
		{"TIKI-3", "Review Task", task.StatusReview, task.TypeStory},
		{"TIKI-4", "Done Task", task.StatusDone, task.TypeStory},
		{"TIKI-5", "Another Todo", task.StatusReady, task.TypeBug},
	}

	for _, task := range tasks {
		if err := CreateTestTask(dir, task.id, task.title, task.status, task.taskType); err != nil {
			return fmt.Errorf("failed to create task %s: %w", task.id, err)
		}
	}

	return nil
}
