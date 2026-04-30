package ruki

import (
	"testing"

	"github.com/boolean-maybe/tiki/task"
)

// These tests pin down the Phase 5 fix for qualified has() in trigger
// contexts. Before the fix, the trigger executor did not override has(),
// so the base evalHas ran with ctx.current pointing at a sentinel task
// and ctx.outer nil — making has(old.*) always false and has(new.*)
// sometimes read the wrong row inside subqueries.

// --- has(new.<field>) ---

func TestPhase5_Trigger_HasNewFieldTrueWhenPresent(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// Canonical "auto-assign on create when no assignee was supplied"
	// expressed via has(new.assignee). The guard should MATCH (and the
	// deny would fire) only when the new task has no assignee field.
	trig, err := p.ParseTrigger(`before create where not has(new.assignee) deny "needs assignee"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	withAssignee := &task.Task{
		ID: "HAS001", Assignee: "alice",
		WorkflowFrontmatter: map[string]interface{}{"assignee": ""},
	}
	tc := &TriggerContext{Old: nil, New: withAssignee}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("guard `not has(new.assignee)` should NOT match when assignee is present")
	}
}

func TestPhase5_Trigger_HasNewFieldFalseWhenAbsent(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before create where not has(new.assignee) deny "needs assignee"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// No WorkflowFrontmatter, typed Assignee zero → absent.
	plain := &task.Task{ID: "PLAIN1"}
	tc := &TriggerContext{Old: nil, New: plain}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard `not has(new.assignee)` should match when assignee is absent on the new task")
	}
}

// --- has(old.<field>) ---
//
// The key regression test: pre-fix, has(old.*) ALWAYS returned false
// because the base evalHas read ctx.outer (nil for a guard), not tc.Old.

func TestPhase5_Trigger_HasOldFieldTrueWhenPresent(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where has(old.status) deny "had status"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	oldTask := &task.Task{
		ID: "TK01", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}
	newTask := &task.Task{ID: "TK01", Status: "done"}
	tc := &TriggerContext{Old: oldTask, New: newTask}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard `has(old.status)` should match when the old task carried a status")
	}
}

func TestPhase5_Trigger_HasOldFieldFalseWhenAbsent(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where has(old.assignee) deny "had assignee"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	oldTask := &task.Task{ID: "TK01"} // no assignee declared
	newTask := &task.Task{ID: "TK01", Assignee: "alice"}
	tc := &TriggerContext{Old: oldTask, New: newTask}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("guard `has(old.assignee)` should NOT match when the old task had no assignee")
	}
}

// --- has(old.*) distinguished from has(new.*) ---
//
// Proves the two qualifiers route to the right TriggerContext side even
// in the same guard, which is the scenario where pre-fix behavior was
// most broken: has(old.X) was a flat false regardless of tc.Old.

func TestPhase5_Trigger_HasOldAndHasNewResolveDistinctTasks(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// Guard should match when a status field is being ADDED: absent on
	// old, present on new. Pre-fix this guard would never match because
	// has(old.status) was always false but has(new.status) was also
	// sometimes wrong.
	trig, err := p.ParseTrigger(`before update where not has(old.status) and has(new.status) deny "adding status"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	oldTask := &task.Task{ID: "TK01"} // no status key
	newTask := &task.Task{
		ID: "TK01", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}
	tc := &TriggerContext{Old: oldTask, New: newTask}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard should match: old has no status, new does")
	}

	// Flip it — both sides now have status, guard should NOT match.
	oldTask2 := &task.Task{
		ID: "TK02", Status: "ready",
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}
	newTask2 := &task.Task{
		ID: "TK02", Status: "done",
		WorkflowFrontmatter: map[string]interface{}{"status": "done"},
	}
	tc2 := &TriggerContext{Old: oldTask2, New: newTask2}
	ok, err = te.EvalGuard(trig, tc2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("guard should NOT match: old already had status")
	}
}
