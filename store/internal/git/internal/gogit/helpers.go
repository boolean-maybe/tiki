package gogit

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// matchesPattern checks whether filePath matches a pathspec-like pattern
// in the style of `git log -- <pattern>`. Two shapes are supported:
//
//   - Glob pattern (contains '*', '?' or '['): evaluated with path.Match so
//     '/' is the separator and '*' never crosses directory boundaries on
//     any OS. Matches flat-directory shapes like `.doc/tiki/*.md`.
//   - Directory pattern (no glob chars): treated as "everything under this
//     directory, recursively." Matches shell git's behavior where passing
//     a bare directory as pathspec walks the whole subtree. Phase 2 passes
//     `.doc/` here to scan both flat and nested document layouts.
//
// An exact filepath match still works either way.
func matchesPattern(filePath, pattern string) bool {
	filePath = filepath.ToSlash(filePath)
	pattern = filepath.ToSlash(pattern)

	if hasGlobChars(pattern) {
		matched, err := path.Match(pattern, filePath)
		if err != nil {
			return false
		}
		return matched
	}

	// Directory pathspec: match the directory itself and anything beneath
	// it. Strip a trailing slash from the pattern so `.doc/` and `.doc`
	// behave identically, matching git's own pathspec rules.
	pattern = strings.TrimSuffix(pattern, "/")
	if filePath == pattern {
		return true
	}
	return strings.HasPrefix(filePath, pattern+"/")
}

// hasGlobChars reports whether pattern contains any of the standard shell
// glob metacharacters supported by path.Match.
func hasGlobChars(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
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
