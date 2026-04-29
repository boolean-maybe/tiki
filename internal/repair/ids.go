package repair

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/boolean-maybe/tiki/document"
)

// Mode determines whether RepairIDs writes changes to disk.
type Mode int

const (
	// ModeCheck reports issues without modifying any files.
	ModeCheck Mode = iota
	// ModeFix writes changes: inserts missing ids, replaces legacy TIKI-
	// values with bare ids, and optionally regenerates duplicates.
	ModeFix
)

// Options configures a repair run.
type Options struct {
	// Dir is the directory containing .md files to inspect. Non-recursive;
	// Phase 2 will flip this to recursive as part of the .doc/**/*.md move.
	Dir string
	// Mode is ModeCheck or ModeFix.
	Mode Mode
	// RegenerateDuplicates, when true in ModeFix, assigns fresh bare ids to
	// all-but-one file in a duplicate set. When false, duplicates are
	// reported but not fixed so the user can decide which to keep.
	RegenerateDuplicates bool
}

// Report summarizes what RepairIDs found (and, in ModeFix, what it changed).
type Report struct {
	Scanned         int
	MissingID       []string // paths whose frontmatter lacked an id
	LegacyID        []string // paths with TIKI-XXXXXX ids
	InvalidID       []string // paths with an id that is neither bare nor legacy
	DuplicateIDs    map[string][]string
	FixedMissingID  []string
	FixedLegacyID   []string
	FixedDuplicates []string
	ParseErrors     map[string]error
}

// HasIssues reports whether the report contains any problem that requires user
// attention (in check mode) or that fix mode could not resolve.
func (r *Report) HasIssues() bool {
	if r == nil {
		return false
	}
	return len(r.MissingID) > 0 || len(r.LegacyID) > 0 || len(r.InvalidID) > 0 || len(r.DuplicateIDs) > 0 || len(r.ParseErrors) > 0
}

// RepairIDs scans opts.Dir for managed markdown files and reports, or fixes,
// id-related issues. The function is safe to call in ModeCheck: no writes.
// In ModeFix, changes are narrow textual edits to each affected file.
func RepairIDs(opts Options) (*Report, error) {
	if opts.Dir == "" {
		return nil, fmt.Errorf("repair: Dir is required")
	}
	rep := &Report{
		DuplicateIDs: map[string][]string{},
		ParseErrors:  map[string]error{},
	}

	entries, err := os.ReadDir(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", opts.Dir, err)
	}

	// pass 1 — classify. Collect ids so we can detect duplicates before
	// any write touches the filesystem.
	var results []*scanResultT
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		full := filepath.Join(opts.Dir, entry.Name())
		rep.Scanned++
		sr, err := classifyFile(full)
		if err != nil {
			rep.ParseErrors[full] = err
			continue
		}
		results = append(results, sr)
		switch {
		case !sr.hadID:
			rep.MissingID = append(rep.MissingID, full)
		case sr.wasLegacy:
			rep.LegacyID = append(rep.LegacyID, full)
		case sr.wasInvalid:
			rep.InvalidID = append(rep.InvalidID, full)
		}
	}

	// pass 2 — duplicate detection across successfully-parsed files.
	idToPaths := map[string][]string{}
	for _, sr := range results {
		if sr.id == "" {
			continue
		}
		key := strings.ToUpper(sr.id)
		idToPaths[key] = append(idToPaths[key], sr.path)
	}
	for id, paths := range idToPaths {
		if len(paths) > 1 {
			sort.Strings(paths)
			rep.DuplicateIDs[id] = paths
		}
	}

	if opts.Mode == ModeCheck {
		return rep, nil
	}

	// pass 3 — fix. Order: missing ids first (pure insert), then legacy id
	// replacements, then duplicate regeneration if requested.
	occupied := map[string]struct{}{}
	for id := range idToPaths {
		occupied[id] = struct{}{}
	}

	for _, sr := range results {
		if sr.hadID {
			continue
		}
		newID := nextFreeID(occupied)
		if err := replaceFrontmatterID(sr.path, newID); err != nil {
			rep.ParseErrors[sr.path] = err
			continue
		}
		occupied[newID] = struct{}{}
		rep.FixedMissingID = append(rep.FixedMissingID, sr.path)
	}

	for _, sr := range results {
		if !sr.wasLegacy && !sr.wasInvalid {
			continue
		}
		newID := nextFreeID(occupied)
		if err := replaceFrontmatterID(sr.path, newID); err != nil {
			rep.ParseErrors[sr.path] = err
			continue
		}
		occupied[newID] = struct{}{}
		rep.FixedLegacyID = append(rep.FixedLegacyID, sr.path)
	}

	if opts.RegenerateDuplicates {
		for _, paths := range rep.DuplicateIDs {
			// keep the first (sorted) file's id; regenerate the rest.
			for _, p := range paths[1:] {
				newID := nextFreeID(occupied)
				if err := replaceFrontmatterID(p, newID); err != nil {
					rep.ParseErrors[p] = err
					continue
				}
				occupied[newID] = struct{}{}
				rep.FixedDuplicates = append(rep.FixedDuplicates, p)
			}
		}
	}

	return rep, nil
}

// classifyFile reads path, parses its frontmatter, and returns a scan result.
// A nil error with fmBroken=true indicates the file had no recoverable
// frontmatter; the caller records it and moves on.
func classifyFile(path string) (*scanResultT, error) {
	content, err := os.ReadFile(path) //nolint:gosec // G304: repair walks the user's .doc dir by design
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	fm, err := document.ParseFrontmatter(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	sr := &scanResultT{path: path, content: content, fm: fm}
	raw, ok := document.FrontmatterID(fm.Map)
	if !ok || raw == "" {
		return sr, nil
	}
	sr.hadID = true
	normalized := document.NormalizeID(raw)
	switch {
	case document.IsValidID(normalized):
		sr.id = normalized
	case isLegacyTikiID(normalized):
		sr.id = normalized
		sr.wasLegacy = true
	default:
		sr.id = normalized
		sr.wasInvalid = true
	}
	return sr, nil
}

// scanResultT mirrors scanResult so classifyFile can live outside RepairIDs
// (the closure would otherwise capture a local type).
type scanResultT struct {
	path       string
	content    []byte
	fm         document.ParsedFrontmatter
	id         string
	hadID      bool
	wasLegacy  bool
	wasInvalid bool
}

// nextFreeID generates a bare id not present in the occupied set, registering
// it as occupied on return. With a 36^6 alphabet the retry loop terminates
// practically immediately.
func nextFreeID(occupied map[string]struct{}) string {
	for {
		candidate := document.NewID()
		if _, taken := occupied[candidate]; !taken {
			return candidate
		}
	}
}

// replaceFrontmatterID writes a new bare id into the file's frontmatter,
// replacing any existing `id:` line or inserting one as the first frontmatter
// line if none is present. It does not touch other fields or the body.
func replaceFrontmatterID(path, newID string) error {
	content, err := os.ReadFile(path) //nolint:gosec // G304: repair walks the user's .doc dir by design
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	updated, err := rewriteFrontmatterID(string(content), newID)
	if err != nil {
		return err
	}
	//nolint:gosec // G306: preserve existing 0644 used for task markdown
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// rewriteFrontmatterID is a narrow textual edit that either replaces an
// existing id line or inserts one as the first frontmatter line. When there
// is no frontmatter at all, it prepends a minimal block containing only id.
func rewriteFrontmatterID(content, newID string) (string, error) {
	// no frontmatter: prepend a minimal block. "\uFEFF" is the UTF-8 BOM.
	// The id is quoted so all-digit ids (e.g. "000001") survive a strict
	// load \u2014 unquoted yaml would decode them as integers and drop leading
	// zeros.
	if !strings.HasPrefix(strings.TrimPrefix(content, "\uFEFF"), "---") {
		return fmt.Sprintf("---\nid: %q\n---\n%s", newID, content), nil
	}

	firstNL := strings.Index(content, "\n")
	if firstNL == -1 {
		return content, fmt.Errorf("malformed frontmatter: no newline after opening delimiter")
	}
	closeIdx := strings.Index(content[firstNL+1:], "\n---")
	if closeIdx == -1 {
		return content, fmt.Errorf("malformed frontmatter: no closing delimiter")
	}
	closeAbs := firstNL + 1 + closeIdx

	head := content[:firstNL+1]
	body := content[firstNL+1 : closeAbs]
	tail := content[closeAbs:]

	lines := strings.Split(body, "\n")
	replaced := false
	for i, line := range lines {
		// Match only top-level id: — zero leading whitespace. Nested keys
		// (metadata:\n  id: nested) must never be touched or we risk
		// corrupting unrelated YAML structures. YAML top-level keys are
		// always column-0; we do not forgive indentation here.
		if isTopLevelIDLine(line) {
			lines[i] = fmt.Sprintf("id: %q", newID)
			replaced = true
			break
		}
	}
	if !replaced {
		// insert id as the first line so the field is easy to spot.
		lines = append([]string{fmt.Sprintf("id: %q", newID)}, lines...)
	}
	return head + strings.Join(lines, "\n") + tail, nil
}

// isTopLevelIDLine reports whether line begins at column 0 with `id:` — the
// YAML top-level key. Any leading whitespace disqualifies the line; nested
// `id:` keys inside mapping values must never be rewritten.
func isTopLevelIDLine(line string) bool {
	if len(line) == 0 {
		return false
	}
	// explicitly reject any leading whitespace (tabs too).
	if line[0] == ' ' || line[0] == '\t' {
		return false
	}
	if !strings.HasPrefix(line, "id:") {
		return false
	}
	// the character after "id:" must be whitespace, end-of-line, or nothing.
	// This rejects `idX:` / `identifier:` / `id:value-without-space`, which
	// isn't strictly required but avoids false positives.
	if len(line) == 3 {
		return true
	}
	next := line[3]
	return next == ' ' || next == '\t'
}

// WriteReport renders a human-readable summary to w. Suitable for both
// --check output and post-fix confirmation. Ignores write errors; the
// caller's Writer is user-facing (os.Stdout) and partial writes are benign.
func WriteReport(w io.Writer, rep *Report, mode Mode) {
	_, _ = fmt.Fprintf(w, "scanned %d file(s)\n", rep.Scanned)
	section(w, "missing id", rep.MissingID)
	section(w, "legacy TIKI- id", rep.LegacyID)
	section(w, "invalid id", rep.InvalidID)
	if len(rep.DuplicateIDs) > 0 {
		_, _ = fmt.Fprintf(w, "\nduplicate ids (%d):\n", len(rep.DuplicateIDs))
		for id, paths := range rep.DuplicateIDs {
			_, _ = fmt.Fprintf(w, "  %s\n", id)
			for _, p := range paths {
				_, _ = fmt.Fprintf(w, "    - %s\n", p)
			}
		}
	}
	if len(rep.ParseErrors) > 0 {
		_, _ = fmt.Fprintf(w, "\nparse errors (%d):\n", len(rep.ParseErrors))
		for p, err := range rep.ParseErrors {
			_, _ = fmt.Fprintf(w, "  - %s: %v\n", p, err)
		}
	}
	if mode == ModeFix {
		section(w, "fixed (added missing id)", rep.FixedMissingID)
		section(w, "fixed (replaced legacy id)", rep.FixedLegacyID)
		section(w, "fixed (regenerated duplicate)", rep.FixedDuplicates)
	}
}

func section(w io.Writer, label string, paths []string) {
	if len(paths) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "\n%s (%d):\n", label, len(paths))
	for _, p := range paths {
		_, _ = fmt.Fprintf(w, "  - %s\n", p)
	}
}
