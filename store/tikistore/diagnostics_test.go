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

	// missing id.
	mustWrite(t, dir, "missing.md", "---\ntitle: no id\n---\nbody\n")

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
		LoadReasonMissingID:   1,
		LoadReasonInvalidID:   2, // TIKI-LEG001 and AB both go here
		LoadReasonDuplicateID: 1,
		LoadReasonParseError:  1,
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
// With missing + invalid + duplicate rejections, all three guidance lines
// must show up.
func TestLoadDiagnostics_SummaryFormatsEachReasonGroup(t *testing.T) {
	diag := newLoadDiagnostics()
	diag.record("/a/missing.md", LoadReasonMissingID, "missing")
	diag.record("/a/invalid.md", LoadReasonInvalidID, "invalid")
	diag.record("/a/dup.md", LoadReasonDuplicateID, "duplicate")

	out := diag.Summary()
	if !strings.Contains(out, "3 file(s) failed to load") {
		t.Errorf("summary missing count line: %s", out)
	}
	if !strings.Contains(out, "missing id (1)") {
		t.Errorf("summary missing 'missing id' group: %s", out)
	}
	if !strings.Contains(out, "invalid id (1)") {
		t.Errorf("summary missing 'invalid id' group: %s", out)
	}
	if !strings.Contains(out, "duplicate id (1)") {
		t.Errorf("summary missing 'duplicate id' group: %s", out)
	}
	if !strings.Contains(out, "Add an `id:` frontmatter field") {
		t.Errorf("summary missing add-id hint when missing ids are present: %s", out)
	}
	if !strings.Contains(out, "Assign a fresh bare id") {
		t.Errorf("summary missing duplicate-id hint when duplicate ids are present: %s", out)
	}
	if !strings.Contains(out, "manual edits") {
		t.Errorf("summary missing manual-edit guidance when invalid is present: %s", out)
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

// TestLoadDiagnostics_SummaryOmitsManualNoteWhenOnlyMissing verifies the
// mirror case: when every rejection is a missing id, the user sees the
// add-id hint and is not bothered by the "manual edits" sentence.
func TestLoadDiagnostics_SummaryOmitsManualNoteWhenOnlyMissing(t *testing.T) {
	diag := newLoadDiagnostics()
	diag.record("/a/missing.md", LoadReasonMissingID, "missing")

	out := diag.Summary()
	if !strings.Contains(out, "Add an `id:`") {
		t.Errorf("summary missing add-id hint: %s", out)
	}
	if strings.Contains(out, "manual edits") {
		t.Errorf("summary must not mention manual edits when only missing ids are present: %s", out)
	}
	if strings.Contains(out, "Assign a fresh bare id") {
		t.Errorf("summary must not mention duplicate-id hint when only missing ids are present: %s", out)
	}
}

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

func mustWrite(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
