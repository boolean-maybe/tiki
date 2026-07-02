package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store/tikistore"
)

// setupDemoTest creates a hermetic environment for runDemo tests:
//   - chdirs into a fresh temp dir (so the demo extraction lands under it)
//   - points XDG_CONFIG_HOME and XDG_CACHE_HOME at temp dirs so config loading
//     never reads or writes the real user's dotfiles
//   - resets the config path manager singleton before and after the test
func setupDemoTest(t *testing.T) {
	t.Helper()

	t.Chdir(t.TempDir())

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	config.ResetPathManager()
	t.Cleanup(config.ResetPathManager)
}

// countFiles returns the number of regular files under root (recursive).
func countFiles(t *testing.T, root string) int {
	t.Helper()
	var n int
	err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return n
}

func TestRunDemo_MaterializesAllFiles(t *testing.T) {
	setupDemoTest(t)

	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	// runDemo chdirs into tiki-demo; absolute path of that dir is cwd now
	demoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if filepath.Base(demoRoot) != demoDirName {
		t.Fatalf("cwd = %q, want basename %q", demoRoot, demoDirName)
	}

	if got := countFiles(t, demoRoot); got < 90 {
		t.Errorf("file count = %d, want at least 90", got)
	}

	// the demo ships flat: workflow.yaml and .gitignore at the root, no .doc.
	for _, rel := range []string{".gitignore", "workflow.yaml"} {
		if _, err := os.Stat(filepath.Join(demoRoot, rel)); err != nil {
			t.Errorf("missing expected entry %q: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(demoRoot, ".doc")); !os.IsNotExist(err) {
		t.Error("tiki-demo must not contain a .doc directory")
	}
}

// TestRunDemo_DemoLoadsCleanlyUnderStrictIDs is the regression gate for the
// embedded demo dataset under the flat layout. It proves:
//
//  1. every demo document (workflow + plain, flat and nested) carries a valid
//     bare id in frontmatter — no missing/legacy/invalid-id diagnostics
//     when loaded through the strict store;
//  2. the store resolves a known id to a real `.md` path under the demo root,
//     regardless of the filename chosen — identity comes from frontmatter,
//     not from filename. Demo files use human-readable topical names; this
//     test must keep passing whether filenames are `<ID>.md` or topical.
func TestRunDemo_DemoLoadsCleanlyUnderStrictIDs(t *testing.T) {
	setupDemoTest(t)

	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	// runDemo materialized workflow.yaml; the store needs the status / type
	// / custom-field registries loaded from it before parsing any doc.
	if err := config.LoadWorkflowFields(); err != nil {
		t.Fatalf("LoadWorkflowFields: %v", err)
	}

	// runDemo chdirs into tiki-demo, so the scan root is cwd.
	store, err := tikistore.NewTikiStore(".")
	if err != nil {
		t.Fatalf("NewTikiStore on demo: %v", err)
	}

	diag := store.LoadDiagnostics()
	if diag != nil && diag.HasIssues() {
		t.Fatalf("demo load surfaced diagnostics: %s", diag.Summary())
	}

	// Sanity: at least some workflow tikis must be present. The embedded
	// demo has 41 workflow files with status/priority/points — all should
	// classify as workflow. If this drops to zero, a regression probably
	// made IsWorkflow default to false.
	if got := len(store.GetAllTikis()); got < 40 {
		t.Errorf("workflow tiki count after demo load = %d, want >= 40", got)
	}

	// Sanity: a known tiki id from the demo must resolve to a `.md` file under
	// the demo root. Filename is opaque — identity comes from frontmatter — so
	// we don't assert a specific name, only that it is a .md path.
	demoTiki := store.GetTiki("XVG0FN")
	if demoTiki == nil {
		t.Fatal("demo tiki XVG0FN missing after load")
	}
	if demoTiki.Path() == "" {
		t.Error("demo tiki XVG0FN has empty Path")
	}
	if !strings.HasSuffix(demoTiki.Path(), ".md") {
		t.Errorf("tiki XVG0FN resolved to %s, want a .md path", demoTiki.Path())
	}
}

// TestRunDemo_WritesEmbeddedKanbanWorkflow pins the single-source-of-truth
// contract: the demo's workflow.yaml must match the canonical embedded kanban
// workflow byte-for-byte. Compares against config.GetDefaultWorkflowYAML() —
// not a version literal — so the test survives future kanban updates.
func TestRunDemo_WritesEmbeddedKanbanWorkflow(t *testing.T) {
	setupDemoTest(t)

	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	// runDemo chdirs into tiki-demo, so workflow.yaml is at the cwd root.
	got, err := os.ReadFile("workflow.yaml")
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	if string(got) != config.GetDefaultWorkflowYAML() {
		t.Errorf("workflow.yaml does not match embedded kanban")
	}
}

// TestRunDemo_PreservesExistingWorkflowEdits pins the "write if absent"
// contract: a pre-existing workflow.yaml (e.g. user's local edits) must not
// be clobbered on re-run. Regression guard against reverting to
// "overwrite on every launch".
func TestRunDemo_PreservesExistingWorkflowEdits(t *testing.T) {
	setupDemoTest(t)

	// pre-populate tiki-demo/workflow.yaml with a sentinel that is
	// guaranteed to differ from the embedded kanban.
	if err := os.MkdirAll(demoDirName, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	custom := []byte("version: 9.9.9\n# user's edits\n")
	wfPath := filepath.Join(demoDirName, "workflow.yaml")
	if err := os.WriteFile(wfPath, custom, 0o644); err != nil {
		t.Fatalf("seed workflow: %v", err)
	}

	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	// cwd is now inside tiki-demo, so workflow.yaml is at ./workflow.yaml.
	got, err := os.ReadFile("workflow.yaml")
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	if string(got) != string(custom) {
		t.Errorf("workflow.yaml was overwritten; want preserved user edits")
	}
}

// TestRunDemo_HealsMissingWorkflowOnReuse guards the stranded-half-init
// scenario: an existing tiki-demo/ dir without workflow.yaml (e.g. from
// an interrupted prior run or a user deletion) must self-heal by writing the
// embedded kanban rather than leaving the demo in a state where
// FindWorkflowFile silently falls back to user-config scope.
func TestRunDemo_HealsMissingWorkflowOnReuse(t *testing.T) {
	setupDemoTest(t)

	// pre-create tiki-demo/ with a sentinel but no workflow.yaml
	if err := os.MkdirAll(demoDirName, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sentinelPath := filepath.Join(demoDirName, "sentinel.txt")
	if err := os.WriteFile(sentinelPath, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	got, err := os.ReadFile("workflow.yaml")
	if err != nil {
		t.Fatalf("read healed workflow: %v", err)
	}
	if string(got) != config.GetDefaultWorkflowYAML() {
		t.Errorf("healed workflow.yaml does not match embedded kanban")
	}
	if _, err := os.Stat("sentinel.txt"); err != nil {
		t.Errorf("sentinel lost during heal: %v", err)
	}
}

// TestRunDemo_HealsMissingAssetsOnReuse guards reused demo directories from
// keeping documents that reference embedded media files which were never
// extracted (or were removed after extraction). The heal is non-clobbering so
// local edits to existing assets survive, but missing bundled assets reappear.
func TestRunDemo_HealsMissingAssetsOnReuse(t *testing.T) {
	setupDemoTest(t)

	if err := os.MkdirAll(demoDirName, 0o750); err != nil {
		t.Fatalf("mkdir demo dir: %v", err)
	}
	doc := []byte("---\nid: 3GDPPQ\ntitle: diagram doc\nstatus: inbox\n---\n\n![diagram](assets/api-grpc-api.svg)\n")
	if err := os.WriteFile(filepath.Join(demoDirName, "fleet-certificate-rotation.md"), doc, 0o644); err != nil {
		t.Fatalf("write demo doc: %v", err)
	}

	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	assetPath := filepath.Join("assets", "api-grpc-api.svg")
	if _, err := os.Stat(assetPath); err != nil {
		t.Fatalf("expected missing demo asset to be healed at %s: %v", assetPath, err)
	}
}

// TestRunDemo_FlatLayout pins the demo-shape contract end-to-end: the extracted
// demo must have no .doc subtree, must place workflow documents flat at the demo
// root (with human-readable filenames — identity comes from frontmatter, not the
// filename), and must keep shared media under assets/. If a regression brings the
// old .doc layout back (via embed changes or a stray copy step) this fails loudly.
func TestRunDemo_FlatLayout(t *testing.T) {
	setupDemoTest(t)

	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	// runDemo chdirs into tiki-demo, so paths are relative to cwd.
	if _, err := os.Stat(".doc"); !os.IsNotExist(err) {
		t.Error(".doc must not exist in the flat demo")
	}

	// assets/ must exist and hold the shared markdown.png (no per-subtree copies).
	if _, err := os.Stat("assets"); err != nil {
		t.Errorf("assets/ missing: %v", err)
	}

	// At least 20 workflow markdown files must be present flat at the demo root,
	// excluding reserved files and the index. Filename is opaque — demo files
	// use human-readable topical names; this check is filename-shape-agnostic.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read demo root: %v", err)
	}
	reserved := map[string]struct{}{
		"workflow.yaml": {},
		"config.yaml":   {},
		"index.md":      {},
	}
	var workflowFileCount int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "tiki-") {
			t.Errorf("legacy-named workflow file %s; demo must not use tiki- prefix", name)
		}
		if _, isReserved := reserved[name]; isReserved {
			continue
		}
		if strings.HasSuffix(name, ".md") {
			workflowFileCount++
		}
	}
	if workflowFileCount < 20 {
		t.Errorf("flat *.md workflow file count = %d, want >= 20", workflowFileCount)
	}

	// Demo workflow docs carry explicit workflow fields (status, type,
	// priority, points); demo plain docs carry only id. Spot-check a known
	// workflow file carrying all four — if its frontmatter loses any of
	// them, the board / list / type-filter flows regress. Resolve the file
	// via the store rather than hardcoding a filename, so this test stays
	// filename-agnostic.
	if err := config.LoadWorkflowFields(); err != nil {
		t.Fatalf("LoadWorkflowFields: %v", err)
	}
	wfStore, err := tikistore.NewTikiStore(".")
	if err != nil {
		t.Fatalf("NewTikiStore on demo: %v", err)
	}
	spotTiki := wfStore.GetTiki("5LXO6Q")
	if spotTiki == nil {
		t.Fatal("demo tiki 5LXO6Q missing — cannot spot-check workflow fields")
	}
	body, err := os.ReadFile(spotTiki.Path())
	if err != nil {
		t.Fatalf("read demo workflow doc %s: %v", spotTiki.Path(), err)
	}
	for _, field := range []string{"status:", "type:", "priority:", "points:"} {
		if !strings.Contains(string(body), field) {
			t.Errorf("demo workflow doc 5LXO6Q missing %s field", field)
		}
	}

	// Plain doc spot-check: index.md is a bundled plain document —
	// it must declare an id but no workflow fields.
	plain, err := os.ReadFile("index.md")
	if err != nil {
		t.Fatalf("read demo plain doc: %v", err)
	}
	plainStr := string(plain)
	if !strings.Contains(plainStr, "id:") {
		t.Error("demo plain doc index.md missing id")
	}
	// Frontmatter ends at the second `---` line; only look above it.
	if end := strings.Index(plainStr[4:], "---"); end > 0 {
		frontmatter := plainStr[:4+end]
		for _, forbidden := range []string{"status:", "priority:", "points:"} {
			if strings.Contains(frontmatter, forbidden) {
				t.Errorf("demo plain doc index.md must not carry workflow field %s", forbidden)
			}
		}
	}

	// The demo must exercise both relative markdown links and `[[ID]]` links.
	if !strings.Contains(plainStr, "](architecture/index.md)") {
		t.Error("demo index should contain relative markdown link")
	}
	if !strings.Contains(plainStr, "[[9XPSEI]]") && !strings.Contains(plainStr, "[[XVG0FN]]") {
		t.Error("demo index should contain at least one [[ID]] link")
	}
}

func TestRunDemo_ReusesExistingDir(t *testing.T) {
	setupDemoTest(t)

	// pre-create tiki-demo/ with a sentinel file
	if err := os.MkdirAll(demoDirName, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sentinel := filepath.Join(demoDirName, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	// cwd is now inside the reused tiki-demo; sentinel should still be there
	if _, err := os.Stat("sentinel.txt"); err != nil {
		t.Errorf("sentinel missing after reuse: %v", err)
	}
	// the embedded demo tree (workflow docs, gitignore) must not have been
	// written, because extraction was skipped. workflow.yaml and assets/ are
	// allowed self-heals, not full tree extraction.
	// telemetry-timestamp-drift.md is a representative flat workflow doc from
	// the demo dataset (frontmatter id: XVG0FN).
	for _, rel := range []string{".gitignore", "telemetry-timestamp-drift.md"} {
		if _, err := os.Stat(rel); err == nil {
			t.Errorf("%s should not exist — reused dir should not be re-extracted", rel)
		}
	}
}
