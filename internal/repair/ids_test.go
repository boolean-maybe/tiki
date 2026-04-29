package repair

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/document"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func TestRepairIDs_checkReportsMissingLegacyAndDuplicates(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "missing.md", "---\ntitle: hello\n---\nbody\n")
	writeFile(t, dir, "legacy.md", "---\nid: TIKI-ABC123\ntitle: hello\n---\nbody\n")
	writeFile(t, dir, "valid.md", "---\nid: \"XYZ789\"\ntitle: hello\n---\nbody\n")
	writeFile(t, dir, "dup-a.md", "---\nid: \"QQQQQQ\"\ntitle: a\n---\nbody\n")
	writeFile(t, dir, "dup-b.md", "---\nid: \"QQQQQQ\"\ntitle: b\n---\nbody\n")

	rep, err := RepairIDs(Options{Dir: dir, Mode: ModeCheck})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Scanned != 5 {
		t.Errorf("Scanned = %d, want 5", rep.Scanned)
	}
	if len(rep.MissingID) != 1 {
		t.Errorf("MissingID = %v, want 1 entry", rep.MissingID)
	}
	if len(rep.LegacyID) != 1 {
		t.Errorf("LegacyID = %v, want 1 entry", rep.LegacyID)
	}
	if _, has := rep.DuplicateIDs["QQQQQQ"]; !has {
		t.Errorf("DuplicateIDs missing QQQQQQ: %+v", rep.DuplicateIDs)
	}
	if !rep.HasIssues() {
		t.Error("HasIssues should be true")
	}

	// check mode must not have modified anything.
	content, _ := os.ReadFile(filepath.Join(dir, "missing.md"))
	if strings.Contains(string(content), "id:") {
		t.Errorf("check mode wrote id into missing.md: %s", content)
	}
}

func TestRepairIDs_fixMissingID(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "missing.md", "---\ntitle: hello\n---\nbody\n")

	rep, err := RepairIDs(Options{Dir: dir, Mode: ModeFix})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rep.FixedMissingID) != 1 {
		t.Fatalf("FixedMissingID = %v, want 1 entry", rep.FixedMissingID)
	}

	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read after fix: %v", err)
	}
	// round-trip through the document parser so we get the unquoted id
	// value regardless of YAML quoting style.
	parsed, err := document.ParseFrontmatter(string(content))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	id, ok := document.FrontmatterID(parsed.Map)
	if !ok {
		t.Fatal("id missing after fix")
	}
	if !document.IsValidID(id) {
		t.Errorf("inserted id %q is not valid", id)
	}
	// title must still be there.
	if !strings.Contains(string(content), "title: hello") {
		t.Errorf("title lost after fix: %s", content)
	}
}

func TestRepairIDs_fixLegacyID(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "legacy.md", "---\nid: TIKI-ABC123\ntitle: hello\n---\nbody\n")

	rep, err := RepairIDs(Options{Dir: dir, Mode: ModeFix})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rep.FixedLegacyID) != 1 {
		t.Fatalf("FixedLegacyID = %v, want 1 entry", rep.FixedLegacyID)
	}

	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(content), "TIKI-") {
		t.Errorf("legacy id still present: %s", content)
	}
}

func TestRepairIDs_fixDuplicatesRequiresFlag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.md", "---\nid: \"DUPDUP\"\ntitle: a\n---\nbody\n")
	writeFile(t, dir, "b.md", "---\nid: \"DUPDUP\"\ntitle: b\n---\nbody\n")

	// without --regenerate-duplicates, duplicates are reported but not fixed.
	rep, err := RepairIDs(Options{Dir: dir, Mode: ModeFix, RegenerateDuplicates: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rep.DuplicateIDs) != 1 {
		t.Errorf("expected 1 duplicate set, got %+v", rep.DuplicateIDs)
	}
	if len(rep.FixedDuplicates) != 0 {
		t.Errorf("FixedDuplicates should be empty without flag, got %v", rep.FixedDuplicates)
	}

	// with --regenerate-duplicates, one of the two gets rewritten.
	rep, err = RepairIDs(Options{Dir: dir, Mode: ModeFix, RegenerateDuplicates: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rep.FixedDuplicates) != 1 {
		t.Errorf("FixedDuplicates = %v, want 1 entry", rep.FixedDuplicates)
	}
}

func TestRewriteFrontmatterID_preservesOtherFields(t *testing.T) {
	in := "---\nid: TIKI-ABC123\ntitle: hello\ntype: story\n---\nbody\n\nmore body\n"
	out, err := rewriteFrontmatterID(in, "ZZZZZZ")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out, `id: "ZZZZZZ"`) {
		t.Errorf("new id missing: %s", out)
	}
	if strings.Contains(out, "TIKI-ABC123") {
		t.Errorf("old id not replaced: %s", out)
	}
	if !strings.Contains(out, "title: hello") {
		t.Errorf("title lost: %s", out)
	}
	if !strings.Contains(out, "type: story") {
		t.Errorf("type lost: %s", out)
	}
	if !strings.Contains(out, "body\n\nmore body") {
		t.Errorf("body not preserved: %s", out)
	}
}

func TestRewriteFrontmatterID_insertsWhenMissing(t *testing.T) {
	in := "---\ntitle: hello\n---\nbody\n"
	out, err := rewriteFrontmatterID(in, "ABC123")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.HasPrefix(out, "---\nid: \"ABC123\"\ntitle: hello\n---\n") {
		t.Errorf("insert shape wrong:\n%s", out)
	}
}

// TestRewriteFrontmatterID_doesNotTouchNestedID verifies the H1 fix: an id:
// key nested inside another mapping (column > 0) must be left alone. The
// top-level id line is the only one that should be rewritten.
func TestRewriteFrontmatterID_doesNotTouchNestedID(t *testing.T) {
	in := "---\nid: \"OLD001\"\nmetadata:\n  id: nested-untouched\n  other: value\n---\nbody\n"
	out, err := rewriteFrontmatterID(in, "NEW001")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out, `id: "NEW001"`) {
		t.Errorf("top-level id not replaced: %s", out)
	}
	if strings.Contains(out, "OLD001") {
		t.Errorf("top-level id still present: %s", out)
	}
	if !strings.Contains(out, "id: nested-untouched") {
		t.Errorf("nested id was incorrectly rewritten: %s", out)
	}
}

// TestRewriteFrontmatterID_doesNotTouchIndentedTopLevel verifies that a
// mis-indented top-level id (which is a YAML error waiting to happen) is
// still not rewritten. We do not guess intent.
func TestRewriteFrontmatterID_doesNotTouchIndentedTopLevel(t *testing.T) {
	in := "---\n  id: looks-indented\ntitle: hi\n---\nbody\n"
	out, err := rewriteFrontmatterID(in, "NEW001")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// Indented "id:" is not top-level; an id should be inserted above it,
	// not replace it.
	if !strings.HasPrefix(out, "---\nid: \"NEW001\"\n") {
		t.Errorf("expected inserted top-level id at start, got: %s", out)
	}
	if !strings.Contains(out, "id: looks-indented") {
		t.Errorf("indented pseudo-id was rewritten: %s", out)
	}
}

// TestRewriteFrontmatterID_onlyReplacesOnce verifies we stop after the first
// top-level match, so a file with accidental top-level id duplication (which
// is invalid YAML but survives parsing) gets exactly one rewrite.
func TestRewriteFrontmatterID_onlyReplacesOnce(t *testing.T) {
	in := "---\nid: \"FIRST1\"\ntitle: hi\nid: \"SECOND\"\n---\nbody\n"
	out, err := rewriteFrontmatterID(in, "NEW001")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out, `id: "NEW001"`) {
		t.Errorf("first id not replaced: %s", out)
	}
	if strings.Contains(out, "FIRST1") {
		t.Errorf("first id still present: %s", out)
	}
	// second top-level id is left alone — repair only rewrites the first.
	if !strings.Contains(out, `id: "SECOND"`) {
		t.Errorf("second id was rewritten: %s", out)
	}
}

func TestIsTopLevelIDLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{`id: "ABC123"`, true},
		{"id:", true}, // valueless — still structurally a top-level id key
		{"id:\tvalue", true},
		{"  id: nested", false},
		{" id: single-space-indent", false},
		{"\tid: tab-indent", false},
		{"identifier: foo", false},
		{"idx: foo", false},
		{"", false},
		{"title: hi", false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := isTopLevelIDLine(tt.line); got != tt.want {
				t.Errorf("isTopLevelIDLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

// TestRepairIDs_walksRecursivelyAndExcludesConfig is the Phase 2 gate:
// `tiki repair ids` must see the same files the store sees. A missing id
// on a nested document or a new `.doc/<ID>.md` must be reported and
// fixable; reserved top-level config files must never be misread as
// documents (a valid-looking `workflow.md` should not be inserted-into).
func TestRepairIDs_walksRecursivelyAndExcludesConfig(t *testing.T) {
	root := t.TempDir()

	// flat-at-root: gets reported.
	writeFile(t, root, "flat-missing.md", "---\ntitle: flat\n---\nbody\n")
	// nested one level deep: must be reached by the recursive walk.
	nestedDir := filepath.Join(root, "docs")
	//nolint:gosec // G301: 0755 matches the rest of the repair test suite
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	writeFile(t, nestedDir, "nested-missing.md", "---\ntitle: nested\n---\nbody\n")
	// legacy-layout subdir (`.doc/tiki/<file>.md`): also picked up.
	legacyDir := filepath.Join(root, "tiki")
	//nolint:gosec // G301: 0755 matches the rest of the repair test suite
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy: %v", err)
	}
	writeFile(t, legacyDir, "legacy-missing.md", "---\ntitle: legacy-layout\n---\nbody\n")
	// reserved project config files: must never be treated as documents.
	writeFile(t, root, "workflow.md", "---\ntitle: reserved\n---\nbody\n")
	writeFile(t, root, "config.md", "---\ntitle: reserved\n---\nbody\n")

	rep, err := RepairIDs(Options{Dir: root, Mode: ModeCheck})
	if err != nil {
		t.Fatalf("RepairIDs: %v", err)
	}

	// Three real documents, no config files.
	if rep.Scanned != 3 {
		t.Errorf("Scanned = %d, want 3 (config files must be excluded)", rep.Scanned)
	}
	if len(rep.MissingID) != 3 {
		t.Errorf("MissingID count = %d, want 3: %v", len(rep.MissingID), rep.MissingID)
	}

	// Ensure every expected path is reported — regression guard against a
	// walker that skips nested directories.
	want := map[string]bool{
		filepath.Join(root, "flat-missing.md"):        true,
		filepath.Join(nestedDir, "nested-missing.md"): true,
		filepath.Join(legacyDir, "legacy-missing.md"): true,
	}
	for _, p := range rep.MissingID {
		delete(want, p)
	}
	for p := range want {
		t.Errorf("expected MissingID to include %s", p)
	}

	// Config files must never appear in any report bucket.
	for _, bucket := range [][]string{rep.MissingID, rep.LegacyID, rep.InvalidID} {
		for _, p := range bucket {
			if strings.HasSuffix(p, "config.md") || strings.HasSuffix(p, "workflow.md") {
				t.Errorf("reserved config file leaked into report: %s", p)
			}
		}
	}
}

func TestWriteReport_includesSections(t *testing.T) {
	rep := &Report{
		Scanned:       3,
		MissingID:     []string{"/a/b.md"},
		LegacyID:      []string{"/a/c.md"},
		FixedLegacyID: []string{"/a/c.md"},
	}
	var buf bytes.Buffer
	WriteReport(&buf, rep, ModeFix)
	out := buf.String()
	if !strings.Contains(out, "scanned 3") {
		t.Errorf("missing scanned count: %s", out)
	}
	if !strings.Contains(out, "missing id") || !strings.Contains(out, "/a/b.md") {
		t.Errorf("missing-id section missing: %s", out)
	}
	if !strings.Contains(out, "fixed (replaced legacy id)") {
		t.Errorf("fix section missing: %s", out)
	}
}
