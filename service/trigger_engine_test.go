package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/config"
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
		"in progress": "inProgress",
		"inProgress":  "inProgress",
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

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
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

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// create a 2nd task for alice — count(alice tasks) = 2, should be allowed
	newTask := &task.Task{ID: "TIKI-CAP002", Title: "e2", Status: "ready", Assignee: "alice", Type: "story", Priority: 3}
	if err := gate.CreateTask(context.Background(), newTask); err != nil {
		t.Fatalf("unexpected denial: %v", err)
	}
}

func TestTriggerEngine_BeforeDeny(t *testing.T) {
	entry := parseTriggerEntry(t, "block completion with open deps",
		`before update where old.status = "inProgress" and new.status = "done" deny "cannot skip review"`)

	dep := &task.Task{ID: "TIKI-DEP001", Title: "dep", Status: "inProgress", Type: "story", Priority: 3}
	main := &task.Task{ID: "TIKI-MAIN01", Title: "main", Status: "inProgress", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(dep, main)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
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
		`before update where old.status = "inProgress" and new.status = "done" deny "no"`)

	tk := &task.Task{ID: "TIKI-000001", Title: "test", Status: "ready", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// move ready → in_progress — should NOT be denied
	updated := tk.Clone()
	updated.Status = "inProgress"
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
	existing1 := &task.Task{ID: "TIKI-WIP001", Title: "a1", Status: "inProgress", Assignee: "alice", Type: "story", Priority: 3}
	existing2 := &task.Task{ID: "TIKI-WIP002", Title: "a2", Status: "inProgress", Assignee: "alice", Type: "story", Priority: 3}
	target := &task.Task{ID: "TIKI-WIP003", Title: "a3", Status: "ready", Assignee: "alice", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(existing1, existing2, target)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// move target ready → in_progress — would be 3 in-progress for alice, should be denied
	updated := target.Clone()
	updated.Status = "inProgress"
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
	existing := &task.Task{ID: "TIKI-WIP001", Title: "a1", Status: "inProgress", Assignee: "alice", Type: "story", Priority: 3}
	target := &task.Task{ID: "TIKI-WIP002", Title: "a2", Status: "ready", Assignee: "alice", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(existing, target)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// move target ready → in_progress — only 2 in-progress, should be allowed
	updated := target.Clone()
	updated.Status = "inProgress"
	if err := gate.UpdateTask(context.Background(), updated); err != nil {
		t.Fatalf("unexpected denial: %v", err)
	}
}

// --- after-trigger tests ---

func TestTriggerEngine_AfterUpdateCascade(t *testing.T) {
	entry := parseTriggerEntry(t, "auto-assign urgent",
		`after create where new.priority <= 2 and new.assignee is empty update where id = new.id set assignee="autobot"`)

	gate, s := newGateWithStoreAndTasks()
	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
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
	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
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

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
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

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
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
		`after update where new.status = "inProgress" update where id = old.id set priority=new.priority`)

	tk := &task.Task{ID: "TIKI-LOOP01", Title: "loop", Status: "ready", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// update to in_progress — should cascade but not infinite loop
	updated := tk.Clone()
	updated.Status = "inProgress"
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
	updated := &task.Task{ID: "TIKI-000001", Title: "test", Status: "inProgress", Type: "story", Priority: 3}
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

	tk := &task.Task{ID: "TIKI-RUN001", Title: "run test", Status: "inProgress", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
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

	tk := &task.Task{ID: "TIKI-FAIL01", Title: "fail test", Status: "inProgress", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
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

	tk := &task.Task{ID: "TIKI-SLOW01", Title: "slow", Status: "inProgress", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
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
		ID: "TIKI-REC001", Title: "Daily standup", Status: "inProgress",
		Type: "story", Priority: 3, Recurrence: task.RecurrenceDaily,
	}
	gate, s := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
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

	tk := &task.Task{ID: "TIKI-PRIO01", Title: "critical", Status: "inProgress", Type: "story", Priority: 1}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
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

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
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

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
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

// --- addTrigger routing ---

func TestTriggerEngine_AddTriggerRouting(t *testing.T) {
	// build entries covering all 6 timing×event combinations
	entries := []triggerEntry{
		parseTriggerEntry(t, "bc", `before create deny "bc"`),
		parseTriggerEntry(t, "bu", `before update where old.status = "ready" deny "bu"`),
		parseTriggerEntry(t, "bd", `before delete where old.priority <= 1 deny "bd"`),
		parseTriggerEntry(t, "ac", `after create where new.priority = 1 update where id = new.id set title="ac"`),
		parseTriggerEntry(t, "au", `after update where new.status = "done" update where id = old.id set title="au"`),
		parseTriggerEntry(t, "ad", `after delete update where old.id in dependsOn set dependsOn=dependsOn - [old.id]`),
	}

	engine := NewTriggerEngine(entries, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))

	if len(engine.beforeCreate) != 1 {
		t.Errorf("beforeCreate: got %d, want 1", len(engine.beforeCreate))
	}
	if len(engine.beforeUpdate) != 1 {
		t.Errorf("beforeUpdate: got %d, want 1", len(engine.beforeUpdate))
	}
	if len(engine.beforeDelete) != 1 {
		t.Errorf("beforeDelete: got %d, want 1", len(engine.beforeDelete))
	}
	if len(engine.afterCreate) != 1 {
		t.Errorf("afterCreate: got %d, want 1", len(engine.afterCreate))
	}
	if len(engine.afterUpdate) != 1 {
		t.Errorf("afterUpdate: got %d, want 1", len(engine.afterUpdate))
	}
	if len(engine.afterDelete) != 1 {
		t.Errorf("afterDelete: got %d, want 1", len(engine.afterDelete))
	}
}

// --- before-trigger unconditional deny (no where clause) ---

func TestTriggerEngine_BeforeCreateUnconditionalDeny(t *testing.T) {
	entry := parseTriggerEntry(t, "block all creates",
		`before create deny "no new tasks allowed"`)

	gate, _ := newGateWithStoreAndTasks()
	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	tk := &task.Task{ID: "TIKI-NEW001", Title: "new", Status: "ready", Type: "story", Priority: 3}
	err := gate.CreateTask(context.Background(), tk)
	if err == nil {
		t.Fatal("expected denial")
	}
	if !strings.Contains(err.Error(), "no new tasks allowed") {
		t.Fatalf("expected denial message, got: %v", err)
	}
}

func TestTriggerEngine_BeforeDeleteUnconditionalDeny(t *testing.T) {
	entry := parseTriggerEntry(t, "block all deletes",
		`before delete deny "deletes are forbidden"`)

	tk := &task.Task{ID: "TIKI-DEL001", Title: "test", Status: "ready", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	err := gate.DeleteTask(context.Background(), tk)
	if err == nil {
		t.Fatal("expected denial")
	}
	if !strings.Contains(err.Error(), "deletes are forbidden") {
		t.Fatalf("expected denial message, got: %v", err)
	}
}

// --- LoadAndRegisterTriggers ---

func TestLoadAndRegisterTriggers_EmptyDefs(t *testing.T) {
	// no workflow files → empty defs → engine, 0, nil

	// isolate from real user/project workflow.yaml files
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	originalDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	_ = os.Chdir(t.TempDir())
	config.ResetPathManager()

	gate := NewTaskMutationGate()
	schema := testTriggerSchema{}
	engine, count, err := LoadAndRegisterTriggers(gate, schema, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 triggers loaded, got %d", count)
	}
	if engine == nil {
		t.Fatal("expected non-nil engine even with zero triggers")
	}
}

// --- coverage gap tests ---

func TestTriggerEngine_BeforeGuardEvalError(t *testing.T) {
	// before-trigger whose guard references an unknown qualifier ("mid.status")
	// should produce a rejection (fail-closed)
	entry := parseTriggerEntry(t, "broken guard",
		`before update where old.status = "ready" deny "blocked"`)
	// overwrite the parsed where with one that will fail at eval time:
	// use a QualifiedRef with unknown qualifier "mid"
	entry.trigger.Where = &ruki.CompareExpr{
		Left:  &ruki.QualifiedRef{Qualifier: "mid", Name: "status"},
		Op:    "=",
		Right: &ruki.StringLiteral{Value: "ready"},
	}

	tk := &task.Task{ID: "TIKI-ERR001", Title: "test", Status: "ready", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Status = "inProgress"
	err := gate.UpdateTask(context.Background(), updated)
	if err == nil {
		t.Fatal("expected rejection when guard eval fails")
	}
	if !strings.Contains(err.Error(), "guard evaluation failed") {
		t.Fatalf("expected 'guard evaluation failed' error, got: %v", err)
	}
}

func TestTriggerEngine_AfterGuardEvalError(t *testing.T) {
	// after-trigger whose guard evaluation fails (unknown qualifier)
	// should log and skip (not propagate error)
	entry := parseTriggerEntry(t, "broken after guard",
		`after update where new.status = "inProgress" update where id = new.id set title="updated"`)
	// overwrite the parsed where with one that will fail at eval time
	entry.trigger.Where = &ruki.CompareExpr{
		Left:  &ruki.QualifiedRef{Qualifier: "mid", Name: "status"},
		Op:    "=",
		Right: &ruki.StringLiteral{Value: "inProgress"},
	}

	tk := &task.Task{ID: "TIKI-ERR001", Title: "test", Status: "ready", Type: "story", Priority: 3}
	gate, s := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Status = "inProgress"
	// guard eval error is logged and skipped → mutation should succeed
	if err := gate.UpdateTask(context.Background(), updated); err != nil {
		t.Fatalf("unexpected error (guard eval error should be logged, not propagated): %v", err)
	}

	// the after-trigger should NOT have fired (guard errored → skipped)
	persisted := s.GetTask("TIKI-ERR001")
	if persisted.Title != "test" {
		t.Errorf("title should remain unchanged, got %q", persisted.Title)
	}
}

func TestTriggerEngine_ExecActionError(t *testing.T) {
	// after-trigger whose action execution fails
	// the error is logged by runAfterHooks, not propagated to the caller
	entry := parseTriggerEntry(t, "broken action",
		`after update where new.status = "inProgress" update where id = new.id set title="x"`)
	// overwrite the action with an empty statement to trigger exec error
	entry.trigger.Action = &ruki.Statement{}

	tk := &task.Task{ID: "TIKI-ERR002", Title: "test", Status: "ready", Type: "story", Priority: 3}
	gate, s := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Status = "inProgress"
	// after-hook errors are logged but not propagated — mutation succeeds
	if err := gate.UpdateTask(context.Background(), updated); err != nil {
		t.Fatalf("unexpected error (after-hook errors should be logged, not propagated): %v", err)
	}

	// the task should have been updated (the after-trigger's action failed, but the mutation itself succeeded)
	persisted := s.GetTask("TIKI-ERR002")
	if persisted.Status != "inProgress" {
		t.Errorf("expected status in_progress, got %q", persisted.Status)
	}
	// title should remain unchanged since the action failed
	if persisted.Title != "test" {
		t.Errorf("title should remain unchanged since action failed, got %q", persisted.Title)
	}
}

func TestTriggerEngine_AfterDeleteCascadeDelete(t *testing.T) {
	// exercises the persistResult delete branch:
	// when a task is deleted, also delete all tasks that depend on it
	entry := parseTriggerEntry(t, "cascade delete deps",
		`after delete delete where old.id in dependsOn`)

	parent := &task.Task{ID: "TIKI-PAR001", Title: "parent", Status: "done", Type: "story", Priority: 3}
	child := &task.Task{
		ID: "TIKI-CHI001", Title: "child", Status: "ready", Type: "story", Priority: 3,
		DependsOn: []string{"TIKI-PAR001"},
	}
	unrelated := &task.Task{ID: "TIKI-UNR001", Title: "unrelated", Status: "ready", Type: "story", Priority: 3}
	gate, s := newGateWithStoreAndTasks(parent, child, unrelated)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	if err := gate.DeleteTask(context.Background(), parent); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// parent should be gone
	if s.GetTask("TIKI-PAR001") != nil {
		t.Error("parent task should have been deleted")
	}
	// child should be gone (cascade delete)
	if s.GetTask("TIKI-CHI001") != nil {
		t.Error("child task should have been cascade-deleted")
	}
	// unrelated should remain
	if s.GetTask("TIKI-UNR001") == nil {
		t.Error("unrelated task should remain")
	}
}

// --- LoadAndRegisterTriggers full path ---

// setupTriggerLoadTest creates a temp environment for LoadAndRegisterTriggers tests.
// Returns the cwd where workflow.yaml should be written.
func setupTriggerLoadTest(t *testing.T) string {
	t.Helper()
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	if err := os.MkdirAll(filepath.Join(userDir, "tiki"), 0750); err != nil {
		t.Fatal(err)
	}

	cwdDir := t.TempDir()
	// create .doc so path manager recognizes this as a project root
	if err := os.MkdirAll(filepath.Join(cwdDir, ".doc"), 0750); err != nil {
		t.Fatal(err)
	}

	originalDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	_ = os.Chdir(cwdDir)

	config.ResetPathManager()
	return cwdDir
}

func TestLoadAndRegisterTriggers_WithValidTriggers(t *testing.T) {
	cwdDir := setupTriggerLoadTest(t)

	content := `triggers:
  - description: "block done"
    ruki: 'before update where new.status = "done" deny "no"'
  - description: "auto-assign"
    ruki: 'after create where new.assignee is empty update where id = new.id set assignee="bot"'
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	gate := NewTaskMutationGate()
	s := store.NewInMemoryStore()
	gate.SetStore(s)

	_, count, err := LoadAndRegisterTriggers(gate, testTriggerSchema{}, func() string { return "test-user" })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 triggers loaded, got %d", count)
	}

	// verify the before-trigger works: try moving a task to done
	tk := &task.Task{ID: "TIKI-LRT001", Title: "test", Status: "ready", Type: "story", Priority: 3}
	if err := gate.CreateTask(context.Background(), tk); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	updated := tk.Clone()
	updated.Status = "done"
	err = gate.UpdateTask(context.Background(), updated)
	if err == nil || !strings.Contains(err.Error(), "no") {
		t.Fatalf("expected 'no' denial from trigger, got: %v", err)
	}
}

func TestLoadAndRegisterTriggers_ParseError(t *testing.T) {
	cwdDir := setupTriggerLoadTest(t)

	content := `triggers:
  - description: "broken"
    ruki: 'before update where garbled %%% deny "no"'
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	gate := NewTaskMutationGate()
	gate.SetStore(store.NewInMemoryStore())

	engine, _, err := LoadAndRegisterTriggers(gate, testTriggerSchema{}, nil)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Fatalf("expected trigger description in error, got: %v", err)
	}
	if engine == nil {
		t.Fatal("expected non-nil engine even on error")
	}
}

func TestLoadAndRegisterTriggers_ParseErrorNoDescription(t *testing.T) {
	cwdDir := setupTriggerLoadTest(t)

	content := `triggers:
  - ruki: 'before update where garbled %%% deny "no"'
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	gate := NewTaskMutationGate()
	gate.SetStore(store.NewInMemoryStore())

	_, _, err := LoadAndRegisterTriggers(gate, testTriggerSchema{}, nil)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "#1") {
		t.Fatalf("expected fallback description '#1' in error, got: %v", err)
	}
}

// --- persistResult error branches ---

func TestTriggerEngine_PersistCreateTemplateError(t *testing.T) {
	// after-trigger that creates a task, but the store fails on NewTaskTemplate
	entry := parseTriggerEntry(t, "create on delete",
		`after delete create title="replacement" status="ready" type=old.type priority=3`)

	s := store.NewInMemoryStore()
	gate := NewTaskMutationGate()
	RegisterFieldValidators(gate)
	gate.SetStore(s)

	tk := &task.Task{ID: "TIKI-TPL001", Title: "original", Status: "ready", Type: "story", Priority: 3}
	if err := gate.CreateTask(context.Background(), tk); err != nil {
		t.Fatal(err)
	}

	// now swap to a failing template store
	gate.SetStore(&failingTemplateWrapper{Store: s})

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// delete triggers the after-hook which tries to create → template fails
	// after-hook errors are logged, not propagated, so we just verify no panic
	_ = gate.DeleteTask(context.Background(), tk)
}

type failingTemplateWrapper struct {
	store.Store
}

func (f *failingTemplateWrapper) NewTaskTemplate() (*task.Task, error) {
	return nil, fmt.Errorf("simulated template failure")
}

func TestTriggerEngine_PersistCreateGateError(t *testing.T) {
	// after-trigger that creates a valid task, but gate rejects via custom validator
	entry := parseTriggerEntry(t, "create on delete",
		`after delete create title="valid title" status="ready" type=old.type priority=3`)

	tk := &task.Task{ID: "TIKI-GCR001", Title: "original", Status: "ready", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	// add a custom create validator that rejects all trigger-created tasks
	gate.OnCreate(func(old, new *task.Task, allTasks []*task.Task) *Rejection {
		if new.Title == "valid title" {
			return &Rejection{Reason: "no trigger creates allowed"}
		}
		return nil
	})

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// delete triggers the after-hook which tries to create → gate rejects
	// after-hook errors are logged, not propagated
	_ = gate.DeleteTask(context.Background(), tk)
}

func TestTriggerEngine_PersistDeleteError(t *testing.T) {
	// after-trigger that deletes tasks, but gate rejects delete via before-delete trigger
	blockDelete := parseTriggerEntry(t, "block all deletes",
		`before delete deny "deletes forbidden"`)
	cascadeDelete := parseTriggerEntry(t, "cascade delete",
		`after update where new.status = "done" delete where id != old.id`)

	tk := &task.Task{ID: "TIKI-PDL001", Title: "main", Status: "ready", Type: "story", Priority: 3}
	other := &task.Task{ID: "TIKI-PDL002", Title: "other", Status: "ready", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk, other)

	entries := []triggerEntry{blockDelete, cascadeDelete}
	engine := NewTriggerEngine(entries, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// update tk to done → cascade tries to delete other → blocked by before-delete
	updated := tk.Clone()
	updated.Status = "done"
	// after-hook errors are logged, not propagated
	_ = gate.UpdateTask(context.Background(), updated)
}

func TestLoadAndRegisterTriggers_LoadDefError(t *testing.T) {
	cwdDir := setupTriggerLoadTest(t)

	// write an unreadable workflow.yaml to trigger a LoadTriggerDefs error
	f := filepath.Join(cwdDir, "workflow.yaml")
	if err := os.WriteFile(f, []byte("triggers: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f, 0000); err != nil {
		t.Skip("cannot change file permissions on this platform")
	}
	t.Cleanup(func() { _ = os.Chmod(f, 0600) })
	if r, openErr := os.Open(f); openErr == nil {
		_ = r.Close()
		t.Skip("chmod 0000 did not restrict read access on this platform")
	}

	gate := NewTaskMutationGate()
	gate.SetStore(store.NewInMemoryStore())

	engine, _, err := LoadAndRegisterTriggers(gate, testTriggerSchema{}, nil)
	if err == nil {
		t.Fatal("expected error for unreadable workflow.yaml")
	}
	if !strings.Contains(err.Error(), "loading trigger definitions") {
		t.Fatalf("expected 'loading trigger definitions' error, got: %v", err)
	}
	if engine == nil {
		t.Fatal("expected non-nil engine even on load error")
	}
}

func TestTriggerEngine_ExecRunEvalError(t *testing.T) {
	// after-trigger with run() that references an unknown qualifier
	entry := parseTriggerEntry(t, "broken run",
		`after update where new.status = "done" run("echo " + old.id)`)
	// overwrite run command to reference unknown qualifier
	entry.trigger.Run = &ruki.RunAction{
		Command: &ruki.QualifiedRef{Qualifier: "mid", Name: "title"},
	}

	tk := &task.Task{ID: "TIKI-RNE001", Title: "test", Status: "ready", Type: "story", Priority: 3}
	gate, _ := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Status = "done"
	// after-hook errors are logged, not propagated
	if err := gate.UpdateTask(context.Background(), updated); err != nil {
		t.Fatalf("unexpected error (run eval error should be logged): %v", err)
	}
}

// --- time trigger loading ---

func TestLoadAndRegisterTriggers_MixedEventAndTimeTriggers(t *testing.T) {
	cwdDir := setupTriggerLoadTest(t)

	content := `triggers:
  - description: "block done"
    ruki: 'before update where new.status = "done" deny "no"'
  - description: "stale cleanup"
    ruki: 'every 1hour update where status = "inProgress" and updatedAt < now() - 7day set status="backlog"'
  - description: "auto-assign"
    ruki: 'after create where new.assignee is empty update where id = new.id set assignee="bot"'
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	gate := NewTaskMutationGate()
	gate.SetStore(store.NewInMemoryStore())

	engine, count, err := LoadAndRegisterTriggers(gate, testTriggerSchema{}, func() string { return "test-user" })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 triggers loaded, got %d", count)
	}
	if engine == nil {
		t.Fatal("expected non-nil engine with mixed triggers")
	}
}

func TestLoadAndRegisterTriggers_TimeTriggerAccessor(t *testing.T) {
	p := ruki.NewParser(testTriggerSchema{})
	tt, err := p.ParseTimeTrigger(`every 1day delete where status = "done"`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "daily cleanup", Trigger: tt},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))

	result := engine.TimeTriggers()
	if len(result) != 1 {
		t.Fatalf("expected 1 time trigger, got %d", len(result))
	}
	if result[0].Description != "daily cleanup" {
		t.Fatalf("expected description 'daily cleanup', got %q", result[0].Description)
	}
	if result[0].Trigger.Interval.Value != 1 || result[0].Trigger.Interval.Unit != "day" {
		t.Fatalf("expected 1day interval, got %d%s", result[0].Trigger.Interval.Value, result[0].Trigger.Interval.Unit)
	}
}

func TestLoadAndRegisterTriggers_InvalidTimeTrigger(t *testing.T) {
	cwdDir := setupTriggerLoadTest(t)

	content := `triggers:
  - description: "broken time trigger"
    ruki: 'every 0day delete where status = "done"'
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	gate := NewTaskMutationGate()
	gate.SetStore(store.NewInMemoryStore())

	_, _, err := LoadAndRegisterTriggers(gate, testTriggerSchema{}, nil)
	if err == nil {
		t.Fatal("expected error for invalid time trigger")
	}
	if !strings.Contains(err.Error(), "broken time trigger") {
		t.Fatalf("expected trigger description in error, got: %v", err)
	}
}

func TestLoadAndRegisterTriggers_RunRejectedInTimeTrigger(t *testing.T) {
	cwdDir := setupTriggerLoadTest(t)

	content := `triggers:
  - description: "run not allowed"
    ruki: 'every 1hour run("echo hi")'
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	gate := NewTaskMutationGate()
	gate.SetStore(store.NewInMemoryStore())

	_, _, err := LoadAndRegisterTriggers(gate, testTriggerSchema{}, nil)
	if err == nil {
		t.Fatal("expected error for run() in time trigger")
	}
	if !strings.Contains(err.Error(), "run not allowed") {
		t.Fatalf("expected trigger description in error, got: %v", err)
	}
}

// --- StartScheduler tests ---

func TestTriggerEngine_StartScheduler_TickExecutes(t *testing.T) {
	// time trigger: delete all done tasks every 50ms
	p := ruki.NewParser(testTriggerSchema{})
	tt, err := p.ParseTimeTrigger(`every 1sec delete where status = "done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	doneTk := &task.Task{ID: "TIKI-DONE01", Title: "done task", Status: "done", Type: "story", Priority: 3}
	activeTk := &task.Task{ID: "TIKI-ACT001", Title: "active", Status: "ready", Type: "story", Priority: 3}
	gate, s := newGateWithStoreAndTasks(doneTk, activeTk)

	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "cleanup", Trigger: tt},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// override the interval to 50ms for fast test
	go engine.runTimeTrigger(ctx, engine.timeTriggers[0], 50*time.Millisecond)

	// wait long enough for at least one tick
	time.Sleep(200 * time.Millisecond)
	cancel()

	// done task should have been deleted
	if s.GetTask("TIKI-DONE01") != nil {
		t.Fatal("expected done task to be deleted by time trigger")
	}
	// active task should remain
	if s.GetTask("TIKI-ACT001") == nil {
		t.Fatal("expected active task to remain")
	}
}

func TestTriggerEngine_StartScheduler_NoTimeTriggers(t *testing.T) {
	// StartScheduler with no time triggers should return immediately without error
	engine := NewTriggerEngine(nil, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// should not block or panic
	engine.StartScheduler(ctx)
}

func TestTriggerEngine_StartScheduler_ContextCancellation(t *testing.T) {
	p := ruki.NewParser(testTriggerSchema{})
	tt, err := p.ParseTimeTrigger(`every 1day delete where status = "done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	gate, _ := newGateWithStoreAndTasks()
	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "daily cleanup", Trigger: tt},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		engine.runTimeTrigger(ctx, engine.timeTriggers[0], 1*time.Hour)
		close(done)
	}()

	// cancel immediately — goroutine should exit promptly
	cancel()
	select {
	case <-done:
		// success — goroutine exited
	case <-time.After(2 * time.Second):
		t.Fatal("runTimeTrigger did not exit after context cancellation")
	}
}

func TestTriggerEngine_StartScheduler_ActionErrorContinues(t *testing.T) {
	// time trigger with an action that will error on execution
	// (update with assignment to immutable field)
	p := ruki.NewParser(testTriggerSchema{})
	tt, err := p.ParseTimeTrigger(`every 1sec update where status = "ready" set createdBy="hacker"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tk := &task.Task{ID: "TIKI-ERR001", Title: "test", Status: "ready", Type: "story", Priority: 3}
	gate, s := newGateWithStoreAndTasks(tk)

	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "broken trigger", Trigger: tt},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// run with a short interval — the error should be swallowed and ticker continues
	go engine.runTimeTrigger(ctx, engine.timeTriggers[0], 50*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	cancel()

	// task should remain unchanged since the action errored
	persisted := s.GetTask("TIKI-ERR001")
	if persisted == nil {
		t.Fatal("task should still exist")
	}
	if persisted.CreatedBy != "" {
		t.Errorf("createdBy should be unchanged, got %q", persisted.CreatedBy)
	}
}

func TestTriggerEngine_StartScheduler_ValidTriggerRuns(t *testing.T) {
	// verify StartScheduler actually launches goroutines that execute the trigger
	p := ruki.NewParser(testTriggerSchema{})
	tt, err := p.ParseTimeTrigger(`every 1sec delete where status = "done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	doneTk := &task.Task{ID: "TIKI-SCH001", Title: "done task", Status: "done", Type: "story", Priority: 3}
	gate, s := newGateWithStoreAndTasks(doneTk)

	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "scheduler-test", Trigger: tt},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine.StartScheduler(ctx)
	// 1sec is the smallest parseable interval; wait long enough for one tick
	time.Sleep(1500 * time.Millisecond)
	cancel()

	if s.GetTask("TIKI-SCH001") != nil {
		t.Fatal("expected done task to be deleted by scheduler")
	}
}

func TestTriggerEngine_StartScheduler_InvalidIntervalSkipped(t *testing.T) {
	// construct a time trigger with an unrecognized unit — StartScheduler should
	// log an error and skip it without panicking or launching a goroutine
	tt := &ruki.TimeTrigger{
		Interval: ruki.DurationLiteral{Value: 1, Unit: "fortnights"},
		Action:   nil, // won't be reached
	}

	gate, _ := newGateWithStoreAndTasks()
	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "bad interval", Trigger: tt},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// should not panic or launch any goroutines
	engine.StartScheduler(ctx)
}

func TestTriggerEngine_ExecuteTimeTrigger_PersistError(t *testing.T) {
	// update time trigger where persistResult fails because a before-update
	// validator denies the mutation
	p := ruki.NewParser(testTriggerSchema{})
	tt, err := p.ParseTimeTrigger(`every 1sec update where status = "ready" set status="inProgress"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tk := &task.Task{ID: "TIKI-PER001", Title: "target", Status: "ready", Type: "story", Priority: 3}
	gate, s := newGateWithStoreAndTasks(tk)

	// register a before-update validator that always denies
	gate.OnUpdate(func(old, proposed *task.Task, all []*task.Task) *Rejection {
		return &Rejection{Reason: "update blocked by validator"}
	})

	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "persist-fail", Trigger: tt},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// run one tick — persist should fail (logged), ticker continues
	go engine.runTimeTrigger(ctx, engine.timeTriggers[0], 50*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	cancel()

	// task should remain unchanged since persist was rejected
	persisted := s.GetTask("TIKI-PER001")
	if persisted == nil {
		t.Fatal("task should still exist")
	}
	if persisted.Status != "ready" {
		t.Errorf("status should be unchanged, got %q", persisted.Status)
	}
}
