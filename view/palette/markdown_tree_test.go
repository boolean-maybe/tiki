package palette

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
)

func TestMain(m *testing.M) {
	// widget constructors call theme.Roles(); bootstrap normally sets the
	// theme, so tests must do it before any widget is built.
	theme.SetTheme(theme.LoadByName("dark"))
	os.Exit(m.Run())
}

func sampleTree() *store.MarkdownDir {
	// root
	//   docs/
	//     ruki/ select.md
	//     README.md
	//   scratch.md
	ruki := &store.MarkdownDir{Name: "ruki", RelPath: filepath.Join("docs", "ruki"),
		Files: []*store.MarkdownFile{{Name: "select.md", RelPath: filepath.Join("docs", "ruki", "select.md")}}}
	docs := &store.MarkdownDir{Name: "docs", RelPath: "docs",
		Dirs:  []*store.MarkdownDir{ruki},
		Files: []*store.MarkdownFile{{Name: "README.md", RelPath: filepath.Join("docs", "README.md")}}}
	return &store.MarkdownDir{
		Dirs:  []*store.MarkdownDir{docs},
		Files: []*store.MarkdownFile{{Name: "scratch.md", RelPath: "scratch.md"}},
	}
}

func TestMatchingFiles_EmptyQueryReturnsAll(t *testing.T) {
	m := matchingFiles(sampleTree(), "")
	if len(m) != 3 {
		t.Fatalf("empty query matched %d files, want 3", len(m))
	}
}

func TestMatchingFiles_FuzzyOnPath(t *testing.T) {
	m := matchingFiles(sampleTree(), "select")
	if !m[filepath.Join("docs", "ruki", "select.md")] {
		t.Fatalf("expected select.md matched, got %v", m)
	}
	if m["scratch.md"] {
		t.Fatalf("did not expect scratch.md to match 'select'")
	}
}

func TestExpandedDirsForMatches_ForcesAncestors(t *testing.T) {
	matched := matchingFiles(sampleTree(), "select")
	exp := expandedDirsForMatches(sampleTree(), matched)
	if !exp["docs"] || !exp[filepath.Join("docs", "ruki")] {
		t.Fatalf("expected docs and docs/ruki force-expanded, got %v", exp)
	}
}

func TestMarkdownTree_OnShowSelectsFirstAndEnterSelectsFile(t *testing.T) {
	cfg := model.NewMarkdownTreeConfig()
	var picked string
	cfg.SetOnSelect(func(p string) { picked = p })

	mt := NewMarkdownTree(cfg)
	mt.OnShow(sampleTree())

	// navigate to the first *file* node and simulate Enter.
	if got := mt.selectFirstFileForTest(); got == "" {
		t.Fatal("no file node found")
	}
	mt.pressEnterForTest()
	if picked == "" {
		t.Fatalf("Enter on file did not fire Select; picked=%q", picked)
	}
}

func TestMarkdownTree_ClearingFilterRestoresManualExpand(t *testing.T) {
	cfg := model.NewMarkdownTreeConfig()
	mt := NewMarkdownTree(cfg)
	mt.SetChangedFunc()
	mt.OnShow(sampleTree())

	// collapse "docs" manually, then filter and clear
	mt.setDirExpandedForTest("docs", false)
	mt.onFilterChanged("select") // captures manual state (docs=false), force-expands
	mt.onFilterChanged("")       // restores

	if mt.dirExpandedForTest("docs") {
		t.Fatal("docs should be restored to collapsed after clearing filter")
	}
}
