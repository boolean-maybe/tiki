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
