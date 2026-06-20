package document

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// WalkDocuments walks root recursively and returns every file path that
// should be treated as a managed markdown document. It is the single
// source of truth for "which files on disk count as documents", shared by
// the runtime store loader and any future maintenance tooling.
//
// Exclusion rules (see per-helper godoc for specifics):
//
//   - non-`.md` files are ignored;
//   - hidden subdirectories (`.git`, `.idea`, `.obsidian`, ...) are pruned;
//   - paths matched by `.gitignore` or `.tikiignore` at root are excluded.
//
// Returned paths are sorted so callers that iterate them get deterministic
// ordering for diagnostics and duplicate-id reporting.
func WalkDocuments(root string) ([]string, error) {
	ignore, err := LoadIgnoreMatcher(root)
	if err != nil {
		return nil, err
	}
	var paths []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			if IsSkippableSubdir(path, root, d.Name()) {
				return filepath.SkipDir
			}
			if path != root && ignore.Match(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		if ignore.Match(rel, false) {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Strings(paths)
	return paths, nil
}

// IsSkippableSubdir reports whether a directory encountered during a walk
// should be pruned. The root itself is never skipped. Hidden names (names
// starting with ".") are skipped — they are editor/VCS metadata (`.git`,
// `.obsidian`, `.idea`, ...).
func IsSkippableSubdir(path, root, name string) bool {
	if path == root {
		return false
	}
	return strings.HasPrefix(name, ".")
}
