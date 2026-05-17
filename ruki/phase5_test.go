package ruki

import (
	"reflect"
	"strings"
	"testing"
)

// --- absent-field predicate semantics ---

// TestPhase5_AbsentScalarNotMatched exercises the headline Phase 5 rule:
// a plain document (no workflow frontmatter, no typed fields) must not
// match predicates that compare workflow fields to zero values. Without
// presence tracking, `where points = 0` used to match plain docs
// because Go's zero-int matched the literal 0.
// TestPhase4_AbsentScalarEqualsLiteralIsFalse pins the updated Phase-4
// rule: `missing = value` evaluates to false instead of hard-erroring.
// Only the tiki that has points=0 explicitly should match.
func TestPhase4_AbsentScalarEqualsLiteralIsFalse(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{ID: "PLAIN1", Title: "readme"} // no workflow fields
	workflowZero := &tikiFixture{
		ID: "WRKFL1", Title: "explicit zero", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{
			"status": "ready",
			"points": 0,
		},
	}

	stmt, err := p.ParseStatement(`select where points = 0`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain, workflowZero})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tikis) != 1 || result.Select.Tikis[0].ID != "WRKFL1" {
		gotIDs := tikiIDs(result.Select.Tikis)
		t.Fatalf("expected only WRKFL1 to match points=0, got %v", gotIDs)
	}
}

// TestPhase4_AbsentScalarNotEqualsLiteralIsTrue pins the symmetric rule:
// `missing != value` evaluates to true, so plain tikis satisfy the
// predicate "priority != 5" alongside any tiki whose priority is not 5.
func TestPhase4_AbsentScalarNotEqualsLiteralIsTrue(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{ID: "PLAIN1", Title: "readme"}
	workflow5 := &tikiFixture{
		ID: "WRKFL5", Title: "points 5", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"points": 5},
		Points:              5,
	}

	stmt, err := p.ParseStatement(`select where points != 5`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain, workflow5})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tikis) != 1 || result.Select.Tikis[0].ID != "PLAIN1" {
		gotIDs := tikiIDs(result.Select.Tikis)
		t.Fatalf("expected PLAIN1 (missing priority treated as != 5), got %v", gotIDs)
	}
}

// TestPhase4_AbsentScalarOrderingStillHardErrors pins that ordering
// comparisons (<, >, <=, >=) still require a present field.
func TestPhase4_AbsentScalarOrderingStillHardErrors(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{ID: "PLAIN1", Title: "readme"}

	stmt, err := p.ParseStatement(`select where points < 3`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := e.testExec(stmt, []*tikiFixture{plain}); err == nil {
		t.Fatal("expected hard-error on ordering comparison of absent priority")
	}
}

// --- has() presence predicate ---

func TestPhase5_HasPredicateDistinguishesAbsentFromZero(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{ID: "PLAIN1", Title: "readme"}
	workflowSet := &tikiFixture{
		ID: "WRKFL1", Title: "has status", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}
	workflowUnset := &tikiFixture{
		ID: "WRKFL2", Title: "no status",
		// workflow doc but status key absent
		WorkflowFrontmatter: map[string]interface{}{"points": 3},
		Points:              3,
	}

	stmt, err := p.ParseStatement(`select where has(status)`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain, workflowSet, workflowUnset})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	gotIDs := tikiIDs(result.Select.Tikis)
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

	plain := &tikiFixture{ID: "PLAIN1", Title: "note"}
	stmt, err := p.ParseStatement(`update where id = "PLAIN1" set status = "ready"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain})
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

	plain := &tikiFixture{ID: "PLAIN1", Title: "note"}
	stmt, err := p.ParseStatement(`update where id = "PLAIN1" set priority = "medium-high"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain})
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

	plain := &tikiFixture{ID: "PLAIN1", Title: "note"}
	stmt, err := p.ParseStatement(`update where id = "PLAIN1" set points = 3`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain})
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

	plain := &tikiFixture{ID: "PLAIN1", Title: "note"}
	stmt, err := p.ParseStatement(`update where id = "PLAIN1" set dependsOn = ["ABC123"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain})
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

	tikis := []*tikiFixture{
		{ID: "TIKI-000001", Title: "x", Status: "ready"},
	}
	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set dependsOn = ["TIKI-ABC"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := e.testExec(stmt, tikis); err == nil {
		t.Fatal("expected error for non-bare dependsOn id")
	} else if !strings.Contains(err.Error(), "bare document id") {
		t.Fatalf("expected bare-id error, got: %v", err)
	}
}

func TestPhase5_DependsOnAcceptsBareID(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	tikis := []*tikiFixture{
		{ID: "TIKI-000001", Title: "x", Status: "ready"},
	}
	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set dependsOn = ["ABC123"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, tikis)
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

// TestPhase4_AbsentListIsEmptyIsTrue pins the updated rule: `is empty`
// on an absent list returns true (absent is treated as empty).
func TestPhase4_AbsentListIsEmptyIsTrue(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{ID: "PLAIN1", Title: "no tags"}
	presentEmpty := &tikiFixture{
		ID: "EMPTY1", Title: "explicit empty tags", Status: "ready", Tags: []string{},
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "tags": ""},
	}
	withTags := &tikiFixture{
		ID: "TAGGED", Title: "tagged", Status: "ready", Tags: []string{"a"},
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "tags": ""},
	}

	stmt, err := p.ParseStatement(`select where tags is empty`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain, presentEmpty, withTags})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	gotIDs := tikiIDs(result.Select.Tikis)
	wantIDs := []string{"PLAIN1", "EMPTY1"}
	sortStrings(gotIDs)
	sortStrings(wantIDs)
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("tags is empty: got %v, want %v", gotIDs, wantIDs)
	}
}

// TestPhase4_AbsentListEqualsEmptyKeyword pins the rule: `missing = empty`
// (the empty keyword) follows is-empty semantics, so absent tags match.
// Compare with the list-literal form `tags = []` which is a concrete-
// value comparison and returns false for absent.
func TestPhase4_AbsentListEqualsEmptyKeyword(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{ID: "PLAIN1", Title: "no tags"}

	// `tags = empty` → true for absent tags (is-empty semantics).
	emptyKeyword, err := p.ParseStatement(`select where tags = empty`)
	if err != nil {
		t.Fatalf("parse empty keyword: %v", err)
	}
	result, err := e.testExec(emptyKeyword, []*tikiFixture{plain})
	if err != nil {
		t.Fatalf("execute empty keyword: %v", err)
	}
	if len(result.Select.Tikis) != 1 || result.Select.Tikis[0].ID != "PLAIN1" {
		t.Fatalf("tags = empty: got %v, want [PLAIN1]", tikiIDs(result.Select.Tikis))
	}

	// `tags = []` → false for absent tags (concrete-value compare).
	listLiteral, err := p.ParseStatement(`select where tags = []`)
	if err != nil {
		t.Fatalf("parse list literal: %v", err)
	}
	result, err = e.testExec(listLiteral, []*tikiFixture{plain})
	if err != nil {
		t.Fatalf("execute list literal: %v", err)
	}
	if len(result.Select.Tikis) != 0 {
		t.Fatalf("tags = []: got %v, want []", tikiIDs(result.Select.Tikis))
	}
}

// TestPhase4_AbsentListQuantifier pins that `all` over an absent list is
// vacuous-true (missing treated as empty) and `any` is false.
func TestPhase4_AbsentListQuantifier(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{ID: "PLAIN1", Title: "no deps"}

	// `all` over absent dependsOn is vacuously true.
	allStmt, err := p.ParseStatement(`select where dependsOn all status = "done"`)
	if err != nil {
		t.Fatalf("parse all: %v", err)
	}
	result, err := e.testExec(allStmt, []*tikiFixture{plain})
	if err != nil {
		t.Fatalf("execute all: %v", err)
	}
	if len(result.Select.Tikis) != 1 || result.Select.Tikis[0].ID != "PLAIN1" {
		t.Fatalf("all on absent deps: got %v, want [PLAIN1]", tikiIDs(result.Select.Tikis))
	}

	// `any` over absent dependsOn is false.
	anyStmt, err := p.ParseStatement(`select where dependsOn any status = "done"`)
	if err != nil {
		t.Fatalf("parse any: %v", err)
	}
	result, err = e.testExec(anyStmt, []*tikiFixture{plain})
	if err != nil {
		t.Fatalf("execute any: %v", err)
	}
	if len(result.Select.Tikis) != 0 {
		t.Fatalf("any on absent deps: got %v, want []", tikiIDs(result.Select.Tikis))
	}
}

// sortStrings sorts a []string in place for stable test assertions.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func TestPhase5_AbsentListHasPredicateWorks(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{ID: "PLAIN1", Title: "no deps"}
	presentEmpty := &tikiFixture{
		ID: "EMPTY1", Title: "explicit empty deps", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "dependsOn": ""},
	}

	stmt, err := p.ParseStatement(`select where has(dependsOn)`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain, presentEmpty})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	gotIDs := tikiIDs(result.Select.Tikis)
	wantIDs := []string{"EMPTY1"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("has(dependsOn): got %v, want %v (only the doc with explicit dependsOn key should match)", gotIDs, wantIDs)
	}
}

func TestPhase4_AbsentListBinaryAddAutoZeroes(t *testing.T) {
	// Phase 4 + assignment-RHS carve-out: `set dependsOn = dependsOn + "X"`
	// on a tiki without a prior dependsOn field auto-zeroes the absent read
	// to [] and produces ["X"]. Typos still hard-error at parse time.
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{ID: "PLAIN1", Title: "no deps"}
	stmt, err := p.ParseStatement(`update where id = "PLAIN1" set dependsOn = dependsOn + "ABC123"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated tiki, got %+v", result.Update)
	}
	got := result.Update.Updated[0].DependsOn
	if !reflect.DeepEqual(got, []string{"ABC123"}) {
		t.Errorf("dependsOn = %v, want [ABC123]", got)
	}

	// Pure literal assignment still works.
	literal, err := p.ParseStatement(`update where id = "PLAIN1" set dependsOn = ["ABC123"]`)
	if err != nil {
		t.Fatalf("parse literal: %v", err)
	}
	result, err = e.testExec(literal, []*tikiFixture{plain})
	if err != nil {
		t.Fatalf("execute literal: %v", err)
	}
	if len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated, got %d", len(result.Update.Updated))
	}
	u := result.Update.Updated[0]
	if len(u.DependsOn) != 1 || u.DependsOn[0] != "ABC123" {
		t.Errorf("expected dependsOn=[ABC123], got %v", u.DependsOn)
	}
}

// --- absent `in` / `not in` semantics ---
//
// Updated Phase-4 rule: missing LHS or missing collection treats the
// value as "not a member," so `in` returns false and `not in` returns
// true. Matches the = / != symmetry in the updated spec.

func TestPhase4_NotInAbsentScalarIsTrue(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{ID: "PLAIN1", Title: "no assignee"}
	stmt, err := p.ParseStatement(`select where assignee not in ["alice"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tikis) != 1 || result.Select.Tikis[0].ID != "PLAIN1" {
		t.Fatalf("assignee not in [...]: got %v, want [PLAIN1]", tikiIDs(result.Select.Tikis))
	}
}

func TestPhase4_InAbsentScalarIsFalse(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{ID: "PLAIN1", Title: "no assignee"}
	stmt, err := p.ParseStatement(`select where assignee in ["alice", "bob"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tikis) != 0 {
		t.Fatalf("assignee in [...]: got %v, want []", tikiIDs(result.Select.Tikis))
	}
}

func TestPhase4_NotInAbsentListIsTrue(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{ID: "PLAIN1", Title: "no tags"}
	stmt, err := p.ParseStatement(`select where "urgent" not in tags`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tikis) != 1 || result.Select.Tikis[0].ID != "PLAIN1" {
		t.Fatalf("\"urgent\" not in tags: got %v, want [PLAIN1]", tikiIDs(result.Select.Tikis))
	}
}

// --- absent-sort ordering ---

func TestPhase4_AbsentOrderByHardErrors(t *testing.T) {
	// Phase 4: `order by priority` on an input set containing a tiki
	// without priority hard-errors during sort-key extraction.
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{ID: "PLAIN1", Title: "no priority"}
	p1 := &tikiFixture{ID: "PRI001", Title: "p1", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready", "priority": 1}}

	stmt, err := p.ParseStatement(`select order by priority`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := e.testExec(stmt, []*tikiFixture{plain, p1}); err == nil {
		t.Fatal("expected hard-error on absent priority during order-by")
	}
}
