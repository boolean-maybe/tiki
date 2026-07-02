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
