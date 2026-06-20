package tikistore

import (
	"os/exec"
	"strings"
	"testing"

	gitops "github.com/boolean-maybe/tiki/store/internal/git"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// runGit runs a git subcommand in dir and returns its combined output.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	//nolint:gosec // G204: test-controlled git subcommand args
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// TestSaveDoesNotStage verifies the store never stages files in git. Even with
// a working git util explicitly rooted at the repo (so a stray `git add` would
// succeed), a newly created tiki file must remain untracked — git integration
// is read-only (history/authors), never a writer.
func TestSaveDoesNotStage(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "testuser")
	// an initial commit gives the repo a HEAD so the git backend initializes
	// cleanly and a real `git add` would actually stage.
	runGit(t, dir, "commit", "--allow-empty", "-m", "init")

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	// inject a git util explicitly rooted at the repo so that, if the store
	// still called Add, the file would genuinely be staged — this is what makes
	// the test a real guard against a staging regression rather than passing
	// because git happened to error.
	gu, err := gitops.NewGitOps(dir)
	if err != nil {
		t.Fatalf("NewGitOps: %v", err)
	}
	s.gitUtil = gu

	tk := tikipkg.New()
	tk.SetID("ABC123")
	tk.SetTitle("x")
	tk.Set("type", "story")
	tk.Set("status", "inbox")
	tk.Set("priority", "medium")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	out := runGit(t, dir, "status", "--porcelain")
	if strings.Contains(out, "A  ABC123.md") {
		t.Fatalf("file was staged; expected untracked:\n%s", out)
	}
}

// TestDeleteDoesNotStage verifies the mirror of TestSaveDoesNotStage for the
// delete path: removing a tiki deletes the working-tree file but must not
// stage the removal in the index. Git integration is read-only.
func TestDeleteDoesNotStage(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "testuser")

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	gu, err := gitops.NewGitOps(dir)
	if err != nil {
		t.Fatalf("NewGitOps: %v", err)
	}
	s.gitUtil = gu

	tk := tikipkg.New()
	tk.SetID("DEF456")
	tk.SetTitle("y")
	tk.Set("type", "story")
	tk.Set("status", "inbox")
	tk.Set("priority", "medium")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	// commit the file so its deletion would show as a tracked change.
	runGit(t, dir, "add", "DEF456.md")
	runGit(t, dir, "commit", "-m", "add DEF456")

	s.DeleteTiki("DEF456")

	// the working-tree file must be gone, and the deletion must be UNstaged
	// (" D", leading space) — never staged ("D ", as `git rm` would produce).
	out := runGit(t, dir, "status", "--porcelain")
	if strings.Contains(out, "D  DEF456.md") {
		t.Fatalf("deletion was staged; expected unstaged:\n%s", out)
	}
	if !strings.Contains(out, " D DEF456.md") {
		t.Fatalf("expected an unstaged deletion of DEF456.md:\n%s", out)
	}
}
