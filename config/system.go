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

//go:embed board_sample.md
var initialTaskTemplate string

//go:embed backlog_sample_1.md
var backlogSample1 string

//go:embed backlog_sample_2.md
var backlogSample2 string

//go:embed roadmap_now_sample.md
var roadmapNowSample string

//go:embed roadmap_next_sample.md
var roadmapNextSample string

//go:embed roadmap_later_sample.md
var roadmapLaterSample string

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

	taskDir := GetTaskDir()
	var createdFiles []string

	// Helper function to create a sample tiki
	createSampleTiki := func(template string) (string, error) {
		randomID := GenerateRandomID()
		taskID := fmt.Sprintf("TIKI-%s", randomID)
		taskFilename := fmt.Sprintf("tiki-%s.md", randomID)
		taskPath := filepath.Join(taskDir, taskFilename)

		// Replace placeholder in template
		taskContent := strings.Replace(template, "TIKI-XXXXXX", taskID, 1)
		if err := os.WriteFile(taskPath, []byte(taskContent), 0644); err != nil {
			return "", fmt.Errorf("write task: %w", err)
		}
		return taskPath, nil
	}

	// Create board sample (original welcome tiki)
	boardPath, err := createSampleTiki(initialTaskTemplate)
	if err != nil {
		return fmt.Errorf("create board sample: %w", err)
	}
	createdFiles = append(createdFiles, boardPath)

	// Create backlog samples
	backlog1Path, err := createSampleTiki(backlogSample1)
	if err != nil {
		return fmt.Errorf("create backlog sample 1: %w", err)
	}
	createdFiles = append(createdFiles, backlog1Path)

	backlog2Path, err := createSampleTiki(backlogSample2)
	if err != nil {
		return fmt.Errorf("create backlog sample 2: %w", err)
	}
	createdFiles = append(createdFiles, backlog2Path)

	// Create roadmap samples
	roadmapNowPath, err := createSampleTiki(roadmapNowSample)
	if err != nil {
		return fmt.Errorf("create roadmap now sample: %w", err)
	}
	createdFiles = append(createdFiles, roadmapNowPath)

	roadmapNextPath, err := createSampleTiki(roadmapNextSample)
	if err != nil {
		return fmt.Errorf("create roadmap next sample: %w", err)
	}
	createdFiles = append(createdFiles, roadmapNextPath)

	roadmapLaterPath, err := createSampleTiki(roadmapLaterSample)
	if err != nil {
		return fmt.Errorf("create roadmap later sample: %w", err)
	}
	createdFiles = append(createdFiles, roadmapLaterPath)

	// Write doki documentation files
	dokiDir := GetDokiDir()
	indexPath := filepath.Join(dokiDir, "index.md")
	if err := os.WriteFile(indexPath, []byte(dokiEntryPoint), 0644); err != nil {
		return fmt.Errorf("write doki index: %w", err)
	}
	createdFiles = append(createdFiles, indexPath)

	linkedPath := filepath.Join(dokiDir, "linked.md")
	if err := os.WriteFile(linkedPath, []byte(dokiLinked), 0644); err != nil {
		return fmt.Errorf("write doki linked: %w", err)
	}
	createdFiles = append(createdFiles, linkedPath)

	// Write default config.yaml
	defaultConfig := `logging:
  level: error
header:
  visible: true
tiki:
  maxPoints: 10
appearance:
  theme: auto
  gradientThreshold: 256
`
	configPath := GetProjectConfigFile()
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("write default config.yaml: %w", err)
	}
	createdFiles = append(createdFiles, configPath)

	// Write default workflow.yaml
	defaultWorkflow := "plugins: []\n"
	workflowPath := DefaultWorkflowFilePath()
	if err := os.WriteFile(workflowPath, []byte(defaultWorkflow), 0644); err != nil {
		return fmt.Errorf("write default workflow.yaml: %w", err)
	}
	createdFiles = append(createdFiles, workflowPath)

	// Git add all created files
	gitArgs := append([]string{"add"}, createdFiles...)
	//nolint:gosec // G204: git command with controlled file paths
	cmd := exec.Command("git", gitArgs...)
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
