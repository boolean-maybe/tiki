package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

func writeMarkdownFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestIntegration_MarkdownTreeOpensWikiView drives Ctrl-O → filter → Enter and
// asserts the reusable markdown-file viewer plugin is pushed onto the nav stack.
// The doc scan root is the process cwd (config.GetDocDir()), so the test chdirs
// into a temp dir holding a nested .md before bootstrapping the app.
func TestIntegration_MarkdownTreeOpensWikiView(t *testing.T) {
	docRoot := t.TempDir()
	writeMarkdownFile(t, filepath.Join(docRoot, "docs", "hello.md"))
	t.Chdir(docRoot) // config.GetDocDir() resolves to cwd; do this before NewTestApp

	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// a view must be active for the router to dispatch globals (Ctrl-O)
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyCtrlO, 0, tcell.ModCtrl)
	if !ta.GetMarkdownTreeConfig().IsVisible() {
		t.Fatal("Ctrl-O did not open the markdown tree overlay")
	}

	ta.SendText("hello")
	// first selectable is the ancestor dir "docs/"; move down to the file node
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	want := model.MakePluginViewID(controller.MarkdownFileViewerPlugin)
	if got := ta.NavController.CurrentViewID(); got != want {
		t.Fatalf("current view = %q, want %q", got, want)
	}
}
