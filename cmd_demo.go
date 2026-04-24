package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/bootstrap"
	"github.com/boolean-maybe/tiki/store/tikistore"
)

// demoFS holds the embedded demo project tree. The `all:` prefix includes
// dotfiles (e.g. .doc/, .gitignore) which Go's embed would otherwise skip.
// Note: embed.FS drops executable bits — fine here since no demo asset is a script.
//
//go:embed all:assets/demo
var demoFS embed.FS

const (
	demoDirName = "tiki-demo"
	demoFSRoot  = "assets/demo"
)

// runDemo extracts the embedded demo project into ./tiki-demo (on first run),
// chdirs into it, then reconciles git state according to the loaded config.
// Git reconciliation runs on every invocation so toggling TIKI_STORE_GIT between
// runs keeps the demo dir consistent with the user's config.
// Must run before config.InitPaths() in main so the new cwd is captured as the project root.
func runDemo() error {
	if info, err := os.Stat(demoDirName); err == nil && info.IsDir() {
		fmt.Printf("using existing %s directory\n", demoDirName)
	} else {
		fmt.Printf("extracting demo project into %s/...\n", demoDirName)
		if err := materializeDemo(demoDirName); err != nil {
			return fmt.Errorf("extract demo: %w", err)
		}
	}

	if err := ensureDemoWorkflow(demoDirName); err != nil {
		return fmt.Errorf("ensure demo workflow: %w", err)
	}

	if err := os.Chdir(demoDirName); err != nil {
		return fmt.Errorf("change to demo directory: %w", err)
	}

	// Reset any prior path manager state so InitPaths observes the new cwd.
	config.ResetPathManager()
	if err := config.InitPaths(); err != nil {
		return fmt.Errorf("initialize paths: %w", err)
	}

	if _, err := bootstrap.LoadConfig(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	return reconcileGit()
}

// reconcileGit brings the current directory's git state in line with the
// resolved `store.git` config. Idempotent: safe to call on fresh extracts,
// existing dirs, and pre-existing git repos.
func reconcileGit() error {
	if !config.GetStoreGit() {
		// user disabled git-backed store — leave any existing .git/ alone.
		return nil
	}

	// Check for a local .git entry, not an ancestor repo.
	// tikistore.IsGitRepo walks up the directory tree (via `git rev-parse`
	// or go-git's DetectDotGit), so running `tiki demo` from inside an
	// existing checkout would otherwise cause the demo to attach to the
	// parent repo — `git add .` would then stage demo files into the
	// caller's index. os.Stat(".git") answers the right question: does
	// *this* directory own a repo? Works for both dir-form and file-form
	// (.git-as-file points to a linked worktree).
	if _, err := os.Stat(".git"); err == nil {
		return nil
	}

	if err := tikistore.GitInit("."); err != nil {
		return fmt.Errorf("git init demo dir: %w", err)
	}

	addFn := tikistore.NewGitAdder("")
	if addFn == nil {
		// NewGitAdder returns nil on internal git-ops construction failure.
		// The user asked for `git add .`; if we cannot deliver it, fail loudly.
		return fmt.Errorf("git adder unavailable for demo dir")
	}
	if err := addFn("."); err != nil {
		return fmt.Errorf("git add demo dir: %w", err)
	}
	return nil
}

// ensureDemoWorkflow writes the embedded kanban workflow to <dst>/.doc/workflow.yaml
// when that file is absent. Idempotent: present file is left untouched so user
// edits survive re-runs; missing file self-heals (e.g. after interrupted extract
// or user deletion) rather than silently falling back to user-config scope.
func ensureDemoWorkflow(dst string) error {
	path := filepath.Join(dst, ".doc", "workflow.yaml")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create parent for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(config.GetDefaultWorkflowYAML()), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// materializeDemo walks the embedded demo tree and writes it under dst.
func materializeDemo(dst string) error {
	return fs.WalkDir(demoFS, demoFSRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip the root itself — TrimPrefix on an exact match returns the
		// input unchanged, which would create a spurious nested path.
		if path == demoFSRoot {
			return nil
		}

		rel := strings.TrimPrefix(path, demoFSRoot+"/")
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}

		data, err := demoFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return fmt.Errorf("create parent for %s: %w", target, err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		return nil
	})
}
