package view

import (
	"testing"

	"github.com/boolean-maybe/ruki"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/plugin"
)

// mustParseStmt parses and validates a ruki statement against the runtime
// schema, failing the test on error. Mirrors the controller-package helper.
func mustParseStmt(t *testing.T, input string) *ruki.ValidatedStatement {
	t.Helper()
	schema := rukiRuntime.NewSchema()
	parser := ruki.NewParser(schema)
	stmt, err := parser.ParseAndValidateStatement(input, ruki.ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("parse ruki statement %q: %v", input, err)
	}
	return stmt
}

func rukiGlobal(t *testing.T, label, stmt string) plugin.PluginAction {
	t.Helper()
	return plugin.PluginAction{
		Label:  label,
		Kind:   plugin.ActionKindRuki,
		Action: mustParseStmt(t, stmt),
	}
}

// hasLabel reports whether the surfaced set contains an action with the label.
func hasLabel(surfaced []plugin.PluginAction, label string) bool {
	for _, a := range surfaced {
		if a.Label == label {
			return true
		}
	}
	return false
}

// surfacedGlobalActions is the source of truth for the wiki header/palette.
// A tiki-selection ruki global must not appear there.
func TestSurfacedGlobalsHidesSelectionBuiltin(t *testing.T) {
	globals := []plugin.PluginAction{
		rukiGlobal(t, "Copy ID", `select id where id = id() | clipboard()`),
		rukiGlobal(t, "Copy content",
			`select title, description where filepath = filepath() | clipboard()`),
		rukiGlobal(t, "List all", `select id`),
	}
	surfaced := surfacedGlobalActions(globals, "Docs")
	if hasLabel(surfaced, "Copy ID") {
		t.Error("Copy ID (uses id()) should not surface on the wiki, but it did")
	}
	if hasLabel(surfaced, "Copy content") {
		t.Error("Copy content (uses filepath()) should not surface on the wiki, but it did")
	}
	if !hasLabel(surfaced, "List all") {
		t.Error("List all (no selection builtin) should surface on the wiki, but it did not")
	}
}

// viewGlobal builds a kind:view action targeting Detail with the given require list.
func viewGlobal(label string, require []string) plugin.PluginAction {
	return plugin.PluginAction{
		Label:      label,
		Kind:       plugin.ActionKindView,
		TargetView: "Detail",
		Require:    require,
	}
}

// TestSurfacedGlobalsHidesSelectionViewAction guards the real bug: a kind:view
// action that opens the SELECTED tiki (require: selection:one — e.g. Edit,
// Edit description, Edit tags) must NOT surface on the wiki, which has no
// selection. Only selection-free view actions (plain navigation / view-switch
// / New) belong there. Before the fix the view branch surfaced every kind:view
// action unconditionally, so the edit actions leaked onto Docs (greyed, but
// present) no matter how the workflow YAML was authored.
func TestSurfacedGlobalsHidesSelectionViewAction(t *testing.T) {
	globals := []plugin.PluginAction{
		viewGlobal("Edit", []string{"selection:one"}),
		viewGlobal("Edit description", []string{"selection:one"}),
		viewGlobal("Edit tags", []string{"selection:one"}),
		viewGlobal("New", nil), // no selection required — must survive
	}
	surfaced := surfacedGlobalActions(globals, "Docs")

	for _, label := range []string{"Edit", "Edit description", "Edit tags"} {
		if hasLabel(surfaced, label) {
			t.Errorf("%q requires a selection and must not surface on the wiki, but it did", label)
		}
	}
	if !hasLabel(surfaced, "New") {
		t.Error("New (no selection required) should surface on the wiki, but it did not")
	}
}

// TestWikiViewReportsNoSelection guards against tiki-selection actions lighting
// up on Docs. A kind:wiki view renders a document and has no selectable tiki,
// so it must report no selection even when a stale tiki id was carried in via
// PluginViewParams (e.g. switching to Docs from a board with a card selected).
// Otherwise built-in actions like <e> Edit appear enabled and act on the stale
// tiki.
func TestWikiViewReportsNoSelection(t *testing.T) {
	def := &plugin.WikiPlugin{
		BasePlugin:   plugin.BasePlugin{Name: "Docs", Kind: plugin.KindWiki},
		DocumentPath: "index.md",
	}
	// construct as the factory does when arriving with a carried selection.
	v := NewWikiView(def, nil, nil, nil, nil, "DLBKGY")

	if got := v.GetSelectedID(); got != "" {
		t.Errorf("wiki view must report no selection, got %q", got)
	}
}
