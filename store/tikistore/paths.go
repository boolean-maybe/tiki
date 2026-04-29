package tikistore

import (
	"sort"
)

// PathForID returns the on-disk file path of a loaded document, or the empty
// string when id is unknown to the store. This is the Phase 2 replacement
// for every `filepath.Join(taskDir, lower(id) + ".md")` style reconstruction
// in the codebase: the document's current path is authoritative, so a file
// that was renamed or moved into a subdirectory resolves to its real
// location instead of the id-derived default.
//
// The lookup uses the in-memory task map as the authoritative id→path index;
// a separate map would duplicate state. Callers should not mutate the
// returned string (it is already a value, not a reference).
func (s *TikiStore) PathForID(id string) string {
	normalized := normalizeTaskID(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if t, ok := s.tasks[normalized]; ok {
		return t.FilePath
	}
	return ""
}

// IDForPath returns the document id whose FilePath matches the given path,
// or the empty string when no loaded document lives there. Comparison is
// exact string equality: the store normalizes every FilePath through
// filepath.Abs at load time, so callers who obtained `path` the same way
// (e.g. via filepath.Abs of a user-supplied relative path) will match.
// Callers on case-insensitive filesystems who hand in a differently-cased
// path must normalize it themselves; a blanket case-fold here would
// conflate distinct files on case-sensitive systems like Linux.
//
// This is the Phase 2 replacement for filename-parsing callers; anywhere
// that used to derive an id from `filepath.Base(p)` now has a proper
// reverse lookup available.
func (s *TikiStore) IDForPath(path string) string {
	if path == "" {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for id, t := range s.tasks {
		if t.FilePath == path {
			return id
		}
	}
	return ""
}

// AllPaths returns a sorted snapshot of every loaded document's file path.
// Exposed mostly for tests and diagnostics that need to assert the store's
// current file-layout view without pulling task-level detail.
func (s *TikiStore) AllPaths() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.tasks))
	for _, t := range s.tasks {
		if t.FilePath != "" {
			out = append(out, t.FilePath)
		}
	}
	sort.Strings(out)
	return out
}
