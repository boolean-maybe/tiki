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
func NewTriggerExecutor(schema Schema, userFunc func() string) *TriggerExecutor {
	if userFunc == nil {
		userFunc = func() string { return "" }
	}
	return &TriggerExecutor{schema: schema, userFunc: userFunc}
}

// TriggerContext holds the old/new task snapshots and allTasks for trigger evaluation.
type TriggerContext struct {
	Old      *task.Task // nil for create
	New      *task.Task // nil for delete
	AllTasks []*task.Task
}

// EvalGuard evaluates a trigger's Where condition against the triggering event.
// Returns true if the trigger should fire (guard passes or no guard).
func (te *TriggerExecutor) EvalGuard(trig *Trigger, tc *TriggerContext) (bool, error) {
	if trig.Where == nil {
		return true, nil
	}
	// the guard evaluates qualified refs against old/new directly;
	// there is no "current task" — we use a sentinel that QualifiedRef overrides
	sentinel := te.guardSentinel(tc)
	exec := te.newExecWithOverrides(tc)
	return exec.evalCondition(trig.Where, sentinel, tc.AllTasks)
}

// ExecTimeTriggerAction executes a time trigger's action against all tasks.
// Uses a plain Executor (no old/new overrides) since time triggers have no
// mutation context — the parser forbids qualified refs in them.
func (te *TriggerExecutor) ExecTimeTriggerAction(tt *TimeTrigger, allTasks []*task.Task) (*Result, error) {
	if tt.Action == nil {
		return nil, fmt.Errorf("time trigger has no action")
	}
	exec := NewExecutor(te.schema, te.userFunc)
	return exec.Execute(tt.Action, allTasks)
}

// ExecAction executes a trigger's CRUD action statement and returns the result.
// QualifiedRefs resolve against tc.Old/tc.New. Bare fields resolve against target tasks.
// Returns *Result for persistence by service/.
func (te *TriggerExecutor) ExecAction(trig *Trigger, tc *TriggerContext) (*Result, error) {
	if trig.Action == nil {
		return nil, fmt.Errorf("trigger has no action")
	}
	exec := te.newExecWithOverrides(tc)
	return exec.Execute(trig.Action, tc.AllTasks)
}

// ExecRun evaluates the run() command expression to a string against the trigger context.
// Returns the command string for execution by service/.
func (te *TriggerExecutor) ExecRun(trig *Trigger, tc *TriggerContext) (string, error) {
	if trig.Run == nil {
		return "", fmt.Errorf("trigger has no run action")
	}
	sentinel := te.guardSentinel(tc)
	exec := te.newExecWithOverrides(tc)
	val, err := exec.evalExpr(trig.Run.Command, sentinel, tc.AllTasks)
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

// triggerExecOverride wraps Executor and intercepts QualifiedRef evaluation.
type triggerExecOverride struct {
	*Executor
	tc *TriggerContext
}

// newExecWithOverrides creates a fresh Executor with QualifiedRef interception.
func (te *TriggerExecutor) newExecWithOverrides(tc *TriggerContext) *triggerExecOverride {
	return &triggerExecOverride{
		Executor: NewExecutor(te.schema, te.userFunc),
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
		return extractField(e.tc.Old, qr.Name), nil
	case "new":
		if e.tc.New == nil {
			return nil, nil // new is nil for delete events
		}
		return extractField(e.tc.New, qr.Name), nil
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
		return extractField(t, expr.Name), nil
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
		valStr := normalizeToString(val)
		found := false
		for _, elem := range list {
			if normalizeToString(elem) == valStr {
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
func (e *triggerExecOverride) Execute(stmt *Statement, tasks []*task.Task) (*Result, error) {
	if stmt == nil {
		return nil, fmt.Errorf("nil statement")
	}
	switch {
	case stmt.Create != nil:
		return e.executeCreate(stmt.Create, tasks)
	case stmt.Update != nil:
		return e.executeUpdate(stmt.Update, tasks)
	case stmt.Delete != nil:
		return e.executeDelete(stmt.Delete, tasks)
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

func (e *triggerExecOverride) executeCreate(cr *CreateStmt, tasks []*task.Task) (*Result, error) {
	t := &task.Task{}
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
