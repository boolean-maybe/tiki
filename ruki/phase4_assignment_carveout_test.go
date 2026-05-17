package ruki

import (
	"reflect"
	"testing"

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
	plain := &tikiFixture{
		ID: "ABC123", Title: "story", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}

	stmt, err := p.ParseStatement(`update where id = "ABC123" set tags = tags + ["urgent"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain})
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

// TestPhase4Carveout_IntArithmeticOnAbsentIntProducesValue covers the
// original int-arithmetic carve-out: `points - 1` on an absent points
// field computes 0 - 1 = -1. The executor no longer enforces value
// ranges — those invariants live in the mutation gate validator. The
// carve-out handles absent read → zero; the assignment succeeds
// generically and writes the negative value through.
//
// The earlier version of this test exercised `priority - 1` while
// priority was still an int field. After Phase 3 made priority a
// workflow enum, that statement no longer parses. This test was
// repointed to `points` to keep the int-arithmetic carve-out covered;
// the prev_enum-on-absent-priority behavior is exercised separately
// by TestPhase4Carveout_PrevEnumOnAbsentEnumProducesBoundary below.
func TestPhase4Carveout_IntArithmeticOnAbsentIntProducesValue(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{
		ID: "ABC123", Title: "story", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}

	stmt, err := p.ParseStatement(`update where id = "ABC123" set points = points - 1`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain})
	if err != nil {
		t.Fatalf("expected executor to apply generic assignment, got: %v", err)
	}
	if len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated, got %d", len(result.Update.Updated))
	}
	got := result.Update.Updated[0].Points
	if got != -1 {
		t.Errorf("points = %d, want -1 (absent → zero, then 0-1)", got)
	}
}

// TestPhase4Carveout_PrevEnumOnAbsentEnumProducesBoundary covers the
// enum-stepping side of the carve-out: prev_enum(field) on a tiki where
// the enum field is absent must clamp to the boundary (first allowed
// value for prev_enum, last for next_enum) rather than erroring or
// silently no-op'ing. This is what makes
// `update where id = id() set priority = prev_enum(priority)` work as
// the bound action of a "Priority up" hotkey, even when the user
// triggers it on a tiki that hasn't been assigned a priority yet.
func TestPhase4Carveout_PrevEnumOnAbsentEnumProducesBoundary(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{
		ID: "ABC123", Title: "story", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}

	stmt, err := p.ParseStatement(`update where id = "ABC123" set priority = prev_enum(priority)`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.testExec(stmt, []*tikiFixture{plain})
	if err != nil {
		t.Fatalf("expected executor to apply generic assignment, got: %v", err)
	}
	rawTk := result.raw.Update.Updated[0]
	got, _, _ := rawTk.StringField("priority")
	// testSchema declares priority as
	// [high, medium-high, medium, medium-low, low]; prev_enum on absent
	// clamps to the first value.
	if got != "high" {
		t.Errorf("priority = %q, want %q (prev_enum on absent clamps to first)", got, "high")
	}
}

// TestPhase4Carveout_UnregisteredFieldStillErrors pins the typo-safety
// guarantee: references to unregistered names (typos) stay hard-error.
// The parser rejects unregistered names at parse time, so the error
// surfaces before execution — either path proves the typo is caught.
func TestPhase4Carveout_UnregisteredFieldStillErrors(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	plain := &tikiFixture{
		ID: "ABC123", Title: "story", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}

	// `taggs` is not a registered workflow-declared or Custom field → error
	// either at parse time or execution time.
	stmt, err := p.ParseStatement(`update where id = "ABC123" set tags = taggs + ["oops"]`)
	if err != nil {
		return // parse-time rejection — acceptable
	}
	if _, err := e.testExec(stmt, []*tikiFixture{plain}); err == nil {
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

	stmt, err := p.ParseStatement(`select where points > 0`)
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
