package gogit_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/store/internal/git/internal/gogit"
	gogitlib "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func setupTestRepo(t *testing.T) (string, *gogitlib.Repository) {
	t.Helper()
	dir := t.TempDir()
	repo, err := gogitlib.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	return dir, repo
}

func commitFile(t *testing.T, repo *gogitlib.Repository, dir, relPath, content, authorName, message string) {
	t.Helper()

	absPath := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if _, err := wt.Add(relPath); err != nil {
		t.Fatalf("Add: %v", err)
	}
	_, err = wt.Commit(message, &gogitlib.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorName + "@test.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

func commitFileAt(t *testing.T, repo *gogitlib.Repository, dir, relPath, content, authorName, message string, when time.Time) {
	t.Helper()

	absPath := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if _, err := wt.Add(relPath); err != nil {
		t.Fatalf("Add: %v", err)
	}
	_, err = wt.Commit(message, &gogitlib.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorName + "@test.com",
			When:  when,
		},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

func TestNewUtil(t *testing.T) {
	dir, _ := setupTestRepo(t)
	u, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("NewUtil: %v", err)
	}
	if u == nil {
		t.Fatal("NewUtil returned nil")
	}
}

func TestNewUtil_invalidPath(t *testing.T) {
	_, err := gogit.NewUtil(t.TempDir())
	if err == nil {
		t.Fatal("expected error for non-repo path")
	}
}

func TestNewUtil_fromSubdirectory(t *testing.T) {
	dir, repo := setupTestRepo(t)
	commitFile(t, repo, dir, "tasks/tiki-001.md", "task1", "alice", "add task")

	subDir := filepath.Join(dir, "tasks")
	u, err := gogit.NewUtil(subDir)
	if err != nil {
		t.Fatalf("NewUtil from subdirectory: %v", err)
	}

	// AllAuthors with an absolute pattern from the subdirectory should still
	// resolve against the repo root, not the subdirectory
	absPattern := filepath.Join(dir, "tasks", "*.md")
	authors, err := u.AllAuthors(absPattern)
	if err != nil {
		t.Fatalf("AllAuthors: %v", err)
	}
	if len(authors) != 1 {
		t.Fatalf("expected 1 author entry, got %d: %v", len(authors), authors)
	}
}

func TestCurrentBranch(t *testing.T) {
	dir, repo := setupTestRepo(t)
	commitFile(t, repo, dir, "init.txt", "init", "tester", "initial commit")

	u, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("NewUtil: %v", err)
	}

	branch, err := u.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch == "" {
		t.Fatal("CurrentBranch returned empty string")
	}
}

func TestAdd(t *testing.T) {
	dir, repo := setupTestRepo(t)
	commitFile(t, repo, dir, "init.txt", "init", "tester", "initial commit")

	u, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("NewUtil: %v", err)
	}

	newFile := filepath.Join(dir, "newfile.txt")
	if err := os.WriteFile(newFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := u.Add("newfile.txt"); err != nil {
		t.Fatalf("Add: %v", err)
	}
}

func TestRemove(t *testing.T) {
	dir, repo := setupTestRepo(t)
	commitFile(t, repo, dir, "removeme.txt", "content", "tester", "add file")

	u, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("NewUtil: %v", err)
	}

	if err := u.Remove("removeme.txt"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

func TestCurrentUser(t *testing.T) {
	dir, repo := setupTestRepo(t)
	commitFile(t, repo, dir, "init.txt", "init", "tester", "initial commit")

	u, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("NewUtil: %v", err)
	}

	// CurrentUser reads git config — may or may not be set in test env.
	// Just verify it doesn't panic.
	_, _, _ = u.CurrentUser()
}

func TestAuthor(t *testing.T) {
	dir, repo := setupTestRepo(t)
	commitFile(t, repo, dir, "tasks/tiki-001.md", "---\ntitle: test\n---\n", "alice", "add task")
	commitFile(t, repo, dir, "tasks/tiki-001.md", "---\ntitle: updated\n---\n", "bob", "update task")

	u, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("NewUtil: %v", err)
	}

	author, err := u.Author("tasks/tiki-001.md")
	if err != nil {
		t.Fatalf("Author: %v", err)
	}
	if author.Name != "alice" {
		t.Errorf("Author.Name = %q, want %q", author.Name, "alice")
	}
}

func TestAllAuthors(t *testing.T) {
	dir, repo := setupTestRepo(t)
	commitFile(t, repo, dir, "tasks/tiki-001.md", "task1", "alice", "add task 1")
	commitFile(t, repo, dir, "tasks/tiki-002.md", "task2", "bob", "add task 2")
	commitFile(t, repo, dir, "tasks/tiki-001.md", "task1-updated", "charlie", "update task 1")

	u, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("NewUtil: %v", err)
	}

	authors, err := u.AllAuthors("tasks/*.md")
	if err != nil {
		t.Fatalf("AllAuthors: %v", err)
	}

	if len(authors) != 2 {
		t.Fatalf("expected 2 authors, got %d", len(authors))
	}
	if authors["tasks/tiki-001.md"].Name != "alice" {
		t.Errorf("task 1 author = %q, want %q", authors["tasks/tiki-001.md"].Name, "alice")
	}
	if authors["tasks/tiki-002.md"].Name != "bob" {
		t.Errorf("task 2 author = %q, want %q", authors["tasks/tiki-002.md"].Name, "bob")
	}
}

func TestAllUsers(t *testing.T) {
	dir, repo := setupTestRepo(t)
	commitFile(t, repo, dir, "f1.txt", "a", "alice", "c1")
	commitFile(t, repo, dir, "f2.txt", "b", "bob", "c2")
	commitFile(t, repo, dir, "f3.txt", "c", "alice", "c3")

	u, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("NewUtil: %v", err)
	}

	users, err := u.AllUsers()
	if err != nil {
		t.Fatalf("AllUsers: %v", err)
	}

	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d: %v", len(users), users)
	}

	has := make(map[string]bool)
	for _, u := range users {
		has[u] = true
	}
	if !has["alice"] || !has["bob"] {
		t.Errorf("expected alice and bob, got %v", users)
	}
}

func TestLastCommitTime(t *testing.T) {
	dir, repo := setupTestRepo(t)

	t1 := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	commitFileAt(t, repo, dir, "tasks/tiki-001.md", "v1", "alice", "c1", t1)
	commitFileAt(t, repo, dir, "tasks/tiki-001.md", "v2", "bob", "c2", t2)

	u, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("NewUtil: %v", err)
	}

	lastTime, err := u.LastCommitTime("tasks/tiki-001.md")
	if err != nil {
		t.Fatalf("LastCommitTime: %v", err)
	}

	if !lastTime.Equal(t2) {
		t.Errorf("LastCommitTime = %v, want %v", lastTime, t2)
	}
}

func TestAllLastCommitTimes(t *testing.T) {
	dir, repo := setupTestRepo(t)

	t1 := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 8, 1, 12, 0, 0, 0, time.UTC)

	commitFileAt(t, repo, dir, "tasks/tiki-001.md", "v1", "alice", "c1", t1)
	commitFileAt(t, repo, dir, "tasks/tiki-002.md", "v1", "bob", "c2", t2)
	commitFileAt(t, repo, dir, "tasks/tiki-001.md", "v2", "charlie", "c3", t3)

	u, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("NewUtil: %v", err)
	}

	times, err := u.AllLastCommitTimes("tasks/*.md")
	if err != nil {
		t.Fatalf("AllLastCommitTimes: %v", err)
	}

	if len(times) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(times))
	}
	if !times["tasks/tiki-001.md"].Equal(t3) {
		t.Errorf("task 1 time = %v, want %v", times["tasks/tiki-001.md"], t3)
	}
	if !times["tasks/tiki-002.md"].Equal(t2) {
		t.Errorf("task 2 time = %v, want %v", times["tasks/tiki-002.md"], t2)
	}
}

func TestFileVersionsSince(t *testing.T) {
	dir, repo := setupTestRepo(t)

	t1 := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 8, 1, 12, 0, 0, 0, time.UTC)

	commitFileAt(t, repo, dir, "f.txt", "version1", "alice", "c1", t1)
	commitFileAt(t, repo, dir, "f.txt", "version2", "bob", "c2", t2)
	commitFileAt(t, repo, dir, "f.txt", "version3", "charlie", "c3", t3)

	u, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("NewUtil: %v", err)
	}

	// since mid-2024, no prior
	since := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	versions, err := u.FileVersionsSince("f.txt", since, false)
	if err != nil {
		t.Fatalf("FileVersionsSince: %v", err)
	}

	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0].Content != "version2" {
		t.Errorf("first version content = %q, want %q", versions[0].Content, "version2")
	}
	if versions[1].Content != "version3" {
		t.Errorf("second version content = %q, want %q", versions[1].Content, "version3")
	}

	// with prior
	versions, err = u.FileVersionsSince("f.txt", since, true)
	if err != nil {
		t.Fatalf("FileVersionsSince with prior: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions (1 prior + 2 since), got %d", len(versions))
	}
	if versions[0].Content != "version1" {
		t.Errorf("prior version content = %q, want %q", versions[0].Content, "version1")
	}
}

func TestAllFileVersionsSince(t *testing.T) {
	dir, repo := setupTestRepo(t)

	t1 := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 8, 1, 12, 0, 0, 0, time.UTC)

	commitFileAt(t, repo, dir, "tasks/a.md", "---\nstatus: open\n---\n", "alice", "c1", t1)
	commitFileAt(t, repo, dir, "tasks/a.md", "---\nstatus: closed\n---\n", "bob", "c2", t2)
	commitFileAt(t, repo, dir, "tasks/b.md", "---\nstatus: open\n---\n", "charlie", "c3", t3)

	u, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("NewUtil: %v", err)
	}

	since := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	result, err := u.AllFileVersionsSince("tasks/*.md", since, false)
	if err != nil {
		t.Fatalf("AllFileVersionsSince: %v", err)
	}

	// Should have at least 1 file with status changes in the window
	if len(result) == 0 {
		t.Fatal("expected at least 1 file in results")
	}
}
