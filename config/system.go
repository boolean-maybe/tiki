package config

import (
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
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

//go:embed default_workflow.yaml
var defaultWorkflowYAML string

//go:embed markdown.png
var markdownPNG []byte

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

// sampleFrontmatterRe extracts type and status values from sample tiki frontmatter.
var sampleFrontmatterRe = regexp.MustCompile(`(?m)^(type|status):\s*(.+)$`)

// validateSampleTiki checks whether a sample tiki's type and status
// are valid against the current workflow registries.
func validateSampleTiki(template string) bool {
	matches := sampleFrontmatterRe.FindAllStringSubmatch(template, -1)
	statusReg := GetStatusRegistry()
	typeReg := GetTypeRegistry()
	for _, m := range matches {
		key, val := m[1], strings.TrimSpace(m[2])
		switch key {
		case "type":
			if _, ok := typeReg.ParseType(val); !ok {
				return false
			}
		case "status":
			if !statusReg.IsValid(val) {
				return false
			}
		}
	}
	return true
}

// BootstrapSystem creates the task storage and seeds the initial tiki.
// If createSamples is true, embedded sample tikis are validated against
// the active workflow registries and only valid ones are written.
// gitAdd, when non-nil, is called to stage created files (e.g. ops.Add).
func BootstrapSystem(createSamples bool, gitAdd func(...string) error) error {
	// Create all necessary directories
	if err := EnsureDirs(); err != nil {
		return fmt.Errorf("ensure directories: %w", err)
	}

	taskDir := GetTaskDir()
	var createdFiles []string

	if createSamples {
		// ensure workflow registries are loaded before validating samples;
		// on first run this may require installing the default workflow first
		if err := InstallDefaultWorkflow(); err != nil {
			slog.Warn("failed to install default workflow for sample validation", "error", err)
		}
		if err := LoadWorkflowRegistries(); err != nil {
			slog.Warn("failed to load workflow registries for sample validation; skipping samples", "error", err)
			createSamples = false
		}
	}

	if createSamples {
		type sampleDef struct {
			name     string
			template string
		}
		samples := []sampleDef{
			{"board", initialTaskTemplate},
			{"backlog 1", backlogSample1},
			{"backlog 2", backlogSample2},
			{"roadmap now", roadmapNowSample},
			{"roadmap next", roadmapNextSample},
			{"roadmap later", roadmapLaterSample},
		}

		for _, s := range samples {
			if !validateSampleTiki(s.template) {
				slog.Info("skipping incompatible sample tiki", "name", s.name)
				continue
			}

			randomID := GenerateRandomID()
			taskID := fmt.Sprintf("TIKI-%s", randomID)
			taskFilename := fmt.Sprintf("tiki-%s.md", randomID)
			taskPath := filepath.Join(taskDir, taskFilename)

			taskContent := strings.Replace(s.template, "TIKI-XXXXXX", taskID, 1)
			if err := os.WriteFile(taskPath, []byte(taskContent), 0644); err != nil {
				return fmt.Errorf("create sample %s: %w", s.name, err)
			}
			createdFiles = append(createdFiles, taskPath)
		}
	}

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

	// Copy markdown.png to doki directory
	dokiMarkdownPNGPath := filepath.Join(dokiDir, "markdown.png")
	if err := os.WriteFile(dokiMarkdownPNGPath, markdownPNG, 0644); err != nil {
		return fmt.Errorf("write doki markdown.png: %w", err)
	}
	createdFiles = append(createdFiles, dokiMarkdownPNGPath)

	// Copy markdown.png to task directory
	tikiMarkdownPNGPath := filepath.Join(taskDir, "markdown.png")
	if err := os.WriteFile(tikiMarkdownPNGPath, markdownPNG, 0644); err != nil {
		return fmt.Errorf("write tiki markdown.png: %w", err)
	}
	createdFiles = append(createdFiles, tikiMarkdownPNGPath)

	if gitAdd != nil {
		if err := gitAdd(createdFiles...); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to git add files: %v\n", err)
		}
	}

	return nil
}

// GetDefaultNewTaskTemplate returns the embedded new.md template
func GetDefaultNewTaskTemplate() string {
	return defaultNewTaskTemplate
}

// GetDefaultWorkflowYAML returns the embedded default workflow.yaml content
func GetDefaultWorkflowYAML() string {
	return defaultWorkflowYAML
}

// InstallDefaultWorkflow installs the default workflow.yaml to the user config directory
// if it does not already exist. This runs on every launch to handle first-run and upgrade cases.
func InstallDefaultWorkflow() error {
	path := GetUserConfigWorkflowFile()

	// No-op if the file already exists
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	// Ensure the config directory exists
	dir := filepath.Dir(path)
	//nolint:gosec // G301: 0755 is appropriate for config directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory %s: %w", dir, err)
	}

	//nolint:gosec // G306: 0644 is appropriate for config file
	if err := os.WriteFile(path, []byte(defaultWorkflowYAML), 0644); err != nil {
		return fmt.Errorf("write default workflow.yaml: %w", err)
	}

	return nil
}
