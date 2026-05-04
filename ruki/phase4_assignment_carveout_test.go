package ruki

import (
	"reflect"
	"testing"

	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/tiki"
)

// TestPhase4Carveout_ListArithmeticOnAbsentTags covers the headline
// motivation for the assignment-RHS auto-zero carve-out: `tags + [x]`
// on a tiki without a tags field should succeed and produce [x], not
// hard-error as a bare read would.
func TestPhase4Carveout_ListArithmeticOnAbsentTags(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	// workflow doc, no tags set.
	plain := &task.Task{
		ID: "ABC123", Title: "story", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}

	stmt, err := p.ParseStatement(`update where id = "ABC123" set tags = tags + ["urgent"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*task.Task{plain})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated task, got %+v", result.Update)
	}
	got := result.Update.Updated[0].Tags
	if !reflect.DeepEqual(got, []string{"urgent"}) {
		t.Errorf("tags = %v, want [urgent]", got)
	}
}

// TestPhase4Carveout_IntArithmeticOnAbsentPriority covers the scalar
// case: `priority - 1` on an absent priority should compute 0 - 1 = -1,
// which then fails setField's range validation with a clear error.
// The carve-out handles absent read → zero; value validation handles
// out-of-range.
func TestPhase4Carveout_IntArithmeticOnAbsentPriorityFailsValidation(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{
		ID: "ABC123", Title: "story", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}

	stmt, err := p.ParseStatement(`update where id = "ABC123" set priority = priority - 1`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = e.testExec(stmt, []*task.Task{plain})
	if err == nil {
		t.Fatal("expected setField validation error for priority=-1, got nil")
	}
}

// TestPhase4Carveout_UnregisteredFieldStillErrors pins the typo-safety
// guarantee: references to unregistered names (typos) stay hard-error.
// The parser rejects unregistered names at parse time, so the error
// surfaces before execution — either path proves the typo is caught.
func TestPhase4Carveout_UnregisteredFieldStillErrors(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &task.Task{
		ID: "ABC123", Title: "story", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}

	// `taggs` is not a registered schema-known or Custom field → error
	// either at parse time or execution time.
	stmt, err := p.ParseStatement(`update where id = "ABC123" set tags = taggs + ["oops"]`)
	if err != nil {
		return // parse-time rejection — acceptable
	}
	if _, err := e.testExec(stmt, []*task.Task{plain}); err == nil {
		t.Fatal("expected hard-error on unregistered field 'taggs', got nil")
	}
}

// TestPhase4Carveout_WhereClauseStillHardErrors confirms the carve-out
// does NOT leak into WHERE clauses. An absent priority read in a WHERE
// clause still errors the query. Uses a directly-constructed Tiki to
// bypass the test helper's full-schema presence convention.
func TestPhase4Carveout_WhereClauseStillHardErrors(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	// Truly-absent priority: Fields map holds only status.
	sparse := &tiki.Tiki{
		ID:     "ABC123",
		Title:  "story",
		Fields: map[string]interface{}{"status": "ready"},
	}

	stmt, err := p.ParseStatement(`select where priority > 0`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := e.Execute(stmt, []*tiki.Tiki{sparse}); err == nil {
		t.Fatal("expected hard-error in WHERE clause (carve-out must not apply), got nil")
	}
}

// TestPhase4Carveout_PlainReferenceInAssignment pins the broader rule:
// a plain reference (not wrapped in arithmetic) to an absent registered
// field on the RHS of assignment also auto-zeroes. Without this,
// `set priority = priority` on a priority-less tiki would hard-error.
// Uses a directly-constructed Tiki so presence is truly sparse.
func TestPhase4Carveout_PlainReferenceInAssignment(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	sparse := &tiki.Tiki{
		ID:     "ABC123",
		Title:  "story",
		Fields: map[string]interface{}{"status": "ready"},
	}

	// `set points = points` on an absent-points tiki: carve-out auto-
	// zeroes RHS to 0, setField accepts 0 for points (valid range is
	// 0..maxPoints), result lands as points=0.
	stmt, err := p.ParseStatement(`update where id = "ABC123" set points = points`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*tiki.Tiki{sparse})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated tiki, got %+v", result.Update)
	}
	got, _ := result.Update.Updated[0].Get("points")
	if got != 0 {
		t.Errorf("points = %v (%T), want 0", got, got)
	}
	_ = reflect.DeepEqual
}
