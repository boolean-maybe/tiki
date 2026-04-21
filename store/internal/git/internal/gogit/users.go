package gogit

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/boolean-maybe/tiki/store/internal/git/internal/types"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

// CurrentUser returns the current git user's name and email.
// Results are cached after the first call.
func (g *Util) CurrentUser() (name string, email string, err error) {
	g.currentUserOnce.Do(func() {
		g.currentUserName, g.currentUserEmail, g.currentUserErr = g.loadCurrentUser()
	})
	return g.currentUserName, g.currentUserEmail, g.currentUserErr
}

// Author returns information about who created a file.
// Diffs each commit's tree against its parent to find the Insert action,
// matching shell's --diff-filter=A semantics.
func (g *Util) Author(filePath string) (*types.AuthorInfo, error) {
	relPath, err := g.toRelative(filePath)
	if err != nil {
		return nil, err
	}
	relPath = filepath.ToSlash(relPath)

	commitIter, err := g.repo.Log(&git.LogOptions{
		FileName: &relPath,
		Order:    git.LogOrderCommitterTime,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get git log for %s: %w", relPath, err)
	}
	defer commitIter.Close()

	var result *types.AuthorInfo
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
			action, err := change.Action()
			if err != nil {
				continue
			}
			if action != merkletrie.Insert {
				continue
			}
			if change.To.Name == relPath {
				result = &types.AuthorInfo{
					Name:       c.Author.Name,
					Email:      c.Author.Email,
					Date:       c.Author.When,
					CommitHash: c.Hash.String(),
					Message:    c.Message,
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	if result == nil {
		return nil, fmt.Errorf("no commits found for file %s", relPath)
	}

	return result, nil
}

// AllAuthors returns author information for all files matching dirPattern.
// For each file, returns the author of the commit that first added it
// (matching shell's --diff-filter=A --reverse semantics).
func (g *Util) AllAuthors(dirPattern string) (map[string]*types.AuthorInfo, error) {
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

	result := make(map[string]*types.AuthorInfo)
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
			action, err := change.Action()
			if err != nil {
				continue
			}
			if action != merkletrie.Insert {
				continue
			}
			name := change.To.Name
			if !matchesPattern(name, dirPattern) {
				continue
			}
			result[name] = &types.AuthorInfo{
				Name:       c.Author.Name,
				Email:      c.Author.Email,
				Date:       c.Author.When,
				CommitHash: c.Hash.String(),
				Message:    c.Message,
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return result, nil
}

// AllUsers returns a deduplicated list of all users who have made commits.
// Preserves first-seen traversal order from repo.Log (matching shell behavior).
// Results are cached after the first call.
func (g *Util) AllUsers() ([]string, error) {
	if g.cachedUsers != nil {
		return g.cachedUsers, nil
	}

	commitIter, err := g.repo.Log(&git.LogOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to get git log for all users: %w", err)
	}
	defer commitIter.Close()

	seen := make(map[string]bool)
	var users []string
	err = commitIter.ForEach(func(c *object.Commit) error {
		name := c.Author.Name
		if name != "" && !seen[name] {
			seen[name] = true
			users = append(users, name)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	g.cachedUsers = users
	return users, nil
}

// diffTrees diffs two trees (nil-safe for root commits where parent tree is nil).
func diffTrees(from, to *object.Tree) (object.Changes, error) {
	return object.DiffTreeWithOptions(context.Background(), from, to, &object.DiffTreeOptions{DetectRenames: false})
}
