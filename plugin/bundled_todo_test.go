package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/workflow"
)

// todoSchemaFields mirrors the simplified bundled todo workflow's `fields:`
// block. The plugin package's testinit_test.go installs the canonical
// (kanban-shaped) field set globally, so this test must temporarily swap
// it out and restore it on cleanup. Without this, the lane filter
// `where status = "todo"` fails validation because the canonical schema
// only knows status values like "backlog"/"ready"/"done".
func todoSchemaFields() []workflow.FieldDef {
	return []workflow.FieldDef{
		{
			Name: "status",
			Type: workflow.TypeEnum,
			EnumValues: []workflow.EnumValue{
				{Value: "todo", Label: "Todo", Visual: "📌", Default: true},
				{Value: "done", Label: "Done", Visual: "✅"},
			},
		},
		{Name: "priority", Type: workflow.TypeInt, DefaultValue: 3},
	}
}

// TestBundledTodo_HasSimplifiedShape asserts the simplified shape of the
// bundled todo workflow:
//   - exactly two fields: status (enum with todo/done) and priority
//   - exactly one board view named "Todo" with one lane named "Tasks"
//   - no detail view (the runtime gates `n`/ActionNewTiki on RequireDetailPlugin)
//   - five global actions: Quick create (Ctrl-Q), Mark done (m),
//     Mark todo (M), Bump priority (+), Lower priority (-)
//   - lane filter has an explicit order-by clause; raw YAML contains
//     "order by status desc, priority, updatedAt desc"
//   - layout: id, title (text.secondary), priority — no type.visual prefix
//
// The test combines parsed-structure assertions (for things the loader
// surfaces as typed values) with raw-YAML text assertions (for things
// like layout strings and order-by text that aren't easily reconstructed
// from the parsed AST).
func TestBundledTodo_HasSimplifiedShape(t *testing.T) {
	// Swap the global schema to match the bundled todo workflow's own fields.
	// Restore the canonical (kanban-shaped) schema on cleanup so other tests
	// in this package see the schema they expect.
	config.ResetWorkflowFieldsForTest(todoSchemaFields())
	t.Cleanup(func() {
		config.ResetWorkflowFieldsForTest(teststatuses.CanonicalFields())
	})

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	src := filepath.Join(repoRoot, "config", "workflows", "todo.yaml")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("bundled todo not at expected path %s: %v", src, err)
	}

	plugins, globals, errs := loadPluginsFromFile(src, testSchema())
	if len(errs) != 0 {
		t.Fatalf("bundled todo did not load cleanly: %v", errs)
	}

	// No detail view should be present.
	for _, p := range plugins {
		if dp, ok := p.(*DetailPlugin); ok {
			t.Fatalf("bundled todo must not declare a detail view, found: %q", dp.Name)
		}
	}

	// Exactly one WorkflowPlugin (board view) named "Todo".
	var board *WorkflowPlugin
	for _, p := range plugins {
		if tp, ok := p.(*WorkflowPlugin); ok {
			if board != nil {
				t.Fatalf("bundled todo must have exactly one board view, found multiple")
			}
			board = tp
		}
	}
	if board == nil {
		t.Fatal("bundled todo missing the Todo board view")
	}
	if board.Name != "Todo" {
		t.Errorf("board view name: got %q, want %q", board.Name, "Todo")
	}

	// Exactly one lane named "Tasks".
	if len(board.Lanes) != 1 {
		t.Fatalf("lane count: got %d, want 1", len(board.Lanes))
	}
	lane := board.Lanes[0]
	if lane.Name != "Tasks" {
		t.Errorf("lane name: got %q, want %q", lane.Name, "Tasks")
	}
	if lane.Columns != 1 {
		t.Errorf("lane columns: got %d, want 1 (single-column checklist)", lane.Columns)
	}

	// Lane filter must parse and must declare an explicit order-by clause.
	if lane.Filter == nil {
		t.Fatal("lane filter did not parse")
	}
	if !lane.Filter.HasOrderBy() {
		t.Error("lane filter must declare an explicit order-by clause")
	}

	// Globals: assert the five expected keys are present with expected labels.
	wantKeys := map[string]string{
		"Ctrl-Q": "Quick create",
		"m":      "Mark done",
		"M":      "Mark todo",
		"+":      "Bump priority",
		"-":      "Lower priority",
	}
	gotKeys := map[string]string{}
	for _, g := range globals {
		gotKeys[g.KeyStr] = g.Label
	}
	for k, wantLabel := range wantKeys {
		gotLabel, ok := gotKeys[k]
		if !ok {
			t.Errorf("missing global action with key %q (have: %v)", k, gotKeys)
			continue
		}
		if gotLabel != wantLabel {
			t.Errorf("action %q label: got %q, want %q", k, gotLabel, wantLabel)
		}
	}

	// Forbidden keys must not appear (these were removed by this change).
	forbiddenKeys := []string{"y", "Y", "t", "T"}
	for _, k := range forbiddenKeys {
		if _, ok := gotKeys[k]; ok {
			t.Errorf("action %q must not exist after simplification", k)
		}
	}

	// Raw-YAML assertions: things the parser hides behind opaque AST nodes.
	rawBytes, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read todo.yaml: %v", err)
	}
	raw := string(rawBytes)

	// Layout must NOT reference type.visual (type field was removed).
	if strings.Contains(raw, "type.visual") {
		t.Error("todo.yaml still references removed type.visual in layout or elsewhere")
	}

	// Tags-related text must not appear (field and actions removed).
	for _, frag := range []string{"name: tags", "Add tag", "Remove tag", "Copy ID", "Copy content"} {
		if strings.Contains(raw, frag) {
			t.Errorf("todo.yaml still contains removed fragment %q", frag)
		}
	}

	// Lane filter must contain the expected order-by clause.
	wantOrderBy := "order by status desc, priority, updatedAt desc"
	if !strings.Contains(raw, wantOrderBy) {
		t.Errorf("todo.yaml lane filter missing %q", wantOrderBy)
	}

	// The lane must scope to workflow docs (has(status)), not all documents.
	// Plain doki templates (e.g. index.md, linked.md created during bootstrap)
	// have no status field; an unscoped `where true` causes the order-by to
	// error on those rows and the whole ordered select returns zero rows.
	if !strings.Contains(raw, "has(status)") {
		t.Error("todo.yaml lane filter must use has(status) to skip plain documents")
	}
}
