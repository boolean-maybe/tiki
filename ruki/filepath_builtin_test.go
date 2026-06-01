package ruki

import (
	"errors"
	"strings"
	"testing"
)

// twoFilepathFixtures returns two fixtures with deterministic FilePath values
// so executor evaluation has something to read for filepath() / filepaths().
func twoFilepathFixtures() []*tikiFixture {
	return []*tikiFixture{
		{ID: "TIKI-000001", Title: "alpha", Status: "ready", Type: "tiki", FilePath: "/repo/.doc/TIKI-000001.md"},
		{ID: "TIKI-000002", Title: "beta", Status: "ready", Type: "tiki", FilePath: "/repo/.doc/TIKI-000002.md"},
	}
}

func TestFilepath_SingleSelection_MatchesByPath(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`select where filepath = filepath()`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !vs.UsesFilepathBuiltin() {
		t.Fatal("expected UsesFilepathBuiltin() = true")
	}

	e := NewExecutor(testSchema{}, testDocFactory(), nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	result, err := e.testExec(vs, twoFilepathFixtures(), ExecutionInput{
		SelectedTikiIDs: []string{"TIKI-000001"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Select == nil || len(result.Select.Tikis) != 1 {
		t.Fatalf("expected 1 row, got %+v", result.Select)
	}
	if got := result.Select.Tikis[0].ID; got != "TIKI-000001" {
		t.Errorf("matched id = %q, want TIKI-000001", got)
	}
}

func TestFilepath_ZeroSelection_Errors(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`select where filepath = filepath()`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	e := NewExecutor(testSchema{}, testDocFactory(), nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	_, err = e.testExec(vs, twoFilepathFixtures(), ExecutionInput{})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	var merr *MissingSelectedTikiIDError
	if !errors.As(err, &merr) {
		t.Fatalf("error type = %T, want *MissingSelectedTikiIDError", err)
	}
	// activation predicate fires first (before evalFilepath); message names id()
	// because the predicate uses checkSingleSelectionForID. The per-builtin
	// message would only surface if the predicate were bypassed.
	if !strings.Contains(merr.Error(), "id() is used") {
		t.Errorf("message = %q, want it to mention id()", merr.Error())
	}
}

func TestFilepath_MultiSelection_Ambiguous(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`select where filepath = filepath()`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	e := NewExecutor(testSchema{}, testDocFactory(), nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	_, err = e.testExec(vs, twoFilepathFixtures(), ExecutionInput{
		SelectedTikiIDs: []string{"TIKI-000001", "TIKI-000002"},
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	var aerr *AmbiguousSelectedTikiIDError
	if !errors.As(err, &aerr) {
		t.Fatalf("error type = %T, want *AmbiguousSelectedTikiIDError", err)
	}
	if aerr.Count != 2 {
		t.Errorf("Count = %d, want 2", aerr.Count)
	}
}

// TestFilepath_DirectEval_NamesBuiltin exercises evalFilepath directly so the
// per-builtin error name is observable — the activation predicate uses the
// generic "id()" message, but evalFilepath itself constructs an error tagged
// with BuiltinName == "filepath".
func TestFilepath_DirectEval_NamesBuiltin(t *testing.T) {
	e := NewExecutor(testSchema{}, testDocFactory(), nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	e.currentInput = ExecutionInput{} // zero selection
	_, err := e.evalFilepath(evalContext{allTikis: tikisFromFixtures(twoFilepathFixtures())})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	var merr *MissingSelectedTikiIDError
	if !errors.As(err, &merr) {
		t.Fatalf("error type = %T, want *MissingSelectedTikiIDError", err)
	}
	if merr.BuiltinName != "filepath" {
		t.Errorf("BuiltinName = %q, want %q", merr.BuiltinName, "filepath")
	}
	if !strings.Contains(merr.Error(), "filepath() is used") {
		t.Errorf("message = %q, want it to mention filepath()", merr.Error())
	}

	e.currentInput = ExecutionInput{SelectedTikiIDs: []string{"TIKI-000001", "TIKI-000002"}}
	_, err = e.evalFilepath(evalContext{allTikis: tikisFromFixtures(twoFilepathFixtures())})
	if err == nil {
		t.Fatal("want error for ambiguous, got nil")
	}
	var aerr *AmbiguousSelectedTikiIDError
	if !errors.As(err, &aerr) {
		t.Fatalf("error type = %T, want *AmbiguousSelectedTikiIDError", err)
	}
	if aerr.BuiltinName != "filepath" || aerr.PluralName != "filepaths" {
		t.Errorf("names = (%q, %q), want (filepath, filepaths)", aerr.BuiltinName, aerr.PluralName)
	}
	if !strings.Contains(aerr.Error(), "use filepaths() for multi-selection") {
		t.Errorf("message = %q, want it to suggest filepaths()", aerr.Error())
	}
}

func TestFilepaths_ZeroSelection_ReturnsEmpty(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`select where filepath in filepaths()`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !vs.UsesFilepathsBuiltin() {
		t.Fatal("expected UsesFilepathsBuiltin() = true")
	}

	e := NewExecutor(testSchema{}, testDocFactory(), nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	result, err := e.testExec(vs, twoFilepathFixtures(), ExecutionInput{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Select == nil {
		t.Fatal("expected select result, got nil")
	}
	if got := len(result.Select.Tikis); got != 0 {
		t.Errorf("rows = %d, want 0 (empty filepaths() should match no tikis)", got)
	}
}

func TestFilepaths_MultiSelection_ReturnsAll(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`select where filepath in filepaths()`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	e := NewExecutor(testSchema{}, testDocFactory(), nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	result, err := e.testExec(vs, twoFilepathFixtures(), ExecutionInput{
		SelectedTikiIDs: []string{"TIKI-000001", "TIKI-000002"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Select == nil || len(result.Select.Tikis) != 2 {
		t.Fatalf("expected 2 rows, got %+v", result.Select)
	}
}
