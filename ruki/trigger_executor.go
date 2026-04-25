package ruki

import (
	"fmt"
	"strings"

	"github.com/boolean-maybe/tiki/task"
)

// TriggerExecutor evaluates trigger guards and actions in a trigger context.
// It wraps Executor with old/new mutation context for QualifiedRef resolution.
// A fresh Executor is created per call — no shared mutable state.
type TriggerExecutor struct {
	schema   Schema
	userFunc func() string
}

// NewTriggerExecutor creates a TriggerExecutor.
// If userFunc is nil, user() calls in trigger actions will return an error.
func NewTriggerExecutor(schema Schema, userFunc func() string) *TriggerExecutor {
	return &TriggerExecutor{schema: schema, userFunc: userFunc}
}

// TriggerContext holds the old/new task snapshots and allTasks for trigger evaluation.
type TriggerContext struct {
	Old      *task.Task // nil for create
	New      *task.Task // nil for delete
	AllTasks []*task.Task
}

// EvalGuard evaluates a trigger's where condition against the triggering event.
// Returns true if the trigger should fire (guard passes or no guard).
func (te *TriggerExecutor) EvalGuard(trig any, tc *TriggerContext) (bool, error) {
	validated, err := validateEventTriggerInput(trig)
	if err != nil {
		return false, err
	}
	where := validated.trigger.Where
	if where == nil {
		return true, nil
	}
	// the guard evaluates qualified refs against old/new directly;
	// there is no "current task" — we use a sentinel that QualifiedRef overrides
	sentinel := te.guardSentinel(tc)
	exec := te.newExecWithOverrides(tc)
	return exec.evalCondition(where, sentinel, tc.AllTasks)
}

// ExecTimeTriggerAction executes a time trigger's action against all tasks.
// Uses a plain Executor (no old/new overrides) since time triggers have no
// mutation context — the parser forbids qualified refs in them.
func (te *TriggerExecutor) ExecTimeTriggerAction(tt any, allTasks []*task.Task, inputs ...ExecutionInput) (*Result, error) {
	var input ExecutionInput
	if len(inputs) > 0 {
		input = inputs[0]
	}

	exec := NewExecutor(te.schema, te.userFunc, ExecutorRuntime{Mode: ExecutorRuntimeTimeTrigger})
	switch t := tt.(type) {
	case *ValidatedTimeTrigger:
		if err := t.mustBeSealed(); err != nil {
			return nil, err
		}
		if t.runtime != ExecutorRuntimeTimeTrigger {
			return nil, &RuntimeMismatchError{
				ValidatedFor: t.runtime,
				Runtime:      ExecutorRuntimeTimeTrigger,
			}
		}
		if t.timeTrigger.Action == nil {
			return nil, fmt.Errorf("time trigger has no action")
		}
		action := &ValidatedStatement{
			seal:       validatedSeal,
			runtime:    ExecutorRuntimeTimeTrigger,
			usesIDFunc: t.usesIDFunc,
			statement:  cloneStatement(t.timeTrigger.Action),
		}
		return exec.Execute(action, allTasks, input)
	case *TimeTrigger:
		if t.Action == nil {
			return nil, fmt.Errorf("time trigger has no action")
		}
		return exec.Execute(t.Action, allTasks, input)
	default:
		return nil, fmt.Errorf("unsupported time trigger type %T", tt)
	}
}

// ExecAction executes a trigger's CRUD action statement and returns the result.
// QualifiedRefs resolve against tc.Old/tc.New. Bare fields resolve against target tasks.
// Returns *Result for persistence by service/.
func (te *TriggerExecutor) ExecAction(trig any, tc *TriggerContext, inputs ...ExecutionInput) (*Result, error) {
	var input ExecutionInput
	if len(inputs) > 0 {
		input = inputs[0]
	}

	exec := te.newExecWithOverrides(tc)
	switch t := trig.(type) {
	case *ValidatedTrigger:
		if err := t.mustBeSealed(); err != nil {
			return nil, err
		}
		if t.runtime != ExecutorRuntimeEventTrigger {
			return nil, &RuntimeMismatchError{
				ValidatedFor: t.runtime,
				Runtime:      ExecutorRuntimeEventTrigger,
			}
		}
		if t.trigger.Action == nil {
			return nil, fmt.Errorf("trigger has no action")
		}
		action := &ValidatedStatement{
			seal:       validatedSeal,
			runtime:    ExecutorRuntimeEventTrigger,
			usesIDFunc: t.usesIDFunc,
			statement:  cloneStatement(t.trigger.Action),
		}
		return exec.Execute(action, tc.AllTasks, input)
	case *Trigger:
		if t.Action == nil {
			return nil, fmt.Errorf("trigger has no action")
		}
		return exec.Execute(t.Action, tc.AllTasks, input)
	default:
		return nil, fmt.Errorf("unsupported trigger type %T", trig)
	}
}

// ExecRun evaluates the run() command expression to a string against the trigger context.
// Returns the command string for execution by service/.
func (te *TriggerExecutor) ExecRun(trig any, tc *TriggerContext) (string, error) {
	validated, err := validateEventTriggerInput(trig)
	if err != nil {
		return "", err
	}
	if validated.trigger.Run == nil {
		return "", fmt.Errorf("trigger has no run action")
	}
	command := validated.trigger.Run.Command
	if command == nil {
		return "", fmt.Errorf("trigger has no run action")
	}
	sentinel := te.guardSentinel(tc)
	exec := te.newExecWithOverrides(tc)
	val, err := exec.evalExpr(command, sentinel, tc.AllTasks)
	if err != nil {
		return "", fmt.Errorf("evaluating run command: %w", err)
	}
	s, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("run command did not evaluate to string, got %T", val)
	}
	return s, nil
}

// guardSentinel returns the best "current task" for guard evaluation.
// In guards, all references should be qualified (old./new.), but the executor
// still needs a task to evaluate against. We prefer new (proposed) over old.
func (te *TriggerExecutor) guardSentinel(tc *TriggerContext) *task.Task {
	if tc.New != nil {
		return tc.New
	}
	if tc.Old != nil {
		return tc.Old
	}
	return &task.Task{}
}

func validateEventTriggerInput(trig any) (*ValidatedTrigger, error) {
	switch t := trig.(type) {
	case *ValidatedTrigger:
		if err := t.mustBeSealed(); err != nil {
			return nil, err
		}
		if t.runtime != ExecutorRuntimeEventTrigger {
			return nil, &RuntimeMismatchError{
				ValidatedFor: t.runtime,
				Runtime:      ExecutorRuntimeEventTrigger,
			}
		}
		return t, nil
	case *Trigger:
		return NewSemanticValidator(ExecutorRuntimeEventTrigger).ValidateTrigger(t)
	default:
		return nil, fmt.Errorf("unsupported trigger type %T", trig)
	}
}

// triggerExecOverride wraps Executor and intercepts QualifiedRef evaluation.
type triggerExecOverride struct {
	*Executor
	tc *TriggerContext
}

// newExecWithOverrides creates a fresh Executor with QualifiedRef interception.
func (te *TriggerExecutor) newExecWithOverrides(tc *TriggerContext) *triggerExecOverride {
	return &triggerExecOverride{
		Executor: NewExecutor(te.schema, te.userFunc, ExecutorRuntime{Mode: ExecutorRuntimeEventTrigger}),
		tc:       tc,
	}
}

// evalExpr overrides the base Executor to handle QualifiedRef.
func (e *triggerExecOverride) evalExpr(expr Expr, t *task.Task, allTasks []*task.Task) (interface{}, error) {
	if qr, ok := expr.(*QualifiedRef); ok {
		return e.resolveQualifiedRef(qr)
	}
	// for non-QualifiedRef expressions, delegate to base but with our overridden evalExpr
	// for nested expressions
	return e.evalExprRecursive(expr, t, allTasks)
}

func (e *triggerExecOverride) resolveQualifiedRef(qr *QualifiedRef) (interface{}, error) {
	switch qr.Qualifier {
	case "old":
		if e.tc.Old == nil {
			return nil, nil // old is nil for create events
		}
		return e.extractField(e.tc.Old, qr.Name), nil
	case "new":
		if e.tc.New == nil {
			return nil, nil // new is nil for delete events
		}
		return e.extractField(e.tc.New, qr.Name), nil
	default:
		return nil, fmt.Errorf("unknown qualifier %q", qr.Qualifier)
	}
}

// evalExprRecursive handles all expression types, dispatching QualifiedRef
// to resolveQualifiedRef and delegating everything else to the base Executor.
func (e *triggerExecOverride) evalExprRecursive(expr Expr, t *task.Task, allTasks []*task.Task) (interface{}, error) {
	switch expr := expr.(type) {
	case *QualifiedRef:
		return e.resolveQualifiedRef(expr)
	case *FieldRef:
		return e.extractField(t, expr.Name), nil
	case *BinaryExpr:
		leftVal, err := e.evalExpr(expr.Left, t, allTasks)
		if err != nil {
			return nil, err
		}
		rightVal, err := e.evalExpr(expr.Right, t, allTasks)
		if err != nil {
			return nil, err
		}
		switch expr.Op {
		case "+":
			return addValues(leftVal, rightVal)
		case "-":
			return subtractValues(leftVal, rightVal)
		default:
			return nil, fmt.Errorf("unknown binary operator %q", expr.Op)
		}
	case *ListLiteral:
		result := make([]interface{}, len(expr.Elements))
		for i, elem := range expr.Elements {
			val, err := e.evalExpr(elem, t, allTasks)
			if err != nil {
				return nil, err
			}
			result[i] = val
		}
		return result, nil
	case *FunctionCall:
		return e.evalFunctionCallOverride(expr, t, allTasks)
	default:
		// StringLiteral, IntLiteral, DateLiteral, DurationLiteral, EmptyLiteral, SubQuery
		return e.Executor.evalExpr(expr, t, allTasks)
	}
}

// evalCondition overrides the base to use our evalExpr for expression evaluation.
func (e *triggerExecOverride) evalCondition(c Condition, t *task.Task, allTasks []*task.Task) (bool, error) {
	switch c := c.(type) {
	case *BinaryCondition:
		left, err := e.evalCondition(c.Left, t, allTasks)
		if err != nil {
			return false, err
		}
		switch c.Op {
		case "and":
			if !left {
				return false, nil
			}
			return e.evalCondition(c.Right, t, allTasks)
		case "or":
			if left {
				return true, nil
			}
			return e.evalCondition(c.Right, t, allTasks)
		default:
			return false, fmt.Errorf("unknown binary operator %q", c.Op)
		}
	case *NotCondition:
		val, err := e.evalCondition(c.Inner, t, allTasks)
		if err != nil {
			return false, err
		}
		return !val, nil
	case *BoolExprCondition:
		val, err := e.evalExpr(c.Expr, t, allTasks)
		if err != nil {
			return false, err
		}
		return conditionBoolValue(val)
	case *CompareExpr:
		leftVal, err := e.evalExpr(c.Left, t, allTasks)
		if err != nil {
			return false, err
		}
		rightVal, err := e.evalExpr(c.Right, t, allTasks)
		if err != nil {
			return false, err
		}
		return e.compareValues(leftVal, rightVal, c.Op, c.Left, c.Right)
	case *IsEmptyExpr:
		val, err := e.evalExpr(c.Expr, t, allTasks)
		if err != nil {
			return false, err
		}
		empty := isZeroValue(val)
		if c.Negated {
			return !empty, nil
		}
		return empty, nil
	case *InExpr:
		return e.evalInOverride(c, t, allTasks)
	case *QuantifierExpr:
		return e.evalQuantifierOverride(c, t, allTasks)
	default:
		return false, fmt.Errorf("unknown condition type %T", c)
	}
}

func (e *triggerExecOverride) evalInOverride(c *InExpr, t *task.Task, allTasks []*task.Task) (bool, error) {
	val, err := e.evalExpr(c.Value, t, allTasks)
	if err != nil {
		return false, err
	}
	collVal, err := e.evalExpr(c.Collection, t, allTasks)
	if err != nil {
		return false, err
	}

	if list, ok := collVal.([]interface{}); ok {
		// unset field (nil) is not a member of any list
		if val == nil {
			return c.Negated, nil
		}
		valStr := normalizeToString(val)
		foldCase := isEnumLikeField(e.exprFieldType(c.Value))
		found := false
		for _, elem := range list {
			elemStr := normalizeToString(elem)
			if foldCase && strings.EqualFold(valStr, elemStr) || !foldCase && valStr == elemStr {
				found = true
				break
			}
		}
		if c.Negated {
			return !found, nil
		}
		return found, nil
	}

	if haystack, ok := collVal.(string); ok {
		needle, ok := val.(string)
		if !ok {
			return false, fmt.Errorf("in: substring check requires string value")
		}
		found := strings.Contains(haystack, needle)
		if c.Negated {
			return !found, nil
		}
		return found, nil
	}

	return false, fmt.Errorf("in: collection is not a list or string")
}

func (e *triggerExecOverride) evalQuantifierOverride(q *QuantifierExpr, t *task.Task, allTasks []*task.Task) (bool, error) {
	listVal, err := e.evalExpr(q.Expr, t, allTasks)
	if err != nil {
		return false, err
	}
	refs, ok := listVal.([]interface{})
	if !ok {
		return false, fmt.Errorf("quantifier: expression is not a list")
	}

	refTasks := resolveRefTasks(refs, allTasks)

	switch q.Kind {
	case "any":
		for _, rt := range refTasks {
			match, err := e.evalCondition(q.Condition, rt, allTasks)
			if err != nil {
				return false, err
			}
			if match {
				return true, nil
			}
		}
		return false, nil
	case "all":
		if len(refTasks) == 0 {
			return true, nil
		}
		for _, rt := range refTasks {
			match, err := e.evalCondition(q.Condition, rt, allTasks)
			if err != nil {
				return false, err
			}
			if !match {
				return false, nil
			}
		}
		return true, nil
	default:
		return false, fmt.Errorf("unknown quantifier %q", q.Kind)
	}
}

func (e *triggerExecOverride) evalFunctionCallOverride(fc *FunctionCall, t *task.Task, allTasks []*task.Task) (interface{}, error) {
	switch fc.Name {
	case "count":
		return e.evalCountOverride(fc, allTasks)
	case "blocks":
		return e.evalBlocksOverride(fc, t, allTasks)
	case "next_date":
		return e.evalNextDateOverride(fc, t, allTasks)
	default:
		// now, user, call — delegate to base (no expression args needing QualifiedRef resolution)
		return e.evalFunctionCall(fc, t, allTasks)
	}
}

func (e *triggerExecOverride) evalCountOverride(fc *FunctionCall, allTasks []*task.Task) (interface{}, error) {
	sq, ok := fc.Args[0].(*SubQuery)
	if !ok {
		return nil, fmt.Errorf("count() argument must be a select subquery")
	}
	if sq.Where == nil {
		return len(allTasks), nil
	}
	count := 0
	for _, t := range allTasks {
		match, err := e.evalCondition(sq.Where, t, allTasks)
		if err != nil {
			return nil, err
		}
		if match {
			count++
		}
	}
	return count, nil
}

func (e *triggerExecOverride) evalBlocksOverride(fc *FunctionCall, t *task.Task, allTasks []*task.Task) (interface{}, error) {
	val, err := e.evalExpr(fc.Args[0], t, allTasks)
	if err != nil {
		return nil, err
	}
	return blocksLookup(val, allTasks), nil
}

func (e *triggerExecOverride) evalNextDateOverride(fc *FunctionCall, t *task.Task, allTasks []*task.Task) (interface{}, error) {
	val, err := e.evalExpr(fc.Args[0], t, allTasks)
	if err != nil {
		return nil, err
	}
	rec, ok := val.(task.Recurrence)
	if !ok {
		return nil, fmt.Errorf("next_date() argument must be a recurrence value")
	}
	return task.NextOccurrence(rec), nil
}

// Execute overrides the base Executor to use our evalExpr/evalCondition.
func (e *triggerExecOverride) Execute(stmt any, tasks []*task.Task, inputs ...ExecutionInput) (*Result, error) {
	var input ExecutionInput
	if len(inputs) > 0 {
		input = inputs[0]
	}
	if stmt == nil {
		return nil, fmt.Errorf("nil statement")
	}
	var validated *ValidatedStatement
	var rawStmt *Statement
	rawInput := false
	requiresCreateTemplate := false
	switch s := stmt.(type) {
	case *ValidatedStatement:
		validated = s
		requiresCreateTemplate = true
	case *Statement:
		rawInput = true
		var err error
		validated, err = NewSemanticValidator(e.runtime.Mode).ValidateStatement(s)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported statement type %T", stmt)
	}

	if validated != nil {
		if err := validated.mustBeSealed(); err != nil {
			return nil, err
		}
		if validated.runtime != e.runtime.Mode {
			return nil, &RuntimeMismatchError{
				ValidatedFor: validated.runtime,
				Runtime:      e.runtime.Mode,
			}
		}
		if validated.usesIDFunc && e.runtime.Mode == ExecutorRuntimePlugin && strings.TrimSpace(input.SelectedTaskID) == "" {
			return nil, &MissingSelectedTaskIDError{}
		}
		rawStmt = validated.statement
		if rawInput {
			requiresCreateTemplate = false
		}
	}
	e.currentInput = input
	defer func() { e.currentInput = ExecutionInput{} }()

	switch {
	case rawStmt.Create != nil:
		return e.executeCreate(rawStmt.Create, tasks, requiresCreateTemplate)
	case rawStmt.Update != nil:
		return e.executeUpdate(rawStmt.Update, tasks)
	case rawStmt.Delete != nil:
		return e.executeDelete(rawStmt.Delete, tasks)
	default:
		return nil, fmt.Errorf("unsupported trigger action type")
	}
}

func (e *triggerExecOverride) executeUpdate(upd *UpdateStmt, tasks []*task.Task) (*Result, error) {
	matched, err := e.filterTasks(upd.Where, tasks)
	if err != nil {
		return nil, err
	}

	clones := make([]*task.Task, len(matched))
	for i, t := range matched {
		clones[i] = t.Clone()
	}

	for _, clone := range clones {
		for _, a := range upd.Set {
			val, err := e.evalExpr(a.Value, clone, tasks)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", a.Field, err)
			}
			if err := e.setField(clone, a.Field, val); err != nil {
				return nil, fmt.Errorf("field %q: %w", a.Field, err)
			}
		}
	}

	return &Result{Update: &UpdateResult{Updated: clones}}, nil
}

func (e *triggerExecOverride) executeCreate(cr *CreateStmt, tasks []*task.Task, requireTemplate bool) (*Result, error) {
	if requireTemplate && e.currentInput.CreateTemplate == nil {
		return nil, &MissingCreateTemplateError{}
	}
	var t *task.Task
	if e.currentInput.CreateTemplate != nil {
		t = e.currentInput.CreateTemplate.Clone()
	} else {
		t = &task.Task{}
	}
	for _, a := range cr.Assignments {
		val, err := e.evalExpr(a.Value, t, tasks)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", a.Field, err)
		}
		if err := e.setField(t, a.Field, val); err != nil {
			return nil, fmt.Errorf("field %q: %w", a.Field, err)
		}
	}
	return &Result{Create: &CreateResult{Task: t}}, nil
}

func (e *triggerExecOverride) executeDelete(del *DeleteStmt, tasks []*task.Task) (*Result, error) {
	matched, err := e.filterTasks(del.Where, tasks)
	if err != nil {
		return nil, err
	}
	return &Result{Delete: &DeleteResult{Deleted: matched}}, nil
}

func (e *triggerExecOverride) filterTasks(where Condition, tasks []*task.Task) ([]*task.Task, error) {
	if where == nil {
		result := make([]*task.Task, len(tasks))
		copy(result, tasks)
		return result, nil
	}
	var result []*task.Task
	for _, t := range tasks {
		match, err := e.evalCondition(where, t, tasks)
		if err != nil {
			return nil, err
		}
		if match {
			result = append(result, t)
		}
	}
	return result, nil
}

// resolveRefTasks finds tasks by ID from a list of ref values.
func resolveRefTasks(refs []interface{}, allTasks []*task.Task) []*task.Task {
	result := make([]*task.Task, 0, len(refs))
	for _, ref := range refs {
		refID := normalizeToString(ref)
		for _, at := range allTasks {
			if equalFoldID(at.ID, refID) {
				result = append(result, at)
				break
			}
		}
	}
	return result
}

// equalFoldID compares two task IDs case-insensitively.
func equalFoldID(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'a' && ca <= 'z' {
			ca -= 32
		}
		if cb >= 'a' && cb <= 'z' {
			cb -= 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// blocksLookup finds all task IDs that have the given ID in their dependsOn.
func blocksLookup(val interface{}, allTasks []*task.Task) []interface{} {
	targetID := normalizeToString(val)
	var blockers []interface{}
	for _, at := range allTasks {
		for _, dep := range at.DependsOn {
			if equalFoldID(dep, targetID) {
				blockers = append(blockers, at.ID)
				break
			}
		}
	}
	if blockers == nil {
		blockers = []interface{}{}
	}
	return blockers
}
