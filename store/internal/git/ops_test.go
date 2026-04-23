package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/store/internal/git"
	gogitlib "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := gogitlib.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}

	// need at least one commit for most operations
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	initFile := filepath.Join(dir, "init.txt")
	if err := os.WriteFile(initFile, []byte("init"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := wt.Add("init.txt"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := wt.Commit("initial commit", &gogitlib.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com"},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	return dir
}

func TestNewGitOps_constructorIsCheap(t *testing.T) {
	dir := setupTestRepo(t)

	ops, err := git.NewGitOps(dir)
	if err != nil {
		t.Fatalf("NewGitOps: %v", err)
	}
	if ops == nil {
		t.Fatal("NewGitOps returned nil")
	}
}

func TestNewGitOps_lazyInitialization(t *testing.T) {
	// constructor succeeds even for a non-repo path because init is deferred
	ops, err := git.NewGitOps(t.TempDir())
	if err != nil {
		t.Fatalf("NewGitOps should not fail at construction: %v", err)
	}

	// first method call triggers real init — should fail for non-repo
	_, err = ops.CurrentBranch()
	if err == nil {
		t.Fatal("expected error for non-repo on first method call")
	}
}

func TestNewGitOps_methodsWork(t *testing.T) {
	dir := setupTestRepo(t)

	ops, err := git.NewGitOps(dir)
	if err != nil {
		t.Fatalf("NewGitOps: %v", err)
	}

	branch, err := ops.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch == "" {
		t.Fatal("CurrentBranch returned empty")
	}
}

func TestIsRepo(t *testing.T) {
	dir := setupTestRepo(t)

	if !git.IsRepo(dir) {
		t.Fatalf("IsRepo(%q) = false, want true", dir)
	}

	nonRepo := t.TempDir()
	if git.IsRepo(nonRepo) {
		t.Fatalf("IsRepo(%q) = true, want false", nonRepo)
	}
}

func TestIsRepo_emptyPath(t *testing.T) {
	// should use cwd — may or may not be a repo, just ensure it doesn't panic
	_ = git.IsRepo("")
}
