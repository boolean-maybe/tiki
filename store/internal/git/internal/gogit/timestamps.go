package gogit

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// LastCommitTime returns the timestamp of the most recent commit that modified the file.
func (g *Util) LastCommitTime(filePath string) (time.Time, error) {
	relPath, err := g.toRelative(filePath)
	if err != nil {
		return time.Time{}, err
	}
	relPath = filepath.ToSlash(relPath)

	commitIter, err := g.repo.Log(&git.LogOptions{
		FileName: &relPath,
		Order:    git.LogOrderCommitterTime,
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get git log for %s: %w", relPath, err)
	}
	defer commitIter.Close()

	c, err := commitIter.Next()
	if err != nil {
		return time.Time{}, fmt.Errorf("no commits found for file %s: %w", relPath, err)
	}

	return c.Author.When, nil
}

// AllLastCommitTimes returns the last commit timestamp for all files matching dirPattern.
// Stores the first occurrence per file (newest = first visited in reverse-chronological order).
func (g *Util) AllLastCommitTimes(dirPattern string) (map[string]time.Time, error) {
	dirPattern = g.toRelativePattern(dirPattern)

	pathFilter := func(path string) bool {
		return matchesPattern(path, dirPattern)
	}

	commitIter, err := g.repo.Log(&git.LogOptions{
		All:        true,
		Order:      git.LogOrderCommitterTime,
		PathFilter: pathFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get git log: %w", err)
	}
	defer commitIter.Close()

	result := make(map[string]time.Time)
	err = commitIter.ForEach(func(c *object.Commit) error {
		tree, err := c.Tree()
		if err != nil {
			return err
		}

		pTree, err := parentTree(c)
		if err != nil {
			return err
		}

		changes, err := diffTrees(pTree, tree)
		if err != nil {
			return err
		}

		for _, change := range changes {
			var name string
			if change.To.Name != "" {
				name = change.To.Name
			} else {
				name = change.From.Name
			}
			if !matchesPattern(name, dirPattern) {
				continue
			}
			if _, exists := result[name]; !exists {
				result[name] = c.Author.When
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return result, nil
}
