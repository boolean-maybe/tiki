package store

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/store/internal/git"
)

// mockGitOps implements git.GitOps for testing TaskHistory without a real git repo.
type mockGitOps struct {
	allFileVersions map[string][]git.FileVersion
}

func (m *mockGitOps) Add(...string) error                    { return nil }
func (m *mockGitOps) Remove(...string) error                 { return nil }
func (m *mockGitOps) CurrentUser() (string, string, error)   { return "", "", nil }
func (m *mockGitOps) Author(string) (*git.AuthorInfo, error) { return nil, nil }
func (m *mockGitOps) AllAuthors(string) (map[string]*git.AuthorInfo, error) {
	return nil, nil
}
func (m *mockGitOps) LastCommitTime(string) (time.Time, error) { return time.Time{}, nil }
func (m *mockGitOps) AllLastCommitTimes(string) (map[string]time.Time, error) {
	return nil, nil
}
func (m *mockGitOps) CurrentBranch() (string, error) { return "", nil }
func (m *mockGitOps) FileVersionsSince(string, time.Time, bool) ([]git.FileVersion, error) {
	return nil, nil
}
func (m *mockGitOps) AllFileVersionsSince(_ string, _ time.Time, _ bool) (map[string][]git.FileVersion, error) {
	return m.allFileVersions, nil
}
func (m *mockGitOps) AllUsers() ([]string, error) { return nil, nil }

func taskContent(status string) string {
	return "---\nstatus: " + status + "\n---\nsome body\n"
}

func TestBurndown_StableActiveTask(t *testing.T) {
	// a task set to inProgress before the window with no changes since
	// should appear as baseActive=1, all burndown points = 1
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	beforeWindow := now.AddDate(0, 0, -20) // 20 days ago, well before 14-day window

	mock := &mockGitOps{
		allFileVersions: map[string][]git.FileVersion{
			"tasks/tiki-aaaaaa.md": {
				{Hash: "abc123", Author: "dev", Email: "dev@test.com", When: beforeWindow, Content: taskContent("in_progress")},
			},
		},
	}

	h := NewTaskHistory("tasks", mock)
	h.now = func() time.Time { return now }

	if err := h.Build(); err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if h.baseActive != 1 {
		t.Errorf("baseActive = %d, want 1", h.baseActive)
	}

	points := h.Burndown()
	if len(points) == 0 {
		t.Fatal("Burndown() returned no points")
	}
	for i, p := range points {
		if p.Remaining != 1 {
			t.Errorf("point[%d] Remaining = %d, want 1", i, p.Remaining)
			break
		}
	}
}

func TestBurndown_StableDoneTask(t *testing.T) {
	// a done task before the window should not count as active
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	beforeWindow := now.AddDate(0, 0, -20)

	mock := &mockGitOps{
		allFileVersions: map[string][]git.FileVersion{
			"tasks/tiki-bbbbbb.md": {
				{Hash: "def456", Author: "dev", Email: "dev@test.com", When: beforeWindow, Content: taskContent("done")},
			},
		},
	}

	h := NewTaskHistory("tasks", mock)
	h.now = func() time.Time { return now }

	if err := h.Build(); err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if h.baseActive != 0 {
		t.Errorf("baseActive = %d, want 0", h.baseActive)
	}

	points := h.Burndown()
	for i, p := range points {
		if p.Remaining != 0 {
			t.Errorf("point[%d] Remaining = %d, want 0", i, p.Remaining)
			break
		}
	}
}

func TestBurndown_MixedStableAndTransitioning(t *testing.T) {
	// one stable active task + one task that transitions from inProgress to done mid-window
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	beforeWindow := now.AddDate(0, 0, -20)
	midWindow := now.AddDate(0, 0, -5)

	mock := &mockGitOps{
		allFileVersions: map[string][]git.FileVersion{
			// stable active — only prior commit
			"tasks/tiki-cccccc.md": {
				{Hash: "aaa111", Author: "dev", Email: "dev@test.com", When: beforeWindow, Content: taskContent("in_progress")},
			},
			// transitioning — prior commit + change within window
			"tasks/tiki-dddddd.md": {
				{Hash: "bbb222", Author: "dev", Email: "dev@test.com", When: beforeWindow, Content: taskContent("in_progress")},
				{Hash: "ccc333", Author: "dev", Email: "dev@test.com", When: midWindow, Content: taskContent("done")},
			},
		},
	}

	h := NewTaskHistory("tasks", mock)
	h.now = func() time.Time { return now }

	if err := h.Build(); err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// both tasks start as active baseline
	if h.baseActive != 2 {
		t.Errorf("baseActive = %d, want 2", h.baseActive)
	}

	points := h.Burndown()
	if len(points) == 0 {
		t.Fatal("Burndown() returned no points")
	}

	// first point should be 2 (both active), last should be 1 (one completed)
	if points[0].Remaining != 2 {
		t.Errorf("first point Remaining = %d, want 2", points[0].Remaining)
	}
	last := points[len(points)-1]
	if last.Remaining != 1 {
		t.Errorf("last point Remaining = %d, want 1", last.Remaining)
	}
}

func TestBurndown_NoVersions(t *testing.T) {
	mock := &mockGitOps{
		allFileVersions: map[string][]git.FileVersion{},
	}

	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	h := NewTaskHistory("tasks", mock)
	h.now = func() time.Time { return now }

	if err := h.Build(); err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	points := h.Burndown()
	for i, p := range points {
		if p.Remaining != 0 {
			t.Errorf("point[%d] Remaining = %d, want 0", i, p.Remaining)
			break
		}
	}
}
