package ruki

import (
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
		Old: &task.Task{ID: "TIKI-000001", Status: "in_progress"},
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

	trig, err := p.ParseTrigger(`before update where old.status = "in_progress" and new.status = "done" deny "no skip"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "in_progress"},
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

func TestEvalGuard_QualifiedRefNoMatch(t *testing.T) {
	te := newTestTriggerExecutor()
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where old.status = "in_progress" and new.status = "done" deny "no"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "ready"},
		New: &task.Task{ID: "TIKI-000001", Status: "in_progress"},
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

	dep := &task.Task{ID: "TIKI-DEP001", Status: "in_progress"}
	tc := &TriggerContext{
		Old: &task.Task{ID: "TIKI-000001", Status: "in_progress", DependsOn: []string{"TIKI-DEP001"}},
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
		New: &task.Task{ID: "TIKI-000001", Status: "in_progress", Assignee: "alice"},
		AllTasks: []*task.Task{
			{ID: "TIKI-000001", Status: "in_progress", Assignee: "alice"},
			{ID: "TIKI-000002", Status: "in_progress", Assignee: "alice"},
			{ID: "TIKI-000003", Status: "in_progress", Assignee: "alice"},
			{ID: "TIKI-000004", Status: "in_progress", Assignee: "bob"},
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
		New: &task.Task{ID: "TIKI-000001", Status: "in_progress", Assignee: "alice"},
		AllTasks: []*task.Task{
			{ID: "TIKI-000001", Status: "in_progress", Assignee: "alice"},
			{ID: "TIKI-000002", Status: "in_progress", Assignee: "alice"},
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

	old := &task.Task{ID: "TIKI-000001", Title: "Daily standup", Status: "in_progress", Priority: 2, Tags: []string{"meeting"}}
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

	old := &task.Task{ID: "TIKI-000001", Title: "Stale", Status: "in_progress"}
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
		ID: "TIKI-EPIC01", Title: "Epic", Status: "in_progress", Type: "epic",
		DependsOn: []string{"TIKI-STORY1"},
	}

	tc := &TriggerContext{
		Old:      &task.Task{ID: "TIKI-STORY1", Status: "in_progress"},
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
		New: &task.Task{ID: "TIKI-000001", Status: "in_progress", Tags: []string{"claude"}},
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

	old := &task.Task{ID: "TIKI-000001", Title: "Daily standup", Status: "in_progress", Type: "story", Priority: 3, Recurrence: task.RecurrenceDaily}
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
