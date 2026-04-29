package document

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// WalkDocuments walks root recursively and returns every file path that
// should be treated as a managed markdown document. It is the single
// source of truth for "which files on disk count as documents" and is
// shared by the runtime store loader and the `tiki repair` CLI so the
// two cannot drift: a file the store refuses to load must never be a
// file the repair command silently skips, and vice versa.
//
// Exclusion rules (see per-helper godoc for specifics):
//
//   - non-`.md` files are ignored;
//   - hidden subdirectories (`.git`, `.idea`, `.obsidian`, ...) are pruned;
//   - the reserved top-level project config files (`config.yaml`,
//     `workflow.yaml`, and defensive `.md` variants) are excluded.
//
// Returned paths are sorted so callers that iterate them get deterministic
// ordering for diagnostics and duplicate-id reporting.
func WalkDocuments(root string) ([]string, error) {
	var paths []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if IsSkippableSubdir(path, root, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		if IsProjectConfigFile(path, root, d.Name()) {
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

// reservedTopLevelDirs lists subdirectory names that live directly under
// the document root but are NOT document directories. The walker prunes
// them so assets, bundled workflow YAML, and similar content do not become
// phantom documents the store refuses to load.
//
// Match is by exact directory name AND only when the directory is a direct
// child of root — a user-authored `.doc/projects/assets/<ID>.md` is a
// regular document and must not be excluded.
var reservedTopLevelDirs = map[string]struct{}{
	"assets":    {}, // plan Phase 8 names .doc/assets/ as the shared-asset home
	"workflows": {}, // bundled workflow YAML samples live here
}

// IsSkippableSubdir reports whether a directory encountered during a walk
// should be pruned. The root itself is never skipped. Hidden names (names
// starting with ".") are skipped — they are editor/VCS metadata. The
// reserved top-level directories (`assets`, `workflows`) are skipped too,
// but only when they are direct children of root, so a user-authored
// subfolder that happens to share the name keeps working.
func IsSkippableSubdir(path, root, name string) bool {
	if path == root {
		return false
	}
	if strings.HasPrefix(name, ".") {
		return true
	}
	if _, reserved := reservedTopLevelDirs[name]; reserved {
		// Only prune when the reserved name is a direct child of root:
		// `.doc/assets/` is excluded, `.doc/projects/assets/` is not.
		if filepath.Dir(path) == root {
			return true
		}
	}
	return false
}

// IsProjectConfigFile reports whether path is one of the reserved top-level
// configuration files that live alongside documents under .doc/. The match
// is strict: only direct children of root with the exact names
// `config.yaml` or `workflow.yaml` are excluded; a nested file with the
// same name inside a subdirectory is treated as a normal document.
//
// Today these files use a `.yaml` extension and are already filtered out
// by the `.md` suffix check upstream, so the `.md` variants are defense
// in depth — if a project ever adds `.doc/config.md` or `.doc/workflow.md`
// the walker still refuses to treat them as documents.
func IsProjectConfigFile(path, root, name string) bool {
	if name != "config.yaml" && name != "workflow.yaml" &&
		name != "config.md" && name != "workflow.md" {
		return false
	}
	dir := filepath.Dir(path)
	return dir == root
}
