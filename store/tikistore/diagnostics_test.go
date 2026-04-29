package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadDiagnostics_CategorizesAllRejectionKinds verifies the diagnostics
// bundle covers every rejection reason: missing id, legacy id, invalid id,
// duplicate id, and parse errors.
func TestLoadDiagnostics_CategorizesAllRejectionKinds(t *testing.T) {
	dir := t.TempDir()

	// good file — will load.
	mustWrite(t, dir, "GOOD01.md", "---\nid: GOOD01\ntitle: ok\ntype: story\nstatus: ready\npriority: 1\n---\nbody\n")

	// missing id.
	mustWrite(t, dir, "missing.md", "---\ntitle: no id\n---\nbody\n")

	// legacy TIKI- id.
	mustWrite(t, dir, "legacy.md", "---\nid: TIKI-LEG001\ntitle: legacy\n---\nbody\n")

	// invalid id (too short, not TIKI- shape).
	mustWrite(t, dir, "invalid.md", "---\nid: AB\ntitle: invalid\n---\nbody\n")

	// duplicate id (GOOD01 claimed already).
	mustWrite(t, dir, "dup.md", "---\nid: GOOD01\ntitle: dup\ntype: story\nstatus: ready\npriority: 1\n---\nbody\n")

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
		LoadReasonLegacyID:    1,
		LoadReasonInvalidID:   1,
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
// produces a multi-line report grouping rejections by reason and pointing
// at the repair command.
func TestLoadDiagnostics_SummaryFormatsEachReasonGroup(t *testing.T) {
	diag := newLoadDiagnostics()
	diag.record("/a/missing.md", LoadReasonMissingID, "missing")
	diag.record("/a/legacy.md", LoadReasonLegacyID, "legacy")
	diag.record("/a/dup.md", LoadReasonDuplicateID, "duplicate")

	out := diag.Summary()
	if !strings.Contains(out, "3 file(s) failed to load") {
		t.Errorf("summary missing count line: %s", out)
	}
	if !strings.Contains(out, "missing id (1)") {
		t.Errorf("summary missing 'missing id' group: %s", out)
	}
	if !strings.Contains(out, "legacy TIKI- id (1)") {
		t.Errorf("summary missing 'legacy id' group: %s", out)
	}
	if !strings.Contains(out, "duplicate id (1)") {
		t.Errorf("summary missing 'duplicate id' group: %s", out)
	}
	if !strings.Contains(out, "tiki repair ids") {
		t.Errorf("summary missing repair command hint: %s", out)
	}
}

// TestLoadDiagnostics_EmptyWhenCleanLoad verifies that a load with no issues
// produces a diagnostics object whose HasIssues is false and whose Summary
// is empty.
func TestLoadDiagnostics_EmptyWhenCleanLoad(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "CLN001.md", "---\nid: CLN001\ntitle: clean\ntype: story\nstatus: ready\npriority: 1\n---\nbody\n")

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
