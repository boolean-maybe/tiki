package tikistore

// TikiStore is a file-based Store implementation that persists tikis as markdown files.

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/internal/git"
	"github.com/boolean-maybe/tiki/tiki"
)

// ErrConflict indicates a tiki was modified externally since it was loaded
var ErrConflict = errors.New("tiki was modified externally")

func normalizeTikiID(id string) string {
	return strings.ToUpper(strings.TrimSpace(id))
}

// TikiStore stores tikis as markdown files with YAML frontmatter.
// Each tiki is a separate .md file in the configured directory.
// Commit dates are retrieved from git (not stored in file); the current
// Tiki identity comes from configured identity → git → OS user.
type TikiStore struct {
	mu             sync.RWMutex
	dir            string // directory containing tiki files
	tikis          map[string]*tiki.Tiki
	listeners      map[int]store.ChangeListener
	nextListenerID int
	gitUtil        git.GitOps        // git utility for auto-staging modified files
	upgrader       *LegacyUpgrader   // normalizes legacy field values on load
	identity       *identityResolver // resolves current Tiki identity (config→git→OS)
	diagnostics    *LoadDiagnostics  // rejections from the most recent load/reload cycle
}

// NewTikiStore creates a new TikiStore.
// dir: directory containing tiki markdown files
func NewTikiStore(dir string) (*TikiStore, error) {
	slog.Debug("creating new TikiStore", "dir", dir)
	s := &TikiStore{
		dir:            dir,
		tikis:          make(map[string]*tiki.Tiki),
		listeners:      make(map[int]store.ChangeListener),
		nextListenerID: 1, // Start at 1 to avoid conflict with zero-value sentinel
		upgrader:       &LegacyUpgrader{},
		diagnostics:    newLoadDiagnostics(),
	}

	// git integration is automatic and read-only: when cwd is a repo the store
	// reads history/authors; when it is not, the git methods fail gracefully and
	// the store falls back to mtime/config identity. There is no enable flag.
	gitUtil, err := git.NewGitOps("")
	if err == nil {
		s.gitUtil = gitUtil
	} else {
		slog.Debug("git utility not initialized (not a repo or unavailable)", "error", err)
	}
	s.identity = newIdentityResolver(s.gitUtil)

	s.mu.Lock()
	if err := s.loadLocked(); err != nil {
		s.mu.Unlock()
		slog.Error("failed to load tikis during store initialization", "dir", dir, "error", err)
		return nil, fmt.Errorf("loading tikis: %w", err)
	}
	s.mu.Unlock()

	slog.Info("tikiStore initialized", "dir", dir, "num_tikis", len(s.tikis))
	return s, nil
}

// LoadDiagnostics returns the rejections accumulated during the most recent
// load/reload cycle. Nil-safe: callers can use HasIssues() / Summary() /
// Rejections() directly on the returned value.
func (s *TikiStore) LoadDiagnostics() *LoadDiagnostics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.diagnostics
}

// IsGitRepo checks if the given path is a git repository (for pre-flight checks)
func IsGitRepo(path string) bool {
	return git.IsRepo(path)
}

// GetCurrentUser returns the current Tiki identity (name and email).
// Resolution order: configured `identity.*` → git user → OS user.
// Returns empty strings (no error) when no source resolves, so that callers
// like resolveUserFunc in runner.go can surface a clean "unavailable" error.
func (s *TikiStore) GetCurrentUser() (name string, email string, err error) {
	if s.identity == nil {
		return "", "", nil
	}
	return s.identity.currentUser()
}

// GetStats returns statistics for the header (user, branch).
// User is sourced from the shared identity projection helper so the TUI
// header agrees with ruki `user()`, plugin-action executors, trigger setup,
// and pipe-create trigger setup. Branch stays git-only.
func (s *TikiStore) GetStats() []store.Stat {
	// No lock needed - gitUtil and identity are immutable after initialization
	stats := make([]store.Stat, 0, 2)

	// User stat — delegate to the shared helper so the UI cannot drift from
	// every other consumer of the identity projection
	user := "n/a"
	if display, err := store.CurrentUserDisplay(s); err == nil && display != "" {
		user = display
	}
	stats = append(stats, store.Stat{Name: "User", Value: user, Order: 3})

	// Branch stat — git-only; no meaningful equivalent in no-git mode
	branch := "n/a"
	if s.gitUtil != nil {
		if b, err := s.gitUtil.CurrentBranch(); err == nil {
			branch = b
		}
	}
	stats = append(stats, store.Stat{Name: "Branch", Value: branch, Order: 4})

	return stats
}

// GetGitOps returns the git operations instance.
func (s *TikiStore) GetGitOps() git.GitOps {
	// No lock needed - gitUtil is immutable after initialization
	return s.gitUtil
}

// GetAllUsers returns candidate identities for assignee selection.
// In git mode, merges the configured identity with git's commit-author list.
// In no-git mode, returns the resolved identity (configured or OS user).
func (s *TikiStore) GetAllUsers() ([]string, error) {
	// No lock needed - identity is immutable after initialization
	if s.identity == nil {
		return nil, nil
	}
	return s.identity.allUsers()
}

// ensure TikiStore implements Store
var _ store.Store = (*TikiStore)(nil)
