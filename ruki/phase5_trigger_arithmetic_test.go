package ruki

import (
	"testing"

	"github.com/boolean-maybe/tiki/task"
)

// --- trigger BinaryExpr with absent list ---
//
// The base executor treats absent workflow list fields as empty for
// arithmetic contexts (Phase 5 promotion idiom). The trigger executor
// has its own BinaryExpr path that previously skipped this coercion,
// so `set tags = tags + ["auto"]` inside a trigger action failed with
// `cannot add <nil> + []interface{}` on plain tasks.

func TestPhase5_Trigger_SetTagsPlusListOnPlainTask(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// Canonical "auto-tag on create" trigger. Target task has no tags
	// frontmatter at all.
	trig, err := p.ParseTrigger(`after create update where id = new.id set tags = tags + ["auto"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	plain := &task.Task{ID: "PLAIN1", Title: "fresh task"}
	tc := &TriggerContext{Old: nil, New: plain, AllTasks: []*task.Task{plain}}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated task, got %+v", result.Update)
	}
	u := result.Update.Updated[0]
	if len(u.Tags) != 1 || u.Tags[0] != "auto" {
		t.Errorf("expected tags=[auto], got %v", u.Tags)
	}
	if !u.IsWorkflow {
		t.Error("setting tags should have promoted plain doc to workflow")
	}
}

func TestPhase5_Trigger_SetDependsOnPlusOldIdOnPlainTask(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// Cross-task trigger: on delete, append the deleted id into the
	// downstream task's dependsOn. Downstream has no prior dependsOn.
	trig, err := p.ParseTrigger(`after delete update where id = "DOWN01" set dependsOn = dependsOn + old.id`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	deleted := &task.Task{ID: "DEL001", Title: "gone"}
	downstream := &task.Task{ID: "DOWN01", Title: "plain downstream"} // no dependsOn
	tc := &TriggerContext{
		Old:      deleted,
		New:      nil,
		AllTasks: []*task.Task{downstream},
	}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated task, got %+v", result.Update)
	}
	u := result.Update.Updated[0]
	if len(u.DependsOn) != 1 || u.DependsOn[0] != "DEL001" {
		t.Errorf("expected dependsOn=[DEL001], got %v", u.DependsOn)
	}
	if !u.IsWorkflow {
		t.Error("setting dependsOn should have promoted plain doc to workflow")
	}
}

// --- trigger next_date on absent recurrence ---
//
// Base executor propagates nil for next_date() when recurrence is
// absent. The trigger override previously errored, so a guard like
// `where next_date(new.recurrence) is not empty` aborted the trigger
// instead of returning false.

func TestPhase5_Trigger_NextDateAbsentRecurrenceIsNotEmptyFalse(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where next_date(new.recurrence) is not empty deny "recurring"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Plain task — recurrence absent.
	plain := &task.Task{ID: "PLAIN1"}
	tc := &TriggerContext{Old: plain, New: plain}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error (should propagate nil, not error): %v", err)
	}
	if ok {
		t.Fatal("guard should NOT match: absent recurrence means next_date() is nil, `is not empty` is false")
	}
}

func TestPhase5_Trigger_NextDatePresentRecurrenceStillWorks(t *testing.T) {
	// Regression guard for the positive path: the Phase 5 fix must not
	// break the normal "there is a recurrence, next_date returns a real
	// date" flow.
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where next_date(new.recurrence) is not empty deny "recurring"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	recurring := &task.Task{
		ID: "REC01", Recurrence: task.RecurrenceDaily,
		WorkflowFrontmatter: map[string]interface{}{"recurrence": ""},
	}
	tc := &TriggerContext{Old: recurring, New: recurring}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard should match: present daily recurrence yields a real next_date")
	}
}
