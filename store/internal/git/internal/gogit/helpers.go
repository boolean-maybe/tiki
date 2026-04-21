package gogit

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// matchesPattern checks whether filePath matches a pathspec-like pattern.
// Uses path.Match (not filepath.Match) so that '/' is always the separator
// and '*' never crosses directory boundaries, even on Windows.
func matchesPattern(filePath, pattern string) bool {
	filePath = filepath.ToSlash(filePath)
	pattern = filepath.ToSlash(pattern)

	matched, err := path.Match(pattern, filePath)
	if err != nil {
		return false
	}
	return matched
}

// parentTree returns the tree of the first parent commit, or nil for root commits.
func parentTree(c *object.Commit) (*object.Tree, error) {
	if c.NumParents() == 0 {
		return nil, nil
	}
	parent, err := c.Parent(0)
	if err != nil {
		return nil, err
	}
	return parent.Tree()
}

// statusLineTouched returns true when a file patch contains added or removed
// lines matching "^status:", mirroring shell's `git log -G^status:` behavior.
func statusLineTouched(fp diff.FilePatch) bool {
	for _, chunk := range fp.Chunks() {
		if chunk.Type() == diff.Equal {
			continue
		}
		for _, line := range strings.Split(chunk.Content(), "\n") {
			if strings.HasPrefix(line, "status:") {
				return true
			}
		}
	}
	return false
}
