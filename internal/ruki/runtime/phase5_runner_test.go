package runtime

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/tikistore"
	"github.com/boolean-maybe/tiki/task"
)

// TestPhase5_CLISelectIncludesPlainDocs proves the first review finding is
// closed: ruki `select` at the CLI layer must surface plain documents, not
// just workflow-capable tasks. Pre-fix the runner called readStore.GetAllTasks
// which applies the workflow-only filter at the store boundary, so plain
// docs never reached the executor and `select` could never see them.
//
// The fix swaps the CLI path to DocumentReadStore.GetAllDocuments (projected
// through task.FromDocument), which is the unfiltered view.
func TestPhase5_CLISelectIncludesPlainDocs(t *testing.T) {
	setupRunnerTest(t) // ensure status registry is populated

	s := store.NewInMemoryStore()
	// Seed a workflow doc and a plain doc side by side.
	if err := s.CreateTask(&task.Task{ID: "WRKFL1", Title: "workflow item", Status: "ready", Priority: 1}); err != nil {
		t.Fatalf("seed workflow: %v", err)
	}
	if err := s.CreateDocument(&document.Document{
		ID:    "PLAIN1",
		Title: "plain note",
		Body:  "just a markdown note",
		// no workflow frontmatter — plain doc
	}); err != nil {
		t.Fatalf("seed plain: %v", err)
	}

	var buf bytes.Buffer
	if err := RunSelectQuery(s, "select id, title", &buf); err != nil {
		t.Fatalf("RunSelectQuery: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "WRKFL1") {
		t.Errorf("workflow doc WRKFL1 missing from output:\n%s", out)
	}
	if !strings.Contains(out, "PLAIN1") {
		t.Errorf("plain doc PLAIN1 missing from output — this was the pre-fix bug:\n%s", out)
	}
}

// TestPhase5_CLIHasStatusFiltersPlainDocs proves `has(status)` discriminates
// workflow from plain docs end-to-end at the CLI layer. This is the
// canonical "show only workflow documents" filter and only works when the
// runner has given the executor the unfiltered document set.
func TestPhase5_CLIHasStatusFiltersPlainDocs(t *testing.T) {
	setupRunnerTest(t)

	s := store.NewInMemoryStore()
	if err := s.CreateTask(&task.Task{ID: "WRKFL1", Title: "workflow", Status: "ready", Priority: 1}); err != nil {
		t.Fatalf("seed workflow: %v", err)
	}
	if err := s.CreateDocument(&document.Document{ID: "PLAIN1", Title: "plain", Body: "note"}); err != nil {
		t.Fatalf("seed plain: %v", err)
	}

	var buf bytes.Buffer
	if err := RunSelectQuery(s, "select id, title where has(status)", &buf); err != nil {
		t.Fatalf("RunSelectQuery: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "WRKFL1") {
		t.Errorf("expected WRKFL1 in has(status) output:\n%s", out)
	}
	if strings.Contains(out, "PLAIN1") {
		t.Errorf("PLAIN1 should be filtered out by has(status):\n%s", out)
	}
}

// TestPhase10_FilepathFiltersAcrossNestedFolders proves the Phase 10 invariant
// that documents loaded from arbitrary nested subdirectories under `.doc/`
// can be filtered through ruki's `filepath` field. Without recursive loading
// and a correctly-populated FilePath, a substring predicate on `filepath`
// would never match, silently returning an empty result.
//
// The test seeds three documents at different depths and asserts that a
// substring predicate on `filepath` narrows the result set to just the
// matching folder.
//
// Cross-platform note: `filepath` is populated from `filepath.Abs`, which
// uses OS-native separators. The query fragment must therefore be built
// with `filepath.Join` so it matches backslashes on Windows and forward
// slashes on POSIX; a hard-coded `"projects/alpha"` literal would silently
// fail on Windows CI.
func TestPhase10_FilepathFiltersAcrossNestedFolders(t *testing.T) {
	setupRunnerTest(t)

	root := t.TempDir()
	writeDocForRuntime(t, filepath.Join(root, "FLAT01.md"), "FLAT01", "flat doc")
	writeDocForRuntime(t, filepath.Join(root, "projects", "alpha", "ALPHA1.md"), "ALPHA1", "alpha doc")
	writeDocForRuntime(t, filepath.Join(root, "projects", "beta", "BETA01.md"), "BETA01", "beta doc")

	t.Setenv("TIKI_STORE_GIT", "false")
	s, err := tikistore.NewTikiStore(root)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// On Windows, filepath.Join returns `projects\alpha`; a single backslash
	// inside a double-quoted ruki literal is consumed by the Go-style
	// unquote pass. Double-escape so the query sees the literal separator
	// that FilePath actually contains on this OS.
	needle := strings.ReplaceAll(filepath.Join("projects", "alpha"), `\`, `\\`)
	query := `select id where "` + needle + `" in filepath`

	var buf bytes.Buffer
	if err := RunSelectQuery(s, query, &buf); err != nil {
		t.Fatalf("RunSelectQuery: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ALPHA1") {
		t.Errorf("ALPHA1 missing — filepath substring filter did not match nested doc:\nquery: %s\nout:\n%s", query, out)
	}
	if strings.Contains(out, "FLAT01") {
		t.Errorf("FLAT01 matched a projects/alpha filter but lives at the root:\n%s", out)
	}
	if strings.Contains(out, "BETA01") {
		t.Errorf("BETA01 lives in projects/beta, must not match projects/alpha filter:\n%s", out)
	}
}

// writeDocForRuntime writes a minimal workflow doc at the given path, creating
// parent directories as needed. Used by Phase 10 filepath-filter tests that
// need real nested files on disk for tikistore to discover via its recursive
// walk.
func writeDocForRuntime(t *testing.T, path, id, title string) {
	t.Helper()
	//nolint:gosec // G301: 0755 matches the rest of the test suite's temp-dir perms
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	content := "---\nid: " + id + "\ntitle: " + title + "\ntype: story\nstatus: backlog\n---\nbody\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
