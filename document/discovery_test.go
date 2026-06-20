package document

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWalkDocuments_RespectsIgnoreFiles verifies the walker prunes paths
// matched by .gitignore and .tikiignore at the scan root, for both ignored
// directories (pruned wholesale) and ignored individual files.
func TestWalkDocuments_RespectsIgnoreFiles(t *testing.T) {
	root := t.TempDir()

	mustWrite(t, filepath.Join(root, ".gitignore"), "node_modules/\n")
	mustWrite(t, filepath.Join(root, ".tikiignore"), "DRAFT0.md\n")

	mustWrite(t, filepath.Join(root, "AAAAAA.md"), "valid doc\n")
	mustWrite(t, filepath.Join(root, "DRAFT0.md"), "draft, ignored via .tikiignore\n")
	mustWrite(t, filepath.Join(root, "node_modules", "ZZZZZZ.md"), "ignored via .gitignore\n")
	mustWrite(t, filepath.Join(root, "projects", "BBBBBB.md"), "real doc\n")

	paths, err := WalkDocuments(root)
	if err != nil {
		t.Fatalf("WalkDocuments: %v", err)
	}

	got := map[string]bool{}
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		got[filepath.ToSlash(rel)] = true
	}

	want := map[string]bool{
		"AAAAAA.md":          true,
		"projects/BBBBBB.md": true,
	}
	for w := range want {
		if !got[w] {
			t.Errorf("expected %s in walk result; got %v", w, got)
		}
	}
	for _, excluded := range []string{"DRAFT0.md", "node_modules/ZZZZZZ.md"} {
		if got[excluded] {
			t.Errorf("walk result unexpectedly included ignored path %s", excluded)
		}
	}
}

// TestWalkDocuments_ExcludesHiddenDirs belt-and-braces check for the hidden
// directory rule — `.git`, `.obsidian` etc. must be pruned.
func TestWalkDocuments_ExcludesHiddenDirs(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "AAAAAA.md"), "ok\n")
	mustWrite(t, filepath.Join(root, ".git", "ZZZZZZ.md"), "hidden\n")
	mustWrite(t, filepath.Join(root, ".obsidian", "cache", "YYYYYY.md"), "hidden deep\n")

	paths, err := WalkDocuments(root)
	if err != nil {
		t.Fatalf("WalkDocuments: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if filepath.Base(paths[0]) != "AAAAAA.md" {
		t.Errorf("unexpected path in walk: %s", paths[0])
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	//nolint:gosec // G301: 0755 matches repo-wide test-helper perms
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
