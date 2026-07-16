package store

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# doc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanMarkdown_NestingSortAndFilter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "scratch.md"))
	writeFile(t, filepath.Join(root, "notes", "ideas.md"))
	writeFile(t, filepath.Join(root, "docs", "ruki", "select.md"))
	writeFile(t, filepath.Join(root, "docs", "README.md"))
	writeFile(t, filepath.Join(root, "docs", "image.png")) // ignored: not .md
	writeFile(t, filepath.Join(root, ".git", "config.md")) // ignored: .git

	tree, err := ScanMarkdown(root)
	if err != nil {
		t.Fatalf("ScanMarkdown: %v", err)
	}

	// dirs first (docs, notes), alpha; then files (scratch.md)
	if len(tree.Dirs) != 2 || tree.Dirs[0].Name != "docs" || tree.Dirs[1].Name != "notes" {
		t.Fatalf("top-level dirs = %+v, want [docs notes]", dirNames(tree.Dirs))
	}
	if len(tree.Files) != 1 || tree.Files[0].Name != "scratch.md" {
		t.Fatalf("top-level files = %+v, want [scratch.md]", fileNames(tree.Files))
	}
	docs := tree.Dirs[0]
	if len(docs.Dirs) != 1 || docs.Dirs[0].Name != "ruki" {
		t.Fatalf("docs dirs = %+v, want [ruki]", dirNames(docs.Dirs))
	}
	if len(docs.Files) != 1 || docs.Files[0].Name != "README.md" {
		t.Fatalf("docs files = %+v, want [README.md]", fileNames(docs.Files))
	}
	sel := docs.Dirs[0].Files[0]
	if sel.RelPath != filepath.Join("docs", "ruki", "select.md") {
		t.Fatalf("select relpath = %q", sel.RelPath)
	}
	if sel.AbsPath != filepath.Join(root, "docs", "ruki", "select.md") {
		t.Fatalf("select abspath = %q", sel.AbsPath)
	}
}

func TestScanMarkdown_EmptyDirsOmitted(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "empty", "deeper"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "keep", "a.md"))

	tree, err := ScanMarkdown(root)
	if err != nil {
		t.Fatalf("ScanMarkdown: %v", err)
	}
	if len(tree.Dirs) != 1 || tree.Dirs[0].Name != "keep" {
		t.Fatalf("dirs = %+v, want [keep] (empty dir omitted)", dirNames(tree.Dirs))
	}
}

func dirNames(ds []*MarkdownDir) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Name
	}
	return out
}

func fileNames(fs []*MarkdownFile) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Name
	}
	return out
}
