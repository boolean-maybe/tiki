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
	return CreateTestTaskWithDeps(dir, id, title, status, taskType, nil)
}

// CreateTestTaskWithDeps creates a markdown task file with optional dependsOn IDs in the frontmatter.
func CreateTestTaskWithDeps(dir, id, title string, status task.Status, taskType task.Type, dependsOn []string) error {
	filename := strings.ToLower(id) + ".md"
	filePath := filepath.Join(dir, filename)

	depsYAML := ""
	if len(dependsOn) > 0 {
		depsYAML = "dependsOn:\n"
		for _, dep := range dependsOn {
			depsYAML += fmt.Sprintf("  - %s\n", dep)
		}
	}

	content := fmt.Sprintf(`---
title: %s
type: %s
status: %s
priority: 3
points: 1
%s---
%s
`, title, taskType, status, depsYAML, title)

	return os.WriteFile(filePath, []byte(content), 0644)
}
