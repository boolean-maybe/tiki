package gogit

import (
	"errors"
	"fmt"
	"path/filepath"
)

// Add stages files to the git index (git add)
func (g *Util) Add(paths ...string) error {
	if len(paths) == 0 {
		return errors.New("no paths provided")
	}

	worktree, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	for _, path := range paths {
		relPath, err := g.toRelative(path)
		if err != nil {
			return err
		}

		if _, err := worktree.Add(filepath.ToSlash(relPath)); err != nil {
			return fmt.Errorf("failed to add %s: %w", relPath, err)
		}
	}

	return nil
}

// Remove removes files from the git index and working directory (git rm)
func (g *Util) Remove(paths ...string) error {
	if len(paths) == 0 {
		return errors.New("no paths provided")
	}

	worktree, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	for _, path := range paths {
		relPath, err := g.toRelative(path)
		if err != nil {
			return err
		}

		if _, err := worktree.Remove(filepath.ToSlash(relPath)); err != nil {
			return fmt.Errorf("failed to remove %s: %w", relPath, err)
		}
	}

	return nil
}
