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
	t.Setenv("TIKI_STORE_GIT", "false")

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

	if got := countFiles(t, demoRoot); got < 131 {
		t.Errorf("file count = %d, want at least 131", got)
	}

	for _, rel := range []string{".doc", ".gitignore", ".doc/workflow.yaml"} {
		if _, err := os.Stat(filepath.Join(demoRoot, rel)); err != nil {
			t.Errorf("missing expected entry %q: %v", rel, err)
		}
	}
}

// TestRunDemo_DemoLoadsCleanlyUnderStrictIDs is the Phase 2 regression
// gate for the embedded demo dataset. It proves two invariants end-to-end:
//
//  1. every demo document (tiki + doki, flat and nested) carries a valid
//     bare id in frontmatter — no missing/legacy/invalid-id diagnostics
//     when loaded through the strict store;
//  2. filename is decoupled from identity — the tiki files keep their
//     legacy `tiki-<slug>.md` names while the stored id is the uppercase
//     form in frontmatter, so the store must not be falling back to a
//     filename-derived id anywhere.
//
// This test is the fixture the reviewer asked for: if a future change
// reintroduces a filename-as-id assumption, this fails because
// `tiki-xvg0fn.md` contains `id: "XVG0FN"` and the two must be resolved
// independently.
func TestRunDemo_DemoLoadsCleanlyUnderStrictIDs(t *testing.T) {
	setupDemoTest(t)
	t.Setenv("TIKI_STORE_GIT", "false")

	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	// runDemo materialized workflow.yaml; the store needs the status / type
	// / custom-field registries loaded from it before parsing any doc.
	if err := config.LoadWorkflowRegistries(); err != nil {
		t.Fatalf("LoadWorkflowRegistries: %v", err)
	}

	// runDemo chdirs into tiki-demo, so .doc is relative to cwd.
	docRoot := filepath.Join(".", ".doc")
	store, err := tikistore.NewTikiStore(docRoot)
	if err != nil {
		t.Fatalf("NewTikiStore on demo: %v", err)
	}

	diag := store.LoadDiagnostics()
	if diag != nil && diag.HasIssues() {
		t.Fatalf("demo load surfaced diagnostics: %s", diag.Summary())
	}

	// Sanity: at least some workflow tasks must be present. The embedded
	// demo has 41 tiki files with status/priority/points — all should
	// classify as workflow. If this drops to zero, a regression probably
	// made IsWorkflow default to false.
	if got := len(store.GetAllTasks()); got < 40 {
		t.Errorf("workflow task count after demo load = %d, want >= 40", got)
	}

	// Sanity: a known task id from the demo must resolve. `XVG0FN` comes
	// from `tiki-xvg0fn.md` under the "filename-is-not-identity" rule; if
	// the store ever looked the task up via a reconstructed
	// `.doc/tiki/XVG0FN.md` path, it would miss it because the actual
	// file on disk is named `tiki-xvg0fn.md`.
	task := store.GetTask("XVG0FN")
	if task == nil {
		t.Fatal("demo task XVG0FN missing after load")
	}
	if !strings.HasSuffix(task.FilePath, filepath.Join("tiki", "tiki-xvg0fn.md")) {
		t.Errorf("task XVG0FN resolved to unexpected path: %s", task.FilePath)
	}
}

// TestRunDemo_WritesEmbeddedKanbanWorkflow pins the single-source-of-truth
// contract: the demo's workflow.yaml must match the canonical embedded kanban
// workflow byte-for-byte. Compares against config.GetDefaultWorkflowYAML() —
// not a version literal — so the test survives future kanban updates.
func TestRunDemo_WritesEmbeddedKanbanWorkflow(t *testing.T) {
	setupDemoTest(t)
	t.Setenv("TIKI_STORE_GIT", "false")

	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	// runDemo chdirs into tiki-demo, so .doc/workflow.yaml is relative to cwd.
	got, err := os.ReadFile(filepath.Join(".doc", "workflow.yaml"))
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
	t.Setenv("TIKI_STORE_GIT", "false")

	// pre-populate tiki-demo/.doc/workflow.yaml with a sentinel that is
	// guaranteed to differ from the embedded kanban.
	dir := filepath.Join(demoDirName, ".doc")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	custom := []byte("version: 9.9.9\n# user's edits\n")
	wfPath := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(wfPath, custom, 0o644); err != nil {
		t.Fatalf("seed workflow: %v", err)
	}

	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	// cwd is now inside tiki-demo, so workflow.yaml is at ./.doc/workflow.yaml.
	got, err := os.ReadFile(filepath.Join(".doc", "workflow.yaml"))
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	if string(got) != string(custom) {
		t.Errorf("workflow.yaml was overwritten; want preserved user edits")
	}
}

// TestRunDemo_HealsMissingWorkflowOnReuse guards the stranded-half-init
// scenario: an existing tiki-demo/ dir without .doc/workflow.yaml (e.g. from
// an interrupted prior run or a user deletion) must self-heal by writing the
// embedded kanban rather than leaving the demo in a state where
// FindWorkflowFile silently falls back to user-config or cwd scope.
func TestRunDemo_HealsMissingWorkflowOnReuse(t *testing.T) {
	setupDemoTest(t)
	t.Setenv("TIKI_STORE_GIT", "false")

	// pre-create tiki-demo/ with a sentinel but no .doc/workflow.yaml
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

	got, err := os.ReadFile(filepath.Join(".doc", "workflow.yaml"))
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

func TestRunDemo_ReusesExistingDir(t *testing.T) {
	setupDemoTest(t)
	t.Setenv("TIKI_STORE_GIT", "false")

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
	// the embedded demo tree (tikis, gitignore) must not have been written,
	// because extraction was skipped. .doc/workflow.yaml is allowed — it is
	// written by ensureDemoWorkflow, not by the tree extractor.
	for _, rel := range []string{".gitignore", ".doc/tiki"} {
		if _, err := os.Stat(rel); err == nil {
			t.Errorf("%s should not exist — reused dir should not be re-extracted", rel)
		}
	}
}

func TestRunDemo_GitInitWhenStoreGitTrue(t *testing.T) {
	setupDemoTest(t)
	t.Setenv("TIKI_STORE_GIT", "true")

	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	if _, err := os.Stat(".git"); err != nil {
		t.Errorf("expected .git/ to exist with TIKI_STORE_GIT=true: %v", err)
	}
}

func TestRunDemo_NoGitWhenDisabled(t *testing.T) {
	setupDemoTest(t)
	t.Setenv("TIKI_STORE_GIT", "false")

	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	if _, err := os.Stat(".git"); err == nil {
		t.Errorf(".git/ should not exist with TIKI_STORE_GIT=false")
	}
}

// TestRunDemo_ReconcilesGitOnSecondRun verifies the high-severity fix:
// when the first run leaves no .git/ and the user subsequently enables
// store.git, the second runDemo invocation must initialize git even though
// the tiki-demo directory already exists.
func TestRunDemo_ReconcilesGitOnSecondRun(t *testing.T) {
	setupDemoTest(t)

	// first run: no git
	t.Setenv("TIKI_STORE_GIT", "false")
	if err := runDemo(); err != nil {
		t.Fatalf("first runDemo: %v", err)
	}
	if _, err := os.Stat(".git"); err == nil {
		t.Fatalf(".git/ should not exist after first run")
	}

	// chdir back to the parent so the second call sees tiki-demo as an
	// existing dir (runDemo chdirs into it, so we need to undo that first).
	parent := filepath.Dir(mustGetwd(t))
	if err := os.Chdir(parent); err != nil {
		t.Fatalf("chdir parent: %v", err)
	}

	// second run: enable git; existing dir path must still reconcile
	t.Setenv("TIKI_STORE_GIT", "true")
	// config.ResetPathManager so InitPaths inside runDemo picks up the new cwd
	config.ResetPathManager()
	if err := runDemo(); err != nil {
		t.Fatalf("second runDemo: %v", err)
	}

	if _, err := os.Stat(".git"); err != nil {
		t.Errorf("expected .git/ after second run with TIKI_STORE_GIT=true: %v", err)
	}
}

// TestRunDemo_IsolatedRepoInsideParentRepo guards the regression where
// tikistore.IsGitRepo(".") walks up the directory tree — so running
// `tiki demo` from inside an existing git checkout would skip git init
// on the demo dir, causing subsequent `git add .` to stage demo files
// into the parent repo's index. The demo must always own its own .git.
func TestRunDemo_IsolatedRepoInsideParentRepo(t *testing.T) {
	setupDemoTest(t)

	// turn the current (parent) temp dir into a git repo first
	parent := mustGetwd(t)
	if err := tikistore.GitInit(parent); err != nil {
		t.Fatalf("git init parent: %v", err)
	}

	t.Setenv("TIKI_STORE_GIT", "true")
	if err := runDemo(); err != nil {
		t.Fatalf("runDemo: %v", err)
	}

	// cwd is now inside tiki-demo; resolve symlinks to compare reliably
	// on macOS where /var and /private/var refer to the same tree.
	demoRoot := resolvePath(t, mustGetwd(t))
	parent = resolvePath(t, parent)
	if filepath.Dir(demoRoot) != parent {
		t.Fatalf("demo cwd %q not a child of parent %q", demoRoot, parent)
	}

	// the demo must own its own .git — not inherit the parent's
	demoGit := filepath.Join(demoRoot, ".git")
	if _, err := os.Stat(demoGit); err != nil {
		t.Fatalf("expected %s to exist (demo must own its own repo): %v", demoGit, err)
	}

	// extra belt-and-braces: the parent repo's index must not have staged
	// any demo files. If reconcileGit had mistakenly skipped init and then
	// run `git add .`, the demo tree would be staged in the parent. A
	// fresh `git init` never creates .git/index until something is staged.
	parentIndex := filepath.Join(parent, ".git", "index")
	if _, err := os.Stat(parentIndex); err == nil {
		t.Errorf("parent repo has staged entries — demo leaked into parent's index at %s", parentIndex)
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return cwd
}

// resolvePath canonicalizes a path by following symlinks. Needed on macOS
// where temp dirs under /var resolve via the /private/var symlink.
func resolvePath(t *testing.T, p string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("eval symlinks %q: %v", p, err)
	}
	return resolved
}
