package document

import (
	"strings"
	"testing"
)

func TestSplitFrontmatter_noFrontmatter(t *testing.T) {
	got, err := SplitFrontmatter("just a body\nwith text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Frontmatter != "" {
		t.Errorf("expected empty frontmatter, got %q", got.Frontmatter)
	}
	if got.Body != "just a body\nwith text" {
		t.Errorf("body mismatch: %q", got.Body)
	}
}

func TestSplitFrontmatter_simple(t *testing.T) {
	content := "---\ntitle: hello\nid: ABC123\n---\nbody line"
	got, err := SplitFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got.Frontmatter, "title: hello") {
		t.Errorf("frontmatter missing title: %q", got.Frontmatter)
	}
	if got.Body != "body line" {
		t.Errorf("body: want %q, got %q", "body line", got.Body)
	}
}

func TestSplitFrontmatter_preservesLeadingBlankLines(t *testing.T) {
	// body-byte preservation: a blank line immediately after "---\n" is part
	// of the body, not delimiter whitespace.
	content := "---\nid: ABC123\n---\n\n# heading\n\nparagraph\n"
	got, err := SplitFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "\n# heading\n\nparagraph\n"
	if got.Body != want {
		t.Errorf("body preservation failed\n want: %q\n got:  %q", want, got.Body)
	}
}

func TestSplitFrontmatter_preservesTrailingNewlines(t *testing.T) {
	content := "---\nid: ABC123\n---\nparagraph\n\n\n"
	got, err := SplitFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Body != "paragraph\n\n\n" {
		t.Errorf("trailing newlines lost: %q", got.Body)
	}
}

func TestSplitFrontmatter_bodyWithoutFrontmatterUnchanged(t *testing.T) {
	// plain markdown must not be touched — no leading/trailing trimming.
	content := "\n\n# title\n\ncontent\n\n"
	got, err := SplitFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Body != content {
		t.Errorf("plain markdown body was altered:\n want: %q\n got:  %q", content, got.Body)
	}
}

func TestSplitFrontmatter_missingClose(t *testing.T) {
	_, err := SplitFrontmatter("---\ntitle: hello\nno close")
	if err == nil {
		t.Error("expected error for missing closing delimiter")
	}
}

// TestSplitFrontmatter_emptyBlockDistinct verifies M2 fix: an empty
// frontmatter block (---\n---\nbody) is distinguishable from plain markdown
// (no delimiters at all).
func TestSplitFrontmatter_emptyBlockDistinct(t *testing.T) {
	empty, err := SplitFrontmatter("---\n---\nbody\n")
	if err != nil {
		t.Fatalf("unexpected error for empty block: %v", err)
	}
	if !empty.HadDelimiters {
		t.Error("empty frontmatter block should set HadDelimiters=true")
	}
	if empty.Frontmatter != "" {
		t.Errorf("empty block Frontmatter should be empty, got %q", empty.Frontmatter)
	}
	if empty.Body != "body\n" {
		t.Errorf("empty block Body: want %q, got %q", "body\n", empty.Body)
	}

	plain, err := SplitFrontmatter("body\n")
	if err != nil {
		t.Fatalf("unexpected error for plain markdown: %v", err)
	}
	if plain.HadDelimiters {
		t.Error("plain markdown should set HadDelimiters=false")
	}
}

// TestParseFrontmatter_emptyBlockHasFrontmatter verifies M2 at the parse
// layer: HasFrontmatter is true for an empty block and false for plain
// markdown. Callers can use len(Map) > 0 for the "has keys" question.
func TestParseFrontmatter_emptyBlockHasFrontmatter(t *testing.T) {
	empty, err := ParseFrontmatter("---\n---\nbody\n")
	if err != nil {
		t.Fatalf("empty block parse: %v", err)
	}
	if !empty.HasFrontmatter {
		t.Error("empty block should have HasFrontmatter=true")
	}
	if len(empty.Map) != 0 {
		t.Errorf("empty block should have empty Map, got %+v", empty.Map)
	}

	plain, err := ParseFrontmatter("body\n")
	if err != nil {
		t.Fatalf("plain parse: %v", err)
	}
	if plain.HasFrontmatter {
		t.Error("plain markdown should have HasFrontmatter=false")
	}
}

func TestParseFrontmatter_preservesUnknown(t *testing.T) {
	content := "---\nid: ABC123\ntitle: t\ncustomField: hello\n---\nbody"
	got, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.HasFrontmatter {
		t.Error("expected HasFrontmatter=true")
	}
	if got.Map["customField"] != "hello" {
		t.Errorf("customField not preserved: %+v", got.Map)
	}
	if got.Body != "body" {
		t.Errorf("body: want %q, got %q", "body", got.Body)
	}
}

func TestFrontmatterID(t *testing.T) {
	id, ok := FrontmatterID(map[string]interface{}{"id": "ABC123 "})
	if !ok || id != "ABC123" {
		t.Errorf("FrontmatterID present: got (%q, %v)", id, ok)
	}
	if _, ok := FrontmatterID(map[string]interface{}{}); ok {
		t.Error("FrontmatterID empty map should report missing")
	}
	// Numeric ids are accepted (YAML decodes all-digit values as int).
	id, ok = FrontmatterID(map[string]interface{}{"id": 42})
	if !ok || id != "42" {
		t.Errorf("FrontmatterID int: got (%q, %v), want (\"42\", true)", id, ok)
	}
	// Unsupported types (bool, map, list) are rejected.
	if _, ok := FrontmatterID(map[string]interface{}{"id": true}); ok {
		t.Error("FrontmatterID bool should report missing")
	}
}
