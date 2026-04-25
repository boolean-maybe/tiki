package ruki

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/task"
)

func newTestTriggerExecutor() *TriggerExecutor {
	return NewTriggerExecutor(testSchema{}, func() string { return "alice" })
}

// --- EvalGuard ---

func TestEvalGuard_NoWhere(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{Timing: "before", Event: "update"}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "inProgress"},
		New: &task.Task{ID: "TIKI-000001", Status: "done"},
	}
	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard with no where should pass")
	}
}

func TestEvalGuard_QualifiedRefMatch(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where old.status = "inProgress" and new.status = "done" deny "no skip"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "inProgress"},
		New: &task.Task{ID: "TIKI-000001", Status: "done"},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard should match: old is in_progress, new is done")
	}
}

func TestEvalGuard_QualifiedBareBoolExpr(t *testing.T) {
	te := NewTriggerExecutor(customTestSchema{}, func() string { return "alice" })
	p := newCustomParser()

	trig, err := p.ParseTrigger(`before update where new.flag deny "blocked"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", CustomFields: map[string]interface{}{"flag": false}},
		New: &task.Task{ID: "TIKI-000001", CustomFields: map[string]interface{}{"flag": true}},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard should match: new.flag is true")
	}
}

func TestEvalGuard_QualifiedRefNoMatch(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where old.status = "inProgress" and new.status = "done" deny "no"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready"},
		New: &task.Task{ID: "TIKI-000001", Status: "inProgress"},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("guard should not match: old.status is ready, not in_progress")
	}
}

func TestEvalGuard_OldNilForCreate(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// guard references old.priority — old is nil for creates, so old.priority resolves to nil
	trig, err := p.ParseTrigger(`before create where new.type = "story" and new.description is empty deny "needs description"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: nil, // create — no old
		New: &task.Task{ID: "TIKI-000001", Type: "story", Description: ""},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard should match: new type is story and description is empty")
	}
}

func TestEvalGuard_QuantifierWithQualifiedCollection(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where new.status = "done" and new.dependsOn any status != "done" deny "open deps"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	dep := &task.Task{ID: "TIKI-DEP001", Status: "inProgress"}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "inProgress", DependsOn: []string{"TIKI-DEP001"}},
		New: &task.Task{ID: "TIKI-000001", Status: "done", DependsOn: []string{"TIKI-DEP001"}},
		AllTasks: []*task.Task{
			{ID: "TIKI-000001", Status: "done", DependsOn: []string{"TIKI-DEP001"}},
			dep,
		},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard should match: new.status=done and dep status != done")
	}
}

func TestEvalGuard_CountSubquery(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where new.status = "in progress" and count(select where assignee = new.assignee and status = "in progress") >= 3 deny "WIP limit"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready", Assignee: "alice"},
		New: &task.Task{ID: "TIKI-000001", Status: "inProgress", Assignee: "alice"},
		AllTasks: []*task.Task{
			{ID: "TIKI-000001", Status: "inProgress", Assignee: "alice"},
			{ID: "TIKI-000002", Status: "inProgress", Assignee: "alice"},
			{ID: "TIKI-000003", Status: "inProgress", Assignee: "alice"},
			{ID: "TIKI-000004", Status: "inProgress", Assignee: "bob"},
		},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard should match: 3 in-progress tasks for alice")
	}
}

func TestEvalGuard_CountSubqueryBelowLimit(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where new.status = "in progress" and count(select where assignee = new.assignee and status = "in progress") >= 3 deny "WIP limit"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready", Assignee: "alice"},
		New: &task.Task{ID: "TIKI-000001", Status: "inProgress", Assignee: "alice"},
		AllTasks: []*task.Task{
			{ID: "TIKI-000001", Status: "inProgress", Assignee: "alice"},
			{ID: "TIKI-000002", Status: "inProgress", Assignee: "alice"},
			{ID: "TIKI-000003", Status: "ready", Assignee: "alice"},
		},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("guard should not match: only 2 in-progress tasks for alice")
	}
}

// --- ExecAction ---

func TestExecAction_UpdateWithQualifiedRefs(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// after create: auto-assign urgent tasks
	trig, err := p.ParseTrigger(`after create where new.priority <= 2 and new.assignee is empty update where id = new.id set assignee="booleanmaybe"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	newTask := &task.Task{ID: "TIKI-000001", Title: "Urgent", Status: "ready", Priority: 1}
	tc := &TriggerContext{
		Old: nil,
		New: newTask,
		AllTasks: []*task.Task{
			newTask,
		},
	}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil {
		t.Fatal("expected Update result")
	}
	if len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated task, got %d", len(result.Update.Updated))
	}
	if result.Update.Updated[0].Assignee != "booleanmaybe" {
		t.Fatalf("expected assignee=booleanmaybe, got %q", result.Update.Updated[0].Assignee)
	}
}

func TestExecAction_CreateWithQualifiedRefs(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// after update: create recurring task
	trig, err := p.ParseTrigger(`after update where new.status = "done" and old.recurrence is not empty create title=old.title priority=old.priority tags=old.tags status="ready"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	old := &task.Task{ID: "TIKI-000001", Title: "Daily standup", Status: "inProgress", Priority: 2, Tags: []string{"meeting"}}
	new := &task.Task{ID: "TIKI-000001", Title: "Daily standup", Status: "done", Priority: 2}

	tc := &TriggerContext{
		Old:      old,
		New:      new,
		AllTasks: []*task.Task{new},
	}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Create == nil {
		t.Fatal("expected Create result")
	}
	created := result.Create.Task
	if created.Title != "Daily standup" {
		t.Fatalf("expected title 'Daily standup', got %q", created.Title)
	}
	if created.Priority != 2 {
		t.Fatalf("expected priority 2, got %d", created.Priority)
	}
	if created.Status != "ready" {
		t.Fatalf("expected status ready, got %q", created.Status)
	}
}

func TestExecAction_DeleteWithQualifiedRefs(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`after update where new.status = "done" delete where id = old.id`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	old := &task.Task{ID: "TIKI-000001", Title: "Stale", Status: "inProgress"}
	new := &task.Task{ID: "TIKI-000001", Title: "Stale", Status: "done"}

	tc := &TriggerContext{
		Old:      old,
		New:      new,
		AllTasks: []*task.Task{new, {ID: "TIKI-000002", Title: "Other", Status: "ready"}},
	}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Delete == nil {
		t.Fatal("expected Delete result")
	}
	if len(result.Delete.Deleted) != 1 {
		t.Fatalf("expected 1 deleted, got %d", len(result.Delete.Deleted))
	}
	if result.Delete.Deleted[0].ID != "TIKI-000001" {
		t.Fatalf("expected deleted TIKI-000001, got %s", result.Delete.Deleted[0].ID)
	}
}

func TestExecAction_CascadeEpicCompletion(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// cascade: when a story completes, complete parent epics if all deps done
	trig, err := p.ParseTrigger(`after update where new.status = "done" update where id in blocks(old.id) and type = "epic" and dependsOn all status = "done" set status="done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	story := &task.Task{ID: "TIKI-STORY1", Title: "Story", Status: "done", Type: "story"}
	epic := &task.Task{
		ID: "TIKI-EPIC01", Title: "Epic", Status: "inProgress", Type: "epic",
		DependsOn: []string{"TIKI-STORY1"},
	}

	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-STORY1", Status: "inProgress"},
		New:      story,
		AllTasks: []*task.Task{story, epic},
	}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil {
		t.Fatal("expected Update result")
	}
	if len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated, got %d", len(result.Update.Updated))
	}
	if result.Update.Updated[0].Status != "done" {
		t.Fatalf("expected epic status done, got %q", result.Update.Updated[0].Status)
	}
}

func TestExecAction_CleanupDependsOnDelete(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`after delete update where old.id in dependsOn set dependsOn=dependsOn - [old.id]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	deleted := &task.Task{ID: "TIKI-DEL001", Title: "Deleted"}
	downstream := &task.Task{
		ID: "TIKI-DOWN01", Title: "Downstream", Status: "ready",
		DependsOn: []string{"TIKI-DEL001", "TIKI-OTHER1"},
	}
	unrelated := &task.Task{
		ID: "TIKI-OTHER1", Title: "Unrelated", Status: "done",
		DependsOn: []string{"TIKI-OTHER2"},
	}

	tc := &TriggerContext{
		Old:      deleted,
		New:      nil, // delete event
		AllTasks: []*task.Task{downstream, unrelated},
	}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil {
		t.Fatal("expected Update result")
	}
	if len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated (downstream only), got %d", len(result.Update.Updated))
	}
	updated := result.Update.Updated[0]
	if len(updated.DependsOn) != 1 || updated.DependsOn[0] != "TIKI-OTHER1" {
		t.Fatalf("expected dependsOn=[TIKI-OTHER1], got %v", updated.DependsOn)
	}
}

func TestExecAction_NoAction(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{Timing: "before", Event: "update"}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for trigger with no action")
	}
}

// --- ExecRun ---

func TestExecRun_SimpleString(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`after update where new.status = "in progress" and "claude" in new.tags run("claude -p 'implement tiki " + old.id + "'")`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready", Tags: []string{"claude"}},
		New: &task.Task{ID: "TIKI-000001", Status: "inProgress", Tags: []string{"claude"}},
	}

	cmd, err := te.ExecRun(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "claude -p 'implement tiki TIKI-000001'"
	if cmd != expected {
		t.Fatalf("expected %q, got %q", expected, cmd)
	}
}

func TestExecRun_NoRunAction(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{Timing: "after", Event: "update"}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.ExecRun(trig, tc)
	if err == nil {
		t.Fatal("expected error for trigger with no run action")
	}
}

// --- two-layer scoping: bare fields resolve to target, qualified to triggering ---

func TestExecAction_TwoLayerScoping(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// "update where id = new.id set tags=new.tags + ["auto"]"
	// In the action's set clause: new.tags resolves from tc.New (triggering task)
	// but `tags` in set context resolves from the target task clone
	trig, err := p.ParseTrigger(`after create where new.type = "bug" update where id = new.id set tags=new.tags + ["needs-triage"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	newTask := &task.Task{ID: "TIKI-BUG001", Title: "Bug", Status: "ready", Type: "bug", Tags: []string{"urgent"}}
	tc := &TriggerContext{
		Old: nil,
		New: newTask,
		AllTasks: []*task.Task{
			newTask,
		},
	}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil {
		t.Fatal("expected Update result")
	}
	updated := result.Update.Updated[0]
	// new.tags is ["urgent"] from the triggering context, + ["needs-triage"]
	if len(updated.Tags) != 2 || updated.Tags[0] != "urgent" || updated.Tags[1] != "needs-triage" {
		t.Fatalf("expected tags [urgent, needs-triage], got %v", updated.Tags)
	}
}

// --- helper tests ---

func TestEqualFoldID(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"TIKI-000001", "tiki-000001", true},
		{"tiki-000001", "TIKI-000001", true}, // covers ca lowercase fold
		{"TIKI-000001", "TIKI-000001", true},
		{"TIKI-000001", "TIKI-000002", false},
		{"AB", "ABC", false},
	}
	for _, tt := range tests {
		if got := equalFoldID(tt.a, tt.b); got != tt.want {
			t.Errorf("equalFoldID(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestBlocksLookup(t *testing.T) {
	tasks := []*task.Task{
		{ID: "TIKI-AAA001", DependsOn: []string{"TIKI-TARGET"}},
		{ID: "TIKI-AAA002", DependsOn: []string{"TIKI-OTHER"}},
		{ID: "TIKI-AAA003", DependsOn: []string{"TIKI-TARGET", "TIKI-OTHER"}},
	}
	blockers := blocksLookup("TIKI-TARGET", tasks)
	if len(blockers) != 2 {
		t.Fatalf("expected 2 blockers, got %d", len(blockers))
	}
}

func TestResolveRefTasks(t *testing.T) {
	tasks := []*task.Task{
		{ID: "TIKI-000001"},
		{ID: "TIKI-000002"},
		{ID: "TIKI-000003"},
	}
	refs := []interface{}{"TIKI-000001", "TIKI-000003", "TIKI-NOPE00"}
	resolved := resolveRefTasks(refs, tasks)
	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved tasks, got %d", len(resolved))
	}
	if resolved[0].ID != "TIKI-000001" || resolved[1].ID != "TIKI-000003" {
		t.Fatalf("expected TIKI-000001 and TIKI-000003, got %s and %s", resolved[0].ID, resolved[1].ID)
	}
}

// --- in expression override with string contains ---

func TestExecAction_NextDateWithQualifiedRef(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`after update where new.status = "done" and old.recurrence is not empty create title=old.title status="ready" type=old.type priority=old.priority due=next_date(old.recurrence)`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	old := &task.Task{ID: "TIKI-000001", Title: "Daily standup", Status: "inProgress", Type: "story", Priority: 3, Recurrence: task.RecurrenceDaily}
	new := &task.Task{ID: "TIKI-000001", Title: "Daily standup", Status: "done", Type: "story", Priority: 3}

	tc := &TriggerContext{
		Old:      old,
		New:      new,
		AllTasks: []*task.Task{new},
	}

	before := time.Now()
	result, err := te.ExecAction(trig, tc)
	after := time.Now()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Create == nil {
		t.Fatal("expected Create result")
	}
	created := result.Create.Task
	if created.Due.IsZero() {
		t.Fatal("expected non-zero due date from next_date(old.recurrence)")
	}
	expBefore := task.NextOccurrenceFrom(task.RecurrenceDaily, before)
	expAfter := task.NextOccurrenceFrom(task.RecurrenceDaily, after)
	if !created.Due.Equal(expBefore) && !created.Due.Equal(expAfter) {
		t.Fatalf("expected due=%v or %v, got %v", expBefore, expAfter, created.Due)
	}
}

func TestExecAction_NextDateWithNewQualifiedRef(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`after update where new.status = "ready" and new.recurrence is not empty update where id = new.id set due=next_date(new.recurrence)`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	old := &task.Task{ID: "TIKI-000001", Title: "Recurring", Status: "done", Type: "story", Priority: 3}
	new := &task.Task{ID: "TIKI-000001", Title: "Recurring", Status: "ready", Type: "story", Priority: 3, Recurrence: task.RecurrenceDaily}

	tc := &TriggerContext{
		Old:      old,
		New:      new,
		AllTasks: []*task.Task{new},
	}

	before := time.Now()
	result, err := te.ExecAction(trig, tc)
	after := time.Now()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil {
		t.Fatal("expected Update result")
	}
	if len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated task, got %d", len(result.Update.Updated))
	}
	updated := result.Update.Updated[0]
	if updated.Due.IsZero() {
		t.Fatal("expected non-zero due date from next_date(new.recurrence)")
	}
	expBefore := task.NextOccurrenceFrom(task.RecurrenceDaily, before)
	expAfter := task.NextOccurrenceFrom(task.RecurrenceDaily, after)
	if !updated.Due.Equal(expBefore) && !updated.Due.Equal(expAfter) {
		t.Fatalf("expected due=%v or %v, got %v", expBefore, expAfter, updated.Due)
	}
}

func TestEvalInOverride_SubstringMatch(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where "claude" in new.title deny "no claude"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Title: "implement claude feature"},
		New: &task.Task{ID: "TIKI-000001", Title: "implement claude feature"},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard should match: 'claude' is in new.title")
	}
}

func TestEvalInOverride_NegatedList(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where "blocked" not in new.tags deny "must not be blocked"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// match: "blocked" not in tags → deny fires
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Tags: []string{"ready"}},
		New: &task.Task{ID: "TIKI-000001", Tags: []string{"ready"}},
	}
	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard should match: 'blocked' is not in tags")
	}

	// no match: "blocked" in tags → deny doesn't fire
	tc.New = &task.Task{ID: "TIKI-000001", Tags: []string{"blocked"}}
	ok, err = te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("guard should not match: 'blocked' is in tags")
	}
}

func TestEvalInOverride_ListMembership(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where "claude" in new.tags deny "no claude"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// match
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Tags: []string{"claude", "ai"}},
		New: &task.Task{ID: "TIKI-000001", Tags: []string{"claude", "ai"}},
	}
	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard should match: 'claude' is in new.tags")
	}

	// no match
	tc.New = &task.Task{ID: "TIKI-000001", Tags: []string{"ai"}}
	ok, err = te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("guard should not match: 'claude' not in new.tags")
	}
}

// --- Phase 2: error path tests ---

// custom condition type to trigger "unknown condition type" error
type bogusCondition struct{}

func (*bogusCondition) conditionNode() {}

func TestResolveQualifiedRef_UnknownQualifier(t *testing.T) {
	te := newTestTriggerExecutor()
	// construct a trigger whose guard references an unknown qualifier
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &CompareExpr{
			Left:  &QualifiedRef{Qualifier: "mid", Name: "status"},
			Op:    "=",
			Right: &StringLiteral{Value: "done"},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready"},
		New: &task.Task{ID: "TIKI-000001", Status: "done"},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier")
	}
	if !strings.Contains(err.Error(), "unknown qualifier") {
		t.Fatalf("expected 'unknown qualifier' error, got: %v", err)
	}
}

func TestEvalCondition_UnknownBinaryOp(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &BinaryCondition{
			Op: "xor",
			Left: &CompareExpr{
				Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			},
			Right: &CompareExpr{
				Left: &FieldRef{Name: "priority"}, Op: "=", Right: &IntLiteral{Value: 1},
			},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "done", Priority: 1},
		New: &task.Task{ID: "TIKI-000001", Status: "done", Priority: 1},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown binary operator")
	}
	if !strings.Contains(err.Error(), "unknown binary operator") {
		t.Fatalf("expected 'unknown binary operator' error, got: %v", err)
	}
}

func TestEvalExprRecursive_UnknownBinaryOp(t *testing.T) {
	te := newTestTriggerExecutor()
	// trigger action that uses an unknown binary operator in an expression
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "TIKI-000001"},
				},
				Set: []Assignment{
					{Field: "priority", Value: &BinaryExpr{Op: "*", Left: &IntLiteral{Value: 1}, Right: &IntLiteral{Value: 2}}},
				},
			},
		},
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Priority: 1},
		New: &task.Task{ID: "TIKI-000001", Priority: 1},
		AllTasks: []*task.Task{
			{ID: "TIKI-000001", Priority: 1},
		},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown binary operator in expression")
	}
	if !strings.Contains(err.Error(), "unknown binary operator") {
		t.Fatalf("expected 'unknown binary operator' error, got: %v", err)
	}
}

func TestEvalInOverride_CollectionNotListOrString(t *testing.T) {
	te := newTestTriggerExecutor()
	// "value" in priority — priority is an int, not a list or string
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &InExpr{
			Value:      &StringLiteral{Value: "test"},
			Collection: &FieldRef{Name: "priority"},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Priority: 3},
		New: &task.Task{ID: "TIKI-000001", Priority: 3},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for in with non-list non-string collection")
	}
	if !strings.Contains(err.Error(), "collection is not a list or string") {
		t.Fatalf("expected collection type error, got: %v", err)
	}
}

func TestEvalInOverride_SubstringNonString(t *testing.T) {
	te := newTestTriggerExecutor()
	// 42 in title — value is int, collection is string
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &InExpr{
			Value:      &IntLiteral{Value: 42},
			Collection: &FieldRef{Name: "title"},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Title: "fix bug 42"},
		New: &task.Task{ID: "TIKI-000001", Title: "fix bug 42"},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for in: substring check requires string value")
	}
	if !strings.Contains(err.Error(), "substring check requires string value") {
		t.Fatalf("expected substring type error, got: %v", err)
	}
}

func TestEvalQuantifierOverride_NotAList(t *testing.T) {
	te := newTestTriggerExecutor()
	// title any status = "done" — title is a string, not a list
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &QuantifierExpr{
			Expr: &FieldRef{Name: "title"},
			Kind: "any",
			Condition: &CompareExpr{
				Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Title: "test"},
		New: &task.Task{ID: "TIKI-000001", Title: "test"},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for quantifier on non-list")
	}
	if !strings.Contains(err.Error(), "expression is not a list") {
		t.Fatalf("expected 'expression is not a list' error, got: %v", err)
	}
}

func TestEvalQuantifierOverride_UnknownKind(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &QuantifierExpr{
			Expr: &FieldRef{Name: "tags"},
			Kind: "some", // invalid — only "any" and "all" are valid
			Condition: &CompareExpr{
				Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Tags: []string{"a"}},
		New: &task.Task{ID: "TIKI-000001", Tags: []string{"a"}},
		AllTasks: []*task.Task{
			{ID: "A", Status: "done"},
		},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown quantifier kind")
	}
	if !strings.Contains(err.Error(), "unknown quantifier") {
		t.Fatalf("expected 'unknown quantifier' error, got: %v", err)
	}
}

func TestEvalCondition_UnknownType(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where:  &bogusCondition{},
		Deny:   strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown condition type")
	}
	if !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected 'unknown condition type' error, got: %v", err)
	}
}

func TestExecute_NilStatement(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: nil, // no action
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for nil action")
	}
	if !strings.Contains(err.Error(), "trigger has no action") {
		t.Fatalf("expected 'trigger has no action' error, got: %v", err)
	}
}

func TestExecute_UnsupportedType(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	// statement with all nil variants (no select/create/update/delete)
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for unsupported trigger action type")
	}
	if !strings.Contains(err.Error(), "empty statement") {
		t.Fatalf("expected 'empty statement' error, got: %v", err)
	}
}

func TestFilterTasks_NilWhere(t *testing.T) {
	te := newTestTriggerExecutor()
	// delete with no where — should match all tasks
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Delete: &DeleteStmt{Where: nil},
		},
	}
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "a"},
		{ID: "TIKI-000002", Title: "b"},
	}
	tc := &TriggerContext{
		Old:      tasks[0],
		New:      tasks[0],
		AllTasks: tasks,
	}
	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Delete == nil {
		t.Fatal("expected Delete result")
	}
	if len(result.Delete.Deleted) != 2 {
		t.Fatalf("expected 2 deleted (nil where matches all), got %d", len(result.Delete.Deleted))
	}
}

func TestGuardSentinel_BothNil(t *testing.T) {
	te := newTestTriggerExecutor()
	// trigger with no where (guard passes trivially) and both old/new nil
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		// no Where — always passes
	}
	tc := &TriggerContext{
		Old: nil,
		New: nil,
	}
	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard with no where and both nil should still pass")
	}
}

func TestExecRun_NonStringResult(t *testing.T) {
	te := newTestTriggerExecutor()
	// run() with an expression that evaluates to int, not string
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Run:    &RunAction{Command: &IntLiteral{Value: 42}},
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.ExecRun(trig, tc)
	if err == nil {
		t.Fatal("expected error for run command that doesn't evaluate to string")
	}
	if !strings.Contains(err.Error(), "run command did not evaluate to string") {
		t.Fatalf("expected 'run command did not evaluate to string' error, got: %v", err)
	}
}

func TestEvalGuardRawTriggerRejectsCallBeforeEvaluation(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where 1 = 2 and call("echo hello") = "x" deny "blocked"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready"},
		New: &task.Task{ID: "TIKI-000001", Status: "done"},
	}
	_, err = te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected semantic validation error")
	}
	if !strings.Contains(err.Error(), "call() is not supported yet") {
		t.Fatalf("expected call() semantic validation error, got: %v", err)
	}
}

func TestExecRunRawTriggerRejectsCall(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`after update run(call("echo hello"))`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err = te.ExecRun(trig, tc)
	if err == nil {
		t.Fatal("expected semantic validation error")
	}
	if !strings.Contains(err.Error(), "call() is not supported yet") {
		t.Fatalf("expected call() semantic validation error, got: %v", err)
	}
}

func TestEvalGuardValidatedTriggerRuntimeMismatch(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	validated, err := p.ParseAndValidateTrigger(`before update where new.status = "done" deny "blocked"`, ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready"},
		New: &task.Task{ID: "TIKI-000001", Status: "done"},
	}
	_, err = te.EvalGuard(validated, tc)
	if err == nil {
		t.Fatal("expected runtime mismatch error")
	}
	var mismatch *RuntimeMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected RuntimeMismatchError, got: %v", err)
	}
}

func TestExecTimeTriggerActionValidatedRuntimeMismatch(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	validated, err := p.ParseAndValidateTimeTrigger(`every 1day delete where status = "done"`, ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	_, err = te.ExecTimeTriggerAction(validated, nil)
	if err == nil {
		t.Fatal("expected runtime mismatch error")
	}
	var mismatch *RuntimeMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected RuntimeMismatchError, got: %v", err)
	}
}

// --- coverage gap tests ---

func TestNewTriggerExecutor_NilUserFunc(t *testing.T) {
	te := NewTriggerExecutor(testSchema{}, nil)
	if te.userFunc != nil {
		t.Fatal("expected nil userFunc to be preserved (user() should error at runtime)")
	}
}

func TestGuardSentinel_OldOnly(t *testing.T) {
	te := newTestTriggerExecutor()
	// when new is nil (delete event), sentinel should fall back to old
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "done"},
		New: nil,
	}
	sentinel := te.guardSentinel(tc)
	if sentinel.ID != "TIKI-000001" {
		t.Fatalf("expected sentinel from old, got ID %q", sentinel.ID)
	}
}

func TestEvalCondition_OrShortCircuit(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// left is true → or short-circuits, right is never evaluated
	trig, err := p.ParseTrigger(`before update where new.status = "done" or new.priority = 1 deny "blocked"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready", Priority: 5},
		New: &task.Task{ID: "TIKI-000001", Status: "done", Priority: 5},
	}
	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("or should short-circuit on left=true")
	}
}

func TestEvalCondition_NotCondition(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where not new.status = "done" deny "not done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// new.status is "ready" → not (ready = done) = not false = true
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "inProgress"},
		New: &task.Task{ID: "TIKI-000001", Status: "ready"},
	}
	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("not condition should negate the inner result")
	}
}

func TestEvalCondition_IsEmptyNegated(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where new.assignee is not empty deny "has assignee"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Assignee: "alice"},
		New: &task.Task{ID: "TIKI-000001", Assignee: "alice"},
	}
	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("is not empty should match when assignee is set")
	}
}

func TestEvalCondition_IsEmpty(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before create where new.assignee is empty deny "no assignee"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: nil,
		New: &task.Task{ID: "TIKI-000001", Assignee: ""},
	}
	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("is empty should match when assignee is blank")
	}
}

func TestEvalExprRecursive_ListLiteral(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// action uses a list literal in set clause: tags=["auto", "trigger"]
	trig, err := p.ParseTrigger(`after create update where id = new.id set tags=["auto", "trigger"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	newTask := &task.Task{ID: "TIKI-000001", Title: "Test", Status: "ready"}
	tc := &TriggerContext{
		Old:      nil,
		New:      newTask,
		AllTasks: []*task.Task{newTask},
	}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatal("expected 1 updated task")
	}
	tags := result.Update.Updated[0].Tags
	if len(tags) != 2 || tags[0] != "auto" || tags[1] != "trigger" {
		t.Fatalf("expected tags [auto, trigger], got %v", tags)
	}
}

func TestEvalCountOverride_NoWhere(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// count(select) with no where — should count all tasks
	trig, err := p.ParseTrigger(`before create where count(select) >= 3 deny "too many tasks"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: nil,
		New: &task.Task{ID: "TIKI-000004"},
		AllTasks: []*task.Task{
			{ID: "TIKI-000001"},
			{ID: "TIKI-000002"},
			{ID: "TIKI-000003"},
			{ID: "TIKI-000004"},
		},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("count(select) should return 4 which is >= 3")
	}
}

func TestEvalQuantifierOverride_AllMatch(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where new.dependsOn all status = "done" deny "all deps done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	dep1 := &task.Task{ID: "TIKI-DEP001", Status: "done"}
	dep2 := &task.Task{ID: "TIKI-DEP002", Status: "done"}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready", DependsOn: []string{"TIKI-DEP001", "TIKI-DEP002"}},
		New: &task.Task{ID: "TIKI-000001", Status: "inProgress", DependsOn: []string{"TIKI-DEP001", "TIKI-DEP002"}},
		AllTasks: []*task.Task{
			{ID: "TIKI-000001", Status: "inProgress", DependsOn: []string{"TIKI-DEP001", "TIKI-DEP002"}},
			dep1, dep2,
		},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("all quantifier should match when all deps are done")
	}
}

func TestEvalQuantifierOverride_AllNoMatch(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where new.dependsOn all status = "done" deny "all deps done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	dep1 := &task.Task{ID: "TIKI-DEP001", Status: "done"}
	dep2 := &task.Task{ID: "TIKI-DEP002", Status: "ready"}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready", DependsOn: []string{"TIKI-DEP001", "TIKI-DEP002"}},
		New: &task.Task{ID: "TIKI-000001", Status: "inProgress", DependsOn: []string{"TIKI-DEP001", "TIKI-DEP002"}},
		AllTasks: []*task.Task{
			{ID: "TIKI-000001", Status: "inProgress", DependsOn: []string{"TIKI-DEP001", "TIKI-DEP002"}},
			dep1, dep2,
		},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("all quantifier should not match when one dep is not done")
	}
}

func TestEvalQuantifierOverride_AllEmptyList(t *testing.T) {
	te := newTestTriggerExecutor()

	// all on empty list is vacuously true
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &QuantifierExpr{
			Expr: &QualifiedRef{Qualifier: "new", Name: "dependsOn"},
			Kind: "all",
			Condition: &CompareExpr{
				Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			},
		},
		Deny: strPtr("all done"),
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", DependsOn: nil},
		New: &task.Task{ID: "TIKI-000001", DependsOn: nil},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("all on empty list should be vacuously true")
	}
}

func TestEvalFunctionCallOverride_DelegateNow(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// now() should delegate to base executor
	trig, err := p.ParseTrigger(`before update where new.updatedAt > now() - 1day deny "too recent"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	recent := time.Now()
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", UpdatedAt: recent},
		New: &task.Task{ID: "TIKI-000001", UpdatedAt: recent},
	}

	// should not error — now() delegates to base
	_, err = te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvalFunctionCallOverride_User(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// user() should delegate to base and return "alice"
	trig, err := p.ParseTrigger(`before update where new.assignee = user() deny "self-assign"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Assignee: "alice"},
		New: &task.Task{ID: "TIKI-000001", Assignee: "alice"},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("user() should return 'alice', matching new.assignee")
	}
}

func TestExecuteCreate_ErrorInEvalExpr(t *testing.T) {
	te := newTestTriggerExecutor()
	// create with an expression that causes eval error:
	// unknown qualifier "mid" in assignment value
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Create: &CreateStmt{
				Assignments: []Assignment{
					{Field: "title", Value: &QualifiedRef{Qualifier: "mid", Name: "title"}},
				},
			},
		},
	}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in create assignment")
	}
	if !strings.Contains(err.Error(), "unknown qualifier") {
		t.Fatalf("expected 'unknown qualifier' error, got: %v", err)
	}
}

func TestExecuteUpdate_ErrorInEvalExpr(t *testing.T) {
	te := newTestTriggerExecutor()
	// update with an expression that causes eval error
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "TIKI-000001"},
				},
				Set: []Assignment{
					{Field: "title", Value: &QualifiedRef{Qualifier: "mid", Name: "title"}},
				},
			},
		},
	}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in update set")
	}
	if !strings.Contains(err.Error(), "unknown qualifier") {
		t.Fatalf("expected 'unknown qualifier' error, got: %v", err)
	}
}

func TestExecuteDelete_ErrorInFilterTasks(t *testing.T) {
	te := newTestTriggerExecutor()
	// delete with a where condition that causes an eval error
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Delete: &DeleteStmt{
				Where: &CompareExpr{
					Left: &QualifiedRef{Qualifier: "mid", Name: "id"}, Op: "=", Right: &StringLiteral{Value: "x"},
				},
			},
		},
	}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in delete where")
	}
}

func TestEvalBlocksOverride_ErrorInEvalExpr(t *testing.T) {
	te := newTestTriggerExecutor()
	// blocks() with a QualifiedRef that has unknown qualifier
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "id"}, Op: "=",
					Right: &FunctionCall{Name: "blocks", Args: []Expr{&QualifiedRef{Qualifier: "mid", Name: "id"}}},
				},
				Set: []Assignment{{Field: "title", Value: &StringLiteral{Value: "x"}}},
			},
		},
	}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in blocks() arg")
	}
}

func TestEvalNextDateOverride_ErrorInEvalExpr(t *testing.T) {
	te := newTestTriggerExecutor()
	// next_date() with a QualifiedRef that has unknown qualifier
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Create: &CreateStmt{
				Assignments: []Assignment{
					{Field: "title", Value: &StringLiteral{Value: "test"}},
					{Field: "due", Value: &FunctionCall{Name: "next_date", Args: []Expr{&QualifiedRef{Qualifier: "mid", Name: "recurrence"}}}},
				},
			},
		},
	}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in next_date() arg")
	}
}

func TestEvalNextDateOverride_NonRecurrenceValue(t *testing.T) {
	te := newTestTriggerExecutor()
	// next_date() with a field that doesn't contain a Recurrence value
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Create: &CreateStmt{
				Assignments: []Assignment{
					{Field: "title", Value: &StringLiteral{Value: "test"}},
					{Field: "due", Value: &FunctionCall{Name: "next_date", Args: []Expr{&FieldRef{Name: "title"}}}},
				},
			},
		},
	}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001", Title: "hello"},
		New:      &task.Task{ID: "TIKI-000001", Title: "hello"},
		AllTasks: []*task.Task{{ID: "TIKI-000001", Title: "hello"}},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for next_date() with non-recurrence value")
	}
	if !strings.Contains(err.Error(), "recurrence") {
		t.Fatalf("expected recurrence error, got: %v", err)
	}
}

func TestResolveQualifiedRef_OldNil(t *testing.T) {
	te := newTestTriggerExecutor()

	// construct AST directly to bypass parser's qualifier validation
	// create event: old is nil, referencing old.assignee returns nil (not error)
	trig := &Trigger{
		Timing: "after",
		Event:  "create",
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &QualifiedRef{Qualifier: "new", Name: "id"}, Op: "=",
					Right: &QualifiedRef{Qualifier: "new", Name: "id"},
				},
				Set: []Assignment{
					{Field: "assignee", Value: &QualifiedRef{Qualifier: "old", Name: "assignee"}},
				},
			},
		},
	}

	newTask := &task.Task{ID: "TIKI-000001", Title: "Original", Type: "bug", Assignee: "alice"}
	tc := &TriggerContext{
		Old:      nil,
		New:      newTask,
		AllTasks: []*task.Task{newTask},
	}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// old.assignee resolved to nil → setField should set empty assignee
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatal("expected 1 updated task")
		return
	}
	if result.Update.Updated[0].Assignee != "" {
		t.Errorf("expected empty assignee from old.assignee (old is nil), got %q", result.Update.Updated[0].Assignee)
	}
}

func TestResolveQualifiedRef_NewNil(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// delete event: new is nil
	trig, err := p.ParseTrigger(`after delete update where old.id in dependsOn set dependsOn=dependsOn - [old.id]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	deleted := &task.Task{ID: "TIKI-DEL001"}
	downstream := &task.Task{ID: "TIKI-DOWN01", DependsOn: []string{"TIKI-DEL001"}}
	tc := &TriggerContext{
		Old:      deleted,
		New:      nil,
		AllTasks: []*task.Task{downstream},
	}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil {
		t.Fatal("expected update result")
	}
}

func TestBlocksLookup_NoBlockers(t *testing.T) {
	tasks := []*task.Task{
		{ID: "TIKI-AAA001", DependsOn: []string{"TIKI-OTHER1"}},
	}
	blockers := blocksLookup("TIKI-NOPE00", tasks)
	if len(blockers) != 0 {
		t.Fatalf("expected 0 blockers, got %d", len(blockers))
	}
}

func TestEvalCountOverride_ErrorInCondition(t *testing.T) {
	te := newTestTriggerExecutor()
	// count(select where <bogus condition>) should return error
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &CompareExpr{
			Left: &FunctionCall{
				Name: "count",
				Args: []Expr{&SubQuery{Where: &bogusCondition{}}},
			},
			Op:    ">=",
			Right: &IntLiteral{Value: 1},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for bogus condition in count subquery")
	}
}

func strPtr(s string) *string { return &s }

// --- short-circuit tests for BinaryCondition ---

func TestEvalCondition_AndShortCircuitFalse(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// left side is false (old.status != "inProgress"), right side should not be evaluated
	trig, err := p.ParseTrigger(`before update where old.status = "inProgress" and new.status = "done" deny "no"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready"},
		New: &task.Task{ID: "TIKI-000001", Status: "done"},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("AND should short-circuit: left is false")
	}
}

func TestEvalCondition_OrShortCircuitTrue(t *testing.T) {
	te := newTestTriggerExecutor()
	// construct an OR condition where left is true
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &BinaryCondition{
			Left: &CompareExpr{
				Left:  &QualifiedRef{Qualifier: "old", Name: "status"},
				Op:    "=",
				Right: &StringLiteral{Value: "ready"},
			},
			Op: "or",
			Right: &CompareExpr{
				Left:  &QualifiedRef{Qualifier: "new", Name: "status"},
				Op:    "=",
				Right: &StringLiteral{Value: "done"},
			},
		},
		Deny: strPtr("blocked"),
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready"},
		New: &task.Task{ID: "TIKI-000001", Status: "inProgress"},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("OR should short-circuit: left is true")
	}
}

// --- evalInOverride with []string collection ---

func TestEvalInOverride_StringListFromField(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// "alice" in old.tags — tests the []interface{} path
	trig, err := p.ParseTrigger(`before update where "bug" in new.tags deny "has tag"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Tags: []string{"bug", "api"}},
		New: &task.Task{ID: "TIKI-000001", Tags: []string{"bug", "api"}},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected match: 'bug' in tags")
	}
}

func TestEvalInOverride_NotIn(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where "urgent" not in new.tags deny "missing tag"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Tags: []string{"bug"}},
		New: &task.Task{ID: "TIKI-000001", Tags: []string{"bug"}},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected match: 'urgent' not in tags")
	}
}

// --- evalQuantifierOverride ---

func TestEvalQuantifierOverride_AllWithEmptyList(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// "all dependsOn have status = done" with empty dependsOn should be vacuously true
	trig, err := p.ParseTrigger(`before update where new.dependsOn all status = "done" deny "open deps"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", DependsOn: []string{}},
		New: &task.Task{ID: "TIKI-000001", DependsOn: []string{}},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("all with empty list should be vacuously true")
	}
}

// --- NotCondition override ---

func TestEvalCondition_NotOverride(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &NotCondition{
			Inner: &CompareExpr{
				Left:  &QualifiedRef{Qualifier: "new", Name: "status"},
				Op:    "=",
				Right: &StringLiteral{Value: "done"},
			},
		},
		Deny: strPtr("not done"),
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready"},
		New: &task.Task{ID: "TIKI-000001", Status: "ready"},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("NOT(status=done) should be true when status=ready")
	}
}

// --- IsEmptyExpr override ---

func TestEvalCondition_IsEmptyOverride(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "create",
		Where: &IsEmptyExpr{
			Expr:    &QualifiedRef{Qualifier: "new", Name: "assignee"},
			Negated: false,
		},
		Deny: strPtr("unassigned"),
	}

	tc := &TriggerContext{
		New: &task.Task{ID: "TIKI-000001", Assignee: ""},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("empty assignee should match 'is empty'")
	}
}

func TestEvalCondition_IsNotEmptyOverride(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "create",
		Where: &IsEmptyExpr{
			Expr:    &QualifiedRef{Qualifier: "new", Name: "assignee"},
			Negated: true,
		},
		Deny: strPtr("assigned"),
	}

	tc := &TriggerContext{
		New: &task.Task{ID: "TIKI-000001", Assignee: "alice"},
	}

	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("'alice' is not empty should match 'is not empty'")
	}
}

// --- BinaryExpr in trigger context ---

func TestEvalExprRecursive_BinaryExprSubtract(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// after delete, remove old.id from dependsOn using subtraction
	trig, err := p.ParseTrigger(`after delete update where old.id in dependsOn set dependsOn=dependsOn - [old.id]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	dep := &task.Task{ID: "TIKI-DEP001", Status: "done"}
	downstream := &task.Task{
		ID: "TIKI-DOWN01", Status: "ready",
		DependsOn: []string{"TIKI-DEP001", "TIKI-OTHER1"},
	}
	tc := &TriggerContext{
		Old:      dep,
		New:      nil,
		AllTasks: []*task.Task{dep, downstream},
	}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) == 0 {
		t.Fatal("expected update result")
	}
	updated := result.Update.Updated[0]
	if len(updated.DependsOn) != 1 || updated.DependsOn[0] != "TIKI-OTHER1" {
		t.Errorf("expected dependsOn=[TIKI-OTHER1], got %v", updated.DependsOn)
	}
}

// --- ListLiteral in trigger override ---

func TestEvalExprRecursive_ListLiteralInAction(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`after create update where id = new.id set tags=["auto"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	newTask := &task.Task{ID: "TIKI-000001", Status: "ready"}
	tc := &TriggerContext{
		Old:      nil,
		New:      newTask,
		AllTasks: []*task.Task{newTask},
	}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) == 0 {
		t.Fatal("expected update result")
	}
	if len(result.Update.Updated[0].Tags) != 1 || result.Update.Updated[0].Tags[0] != "auto" {
		t.Errorf("expected tags=[auto], got %v", result.Update.Updated[0].Tags)
	}
}

// --- additional coverage for error propagation and edge cases ---

func TestExecRun_ErrorInEvalExpr(t *testing.T) {
	te := newTestTriggerExecutor()
	// run() with an expression that causes eval error (unknown qualifier)
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Run:    &RunAction{Command: &QualifiedRef{Qualifier: "mid", Name: "title"}},
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.ExecRun(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in run command")
	}
	if !strings.Contains(err.Error(), "evaluating run command") {
		t.Fatalf("expected 'evaluating run command' error, got: %v", err)
	}
}

func TestEvalExprRecursive_BinaryExprLeftError(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "TIKI-000001"},
				},
				Set: []Assignment{
					{Field: "priority", Value: &BinaryExpr{
						Op:    "+",
						Left:  &QualifiedRef{Qualifier: "mid", Name: "priority"},
						Right: &IntLiteral{Value: 1},
					}},
				},
			},
		},
	}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001", Priority: 1},
		New:      &task.Task{ID: "TIKI-000001", Priority: 1},
		AllTasks: []*task.Task{{ID: "TIKI-000001", Priority: 1}},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in binary expr left operand")
	}
}

func TestEvalExprRecursive_BinaryExprRightError(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "TIKI-000001"},
				},
				Set: []Assignment{
					{Field: "priority", Value: &BinaryExpr{
						Op:    "+",
						Left:  &IntLiteral{Value: 1},
						Right: &QualifiedRef{Qualifier: "mid", Name: "priority"},
					}},
				},
			},
		},
	}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001", Priority: 1},
		New:      &task.Task{ID: "TIKI-000001", Priority: 1},
		AllTasks: []*task.Task{{ID: "TIKI-000001", Priority: 1}},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in binary expr right operand")
	}
}

func TestEvalExprRecursive_ListLiteralError(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "TIKI-000001"},
				},
				Set: []Assignment{
					{Field: "tags", Value: &ListLiteral{Elements: []Expr{
						&QualifiedRef{Qualifier: "mid", Name: "title"},
					}}},
				},
			},
		},
	}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in list literal element")
	}
}

func TestEvalCondition_BinaryConditionLeftError(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &BinaryCondition{
			Op: "and",
			Left: &CompareExpr{
				Left: &QualifiedRef{Qualifier: "mid", Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			},
			Right: &CompareExpr{
				Left: &FieldRef{Name: "priority"}, Op: "=", Right: &IntLiteral{Value: 1},
			},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in binary condition left")
	}
}

func TestEvalCondition_NotConditionError(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &NotCondition{
			Inner: &CompareExpr{
				Left: &QualifiedRef{Qualifier: "mid", Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in not condition")
	}
}

func TestEvalCondition_IsEmptyExprError(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &IsEmptyExpr{
			Expr: &QualifiedRef{Qualifier: "mid", Name: "assignee"},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in is empty expr")
	}
}

func TestEvalInOverride_ValueEvalError(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &InExpr{
			Value:      &QualifiedRef{Qualifier: "mid", Name: "title"},
			Collection: &FieldRef{Name: "tags"},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Tags: []string{"a"}},
		New: &task.Task{ID: "TIKI-000001", Tags: []string{"a"}},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in in-expr value")
	}
}

func TestEvalInOverride_CollectionEvalError(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &InExpr{
			Value:      &StringLiteral{Value: "test"},
			Collection: &QualifiedRef{Qualifier: "mid", Name: "tags"},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in in-expr collection")
	}
}

func TestEvalInOverride_NegatedSubstring(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where "xyz" not in new.title deny "missing"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Title: "hello world"},
		New: &task.Task{ID: "TIKI-000001", Title: "hello world"},
	}
	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("'xyz' not in 'hello world' should be true")
	}

	// when substring IS found, negated should be false
	tc.New = &task.Task{ID: "TIKI-000001", Title: "hello xyz world"}
	ok, err = te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("'xyz' not in 'hello xyz world' should be false")
	}
}

func TestEvalQuantifierOverride_ExprEvalError(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &QuantifierExpr{
			Expr: &QualifiedRef{Qualifier: "mid", Name: "dependsOn"},
			Kind: "any",
			Condition: &CompareExpr{
				Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in quantifier expr")
	}
}

func TestEvalQuantifierOverride_AnyConditionError(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &QuantifierExpr{
			Expr: &QualifiedRef{Qualifier: "new", Name: "dependsOn"},
			Kind: "any",
			Condition: &CompareExpr{
				Left: &QualifiedRef{Qualifier: "mid", Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			},
		},
		Deny: strPtr("blocked"),
	}
	dep := &task.Task{ID: "TIKI-DEP001", Status: "done"}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001", DependsOn: []string{"TIKI-DEP001"}},
		New:      &task.Task{ID: "TIKI-000001", DependsOn: []string{"TIKI-DEP001"}},
		AllTasks: []*task.Task{{ID: "TIKI-000001", DependsOn: []string{"TIKI-DEP001"}}, dep},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in any quantifier condition")
	}
}

func TestEvalQuantifierOverride_AllConditionError(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &QuantifierExpr{
			Expr: &QualifiedRef{Qualifier: "new", Name: "dependsOn"},
			Kind: "all",
			Condition: &CompareExpr{
				Left: &QualifiedRef{Qualifier: "mid", Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			},
		},
		Deny: strPtr("blocked"),
	}
	dep := &task.Task{ID: "TIKI-DEP001", Status: "done"}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001", DependsOn: []string{"TIKI-DEP001"}},
		New:      &task.Task{ID: "TIKI-000001", DependsOn: []string{"TIKI-DEP001"}},
		AllTasks: []*task.Task{{ID: "TIKI-000001", DependsOn: []string{"TIKI-DEP001"}}, dep},
	}
	_, err := te.EvalGuard(trig, tc)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in all quantifier condition")
	}
}

func TestExecuteCreate_SetFieldError(t *testing.T) {
	te := newTestTriggerExecutor()
	// create with assignment to immutable field
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Create: &CreateStmt{
				Assignments: []Assignment{
					{Field: "title", Value: &StringLiteral{Value: "test"}},
					{Field: "createdBy", Value: &StringLiteral{Value: "hacker"}},
				},
			},
		},
	}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for immutable field assignment")
	}
	if !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected 'immutable' error, got: %v", err)
	}
}

func TestExecuteUpdate_SetFieldError(t *testing.T) {
	te := newTestTriggerExecutor()
	// update with assignment to immutable field
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "TIKI-000001"},
				},
				Set: []Assignment{
					{Field: "createdBy", Value: &StringLiteral{Value: "hacker"}},
				},
			},
		},
	}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	_, err := te.ExecAction(trig, tc)
	if err == nil {
		t.Fatal("expected error for immutable field in update set")
	}
	if !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected 'immutable' error, got: %v", err)
	}
}

func TestExecuteNilStatement_Override(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	exec := te.newExecWithOverrides(tc)
	_, err := exec.Execute(nil, tc.AllTasks)
	if err == nil {
		t.Fatal("expected error for nil statement")
	}
	if !strings.Contains(err.Error(), "nil statement") {
		t.Fatalf("expected 'nil statement' error, got: %v", err)
	}
}

func TestEvalCondition_OrBothFalse(t *testing.T) {
	te := newTestTriggerExecutor()
	// or condition where left is false, then evaluates right (also false)
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &BinaryCondition{
			Op: "or",
			Left: &CompareExpr{
				Left: &QualifiedRef{Qualifier: "new", Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			},
			Right: &CompareExpr{
				Left: &QualifiedRef{Qualifier: "new", Name: "priority"}, Op: "=", Right: &IntLiteral{Value: 99},
			},
		},
		Deny: strPtr("blocked"),
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready", Priority: 3},
		New: &task.Task{ID: "TIKI-000001", Status: "ready", Priority: 3},
	}
	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("or should be false when both sides are false")
	}
}

func TestGuardSentinel_BothNilFallback(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{Old: nil, New: nil}
	sentinel := te.guardSentinel(tc)
	if sentinel == nil {
		t.Fatal("expected non-nil sentinel even when both old and new are nil")
		return
	}
	if sentinel.ID != "" {
		t.Errorf("expected empty ID on fallback sentinel, got %q", sentinel.ID)
	}
}

func TestResolveQualifiedRef_NewNilReturnsNilNil(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready"},
		New: nil, // simulates delete event
	}
	exec := te.newExecWithOverrides(tc)
	val, err := exec.resolveQualifiedRef(&QualifiedRef{Qualifier: "new", Name: "status"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil value for new.status when new is nil, got %v", val)
	}
}

func TestEvalExprRecursive_FieldRef(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001", Title: "my title"},
	}
	exec := te.newExecWithOverrides(tc)
	val, err := exec.evalExprRecursive(&FieldRef{Name: "title"}, tc.New, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "my title" {
		t.Errorf("expected 'my title', got %v", val)
	}
}

func TestEvalQuantifierOverride_AnyNoMatch(t *testing.T) {
	te := newTestTriggerExecutor()

	// any x in new.dependsOn where x.status = "done" — dep is "ready", so no match
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Where: &QuantifierExpr{
			Expr: &QualifiedRef{Qualifier: "new", Name: "dependsOn"},
			Kind: "any",
			Condition: &CompareExpr{
				Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			},
		},
		Deny: strPtr("blocked"),
	}

	dep := &task.Task{ID: "TIKI-DEP001", Status: "ready", Type: "story", Priority: 3}
	main := &task.Task{ID: "TIKI-000001", Status: "ready", Type: "story", Priority: 3, DependsOn: []string{"TIKI-DEP001"}}

	tc := &TriggerContext{Old: main, New: main, AllTasks: []*task.Task{dep, main}}
	ok, err := te.EvalGuard(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("any quantifier should return false when no dep is done")
	}
}

func TestEvalCountOverride_NonSubQueryArg(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	exec := te.newExecWithOverrides(tc)
	_, err := exec.evalCountOverride(&FunctionCall{
		Name: "count",
		Args: []Expr{&IntLiteral{Value: 42}},
	}, nil)
	if err == nil {
		t.Fatal("expected error for count() with non-SubQuery arg")
	}
	if !strings.Contains(err.Error(), "count() argument must be a select subquery") {
		t.Fatalf("expected 'count() argument must be a select subquery' error, got: %v", err)
	}
}

// --- ExecTimeTriggerAction ---

func TestExecTimeTriggerAction_Update(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	tt, err := p.ParseTimeTrigger(`every 1day update where status = "inProgress" set status="backlog"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tasks := []*task.Task{
		{ID: "TIKI-000001", Status: "inProgress", Title: "stale", Type: "story", Priority: 3},
		{ID: "TIKI-000002", Status: "done", Title: "finished", Type: "story", Priority: 3},
	}

	result, err := te.ExecTimeTriggerAction(tt, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil {
		t.Fatal("expected Update result")
	}
	if len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated task, got %d", len(result.Update.Updated))
	}
	if result.Update.Updated[0].Status != "backlog" {
		t.Fatalf("expected status=backlog, got %q", result.Update.Updated[0].Status)
	}
}

func TestExecTimeTriggerAction_Create(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	tt, err := p.ParseTimeTrigger(`every 1day create title="daily standup" status="ready" type="story" priority=3`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := te.ExecTimeTriggerAction(tt, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Create == nil {
		t.Fatal("expected Create result")
		return
	}
	if result.Create.Task.Title != "daily standup" {
		t.Fatalf("expected title='daily standup', got %q", result.Create.Task.Title)
	}
	if result.Create.Task.Status != "ready" {
		t.Fatalf("expected status=ready, got %q", result.Create.Task.Status)
	}
}

func TestExecTimeTriggerAction_Delete(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	tt, err := p.ParseTimeTrigger(`every 1day delete where status = "done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tasks := []*task.Task{
		{ID: "TIKI-000001", Status: "done", Title: "finished", Type: "story", Priority: 3},
		{ID: "TIKI-000002", Status: "ready", Title: "active", Type: "story", Priority: 3},
		{ID: "TIKI-000003", Status: "done", Title: "also done", Type: "story", Priority: 3},
	}

	result, err := te.ExecTimeTriggerAction(tt, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Delete == nil {
		t.Fatal("expected Delete result")
	}
	if len(result.Delete.Deleted) != 2 {
		t.Fatalf("expected 2 deleted tasks, got %d", len(result.Delete.Deleted))
	}
}

func TestExecTimeTriggerAction_NilAction(t *testing.T) {
	te := newTestTriggerExecutor()
	tt := &TimeTrigger{
		Interval: DurationLiteral{Value: 1, Unit: "day"},
		Action:   nil,
	}
	_, err := te.ExecTimeTriggerAction(tt, nil)
	if err == nil {
		t.Fatal("expected error for nil action")
	}
	if !strings.Contains(err.Error(), "time trigger has no action") {
		t.Fatalf("expected 'time trigger has no action' error, got: %v", err)
	}
}

// --- validateEventTriggerInput uncovered paths ---

func TestValidateEventTriggerInput_UnsealedValidatedTrigger(t *testing.T) {
	// pass a ValidatedTrigger with nil seal — mustBeSealed should fail
	vt := &ValidatedTrigger{} // seal is nil
	_, err := validateEventTriggerInput(vt)
	if err == nil {
		t.Fatal("expected error for unsealed ValidatedTrigger")
	}
	var unvalidated *UnvalidatedWrapperError
	if !errors.As(err, &unvalidated) {
		t.Fatalf("expected UnvalidatedWrapperError, got: %v", err)
	}
}

func TestValidateEventTriggerInput_WrongRuntime(t *testing.T) {
	// pass a ValidatedTrigger validated for plugin runtime (not eventTrigger)
	p := newTestParser()
	validated, err := p.ParseAndValidateTrigger(`before update deny "blocked"`, ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	_, err = validateEventTriggerInput(validated)
	if err == nil {
		t.Fatal("expected runtime mismatch error")
	}
	var mismatch *RuntimeMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected RuntimeMismatchError, got: %v", err)
	}
}

func TestValidateEventTriggerInput_UnsupportedType(t *testing.T) {
	_, err := validateEventTriggerInput("not a trigger")
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if !strings.Contains(err.Error(), "unsupported trigger type") {
		t.Fatalf("expected 'unsupported trigger type' error, got: %v", err)
	}

	_, err = validateEventTriggerInput(42)
	if err == nil {
		t.Fatal("expected error for int type")
	}
	if !strings.Contains(err.Error(), "unsupported trigger type") {
		t.Fatalf("expected 'unsupported trigger type' error, got: %v", err)
	}
}

// --- ExecAction uncovered paths ---

func TestExecAction_UnsealedValidatedTrigger(t *testing.T) {
	te := newTestTriggerExecutor()
	vt := &ValidatedTrigger{} // nil seal
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.ExecAction(vt, tc)
	if err == nil {
		t.Fatal("expected error for unsealed ValidatedTrigger")
	}
	var unvalidated *UnvalidatedWrapperError
	if !errors.As(err, &unvalidated) {
		t.Fatalf("expected UnvalidatedWrapperError, got: %v", err)
	}
}

func TestExecAction_ValidatedTriggerWrongRuntime(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()
	// validate for plugin runtime, which mismatches eventTrigger
	validated, err := p.ParseAndValidateTrigger(`after update update where status = "ready" set status="done"`, ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001", Status: "ready"},
		New:      &task.Task{ID: "TIKI-000001", Status: "done"},
		AllTasks: []*task.Task{{ID: "TIKI-000001", Status: "done"}},
	}
	_, err = te.ExecAction(validated, tc)
	if err == nil {
		t.Fatal("expected runtime mismatch error")
	}
	var mismatch *RuntimeMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected RuntimeMismatchError, got: %v", err)
	}
}

func TestExecAction_UnsupportedType(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.ExecAction("not a trigger", tc)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if !strings.Contains(err.Error(), "unsupported trigger type") {
		t.Fatalf("expected 'unsupported trigger type' error, got: %v", err)
	}
}

func TestExecAction_RawTriggerPath(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()
	trig, err := p.ParseTrigger(`after create update where id = new.id set status="inProgress"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	newTask := &task.Task{ID: "TIKI-000001", Title: "Test", Status: "ready", Type: "story", Priority: 3}
	tc := &TriggerContext{
		Old:      nil,
		New:      newTask,
		AllTasks: []*task.Task{newTask},
	}
	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatal("expected 1 updated task from raw trigger path")
		return
	}
	if result.Update.Updated[0].Status != "inProgress" {
		t.Fatalf("expected status=in_progress, got %q", result.Update.Updated[0].Status)
	}
}

// --- ExecTimeTriggerAction uncovered paths ---

func TestExecTimeTriggerAction_UnsealedValidatedTimeTrigger(t *testing.T) {
	te := newTestTriggerExecutor()
	vtt := &ValidatedTimeTrigger{} // nil seal
	_, err := te.ExecTimeTriggerAction(vtt, nil)
	if err == nil {
		t.Fatal("expected error for unsealed ValidatedTimeTrigger")
	}
	var unvalidated *UnvalidatedWrapperError
	if !errors.As(err, &unvalidated) {
		t.Fatalf("expected UnvalidatedWrapperError, got: %v", err)
	}
}

func TestExecTimeTriggerAction_ValidatedNilAction(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()
	// parse a valid time trigger, then tamper with the inner timeTrigger to nil its action
	// since we cannot tamper directly, we construct a trigger with action and validate it,
	// but the code requires Action != nil check. We test via raw path instead.
	tt := &TimeTrigger{
		Interval: DurationLiteral{Value: 1, Unit: "day"},
		Action: &Statement{
			Delete: &DeleteStmt{Where: &CompareExpr{
				Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			}},
		},
	}
	// first confirm this works
	_, err := te.ExecTimeTriggerAction(tt, []*task.Task{
		{ID: "TIKI-000001", Status: "done", Type: "story", Priority: 3},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// now test nil action on raw path
	ttNilAction := &TimeTrigger{
		Interval: DurationLiteral{Value: 1, Unit: "day"},
		Action:   nil,
	}
	_, err = te.ExecTimeTriggerAction(ttNilAction, nil)
	if err == nil {
		t.Fatal("expected error for nil action in raw TimeTrigger")
	}
	if !strings.Contains(err.Error(), "time trigger has no action") {
		t.Fatalf("expected 'time trigger has no action' error, got: %v", err)
	}

	_ = p // used for clarity only
}

func TestExecTimeTriggerAction_UnsupportedType(t *testing.T) {
	te := newTestTriggerExecutor()
	_, err := te.ExecTimeTriggerAction("not a time trigger", nil)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if !strings.Contains(err.Error(), "unsupported time trigger type") {
		t.Fatalf("expected 'unsupported time trigger type' error, got: %v", err)
	}
}

func TestExecTimeTriggerAction_RawTimeTriggerWithAction(t *testing.T) {
	te := newTestTriggerExecutor()
	tt := &TimeTrigger{
		Interval: DurationLiteral{Value: 1, Unit: "day"},
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "inProgress"},
				},
				Set: []Assignment{
					{Field: "status", Value: &StringLiteral{Value: "backlog"}},
				},
			},
		},
	}
	tasks := []*task.Task{
		{ID: "TIKI-000001", Status: "inProgress", Title: "stale", Type: "story", Priority: 3},
		{ID: "TIKI-000002", Status: "done", Title: "done", Type: "story", Priority: 3},
	}
	result, err := te.ExecTimeTriggerAction(tt, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatal("expected 1 updated task from raw TimeTrigger path")
		return
	}
	if result.Update.Updated[0].Status != "backlog" {
		t.Fatalf("expected status=backlog, got %q", result.Update.Updated[0].Status)
	}
}

// --- triggerExecOverride.Execute uncovered paths ---

func TestTriggerExecOverride_Execute_UnsupportedStatementType(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	exec := te.newExecWithOverrides(tc)
	// pass a string — unsupported type
	_, err := exec.Execute("not a statement", tc.AllTasks)
	if err == nil {
		t.Fatal("expected error for unsupported statement type")
	}
	if !strings.Contains(err.Error(), "unsupported statement type") {
		t.Fatalf("expected 'unsupported statement type' error, got: %v", err)
	}
}

func TestTriggerExecOverride_Execute_RawStatement(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001", Status: "ready"},
		New:      &task.Task{ID: "TIKI-000001", Status: "done"},
		AllTasks: []*task.Task{{ID: "TIKI-000001", Status: "done", Type: "story", Priority: 3}},
	}
	exec := te.newExecWithOverrides(tc)
	// pass a raw *Statement that should go through validation inside Execute
	rawStmt := &Statement{
		Update: &UpdateStmt{
			Where: &CompareExpr{
				Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "TIKI-000001"},
			},
			Set: []Assignment{
				{Field: "status", Value: &StringLiteral{Value: "backlog"}},
			},
		},
	}
	result, err := exec.Execute(rawStmt, tc.AllTasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatal("expected 1 updated task")
		return
	}
	if result.Update.Updated[0].Status != "backlog" {
		t.Fatalf("expected status=backlog, got %q", result.Update.Updated[0].Status)
	}
}

// --- evalExprRecursive default delegation tests ---

func TestEvalExprRecursive_StringLiteral(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		New: &task.Task{ID: "TIKI-000001"},
	}
	exec := te.newExecWithOverrides(tc)
	val, err := exec.evalExprRecursive(&StringLiteral{Value: "hello"}, tc.New, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "hello" {
		t.Errorf("expected 'hello', got %v", val)
	}
}

func TestEvalExprRecursive_IntLiteral(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		New: &task.Task{ID: "TIKI-000001"},
	}
	exec := te.newExecWithOverrides(tc)
	val, err := exec.evalExprRecursive(&IntLiteral{Value: 42}, tc.New, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Errorf("expected 42, got %v", val)
	}
}

func TestEvalExprRecursive_DateLiteral(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		New: &task.Task{ID: "TIKI-000001"},
	}
	exec := te.newExecWithOverrides(tc)
	date := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	val, err := exec.evalExprRecursive(&DateLiteral{Value: date}, tc.New, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != date {
		t.Errorf("expected %v, got %v", date, val)
	}
}

func TestEvalExprRecursive_DurationLiteral(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		New: &task.Task{ID: "TIKI-000001"},
	}
	exec := te.newExecWithOverrides(tc)
	dur := &DurationLiteral{Value: 2, Unit: "day"}
	val, err := exec.evalExprRecursive(dur, tc.New, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// the base executor converts DurationLiteral to time.Duration
	if val == nil {
		t.Fatal("expected non-nil duration value")
	}
}

func TestEvalExprRecursive_EmptyLiteral(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		New: &task.Task{ID: "TIKI-000001"},
	}
	exec := te.newExecWithOverrides(tc)
	val, err := exec.evalExprRecursive(&EmptyLiteral{}, tc.New, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil for EmptyLiteral, got %v", val)
	}
}

// --- resolveRefTasks with no matching tasks ---

func TestResolveRefTasks_NoMatches(t *testing.T) {
	tasks := []*task.Task{
		{ID: "TIKI-000001"},
		{ID: "TIKI-000002"},
	}
	refs := []interface{}{"TIKI-NOPE01", "TIKI-NOPE02"}
	resolved := resolveRefTasks(refs, tasks)
	if len(resolved) != 0 {
		t.Fatalf("expected 0 resolved tasks, got %d", len(resolved))
	}
}

// --- equalFoldID with different lengths ---

func TestEqualFoldID_DifferentLengths(t *testing.T) {
	if equalFoldID("TIKI-000001", "TIKI-00001") {
		t.Fatal("expected false for different lengths")
	}
	if equalFoldID("A", "AB") {
		t.Fatal("expected false for different lengths")
	}
	if equalFoldID("", "A") {
		t.Fatal("expected false for empty vs non-empty")
	}
	if !equalFoldID("", "") {
		t.Fatal("expected true for both empty")
	}
}

// --- ExecTimeTriggerAction with validated trigger that has Create requiring template ---

func TestExecTimeTriggerAction_ValidatedCreateRequiresTemplate(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	validated, err := p.ParseAndValidateTimeTrigger(`every 1day create title="daily" status="ready"`, ExecutorRuntimeTimeTrigger)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	// execute without providing a CreateTemplate — should trigger MissingCreateTemplateError
	_, err = te.ExecTimeTriggerAction(validated, nil)
	if err == nil {
		t.Fatal("expected error for create without template")
	}
	var missing *MissingCreateTemplateError
	if !errors.As(err, &missing) {
		t.Fatalf("expected MissingCreateTemplateError, got: %v", err)
	}
}

// --- triggerExecOverride.Execute empty statement ---

func TestTriggerExecOverride_Execute_EmptyStatement(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	exec := te.newExecWithOverrides(tc)
	// pass a raw Statement with all nil fields — should hit "unsupported trigger action type"
	_, err := exec.Execute(&Statement{}, tc.AllTasks)
	if err == nil {
		t.Fatal("expected error for empty statement")
	}
	if !strings.Contains(err.Error(), "empty statement") {
		t.Fatalf("expected 'empty statement' error, got: %v", err)
	}
}

// --- coverage gap: ExecAction with *ValidatedTrigger (create, update, delete) ---

func TestExecAction_ValidatedTriggerWithCreateAction(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	vt, err := p.ParseAndValidateTrigger(
		`after update where new.status = "done" create title="followup" status="ready"`,
		ExecutorRuntimeEventTrigger,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001", Status: "ready"},
		New:      &task.Task{ID: "TIKI-000001", Status: "done"},
		AllTasks: []*task.Task{{ID: "TIKI-000001", Status: "done"}},
	}
	result, err := te.ExecAction(vt, tc, ExecutionInput{
		CreateTemplate: &task.Task{Title: "template"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Create == nil {
		t.Fatal("expected Create result")
		return
	}
	if result.Create.Task.Title != "followup" {
		t.Fatalf("expected title 'followup', got %q", result.Create.Task.Title)
	}
	if result.Create.Task.Status != "ready" {
		t.Fatalf("expected status 'ready', got %q", result.Create.Task.Status)
	}
}

func TestExecAction_ValidatedTriggerWithUpdateAction(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	vt, err := p.ParseAndValidateTrigger(
		`after update where new.status = "done" update where status = "ready" set status="inProgress"`,
		ExecutorRuntimeEventTrigger,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready"},
		New: &task.Task{ID: "TIKI-000001", Status: "done"},
		AllTasks: []*task.Task{
			{ID: "TIKI-000001", Status: "done"},
			{ID: "TIKI-000002", Status: "ready"},
		},
	}
	result, err := te.ExecAction(vt, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil {
		t.Fatal("expected Update result")
	}
	if len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated task, got %d", len(result.Update.Updated))
	}
	if result.Update.Updated[0].Status != "inProgress" {
		t.Fatalf("expected status 'in_progress', got %q", result.Update.Updated[0].Status)
	}
}

func TestExecAction_ValidatedTriggerWithDeleteAction(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	vt, err := p.ParseAndValidateTrigger(
		`after update where new.status = "done" delete where status = "done"`,
		ExecutorRuntimeEventTrigger,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready"},
		New: &task.Task{ID: "TIKI-000001", Status: "done"},
		AllTasks: []*task.Task{
			{ID: "TIKI-000001", Status: "done"},
			{ID: "TIKI-000002", Status: "ready"},
		},
	}
	result, err := te.ExecAction(vt, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Delete == nil {
		t.Fatal("expected Delete result")
	}
	if len(result.Delete.Deleted) != 1 {
		t.Fatalf("expected 1 deleted task, got %d", len(result.Delete.Deleted))
	}
	if result.Delete.Deleted[0].ID != "TIKI-000001" {
		t.Fatalf("expected deleted TIKI-000001, got %s", result.Delete.Deleted[0].ID)
	}
}

func TestExecAction_ValidatedTriggerNoAction(t *testing.T) {
	te := newTestTriggerExecutor()
	// a validated trigger with no action (deny only)
	vt := &ValidatedTrigger{
		seal:    validatedSeal,
		runtime: ExecutorRuntimeEventTrigger,
		trigger: &Trigger{Timing: "before", Event: "update", Deny: strPtr("blocked")},
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.ExecAction(vt, tc)
	if err == nil {
		t.Fatal("expected error for validated trigger with no action")
	}
	if !strings.Contains(err.Error(), "trigger has no action") {
		t.Fatalf("expected 'trigger has no action' error, got: %v", err)
	}
}

// --- coverage gap: ExecTimeTriggerAction with *ValidatedTimeTrigger that succeeds ---

func TestExecTimeTriggerAction_ValidatedCreateWithTemplate(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	vtt, err := p.ParseAndValidateTimeTrigger(
		`every 1day create title="daily" status="ready"`,
		ExecutorRuntimeTimeTrigger,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	result, err := te.ExecTimeTriggerAction(vtt, nil, ExecutionInput{
		CreateTemplate: &task.Task{Title: "base", Type: "story", Priority: 3},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Create == nil {
		t.Fatal("expected Create result")
	}
	// title should be overridden by the trigger action
	if result.Create.Task.Title != "daily" {
		t.Fatalf("expected title 'daily', got %q", result.Create.Task.Title)
	}
	// type should be inherited from template
	if result.Create.Task.Type != "story" {
		t.Fatalf("expected type 'story' from template, got %q", result.Create.Task.Type)
	}
}

func TestExecTimeTriggerAction_ValidatedUpdate(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	vtt, err := p.ParseAndValidateTimeTrigger(
		`every 1day update where status = "inProgress" set status="backlog"`,
		ExecutorRuntimeTimeTrigger,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	tasks := []*task.Task{
		{ID: "TIKI-000001", Status: "inProgress", Type: "story", Priority: 3},
		{ID: "TIKI-000002", Status: "done", Type: "story", Priority: 3},
	}
	result, err := te.ExecTimeTriggerAction(vtt, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil {
		t.Fatal("expected Update result")
	}
	if len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated, got %d", len(result.Update.Updated))
	}
	if result.Update.Updated[0].Status != "backlog" {
		t.Fatalf("expected status 'backlog', got %q", result.Update.Updated[0].Status)
	}
}

func TestExecTimeTriggerAction_ValidatedDelete(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	vtt, err := p.ParseAndValidateTimeTrigger(
		`every 1day delete where status = "done"`,
		ExecutorRuntimeTimeTrigger,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	tasks := []*task.Task{
		{ID: "TIKI-000001", Status: "done", Type: "story", Priority: 3},
		{ID: "TIKI-000002", Status: "ready", Type: "story", Priority: 3},
	}
	result, err := te.ExecTimeTriggerAction(vtt, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Delete == nil {
		t.Fatal("expected Delete result")
	}
	if len(result.Delete.Deleted) != 1 {
		t.Fatalf("expected 1 deleted, got %d", len(result.Delete.Deleted))
	}
}

func TestExecTimeTriggerAction_ValidatedNilActionSealed(t *testing.T) {
	te := newTestTriggerExecutor()
	// construct a ValidatedTimeTrigger whose inner timeTrigger has nil action
	vtt := &ValidatedTimeTrigger{
		seal:    validatedSeal,
		runtime: ExecutorRuntimeTimeTrigger,
		timeTrigger: &TimeTrigger{
			Interval: DurationLiteral{Value: 1, Unit: "day"},
			Action:   nil,
		},
	}
	_, err := te.ExecTimeTriggerAction(vtt, nil)
	if err == nil {
		t.Fatal("expected error for validated time trigger with nil action")
	}
	if !strings.Contains(err.Error(), "time trigger has no action") {
		t.Fatalf("expected 'time trigger has no action' error, got: %v", err)
	}
}

// --- coverage gap: triggerExecOverride.Execute with *ValidatedStatement ---

func TestTriggerExecOverride_Execute_ValidatedStatement(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready"},
		New: &task.Task{ID: "TIKI-000001", Status: "done"},
		AllTasks: []*task.Task{
			{ID: "TIKI-000001", Status: "done", Type: "story", Priority: 3},
			{ID: "TIKI-000002", Status: "ready", Type: "story", Priority: 3},
		},
	}
	exec := te.newExecWithOverrides(tc)

	// construct a validated statement for the eventTrigger runtime
	vs := &ValidatedStatement{
		seal:    validatedSeal,
		runtime: ExecutorRuntimeEventTrigger,
		statement: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "ready"},
				},
				Set: []Assignment{
					{Field: "status", Value: &StringLiteral{Value: "inProgress"}},
				},
			},
		},
	}
	result, err := exec.Execute(vs, tc.AllTasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatal("expected 1 updated task")
		return
	}
	if result.Update.Updated[0].Status != "inProgress" {
		t.Fatalf("expected status 'in_progress', got %q", result.Update.Updated[0].Status)
	}
}

func TestTriggerExecOverride_Execute_ValidatedStatementRuntimeMismatch(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	exec := te.newExecWithOverrides(tc)

	// validated for plugin runtime, but override runs in eventTrigger runtime
	vs := &ValidatedStatement{
		seal:    validatedSeal,
		runtime: ExecutorRuntimePlugin,
		statement: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "TIKI-000001"},
				},
				Set: []Assignment{
					{Field: "status", Value: &StringLiteral{Value: "done"}},
				},
			},
		},
	}
	_, err := exec.Execute(vs, tc.AllTasks)
	if err == nil {
		t.Fatal("expected runtime mismatch error")
	}
	var mismatch *RuntimeMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected RuntimeMismatchError, got: %v", err)
	}
}

func TestTriggerExecOverride_Execute_ValidatedStatementCreateWithTemplate(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	exec := te.newExecWithOverrides(tc)

	// validated create statement requires template
	vs := &ValidatedStatement{
		seal:    validatedSeal,
		runtime: ExecutorRuntimeEventTrigger,
		statement: &Statement{
			Create: &CreateStmt{
				Assignments: []Assignment{
					{Field: "title", Value: &StringLiteral{Value: "created"}},
					{Field: "status", Value: &StringLiteral{Value: "ready"}},
				},
			},
		},
	}
	result, err := exec.Execute(vs, tc.AllTasks, ExecutionInput{
		CreateTemplate: &task.Task{Type: "story", Priority: 3},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Create == nil {
		t.Fatal("expected Create result")
		return
	}
	if result.Create.Task.Title != "created" {
		t.Fatalf("expected title 'created', got %q", result.Create.Task.Title)
	}
	// type should be inherited from template
	if result.Create.Task.Type != "story" {
		t.Fatalf("expected type 'story' from template, got %q", result.Create.Task.Type)
	}
}

func TestTriggerExecOverride_Execute_ValidatedCreateMissingTemplate(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	exec := te.newExecWithOverrides(tc)

	// validated create requires template, but none provided
	vs := &ValidatedStatement{
		seal:    validatedSeal,
		runtime: ExecutorRuntimeEventTrigger,
		statement: &Statement{
			Create: &CreateStmt{
				Assignments: []Assignment{
					{Field: "title", Value: &StringLiteral{Value: "test"}},
				},
			},
		},
	}
	_, err := exec.Execute(vs, tc.AllTasks)
	if err == nil {
		t.Fatal("expected MissingCreateTemplateError")
	}
	var missing *MissingCreateTemplateError
	if !errors.As(err, &missing) {
		t.Fatalf("expected MissingCreateTemplateError, got: %v", err)
	}
}

func TestTriggerExecOverride_Execute_RawCreateNoTemplate(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	exec := te.newExecWithOverrides(tc)

	// raw statement path: create without template should succeed (requireTemplate=false)
	rawStmt := &Statement{
		Create: &CreateStmt{
			Assignments: []Assignment{
				{Field: "title", Value: &StringLiteral{Value: "raw create"}},
				{Field: "status", Value: &StringLiteral{Value: "ready"}},
			},
		},
	}
	result, err := exec.Execute(rawStmt, tc.AllTasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Create == nil {
		t.Fatal("expected Create result")
		return
	}
	if result.Create.Task.Title != "raw create" {
		t.Fatalf("expected title 'raw create', got %q", result.Create.Task.Title)
	}
}

// --- coverage gap: ExecRun with Run.Command == nil ---

func TestExecRun_RunSetButCommandNil(t *testing.T) {
	te := newTestTriggerExecutor()
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Run:    &RunAction{Command: nil},
	}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	_, err := te.ExecRun(trig, tc)
	if err == nil {
		t.Fatal("expected error for run with nil command")
	}
	if !strings.Contains(err.Error(), "trigger has no run action") {
		t.Fatalf("expected 'trigger has no run action' error, got: %v", err)
	}
}

// --- coverage gap: validateEventTriggerInput raw *Trigger success path ---

func TestValidateEventTriggerInput_RawTriggerSuccess(t *testing.T) {
	p := newTestParser()
	trig, err := p.ParseTrigger(`after update where new.status = "done" update where id = new.id set status="cancelled"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	validated, err := validateEventTriggerInput(trig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if validated == nil {
		t.Fatal("expected non-nil validated trigger")
		return
	}
	if validated.RuntimeMode() != ExecutorRuntimeEventTrigger {
		t.Fatalf("expected eventTrigger runtime, got %q", validated.RuntimeMode())
	}
}

// --- coverage gap: evalExprRecursive QualifiedRef inside nested expression ---

func TestEvalExprRecursive_QualifiedRefInsideFunctionCall(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// blocks(new.id) — function call arg is a QualifiedRef
	trig, err := p.ParseTrigger(
		`after update where new.status = "done" update where id in blocks(new.id) set status="review"`,
	)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	completed := &task.Task{ID: "TIKI-000001", Status: "done"}
	blocker := &task.Task{
		ID: "TIKI-000002", Status: "ready",
		DependsOn: []string{"TIKI-000001"},
	}

	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001", Status: "inProgress"},
		New:      completed,
		AllTasks: []*task.Task{completed, blocker},
	}

	result, err := te.ExecAction(trig, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatal("expected 1 updated task")
		return
	}
	if result.Update.Updated[0].Status != "review" {
		t.Fatalf("expected status 'review', got %q", result.Update.Updated[0].Status)
	}
}

// --- coverage gap: executeUpdate override eval error in assignment value ---

func TestExecuteUpdate_Override_EvalErrorInAssignment(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001", Status: "ready"},
		New:      &task.Task{ID: "TIKI-000001", Status: "done"},
		AllTasks: []*task.Task{{ID: "TIKI-000001", Status: "done", Type: "story", Priority: 3}},
	}
	exec := te.newExecWithOverrides(tc)

	// update with an assignment whose value evaluation fails
	vs := &ValidatedStatement{
		seal:    validatedSeal,
		runtime: ExecutorRuntimeEventTrigger,
		statement: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "TIKI-000001"},
				},
				Set: []Assignment{
					{Field: "title", Value: &BinaryExpr{
						Op:    "*",
						Left:  &IntLiteral{Value: 1},
						Right: &IntLiteral{Value: 2},
					}},
				},
			},
		},
	}
	_, err := exec.Execute(vs, tc.AllTasks)
	if err == nil {
		t.Fatal("expected error for unknown binary operator in update set")
	}
	if !strings.Contains(err.Error(), "unknown binary operator") {
		t.Fatalf("expected 'unknown binary operator' error, got: %v", err)
	}
}

// --- coverage gap: executeCreate override eval error in assignment ---

func TestExecuteCreate_Override_EvalErrorInAssignment(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	exec := te.newExecWithOverrides(tc)

	vs := &ValidatedStatement{
		seal:    validatedSeal,
		runtime: ExecutorRuntimeEventTrigger,
		statement: &Statement{
			Create: &CreateStmt{
				Assignments: []Assignment{
					{Field: "title", Value: &BinaryExpr{
						Op:    "*",
						Left:  &IntLiteral{Value: 1},
						Right: &IntLiteral{Value: 2},
					}},
				},
			},
		},
	}
	_, err := exec.Execute(vs, tc.AllTasks, ExecutionInput{
		CreateTemplate: &task.Task{},
	})
	if err == nil {
		t.Fatal("expected error for unknown binary operator in create assignment")
	}
	if !strings.Contains(err.Error(), "unknown binary operator") {
		t.Fatalf("expected 'unknown binary operator' error, got: %v", err)
	}
}

// --- coverage gap: evalCountOverride error in condition ---

func TestEvalCountOverride_ErrorInConditionOverride(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	exec := te.newExecWithOverrides(tc)

	// count with a condition that uses an unknown qualifier
	_, err := exec.evalCountOverride(&FunctionCall{
		Name: "count",
		Args: []Expr{&SubQuery{Where: &CompareExpr{
			Left:  &QualifiedRef{Qualifier: "mid", Name: "status"},
			Op:    "=",
			Right: &StringLiteral{Value: "done"},
		}}},
	}, tc.AllTasks)
	if err == nil {
		t.Fatal("expected error for unknown qualifier in count condition")
	}
	if !strings.Contains(err.Error(), "unknown qualifier") {
		t.Fatalf("expected 'unknown qualifier' error, got: %v", err)
	}
}

// --- coverage gap: triggerExecOverride.Execute unsupported action (select) ---

func TestTriggerExecOverride_Execute_ValidatedUnsupportedAction(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	exec := te.newExecWithOverrides(tc)

	// a validated statement with Select (which trigger override doesn't support)
	vs := &ValidatedStatement{
		seal:    validatedSeal,
		runtime: ExecutorRuntimeEventTrigger,
		statement: &Statement{
			Select: &SelectStmt{},
		},
	}
	_, err := exec.Execute(vs, tc.AllTasks)
	if err == nil {
		t.Fatal("expected error for unsupported trigger action type")
	}
	if !strings.Contains(err.Error(), "unsupported trigger action type") {
		t.Fatalf("expected 'unsupported trigger action type' error, got: %v", err)
	}
}

// --- coverage gap: validateEventTriggerInput with valid *ValidatedTrigger ---

func TestEvalGuard_ValidatedTriggerPassesValidation(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	// validate a trigger for eventTrigger runtime, then pass to EvalGuard
	vt, err := p.ParseAndValidateTrigger(
		`before update where new.status = "done" deny "blocked"`,
		ExecutorRuntimeEventTrigger,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready"},
		New: &task.Task{ID: "TIKI-000001", Status: "done"},
	}
	ok, err := te.EvalGuard(vt, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("guard should match: new.status = done")
	}
}

func TestExecRun_ValidatedTriggerPassesValidation(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	vt, err := p.ParseAndValidateTrigger(
		`after update run("echo " + new.id)`,
		ExecutorRuntimeEventTrigger,
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001"},
		New: &task.Task{ID: "TIKI-000001"},
	}
	cmd, err := te.ExecRun(vt, tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "echo TIKI-000001" {
		t.Fatalf("expected 'echo TIKI-000001', got %q", cmd)
	}
}

// --- coverage gap: triggerExecOverride.Execute unsealed ValidatedStatement ---

func TestTriggerExecOverride_Execute_UnsealedValidatedStatement(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	exec := te.newExecWithOverrides(tc)

	// nil seal → mustBeSealed fails
	vs := &ValidatedStatement{
		seal:    nil,
		runtime: ExecutorRuntimeEventTrigger,
		statement: &Statement{
			Update: &UpdateStmt{
				Set: []Assignment{{Field: "status", Value: &StringLiteral{Value: "done"}}},
			},
		},
	}
	_, err := exec.Execute(vs, tc.AllTasks)
	if err == nil {
		t.Fatal("expected error for unsealed validated statement")
	}
	var unvalidated *UnvalidatedWrapperError
	if !errors.As(err, &unvalidated) {
		t.Fatalf("expected UnvalidatedWrapperError, got: %v", err)
	}
}

// --- coverage gap: executeUpdate override setField error ---

func TestExecuteUpdate_Override_SetFieldError(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001", Type: "story", Priority: 3}},
	}
	exec := te.newExecWithOverrides(tc)

	// update sets immutable field "createdBy" which triggers setField error
	vs := &ValidatedStatement{
		seal:    validatedSeal,
		runtime: ExecutorRuntimeEventTrigger,
		statement: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "TIKI-000001"},
				},
				Set: []Assignment{
					{Field: "createdBy", Value: &StringLiteral{Value: "hacker"}},
				},
			},
		},
	}
	_, err := exec.Execute(vs, tc.AllTasks)
	if err == nil {
		t.Fatal("expected error for immutable field in update set")
	}
	if !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected 'immutable' error, got: %v", err)
	}
}

// --- coverage gap: executeCreate override setField error ---

func TestExecuteCreate_Override_SetFieldError(t *testing.T) {
	te := newTestTriggerExecutor()
	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-000001"},
		New:      &task.Task{ID: "TIKI-000001"},
		AllTasks: []*task.Task{{ID: "TIKI-000001"}},
	}
	exec := te.newExecWithOverrides(tc)

	// create sets immutable field "createdBy" which triggers setField error
	vs := &ValidatedStatement{
		seal:    validatedSeal,
		runtime: ExecutorRuntimeEventTrigger,
		statement: &Statement{
			Create: &CreateStmt{
				Assignments: []Assignment{
					{Field: "title", Value: &StringLiteral{Value: "test"}},
					{Field: "createdBy", Value: &StringLiteral{Value: "hacker"}},
				},
			},
		},
	}
	_, err := exec.Execute(vs, tc.AllTasks, ExecutionInput{
		CreateTemplate: &task.Task{},
	})
	if err == nil {
		t.Fatal("expected error for immutable field in create assignment")
	}
	if !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected 'immutable' error, got: %v", err)
	}
}
