package ruki

import (
	"testing"

	"github.com/boolean-maybe/tiki/task"
)

// --- Phase 4 + assignment-RHS carve-out: trigger arithmetic on absent list ---
//
// Under Phase 4's carve-out, `tags + ["auto"]` on a task with no prior
// tags field auto-zeroes the absent read to [] inside an update-set RHS.
// The trigger succeeds and produces tags=["auto"].

func TestPhase4_Trigger_SetTagsPlusListOnPlainTaskAutoZeroes(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`after create update where id = new.id set tags = tags + ["auto"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	plainTask := &task.Task{ID: "PLAIN1", Title: "fresh task"}
	plain := tikiFromTask(plainTask)
	tc := &TriggerContext{Old: nil, New: plain, AllTikis: tikisFromTasks([]*task.Task{plainTask})}

	result, err := te.testExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated tiki, got %+v", result.Update)
	}
	u := result.Update.Updated[0]
	if len(u.Tags) != 1 || u.Tags[0] != "auto" {
		t.Errorf("expected tags=[auto], got %v", u.Tags)
	}
}

// Guarded variant works: `set tags = ["auto"]` is a pure assignment.
func TestPhase4_Trigger_SetTagsExplicitLiteralWorks(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`after create update where id = new.id set tags = ["auto"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	plainTask := &task.Task{ID: "PLAIN1", Title: "fresh task"}
	plain := tikiFromTask(plainTask)
	tc := &TriggerContext{Old: nil, New: plain, AllTikis: tikisFromTasks([]*task.Task{plainTask})}

	result, err := te.testExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated tiki, got %+v", result.Update)
	}
	u := result.Update.Updated[0]
	if len(u.Tags) != 1 || u.Tags[0] != "auto" {
		t.Errorf("expected tags=[auto], got %v", u.Tags)
	}
}

// --- Phase 4: next_date on absent recurrence ---
//
// Under the updated Phase-4 rule, `next_date(new.recurrence) is empty`
// on a tiki without a recurrence field is true (absent propagates
// through next_date into is-empty, which treats absent as empty).
// `is not empty` on that same tiki is therefore false — the guard
// does not match and the trigger does not fire.

func TestPhase4_Trigger_NextDateAbsentRecurrenceIsEmpty(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where next_date(new.recurrence) is not empty deny "recurring"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	plain := tikiFromTask(&task.Task{ID: "PLAIN1"})
	tc := &TriggerContext{Old: plain, New: plain}

	matched, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched {
		t.Fatal("guard should not match when recurrence is absent")
	}
}

func TestPhase4_Trigger_NextDatePresentRecurrenceMatches(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where next_date(new.recurrence) is not empty deny "recurring"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	recurring := tikiFromTask(&task.Task{
		ID: "REC01", Recurrence: task.RecurrenceDaily,
	})
	tc := &TriggerContext{Old: recurring, New: recurring}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard should match: present daily recurrence yields a real next_date")
	}
}
