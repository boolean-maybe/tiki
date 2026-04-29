package repair

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/document"
)

// TestRewriteFrontmatterID_QuotesNumericIDs verifies the M3 fix at the
// repair write site: when the new id is all-digit (e.g. 000001), the
// written line must be `id: "000001"` so YAML decodes it as a string on
// next load, not an int that drops leading zeros.
func TestRewriteFrontmatterID_QuotesNumericIDs(t *testing.T) {
	in := "---\ntitle: hello\n---\nbody\n"
	out, err := rewriteFrontmatterID(in, "000001")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out, `id: "000001"`) {
		t.Errorf("expected quoted id, got: %s", out)
	}

	// end-to-end: write, parse with document, verify the id survives as "000001".
	dir := t.TempDir()
	p := filepath.Join(dir, "f.md")
	if err := os.WriteFile(p, []byte(out), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	parsed, err := document.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	id, ok := document.FrontmatterID(parsed.Map)
	if !ok {
		t.Fatal("id missing after round-trip")
	}
	if id != "000001" {
		t.Errorf("id after round-trip = %q, want %q — yaml dropped leading zeros", id, "000001")
	}
}

// TestRewriteFrontmatterID_QuotesLetterIDsToo confirms we don't regress on
// the common case — quoted letter ids still round-trip cleanly.
func TestRewriteFrontmatterID_QuotesLetterIDsToo(t *testing.T) {
	in := "---\ntitle: hello\n---\nbody\n"
	out, err := rewriteFrontmatterID(in, "ABC123")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out, `id: "ABC123"`) {
		t.Errorf("expected quoted id, got: %s", out)
	}
}
