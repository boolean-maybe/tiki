package config

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

//go:embed init_tiki.md
var initialTaskTemplate string

//go:embed new.md
var defaultNewTaskTemplate string

//go:embed index.md
var dokiEntryPoint string

//go:embed linked.md
var dokiLinked string

// GenerateRandomID generates a 6-character random alphanumeric ID (lowercase)
func GenerateRandomID() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	const length = 6
	id, err := gonanoid.Generate(alphabet, length)
	if err != nil {
		// Fallback to simple implementation if nanoid fails
		return "error0"
	}
	return id
}

// BootstrapSystem creates the task storage and seeds the initial tiki.
func BootstrapSystem() error {
	// Create all necessary directories
	if err := EnsureDirs(); err != nil {
		return fmt.Errorf("ensure directories: %w", err)
	}

	// Generate random ID for initial task
	randomID := GenerateRandomID()
	taskID := fmt.Sprintf("TIKI-%s", randomID)
	taskFilename := fmt.Sprintf("tiki-%s.md", randomID)
	taskPath := filepath.Join(GetTaskDir(), taskFilename)

	// Replace placeholder in template
	taskContent := strings.Replace(initialTaskTemplate, "TIKI-XXXXXX", taskID, 1)
	if err := os.WriteFile(taskPath, []byte(taskContent), 0644); err != nil {
		return fmt.Errorf("write initial task: %w", err)
	}

	// Write doki documentation files
	dokiDir := GetDokiDir()
	indexPath := filepath.Join(dokiDir, "index.md")
	if err := os.WriteFile(indexPath, []byte(dokiEntryPoint), 0644); err != nil {
		return fmt.Errorf("write doki index: %w", err)
	}

	linkedPath := filepath.Join(dokiDir, "linked.md")
	if err := os.WriteFile(linkedPath, []byte(dokiLinked), 0644); err != nil {
		return fmt.Errorf("write doki linked: %w", err)
	}

	// Git add initial task and doki files
	//nolint:gosec // G204: git command with controlled file paths
	cmd := exec.Command("git", "add", taskPath, indexPath, linkedPath)
	if err := cmd.Run(); err != nil {
		// Non-fatal: log but don't fail bootstrap if git add fails
		fmt.Fprintf(os.Stderr, "warning: failed to git add files: %v\n", err)
	}

	return nil
}

// GetDefaultNewTaskTemplate returns the embedded new.md template
func GetDefaultNewTaskTemplate() string {
	return defaultNewTaskTemplate
}
