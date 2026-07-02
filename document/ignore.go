package document

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// IgnoreMatcher matches paths against the combined patterns from .gitignore
// and .tikiignore at the scan root. .tikiignore patterns add to .gitignore.
type IgnoreMatcher struct {
	m gitignore.Matcher
}

// LoadIgnoreMatcher reads .gitignore and .tikiignore at root and returns a
// matcher. Missing files are not an error — they contribute no patterns.
func LoadIgnoreMatcher(root string) (*IgnoreMatcher, error) {
	var patterns []gitignore.Pattern
	for _, name := range []string{".gitignore", ".tikiignore"} {
		ps, err := readPatternFile(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, ps...)
	}
	return &IgnoreMatcher{m: gitignore.NewMatcher(patterns)}, nil
}

// readPatternFile parses a gitignore-syntax file into patterns. A missing file
// yields no patterns and no error.
func readPatternFile(path string) ([]gitignore.Pattern, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []gitignore.Pattern
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, gitignore.ParsePattern(line, nil))
	}
	return out, nil
}

// Match reports whether relPath (slash-separated, relative to root) is ignored.
func (im *IgnoreMatcher) Match(relPath string, isDir bool) bool {
	if im == nil {
		return false
	}
	return im.m.Match(strings.Split(filepath.ToSlash(relPath), "/"), isDir)
}
