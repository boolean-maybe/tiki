package gogit

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/boolean-maybe/tiki/store/internal/git/internal/types"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// FileVersionsSince returns file contents for commits since the provided time.
// If includePrior is true, the most recent commit before the given time is included as the first version.
func (g *Util) FileVersionsSince(filePath string, since time.Time, includePrior bool) ([]types.FileVersion, error) {
	relPath, err := g.toRelative(filePath)
	if err != nil {
		return nil, err
	}
	relPath = filepath.ToSlash(relPath)

	iter, err := g.repo.Log(&git.LogOptions{FileName: &relPath})
	if err != nil {
		return nil, fmt.Errorf("failed to get git log for %s: %w", relPath, err)
	}
	defer iter.Close()

	var prior *types.FileVersion
	var sinceVersions []types.FileVersion

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

	var versions []types.FileVersion
	if includePrior && prior != nil {
		versions = append(versions, *prior)
	}
	versions = append(versions, sinceVersions...)

	return versions, nil
}

// AllFileVersionsSince returns file versions for all files matching dirPattern since the given time.
// Only includes commits where a "status:" line was added or removed (matching shell's -G^status: behavior).
func (g *Util) AllFileVersionsSince(dirPattern string, since time.Time, includePrior bool) (map[string][]types.FileVersion, error) {
	dirPattern = g.toRelativePattern(dirPattern)

	pathFilter := func(path string) bool {
		return matchesPattern(path, dirPattern)
	}

	result := make(map[string][]types.FileVersion)

	type fileCommitEntry struct {
		commitHash string
		author     string
		email      string
		when       time.Time
	}
	fileCommits := make(map[string][]fileCommitEntry)

	// since window: walk commits after `since`
	sinceCommitIter, err := g.repo.Log(&git.LogOptions{
		All:        true,
		Since:      &since,
		PathFilter: pathFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get git log for %s: %w", dirPattern, err)
	}

	err = sinceCommitIter.ForEach(func(c *object.Commit) error {
		touched, err := statusChangedFiles(c, dirPattern)
		if err != nil {
			return err
		}
		for _, name := range touched {
			fileCommits[name] = append(fileCommits[name], fileCommitEntry{
				commitHash: c.Hash.String(),
				author:     c.Author.Name,
				email:      c.Author.Email,
				when:       c.Author.When,
			})
		}
		return nil
	})
	sinceCommitIter.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to iterate since-window commits: %w", err)
	}

	// prior pass: if includePrior, find most recent commit before `since` per file
	priorCommits := make(map[string]fileCommitEntry)
	if includePrior {
		until := since
		priorCommitIter, err := g.repo.Log(&git.LogOptions{
			All:        true,
			Until:      &until,
			PathFilter: pathFilter,
		})
		if err == nil {
			_ = priorCommitIter.ForEach(func(c *object.Commit) error {
				touched, err := statusChangedFiles(c, dirPattern)
				if err != nil {
					return err
				}
				for _, name := range touched {
					if _, exists := priorCommits[name]; !exists {
						priorCommits[name] = fileCommitEntry{
							commitHash: c.Hash.String(),
							author:     c.Author.Name,
							email:      c.Author.Email,
							when:       c.Author.When,
						}
					}
				}
				return nil
			})
			priorCommitIter.Close()
		}
	}

	// prepend prior commits
	for file, prior := range priorCommits {
		fileCommits[file] = append([]fileCommitEntry{prior}, fileCommits[file]...)
	}

	// fetch blob content for each commit
	for file, entries := range fileCommits {
		for _, entry := range entries {
			content, err := g.readBlobAt(entry.commitHash, file)
			if err != nil {
				continue
			}
			result[file] = append(result[file], types.FileVersion{
				Hash:    entry.commitHash,
				Author:  entry.author,
				Email:   entry.email,
				When:    entry.when,
				Content: content,
			})
		}
	}

	return result, nil
}

// statusChangedFiles returns the names of files matching dirPattern where a
// "status:" line was touched in the given commit's diff against its parent.
func statusChangedFiles(c *object.Commit, dirPattern string) ([]string, error) {
	tree, err := c.Tree()
	if err != nil {
		return nil, err
	}

	pTree, err := parentTree(c)
	if err != nil {
		return nil, err
	}

	changes, err := diffTrees(pTree, tree)
	if err != nil {
		return nil, err
	}

	patch, err := changes.Patch()
	if err != nil {
		return nil, err
	}

	var matched []string
	for _, fp := range patch.FilePatches() {
		_, to := fp.Files()
		var name string
		if to != nil {
			name = to.Path()
		} else {
			from, _ := fp.Files()
			if from != nil {
				name = from.Path()
			}
		}
		if name == "" {
			continue
		}
		if !matchesPattern(name, dirPattern) {
			continue
		}
		if statusLineTouched(fp) {
			matched = append(matched, name)
		}
	}
	return matched, nil
}

func (g *Util) readBlobAt(commitHash, filePath string) (string, error) {
	c, err := g.repo.CommitObject(plumbing.NewHash(commitHash))
	if err != nil {
		return "", err
	}
	file, err := c.File(filePath)
	if err != nil {
		return "", err
	}
	return file.Contents()
}

func buildVersionFromCommit(c *object.Commit, relPath string) (*types.FileVersion, error) {
	file, err := c.File(relPath)
	if err != nil {
		return nil, err
	}

	content, err := file.Contents()
	if err != nil {
		return nil, fmt.Errorf("failed to read %s at %s: %w", relPath, c.Hash.String(), err)
	}

	return &types.FileVersion{
		Hash:    c.Hash.String(),
		Author:  c.Author.Name,
		Email:   c.Author.Email,
		When:    c.Author.When,
		Content: content,
	}, nil
}
