package gogit

import "testing"

// TestMatchesPattern_DirectoryPathspecRecurses locks in the Phase 2 contract:
// passing a bare directory (no glob chars) must match every file under it
// recursively, mirroring shell git's `git log -- .doc/` behavior. The old
// implementation delegated everything to path.Match, which does not descend
// into subdirectories and silently returned no matches for nested docs.
func TestMatchesPattern_DirectoryPathspecRecurses(t *testing.T) {
	cases := []struct {
		name     string
		pattern  string
		filePath string
		want     bool
	}{
		// Directory pattern matches files at any depth beneath it.
		{"root-flat", ".doc", ".doc/flat.md", true},
		{"root-nested", ".doc", ".doc/sub/nested.md", true},
		{"root-deep", ".doc", ".doc/a/b/c.md", true},
		{"trailing-slash-is-equivalent", ".doc/", ".doc/x.md", true},
		{"dir-itself-matches-dir-entry", ".doc", ".doc", true},
		// Sibling paths must NOT match the directory pattern — regression
		// guard against accidentally treating the pattern as a prefix
		// without the trailing slash.
		{"sibling-prefix", ".doc", ".doc-other/x.md", false},
		{"unrelated", ".doc", "src/foo.md", false},

		// Glob patterns still work the old way — flat only, no descent.
		{"glob-flat-match", ".doc/tiki/*.md", ".doc/tiki/a.md", true},
		{"glob-no-recurse", ".doc/tiki/*.md", ".doc/tiki/sub/a.md", false},
		{"glob-wrong-dir", ".doc/tiki/*.md", ".doc/other/a.md", false},

		// Exact-path pattern.
		{"exact", ".doc/a.md", ".doc/a.md", true},
		{"exact-no-prefix-match", ".doc/a.md", ".doc/a.md.bak", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesPattern(tc.filePath, tc.pattern)
			if got != tc.want {
				t.Errorf("matchesPattern(filePath=%q, pattern=%q) = %v, want %v",
					tc.filePath, tc.pattern, got, tc.want)
			}
		})
	}
}
