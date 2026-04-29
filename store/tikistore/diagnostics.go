package tikistore

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// LoadReason classifies why a file failed to load. One value per finding so
// callers can report them in groups instead of one free-form error per file.
type LoadReason int

const (
	// LoadReasonMissingID means the file had no frontmatter id.
	LoadReasonMissingID LoadReason = iota
	// LoadReasonLegacyID means the frontmatter id matched the pre-unification
	// TIKI-XXXXXX form. Run `tiki repair ids --fix` to migrate.
	LoadReasonLegacyID
	// LoadReasonInvalidID covers every other malformed id value: wrong shape,
	// unsupported type, etc.
	LoadReasonInvalidID
	// LoadReasonDuplicateID means another file had already registered this id.
	// The first file wins; subsequent occurrences are reported here.
	LoadReasonDuplicateID
	// LoadReasonParseError means YAML or markdown splitting failed.
	LoadReasonParseError
	// LoadReasonOther catches reasons not covered above; the Message field
	// carries the raw error text.
	LoadReasonOther
)

// String renders a human-readable label for a reason.
func (r LoadReason) String() string {
	switch r {
	case LoadReasonMissingID:
		return "missing id"
	case LoadReasonLegacyID:
		return "legacy TIKI- id"
	case LoadReasonInvalidID:
		return "invalid id"
	case LoadReasonDuplicateID:
		return "duplicate id"
	case LoadReasonParseError:
		return "parse error"
	default:
		return "other"
	}
}

// LoadRejection records a single file the store refused to load.
type LoadRejection struct {
	Path    string
	Reason  LoadReason
	Message string
}

// loadError is the internal error shape returned by loadTaskFile so
// loadLocked can classify rejections without reparsing the error string.
type loadError struct {
	reason LoadReason
	err    error
}

func (e *loadError) Error() string { return e.err.Error() }
func (e *loadError) Unwrap() error { return e.err }

func newLoadError(reason LoadReason, err error) *loadError {
	return &loadError{reason: reason, err: err}
}

// LoadDiagnostics summarizes every file skipped during NewTikiStore /
// Reload. Safe for concurrent read after load completes.
type LoadDiagnostics struct {
	mu         sync.Mutex
	rejections []LoadRejection
}

func newLoadDiagnostics() *LoadDiagnostics {
	return &LoadDiagnostics{}
}

func (d *LoadDiagnostics) record(path string, reason LoadReason, msg string) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rejections = append(d.rejections, LoadRejection{Path: path, Reason: reason, Message: msg})
}

// Rejections returns a copy of all rejections sorted by path. Safe to call
// after load completes; returns nil when no rejections were recorded.
func (d *LoadDiagnostics) Rejections() []LoadRejection {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.rejections) == 0 {
		return nil
	}
	out := make([]LoadRejection, len(d.rejections))
	copy(out, d.rejections)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// HasIssues reports whether any file was skipped.
func (d *LoadDiagnostics) HasIssues() bool {
	if d == nil {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.rejections) > 0
}

// Summary returns a multi-line human-readable summary suitable for a
// startup warning banner. Empty string when HasIssues() is false.
func (d *LoadDiagnostics) Summary() string {
	rejections := d.Rejections()
	if len(rejections) == 0 {
		return ""
	}
	byReason := map[LoadReason][]string{}
	for _, r := range rejections {
		byReason[r.Reason] = append(byReason[r.Reason], r.Path)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d file(s) failed to load:\n", len(rejections))
	for _, reason := range []LoadReason{LoadReasonMissingID, LoadReasonLegacyID, LoadReasonInvalidID, LoadReasonDuplicateID, LoadReasonParseError, LoadReasonOther} {
		paths := byReason[reason]
		if len(paths) == 0 {
			continue
		}
		fmt.Fprintf(&b, "  %s (%d):\n", reason, len(paths))
		for _, p := range paths {
			fmt.Fprintf(&b, "    - %s\n", p)
		}
	}
	b.WriteString("Run `tiki repair ids --check` for details and `tiki repair ids --fix` to repair.\n")
	return b.String()
}
