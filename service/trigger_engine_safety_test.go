package service

import (
	"context"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/task"
)

func TestTriggerEngine_ValidatedOnlyBeforeEntryDenies(t *testing.T) {
	p := ruki.NewParser(testTriggerSchema{})
	validated, err := p.ParseAndValidateTrigger(`before create deny "blocked by validated trigger"`, ruki.ExecutorRuntimeEventTrigger)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	entry := triggerEntry{
		description: "validated-only",
		validated:   validated,
	}
	gate, _ := newGateWithStoreAndTasks()
	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	err = gate.CreateTask(context.Background(), &task.Task{
		ID:       "TIKI-VAL001",
		Title:    "should be blocked",
		Status:   "ready",
		Type:     "story",
		Priority: 3,
	})
	if err == nil {
		t.Fatal("expected create denial")
	}
	if !strings.Contains(err.Error(), "blocked by validated trigger") {
		t.Fatalf("expected validated deny message, got: %v", err)
	}
}

func TestTriggerEngine_EmptyEventEntryIsSkipped(t *testing.T) {
	gate, _ := newGateWithStoreAndTasks()
	engine := NewTriggerEngine([]triggerEntry{{description: "empty"}}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	if len(engine.beforeCreate)+len(engine.beforeUpdate)+len(engine.beforeDelete)+len(engine.afterCreate)+len(engine.afterUpdate)+len(engine.afterDelete) != 0 {
		t.Fatal("expected empty event entry to be skipped")
	}

	err := gate.CreateTask(context.Background(), &task.Task{
		ID:       "TIKI-EMP001",
		Title:    "allowed",
		Status:   "ready",
		Type:     "story",
		Priority: 3,
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
}

func TestTriggerEngine_ValidatedOnlyTimeEntryExecutes(t *testing.T) {
	p := ruki.NewParser(testTriggerSchema{})
	validated, err := p.ParseAndValidateTimeTrigger(`every 1day create title="from validated time trigger" priority=3 status="ready" type="story"`, ruki.ExecutorRuntimeTimeTrigger)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	gate, s := newGateWithStoreAndTasks()
	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{
			Description: "validated-time",
			Validated:   validated,
		},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	engine.executeTimeTrigger(context.Background(), engine.timeTriggers[0])

	all := s.GetAllTasks()
	if len(all) != 1 {
		t.Fatalf("expected one created task, got %d", len(all))
	}
	if all[0].Title != "from validated time trigger" {
		t.Fatalf("expected created title to match, got %q", all[0].Title)
	}
}

func TestTriggerEngine_StartSchedulerEmptyTimeEntryNoPanic(t *testing.T) {
	gate, _ := newGateWithStoreAndTasks()
	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "empty-time-entry"},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine.StartScheduler(ctx)
}
