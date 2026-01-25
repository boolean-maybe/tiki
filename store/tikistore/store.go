package tikistore

// TikiStore is a file-based Store implementation that persists tasks as markdown files.

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/internal/git"
	taskpkg "github.com/boolean-maybe/tiki/task"
)

// ErrConflict indicates a task was modified externally since it was loaded
var ErrConflict = errors.New("task was modified externally")

// TikiStore stores tasks as markdown files with YAML frontmatter.
// Each task is a separate .md file in the configured directory.
// Author and dates are retrieved from git (not stored in file).
type TikiStore struct {
	mu             sync.RWMutex
	dir            string // directory containing task files
	tasks          map[string]*taskpkg.Task
	listeners      map[int]store.ChangeListener
	nextListenerID int
	gitUtil        git.GitOps         // git utility for auto-staging modified files
	taskHistory    *store.TaskHistory // history for burndown computation
}

// taskFrontmatter represents the YAML frontmatter in task files
type taskFrontmatter struct {
	Title    string                `yaml:"title"`
	Type     string                `yaml:"type"`
	Status   string                `yaml:"status"`
	Tags     taskpkg.TagsValue     `yaml:"tags,omitempty"`
	Assignee string                `yaml:"assignee,omitempty"`
	Priority taskpkg.PriorityValue `yaml:"priority,omitempty"`
	Points   int                   `yaml:"points,omitempty"`
}

// NewTikiStore creates a new TikiStore.
// dir: directory containing task markdown files
func NewTikiStore(dir string) (*TikiStore, error) {
	slog.Debug("creating new TikiStore", "dir", dir)
	s := &TikiStore{
		dir:            dir,
		tasks:          make(map[string]*taskpkg.Task),
		listeners:      make(map[int]store.ChangeListener),
		nextListenerID: 1, // Start at 1 to avoid conflict with zero-value sentinel
	}

	// Initialize git utility (best effort - don't fail if git is not available)
	gitUtil, err := git.NewGitOps("")
	if err == nil {
		s.gitUtil = gitUtil
	} else {
		slog.Debug("git utility not initialized", "error", err)
	}

	s.mu.Lock()
	if err := s.loadLocked(); err != nil {
		s.mu.Unlock()
		slog.Error("failed to load tasks during store initialization", "dir", dir, "error", err)
		return nil, fmt.Errorf("loading tasks: %w", err)
	}
	s.mu.Unlock()

	slog.Info("tikiStore initialized", "dir", dir, "num_tasks", len(s.tasks))
	return s, nil
}

// SetTaskHistory sets the task history instance (called after background build completes)
func (s *TikiStore) SetTaskHistory(history *store.TaskHistory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskHistory = history
}

// IsGitRepo checks if the given path is a git repository (for pre-flight checks)
func IsGitRepo(path string) bool {
	_, err := git.NewGitShellUtil(path)
	return err == nil
}

// GetCurrentUser returns the current git user name and email
func (s *TikiStore) GetCurrentUser() (name string, email string, err error) {
	// No lock needed - gitUtil is immutable after initialization
	if s.gitUtil == nil {
		return "n/a", "", fmt.Errorf("git utility not available")
	}

	return s.gitUtil.CurrentUser()
}

// GetStats returns statistics for the header (user, branch)
func (s *TikiStore) GetStats() []store.Stat {
	// No lock needed - gitUtil is immutable after initialization
	stats := make([]store.Stat, 0, 2)

	// User stat
	user := "n/a"
	if s.gitUtil != nil {
		if name, _, err := s.gitUtil.CurrentUser(); err == nil && name != "" {
			user = name
		}
	}
	stats = append(stats, store.Stat{Name: "User", Value: user, Order: 3})

	// Branch stat
	branch := "n/a"
	if s.gitUtil != nil {
		if b, err := s.gitUtil.CurrentBranch(); err == nil {
			branch = b
		}
	}
	stats = append(stats, store.Stat{Name: "Branch", Value: branch, Order: 4})

	return stats
}

// GetGitOps returns the git operations instance (needed for history construction)
func (s *TikiStore) GetGitOps() git.GitOps {
	// No lock needed - gitUtil is immutable after initialization
	return s.gitUtil
}

// GetBurndown returns the burndown chart data
func (s *TikiStore) GetBurndown() []store.BurndownPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.taskHistory == nil {
		return nil
	}

	return s.taskHistory.Burndown()
}

// GetAllUsers returns list of all git users for assignee selection
func (s *TikiStore) GetAllUsers() ([]string, error) {
	// No lock needed - gitUtil is immutable after initialization
	if s.gitUtil == nil {
		return nil, fmt.Errorf("git utility not available")
	}

	return s.gitUtil.AllUsers()
}

// ensure TikiStore implements Store
var _ store.Store = (*TikiStore)(nil)
