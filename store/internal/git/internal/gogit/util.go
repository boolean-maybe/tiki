package gogit

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
)

// Util provides Git operations using go-git library
type Util struct {
	repoPath string
	repo     *git.Repository

	currentUserOnce  sync.Once
	currentUserName  string
	currentUserEmail string
	currentUserErr   error

	cachedUsers []string
}

// NewUtil creates a new go-git based Util instance
func NewUtil(repoPath string) (*Util, error) {
	if repoPath == "" {
		var err error
		repoPath, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	repo, err := git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository at %s: %w", repoPath, err)
	}

	// resolve the true worktree root — DetectDotGit may have walked up from
	// a subdirectory, so repoPath might not be the repo root
	rootPath := repoPath
	if wt, err := repo.Worktree(); err == nil {
		if cr, ok := wt.Filesystem.(interface{ Root() string }); ok {
			rootPath = cr.Root()
		}
	}

	return &Util{
		repoPath: rootPath,
		repo:     repo,
	}, nil
}

// CurrentBranch returns the name of the currently active branch
func (g *Util) CurrentBranch() (string, error) {
	head, err := g.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get repository HEAD: %w", err)
	}

	if head.Name().IsBranch() {
		return head.Name().Short(), nil
	}
	return head.Hash().String()[:7], nil
}

func (g *Util) toRelative(path string) (string, error) {
	if filepath.IsAbs(path) {
		relPath, err := filepath.Rel(g.repoPath, path)
		if err != nil {
			return "", fmt.Errorf("failed to convert path %s to relative: %w", path, err)
		}
		return relPath, nil
	}
	return path, nil
}

// toRelativePattern converts an absolute pathspec pattern to one relative to
// the repo root, so it can match against go-git's repo-relative file paths.
func (g *Util) toRelativePattern(pattern string) string {
	if !filepath.IsAbs(pattern) {
		return filepath.ToSlash(pattern)
	}
	rel, err := filepath.Rel(g.repoPath, pattern)
	if err != nil {
		return filepath.ToSlash(pattern)
	}
	return filepath.ToSlash(rel)
}

func (g *Util) loadCurrentUser() (name string, email string, err error) {
	cfg, err := g.repo.Config()
	if err != nil {
		return "", "", fmt.Errorf("failed to get git config: %w", err)
	}

	name = cfg.User.Name
	email = cfg.User.Email

	if name == "" || email == "" {
		globalCfg, err := config.LoadConfig(config.GlobalScope)
		if err == nil {
			if name == "" {
				name = globalCfg.User.Name
			}
			if email == "" {
				email = globalCfg.User.Email
			}
		}
	}

	if name == "" && email == "" {
		return "", "", fmt.Errorf("git user not configured (user.name and user.email are empty)")
	}

	return name, email, nil
}
