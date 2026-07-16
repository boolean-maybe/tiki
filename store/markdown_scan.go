package store

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MarkdownFile is a single .md file discovered under the scan root.
type MarkdownFile struct {
	Name    string // base filename, e.g. "select.md"
	RelPath string // path relative to the scan root
	AbsPath string // absolute path on disk
}

// MarkdownDir is a directory node in the scanned markdown tree. The root
// node has an empty Name and RelPath.
type MarkdownDir struct {
	Name    string
	RelPath string
	Dirs    []*MarkdownDir
	Files   []*MarkdownFile
}

// ScanMarkdown walks root recursively and returns a tree of the .md files
// beneath it. Directories with no .md anywhere beneath them are omitted.
// Dirs are listed before files; both are sorted alphabetically by name.
func ScanMarkdown(root string) (*MarkdownDir, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return scanDir(abs, ""), nil
}

func scanDir(absDir, relDir string) *MarkdownDir {
	node := &MarkdownDir{Name: filepath.Base(relDir), RelPath: relDir}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		slog.Debug("markdown scan: unreadable dir", "dir", absDir, "error", err)
		return node
	}
	for _, e := range entries {
		if e.IsDir() {
			if e.Name() == ".git" {
				continue
			}
			childRel := filepath.Join(relDir, e.Name())
			child := scanDir(filepath.Join(absDir, e.Name()), childRel)
			if len(child.Dirs) > 0 || len(child.Files) > 0 {
				node.Dirs = append(node.Dirs, child)
			}
			continue
		}
		if !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			continue
		}
		rel := filepath.Join(relDir, e.Name())
		node.Files = append(node.Files, &MarkdownFile{
			Name:    e.Name(),
			RelPath: rel,
			AbsPath: filepath.Join(absDir, e.Name()),
		})
	}
	sort.Slice(node.Dirs, func(i, j int) bool { return node.Dirs[i].Name < node.Dirs[j].Name })
	sort.Slice(node.Files, func(i, j int) bool { return node.Files[i].Name < node.Files[j].Name })
	return node
}
