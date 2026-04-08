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
	"github.com/boolean-maybe/tiki/util/duration"
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
	validated   *ruki.ValidatedTrigger
}

// TimeTriggerEntry holds a parsed time trigger and its description.
type TimeTriggerEntry struct {
	Description string
	Trigger     *ruki.TimeTrigger
	Validated   *ruki.ValidatedTimeTrigger
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
	timing, event, ok := triggerTimingEvent(entry)
	if !ok {
		slog.Warn("skipping trigger with missing timing/event metadata",
			"trigger", entry.description)
		return
	}
	switch {
	case timing == "before" && event == "create":
		te.beforeCreate = append(te.beforeCreate, entry)
	case timing == "before" && event == "update":
		te.beforeUpdate = append(te.beforeUpdate, entry)
	case timing == "before" && event == "delete":
		te.beforeDelete = append(te.beforeDelete, entry)
	case timing == "after" && event == "create":
		te.afterCreate = append(te.afterCreate, entry)
	case timing == "after" && event == "update":
		te.afterUpdate = append(te.afterUpdate, entry)
	case timing == "after" && event == "delete":
		te.afterDelete = append(te.afterDelete, entry)
	default:
		slog.Warn("skipping trigger with unsupported timing/event",
			"trigger", entry.description, "timing", timing, "event", event)
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
		match, err := te.executor.EvalGuard(eventTriggerForExec(entry), tc)
		if err != nil {
			return &Rejection{
				Reason: fmt.Sprintf("trigger %q guard evaluation failed: %v", entry.description, err),
			}
		}
		if match {
			if msg, ok := triggerDenyMessage(eventTriggerForExec(entry)); ok {
				return &Rejection{Reason: msg}
			}
			return &Rejection{Reason: "trigger rejected"}
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

		match, err := te.executor.EvalGuard(eventTriggerForExec(entry), tc)
		if err != nil {
			slog.Error("after-trigger guard evaluation failed",
				"trigger", entry.description, "error", err)
			return nil
		}
		if !match {
			return nil
		}

		childCtx := withTriggerDepth(ctx, depth+1)

		if triggerHasRunAction(eventTriggerForExec(entry)) {
			return te.execRun(childCtx, entry, tc)
		}
		return te.execAction(childCtx, entry, tc)
	}
}

func (te *TriggerEngine) execAction(ctx context.Context, entry triggerEntry, tc *ruki.TriggerContext) error {
	input := ruki.ExecutionInput{}
	if triggerRequiresCreateTemplate(eventTriggerForExec(entry)) {
		tmpl, err := te.gate.ReadStore().NewTaskTemplate()
		if err != nil {
			return fmt.Errorf("create template: %w", err)
		}
		if tmpl == nil {
			return fmt.Errorf("create template: store returned nil template")
		}
		input.CreateTemplate = tmpl
	}
	result, err := te.executor.ExecAction(eventTriggerForExec(entry), tc, input)
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
	cmdStr, err := te.executor.ExecRun(eventTriggerForExec(entry), tc)
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
// and registers them with the gate. Returns the engine (always non-nil), the number
// of triggers loaded, and any error. Callers can call StartScheduler on the engine
// without nil-checking — it early-returns on zero time triggers.
// Fails fast on parse errors — a bad trigger blocks startup.
func LoadAndRegisterTriggers(gate *TaskMutationGate, schema ruki.Schema, userFunc func() string) (*TriggerEngine, int, error) {
	executor := ruki.NewTriggerExecutor(schema, userFunc)
	empty := func() *TriggerEngine { return NewTriggerEngine(nil, nil, executor) }

	defs, err := config.LoadTriggerDefs()
	if err != nil {
		return empty(), 0, fmt.Errorf("loading trigger definitions: %w", err)
	}

	if len(defs) == 0 {
		return empty(), 0, nil
	}

	parser := ruki.NewParser(schema)
	var eventEntries []triggerEntry
	var timeEntries []TimeTriggerEntry

	for i, def := range defs {
		desc := def.Description
		if desc == "" {
			desc = fmt.Sprintf("#%d", i+1)
		}

		rule, err := parser.ParseAndValidateRule(def.Ruki)
		if err != nil {
			return empty(), 0, fmt.Errorf("trigger %q: %w", desc, err)
		}

		switch r := rule.(type) {
		case ruki.ValidatedTimeRule:
			vtt := r.TimeTrigger()
			timeEntries = append(timeEntries, TimeTriggerEntry{
				Description: def.Description,
				Trigger:     cloneTimeTriggerForService(vtt.TimeTriggerClone()),
				Validated:   vtt,
			})
		case ruki.ValidatedEventRule:
			vt := r.Trigger()
			eventEntries = append(eventEntries, triggerEntry{
				description: def.Description,
				trigger:     cloneTriggerForService(vt.TriggerClone()),
				validated:   vt,
			})
		default:
			return empty(), 0, fmt.Errorf("trigger %q: unknown validated rule type %T", desc, rule)
		}
	}

	engine := NewTriggerEngine(eventEntries, timeEntries, executor)
	engine.RegisterWithGate(gate)

	total := len(eventEntries) + len(timeEntries)
	slog.Info("triggers loaded", "event", len(eventEntries), "time", len(timeEntries))

	return engine, total, nil
}

// StartScheduler launches a background goroutine for each time trigger.
// Each goroutine fires on a time.Ticker interval. Context cancellation stops all goroutines.
// Safe to call even when there are no time triggers — returns immediately.
func (te *TriggerEngine) StartScheduler(ctx context.Context) {
	if len(te.timeTriggers) == 0 {
		return
	}
	for _, entry := range te.timeTriggers {
		interval, ok := timeTriggerInterval(entry)
		if !ok {
			slog.Warn("skipping time trigger with missing interval metadata",
				"trigger", entry.Description)
			continue
		}
		d, err := duration.ToDuration(interval.Value, interval.Unit)
		if err != nil {
			slog.Error("invalid time trigger interval, skipping",
				"trigger", entry.Description, "error", err)
			continue
		}
		slog.Info("starting time trigger scheduler",
			"trigger", entry.Description, "interval", d)
		go te.runTimeTrigger(ctx, entry, d)
	}
}

// runTimeTrigger runs a single time trigger on a ticker loop until ctx is cancelled.
// All errors are logged and swallowed — the ticker keeps running (fail-open).
func (te *TriggerEngine) runTimeTrigger(ctx context.Context, entry TimeTriggerEntry, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			te.executeTimeTrigger(ctx, entry)
		}
	}
}

// executeTimeTrigger runs a single tick of a time trigger: snapshot tasks, execute, persist.
func (te *TriggerEngine) executeTimeTrigger(ctx context.Context, entry TimeTriggerEntry) {
	allTasks := te.gate.ReadStore().GetAllTasks()
	input := ruki.ExecutionInput{}
	if timeTriggerRequiresCreateTemplate(timeTriggerForExec(entry)) {
		tmpl, err := te.gate.ReadStore().NewTaskTemplate()
		if err != nil {
			slog.Error("create template failed", "trigger", entry.Description, "error", err)
			return
		}
		if tmpl == nil {
			slog.Error("create template failed", "trigger", entry.Description, "error", "store returned nil template")
			return
		}
		input.CreateTemplate = tmpl
	}
	result, err := te.executor.ExecTimeTriggerAction(timeTriggerForExec(entry), allTasks, input)
	if err != nil {
		slog.Error("time trigger action failed",
			"trigger", entry.Description, "error", err)
		return
	}
	if err := te.persistResult(ctx, result); err != nil {
		slog.Error("time trigger persist failed",
			"trigger", entry.Description, "error", err)
	}
}

func triggerTimingEvent(entry triggerEntry) (string, string, bool) {
	switch {
	case entry.validated != nil:
		timing, event := entry.validated.Timing(), entry.validated.Event()
		if timing == "" || event == "" {
			return "", "", false
		}
		return timing, event, true
	case entry.trigger != nil:
		if entry.trigger.Timing == "" || entry.trigger.Event == "" {
			return "", "", false
		}
		return entry.trigger.Timing, entry.trigger.Event, true
	default:
		return "", "", false
	}
}

func timeTriggerInterval(entry TimeTriggerEntry) (ruki.DurationLiteral, bool) {
	switch {
	case entry.Validated != nil:
		interval := entry.Validated.IntervalLiteral()
		if interval.Unit == "" {
			return ruki.DurationLiteral{}, false
		}
		return interval, true
	case entry.Trigger != nil:
		if entry.Trigger.Interval.Unit == "" {
			return ruki.DurationLiteral{}, false
		}
		return entry.Trigger.Interval, true
	default:
		return ruki.DurationLiteral{}, false
	}
}

func triggerDenyMessage(trig any) (string, bool) {
	switch t := trig.(type) {
	case *ruki.ValidatedTrigger:
		return t.DenyMessage()
	case *ruki.Trigger:
		if t.Deny == nil {
			return "", false
		}
		return *t.Deny, true
	default:
		return "", false
	}
}

func triggerHasRunAction(trig any) bool {
	switch t := trig.(type) {
	case *ruki.ValidatedTrigger:
		return t.HasRunAction()
	case *ruki.Trigger:
		return t.Run != nil
	default:
		return false
	}
}

func triggerRequiresCreateTemplate(trig any) bool {
	switch t := trig.(type) {
	case *ruki.ValidatedTrigger:
		return t.RequiresCreateTemplate()
	case *ruki.Trigger:
		return t != nil && t.Action != nil && t.Action.Create != nil
	default:
		return false
	}
}

func timeTriggerRequiresCreateTemplate(trig any) bool {
	switch t := trig.(type) {
	case *ruki.ValidatedTimeTrigger:
		return t.RequiresCreateTemplate()
	case *ruki.TimeTrigger:
		return t != nil && t.Action != nil && t.Action.Create != nil
	default:
		return false
	}
}

func eventTriggerForExec(entry triggerEntry) any {
	if entry.validated != nil {
		return entry.validated
	}
	return entry.trigger
}

func timeTriggerForExec(entry TimeTriggerEntry) any {
	if entry.Validated != nil {
		return entry.Validated
	}
	return entry.Trigger
}

func cloneTriggerForService(trig *ruki.Trigger) *ruki.Trigger {
	if trig == nil {
		return nil
	}
	return &ruki.Trigger{
		Timing: trig.Timing,
		Event:  trig.Event,
		Where:  trig.Where,
		Action: trig.Action,
		Run:    trig.Run,
		Deny:   trig.Deny,
	}
}

func cloneTimeTriggerForService(tt *ruki.TimeTrigger) *ruki.TimeTrigger {
	if tt == nil {
		return nil
	}
	return &ruki.TimeTrigger{
		Interval: tt.Interval,
		Action:   tt.Action,
	}
}
