package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/config"
)

// demoFS holds the embedded demo project tree. The `all:` prefix includes
// dotfiles (e.g. .gitignore) which Go's embed would otherwise skip.
// Note: embed.FS drops executable bits — fine here since no demo asset is a script.
//
//go:embed all:assets/demo
var demoFS embed.FS

const (
	demoDirName = "tiki-demo"
	demoFSRoot  = "assets/demo"
)

// runDemo extracts the embedded demo project into ./tiki-demo (on first run),
// then chdirs into it and re-initializes the path manager so the new cwd is
// the document scan root. The demo is a flat directory of Markdown — no .doc
// subdir, no git. Must run before config.InitPaths() in main so the new cwd is
// captured as the project root.
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
	if err := ensureDemoAssets(demoDirName); err != nil {
		return fmt.Errorf("ensure demo assets: %w", err)
	}

	if err := os.Chdir(demoDirName); err != nil {
		return fmt.Errorf("change to demo directory: %w", err)
	}

	// reset any prior path manager state so InitPaths observes the new cwd
	// (now the demo dir) as the document scan root.
	config.ResetPathManager()
	if err := config.InitPaths(); err != nil {
		return fmt.Errorf("initialize paths: %w", err)
	}

	return nil
}

// ensureDemoWorkflow writes the canonical embedded kanban workflow to
// <dst>/workflow.yaml when that file is absent. It is the single source of the
// demo's workflow: the embedded demo tree (demoFS) deliberately ships no
// workflow.yaml of its own, so this is the only writer — config.GetDefaultWorkflowYAML()
// is the one authoritative copy. Idempotent: a present file is left untouched so
// user edits survive re-runs; a missing file self-heals (first run, interrupted
// extract, or user deletion).
func ensureDemoWorkflow(dst string) error {
	path := filepath.Join(dst, "workflow.yaml")
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

// ensureDemoAssets restores bundled demo media that existing demo directories
// may be missing, without overwriting local edits to files already present.
func ensureDemoAssets(dst string) error {
	// embed.FS paths always use forward slashes; use path.Join, not
	// filepath.Join, which would emit backslashes on Windows and break the walk.
	assetRoot := path.Join(demoFSRoot, "assets")
	return fs.WalkDir(demoFS, assetRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == assetRoot {
			return nil
		}

		rel := strings.TrimPrefix(path, demoFSRoot+"/")
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		if _, err := os.Stat(target); err == nil {
			return nil
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
