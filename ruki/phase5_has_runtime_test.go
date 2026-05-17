package ruki

import (
	"strings"
	"testing"
)

// These tests lock in that the has() runtime resolves every qualifier
// the validator accepts, so `has(outer.X)` inside subqueries and
// `has(target.X)` / `has(targets.X)` in plugin actions execute without
// "unknown qualifier" runtime errors.

// --- has(outer.X) inside a subquery ---

func TestPhase5_Has_OuterQualifierResolvesParentRow(t *testing.T) {
	// `select where exists(select where has(outer.status))` must evaluate
	// has() against each row's parent-query candidate. Before the fix,
	// the validator allowed outer. but evalHas errored with "unknown
	// qualifier". Verify the query now runs and the exists() subquery
	// returns true for the row whose parent task has an explicit status.
	e := newTestExecutor()
	p := newTestParser()

	workflow := &tikiFixture{
		ID: "WKF01", Title: "with status", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}
	plain := &tikiFixture{ID: "PLN01", Title: "plain, no status"}

	// exists() with a trivial body that references the outer row's
	// status presence. Matches WKF01 only.
	stmt, err := p.ParseStatement(`select where exists(select where has(outer.status))`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{workflow, plain})
	if err != nil {
		t.Fatalf("execute: %v (should run, not error on outer. qualifier)", err)
	}
	gotIDs := tikiIDs(result.Select.Tasks)
	wantIDs := []string{"WKF01"}
	if len(gotIDs) != len(wantIDs) || gotIDs[0] != wantIDs[0] {
		t.Fatalf("has(outer.status): got %v, want %v", gotIDs, wantIDs)
	}
}

// --- has(target.X) in plugin runtime ---

func TestPhase5_Has_TargetQualifierResolvesSelectedTask(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`select where has(target.status)`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("parse/validate: %v", err)
	}

	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})

	// Two tasks: the selected one has an explicit status, the other
	// doesn't. has(target.status) is evaluated once per row and is
	// constant across rows (it reads the selected task, not the current
	// row), so if the selected task has status, ALL rows match.
	selected := &tikiFixture{
		ID: "SEL01", Title: "selected", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}
	other := &tikiFixture{ID: "OTH01", Title: "other"}

	result, err := e.testExec(vs, []*tikiFixture{selected, other}, ExecutionInput{
		SelectedTaskIDs: []string{"SEL01"},
	})
	if err != nil {
		t.Fatalf("execute: %v (should resolve target. qualifier)", err)
	}
	if len(result.Select.Tasks) != 2 {
		t.Fatalf("has(target.status) with selected=SEL01(status present): expected 2 matches, got %d", len(result.Select.Tasks))
	}
}

func TestPhase5_Has_TargetQualifierFalseWhenSelectedLacksField(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`select where has(target.status)`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("parse/validate: %v", err)
	}

	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	selected := &tikiFixture{ID: "SEL01", Title: "selected, no status"}
	other := &tikiFixture{
		ID: "OTH01", Title: "other has status", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}

	result, err := e.testExec(vs, []*tikiFixture{selected, other}, ExecutionInput{
		SelectedTaskIDs: []string{"SEL01"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 0 {
		t.Fatalf("has(target.status) with selected=SEL01(absent status): expected 0 matches, got %d", len(result.Select.Tasks))
	}
}

// --- has(targets.X) in plugin runtime ---

func TestPhase5_Has_TargetsQualifierTrueWhenAnySelectedHasField(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`select where has(targets.status)`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("parse/validate: %v", err)
	}

	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})

	// Two selected tasks: one has status, one doesn't. `has(targets.X)`
	// is any-present semantics, so this evaluates true.
	withStatus := &tikiFixture{
		ID: "WITH01", Title: "has status", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}
	withoutStatus := &tikiFixture{ID: "WITHOUT01", Title: "no status"}
	bystander := &tikiFixture{ID: "BYST01", Title: "bystander"}

	result, err := e.testExec(vs, []*tikiFixture{withStatus, withoutStatus, bystander}, ExecutionInput{
		SelectedTaskIDs: []string{"WITH01", "WITHOUT01"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// any-present → true for every row.
	if len(result.Select.Tasks) != 3 {
		t.Fatalf("expected 3 rows to match (any-present), got %d", len(result.Select.Tasks))
	}
}

func TestPhase5_Has_TargetsQualifierFalseWhenNoneHaveField(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`select where has(targets.status)`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("parse/validate: %v", err)
	}

	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	plain1 := &tikiFixture{ID: "PLN01", Title: "p1"}
	plain2 := &tikiFixture{ID: "PLN02", Title: "p2"}
	bystander := &tikiFixture{
		ID: "BYST01", Title: "has status but not selected", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}

	result, err := e.testExec(vs, []*tikiFixture{plain1, plain2, bystander}, ExecutionInput{
		SelectedTaskIDs: []string{"PLN01", "PLN02"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 0 {
		t.Fatalf("expected 0 rows to match (no selected task has status), got %d", len(result.Select.Tasks))
	}
}

func TestPhase5_Has_TargetsQualifierFalseWhenNothingSelected(t *testing.T) {
	// Zero selections: has(targets.X) trivially false.
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`select where has(targets.status)`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("parse/validate: %v", err)
	}

	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	plain := &tikiFixture{ID: "PLN01", Title: "p"}

	result, err := e.testExec(vs, []*tikiFixture{plain}, ExecutionInput{
		// no SelectedTaskIDs
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 0 {
		t.Fatalf("expected 0 matches for has(targets.X) with zero selection, got %d", len(result.Select.Tasks))
	}
}

// --- has(outer.X) outside a subquery errors clearly ---

func TestPhase5_Has_OuterOutsideSubqueryErrors(t *testing.T) {
	// `has(outer.X)` is only valid inside a subquery body. Validator
	// rejects it at the top level because allowOuter is not set outside
	// subqueries. Double-check: the parse-level error is the right
	// one, not a later runtime error.
	p := newTestParser()
	_, err := p.ParseStatement(`select where has(outer.status)`)
	if err == nil {
		t.Fatal("expected parse error: has(outer.*) at top level")
	}
	if !strings.Contains(err.Error(), "outer") {
		t.Fatalf("expected outer-qualifier error, got: %v", err)
	}
}
