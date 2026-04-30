package ruki

import (
	"reflect"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/task"
)

// --- absent-field predicate semantics ---

// TestPhase5_AbsentScalarNotMatched exercises the headline Phase 5 rule:
// a plain document (no workflow frontmatter, no typed fields) must not
// match predicates that compare workflow fields to zero values. Without
// presence tracking, `where priority = 0` used to match plain docs
// because Go's zero-int matched the literal 0.
func TestPhase5_AbsentScalarNotMatched(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "readme"} // no workflow fields
	workflowZero := &task.Task{
		ID: "WRKFL1", Title: "explicit zero", Status: "ready",
		// priority present but 0 — explicitly declared zero.
		WorkflowFrontmatter: map[string]interface{}{
			"status":   "ready",
			"priority": 0,
		},
	}

	stmt, err := p.ParseStatement(`select where priority = 0`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain, workflowZero})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 || result.Select.Tasks[0].ID != "WRKFL1" {
		gotIDs := taskIDs(result.Select.Tasks)
		t.Fatalf("expected only WRKFL1 (explicit priority=0) to match, got %v", gotIDs)
	}
}

// --- has() presence predicate ---

func TestPhase5_HasPredicateDistinguishesAbsentFromZero(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "readme"}
	workflowSet := &task.Task{
		ID: "WRKFL1", Title: "has status", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}
	workflowUnset := &task.Task{
		ID: "WRKFL2", Title: "no status",
		// workflow doc but status key absent
		WorkflowFrontmatter: map[string]interface{}{"priority": 3},
		Priority:            3,
	}

	stmt, err := p.ParseStatement(`select where has(status)`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain, workflowSet, workflowUnset})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	gotIDs := taskIDs(result.Select.Tasks)
	wantIDs := []string{"WRKFL1"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("has(status): got %v, want %v", gotIDs, wantIDs)
	}
}

func TestPhase5_HasPredicateRejectsStringLiteral(t *testing.T) {
	p := newTestParser()
	if _, err := p.ParseStatement(`select where has("status")`); err == nil {
		t.Fatal("expected parse/validate error: has() with string literal")
	} else if !strings.Contains(err.Error(), "has(") {
		t.Fatalf("expected error mentioning has(), got: %v", err)
	}
}

func TestPhase5_HasPredicateUnknownField(t *testing.T) {
	p := newTestParser()
	if _, err := p.ParseStatement(`select where has(nonexistent)`); err == nil {
		t.Fatal("expected error for has(unknown field)")
	}
}

// --- workflow promotion on set ---

func TestPhase5_SetStatusPromotesPlainToWorkflow(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "note"}
	stmt, err := p.ParseStatement(`update where id = "PLAIN1" set status = "ready"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated, got %d", len(result.Update.Updated))
	}
	u := result.Update.Updated[0]
	if !u.IsWorkflow {
		t.Error("setting status should promote plain doc to workflow")
	}
	if u.Status != "ready" {
		t.Errorf("status = %q, want ready", u.Status)
	}
}

func TestPhase5_SetPriorityPromotesPlainToWorkflow(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "note"}
	stmt, err := p.ParseStatement(`update where id = "PLAIN1" set priority = 2`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.Update.Updated[0].IsWorkflow {
		t.Error("setting priority should promote plain doc to workflow")
	}
}

func TestPhase5_SetPointsPromotesPlainToWorkflow(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "note"}
	stmt, err := p.ParseStatement(`update where id = "PLAIN1" set points = 3`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.Update.Updated[0].IsWorkflow {
		t.Error("setting points should promote plain doc to workflow")
	}
}

func TestPhase5_SetDependsOnPromotesPlainToWorkflow(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "note"}
	stmt, err := p.ParseStatement(`update where id = "PLAIN1" set dependsOn = ["ABC123"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.Update.Updated[0].IsWorkflow {
		t.Error("setting dependsOn should promote plain doc to workflow")
	}
}

// --- bare-ID validation at the ruki layer ---

func TestPhase5_DependsOnRejectsNonBareID(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready"},
	}
	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set dependsOn = ["TIKI-ABC"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := e.Execute(stmt, tasks); err == nil {
		t.Fatal("expected error for non-bare dependsOn id")
	} else if !strings.Contains(err.Error(), "bare document id") {
		t.Fatalf("expected bare-id error, got: %v", err)
	}
}

func TestPhase5_DependsOnAcceptsBareID(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready"},
	}
	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set dependsOn = ["ABC123"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	u := result.Update.Updated[0]
	if len(u.DependsOn) != 1 || u.DependsOn[0] != "ABC123" {
		t.Errorf("expected [ABC123], got %v", u.DependsOn)
	}
}

// --- absent-list predicate semantics ---
//
// The tests below pin down the rule that absent list workflow fields
// (tags, dependsOn) must fail every predicate except has(). Before the
// Phase 5 list fix, extractField returned []interface{}{} for absent
// list fields, letting `where tags is empty` / `where dependsOn = []`
// falsely match plain documents.

func TestPhase5_AbsentListNotIsEmpty(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "no tags"}
	presentEmpty := &task.Task{
		ID: "EMPTY1", Title: "explicit empty tags", Status: "ready", Tags: []string{},
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "tags": ""},
	}
	withTags := &task.Task{
		ID: "TAGGED", Title: "tagged", Status: "ready", Tags: []string{"a"},
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "tags": ""},
	}

	stmt, err := p.ParseStatement(`select where tags is empty`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain, presentEmpty, withTags})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	gotIDs := taskIDs(result.Select.Tasks)
	wantIDs := []string{"EMPTY1"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("tags is empty: got %v, want %v (absent tags on PLAIN1 must NOT match)", gotIDs, wantIDs)
	}
}

func TestPhase5_AbsentListNotEqualsEmptyLiteral(t *testing.T) {
	// tags is list<string>; using it avoids a list<ref>-vs-list<string>
	// type mismatch that the validator rejects for dependsOn.
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "no tags"}
	presentEmpty := &task.Task{
		ID: "EMPTY1", Title: "explicit empty tags", Status: "ready", Tags: []string{},
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "tags": ""},
	}

	stmt, err := p.ParseStatement(`select where tags = []`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain, presentEmpty})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	gotIDs := taskIDs(result.Select.Tasks)
	wantIDs := []string{"EMPTY1"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("tags = []: got %v, want %v (absent tags must NOT match [])", gotIDs, wantIDs)
	}
}

func TestPhase5_AbsentListQuantifierAllReturnsFalse(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	// Phase 5: `all` over an absent list is FALSE, not vacuously true.
	// Only a present-but-empty list keeps vacuous truth.
	plain := &task.Task{ID: "PLAIN1", Title: "no deps"}
	presentEmpty := &task.Task{
		ID: "EMPTY1", Title: "explicit empty deps", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "dependsOn": ""},
	}

	stmt, err := p.ParseStatement(`select where dependsOn all status = "done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain, presentEmpty})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	gotIDs := taskIDs(result.Select.Tasks)
	wantIDs := []string{"EMPTY1"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("all on absent dependsOn: got %v, want %v", gotIDs, wantIDs)
	}
}

func TestPhase5_AbsentListHasPredicateWorks(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "no deps"}
	presentEmpty := &task.Task{
		ID: "EMPTY1", Title: "explicit empty deps", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "dependsOn": ""},
	}

	stmt, err := p.ParseStatement(`select where has(dependsOn)`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain, presentEmpty})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	gotIDs := taskIDs(result.Select.Tasks)
	wantIDs := []string{"EMPTY1"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("has(dependsOn): got %v, want %v (only the doc with explicit dependsOn key should match)", gotIDs, wantIDs)
	}
}

func TestPhase5_AbsentListBinaryAddPromotes(t *testing.T) {
	// `set dependsOn = dependsOn + "X"` on a plain document must treat
	// the absent left-side dependsOn as [] for arithmetic so the result
	// is ["X"], not a "cannot add nil + string" error. This is the
	// canonical promotion idiom and must keep working after the list-
	// presence fix.
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "no deps"}
	stmt, err := p.ParseStatement(`update where id = "PLAIN1" set dependsOn = dependsOn + "ABC123"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated, got %d", len(result.Update.Updated))
	}
	u := result.Update.Updated[0]
	if len(u.DependsOn) != 1 || u.DependsOn[0] != "ABC123" {
		t.Errorf("expected dependsOn=[ABC123], got %v", u.DependsOn)
	}
	if !u.IsWorkflow {
		t.Error("set dependsOn should have promoted plain doc to workflow")
	}
}

// --- absent `in` / `not in` semantics ---
//
// These lock in that BOTH `in` and `not in` return false when either
// side references an absent workflow field. Before the fix, `not in`
// fell through to `c.Negated` and matched plain documents, which
// violated the plan's "predicates on absent fields evaluate false
// except explicit presence checks" rule.

func TestPhase5_NotInAbsentScalarDoesNotMatch(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "no assignee"}
	withAssignee := &task.Task{
		ID: "WITH01", Title: "bob", Status: "ready", Assignee: "bob",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "assignee": ""},
	}

	stmt, err := p.ParseStatement(`select where assignee not in ["alice"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain, withAssignee})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	gotIDs := taskIDs(result.Select.Tasks)
	wantIDs := []string{"WITH01"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("assignee not in [alice]: got %v, want %v (plain doc's absent assignee must NOT match)", gotIDs, wantIDs)
	}
}

func TestPhase5_InAbsentScalarDoesNotMatch(t *testing.T) {
	// Symmetric positive: `in` on an absent field is also false. The only
	// document that matches has an explicit present assignee that is in
	// the literal list.
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "no assignee"}
	withAssignee := &task.Task{
		ID: "WITH01", Title: "alice", Status: "ready", Assignee: "alice",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "assignee": ""},
	}

	stmt, err := p.ParseStatement(`select where assignee in ["alice", "bob"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain, withAssignee})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	gotIDs := taskIDs(result.Select.Tasks)
	wantIDs := []string{"WITH01"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("assignee in [alice,bob]: got %v, want %v", gotIDs, wantIDs)
	}
}

func TestPhase5_NotInAbsentListDoesNotMatch(t *testing.T) {
	// The right-hand operand is the absent workflow list this time.
	// Same rule: `not in` returns false so plain docs don't leak through.
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "no tags"}
	withTags := &task.Task{
		ID: "WITH01", Title: "tagged", Status: "ready", Tags: []string{"bug"},
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "tags": ""},
	}

	stmt, err := p.ParseStatement(`select where "urgent" not in tags`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain, withTags})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	gotIDs := taskIDs(result.Select.Tasks)
	wantIDs := []string{"WITH01"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("urgent not in tags: got %v, want %v (plain doc's absent tags must NOT match)", gotIDs, wantIDs)
	}
}

// --- absent-sort ordering ---

func TestPhase5_AbsentValueSortsLast(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{ID: "PLAIN1", Title: "no priority"}
	p1 := &task.Task{ID: "PRI001", Title: "p1", Status: "ready", Priority: 1,
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "priority": 1}}
	p3 := &task.Task{ID: "PRI003", Title: "p3", Status: "ready", Priority: 3,
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "priority": 3}}

	stmt, err := p.ParseStatement(`select order by priority`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{plain, p3, p1})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	gotIDs := taskIDs(result.Select.Tasks)
	wantIDs := []string{"PRI001", "PRI003", "PLAIN1"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("ascending: got %v, want %v", gotIDs, wantIDs)
	}
}
