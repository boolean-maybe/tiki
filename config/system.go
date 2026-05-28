package config

import (
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/workflow"
)

//go:embed board_sample.md
var initialTikiTemplate string

//go:embed index.md
var indexEntryPoint string

//go:embed linked.md
var linkedEntryPoint string

//go:embed workflows/kanban.yaml
var embeddedKanbanYAML string

//go:embed workflows/todo.yaml
var embeddedTodoYAML string

//go:embed workflows/bug-tracker.yaml
var embeddedBugTrackerYAML string

const DefaultEmbeddedWorkflowName = "kanban"

var embeddedWorkflows = map[string]string{
	"kanban":      embeddedKanbanYAML,
	"todo":        embeddedTodoYAML,
	"bug-tracker": embeddedBugTrackerYAML,
}

// LookupEmbeddedWorkflow returns the YAML content for a bundled workflow name.
func LookupEmbeddedWorkflow(name string) (string, bool) {
	content, ok := embeddedWorkflows[name]
	return content, ok
}

// EmbeddedWorkflowNames returns the sorted list of bundled workflow names.
func EmbeddedWorkflowNames() []string {
	names := make([]string, 0, len(embeddedWorkflows))
	for k := range embeddedWorkflows {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

//go:embed markdown.png
var markdownPNG []byte

// GenerateRandomIDForTest is an optional injection point used only by tests
// that need deterministic id sequences (e.g. to exercise collision retries).
// When non-nil, GenerateRandomID delegates here; otherwise it uses
// document.NewID. Production code paths must not set this.
var GenerateRandomIDForTest func() string

// GenerateRandomID generates a bare uppercase document ID (no TIKI- prefix).
// Callers must not prepend "TIKI-" any longer; the identifier is the raw ID.
// Delegates to document.NewID so there is a single authoritative generator.
func GenerateRandomID() string {
	if GenerateRandomIDForTest != nil {
		return GenerateRandomIDForTest()
	}
	return document.NewID()
}

// sampleFrontmatterRe extracts type and status values from sample tiki frontmatter.
var sampleFrontmatterRe = regexp.MustCompile(`(?m)^(type|status):\s*(.+)$`)

// allDigitsRe matches an id consisting solely of decimal digits.
var allDigitsRe = regexp.MustCompile(`^[0-9]+$`)

// formatFrontmatterID returns id ready for emission as a YAML scalar value.
// All-digit ids are quoted so yaml.v3 does not decode them as integers (which
// would drop leading zeros and fail strict id validation on the next load);
// ids containing at least one letter are emitted bare for cleaner frontmatter.
func formatFrontmatterID(id string) string {
	if allDigitsRe.MatchString(id) {
		return fmt.Sprintf("%q", id)
	}
	return id
}

// withBundledID returns body prepended with a minimal frontmatter
// block carrying a freshly-generated bare id. Used for the two bundled
// wiki entry points (`index.md`, `linked.md`) written during project init.
// The strict loader rejects any `.md` under `.doc/` without an id, so
// these templates cannot ship as plain markdown — but embedding a
// literal id in the template file itself would collide across projects,
// so we wrap at write time instead.
//
// If body already starts with a frontmatter fence, the body is returned
// unchanged — protection against double-wrapping if a future template
// gets its own frontmatter.
func withBundledID(body string) string {
	if strings.HasPrefix(body, "---\n") {
		return body
	}
	return fmt.Sprintf("---\nid: %s\n---\n%s", formatFrontmatterID(GenerateRandomID()), body)
}

// validateSampleTiki checks whether a sample tiki's frontmatter values are
// valid against the current workflow field catalog. Any enum field present
// in the sample is checked; non-enum fields are accepted as-is here.
func validateSampleTiki(template string) bool {
	matches := sampleFrontmatterRe.FindAllStringSubmatch(template, -1)
	for _, m := range matches {
		key, val := m[1], strings.TrimSpace(m[2])
		fd, ok := workflow.Field(key)
		if !ok || fd.Type != workflow.TypeEnum {
			continue
		}
		if !fd.IsValidEnum(val) {
			return false
		}
	}
	return true
}

// BootstrapSystem creates the document storage and writes the welcome tiki
// plus the bundled wiki entry points. The welcome tiki is validated against
// the active workflow registries and skipped when its frontmatter does not
// match (e.g. under a custom workflow whose statuses differ from the bundled
// kanban). gitAdd, when non-nil, is called to stage created files
// (e.g. ops.Add).
//
// All bootstrap files land directly under `.doc/` — flat by default.
// `.doc/<ID>.md` is the canonical path for new files, but projects are free
// to organize content in subdirectories. `markdown.png` is written once
// under `.doc/assets/`.
func BootstrapSystem(gitAdd func(...string) error) error {
	if err := EnsureDirs(); err != nil {
		return fmt.Errorf("ensure directories: %w", err)
	}

	var createdFiles []string

	// ensure workflow registries are loaded before validating the welcome tiki;
	// on first run this may require installing the default workflow first
	writeWelcomeTiki := true
	if err := InstallDefaultWorkflow(); err != nil {
		slog.Warn("failed to install default workflow for welcome tiki validation", "error", err)
	}
	if err := LoadWorkflowFields(); err != nil {
		slog.Warn("failed to load workflow fields; skipping welcome tiki", "error", err)
		writeWelcomeTiki = false
	}

	if writeWelcomeTiki {
		if !validateSampleTiki(initialTikiTemplate) {
			slog.Info("skipping welcome tiki: frontmatter incompatible with active workflow")
		} else {
			tikiID := GenerateRandomID()
			tikiPath := filepath.Join(GetDocDir(), fmt.Sprintf("%s.md", tikiID))

			// Template embeds `id: XXXXXX` as a placeholder; replace with the
			// generated bare id. formatFrontmatterID quotes only when needed
			// (all-digit ids), so most ids emit bare while edge cases like
			// "000001" still survive strict load.
			tikiContent := strings.ReplaceAll(initialTikiTemplate, "id: XXXXXX", "id: "+formatFrontmatterID(tikiID))
			if err := os.WriteFile(tikiPath, []byte(tikiContent), 0644); err != nil {
				return fmt.Errorf("create welcome tiki: %w", err)
			}
			createdFiles = append(createdFiles, tikiPath)
		}
	}

	// Write bundled wiki entry points. The strict loader requires a
	// frontmatter id on every managed `.md` file under `.doc/`; since the
	// bundled templates ship as plain markdown, we prepend a generated id at
	// write time so each freshly-initialized project gets unique ids.
	docDir := GetDocDir()
	indexPath := filepath.Join(docDir, "index.md")
	if err := os.WriteFile(indexPath, []byte(withBundledID(indexEntryPoint)), 0644); err != nil {
		return fmt.Errorf("write document index: %w", err)
	}
	createdFiles = append(createdFiles, indexPath)

	linkedPath := filepath.Join(docDir, "linked.md")
	if err := os.WriteFile(linkedPath, []byte(withBundledID(linkedEntryPoint)), 0644); err != nil {
		return fmt.Errorf("write document linked: %w", err)
	}
	createdFiles = append(createdFiles, linkedPath)

	// markdown.png is a shared asset, written once under `.doc/assets/`.
	assetsDir := filepath.Join(docDir, "assets")
	//nolint:gosec // G301: 0755 is appropriate for project assets directory
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("create assets directory %s: %w", assetsDir, err)
	}
	markdownPNGPath := filepath.Join(assetsDir, "markdown.png")
	if err := os.WriteFile(markdownPNGPath, markdownPNG, 0644); err != nil {
		return fmt.Errorf("write markdown.png: %w", err)
	}
	createdFiles = append(createdFiles, markdownPNGPath)

	if gitAdd != nil {
		if err := gitAdd(createdFiles...); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to git add files: %v\n", err)
		}
	}

	return nil
}

// GetDefaultWorkflowYAML returns the embedded default workflow.yaml content
func GetDefaultWorkflowYAML() string {
	return embeddedKanbanYAML
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
	if err := os.WriteFile(path, []byte(embeddedKanbanYAML), 0644); err != nil {
		return fmt.Errorf("write default workflow.yaml: %w", err)
	}

	return nil
}
