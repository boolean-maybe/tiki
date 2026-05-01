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

//go:embed index.md
var dokiEntryPoint string

//go:embed linked.md
var dokiLinked string

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

// withBundledDokiID returns body prepended with a minimal frontmatter
// block carrying a freshly-generated bare id. Used for the two bundled
// doki templates (`index.md`, `linked.md`) written during project init.
// Phase 2's strict loader rejects any `.md` under `.doc/` without an id,
// so these templates cannot ship as plain markdown — but embedding a
// literal id in the template file itself would collide across projects,
// so we wrap at write time instead.
//
// If body already starts with a frontmatter fence, the body is returned
// unchanged — protection against double-wrapping if a future template
// gets its own frontmatter.
func withBundledDokiID(body string) string {
	if strings.HasPrefix(body, "---\n") {
		return body
	}
	return fmt.Sprintf("---\nid: %q\n---\n%s", GenerateRandomID(), body)
}

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

// BootstrapSystem creates the document storage and seeds the initial sample
// documents. If createSamples is true, embedded workflow samples are validated
// against the active workflow registries and only valid ones are written.
// gitAdd, when non-nil, is called to stage created files (e.g. ops.Add).
//
// Phase 8 of the unified-document migration removes all references to the
// legacy `.doc/tiki` and `.doc/doki` subdirectories here:
//   - samples land directly under `.doc/<ID>.md`
//   - bundled doki index/linked land directly under `.doc/`
//   - `markdown.png` is written once under `.doc/assets/`, not duplicated
//     into two legacy subdirectories.
func BootstrapSystem(createSamples bool, gitAdd func(...string) error) error {
	// Create all necessary directories
	if err := EnsureDirs(); err != nil {
		return fmt.Errorf("ensure directories: %w", err)
	}

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

		// Samples land directly under the unified document root (`.doc/<ID>.md`).
		// This matches the default new-document location used by CreateTask so
		// bundled samples and user-created tasks share one layout.
		sampleDir := GetDocDir()
		for _, s := range samples {
			if !validateSampleTiki(s.template) {
				slog.Info("skipping incompatible sample tiki", "name", s.name)
				continue
			}

			taskID := GenerateRandomID()
			// Phase 1 filename convention: <ID>.md (bare, case-preserving).
			taskFilename := fmt.Sprintf("%s.md", taskID)
			taskPath := filepath.Join(sampleDir, taskFilename)

			// Samples embed `id: XXXXXX` as a placeholder; replace with the
			// generated bare id for this instance. The replacement is
			// YAML-quoted so all-digit ids (e.g. "000001") survive strict
			// load — without quotes, yaml.v3 decodes them as integers and
			// loses leading zeros, failing ValidateID.
			// Also back-compat with any pre-unification references to
			// "TIKI-XXXXXX" that may linger in sample prose (not frontmatter).
			taskContent := strings.ReplaceAll(s.template, "id: XXXXXX", fmt.Sprintf("id: %q", taskID))
			taskContent = strings.ReplaceAll(taskContent, "TIKI-XXXXXX", taskID)
			if err := os.WriteFile(taskPath, []byte(taskContent), 0644); err != nil {
				return fmt.Errorf("create sample %s: %w", s.name, err)
			}
			createdFiles = append(createdFiles, taskPath)
		}
	}

	// Write bundled documentation templates. Phase 2's strict loader requires
	// a frontmatter id on every managed `.md` file under `.doc/`; since the
	// bundled templates ship as plain markdown, we prepend a generated id at
	// write time so each freshly-initialized project gets unique ids for its
	// bundled docs. Phase 8 lands these at the unified document root — no
	// more `.doc/doki/` subdirectory.
	docDir := GetDocDir()
	indexPath := filepath.Join(docDir, "index.md")
	if err := os.WriteFile(indexPath, []byte(withBundledDokiID(dokiEntryPoint)), 0644); err != nil {
		return fmt.Errorf("write document index: %w", err)
	}
	createdFiles = append(createdFiles, indexPath)

	linkedPath := filepath.Join(docDir, "linked.md")
	if err := os.WriteFile(linkedPath, []byte(withBundledDokiID(dokiLinked)), 0644); err != nil {
		return fmt.Errorf("write document linked: %w", err)
	}
	createdFiles = append(createdFiles, linkedPath)

	// Phase 8: markdown.png is a shared asset, written once under
	// `.doc/assets/`. Prior releases duplicated it into both `.doc/tiki/` and
	// `.doc/doki/` so each subdirectory could resolve `![](markdown.png)`
	// locally; under the unified layout there is one root for documents and
	// one well-known asset directory.
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
