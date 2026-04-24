package main

import (
	"os"
	"path/filepath"
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

	for _, rel := range []string{".doc", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(demoRoot, rel)); err != nil {
			t.Errorf("missing expected entry %q: %v", rel, err)
		}
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
	// no .doc/ should have been written, because extraction was skipped
	if _, err := os.Stat(".doc"); err == nil {
		t.Errorf(".doc should not exist — reused dir should not be re-extracted")
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
