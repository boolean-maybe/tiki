package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/store/internal/git/internal/gogit"
	"github.com/boolean-maybe/tiki/store/internal/git/internal/shell"
	"github.com/boolean-maybe/tiki/store/internal/git/internal/types"
	gogitlib "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// setupParityRepo creates a temp git repo with deterministic history for parity testing.
// Uses go-git for commits to avoid needing shell git config.
func setupParityRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	repo, err := gogitlib.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	base := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// commit 1: alice creates two task files
	for _, name := range []string{"tasks/tiki-001.md", "tasks/tiki-002.md"} {
		abs := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		content := "---\ntitle: " + name + "\nstatus: open\n---\n"
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if _, err := wt.Add(name); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if _, err := wt.Commit("alice creates tasks", &gogitlib.CommitOptions{
		Author: &object.Signature{Name: "alice", Email: "alice@test.com", When: base},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// commit 2: bob changes status of tiki-001
	t2 := base.Add(30 * 24 * time.Hour)
	abs := filepath.Join(dir, "tasks/tiki-001.md")
	if err := os.WriteFile(abs, []byte("---\ntitle: tasks/tiki-001.md\nstatus: closed\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := wt.Add("tasks/tiki-001.md"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := wt.Commit("bob closes task 1", &gogitlib.CommitOptions{
		Author: &object.Signature{Name: "bob", Email: "bob@test.com", When: t2},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// commit 3: charlie creates tiki-003
	t3 := base.Add(60 * 24 * time.Hour)
	abs3 := filepath.Join(dir, "tasks/tiki-003.md")
	if err := os.WriteFile(abs3, []byte("---\ntitle: task 3\nstatus: open\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := wt.Add("tasks/tiki-003.md"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := wt.Commit("charlie creates task 3", &gogitlib.CommitOptions{
		Author: &object.Signature{Name: "charlie", Email: "charlie@test.com", When: t3},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// set git config for shell backend (needed for CurrentUser)
	gitCmd := exec.Command("git", "config", "user.name", "testuser")
	gitCmd.Dir = dir
	_ = gitCmd.Run()
	gitCmd = exec.Command("git", "config", "user.email", "testuser@test.com")
	gitCmd.Dir = dir
	_ = gitCmd.Run()

	return dir
}

func requireShellGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("shell git not available")
	}
}

func TestParity_CurrentBranch(t *testing.T) {
	requireShellGit(t)
	dir := setupParityRepo(t)

	sh, err := shell.NewUtil(dir)
	if err != nil {
		t.Fatalf("shell.NewUtil: %v", err)
	}
	gg, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("gogit.NewUtil: %v", err)
	}

	shellBranch, err := sh.CurrentBranch()
	if err != nil {
		t.Fatalf("shell.CurrentBranch: %v", err)
	}
	gogitBranch, err := gg.CurrentBranch()
	if err != nil {
		t.Fatalf("gogit.CurrentBranch: %v", err)
	}

	if shellBranch != gogitBranch {
		t.Errorf("CurrentBranch mismatch: shell=%q gogit=%q", shellBranch, gogitBranch)
	}
}

func TestParity_Author(t *testing.T) {
	requireShellGit(t)
	dir := setupParityRepo(t)

	sh, err := shell.NewUtil(dir)
	if err != nil {
		t.Fatalf("shell.NewUtil: %v", err)
	}
	gg, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("gogit.NewUtil: %v", err)
	}

	for _, file := range []string{"tasks/tiki-001.md", "tasks/tiki-002.md", "tasks/tiki-003.md"} {
		shellAuthor, err := sh.Author(file)
		if err != nil {
			t.Fatalf("shell.Author(%s): %v", file, err)
		}
		gogitAuthor, err := gg.Author(file)
		if err != nil {
			t.Fatalf("gogit.Author(%s): %v", file, err)
		}

		if shellAuthor.Name != gogitAuthor.Name {
			t.Errorf("Author(%s) name: shell=%q gogit=%q", file, shellAuthor.Name, gogitAuthor.Name)
		}
		if shellAuthor.Email != gogitAuthor.Email {
			t.Errorf("Author(%s) email: shell=%q gogit=%q", file, shellAuthor.Email, gogitAuthor.Email)
		}
	}
}

func TestParity_AllAuthors(t *testing.T) {
	requireShellGit(t)
	dir := setupParityRepo(t)

	sh, err := shell.NewUtil(dir)
	if err != nil {
		t.Fatalf("shell.NewUtil: %v", err)
	}
	gg, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("gogit.NewUtil: %v", err)
	}

	pattern := filepath.Join(dir, "tasks", "*.md")
	shellAuthors, err := sh.AllAuthors(pattern)
	if err != nil {
		t.Fatalf("shell.AllAuthors: %v", err)
	}
	gogitAuthors, err := gg.AllAuthors(pattern)
	if err != nil {
		t.Fatalf("gogit.AllAuthors: %v", err)
	}

	// normalize keys and compare
	shellKeys := sortedKeys(shellAuthors)
	gogitKeys := sortedKeys(gogitAuthors)

	if len(shellKeys) != len(gogitKeys) {
		t.Fatalf("AllAuthors key count: shell=%d gogit=%d\nshell=%v\ngogit=%v", len(shellKeys), len(gogitKeys), shellKeys, gogitKeys)
	}

	for _, key := range shellKeys {
		sa := shellAuthors[key]
		ga, ok := gogitAuthors[key]
		if !ok {
			t.Errorf("AllAuthors: shell has key %q, gogit does not", key)
			continue
		}
		if sa.Name != ga.Name {
			t.Errorf("AllAuthors[%s] name: shell=%q gogit=%q", key, sa.Name, ga.Name)
		}
	}
}

func TestParity_AllUsers(t *testing.T) {
	requireShellGit(t)
	dir := setupParityRepo(t)

	sh, err := shell.NewUtil(dir)
	if err != nil {
		t.Fatalf("shell.NewUtil: %v", err)
	}
	gg, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("gogit.NewUtil: %v", err)
	}

	shellUsers, err := sh.AllUsers()
	if err != nil {
		t.Fatalf("shell.AllUsers: %v", err)
	}
	gogitUsers, err := gg.AllUsers()
	if err != nil {
		t.Fatalf("gogit.AllUsers: %v", err)
	}

	// both should have same users in same order (first-seen traversal)
	if len(shellUsers) != len(gogitUsers) {
		t.Fatalf("AllUsers count: shell=%d gogit=%d\nshell=%v\ngogit=%v", len(shellUsers), len(gogitUsers), shellUsers, gogitUsers)
	}

	for i := range shellUsers {
		if shellUsers[i] != gogitUsers[i] {
			t.Errorf("AllUsers[%d]: shell=%q gogit=%q", i, shellUsers[i], gogitUsers[i])
		}
	}
}

func TestParity_LastCommitTime(t *testing.T) {
	requireShellGit(t)
	dir := setupParityRepo(t)

	sh, err := shell.NewUtil(dir)
	if err != nil {
		t.Fatalf("shell.NewUtil: %v", err)
	}
	gg, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("gogit.NewUtil: %v", err)
	}

	for _, file := range []string{"tasks/tiki-001.md", "tasks/tiki-002.md"} {
		shellTime, err := sh.LastCommitTime(file)
		if err != nil {
			t.Fatalf("shell.LastCommitTime(%s): %v", file, err)
		}
		gogitTime, err := gg.LastCommitTime(file)
		if err != nil {
			t.Fatalf("gogit.LastCommitTime(%s): %v", file, err)
		}

		if !shellTime.Equal(gogitTime) {
			t.Errorf("LastCommitTime(%s): shell=%v gogit=%v", file, shellTime, gogitTime)
		}
	}
}

func TestParity_AllLastCommitTimes(t *testing.T) {
	requireShellGit(t)
	dir := setupParityRepo(t)

	sh, err := shell.NewUtil(dir)
	if err != nil {
		t.Fatalf("shell.NewUtil: %v", err)
	}
	gg, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("gogit.NewUtil: %v", err)
	}

	pattern := filepath.Join(dir, "tasks", "*.md")
	shellTimes, err := sh.AllLastCommitTimes(pattern)
	if err != nil {
		t.Fatalf("shell.AllLastCommitTimes: %v", err)
	}
	gogitTimes, err := gg.AllLastCommitTimes(pattern)
	if err != nil {
		t.Fatalf("gogit.AllLastCommitTimes: %v", err)
	}

	shellKeys := sortedTimeKeys(shellTimes)
	gogitKeys := sortedTimeKeys(gogitTimes)

	if len(shellKeys) != len(gogitKeys) {
		t.Fatalf("AllLastCommitTimes key count: shell=%d gogit=%d\nshell=%v\ngogit=%v", len(shellKeys), len(gogitKeys), shellKeys, gogitKeys)
	}

	for _, key := range shellKeys {
		st := shellTimes[key]
		gt, ok := gogitTimes[key]
		if !ok {
			t.Errorf("AllLastCommitTimes: shell has key %q, gogit does not", key)
			continue
		}
		if !st.Equal(gt) {
			t.Errorf("AllLastCommitTimes[%s]: shell=%v gogit=%v", key, st, gt)
		}
	}
}

func TestParity_FileVersionsSince(t *testing.T) {
	requireShellGit(t)
	dir := setupParityRepo(t)

	sh, err := shell.NewUtil(dir)
	if err != nil {
		t.Fatalf("shell.NewUtil: %v", err)
	}
	gg, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("gogit.NewUtil: %v", err)
	}

	since := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	for _, includePrior := range []bool{false, true} {
		shellVersions, err := sh.FileVersionsSince("tasks/tiki-001.md", since, includePrior)
		if err != nil {
			t.Fatalf("shell.FileVersionsSince(prior=%v): %v", includePrior, err)
		}
		gogitVersions, err := gg.FileVersionsSince("tasks/tiki-001.md", since, includePrior)
		if err != nil {
			t.Fatalf("gogit.FileVersionsSince(prior=%v): %v", includePrior, err)
		}

		if len(shellVersions) != len(gogitVersions) {
			t.Fatalf("FileVersionsSince(prior=%v) count: shell=%d gogit=%d", includePrior, len(shellVersions), len(gogitVersions))
		}

		for i := range shellVersions {
			if shellVersions[i].Content != gogitVersions[i].Content {
				t.Errorf("FileVersionsSince(prior=%v)[%d] content mismatch:\nshell=%q\ngogit=%q", includePrior, i, shellVersions[i].Content, gogitVersions[i].Content)
			}
			if shellVersions[i].Author != gogitVersions[i].Author {
				t.Errorf("FileVersionsSince(prior=%v)[%d] author: shell=%q gogit=%q", includePrior, i, shellVersions[i].Author, gogitVersions[i].Author)
			}
		}
	}
}

func TestParity_AllFileVersionsSince(t *testing.T) {
	requireShellGit(t)
	dir := setupParityRepo(t)

	sh, err := shell.NewUtil(dir)
	if err != nil {
		t.Fatalf("shell.NewUtil: %v", err)
	}
	gg, err := gogit.NewUtil(dir)
	if err != nil {
		t.Fatalf("gogit.NewUtil: %v", err)
	}

	since := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	pattern := filepath.Join(dir, "tasks", "*.md")

	for _, includePrior := range []bool{false, true} {
		shellResult, err := sh.AllFileVersionsSince(pattern, since, includePrior)
		if err != nil {
			t.Fatalf("shell.AllFileVersionsSince(prior=%v): %v", includePrior, err)
		}
		gogitResult, err := gg.AllFileVersionsSince(pattern, since, includePrior)
		if err != nil {
			t.Fatalf("gogit.AllFileVersionsSince(prior=%v): %v", includePrior, err)
		}

		shellFileKeys := sortedVersionKeys(shellResult)
		gogitFileKeys := sortedVersionKeys(gogitResult)

		if len(shellFileKeys) != len(gogitFileKeys) {
			t.Errorf("AllFileVersionsSince(prior=%v) file count: shell=%d gogit=%d\nshell=%v\ngogit=%v",
				includePrior, len(shellFileKeys), len(gogitFileKeys), shellFileKeys, gogitFileKeys)
			continue
		}

		for _, key := range shellFileKeys {
			sv := shellResult[key]
			gv := gogitResult[key]

			// sort versions by time then hash for stable comparison
			sortVersions(sv)
			sortVersions(gv)

			if len(sv) != len(gv) {
				t.Errorf("AllFileVersionsSince(prior=%v)[%s] version count: shell=%d gogit=%d",
					includePrior, key, len(sv), len(gv))
				continue
			}

			for i := range sv {
				if sv[i].Content != gv[i].Content {
					t.Errorf("AllFileVersionsSince(prior=%v)[%s][%d] content mismatch", includePrior, key, i)
				}
				if sv[i].Author != gv[i].Author {
					t.Errorf("AllFileVersionsSince(prior=%v)[%s][%d] author: shell=%q gogit=%q", includePrior, key, i, sv[i].Author, gv[i].Author)
				}
			}
		}
	}
}

func sortedKeys(m map[string]*types.AuthorInfo) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedTimeKeys(m map[string]time.Time) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedVersionKeys(m map[string][]types.FileVersion) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortVersions(v []types.FileVersion) {
	sort.SliceStable(v, func(i, j int) bool {
		if v[i].When.Equal(v[j].When) {
			return v[i].Hash < v[j].Hash
		}
		return v[i].When.Before(v[j].When)
	})
}
