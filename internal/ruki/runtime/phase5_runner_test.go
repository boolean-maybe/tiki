package runtime

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/store"
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
