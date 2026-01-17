package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GitUtil provides Git operations using go-git library
type GitUtil struct {
	repoPath string
	repo     *git.Repository
}

// NewGitUtilWithGoGit creates a new GitUtil instance using go-git
func NewGitUtilWithGoGit(repoPath string) (*GitUtil, error) {
	if repoPath == "" {
		var err error
		repoPath, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository at %s: %w", repoPath, err)
	}

	return &GitUtil{
		repoPath: repoPath,
		repo:     repo,
	}, nil
}

// Add stages files to the git index (git add)
func (g *GitUtil) Add(paths ...string) error {
	if len(paths) == 0 {
		return errors.New("no paths provided")
	}

	worktree, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	for _, path := range paths {
		relPath := path
		if filepath.IsAbs(path) {
			var err error
			relPath, err = filepath.Rel(g.repoPath, path)
			if err != nil {
				return fmt.Errorf("failed to convert path %s to relative: %w", path, err)
			}
		}

		if _, err := worktree.Add(relPath); err != nil {
			return fmt.Errorf("failed to add %s: %w", relPath, err)
		}
	}

	return nil
}

// Remove removes files from the git index and working directory (git rm)
func (g *GitUtil) Remove(paths ...string) error {
	if len(paths) == 0 {
		return errors.New("no paths provided")
	}

	worktree, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	for _, path := range paths {
		relPath := path
		if filepath.IsAbs(path) {
			var err error
			relPath, err = filepath.Rel(g.repoPath, path)
			if err != nil {
				return fmt.Errorf("failed to convert path %s to relative: %w", path, err)
			}
		}

		if _, err := worktree.Remove(relPath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", relPath, err)
		}
	}

	return nil
}

// CurrentUser returns the current git user's name and email
func (g *GitUtil) CurrentUser() (name string, email string, err error) {
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
		return "", "", errors.New("git user not configured (user.name and user.email are empty)")
	}

	return name, email, nil
}

// Author returns information about who created a file
func (g *GitUtil) Author(filePath string) (*AuthorInfo, error) {
	relPath := filePath
	if filepath.IsAbs(filePath) {
		var err error
		relPath, err = filepath.Rel(g.repoPath, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to convert path %s to relative: %w", filePath, err)
		}
	}

	commitIter, err := g.repo.Log(&git.LogOptions{
		FileName: &relPath,
		Order:    git.LogOrderCommitterTime,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get git log for %s: %w", relPath, err)
	}
	defer commitIter.Close()

	var firstCommit *object.Commit
	if err := commitIter.ForEach(func(c *object.Commit) error {
		firstCommit = c
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	if firstCommit == nil {
		return nil, fmt.Errorf("no commits found for file %s", relPath)
	}

	return &AuthorInfo{
		Name:       firstCommit.Author.Name,
		Email:      firstCommit.Author.Email,
		Date:       firstCommit.Author.When,
		CommitHash: firstCommit.Hash.String(),
		Message:    firstCommit.Message,
	}, nil
}

// CurrentBranch returns the name of the currently active branch
func (g *GitUtil) CurrentBranch() (string, error) {
	head, err := g.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get repository HEAD: %w", err)
	}

	if head.Name().IsBranch() {
		return head.Name().Short(), nil
	}
	return head.Hash().String()[:7], nil
}

// FileVersionsSince returns file contents for commits since the provided time.
// if includePrior is true, the most recent commit before the given time is included as the first version.
func (g *GitUtil) FileVersionsSince(filePath string, since time.Time, includePrior bool) ([]FileVersion, error) {
	relPath := filePath
	if filepath.IsAbs(filePath) {
		var err error
		relPath, err = filepath.Rel(g.repoPath, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to convert path %s to relative: %w", filePath, err)
		}
	}
	relPath = filepath.ToSlash(relPath)

	iter, err := g.repo.Log(&git.LogOptions{FileName: &relPath})
	if err != nil {
		return nil, fmt.Errorf("failed to get git log for %s: %w", relPath, err)
	}
	defer iter.Close()

	var prior *FileVersion
	var sinceVersions []FileVersion

	err = iter.ForEach(func(c *object.Commit) error {
		version, err := buildVersionFromCommit(c, relPath)
		if err != nil {
			if errors.Is(err, object.ErrFileNotFound) {
				return nil
			}
			return err
		}

		if c.Author.When.Before(since) {
			if includePrior && prior == nil {
				prior = version
			}
			return nil
		}

		sinceVersions = append(sinceVersions, *version)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate git log for %s: %w", relPath, err)
	}

	// commits from go-git are newest-first; reverse to chronological order
	for i, j := 0, len(sinceVersions)-1; i < j; i, j = i+1, j-1 {
		sinceVersions[i], sinceVersions[j] = sinceVersions[j], sinceVersions[i]
	}

	var versions []FileVersion
	if includePrior && prior != nil {
		versions = append(versions, *prior)
	}
	versions = append(versions, sinceVersions...)

	return versions, nil
}

func buildVersionFromCommit(c *object.Commit, relPath string) (*FileVersion, error) {
	file, err := c.File(relPath)
	if err != nil {
		return nil, err
	}

	content, err := file.Contents()
	if err != nil {
		return nil, fmt.Errorf("failed to read %s at %s: %w", relPath, c.Hash.String(), err)
	}

	return &FileVersion{
		Hash:    c.Hash.String(),
		Author:  c.Author.Name,
		Email:   c.Author.Email,
		When:    c.Author.When,
		Content: content,
	}, nil
}
