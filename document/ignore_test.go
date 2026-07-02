package document

import (
	"path/filepath"
	"testing"
)

func TestIgnoreMatcherRespectsGitignoreAndTikiignore(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".gitignore"), "node_modules/\n")
	mustWrite(t, filepath.Join(root, ".tikiignore"), "README.md\n")

	m, err := LoadIgnoreMatcher(root)
	if err != nil {
		t.Fatalf("LoadIgnoreMatcher: %v", err)
	}
	if !m.Match("node_modules/x.md", false) {
		t.Error("gitignore pattern not honored")
	}
	if !m.Match("README.md", false) {
		t.Error("tikiignore pattern not honored")
	}
	if m.Match("docs/keep.md", false) {
		t.Error("unmatched path should not be ignored")
	}
}
