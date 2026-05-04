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
	tikipkg "github.com/boolean-maybe/tiki/tiki"
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

// newTiki creates a workflow tiki with the given fields for tests.
func newTiki(id, title, status, typ string, priority int) *tikipkg.Tiki {
	tk := &tikipkg.Tiki{ID: id, Title: title}
	if status != "" {
		tk.Set(tikipkg.FieldStatus, status)
	}
	if typ != "" {
		tk.Set(tikipkg.FieldType, typ)
	}
	if priority != 0 {
		tk.Set(tikipkg.FieldPriority, priority)
	}
	return tk
}

func newGateWithStoreAndTikis(tikis ...*tikipkg.Tiki) (*TaskMutationGate, store.Store) {
	gate := NewTaskMutationGate()
	RegisterFieldValidators(gate)
	s := store.NewInMemoryStore()
	gate.SetStore(s)
	for _, tk := range tikis {
		if err := gate.CreateTiki(context.Background(), tk); err != nil {
			panic("setup: " + err.Error())
		}
	}
	return gate, s
}

// --- before-trigger tests ---

func TestTriggerEngine_BeforeCreateDenyAggregate(t *testing.T) {
	// task cap: deny when 3+ tasks for same assignee.
	entry := parseTriggerEntry(t, "task cap",
		`before create where count(select where assignee = new.assignee) >= 3 deny "task cap reached"`)

	existing1 := newTiki("CAP001", "e1", "ready", "story", 3)
	existing1.Set(tikipkg.FieldAssignee, "alice")
	existing2 := newTiki("CAP002", "e2", "ready", "story", 3)
	existing2.Set(tikipkg.FieldAssignee, "alice")
	gate, _ := newGateWithStoreAndTikis(existing1, existing2)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	newTask := newTiki("CAP003", "e3", "ready", "story", 3)
	newTask.Set(tikipkg.FieldAssignee, "alice")
	err := gate.CreateTiki(context.Background(), newTask)
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

	existing := newTiki("CAP001", "e1", "ready", "story", 3)
	existing.Set(tikipkg.FieldAssignee, "alice")
	gate, _ := newGateWithStoreAndTikis(existing)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	newTask := newTiki("CAP002", "e2", "ready", "story", 3)
	newTask.Set(tikipkg.FieldAssignee, "alice")
	if err := gate.CreateTiki(context.Background(), newTask); err != nil {
		t.Fatalf("unexpected denial: %v", err)
	}
}

func TestTriggerEngine_BeforeDeny(t *testing.T) {
	entry := parseTriggerEntry(t, "block completion with open deps",
		`before update where old.status = "inProgress" and new.status = "done" deny "cannot skip review"`)

	dep := newTiki("DEP001", "dep", "inProgress", "story", 3)
	main := newTiki("MAIN01", "main", "inProgress", "story", 3)
	gate, _ := newGateWithStoreAndTikis(dep, main)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := main.Clone()
	updated.Set(tikipkg.FieldStatus, "done")
	err := gate.UpdateTiki(context.Background(), updated)
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

	tk := newTiki("000001", "test", "ready", "story", 3)
	gate, _ := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Set(tikipkg.FieldStatus, "inProgress")
	if err := gate.UpdateTiki(context.Background(), updated); err != nil {
		t.Fatalf("unexpected denial: %v", err)
	}
}

func TestTriggerEngine_BeforeDenyWIPLimit(t *testing.T) {
	// WIP limit: deny when 3+ in-progress tasks for same assignee.
	entry := parseTriggerEntry(t, "WIP limit",
		`before update where new.status = "in progress" and count(select where assignee = new.assignee and status = "in progress") >= 3 deny "WIP limit reached"`)

	existing1 := newTiki("WIP001", "a1", "inProgress", "story", 3)
	existing1.Set(tikipkg.FieldAssignee, "alice")
	existing2 := newTiki("WIP002", "a2", "inProgress", "story", 3)
	existing2.Set(tikipkg.FieldAssignee, "alice")
	target := newTiki("WIP003", "a3", "ready", "story", 3)
	target.Set(tikipkg.FieldAssignee, "alice")
	gate, _ := newGateWithStoreAndTikis(existing1, existing2, target)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := target.Clone()
	updated.Set(tikipkg.FieldStatus, "inProgress")
	err := gate.UpdateTiki(context.Background(), updated)
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

	existing := newTiki("WIP001", "a1", "inProgress", "story", 3)
	existing.Set(tikipkg.FieldAssignee, "alice")
	target := newTiki("WIP002", "a2", "ready", "story", 3)
	target.Set(tikipkg.FieldAssignee, "alice")
	gate, _ := newGateWithStoreAndTikis(existing, target)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := target.Clone()
	updated.Set(tikipkg.FieldStatus, "inProgress")
	if err := gate.UpdateTiki(context.Background(), updated); err != nil {
		t.Fatalf("unexpected denial: %v", err)
	}
}

// --- after-trigger tests ---

func TestTriggerEngine_AfterUpdateCascade(t *testing.T) {
	entry := parseTriggerEntry(t, "auto-assign urgent",
		`after create where new.priority <= 2 and not has(new.assignee) update where id = new.id set assignee="autobot"`)

	gate, s := newGateWithStoreAndTikis()
	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	tk := newTiki("URGENT", "urgent bug", "ready", "bug", 1)
	if err := gate.CreateTiki(context.Background(), tk); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	persisted := s.GetTiki("URGENT")
	if persisted == nil {
		t.Fatal("tiki not found")
		return
	}
	assignee, _, _ := persisted.StringField(tikipkg.FieldAssignee)
	if assignee != "autobot" {
		t.Fatalf("expected assignee=autobot, got %q", assignee)
	}
}

func TestTriggerEngine_AfterTriggerNoMatchSkipped(t *testing.T) {
	entry := parseTriggerEntry(t, "auto-assign urgent",
		`after create where new.priority <= 2 and new.assignee is empty update where id = new.id set assignee="autobot"`)

	gate, s := newGateWithStoreAndTikis()
	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	tk := newTiki("LOWPRI", "low pri", "ready", "story", 5)
	if err := gate.CreateTiki(context.Background(), tk); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	persisted := s.GetTiki("LOWPRI")
	assignee, _, _ := persisted.StringField(tikipkg.FieldAssignee)
	if assignee != "" {
		t.Fatalf("expected empty assignee, got %q", assignee)
	}
}

func TestTriggerEngine_AfterDeleteCleanupDeps(t *testing.T) {
	entry := parseTriggerEntry(t, "cleanup deps on delete",
		`after delete update where has(dependsOn) and old.id in dependsOn set dependsOn=dependsOn - [old.id]`)

	dep := newTiki("DEP001", "dep", "done", "story", 3)
	downstream := newTiki("DOWN01", "downstream", "ready", "story", 3)
	downstream.Set(tikipkg.FieldDependsOn, []string{"DEP001", "OTHER1"})
	other := newTiki("OTHER1", "other", "done", "story", 3)
	gate, s := newGateWithStoreAndTikis(dep, downstream, other)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	if err := gate.DeleteTiki(context.Background(), dep); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	persisted := s.GetTiki("DOWN01")
	if persisted == nil {
		t.Fatal("downstream tiki missing")
		return
	}
	deps, _, _ := persisted.StringSliceField(tikipkg.FieldDependsOn)
	if len(deps) != 1 || deps[0] != "OTHER1" {
		t.Fatalf("expected dependsOn=[OTHER1], got %v", deps)
	}
}

func TestTriggerEngine_AfterCascadePartialFailureSurfaced(t *testing.T) {
	entry := parseTriggerEntry(t, "cascade to peers",
		`after create where new.priority = 1 update where assignee = new.assignee and id != new.id set priority=1`)

	peer1 := newTiki("PEER01", "peer1", "ready", "story", 5)
	peer1.Set(tikipkg.FieldAssignee, "alice")
	peer2 := newTiki("PEER02", "peer2", "ready", "story", 5)
	peer2.Set(tikipkg.FieldAssignee, "alice")
	gate, s := newGateWithStoreAndTikis(peer1, peer2)

	// register a validator that blocks updates to PEER02 specifically
	gate.OnUpdate(func(old, new *tikipkg.Tiki, allTikis []*tikipkg.Tiki) *Rejection {
		p, _, _ := new.IntField(tikipkg.FieldPriority)
		if new.ID == "PEER02" && p == 1 {
			return &Rejection{Reason: "peer2 blocked"}
		}
		return nil
	})

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	trigger := newTiki("TRIG01", "trigger", "ready", "story", 1)
	trigger.Set(tikipkg.FieldAssignee, "alice")
	if err := gate.CreateTiki(context.Background(), trigger); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	p1 := s.GetTiki("PEER01")
	p1pri, _, _ := p1.IntField(tikipkg.FieldPriority)
	if p1pri != 1 {
		t.Errorf("peer1 priority = %d, want 1 (cascade should succeed)", p1pri)
	}

	p2 := s.GetTiki("PEER02")
	p2pri, _, _ := p2.IntField(tikipkg.FieldPriority)
	if p2pri != 5 {
		t.Errorf("peer2 priority = %d, want 5 (cascade should have been blocked)", p2pri)
	}
}

// --- recursion limit ---

func TestTriggerEngine_RecursionLimit(t *testing.T) {
	entry := parseTriggerEntry(t, "infinite cascade",
		`after update where new.status = "inProgress" update where id = old.id set priority=new.priority`)

	tk := newTiki("LOOP01", "loop", "ready", "story", 3)
	gate, _ := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Set(tikipkg.FieldStatus, "inProgress")
	err := gate.UpdateTiki(context.Background(), updated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTriggerEngine_DepthExceededAtGateLevel(t *testing.T) {
	gate, _ := newGateWithStoreAndTikis(
		newTiki("000001", "test", "ready", "story", 3),
	)

	ctx := withTriggerDepth(context.Background(), maxTriggerDepth+1)
	updated := newTiki("000001", "test", "inProgress", "story", 3)
	err := gate.UpdateTiki(ctx, updated)
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

	tk := newTiki("RUN001", "run test", "inProgress", "story", 3)
	gate, _ := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Set(tikipkg.FieldStatus, "done")
	if err := gate.UpdateTiki(context.Background(), updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTriggerEngine_RunCommandFailure(t *testing.T) {
	skipOnWindows(t)
	entry := parseTriggerEntry(t, "failing command",
		`after update where new.status = "done" run("exit 1")`)

	tk := newTiki("FAIL01", "fail test", "inProgress", "story", 3)
	gate, _ := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Set(tikipkg.FieldStatus, "done")
	// run() failure is logged, not propagated — mutation should succeed
	if err := gate.UpdateTiki(context.Background(), updated); err != nil {
		t.Fatalf("unexpected error (run failure should be swallowed): %v", err)
	}
}

func TestTriggerEngine_RunCommandTimeout(t *testing.T) {
	skipOnWindows(t)
	entry := parseTriggerEntry(t, "slow command",
		`after update where new.status = "done" run("sleep 30")`)

	tk := newTiki("SLOW01", "slow", "inProgress", "story", 3)
	gate, _ := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	updated := tk.Clone()
	updated.Set(tikipkg.FieldStatus, "done")

	start := time.Now()
	if err := gate.UpdateTiki(ctx, updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("expected timeout to kill the command quickly, but took %v", elapsed)
	}
}

func TestTriggerEngine_AfterUpdateCreateWithNextDate(t *testing.T) {
	entry := parseTriggerEntry(t, "recurring follow-up",
		`after update where new.status = "done" and old.recurrence is not empty create title=old.title status="ready" type=old.type priority=old.priority due=next_date(old.recurrence)`)

	const recurrenceDaily = "0 0 * * *"

	tk := newTiki("REC001", "Daily standup", "inProgress", "story", 3)
	tk.Set(tikipkg.FieldRecurrence, recurrenceDaily)
	gate, s := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	before := time.Now()
	updated := tk.Clone()
	updated.Set(tikipkg.FieldStatus, "done")
	if err := gate.UpdateTiki(context.Background(), updated); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	after := time.Now()

	allTikis := s.GetAllTikis()
	if len(allTikis) < 2 {
		t.Fatalf("expected at least 2 tikis (original + created), got %d", len(allTikis))
	}

	var created *tikipkg.Tiki
	for _, at := range allTikis {
		if at.ID != "REC001" {
			created = at
			break
		}
	}
	if created == nil {
		t.Fatal("trigger-created tiki not found")
		return
	}
	if created.Title != "Daily standup" {
		t.Fatalf("expected title 'Daily standup', got %q", created.Title)
	}
	dueVal, ok, _ := created.TimeField(tikipkg.FieldDue)
	if !ok || dueVal.IsZero() {
		t.Fatal("expected non-zero due date from next_date(old.recurrence)")
	}
	expBefore := task.NextOccurrenceFrom(task.RecurrenceDaily, before)
	expAfter := task.NextOccurrenceFrom(task.RecurrenceDaily, after)
	if !dueVal.Equal(expBefore) && !dueVal.Equal(expAfter) {
		t.Fatalf("expected due=%v or %v, got %v", expBefore, expAfter, dueVal)
	}
}

// --- before-delete trigger ---

func TestTriggerEngine_BeforeDeleteDeny(t *testing.T) {
	entry := parseTriggerEntry(t, "block delete of high priority",
		`before delete where old.priority <= 2 deny "cannot delete high priority tasks"`)

	tk := newTiki("PRIO01", "critical", "inProgress", "story", 1)
	gate, _ := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	err := gate.DeleteTiki(context.Background(), tk)
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

	tk := newTiki("LOWP01", "low priority", "ready", "story", 5)
	gate, _ := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	if err := gate.DeleteTiki(context.Background(), tk); err != nil {
		t.Fatalf("unexpected denial: %v", err)
	}
}

// --- after-delete trigger creating new tiki ---

func TestTriggerEngine_AfterDeleteCascadeCreate(t *testing.T) {
	entry := parseTriggerEntry(t, "create archive on delete",
		`after delete create title="archived: " + old.title status="done" type=old.type priority=5`)

	tk := newTiki("ADEL01", "delete me", "ready", "bug", 3)
	gate, s := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	if err := gate.DeleteTiki(context.Background(), tk); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if s.GetTiki("ADEL01") != nil {
		t.Fatal("original tiki should have been deleted")
	}

	allTikis := s.GetAllTikis()
	if len(allTikis) < 1 {
		t.Fatal("expected at least 1 tiki (the archive placeholder)")
	}
	found := false
	for _, at := range allTikis {
		if strings.Contains(at.Title, "archived: delete me") {
			found = true
			status, _, _ := at.StringField(tikipkg.FieldStatus)
			if status != "done" {
				t.Errorf("expected status done, got %q", status)
			}
		}
	}
	if !found {
		t.Fatal("archive placeholder tiki not found")
	}
}

// --- addTrigger routing ---

func TestTriggerEngine_AddTriggerRouting(t *testing.T) {
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

	gate, _ := newGateWithStoreAndTikis()
	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	tk := newTiki("NEW001", "new", "ready", "story", 3)
	err := gate.CreateTiki(context.Background(), tk)
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

	tk := newTiki("DEL001", "test", "ready", "story", 3)
	gate, _ := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	err := gate.DeleteTiki(context.Background(), tk)
	if err == nil {
		t.Fatal("expected denial")
	}
	if !strings.Contains(err.Error(), "deletes are forbidden") {
		t.Fatalf("expected denial message, got: %v", err)
	}
}

// --- LoadAndRegisterTriggers ---

func TestLoadAndRegisterTriggers_EmptyDefs(t *testing.T) {
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
	entry := parseTriggerEntry(t, "broken guard",
		`before update where old.status = "ready" deny "blocked"`)
	entry.trigger.Where = &ruki.CompareExpr{
		Left:  &ruki.QualifiedRef{Qualifier: "mid", Name: "status"},
		Op:    "=",
		Right: &ruki.StringLiteral{Value: "ready"},
	}

	tk := newTiki("ERR001", "test", "ready", "story", 3)
	gate, _ := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Set(tikipkg.FieldStatus, "inProgress")
	err := gate.UpdateTiki(context.Background(), updated)
	if err == nil {
		t.Fatal("expected rejection when guard eval fails")
	}
	if !strings.Contains(err.Error(), "guard evaluation failed") {
		t.Fatalf("expected 'guard evaluation failed' error, got: %v", err)
	}
}

func TestTriggerEngine_AfterGuardEvalError(t *testing.T) {
	entry := parseTriggerEntry(t, "broken after guard",
		`after update where new.status = "inProgress" update where id = new.id set title="updated"`)
	entry.trigger.Where = &ruki.CompareExpr{
		Left:  &ruki.QualifiedRef{Qualifier: "mid", Name: "status"},
		Op:    "=",
		Right: &ruki.StringLiteral{Value: "inProgress"},
	}

	tk := newTiki("ERR001", "test", "ready", "story", 3)
	gate, s := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Set(tikipkg.FieldStatus, "inProgress")
	if err := gate.UpdateTiki(context.Background(), updated); err != nil {
		t.Fatalf("unexpected error (guard eval error should be logged, not propagated): %v", err)
	}

	persisted := s.GetTiki("ERR001")
	if persisted.Title != "test" {
		t.Errorf("title should remain unchanged, got %q", persisted.Title)
	}
}

func TestTriggerEngine_ExecActionError(t *testing.T) {
	entry := parseTriggerEntry(t, "broken action",
		`after update where new.status = "inProgress" update where id = new.id set title="x"`)
	entry.trigger.Action = &ruki.Statement{}

	tk := newTiki("ERR002", "test", "ready", "story", 3)
	gate, s := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Set(tikipkg.FieldStatus, "inProgress")
	if err := gate.UpdateTiki(context.Background(), updated); err != nil {
		t.Fatalf("unexpected error (after-hook errors should be logged, not propagated): %v", err)
	}

	persisted := s.GetTiki("ERR002")
	status, _, _ := persisted.StringField(tikipkg.FieldStatus)
	if status != "inProgress" {
		t.Errorf("expected status inProgress, got %q", status)
	}
	if persisted.Title != "test" {
		t.Errorf("title should remain unchanged since action failed, got %q", persisted.Title)
	}
}

func TestTriggerEngine_AfterDeleteCascadeDelete(t *testing.T) {
	entry := parseTriggerEntry(t, "cascade delete deps",
		`after delete delete where has(dependsOn) and old.id in dependsOn`)

	parent := newTiki("PAR001", "parent", "done", "story", 3)
	child := newTiki("CHI001", "child", "ready", "story", 3)
	child.Set(tikipkg.FieldDependsOn, []string{"PAR001"})
	unrelated := newTiki("UNR001", "unrelated", "ready", "story", 3)
	gate, s := newGateWithStoreAndTikis(parent, child, unrelated)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	if err := gate.DeleteTiki(context.Background(), parent); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if s.GetTiki("PAR001") != nil {
		t.Error("parent tiki should have been deleted")
	}
	if s.GetTiki("CHI001") != nil {
		t.Error("child tiki should have been cascade-deleted")
	}
	if s.GetTiki("UNR001") == nil {
		t.Error("unrelated tiki should remain")
	}
}

// --- LoadAndRegisterTriggers full path ---

func setupTriggerLoadTest(t *testing.T) string {
	t.Helper()
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	if err := os.MkdirAll(filepath.Join(userDir, "tiki"), 0750); err != nil {
		t.Fatal(err)
	}

	cwdDir := t.TempDir()
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

	tk := newTiki("LRT001", "test", "ready", "story", 3)
	if err := gate.CreateTiki(context.Background(), tk); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	updated := tk.Clone()
	updated.Set(tikipkg.FieldStatus, "done")
	err = gate.UpdateTiki(context.Background(), updated)
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
	entry := parseTriggerEntry(t, "create on delete",
		`after delete create title="replacement" status="ready" type=old.type priority=3`)

	s := store.NewInMemoryStore()
	gate := NewTaskMutationGate()
	RegisterFieldValidators(gate)
	gate.SetStore(s)

	tk := newTiki("TPL001", "original", "ready", "story", 3)
	if err := gate.CreateTiki(context.Background(), tk); err != nil {
		t.Fatal(err)
	}

	// now swap to a failing template store
	gate.SetStore(&failingTemplateWrapper{Store: s})

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// delete triggers the after-hook which tries to create → template fails
	// after-hook errors are logged, not propagated, so we just verify no panic
	_ = gate.DeleteTiki(context.Background(), tk)
}

type failingTemplateWrapper struct {
	store.Store
}

func (f *failingTemplateWrapper) NewTikiTemplate() (*tikipkg.Tiki, error) {
	return nil, fmt.Errorf("simulated template failure")
}

func TestTriggerEngine_PersistCreateGateError(t *testing.T) {
	entry := parseTriggerEntry(t, "create on delete",
		`after delete create title="valid title" status="ready" type=old.type priority=3`)

	tk := newTiki("GCR001", "original", "ready", "story", 3)
	gate, _ := newGateWithStoreAndTikis(tk)

	gate.OnCreate(func(old, new *tikipkg.Tiki, allTikis []*tikipkg.Tiki) *Rejection {
		if new.Title == "valid title" {
			return &Rejection{Reason: "no trigger creates allowed"}
		}
		return nil
	})

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	// after-hook errors are logged, not propagated
	_ = gate.DeleteTiki(context.Background(), tk)
}

func TestTriggerEngine_PersistDeleteError(t *testing.T) {
	blockDelete := parseTriggerEntry(t, "block all deletes",
		`before delete deny "deletes forbidden"`)
	cascadeDelete := parseTriggerEntry(t, "cascade delete",
		`after update where new.status = "done" delete where id != old.id`)

	tk := newTiki("PDL001", "main", "ready", "story", 3)
	other := newTiki("PDL002", "other", "ready", "story", 3)
	gate, _ := newGateWithStoreAndTikis(tk, other)

	entries := []triggerEntry{blockDelete, cascadeDelete}
	engine := NewTriggerEngine(entries, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Set(tikipkg.FieldStatus, "done")
	// after-hook errors are logged, not propagated
	_ = gate.UpdateTiki(context.Background(), updated)
}

func TestLoadAndRegisterTriggers_LoadDefError(t *testing.T) {
	cwdDir := setupTriggerLoadTest(t)

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
	entry := parseTriggerEntry(t, "broken run",
		`after update where new.status = "done" run("echo " + old.id)`)
	entry.trigger.Run = &ruki.RunAction{
		Command: &ruki.QualifiedRef{Qualifier: "mid", Name: "title"},
	}

	tk := newTiki("RNE001", "test", "ready", "story", 3)
	gate, _ := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	updated := tk.Clone()
	updated.Set(tikipkg.FieldStatus, "done")
	if err := gate.UpdateTiki(context.Background(), updated); err != nil {
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
	p := ruki.NewParser(testTriggerSchema{})
	tt, err := p.ParseTimeTrigger(`every 1sec delete where status = "done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	doneTk := newTiki("DONE01", "done task", "done", "story", 3)
	activeTk := newTiki("ACT001", "active", "ready", "story", 3)
	gate, s := newGateWithStoreAndTikis(doneTk, activeTk)

	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "cleanup", Trigger: tt},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go engine.runTimeTrigger(ctx, engine.timeTriggers[0], 50*time.Millisecond)

	time.Sleep(200 * time.Millisecond)
	cancel()

	if s.GetTiki("DONE01") != nil {
		t.Fatal("expected done tiki to be deleted by time trigger")
	}
	if s.GetTiki("ACT001") == nil {
		t.Fatal("expected active tiki to remain")
	}
}

func TestTriggerEngine_StartScheduler_NoTimeTriggers(t *testing.T) {
	engine := NewTriggerEngine(nil, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine.StartScheduler(ctx)
}

func TestTriggerEngine_StartScheduler_ContextCancellation(t *testing.T) {
	p := ruki.NewParser(testTriggerSchema{})
	tt, err := p.ParseTimeTrigger(`every 1day delete where status = "done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	gate, _ := newGateWithStoreAndTikis()
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

	cancel()
	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("runTimeTrigger did not exit after context cancellation")
	}
}

func TestTriggerEngine_StartScheduler_ActionErrorContinues(t *testing.T) {
	p := ruki.NewParser(testTriggerSchema{})
	tt, err := p.ParseTimeTrigger(`every 1sec update where status = "ready" set createdBy="hacker"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tk := newTiki("ERR001", "test", "ready", "story", 3)
	gate, s := newGateWithStoreAndTikis(tk)

	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "broken trigger", Trigger: tt},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go engine.runTimeTrigger(ctx, engine.timeTriggers[0], 50*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	cancel()

	persisted := s.GetTiki("ERR001")
	if persisted == nil {
		t.Fatal("tiki should still exist")
		return
	}
	createdBy, _, _ := persisted.StringField("createdBy")
	if createdBy != "" {
		t.Errorf("createdBy should be unchanged, got %q", createdBy)
	}
}

func TestTriggerEngine_StartScheduler_ValidTriggerRuns(t *testing.T) {
	p := ruki.NewParser(testTriggerSchema{})
	tt, err := p.ParseTimeTrigger(`every 1sec delete where status = "done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	doneTk := newTiki("SCH001", "done task", "done", "story", 3)
	gate, s := newGateWithStoreAndTikis(doneTk)

	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "scheduler-test", Trigger: tt},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine.StartScheduler(ctx)
	time.Sleep(1500 * time.Millisecond)
	cancel()

	if s.GetTiki("SCH001") != nil {
		t.Fatal("expected done tiki to be deleted by scheduler")
	}
}

func TestTriggerEngine_StartScheduler_InvalidIntervalSkipped(t *testing.T) {
	tt := &ruki.TimeTrigger{
		Interval: ruki.DurationLiteral{Value: 1, Unit: "fortnights"},
		Action:   nil,
	}

	gate, _ := newGateWithStoreAndTikis()
	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "bad interval", Trigger: tt},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine.StartScheduler(ctx)
}

func TestTriggerEngine_ExecuteTimeTrigger_PersistError(t *testing.T) {
	p := ruki.NewParser(testTriggerSchema{})
	tt, err := p.ParseTimeTrigger(`every 1sec update where status = "ready" set status="inProgress"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tk := newTiki("PER001", "target", "ready", "story", 3)
	gate, s := newGateWithStoreAndTikis(tk)

	gate.OnUpdate(func(old, proposed *tikipkg.Tiki, all []*tikipkg.Tiki) *Rejection {
		return &Rejection{Reason: "update blocked by validator"}
	})

	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "persist-fail", Trigger: tt},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go engine.runTimeTrigger(ctx, engine.timeTriggers[0], 50*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	cancel()

	persisted := s.GetTiki("PER001")
	if persisted == nil {
		t.Fatal("tiki should still exist")
		return
	}
	status, _, _ := persisted.StringField(tikipkg.FieldStatus)
	if status != "ready" {
		t.Errorf("status should be unchanged, got %q", status)
	}
}
