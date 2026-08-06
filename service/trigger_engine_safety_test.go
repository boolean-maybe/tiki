package service

import (
	"context"
	"strings"
	"testing"

	"github.com/boolean-maybe/ruki"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
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
	gate, _ := newGateWithStoreAndTikis()
	engine := NewTriggerEngine([]triggerEntry{entry}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, testTriggerDocFactory(), nil))
	engine.RegisterWithGate(gate)

	blocked := tikipkg.New()
	blocked.SetID("VAL001")
	blocked.SetTitle("should be blocked")
	blocked.Fields = map[string]interface{}{
		"status":   "ready",
		"type":     "story",
		"priority": "medium",
	}
	err = gate.CreateTiki(context.Background(), blocked)
	if err == nil {
		t.Fatal("expected create denial")
	}
	if !strings.Contains(err.Error(), "blocked by validated trigger") {
		t.Fatalf("expected validated deny message, got: %v", err)
	}
}

func TestTriggerEngine_EmptyEventEntryIsSkipped(t *testing.T) {
	gate, _ := newGateWithStoreAndTikis()
	engine := NewTriggerEngine([]triggerEntry{{description: "empty"}}, nil, ruki.NewTriggerExecutor(testTriggerSchema{}, testTriggerDocFactory(), nil))
	engine.RegisterWithGate(gate)

	if len(engine.beforeCreate)+len(engine.beforeUpdate)+len(engine.beforeDelete)+len(engine.afterCreate)+len(engine.afterUpdate)+len(engine.afterDelete) != 0 {
		t.Fatal("expected empty event entry to be skipped")
	}

	allowed := tikipkg.New()
	allowed.SetID("EMP001")
	allowed.SetTitle("allowed")
	allowed.Fields = map[string]interface{}{
		"status":   "ready",
		"type":     "story",
		"priority": "medium",
	}
	err := gate.CreateTiki(context.Background(), allowed)
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
}

func TestTriggerEngine_ValidatedOnlyTimeEntryExecutes(t *testing.T) {
	p := ruki.NewParser(testTriggerSchema{})
	validated, err := p.ParseAndValidateTimeTrigger(`every 1day create title="from validated time trigger" priority="medium" status="ready" type="story"`, ruki.ExecutorRuntimeTimeTrigger)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	gate, s := newGateWithStoreAndTikis()
	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{
			Description: "validated-time",
			Validated:   validated,
		},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, testTriggerDocFactory(), nil))
	engine.RegisterWithGate(gate)

	engine.executeTimeTrigger(context.Background(), engine.timeTriggers[0])

	all := s.GetAllTikis()
	if len(all) != 1 {
		t.Fatalf("expected one created tiki, got %d", len(all))
	}
	if all[0].Title() != "from validated time trigger" {
		t.Fatalf("expected created title to match, got %q", all[0].Title())
	}
}

func TestTriggerEngine_StartSchedulerEmptyTimeEntryNoPanic(t *testing.T) {
	gate, _ := newGateWithStoreAndTikis()
	engine := NewTriggerEngine(nil, []TimeTriggerEntry{
		{Description: "empty-time-entry"},
	}, ruki.NewTriggerExecutor(testTriggerSchema{}, testTriggerDocFactory(), nil))
	engine.RegisterWithGate(gate)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine.StartScheduler(ctx)
}
