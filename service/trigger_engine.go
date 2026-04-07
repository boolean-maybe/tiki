package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/task"
)

// maxTriggerDepth is the maximum cascade depth for triggers.
// Root mutation is depth 0; up to 8 cascades are allowed.
const maxTriggerDepth = 8

// runCommandTimeout is the timeout for run() commands executed by triggers.
const runCommandTimeout = 30 * time.Second

// triggerEntry holds a parsed trigger and its description for logging.
type triggerEntry struct {
	description string
	trigger     *ruki.Trigger
}

// TimeTriggerEntry holds a parsed time trigger and its description.
type TimeTriggerEntry struct {
	Description string
	Trigger     *ruki.TimeTrigger
}

// TriggerEngine bridges parsed triggers with the mutation gate.
// Before-triggers become MutationValidators, after-triggers become AfterHooks.
type TriggerEngine struct {
	beforeCreate []triggerEntry
	beforeUpdate []triggerEntry
	beforeDelete []triggerEntry
	afterCreate  []triggerEntry
	afterUpdate  []triggerEntry
	afterDelete  []triggerEntry
	timeTriggers []TimeTriggerEntry
	executor     *ruki.TriggerExecutor
	gate         *TaskMutationGate
}

// NewTriggerEngine creates a TriggerEngine from parsed event and time triggers.
func NewTriggerEngine(triggers []triggerEntry, timeTriggers []TimeTriggerEntry, executor *ruki.TriggerExecutor) *TriggerEngine {
	te := &TriggerEngine{timeTriggers: timeTriggers, executor: executor}
	for _, entry := range triggers {
		te.addTrigger(entry)
	}
	return te
}

func (te *TriggerEngine) addTrigger(entry triggerEntry) {
	trig := entry.trigger
	switch {
	case trig.Timing == "before" && trig.Event == "create":
		te.beforeCreate = append(te.beforeCreate, entry)
	case trig.Timing == "before" && trig.Event == "update":
		te.beforeUpdate = append(te.beforeUpdate, entry)
	case trig.Timing == "before" && trig.Event == "delete":
		te.beforeDelete = append(te.beforeDelete, entry)
	case trig.Timing == "after" && trig.Event == "create":
		te.afterCreate = append(te.afterCreate, entry)
	case trig.Timing == "after" && trig.Event == "update":
		te.afterUpdate = append(te.afterUpdate, entry)
	case trig.Timing == "after" && trig.Event == "delete":
		te.afterDelete = append(te.afterDelete, entry)
	}
}

// TimeTriggers returns the stored time trigger entries.
func (te *TriggerEngine) TimeTriggers() []TimeTriggerEntry {
	return te.timeTriggers
}

// RegisterWithGate wires the triggers into the gate as validators and hooks.
func (te *TriggerEngine) RegisterWithGate(gate *TaskMutationGate) {
	te.gate = gate

	// before-triggers become validators
	for _, entry := range te.beforeCreate {
		gate.OnCreate(te.makeBeforeValidator(entry))
	}
	for _, entry := range te.beforeUpdate {
		gate.OnUpdate(te.makeBeforeValidator(entry))
	}
	for _, entry := range te.beforeDelete {
		gate.OnDelete(te.makeBeforeValidator(entry))
	}

	// after-triggers become hooks
	for _, entry := range te.afterCreate {
		gate.OnAfterCreate(te.makeAfterHook(entry))
	}
	for _, entry := range te.afterUpdate {
		gate.OnAfterUpdate(te.makeAfterHook(entry))
	}
	for _, entry := range te.afterDelete {
		gate.OnAfterDelete(te.makeAfterHook(entry))
	}
}

// makeBeforeValidator creates a MutationValidator from a before-trigger.
// Fail-closed: guard evaluation errors produce a rejection.
func (te *TriggerEngine) makeBeforeValidator(entry triggerEntry) MutationValidator {
	return func(old, new *task.Task, allTasks []*task.Task) *Rejection {
		tc := &ruki.TriggerContext{Old: old, New: new, AllTasks: allTasks}
		match, err := te.executor.EvalGuard(entry.trigger, tc)
		if err != nil {
			return &Rejection{
				Reason: fmt.Sprintf("trigger %q guard evaluation failed: %v", entry.description, err),
			}
		}
		if match {
			return &Rejection{Reason: *entry.trigger.Deny}
		}
		return nil
	}
}

// makeAfterHook creates an AfterHook from an after-trigger.
// Guard evaluation errors are logged and the trigger is skipped.
func (te *TriggerEngine) makeAfterHook(entry triggerEntry) AfterHook {
	return func(ctx context.Context, old, new *task.Task) error {
		depth := triggerDepth(ctx)
		if depth >= maxTriggerDepth {
			slog.Warn("trigger cascade depth exceeded, skipping",
				"trigger", entry.description, "depth", depth)
			return nil
		}

		allTasks := te.gate.ReadStore().GetAllTasks()
		tc := &ruki.TriggerContext{Old: old, New: new, AllTasks: allTasks}

		match, err := te.executor.EvalGuard(entry.trigger, tc)
		if err != nil {
			slog.Error("after-trigger guard evaluation failed",
				"trigger", entry.description, "error", err)
			return nil
		}
		if !match {
			return nil
		}

		childCtx := withTriggerDepth(ctx, depth+1)

		if entry.trigger.Run != nil {
			return te.execRun(childCtx, entry, tc)
		}
		return te.execAction(childCtx, entry, tc)
	}
}

func (te *TriggerEngine) execAction(ctx context.Context, entry triggerEntry, tc *ruki.TriggerContext) error {
	result, err := te.executor.ExecAction(entry.trigger, tc)
	if err != nil {
		return fmt.Errorf("trigger %q action execution failed: %w", entry.description, err)
	}
	return te.persistResult(ctx, result)
}

func (te *TriggerEngine) persistResult(ctx context.Context, result *ruki.Result) error {
	var errs []error
	switch {
	case result.Update != nil:
		for _, t := range result.Update.Updated {
			if err := te.gate.UpdateTask(ctx, t); err != nil {
				errs = append(errs, fmt.Errorf("update %s: %w", t.ID, err))
			}
		}
	case result.Create != nil:
		t := result.Create.Task
		tmpl, err := te.gate.ReadStore().NewTaskTemplate()
		if err != nil {
			return fmt.Errorf("create template: %w", err)
		}
		t.ID = tmpl.ID
		t.CreatedBy = tmpl.CreatedBy
		if err := te.gate.CreateTask(ctx, t); err != nil {
			return fmt.Errorf("trigger create failed: %w", err)
		}
	case result.Delete != nil:
		for _, t := range result.Delete.Deleted {
			if err := te.gate.DeleteTask(ctx, t); err != nil {
				errs = append(errs, fmt.Errorf("delete %s: %w", t.ID, err))
			}
		}
	}
	return errors.Join(errs...)
}

func (te *TriggerEngine) execRun(ctx context.Context, entry triggerEntry, tc *ruki.TriggerContext) error {
	cmdStr, err := te.executor.ExecRun(entry.trigger, tc)
	if err != nil {
		return fmt.Errorf("trigger %q run evaluation failed: %w", entry.description, err)
	}

	runCtx, cancel := context.WithTimeout(ctx, runCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "sh", "-c", cmdStr) //nolint:gosec // cmdStr is a user-configured trigger action, intentionally dynamic
	setProcessGroup(cmd)
	cmd.WaitDelay = 3 * time.Second
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("trigger run() command failed",
			"trigger", entry.description,
			"command", cmdStr,
			"output", string(output),
			"error", err)
		return nil // logged, chain continues
	}

	slog.Info("trigger run() command succeeded",
		"trigger", entry.description,
		"command", cmdStr)
	return nil
}

// LoadAndRegisterTriggers loads trigger definitions from workflow.yaml, parses them,
// and registers them with the gate. Returns the number of triggers loaded.
// Fails fast on parse errors — a bad trigger blocks startup.
func LoadAndRegisterTriggers(gate *TaskMutationGate, schema ruki.Schema, userFunc func() string) (int, error) {
	defs, err := config.LoadTriggerDefs()
	if err != nil {
		return 0, fmt.Errorf("loading trigger definitions: %w", err)
	}
	if len(defs) == 0 {
		return 0, nil
	}

	parser := ruki.NewParser(schema)
	var eventEntries []triggerEntry
	var timeEntries []TimeTriggerEntry

	for i, def := range defs {
		desc := def.Description
		if desc == "" {
			desc = fmt.Sprintf("#%d", i+1)
		}

		rule, err := parser.ParseRule(def.Ruki)
		if err != nil {
			return 0, fmt.Errorf("trigger %q: %w", desc, err)
		}

		switch {
		case rule.TimeTrigger != nil:
			timeEntries = append(timeEntries, TimeTriggerEntry{
				Description: def.Description,
				Trigger:     rule.TimeTrigger,
			})
		case rule.Trigger != nil:
			eventEntries = append(eventEntries, triggerEntry{
				description: def.Description,
				trigger:     rule.Trigger,
			})
		}
	}

	executor := ruki.NewTriggerExecutor(schema, userFunc)
	engine := NewTriggerEngine(eventEntries, timeEntries, executor)
	engine.RegisterWithGate(gate)

	total := len(eventEntries) + len(timeEntries)
	slog.Info("triggers loaded", "event", len(eventEntries), "time", len(timeEntries))

	return total, nil
}
