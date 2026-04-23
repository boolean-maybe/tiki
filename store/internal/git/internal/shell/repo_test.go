package shell_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/store/internal/git/internal/shell"
)

func requireShellGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("shell git not available")
	}
}

func TestInit(t *testing.T) {
	requireShellGit(t)
	dir := filepath.Join(t.TempDir(), "repo")
	if err := shell.Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf(".git not created: %v", err)
	}
}

func TestClone(t *testing.T) {
	requireShellGit(t)

	// create a source repo with one commit for consistent behavior across backends
	src := filepath.Join(t.TempDir(), "src")
	if err := shell.Init(src); err != nil {
		t.Fatalf("Init: %v", err)
	}
	f := filepath.Join(src, "readme.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cmd := exec.Command("git", "add", "readme.txt")
	cmd.Dir = src
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %s", out)
	}
	cmd = exec.Command("git", "-c", "user.name=test", "-c", "user.email=test@test.com", "commit", "-m", "init")
	cmd.Dir = src
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %s", out)
	}

	dest := filepath.Join(t.TempDir(), "clone")
	if err := shell.Clone(src, dest, os.Stdout, os.Stderr); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Fatalf(".git not created in clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "readme.txt")); err != nil {
		t.Fatalf("cloned file not found: %v", err)
	}
}
