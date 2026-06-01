package ruki

import (
	"fmt"
	"strings"

	"github.com/boolean-maybe/tiki/ruki/recurrence"
)

// TriggerExecutor evaluates trigger guards and actions in a trigger context.
// It wraps Executor with old/new mutation context for QualifiedRef resolution.
type TriggerExecutor struct {
	schema   Schema
	factory  DocumentFactory
	userFunc func() string
}

// NewTriggerExecutor creates a TriggerExecutor. The factory builds the blank
// Document a template-less create action fills in, and the guard sentinel for
// create events that lack both old and new snapshots.
// If userFunc is nil, user() calls in trigger actions will return an error.
func NewTriggerExecutor(schema Schema, factory DocumentFactory, userFunc func() string) *TriggerExecutor {
	return &TriggerExecutor{schema: schema, factory: factory, userFunc: userFunc}
}

// TriggerContext holds the old/new tiki snapshots and allTikis for trigger evaluation.
type TriggerContext struct {
	Old      Document // nil for create
	New      Document // nil for delete
	AllTikis []Document
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
	sentinel := te.guardSentinel(tc)
	exec := te.newExecWithOverrides(tc)
	return exec.evalCondition(where, evalContext{current: sentinel, allTikis: tc.AllTikis})
}

// ExecTimeTriggerAction executes a time trigger's action against all tikis.
func (te *TriggerExecutor) ExecTimeTriggerAction(tt any, allTikis []Document, inputs ...ExecutionInput) (*Result, error) {
	var input ExecutionInput
	if len(inputs) > 0 {
		input = inputs[0]
	}

	exec := NewExecutor(te.schema, te.factory, te.userFunc, ExecutorRuntime{Mode: ExecutorRuntimeTimeTrigger})
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
			seal:             validatedSeal,
			runtime:          ExecutorRuntimeTimeTrigger,
			usesIDFunc:       t.usesIDFunc,
			usesFilepathFunc: t.usesFilepathFunc,
			statement:        cloneStatement(t.timeTrigger.Action),
		}
		return exec.Execute(action, allTikis, input)
	case *TimeTrigger:
		if t.Action == nil {
			return nil, fmt.Errorf("time trigger has no action")
		}
		return exec.Execute(t.Action, allTikis, input)
	default:
		return nil, fmt.Errorf("unsupported time trigger type %T", tt)
	}
}

// ExecAction executes a trigger's CRUD action statement and returns the result.
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
			seal:             validatedSeal,
			runtime:          ExecutorRuntimeEventTrigger,
			usesIDFunc:       t.usesIDFunc,
			usesFilepathFunc: t.usesFilepathFunc,
			statement:        cloneStatement(t.trigger.Action),
		}
		return exec.Execute(action, tc.AllTikis, input)
	case *Trigger:
		if t.Action == nil {
			return nil, fmt.Errorf("trigger has no action")
		}
		return exec.Execute(t.Action, tc.AllTikis, input)
	default:
		return nil, fmt.Errorf("unsupported trigger type %T", trig)
	}
}

// ExecRun evaluates the run() command expression to a string against the trigger context.
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
	val, err := exec.evalExpr(command, evalContext{current: sentinel, allTikis: tc.AllTikis})
	if err != nil {
		return "", fmt.Errorf("evaluating run command: %w", err)
	}
	s, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("run command did not evaluate to string, got %T", val)
	}
	return s, nil
}

// guardSentinel returns the best "current tiki" for guard evaluation.
func (te *TriggerExecutor) guardSentinel(tc *TriggerContext) Document {
	if tc.New != nil {
		return tc.New
	}
	if tc.Old != nil {
		return tc.Old
	}
	if te.factory != nil {
		return te.factory()
	}
	return nil
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
		Executor: NewExecutor(te.schema, te.factory, te.userFunc, ExecutorRuntime{Mode: ExecutorRuntimeEventTrigger}),
		tc:       tc,
	}
}

// evalExpr overrides the base Executor to handle QualifiedRef.
func (e *triggerExecOverride) evalExpr(expr Expr, ctx evalContext) (interface{}, error) {
	if qr, ok := expr.(*QualifiedRef); ok {
		return e.resolveQualifiedRef(qr, ctx)
	}
	return e.evalExprRecursive(expr, ctx)
}

func (e *triggerExecOverride) resolveQualifiedRef(qr *QualifiedRef, ctx evalContext) (interface{}, error) {
	switch qr.Qualifier {
	case "old":
		if e.tc.Old == nil {
			// old is nil for create events — surface as absent.
			return nil, absentFieldError(nil, "old."+qr.Name)
		}
		return e.readFieldRefWithCarveOut(e.tc.Old, qr.Name, ctx)
	case "new":
		if e.tc.New == nil {
			return nil, absentFieldError(nil, "new."+qr.Name)
		}
		return e.readFieldRefWithCarveOut(e.tc.New, qr.Name, ctx)
	case "outer":
		if ctx.outer == nil {
			return nil, fmt.Errorf("outer.%s is not available outside a subquery", qr.Name)
		}
		return e.readFieldRefWithCarveOut(ctx.outer, qr.Name, ctx)
	case "target", "targets":
		return nil, fmt.Errorf("%s. qualifier is not valid in trigger execution", qr.Qualifier)
	default:
		return nil, fmt.Errorf("unknown qualifier %q", qr.Qualifier)
	}
}

// evalExprRecursive dispatches QualifiedRef through the override and
// delegates everything else through the base Executor (which still uses
// e.evalExpr for its nested calls thanks to method dispatch).
func (e *triggerExecOverride) evalExprRecursive(expr Expr, ctx evalContext) (interface{}, error) {
	switch expr := expr.(type) {
	case *QualifiedRef:
		return e.resolveQualifiedRef(expr, ctx)
	case *FieldRef:
		return e.readFieldRefWithCarveOut(ctx.current, expr.Name, ctx)
	case *BinaryExpr:
		leftVal, err := e.evalExpr(expr.Left, ctx)
		if err != nil {
			return nil, err
		}
		rightVal, err := e.evalExpr(expr.Right, ctx)
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
			val, err := e.evalExpr(elem, ctx)
			if err != nil {
				return nil, err
			}
			result[i] = val
		}
		return result, nil
	case *FunctionCall:
		return e.evalFunctionCallOverride(expr, ctx)
	default:
		return e.Executor.evalExpr(expr, ctx)
	}
}

// evalCondition overrides the base to use our evalExpr for expression evaluation.
func (e *triggerExecOverride) evalCondition(c Condition, ctx evalContext) (bool, error) {
	switch c := c.(type) {
	case *BinaryCondition:
		left, err := e.evalCondition(c.Left, ctx)
		if err != nil {
			return false, err
		}
		switch c.Op {
		case "and":
			if !left {
				return false, nil
			}
			return e.evalCondition(c.Right, ctx)
		case "or":
			if left {
				return true, nil
			}
			return e.evalCondition(c.Right, ctx)
		default:
			return false, fmt.Errorf("unknown binary operator %q", c.Op)
		}
	case *NotCondition:
		val, err := e.evalCondition(c.Inner, ctx)
		if err != nil {
			return false, err
		}
		return !val, nil
	case *BoolExprCondition:
		val, err := e.evalExpr(c.Expr, ctx)
		if err != nil {
			return false, err
		}
		return conditionBoolValue(val)
	case *CompareExpr:
		leftVal, leftAbsent, err := absorbAbsent(e.evalExpr, c.Left, ctx)
		if err != nil {
			return false, err
		}
		rightVal, rightAbsent, err := absorbAbsent(e.evalExpr, c.Right, ctx)
		if err != nil {
			return false, err
		}
		if leftAbsent || rightAbsent {
			return missingFieldCompareResult(c.Op, leftAbsent, rightAbsent, c.Left, c.Right, leftVal, rightVal)
		}
		return e.compareValues(leftVal, rightVal, c.Op, c.Left, c.Right)
	case *IsEmptyExpr:
		val, absent, err := absorbAbsent(e.evalExpr, c.Expr, ctx)
		if err != nil {
			return false, err
		}
		empty := absent || isZeroValue(val)
		if c.Negated {
			return !empty, nil
		}
		return empty, nil
	case *InExpr:
		return e.evalInOverride(c, ctx)
	case *QuantifierExpr:
		return e.evalQuantifierOverride(c, ctx)
	default:
		return false, fmt.Errorf("unknown condition type %T", c)
	}
}

func (e *triggerExecOverride) evalInOverride(c *InExpr, ctx evalContext) (bool, error) {
	val, valAbsent, err := absorbAbsent(e.evalExpr, c.Value, ctx)
	if err != nil {
		return false, err
	}
	collVal, collAbsent, err := absorbAbsent(e.evalExpr, c.Collection, ctx)
	if err != nil {
		return false, err
	}

	// Missing LHS or collection: `in` → false, `not in` → true (parity
	// with base executor's updated Phase-4 rule).
	if valAbsent || collAbsent {
		return c.Negated, nil
	}

	if list, ok := collVal.([]interface{}); ok {
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

	if collVal == nil {
		return c.Negated, nil
	}

	return false, fmt.Errorf("in: collection is not a list or string")
}

func (e *triggerExecOverride) evalQuantifierOverride(q *QuantifierExpr, ctx evalContext) (bool, error) {
	listVal, absent, err := absorbAbsent(e.evalExpr, q.Expr, ctx)
	if err != nil {
		return false, err
	}
	if absent {
		// Missing list acts as empty: all→true, any→false.
		return q.Kind == "all", nil
	}
	refs, ok := listVal.([]interface{})
	if !ok {
		return false, fmt.Errorf("quantifier: expression is not a list")
	}

	refTikis := resolveRefTikis(refs, ctx.allTikis)

	switch q.Kind {
	case "any":
		for _, rt := range refTikis {
			match, err := e.evalCondition(q.Condition, ctx.withCurrent(rt))
			if err != nil {
				continue
			}
			if match {
				return true, nil
			}
		}
		return false, nil
	case "all":
		if len(refTikis) == 0 {
			return true, nil
		}
		for _, rt := range refTikis {
			match, err := e.evalCondition(q.Condition, ctx.withCurrent(rt))
			if err != nil {
				return false, nil
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

func (e *triggerExecOverride) evalFunctionCallOverride(fc *FunctionCall, ctx evalContext) (interface{}, error) {
	switch fc.Name {
	case "count":
		return e.evalCountOverride(fc, ctx.current, ctx.allTikis)
	case "exists":
		return e.evalExistsOverride(fc, ctx.current, ctx.allTikis)
	case "blocks":
		return e.evalBlocksOverride(fc, ctx)
	case "next_date":
		return e.evalNextDateOverride(fc, ctx)
	case "next_enum":
		return e.evalEnumStepOverride(fc, ctx, +1)
	case "prev_enum":
		return e.evalEnumStepOverride(fc, ctx, -1)
	case "has":
		return e.evalHasOverride(fc, ctx)
	default:
		return e.Executor.evalFunctionCall(fc, ctx)
	}
}

// evalHasOverride implements has(<field>) for trigger contexts.
func (e *triggerExecOverride) evalHasOverride(fc *FunctionCall, ctx evalContext) (interface{}, error) {
	if len(fc.Args) != 1 {
		return nil, fmt.Errorf("has() expects 1 argument, got %d", len(fc.Args))
	}
	var name, qualifier string
	switch ref := fc.Args[0].(type) {
	case *FieldRef:
		name = ref.Name
	case *QualifiedRef:
		name = ref.Name
		qualifier = ref.Qualifier
	default:
		return nil, fmt.Errorf("has() argument must be a field reference, e.g. has(status) or has(new.status)")
	}
	var target Document
	switch qualifier {
	case "":
		target = ctx.current
	case "new":
		target = e.tc.New
	case "old":
		target = e.tc.Old
	case "outer":
		if ctx.outer == nil {
			return nil, fmt.Errorf("has(outer.%s) is not available outside a subquery", name)
		}
		target = ctx.outer
	case "target", "targets":
		return nil, fmt.Errorf("has(%s.%s): %s. qualifier is not valid in trigger contexts", qualifier, name, qualifier)
	default:
		return nil, fmt.Errorf("has(%s.%s): unknown qualifier %q", qualifier, name, qualifier)
	}
	if target == nil {
		return false, nil
	}
	return tikiHas(target, name), nil
}

func (e *triggerExecOverride) evalCountOverride(fc *FunctionCall, parent Document, allTikis []Document) (interface{}, error) {
	sq, ok := fc.Args[0].(*SubQuery)
	if !ok {
		return nil, fmt.Errorf("count() argument must be a select subquery")
	}
	if sq.Where == nil {
		return len(allTikis), nil
	}
	count := 0
	for _, t := range allTikis {
		match, err := e.evalCondition(sq.Where, evalContext{current: t, outer: parent, allTikis: allTikis})
		if err != nil {
			continue
		}
		if match {
			count++
		}
	}
	return count, nil
}

func (e *triggerExecOverride) evalExistsOverride(fc *FunctionCall, parent Document, allTikis []Document) (interface{}, error) {
	sq, ok := fc.Args[0].(*SubQuery)
	if !ok {
		return nil, fmt.Errorf("exists() argument must be a select subquery")
	}
	if sq.Where == nil {
		return len(allTikis) > 0, nil
	}
	for _, t := range allTikis {
		match, err := e.evalCondition(sq.Where, evalContext{current: t, outer: parent, allTikis: allTikis})
		if err != nil {
			continue
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

func (e *triggerExecOverride) evalBlocksOverride(fc *FunctionCall, ctx evalContext) (interface{}, error) {
	val, err := e.evalExpr(fc.Args[0], ctx)
	if err != nil {
		return nil, err
	}
	return blocksLookup(val, ctx.allTikis), nil
}

func (e *triggerExecOverride) evalNextDateOverride(fc *FunctionCall, ctx evalContext) (interface{}, error) {
	if _, isField := fc.Args[0].(*FieldRef); !isField {
		if _, isQual := fc.Args[0].(*QualifiedRef); !isQual {
			val, err := e.evalExpr(fc.Args[0], ctx)
			if err != nil {
				return nil, err
			}
			if val == nil {
				return nil, nil
			}
			if rec, ok := val.(recurrence.Recurrence); ok {
				return recurrence.NextOccurrence(rec), nil
			}
			return nil, fmt.Errorf("next_date() argument must be a recurrence value, got %T", val)
		}
	}

	val, err := e.evalExpr(fc.Args[0], ctx)
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, nil
	}
	var rec recurrence.Recurrence
	switch v := val.(type) {
	case recurrence.Recurrence:
		rec = v
	case string:
		rec = recurrence.Recurrence(v)
	default:
		return nil, fmt.Errorf("next_date() argument must be a recurrence value, got %T", val)
	}
	return recurrence.NextOccurrence(rec), nil
}

// evalEnumStepOverride implements next_enum / prev_enum for trigger
// contexts. The base executor's evalEnumStep delegates to its own
// evalExpr, which rejects old./new. qualifiers — so a trigger rule like
// `set priority = prev_enum(new.priority)` would silently land on a
// boundary value (because evalEnumStep treated the qualifier-rejection
// error as "absent field"). The override threads the override's own
// evalExpr through evalEnumStepWithLookup so qualified refs resolve via
// resolveQualifiedRef against the trigger's old/new tikis.
func (e *triggerExecOverride) evalEnumStepOverride(fc *FunctionCall, ctx evalContext, direction int) (interface{}, error) {
	return evalEnumStepWithLookup(e.schema, fc, direction, func(arg Expr) (interface{}, error) {
		return e.evalExpr(arg, ctx)
	})
}

// Execute overrides the base Executor to use our evalExpr/evalCondition.
func (e *triggerExecOverride) Execute(stmt any, tikis []Document, inputs ...ExecutionInput) (*Result, error) {
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
		if (validated.usesIDFunc || validated.usesFilepathFunc) && e.runtime.Mode == ExecutorRuntimePlugin {
			if err := checkSingleSelectionForID(input); err != nil {
				return nil, err
			}
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
		return e.executeCreate(rawStmt.Create, tikis, requiresCreateTemplate)
	case rawStmt.Update != nil:
		return e.executeUpdate(rawStmt.Update, tikis)
	case rawStmt.Delete != nil:
		return e.executeDelete(rawStmt.Delete, tikis)
	default:
		return nil, fmt.Errorf("unsupported trigger action type")
	}
}

func (e *triggerExecOverride) executeUpdate(upd *UpdateStmt, tikis []Document) (*Result, error) {
	matched, err := e.filterTikis(upd.Where, tikis)
	if err != nil {
		return nil, err
	}

	clones := make([]Document, len(matched))
	for i, t := range matched {
		clones[i] = t.Clone()
	}

	for _, clone := range clones {
		for _, a := range upd.Set {
			val, err := e.evalExpr(a.Value, evalContext{current: clone, allTikis: tikis, inAssignmentRHS: true})
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

func (e *triggerExecOverride) executeCreate(cr *CreateStmt, tikis []Document, requireTemplate bool) (*Result, error) {
	if requireTemplate && e.currentInput.CreateTemplate == nil {
		return nil, &MissingCreateTemplateError{}
	}
	var t Document
	if e.currentInput.CreateTemplate != nil {
		t = e.currentInput.CreateTemplate.Clone()
	} else {
		if e.factory == nil {
			return nil, fmt.Errorf("create requires a document factory")
		}
		t = e.factory()
	}
	for _, a := range cr.Assignments {
		val, err := e.evalExpr(a.Value, evalContext{current: t, allTikis: tikis, inAssignmentRHS: true})
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", a.Field, err)
		}
		if err := e.setField(t, a.Field, val); err != nil {
			return nil, fmt.Errorf("field %q: %w", a.Field, err)
		}
	}
	return &Result{Create: &CreateResult{Tiki: t}}, nil
}

func (e *triggerExecOverride) executeDelete(del *DeleteStmt, tikis []Document) (*Result, error) {
	matched, err := e.filterTikis(del.Where, tikis)
	if err != nil {
		return nil, err
	}
	return &Result{Delete: &DeleteResult{Deleted: matched}}, nil
}

func (e *triggerExecOverride) filterTikis(where Condition, tikis []Document) ([]Document, error) {
	if where == nil {
		result := make([]Document, len(tikis))
		copy(result, tikis)
		return result, nil
	}
	var result []Document
	for _, t := range tikis {
		match, err := e.evalCondition(where, evalContext{current: t, allTikis: tikis})
		if err != nil {
			return nil, err
		}
		if match {
			result = append(result, t)
		}
	}
	return result, nil
}

// resolveRefTikis finds tikis by ID from a list of ref values.
func resolveRefTikis(refs []interface{}, allTikis []Document) []Document {
	result := make([]Document, 0, len(refs))
	for _, ref := range refs {
		refID := normalizeToString(ref)
		for _, at := range allTikis {
			if equalFoldID(at.ID(), refID) {
				result = append(result, at)
				break
			}
		}
	}
	return result
}

// equalFoldID compares two tiki IDs case-insensitively.
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

// blocksLookup finds all tiki IDs that have the given ID in their dependsOn.
func blocksLookup(val interface{}, allTikis []Document) []interface{} {
	targetID := normalizeToString(val)
	var blockers []interface{}
	for _, at := range allTikis {
		deps, ok := tikiStringSlice(at, fieldDependsOn)
		if !ok {
			continue
		}
		for _, dep := range deps {
			if equalFoldID(dep, targetID) {
				blockers = append(blockers, at.ID())
				break
			}
		}
	}
	if blockers == nil {
		blockers = []interface{}{}
	}
	return blockers
}
