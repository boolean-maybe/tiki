package service

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// testTriggerSchema implements ruki.Schema for trigger engine tests.
type testTriggerSchema struct{}

func (testTriggerSchema) Field(name string) (ruki.FieldSpec, bool) {
	fields := map[string]ruki.FieldSpec{
		"id":          {Name: "id", Type: ruki.ValueID},
		"title":       {Name: "title", Type: ruki.ValueString},
		"description": {Name: "description", Type: ruki.ValueString},
		"status":      {Name: "status", Type: ruki.ValueStatus},
		"type":        {Name: "type", Type: ruki.ValueTaskType},
		"tags":        {Name: "tags", Type: ruki.ValueListString},
		"dependsOn":   {Name: "dependsOn", Type: ruki.ValueListRef},
		"due":         {Name: "due", Type: ruki.ValueDate},
		"recurrence":  {Name: "recurrence", Type: ruki.ValueRecurrence},
		"assignee":    {Name: "assignee", Type: ruki.ValueString},
		"priority":    {Name: "priority", Type: ruki.ValueInt},
		"points":      {Name: "points", Type: ruki.ValueInt},
		"createdBy":   {Name: "createdBy", Type: ruki.ValueString},
		"createdAt":   {Name: "createdAt", Type: ruki.ValueTimestamp},
		"updatedAt":   {Name: "updatedAt", Type: ruki.ValueTimestamp},
	}
	f, ok := fields[name]
	return f, ok
}

func (testTriggerSchema) NormalizeStatus(raw string) (string, bool) {
	valid := map[string]string{
		"backlog":     "backlog",
		"ready":       "ready",
		"in progress": "in_progress",
		"in_progress": "in_progress",
		"done":        "done",
		"cancelled":   "cancelled",
	}
	canonical, ok := valid[raw]
	return canonical, ok
}

func (testTriggerSchema) NormalizeType(raw string) (string, bool) {
	valid := map[string]string{
		"story": "story",
		"bug":   "bug",
		"spike": "spike",
		"epic":  "epic",
	}
	canonical, ok := valid[raw]
	return canonical, ok
}

func parseTriggerEntry(t *testing.T, desc, input string) triggerEntry {
	t.Helper()
	p := ruki.NewParser(testTriggerSchema{})
	trig, err := p.ParseTrigger(input)
	if err != nil {
		t.Fatalf("parse trigger %q: %v", desc, err)
	}
	return triggerEntry{description: desc, trigger: trig}
}

func newGateWithStoreAndTasks(tasks ...*task.Task) (*TaskMutationGate, store.Store) {
	gate := NewTaskMutationGate()
	RegisterFieldValidators(gate)
	s := store.NewInMemoryStore()
	gate.SetStore(s)
	for _, tk := range tasks {
		if err := gate.CreateTask(context.Background(), tk); err != nil {
			panic("setup: " + err.Error())
		}
	}
	return gate, s
}

// --- before-trigger tests ---

func TestTriggerEngine_BeforeCreateDenyAggregate(t *testing.T) {
	// task cap: deny when 3+ tasks for same assignee.
	// Regression: allTasks must include the proposed new task,
	// otherwise the count undercounts by one.
	entry := parseTriggerEntry(t, "task cap",
		`before create where count(select where assignee = new.assignee) >= 3 deny "task cap reached"`)

	existing1 := &task.Task{ID: "TIKI-CAP001", Title: "e1", Status: "ready", Assignee: "alice", Type: "story", Priority: 3}
	existing2 := &task.Task{ID: "TIKI-CAP002", Title: "e2", Status: "ready", Assignee: "alice", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(existing1, existing2)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// create a 3rd task for alice — count(alice tasks) = 3, should be denied
	newTask := &task.Task{ID: "TIKI-CAP003", Title: "e3", Status: "ready", Assignee: "alice", Type: "story", Priority: 3}
	err := gate.CreateTask(context.Background(), newTask)
	if err == nil {
		t.Fatal("expected task cap denial, got nil")
	}
	if !strings.Contains(err.Error(), "task cap reached") {
		t.Fatalf("expected task cap message, got: %v", err)
	}
}

func TestTriggerEngine_BeforeCreateAllowUnderAggregate(t *testing.T) {
	entry := parseTriggerEntry(t, "task cap",
		`before create where count(select where assignee = new.assignee) >= 3 deny "task cap reached"`)

	existing := &task.Task{ID: "TIKI-CAP001", Title: "e1", Status: "ready", Assignee: "alice", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(existing)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// create a 2nd task for alice — count(alice tasks) = 2, should be allowed
	newTask := &task.Task{ID: "TIKI-CAP002", Title: "e2", Status: "ready", Assignee: "alice", Type: "story", Priority: 3}
	if err := gate.CreateTask(context.Background(), newTask); err != nil {
		t.Fatalf("unexpected denial: %v", err)
	}
}

func TestTriggerEngine_BeforeDeny(t *testing.T) {
	entry := parseTriggerEntry(t, "block completion with open deps",
		`before update where old.status = "in_progress" and new.status = "done" deny "cannot skip review"`)

	dep := &task.Task{ID: "TIKI-DEP001", Title: "dep", Status: "in_progress", Type: "story", Priority: 3}
	main := &task.Task{ID: "TIKI-MAIN01", Title: "main", Status: "in_progress", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(dep, main)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// try to move main to done — should be denied
	updated := main.Clone()
	updated.Status = "done"
	err := gate.UpdateTask(context.Background(), updated)
	if err == nil {
		t.Fatal("expected denial, got nil")
	}
	if !strings.Contains(err.Error(), "cannot skip review") {
		t.Fatalf("expected denial message, got: %v", err)
	}
}

func TestTriggerEngine_BeforeDenyNoMatch(t *testing.T) {
	entry := parseTriggerEntry(t, "block completion",
		`before update where old.status = "in_progress" and new.status = "done" deny "no"`)

	tk := &task.Task{ID: "TIKI-000001", Title: "test", Status: "ready", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// move ready → in_progress — should NOT be denied
	updated := tk.Clone()
	updated.Status = "in_progress"
	if err := gate.UpdateTask(context.Background(), updated); err != nil {
		t.Fatalf("unexpected denial: %v", err)
	}
}

func TestTriggerEngine_BeforeDenyWIPLimit(t *testing.T) {
	// WIP limit: deny when 3+ in-progress tasks for same assignee.
	// Regression: allTasks must contain proposed values for the task being updated,
	// otherwise the count sees the old status and undercounts.
	entry := parseTriggerEntry(t, "WIP limit",
		`before update where new.status = "in progress" and count(select where assignee = new.assignee and status = "in progress") >= 3 deny "WIP limit reached"`)

	// two tasks already in_progress for alice, plus the one about to transition
	existing1 := &task.Task{ID: "TIKI-WIP001", Title: "a1", Status: "in_progress", Assignee: "alice", Type: "story", Priority: 3}
	existing2 := &task.Task{ID: "TIKI-WIP002", Title: "a2", Status: "in_progress", Assignee: "alice", Type: "story", Priority: 3}
	target := &task.Task{ID: "TIKI-WIP003", Title: "a3", Status: "ready", Assignee: "alice", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(existing1, existing2, target)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// move target ready → in_progress — would be 3 in-progress for alice, should be denied
	updated := target.Clone()
	updated.Status = "in_progress"
	err := gate.UpdateTask(context.Background(), updated)
	if err == nil {
		t.Fatal("expected WIP limit denial, got nil")
	}
	if !strings.Contains(err.Error(), "WIP limit reached") {
		t.Fatalf("expected WIP limit message, got: %v", err)
	}
}

func TestTriggerEngine_BeforeAllowUnderWIPLimit(t *testing.T) {
	entry := parseTriggerEntry(t, "WIP limit",
		`before update where new.status = "in progress" and count(select where assignee = new.assignee and status = "in progress") >= 3 deny "WIP limit reached"`)

	// only one task already in_progress for alice
	existing := &task.Task{ID: "TIKI-WIP001", Title: "a1", Status: "in_progress", Assignee: "alice", Type: "story", Priority: 3}
	target := &task.Task{ID: "TIKI-WIP002", Title: "a2", Status: "ready", Assignee: "alice", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(existing, target)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// move target ready → in_progress — only 2 in-progress, should be allowed
	updated := target.Clone()
	updated.Status = "in_progress"
	if err := gate.UpdateTask(context.Background(), updated); err != nil {
		t.Fatalf("unexpected denial: %v", err)
	}
}

// --- after-trigger tests ---

func TestTriggerEngine_AfterUpdateCascade(t *testing.T) {
	entry := parseTriggerEntry(t, "auto-assign urgent",
		`after create where new.priority <= 2 and new.assignee is empty update where id = new.id set assignee="autobot"`)

	gate, s := newGateWithStoreAndTasks()
	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// create an urgent task without assignee — trigger should auto-assign
	tk := &task.Task{ID: "TIKI-URGENT", Title: "urgent bug", Status: "ready", Type: "bug", Priority: 1}
	if err := gate.CreateTask(context.Background(), tk); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	persisted := s.GetTask("TIKI-URGENT")
	if persisted == nil {
		t.Fatal("task not found")
	}
	if persisted.Assignee != "autobot" {
		t.Fatalf("expected assignee=autobot, got %q", persisted.Assignee)
	}
}

func TestTriggerEngine_AfterTriggerNoMatchSkipped(t *testing.T) {
	entry := parseTriggerEntry(t, "auto-assign urgent",
		`after create where new.priority <= 2 and new.assignee is empty update where id = new.id set assignee="autobot"`)

	gate, s := newGateWithStoreAndTasks()
	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// create a low-priority task — trigger should NOT fire
	tk := &task.Task{ID: "TIKI-LOWPRI", Title: "low pri", Status: "ready", Type: "story", Priority: 5}
	if err := gate.CreateTask(context.Background(), tk); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	persisted := s.GetTask("TIKI-LOWPRI")
	if persisted.Assignee != "" {
		t.Fatalf("expected empty assignee, got %q", persisted.Assignee)
	}
}

func TestTriggerEngine_AfterDeleteCleanupDeps(t *testing.T) {
	entry := parseTriggerEntry(t, "cleanup deps on delete",
		`after delete update where old.id in dependsOn set dependsOn=dependsOn - [old.id]`)

	dep := &task.Task{ID: "TIKI-DEP001", Title: "dep", Status: "done", Type: "story", Priority: 3}
	downstream := &task.Task{
		ID: "TIKI-DOWN01", Title: "downstream", Status: "ready", Type: "story", Priority: 3,
		DependsOn: []string{"TIKI-DEP001", "TIKI-OTHER1"},
	}
	other := &task.Task{ID: "TIKI-OTHER1", Title: "other", Status: "done", Type: "story", Priority: 3}
	gate, s := newGateWithStoreAndTasks(dep, downstream, other)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// delete dep — should remove it from downstream's dependsOn
	if err := gate.DeleteTask(context.Background(), dep); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	persisted := s.GetTask("TIKI-DOWN01")
	if persisted == nil {
		t.Fatal("downstream task missing")
	}
	if len(persisted.DependsOn) != 1 || persisted.DependsOn[0] != "TIKI-OTHER1" {
		t.Fatalf("expected dependsOn=[TIKI-OTHER1], got %v", persisted.DependsOn)
	}
}

func TestTriggerEngine_AfterCascadePartialFailureSurfaced(t *testing.T) {
	// trigger: after create, update all tasks with same assignee to priority=1.
	// One of the target tasks has an invalid state (priority=0 after set, which
	// passes here), but we deliberately set up a validator that rejects priority=0.
	// The trigger should succeed for some tasks and fail for the blocked one.
	// Key assertion: the successful updates persist, the failed one doesn't.
	entry := parseTriggerEntry(t, "cascade to peers",
		`after create where new.priority = 1 update where assignee = new.assignee and id != new.id set priority=1`)

	peer1 := &task.Task{ID: "TIKI-PEER01", Title: "peer1", Status: "ready", Assignee: "alice", Type: "story", Priority: 5}
	peer2 := &task.Task{ID: "TIKI-PEER02", Title: "peer2", Status: "ready", Assignee: "alice", Type: "story", Priority: 5}
	gate, s := newGateWithStoreAndTasks(peer1, peer2)

	// register a validator that blocks updates to TIKI-PEER02 specifically
	gate.OnUpdate(func(old, new *task.Task, allTasks []*task.Task) *Rejection {
		if new.ID == "TIKI-PEER02" && new.Priority == 1 {
			return &Rejection{Reason: "peer2 blocked"}
		}
		return nil
	})

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// create a task that fires the trigger
	trigger := &task.Task{ID: "TIKI-TRIG01", Title: "trigger", Status: "ready", Assignee: "alice", Type: "story", Priority: 1}
	if err := gate.CreateTask(context.Background(), trigger); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// peer1 should have been updated (priority=1)
	p1 := s.GetTask("TIKI-PEER01")
	if p1.Priority != 1 {
		t.Errorf("peer1 priority = %d, want 1 (cascade should succeed)", p1.Priority)
	}

	// peer2 should NOT have been updated (blocked by validator)
	p2 := s.GetTask("TIKI-PEER02")
	if p2.Priority != 5 {
		t.Errorf("peer2 priority = %d, want 5 (cascade should have been blocked)", p2.Priority)
	}
}

// --- recursion limit ---

func TestTriggerEngine_RecursionLimit(t *testing.T) {
	// trigger that cascades indefinitely: every update triggers another update
	entry := parseTriggerEntry(t, "infinite cascade",
		`after update where new.status = "in_progress" update where id = old.id set priority=new.priority`)

	tk := &task.Task{ID: "TIKI-LOOP01", Title: "loop", Status: "ready", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// update to in_progress — should cascade but not infinite loop
	updated := tk.Clone()
	updated.Status = "in_progress"
	err := gate.UpdateTask(context.Background(), updated)
	// should not error — recursion limit is handled gracefully by skipping at depth
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTriggerEngine_DepthExceededAtGateLevel(t *testing.T) {
	gate, _ := newGateWithStoreAndTasks(
		&task.Task{ID: "TIKI-000001", Title: "test", Status: "ready", Type: "story", Priority: 3},
	)

	// simulate a context already at max+1 depth
	ctx := withTriggerDepth(context.Background(), maxTriggerDepth+1)
	updated := &task.Task{ID: "TIKI-000001", Title: "test", Status: "in_progress", Type: "story", Priority: 3}
	err := gate.UpdateTask(ctx, updated)
	if err == nil {
		t.Fatal("expected depth exceeded error")
	}
	if !strings.Contains(err.Error(), "cascade depth exceeded") {
		t.Fatalf("expected cascade depth error, got: %v", err)
	}
}

// --- run() trigger ---
// These tests invoke sh -c which requires a Unix shell.

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("run() triggers use sh -c, skipping on Windows")
	}
}

func TestTriggerEngine_RunCommand(t *testing.T) {
	skipOnWindows(t)
	entry := parseTriggerEntry(t, "echo trigger",
		`after update where new.status = "done" run("echo " + old.id)`)

	tk := &task.Task{ID: "TIKI-RUN001", Title: "run test", Status: "in_progress", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Status = "done"
	// should not error — command succeeds
	if err := gate.UpdateTask(context.Background(), updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTriggerEngine_RunCommandFailure(t *testing.T) {
	skipOnWindows(t)
	entry := parseTriggerEntry(t, "failing command",
		`after update where new.status = "done" run("exit 1")`)

	tk := &task.Task{ID: "TIKI-FAIL01", Title: "fail test", Status: "in_progress", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Status = "done"
	// run() failure is logged, not propagated — mutation should succeed
	if err := gate.UpdateTask(context.Background(), updated); err != nil {
		t.Fatalf("unexpected error (run failure should be swallowed): %v", err)
	}
}

func TestTriggerEngine_RunCommandTimeout(t *testing.T) {
	skipOnWindows(t)
	// use a run() trigger whose command outlives the parent context's deadline
	entry := parseTriggerEntry(t, "slow command",
		`after update where new.status = "done" run("sleep 30")`)

	tk := &task.Task{ID: "TIKI-SLOW01", Title: "slow", Status: "in_progress", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// parent context with a very short deadline — the 30s sleep will be killed
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	updated := tk.Clone()
	updated.Status = "done"

	start := time.Now()
	// mutation should succeed (run failures are logged, not propagated)
	if err := gate.UpdateTask(ctx, updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)

	// the command should have been killed quickly, not run for 30 seconds
	if elapsed > 5*time.Second {
		t.Fatalf("expected timeout to kill the command quickly, but took %v", elapsed)
	}
}

func TestTriggerEngine_AfterUpdateCreateWithNextDate(t *testing.T) {
	entry := parseTriggerEntry(t, "recurring follow-up",
		`after update where new.status = "done" and old.recurrence is not empty create title=old.title status="ready" type=old.type priority=old.priority due=next_date(old.recurrence)`)

	tk := &task.Task{
		ID: "TIKI-REC001", Title: "Daily standup", Status: "in_progress",
		Type: "story", Priority: 3, Recurrence: task.RecurrenceDaily,
	}
	gate, s := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	before := time.Now()
	updated := tk.Clone()
	updated.Status = "done"
	if err := gate.UpdateTask(context.Background(), updated); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	after := time.Now()

	allTasks := s.GetAllTasks()
	if len(allTasks) < 2 {
		t.Fatalf("expected at least 2 tasks (original + created), got %d", len(allTasks))
	}

	// find created task by predicate, not by slice index
	var created *task.Task
	for _, at := range allTasks {
		if at.ID != "TIKI-REC001" {
			created = at
			break
		}
	}
	if created == nil {
		t.Fatal("trigger-created task not found")
	}
	if created.Title != "Daily standup" {
		t.Fatalf("expected title 'Daily standup', got %q", created.Title)
	}
	if created.Due.IsZero() {
		t.Fatal("expected non-zero due date from next_date(old.recurrence)")
	}
	expBefore := task.NextOccurrenceFrom(task.RecurrenceDaily, before)
	expAfter := task.NextOccurrenceFrom(task.RecurrenceDaily, after)
	if !created.Due.Equal(expBefore) && !created.Due.Equal(expAfter) {
		t.Fatalf("expected due=%v or %v, got %v", expBefore, expAfter, created.Due)
	}
}

// --- before-delete trigger ---

func TestTriggerEngine_BeforeDeleteDeny(t *testing.T) {
	entry := parseTriggerEntry(t, "block delete of high priority",
		`before delete where old.priority <= 2 deny "cannot delete high priority tasks"`)

	tk := &task.Task{ID: "TIKI-PRIO01", Title: "critical", Status: "in_progress", Type: "story", Priority: 1}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	err := gate.DeleteTask(context.Background(), tk)
	if err == nil {
		t.Fatal("expected delete denial, got nil")
	}
	if !strings.Contains(err.Error(), "cannot delete high priority tasks") {
		t.Fatalf("expected denial message, got: %v", err)
	}
}

func TestTriggerEngine_BeforeDeleteAllow(t *testing.T) {
	entry := parseTriggerEntry(t, "block delete of high priority",
		`before delete where old.priority <= 2 deny "cannot delete high priority tasks"`)

	tk := &task.Task{ID: "TIKI-LOWP01", Title: "low priority", Status: "ready", Type: "story", Priority: 5}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	if err := gate.DeleteTask(context.Background(), tk); err != nil {
		t.Fatalf("unexpected denial: %v", err)
	}
}

// --- after-delete trigger creating new task ---

func TestTriggerEngine_AfterDeleteCascadeCreate(t *testing.T) {
	// when a task is deleted, create an archive placeholder
	entry := parseTriggerEntry(t, "create archive on delete",
		`after delete create title="archived: " + old.title status="done" type=old.type priority=5`)

	tk := &task.Task{ID: "TIKI-ADEL01", Title: "delete me", Status: "ready", Type: "bug", Priority: 3}
	gate, s := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	if err := gate.DeleteTask(context.Background(), tk); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// original should be gone
	if s.GetTask("TIKI-ADEL01") != nil {
		t.Fatal("original task should have been deleted")
	}

	// a new task should have been created
	allTasks := s.GetAllTasks()
	if len(allTasks) < 1 {
		t.Fatal("expected at least 1 task (the archive placeholder)")
	}
	found := false
	for _, at := range allTasks {
		if strings.Contains(at.Title, "archived: delete me") {
			found = true
			if at.Status != "done" {
				t.Errorf("expected status done, got %q", at.Status)
			}
		}
	}
	if !found {
		t.Fatal("archive placeholder task not found")
	}
}

// --- LoadAndRegisterTriggers ---

func TestLoadAndRegisterTriggers_EmptyDefs(t *testing.T) {
	// no workflow files → empty defs → 0, nil
	gate := NewTaskMutationGate()
	schema := testTriggerSchema{}
	count, err := LoadAndRegisterTriggers(gate, schema, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 triggers loaded, got %d", count)
	}
}
