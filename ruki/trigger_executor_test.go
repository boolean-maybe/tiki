package ruki

import (
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
	if !strings.Contains(err.Error(), "unsupported trigger action type") {
		t.Fatalf("expected 'unsupported trigger action type' error, got: %v", err)
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

func strPtr(s string) *string { return &s }
