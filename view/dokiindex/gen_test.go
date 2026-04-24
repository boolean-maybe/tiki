package dokiindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scaffold creates a temp directory tree from a map of relative path → content.
func scaffold(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(abs), err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", abs, err)
		}
	}
	return root
}

func TestInjectTags_NoTags(t *testing.T) {
	content := "# Hello\n\nNo tags here."
	got := InjectTags(content, "/some/file.md")
	if got != content {
		t.Errorf("expected unchanged content, got %q", got)
	}
}

func TestInjectTags_EmptyFilePath(t *testing.T) {
	content := "<!-- INDEX_NAV -->"
	got := InjectTags(content, "")
	if got != content {
		t.Errorf("expected unchanged content with empty filePath, got %q", got)
	}
}

func TestIndexNav_Basic(t *testing.T) {
	root := scaffold(t, map[string]string{
		"index.md":           "# Root\n\n<!-- INDEX_NAV -->\n",
		"alpha/index.md":     "# Alpha\n\nAlpha content.\n",
		"beta/index.md":      "# Beta\n\nBeta content.\n",
		"beta/sub/index.md":  "# Sub\n\nSub content.\n",
	})

	content, _ := os.ReadFile(filepath.Join(root, "index.md"))
	got := InjectTags(string(content), filepath.Join(root, "index.md"))

	wantLines := []string{
		"- [Alpha](alpha/index.md)",
		"- [Beta](beta/index.md)",
		"- [  Sub](beta/sub/index.md)",
	}
	for _, line := range wantLines {
		if !strings.Contains(got, line) {
			t.Errorf("missing line %q in output:\n%s", line, got)
		}
	}
}

func TestIndexNav_OrderIsHierarchical(t *testing.T) {
	root := scaffold(t, map[string]string{
		"index.md":                  "<!-- INDEX_NAV -->",
		"external/index.md":         "# External\n",
		"external/friends/index.md": "# Friends\n",
		"cheat/index.md":            "# Cheat\n",
	})

	got := InjectTags("<!-- INDEX_NAV -->", filepath.Join(root, "index.md"))

	cheatPos := strings.Index(got, "Cheat")
	externalPos := strings.Index(got, "External")
	friendsPos := strings.Index(got, "Friends")

	if cheatPos < 0 || externalPos < 0 || friendsPos < 0 {
		t.Fatalf("missing entries in output:\n%s", got)
	}
	if cheatPos > externalPos {
		t.Errorf("expected Cheat before External (alphabetical top-level order)")
	}
	if externalPos > friendsPos {
		t.Errorf("expected External before Friends (parent before child)")
	}
}

func TestIndexNav_FallbackToDirectoryName(t *testing.T) {
	root := scaffold(t, map[string]string{
		"index.md":        "<!-- INDEX_NAV -->",
		"nosection/index.md": "No heading here.\n",
	})

	got := InjectTags("<!-- INDEX_NAV -->", filepath.Join(root, "index.md"))
	if !strings.Contains(got, "nosection") {
		t.Errorf("expected directory name fallback 'nosection' in output:\n%s", got)
	}
}

func TestIndexNav_NoSubdirs(t *testing.T) {
	root := scaffold(t, map[string]string{
		"index.md": "<!-- INDEX_NAV -->",
		"page.md":  "# Page\n",
	})

	got := InjectTags("<!-- INDEX_NAV -->", filepath.Join(root, "index.md"))
	// Tag should be replaced with empty string (no sub-index files found)
	if strings.Contains(got, "<!--") {
		t.Errorf("tag should be replaced, got:\n%s", got)
	}
}

func TestInclude_Basic(t *testing.T) {
	root := scaffold(t, map[string]string{
		"index.md":  "Before\n<!-- INCLUDE:snippet.md -->\nAfter\n",
		"snippet.md": "# Snippet\n\nIncluded content.\n",
	})

	content, _ := os.ReadFile(filepath.Join(root, "index.md"))
	got := InjectTags(string(content), filepath.Join(root, "index.md"))

	if !strings.Contains(got, "Included content.") {
		t.Errorf("expected included content in output:\n%s", got)
	}
	if !strings.Contains(got, "Before") || !strings.Contains(got, "After") {
		t.Errorf("expected surrounding content preserved:\n%s", got)
	}
	if strings.Contains(got, "<!--") {
		t.Errorf("tag should be replaced:\n%s", got)
	}
}

func TestInclude_MissingFile(t *testing.T) {
	root := scaffold(t, map[string]string{
		"index.md": "<!-- INCLUDE:missing.md -->",
	})

	got := InjectTags("<!-- INCLUDE:missing.md -->", filepath.Join(root, "index.md"))
	// Missing file: tag replaced with empty string, no panic
	if strings.Contains(got, "<!--") {
		t.Errorf("tag should be replaced even for missing file:\n%s", got)
	}
}

func TestInclude_LinkRewrite(t *testing.T) {
	root := scaffold(t, map[string]string{
		"index.md":         "<!-- INCLUDE:sub/notes.md -->",
		"sub/notes.md":     "See [this](other.md) and [that](../top.md).\n",
		"sub/other.md":     "",
		"top.md":           "",
	})

	got := InjectTags("<!-- INCLUDE:sub/notes.md -->", filepath.Join(root, "index.md"))

	if !strings.Contains(got, "[this](sub/other.md)") {
		t.Errorf("expected relative link rewritten to sub/other.md:\n%s", got)
	}
	if !strings.Contains(got, "[that](top.md)") {
		t.Errorf("expected relative link rewritten to top.md:\n%s", got)
	}
}

func TestInclude_AbsoluteLinksUnchanged(t *testing.T) {
	root := scaffold(t, map[string]string{
		"index.md":     "<!-- INCLUDE:sub/notes.md -->",
		"sub/notes.md": "[ext](https://example.com) [anchor](#heading)\n",
	})

	got := InjectTags("<!-- INCLUDE:sub/notes.md -->", filepath.Join(root, "index.md"))

	if !strings.Contains(got, "https://example.com") {
		t.Errorf("expected absolute URL preserved:\n%s", got)
	}
	if !strings.Contains(got, "#heading") {
		t.Errorf("expected anchor link preserved:\n%s", got)
	}
}

func TestInclude_NoRecursion(t *testing.T) {
	// INCLUDE (non-recursive) should NOT expand tags inside the included file
	root := scaffold(t, map[string]string{
		"index.md":   "<!-- INCLUDE:a.md -->",
		"a.md":       "<!-- INCLUDE:b.md -->",
		"b.md":       "B content\n",
	})

	got := InjectTags("<!-- INCLUDE:a.md -->", filepath.Join(root, "index.md"))

	// a.md's content is included, but its INCLUDE tag should NOT be expanded
	if strings.Contains(got, "B content") {
		t.Errorf("INCLUDE should not recurse into nested tags, got:\n%s", got)
	}
	if !strings.Contains(got, "<!-- INCLUDE:b.md -->") {
		t.Errorf("nested INCLUDE tag should be preserved as literal text:\n%s", got)
	}
}

func TestIncludeRecursive_Expands(t *testing.T) {
	root := scaffold(t, map[string]string{
		"index.md": "<!-- INCLUDE_RECURSIVE:a.md -->",
		"a.md":     "A\n<!-- INCLUDE:b.md -->\n",
		"b.md":     "B content\n",
	})

	got := InjectTags("<!-- INCLUDE_RECURSIVE:a.md -->", filepath.Join(root, "index.md"))

	if !strings.Contains(got, "B content") {
		t.Errorf("INCLUDE_RECURSIVE should expand nested INCLUDE tags:\n%s", got)
	}
}

func TestIncludeRecursive_CycleDetection(t *testing.T) {
	root := scaffold(t, map[string]string{
		"index.md": "<!-- INCLUDE_RECURSIVE:a.md -->",
		"a.md":     "A\n<!-- INCLUDE_RECURSIVE:index.md -->\n",
	})

	// Should not infinite loop or panic
	got := InjectTags("<!-- INCLUDE_RECURSIVE:a.md -->", filepath.Join(root, "index.md"))

	if strings.Contains(got, "<!--") {
		// Cycle should be silently dropped, not re-emitted as a tag
		t.Errorf("cycle should be silently skipped:\n%s", got)
	}
}

func TestRewriteLinks_SameDir(t *testing.T) {
	content := "[link](file.md)"
	got := rewriteLinks(content, "/a/b", "/a/b")
	if got != content {
		t.Errorf("same dir should return content unchanged, got %q", got)
	}
}

func TestRewriteLinks_ParentToChild(t *testing.T) {
	content := "[link](other.md)"
	got := rewriteLinks(content, "/a", "/a/sub")
	if got != "[link](sub/other.md)" {
		t.Errorf("got %q", got)
	}
}

func TestRewriteLinks_WithFragment(t *testing.T) {
	content := "[link](page.md#section)"
	got := rewriteLinks(content, "/a", "/a/sub")
	if got != "[link](sub/page.md#section)" {
		t.Errorf("got %q", got)
	}
}

func TestExtractH1(t *testing.T) {
	root := scaffold(t, map[string]string{
		"a.md": "Some preamble\n# My Title\nContent\n",
		"b.md": "## Not H1\nContent\n",
		"c.md": "# First\n# Second\n",
	})

	if got := extractH1(filepath.Join(root, "a.md")); got != "My Title" {
		t.Errorf("a.md: got %q", got)
	}
	if got := extractH1(filepath.Join(root, "b.md")); got != "" {
		t.Errorf("b.md: expected empty, got %q", got)
	}
	if got := extractH1(filepath.Join(root, "c.md")); got != "First" {
		t.Errorf("c.md: expected first H1, got %q", got)
	}
}
