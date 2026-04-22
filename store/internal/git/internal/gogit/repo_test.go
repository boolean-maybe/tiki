package gogit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/store/internal/git/internal/gogit"
	gogitlib "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestInit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	if err := gogit.Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf(".git not created: %v", err)
	}
}

func TestClone(t *testing.T) {
	// create a source repo with one commit (go-git cannot clone empty repos)
	src := filepath.Join(t.TempDir(), "src")
	repo, err := gogitlib.PlainInit(src, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	f := filepath.Join(src, "readme.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := wt.Add("readme.txt"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := wt.Commit("init", &gogitlib.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com"},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	dest := filepath.Join(t.TempDir(), "clone")
	if err := gogit.Clone(src, dest, os.Stdout, os.Stderr); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Fatalf(".git not created in clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "readme.txt")); err != nil {
		t.Fatalf("cloned file not found: %v", err)
	}
}
