package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadDiagnostics_CategorizesAllRejectionKinds verifies the diagnostics
// bundle covers every rejection reason: missing id, invalid id (which now
// includes pre-unification TIKI- shapes as just another malformed value),
// duplicate id, and parse errors.
func TestLoadDiagnostics_CategorizesAllRejectionKinds(t *testing.T) {
	dir := t.TempDir()

	// good file — will load.
	mustWrite(t, dir, "GOOD01.md", "---\nid: GOOD01\ntitle: ok\ntype: story\nstatus: ready\npriority: high\n---\nbody\n")

	// no-id file: ordinary wiki content, skipped silently (no diagnostic).
	mustWrite(t, dir, "noid.md", "---\ntitle: no id\n---\nbody\n")

	// invalid id — TIKI- prefixed: no longer a recognized identity, so it
	// lands in the generic invalid bucket alongside other malformed values.
	mustWrite(t, dir, "legacy.md", "---\nid: TIKI-LEG001\ntitle: legacy\n---\nbody\n")

	// invalid id (too short).
	mustWrite(t, dir, "invalid.md", "---\nid: AB\ntitle: invalid\n---\nbody\n")

	// duplicate id (GOOD01 claimed already).
	mustWrite(t, dir, "dup.md", "---\nid: GOOD01\ntitle: dup\ntype: story\nstatus: ready\npriority: high\n---\nbody\n")

	// parse error — no closing delimiter.
	mustWrite(t, dir, "broken.md", "---\nid: ABC123\ntitle: broken\nno close\n")

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	diag := s.LoadDiagnostics()
	if !diag.HasIssues() {
		t.Fatal("expected HasIssues=true")
	}

	byReason := map[LoadReason]int{}
	for _, r := range diag.Rejections() {
		byReason[r.Reason]++
	}

	want := map[LoadReason]int{
		LoadReasonInvalidID:   2, // TIKI-LEG001 and AB both go here
		LoadReasonDuplicateID: 1,
		LoadReasonParseError:  1,
	}
	// the no-id file must not appear as a rejection at all.
	if len(diag.Rejections()) != 4 {
		t.Errorf("expected 4 rejections (no-id file skipped silently), got %d", len(diag.Rejections()))
	}
	for reason, count := range want {
		if got := byReason[reason]; got != count {
			t.Errorf("reason %s: got %d, want %d", reason, got, count)
		}
	}
}

// TestLoadDiagnostics_SummaryFormatsEachReasonGroup verifies that Summary()
// produces a multi-line report grouping rejections by reason, and that each
// guidance line appears iff a rejection of the matching kind is present.
// With invalid + duplicate + parse rejections, the duplicate and manual-edit
// guidance lines must show up.
func TestLoadDiagnostics_SummaryFormatsEachReasonGroup(t *testing.T) {
	diag := newLoadDiagnostics()
	diag.record("/a/invalid.md", LoadReasonInvalidID, "invalid")
	diag.record("/a/dup.md", LoadReasonDuplicateID, "duplicate")
	diag.record("/a/broken.md", LoadReasonParseError, "parse")

	out := diag.Summary()
	if !strings.Contains(out, "3 file(s) failed to load") {
		t.Errorf("summary missing count line: %s", out)
	}
	if !strings.Contains(out, "invalid id (1)") {
		t.Errorf("summary missing 'invalid id' group: %s", out)
	}
	if !strings.Contains(out, "duplicate id (1)") {
		t.Errorf("summary missing 'duplicate id' group: %s", out)
	}
	if !strings.Contains(out, "parse error (1)") {
		t.Errorf("summary missing 'parse error' group: %s", out)
	}
	if !strings.Contains(out, "Assign a fresh bare id") {
		t.Errorf("summary missing duplicate-id hint when duplicate ids are present: %s", out)
	}
	if !strings.Contains(out, "manual edits") {
		t.Errorf("summary missing manual-edit guidance when invalid is present: %s", out)
	}
	// an id-less file is never a rejection, so the add-id hint must never appear.
	if strings.Contains(out, "Add an `id:`") {
		t.Errorf("summary must not suggest add-id hint (id-less files are not rejected): %s", out)
	}
	// explicit negative: no legacy TIKI- wording anywhere in the report.
	if strings.Contains(out, "legacy") || strings.Contains(out, "TIKI-") {
		t.Errorf("summary must not reference legacy/TIKI-: %s", out)
	}
}

// TestLoadDiagnostics_SummaryOmitsFixHintWhenOnlyInvalid is the regression
// test for the "endless-loop banner" bug: if the only rejection is an
// invalid id (e.g. a TIKI-ABC123 value), the banner must NOT suggest the
// add-id or assign-fresh-id paths, since neither applies to an existing
// but malformed id.
func TestLoadDiagnostics_SummaryOmitsFixHintWhenOnlyInvalid(t *testing.T) {
	diag := newLoadDiagnostics()
	diag.record("/a/invalid.md", LoadReasonInvalidID, "invalid")

	out := diag.Summary()
	if !strings.Contains(out, "invalid id (1)") {
		t.Errorf("summary missing invalid-id group: %s", out)
	}
	if !strings.Contains(out, "manual edits") {
		t.Errorf("summary missing manual-edit guidance: %s", out)
	}
	if strings.Contains(out, "Add an `id:`") || strings.Contains(out, "Assign a fresh bare id") {
		t.Errorf("summary must not suggest add/assign-id hints when there are no missing or duplicate ids: %s", out)
	}
}

// note: the former TestLoadDiagnostics_SummaryOmitsManualNoteWhenOnlyMissing
// was removed. Id-less files are no longer a rejection reason (they are
// ordinary wiki content, skipped silently), so there is no "only missing ids"
// case for the summary to format.

// TestLoadDiagnostics_SummaryOffersAssignWhenOnlyDuplicates verifies that
// the banner surfaces the "assign a fresh id" guidance for duplicates
// rather than lumping them into the manual-edit bucket.
func TestLoadDiagnostics_SummaryOffersAssignWhenOnlyDuplicates(t *testing.T) {
	diag := newLoadDiagnostics()
	diag.record("/a/dup.md", LoadReasonDuplicateID, "duplicate")

	out := diag.Summary()
	if !strings.Contains(out, "duplicate id (1)") {
		t.Errorf("summary missing duplicate-id group: %s", out)
	}
	if !strings.Contains(out, "Assign a fresh bare id") {
		t.Errorf("summary must suggest assign-fresh-id for duplicate-only load: %s", out)
	}
	if strings.Contains(out, "manual edits") {
		t.Errorf("summary must not mention manual edits when duplicates are the only issue: %s", out)
	}
}

// TestLoadDiagnostics_EmptyWhenCleanLoad verifies that a load with no issues
// produces a diagnostics object whose HasIssues is false and whose Summary
// is empty.
func TestLoadDiagnostics_EmptyWhenCleanLoad(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "CLN001.md", "---\nid: CLN001\ntitle: clean\ntype: story\nstatus: ready\npriority: high\n---\nbody\n")

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	diag := s.LoadDiagnostics()
	if diag.HasIssues() {
		t.Errorf("clean load should have no issues, got: %+v", diag.Rejections())
	}
	if diag.Summary() != "" {
		t.Errorf("clean load Summary should be empty, got: %q", diag.Summary())
	}
}

// TestIDLessFileSilentlySkipped verifies that a markdown file with no
// frontmatter id is treated as ordinary (wiki) content: it is not indexed as
// a managed tiki and produces no load diagnostic.
func TestIDLessFileSilentlySkipped(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "notes.md", "# Just prose\n\nno frontmatter here\n")

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	if got := len(s.GetAllTikis()); got != 0 {
		t.Fatalf("expected 0 tikis, got %d", got)
	}
	if s.LoadDiagnostics().HasIssues() {
		t.Fatalf("id-less file must not produce diagnostics: %s", s.LoadDiagnostics().Summary())
	}
}

func mustWrite(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
