package ruki

import (
	"fmt"
	"strings"
)

// validate.go — structural validation and semantic type-checking.

// qualifierPolicy controls which old./new./outer./target./targets. qualifiers
// are allowed during validation. target. and targets. are plugin-runtime
// qualifiers; the structural validator admits them in standalone statement
// parsing, while the semantic validator rejects them outside plugin runtime.
type qualifierPolicy struct {
	allowOld     bool
	allowNew     bool
	allowOuter   bool
	allowTarget  bool
	allowTargets bool
}

// noQualifiers is the default for standalone statements. target. and targets.
// are admitted structurally here; the semantic validator gates them by runtime.
var noQualifiers = qualifierPolicy{allowTarget: true, allowTargets: true}

func triggerQualifiers(event string) qualifierPolicy {
	switch event {
	case "create":
		return qualifierPolicy{allowNew: true}
	case "delete":
		return qualifierPolicy{allowOld: true}
	default: // "update"
		return qualifierPolicy{allowOld: true, allowNew: true}
	}
}

// known builtins and their return types.
var builtinFuncs = map[string]struct {
	returnType ValueType
	minArgs    int
	maxArgs    int
}{
	"count":          {ValueInt, 1, 1},
	"choose":         {ValueRef, 1, 1},
	"exists":         {ValueBool, 1, 1},
	"has":            {ValueBool, 1, 1},
	"id":             {ValueID, 0, 0},
	"ids":            {ValueListRef, 0, 0},
	"selected_count": {ValueInt, 0, 0},
	"now":            {ValueTimestamp, 0, 0},
	"next_date":      {ValueDate, 1, 1},
	"blocks":         {ValueListRef, 1, 1},
	"call":           {ValueString, 1, 1},
	"user":           {ValueString, 0, 0},
}

// --- structural validation ---

func (p *Parser) validateStatement(s *Statement) error {
	switch {
	case s.Create != nil:
		if len(s.Create.Assignments) == 0 {
			return fmt.Errorf("create must have at least one assignment")
		}
		return p.validateAssignments(s.Create.Assignments)
	case s.Update != nil:
		if len(s.Update.Set) == 0 {
			return fmt.Errorf("update must have at least one assignment in set")
		}
		if err := p.validateCondition(s.Update.Where); err != nil {
			return err
		}
		return p.validateAssignments(s.Update.Set)
	case s.Delete != nil:
		return p.validateCondition(s.Delete.Where)
	case s.Expr != nil:
		return p.validateExprStmt(s.Expr)
	case s.Select != nil:
		if err := p.validateSelectFields(s.Select.Fields); err != nil {
			return err
		}
		if s.Select.Where != nil {
			if err := p.validateCondition(s.Select.Where); err != nil {
				return err
			}
		}
		if err := p.validateOrderBy(s.Select.OrderBy); err != nil {
			return err
		}
		if err := p.validateLimit(s.Select.Limit); err != nil {
			return err
		}
		if s.Select.Pipe != nil {
			if len(s.Select.Fields) == 0 {
				return fmt.Errorf("pipe requires explicit field names in select (not select * or bare select)")
			}
			if s.Select.Pipe.Run != nil {
				typ, err := p.inferExprType(s.Select.Pipe.Run.Command)
				if err != nil {
					return fmt.Errorf("pipe command: %w", err)
				}
				if typ != ValueString {
					return fmt.Errorf("pipe command must be string, got %s", typeName(typ))
				}
				if exprContainsFieldRef(s.Select.Pipe.Run.Command) {
					return fmt.Errorf("pipe command must not contain field references — use $1, $2 for positional args")
				}
			}
			// clipboard() has no arguments — grammar enforces empty parens
		}
		return nil
	default:
		return fmt.Errorf("empty statement")
	}
}

// validateExprStmt type-checks a top-level expression statement. Bare field
// references are rejected because there is no "current task" at the top
// level; references inside subqueries still resolve against their candidate
// task (validateSubQueryFuncCall resets the reject flag).
func (p *Parser) validateExprStmt(es *ExprStmt) error {
	if es == nil || es.Expr == nil {
		return fmt.Errorf("empty expression statement")
	}
	savedReject := p.rejectBareFieldRefs
	p.rejectBareFieldRefs = true
	typ, err := p.inferExprType(es.Expr)
	p.rejectBareFieldRefs = savedReject
	if err != nil {
		return err
	}
	es.Type = typ
	return nil
}

func (p *Parser) validateTrigger(t *Trigger) error {
	if t.Timing == "before" {
		if t.Action != nil || t.Run != nil {
			return fmt.Errorf("before-trigger must not have an action")
		}
		if t.Deny == nil {
			return fmt.Errorf("before-trigger must have deny")
		}
	}
	if t.Timing == "after" {
		if t.Deny != nil {
			return fmt.Errorf("after-trigger must not have deny")
		}
		if t.Action == nil && t.Run == nil {
			return fmt.Errorf("after-trigger must have an action")
		}
	}

	// zone 1: trigger where-guard requires qualifiers
	if t.Where != nil {
		p.requireQualifiers = true
		err := p.validateCondition(t.Where)
		p.requireQualifiers = false
		if err != nil {
			return err
		}
	}

	// zone 2: action statement — bare fields resolve against target task.
	// allowlist the mutating variants so new Statement variants (e.g. Expr)
	// fail closed instead of silently slipping through.
	if t.Action != nil {
		if t.Action.Create == nil && t.Action.Update == nil && t.Action.Delete == nil {
			return fmt.Errorf("trigger action must be create, update, or delete")
		}
		if err := p.validateStatement(t.Action); err != nil {
			return err
		}
	}

	if t.Run != nil {
		typ, err := p.inferExprType(t.Run.Command)
		if err != nil {
			return fmt.Errorf("run command: %w", err)
		}
		if typ != ValueString {
			return fmt.Errorf("run command must be string, got %s", typeName(typ))
		}
	}

	return nil
}

func (p *Parser) validateRule(r *Rule) error {
	switch {
	case r.TimeTrigger != nil:
		// time triggers forbid target./targets. (plugin-only qualifiers)
		p.qualifiers = qualifierPolicy{}
		return p.validateTimeTrigger(r.TimeTrigger)
	case r.Trigger != nil:
		// event triggers forbid target./targets. (plugin-only qualifiers)
		p.qualifiers = triggerQualifiers(r.Trigger.Event)
		return p.validateTrigger(r.Trigger)
	default:
		return fmt.Errorf("empty rule")
	}
}

func (p *Parser) validateTimeTrigger(tt *TimeTrigger) error {
	if tt.Interval.Value <= 0 {
		return fmt.Errorf("every interval must be positive, got %d%s", tt.Interval.Value, tt.Interval.Unit)
	}
	if tt.Action == nil ||
		(tt.Action.Create == nil && tt.Action.Update == nil && tt.Action.Delete == nil) {
		return fmt.Errorf("time trigger action must be create, update, or delete")
	}
	// time triggers forbid all qualifiers including target./targets.
	p.qualifiers = qualifierPolicy{}
	return p.validateStatement(tt.Action)
}

func (p *Parser) validateAssignments(assignments []Assignment) error {
	seen := make(map[string]struct{}, len(assignments))
	for _, a := range assignments {
		if _, dup := seen[a.Field]; dup {
			return fmt.Errorf("duplicate assignment to field %q", a.Field)
		}
		seen[a.Field] = struct{}{}
		fs, ok := p.schema.Field(a.Field)
		if !ok {
			return fmt.Errorf("unknown field %q in assignment", a.Field)
		}
		rhsType, err := p.inferExprType(a.Value)
		if err != nil {
			return fmt.Errorf("field %q: %w", a.Field, err)
		}
		if err := p.checkAssignmentCompat(fs, rhsType, a.Value); err != nil {
			return fmt.Errorf("field %q: %w", a.Field, err)
		}
	}
	return nil
}

// --- select field validation ---

func (p *Parser) validateSelectFields(fields []string) error {
	if len(fields) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		if _, dup := seen[f]; dup {
			return fmt.Errorf("duplicate field %q in select", f)
		}
		seen[f] = struct{}{}
		if _, ok := p.schema.Field(f); !ok {
			return fmt.Errorf("unknown field %q in select", f)
		}
	}
	return nil
}

// --- order by validation ---

func (p *Parser) validateOrderBy(clauses []OrderByClause) error {
	if len(clauses) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(clauses))
	for _, c := range clauses {
		if _, dup := seen[c.Field]; dup {
			return fmt.Errorf("duplicate field %q in order by", c.Field)
		}
		seen[c.Field] = struct{}{}
		fs, ok := p.schema.Field(c.Field)
		if !ok {
			return fmt.Errorf("unknown field %q in order by", c.Field)
		}
		if !isOrderableType(fs.Type) {
			return fmt.Errorf("cannot order by %s field %q", typeName(fs.Type), c.Field)
		}
	}
	return nil
}

// --- limit validation ---

func (p *Parser) validateLimit(limit *int) error {
	if limit == nil {
		return nil
	}
	if *limit <= 0 {
		return fmt.Errorf("limit must be a positive integer, got %d", *limit)
	}
	return nil
}

func isOrderableType(t ValueType) bool {
	switch t {
	case ValueInt, ValueDate, ValueTimestamp, ValueDuration,
		ValueString, ValueStatus, ValueTaskType, ValueID, ValueRef,
		ValueEnum, ValueBool:
		return true
	default:
		return false
	}
}

// --- condition validation with type-checking ---

func (p *Parser) validateCondition(c Condition) error {
	switch c := c.(type) {
	case *BinaryCondition:
		if err := p.validateCondition(c.Left); err != nil {
			return err
		}
		return p.validateCondition(c.Right)

	case *NotCondition:
		return p.validateCondition(c.Inner)

	case *BoolExprCondition:
		return p.validateBoolExprCondition(c)

	case *CompareExpr:
		return p.validateCompare(c)

	case *IsEmptyExpr:
		_, err := p.inferExprType(c.Expr)
		return err

	case *InExpr:
		return p.validateIn(c)

	case *QuantifierExpr:
		return p.validateQuantifier(c)

	default:
		return fmt.Errorf("unknown condition type %T", c)
	}
}

func (p *Parser) validateBoolExprCondition(c *BoolExprCondition) error {
	exprType, err := p.inferExprType(c.Expr)
	if err != nil {
		return err
	}
	if exprType != ValueBool {
		return fmt.Errorf("condition expression must be bool, got %s", typeName(exprType))
	}
	return nil
}

func (p *Parser) validateCompare(c *CompareExpr) error {
	leftType, err := p.inferExprType(c.Left)
	if err != nil {
		return err
	}
	rightType, err := p.inferExprType(c.Right)
	if err != nil {
		return err
	}

	// resolve empty from context
	leftType, rightType = resolveEmptyPair(leftType, rightType)

	// implicit midnight-UTC coercion: timestamp vs date literal
	if leftType == ValueTimestamp {
		if _, ok := c.Right.(*DateLiteral); ok && rightType == ValueDate {
			rightType = ValueTimestamp
		}
	}
	if rightType == ValueTimestamp {
		if _, ok := c.Left.(*DateLiteral); ok && leftType == ValueDate {
			leftType = ValueTimestamp
		}
	}

	// implicit bool coercion: string literal "true"/"false" vs bool field
	if leftType == ValueBool && rightType == ValueString && isBoolStringLiteral(c.Right) {
		rightType = ValueBool
	}
	if rightType == ValueBool && leftType == ValueString && isBoolStringLiteral(c.Left) {
		leftType = ValueBool
	}

	if !typesCompatible(leftType, rightType) {
		return fmt.Errorf("cannot compare %s %s %s", typeName(leftType), c.Op, typeName(rightType))
	}

	// reject cross-type comparisons involving enum fields,
	// unless the other side is a string literal (e.g. status = "done")
	if err := p.checkCompareCompat(leftType, rightType, c.Left, c.Right); err != nil {
		return err
	}

	// use the most specific type for operator and enum validation
	enumType := leftType
	if rightType == ValueStatus || rightType == ValueTaskType || rightType == ValueEnum {
		enumType = rightType
	}

	if err := checkCompareOp(enumType, c.Op); err != nil {
		return err
	}

	return p.validateEnumLiterals(c.Left, c.Right, enumType)
}

func (p *Parser) validateIn(c *InExpr) error {
	valType, err := p.inferExprType(c.Value)
	if err != nil {
		return err
	}

	collType, err := p.inferExprType(c.Collection)
	if err != nil {
		return err
	}

	// list membership mode: collection is a list type
	if listElementType(collType) != -1 {
		elemType, err := p.inferListElementType(c.Collection)
		if err != nil {
			return err
		}
		if !membershipCompatible(valType, elemType) {
			ll, isLiteral := c.Collection.(*ListLiteral)
			// allow enum value checked against a list of string literals
			enumInStringList := isLiteral && valType == ValueEnum && allStringLiterals(ll)
			// allow bool field checked against a list of bool-string literals
			boolInStringList := isLiteral && valType == ValueBool && allBoolStringLiterals(ll)
			// allow an enum-typed value (status, type, or custom enum) on the
			// left-hand side of a `in targets.<enum-field>` projection. The
			// projection collapses enum-typed tasks to list<string>, but the
			// underlying field semantics are still enum, so membership is
			// well-defined when the two sides refer to the same domain.
			enumInTargetsProjection := isEnumType(valType) && p.isTargetsEnumProjection(c.Collection, valType, c.Value)
			if !enumInStringList && !boolInStringList && !enumInTargetsProjection {
				if !isLiteral || !isStringLike(valType) || !allStringLiterals(ll) {
					return fmt.Errorf("element type mismatch: %s in %s", typeName(valType), typeName(collType))
				}
			}
		}
		enumField, _ := exprFieldName(c.Value)
		return p.validateEnumListElements(c.Collection, valType, enumField)
	}

	// substring mode: both sides must be string (not string-like)
	if valType == ValueString && collType == ValueString {
		return nil
	}

	return fmt.Errorf("cannot check %s in %s", typeName(valType), typeName(collType))
}

func (p *Parser) validateQuantifier(q *QuantifierExpr) error {
	exprType, err := p.inferExprType(q.Expr)
	if err != nil {
		return err
	}
	if exprType != ValueListRef {
		return fmt.Errorf("quantifier %s requires list<ref>, got %s", q.Kind, typeName(exprType))
	}
	// zone 3: quantifier bodies — bare fields refer to each related task.
	// old./new. and requireQualifiers are reset, while outer./target./targets.
	// remain allowed when the quantifier itself appears inside a context that
	// already permits them.
	savedQualifiers := p.qualifiers
	savedRequire := p.requireQualifiers
	p.qualifiers = qualifierPolicy{
		allowOuter:   savedQualifiers.allowOuter,
		allowTarget:  savedQualifiers.allowTarget,
		allowTargets: savedQualifiers.allowTargets,
	}
	p.requireQualifiers = false
	err = p.validateCondition(q.Condition)
	p.qualifiers = savedQualifiers
	p.requireQualifiers = savedRequire
	return err
}

// --- type inference ---

func (p *Parser) inferExprType(e Expr) (ValueType, error) {
	switch e := e.(type) {
	case *FieldRef:
		if p.requireQualifiers {
			return 0, fmt.Errorf("bare field %q not allowed in trigger guard — use old.%s or new.%s", e.Name, e.Name, e.Name)
		}
		if p.rejectBareFieldRefs {
			return 0, fmt.Errorf("bare field %q is not valid at the top level (no current task)", e.Name)
		}
		fs, ok := p.schema.Field(e.Name)
		if !ok {
			return 0, fmt.Errorf("unknown field %q", e.Name)
		}
		return fs.Type, nil

	case *QualifiedRef:
		switch e.Qualifier {
		case "old":
			if !p.qualifiers.allowOld {
				return 0, fmt.Errorf("old. qualifier is not valid in this context")
			}
		case "new":
			if !p.qualifiers.allowNew {
				return 0, fmt.Errorf("new. qualifier is not valid in this context")
			}
		case "outer":
			if !p.qualifiers.allowOuter {
				return 0, fmt.Errorf("outer. qualifier is not valid in this context")
			}
		case "target":
			if !p.qualifiers.allowTarget {
				return 0, fmt.Errorf("target. qualifier is not valid in this context")
			}
		case "targets":
			if !p.qualifiers.allowTargets {
				return 0, fmt.Errorf("targets. qualifier is not valid in this context")
			}
		default:
			return 0, fmt.Errorf("unknown qualifier %q", e.Qualifier)
		}
		fs, ok := p.schema.Field(e.Name)
		if !ok {
			return 0, fmt.Errorf("unknown field %q in %s.%s", e.Name, e.Qualifier, e.Name)
		}
		if e.Qualifier == "targets" {
			return projectedListType(fs.Type, e.Name)
		}
		return fs.Type, nil

	case *StringLiteral:
		return ValueString, nil

	case *IntLiteral:
		return ValueInt, nil

	case *DateLiteral:
		return ValueDate, nil

	case *DurationLiteral:
		return ValueDuration, nil

	case *ListLiteral:
		return p.inferListType(e)

	case *BoolLiteral:
		return ValueBool, nil

	case *EmptyLiteral:
		return -1, nil // sentinel: resolved from context

	case *FunctionCall:
		return p.inferFuncCallType(e)

	case *BinaryExpr:
		return p.inferBinaryExprType(e)

	case *SubQuery:
		return 0, fmt.Errorf("subquery is only valid as argument to count(), choose(), or exists()")

	default:
		return 0, fmt.Errorf("unknown expression type %T", e)
	}
}

func (p *Parser) inferListType(l *ListLiteral) (ValueType, error) {
	if len(l.Elements) == 0 {
		return ValueListString, nil // default empty list type
	}
	firstType, err := p.inferExprType(l.Elements[0])
	if err != nil {
		return 0, err
	}
	for i := 1; i < len(l.Elements); i++ {
		t, err := p.inferExprType(l.Elements[i])
		if err != nil {
			return 0, err
		}
		if !typesCompatible(firstType, t) {
			return 0, fmt.Errorf("list elements must be the same type: got %s and %s", typeName(firstType), typeName(t))
		}
	}
	switch firstType {
	case ValueRef, ValueID:
		return ValueListRef, nil
	default:
		return ValueListString, nil
	}
}

// inferListElementType returns the element type of a list expression,
// checking literal elements directly when the list type enum is too coarse.
func (p *Parser) inferListElementType(e Expr) (ValueType, error) {
	if ll, ok := e.(*ListLiteral); ok && len(ll.Elements) > 0 {
		return p.inferExprType(ll.Elements[0])
	}
	collType, err := p.inferExprType(e)
	if err != nil {
		return 0, err
	}
	elem := listElementType(collType)
	if elem == -1 {
		return collType, nil // not a list type — return as-is for error reporting
	}
	return elem, nil
}

func (p *Parser) inferFuncCallType(fc *FunctionCall) (ValueType, error) {
	if fc.Name == "input" {
		if len(fc.Args) != 0 {
			return 0, fmt.Errorf("input() takes no arguments, got %d", len(fc.Args))
		}
		if p.inputType == nil {
			return 0, fmt.Errorf("input() requires 'input:' declaration on action")
		}
		return *p.inputType, nil
	}

	builtin, ok := builtinFuncs[fc.Name]
	if !ok {
		return 0, fmt.Errorf("unknown function %q", fc.Name)
	}
	if len(fc.Args) < builtin.minArgs || len(fc.Args) > builtin.maxArgs {
		if builtin.minArgs == builtin.maxArgs {
			return 0, fmt.Errorf("%s() expects %d argument(s), got %d", fc.Name, builtin.minArgs, len(fc.Args))
		}
		return 0, fmt.Errorf("%s() expects %d-%d arguments, got %d", fc.Name, builtin.minArgs, builtin.maxArgs, len(fc.Args))
	}

	// validate argument types for specific functions
	switch fc.Name {
	case "count", "choose", "exists":
		if err := p.validateSubQueryFuncCall(fc.Name, fc.Args[0]); err != nil {
			return 0, err
		}
	case "has":
		if err := p.validateHasFuncCall(fc.Args[0]); err != nil {
			return 0, err
		}
	case "blocks":
		argType, err := p.inferExprType(fc.Args[0])
		if err != nil {
			return 0, err
		}
		if argType != ValueID && argType != ValueRef && argType != ValueString {
			return 0, fmt.Errorf("blocks() argument must be an id or ref, got %s", typeName(argType))
		}
		if argType == ValueString {
			if _, ok := fc.Args[0].(*StringLiteral); !ok {
				return 0, fmt.Errorf("blocks() argument must be an id or ref, got %s", typeName(argType))
			}
		}
	case "call":
		t, err := p.inferExprType(fc.Args[0])
		if err != nil {
			return 0, err
		}
		if t != ValueString {
			return 0, fmt.Errorf("call() argument must be string, got %s", typeName(t))
		}
	case "next_date":
		t, err := p.inferExprType(fc.Args[0])
		if err != nil {
			return 0, err
		}
		if t != ValueRecurrence {
			return 0, fmt.Errorf("next_date() argument must be recurrence, got %s", typeName(t))
		}
	}

	return builtin.returnType, nil
}

// validateHasFuncCall rejects anything other than a single field reference
// as the argument to has(). Both bare (`has(status)`) and qualified
// (`has(new.status)`, `has(old.assignee)`) forms are accepted — the latter
// are meaningful in trigger contexts where the predicate needs to ask
// "did the new or old version of the task declare this field?". The
// reason for the stricter contract vs. arbitrary expressions is semantic
// clarity: has() answers "is this field *present*", which is only
// meaningful with a concrete field name, not a string literal or function
// call.
//
// Argument validation routes through inferExprType so that bare and
// qualified refs obey the SAME qualifier-policy rules as ordinary
// field references: `has(old.status)` is rejected in CLI context,
// `has(target.status)` is rejected outside plugin target contexts, and
// bare `has(status)` is rejected in trigger guards where bare refs are
// otherwise rejected. Without this, those forms would parse silently
// and either evaluate false at runtime or produce confusing errors —
// hiding authoring mistakes until execution time.
func (p *Parser) validateHasFuncCall(arg Expr) error {
	switch arg.(type) {
	case *FieldRef, *QualifiedRef:
		// fall through to shared qualifier/schema checks below
	default:
		return fmt.Errorf("has() argument must be a field reference, e.g. has(status) or has(new.status)")
	}
	if _, err := p.inferExprType(arg); err != nil {
		return fmt.Errorf("has(): %w", err)
	}
	return nil
}

func (p *Parser) validateSubQueryFuncCall(name string, arg Expr) error {
	sq, ok := arg.(*SubQuery)
	if !ok {
		return fmt.Errorf("%s() argument must be a select subquery", name)
	}
	if sq.Where == nil {
		return nil
	}
	// zone 4: subquery bodies use candidate-task fields, so trigger guards stop requiring qualifiers
	// and top-level expression statements stop rejecting bare field refs.
	// outer. is valid here and resolves to the immediate parent query row.
	savedQualifiers := p.qualifiers
	savedRequire := p.requireQualifiers
	savedReject := p.rejectBareFieldRefs
	p.qualifiers.allowOuter = true
	p.requireQualifiers = false
	p.rejectBareFieldRefs = false
	err := p.validateCondition(sq.Where)
	p.qualifiers = savedQualifiers
	p.requireQualifiers = savedRequire
	p.rejectBareFieldRefs = savedReject
	if err != nil {
		return fmt.Errorf("%s() subquery: %w", name, err)
	}
	return nil
}

func (p *Parser) inferBinaryExprType(b *BinaryExpr) (ValueType, error) {
	leftType, err := p.inferExprType(b.Left)
	if err != nil {
		return 0, err
	}
	rightType, err := p.inferExprType(b.Right)
	if err != nil {
		return 0, err
	}

	leftType, rightType = resolveEmptyPair(leftType, rightType)

	switch b.Op {
	case "+":
		return p.inferPlusType(leftType, rightType, b.Right)
	case "-":
		return p.inferMinusType(leftType, rightType, b.Right)
	default:
		return 0, fmt.Errorf("unknown binary operator %q", b.Op)
	}
}

func isStringLike(t ValueType) bool {
	switch t {
	case ValueString, ValueStatus, ValueTaskType, ValueID, ValueRef:
		return true
	default:
		return false
	}
}

func (p *Parser) inferPlusType(left, right ValueType, rightExpr Expr) (ValueType, error) {
	switch {
	case isStringLike(left) && isStringLike(right):
		return ValueString, nil
	case left == ValueInt && right == ValueInt:
		return ValueInt, nil
	case left == ValueListString && (right == ValueString || right == ValueListString):
		return ValueListString, nil
	case left == ValueListRef && (isRefCompatible(right) || right == ValueListRef):
		return ValueListRef, nil
	case left == ValueListRef && right == ValueString:
		if _, ok := rightExpr.(*StringLiteral); ok {
			return ValueListRef, nil
		}
		return 0, fmt.Errorf("cannot add %s + %s", typeName(left), typeName(right))
	case left == ValueListRef && right == ValueListString:
		if _, ok := rightExpr.(*ListLiteral); ok {
			return ValueListRef, nil
		}
		return 0, fmt.Errorf("cannot add list<string> field to list<ref>")
	case left == ValueDate && right == ValueDuration:
		return ValueDate, nil
	case left == ValueTimestamp && right == ValueDuration:
		return ValueTimestamp, nil
	default:
		return 0, fmt.Errorf("cannot add %s + %s", typeName(left), typeName(right))
	}
}

func (p *Parser) inferMinusType(left, right ValueType, rightExpr Expr) (ValueType, error) {
	switch {
	case left == ValueListString && (right == ValueString || right == ValueListString):
		return ValueListString, nil
	case left == ValueListRef && (isRefCompatible(right) || right == ValueListRef):
		return ValueListRef, nil
	case left == ValueListRef && right == ValueString:
		if _, ok := rightExpr.(*StringLiteral); ok {
			return ValueListRef, nil
		}
		return 0, fmt.Errorf("cannot subtract %s - %s", typeName(left), typeName(right))
	case left == ValueListRef && right == ValueListString:
		if _, ok := rightExpr.(*ListLiteral); ok {
			return ValueListRef, nil
		}
		return 0, fmt.Errorf("cannot subtract list<string> field from list<ref>")
	case left == ValueInt && right == ValueInt:
		return ValueInt, nil
	case left == ValueDate && right == ValueDuration:
		return ValueDate, nil
	case left == ValueDate && right == ValueDate:
		return ValueDuration, nil
	case left == ValueTimestamp && right == ValueDuration:
		return ValueTimestamp, nil
	case left == ValueTimestamp && right == ValueTimestamp:
		return ValueDuration, nil
	default:
		return 0, fmt.Errorf("cannot subtract %s - %s", typeName(left), typeName(right))
	}
}

// --- enum literal validation ---

func (p *Parser) validateEnumLiterals(left, right Expr, resolvedType ValueType) error {
	if resolvedType == ValueStatus {
		if s, ok := right.(*StringLiteral); ok {
			if _, valid := p.schema.NormalizeStatus(s.Value); !valid {
				return fmt.Errorf("unknown status %q", s.Value)
			}
		}
		if s, ok := left.(*StringLiteral); ok {
			if _, valid := p.schema.NormalizeStatus(s.Value); !valid {
				return fmt.Errorf("unknown status %q", s.Value)
			}
		}
	}
	if resolvedType == ValueTaskType {
		if s, ok := right.(*StringLiteral); ok {
			if _, valid := p.schema.NormalizeType(s.Value); !valid {
				return fmt.Errorf("unknown type %q", s.Value)
			}
		}
		if s, ok := left.(*StringLiteral); ok {
			if _, valid := p.schema.NormalizeType(s.Value); !valid {
				return fmt.Errorf("unknown type %q", s.Value)
			}
		}
	}
	if resolvedType == ValueEnum {
		fieldName, _ := exprFieldName(left)
		if fieldName == "" {
			fieldName, _ = exprFieldName(right)
		}
		if fieldName != "" {
			if s, ok := right.(*StringLiteral); ok {
				if _, valid := p.normalizeEnumValue(fieldName, s.Value); !valid {
					return fmt.Errorf("unknown value %q for field %q", s.Value, fieldName)
				}
			}
			if s, ok := left.(*StringLiteral); ok {
				if _, valid := p.normalizeEnumValue(fieldName, s.Value); !valid {
					return fmt.Errorf("unknown value %q for field %q", s.Value, fieldName)
				}
			}
		}
	}
	return nil
}

// validateEnumListElements checks string literals inside a list expression
// against the appropriate enum normalizer, based on the value type being checked.
// enumFieldName is only used when valType is ValueEnum — it identifies the custom
// enum field whose AllowedValues should be checked against.
func (p *Parser) validateEnumListElements(collection Expr, valType ValueType, enumFieldName string) error {
	ll, ok := collection.(*ListLiteral)
	if !ok {
		return nil
	}
	for _, elem := range ll.Elements {
		s, ok := elem.(*StringLiteral)
		if !ok {
			continue
		}
		switch valType {
		case ValueStatus:
			if _, valid := p.schema.NormalizeStatus(s.Value); !valid {
				return fmt.Errorf("unknown status %q", s.Value)
			}
		case ValueTaskType:
			if _, valid := p.schema.NormalizeType(s.Value); !valid {
				return fmt.Errorf("unknown type %q", s.Value)
			}
		case ValueEnum:
			if enumFieldName != "" {
				if _, valid := p.normalizeEnumValue(enumFieldName, s.Value); !valid {
					return fmt.Errorf("unknown value %q for field %q", s.Value, enumFieldName)
				}
			}
		}
	}
	return nil
}

// --- assignment compatibility ---

func (p *Parser) checkAssignmentCompat(fs FieldSpec, rhsType ValueType, rhs Expr) error {
	fieldType := fs.Type

	// empty is assignable to anything
	if _, ok := rhs.(*EmptyLiteral); ok {
		return nil
	}
	if rhsType == -1 { // unresolved empty
		return nil
	}

	// implicit midnight-UTC coercion: date literal assignable to timestamp field
	if fieldType == ValueTimestamp && rhsType == ValueDate {
		if _, ok := rhs.(*DateLiteral); ok {
			return nil
		}
	}

	// implicit bool coercion: string literal "true"/"false" assignable to bool field
	if fieldType == ValueBool && rhsType == ValueString && isBoolStringLiteral(rhs) {
		return nil
	}

	if typesCompatible(fieldType, rhsType) {
		// built-in enum fields only accept same-type or string literals
		if (fieldType == ValueStatus || fieldType == ValueTaskType) && rhsType != fieldType {
			if _, ok := rhs.(*StringLiteral); !ok {
				return fmt.Errorf("cannot assign %s to %s field", typeName(rhsType), typeName(fieldType))
			}
		}
		// custom enum fields only accept same-field enum or string literals
		if fieldType == ValueEnum && rhsType != ValueEnum {
			if _, ok := rhs.(*StringLiteral); !ok {
				return fmt.Errorf("cannot assign %s to %s field", typeName(rhsType), typeName(fieldType))
			}
		}
		if fieldType == ValueEnum && rhsType == ValueEnum {
			// reject cross-field enum assignment
			rhsField, _ := exprFieldName(rhs)
			if rhsField != "" && rhsField != fs.Name {
				return fmt.Errorf("cannot assign field %q to enum field %q (different enum domains)", rhsField, fs.Name)
			}
		}
		// non-enum string-like fields reject enum-typed RHS
		if (fieldType == ValueString || fieldType == ValueID || fieldType == ValueRef) &&
			(rhsType == ValueStatus || rhsType == ValueTaskType || rhsType == ValueEnum) {
			return fmt.Errorf("cannot assign %s to %s field", typeName(rhsType), typeName(fieldType))
		}

		// list<string> field rejects list literals with non-string elements
		if fieldType == ValueListString {
			if ll, ok := rhs.(*ListLiteral); ok {
				for _, elem := range ll.Elements {
					elemType, err := p.inferExprType(elem)
					if err == nil && elemType != ValueString {
						if _, isLit := elem.(*StringLiteral); !isLit {
							return fmt.Errorf("cannot assign %s to list<string> field", typeName(elemType))
						}
					}
				}
			}
		}

		// validate built-in enum values
		if fieldType == ValueStatus {
			if s, ok := rhs.(*StringLiteral); ok {
				if _, valid := p.schema.NormalizeStatus(s.Value); !valid {
					return fmt.Errorf("unknown status %q", s.Value)
				}
			}
		}
		if fieldType == ValueTaskType {
			if s, ok := rhs.(*StringLiteral); ok {
				if _, valid := p.schema.NormalizeType(s.Value); !valid {
					return fmt.Errorf("unknown type %q", s.Value)
				}
			}
		}
		// validate custom enum values
		if fieldType == ValueEnum {
			if s, ok := rhs.(*StringLiteral); ok {
				if _, valid := p.normalizeEnumValue(fs.Name, s.Value); !valid {
					return fmt.Errorf("unknown value %q for field %q", s.Value, fs.Name)
				}
			}
		}
		return nil
	}

	// list<string> literal is assignable to list<ref>, but only if all elements are string literals
	if fieldType == ValueListRef && rhsType == ValueListString {
		if ll, ok := rhs.(*ListLiteral); ok && allStringLiterals(ll) {
			return nil
		}
	}

	return fmt.Errorf("cannot assign %s to %s field", typeName(rhsType), typeName(fieldType))
}

// --- type helpers ---

func typesCompatible(a, b ValueType) bool {
	if a == b {
		return true
	}
	if a == -1 || b == -1 { // unresolved empty
		return true
	}
	// string-like types are compatible with each other for comparison/assignment
	stringLike := map[ValueType]bool{
		ValueString:   true,
		ValueStatus:   true,
		ValueTaskType: true,
		ValueID:       true,
		ValueRef:      true,
		ValueEnum:     true,
	}
	return stringLike[a] && stringLike[b]
}

func isEnumType(t ValueType) bool {
	return t == ValueStatus || t == ValueTaskType || t == ValueEnum
}

// allStringLiterals returns true if every element in the list is a *StringLiteral.
func allStringLiterals(ll *ListLiteral) bool {
	for _, elem := range ll.Elements {
		if _, ok := elem.(*StringLiteral); !ok {
			return false
		}
	}
	return true
}

// isBoolStringLiteral reports whether e is a StringLiteral with value "true" or "false".
// Used to coerce legacy-converted string values into bool-compatible operands.
func isBoolStringLiteral(e Expr) bool {
	s, ok := e.(*StringLiteral)
	if !ok {
		return false
	}
	return strings.EqualFold(s.Value, "true") || strings.EqualFold(s.Value, "false")
}

// allBoolStringLiterals reports whether every element in the list is a bool-string literal.
func allBoolStringLiterals(ll *ListLiteral) bool {
	for _, elem := range ll.Elements {
		if !isBoolStringLiteral(elem) {
			return false
		}
	}
	return true
}

// checkCompareCompat rejects nonsensical cross-type comparisons in WHERE clauses.
// e.g. status = title (enum vs string field) is rejected,
// but status = "done" (enum vs string literal) is allowed.
func (p *Parser) checkCompareCompat(leftType, rightType ValueType, left, right Expr) error {
	// two custom enum fields: must reference the same field
	if leftType == ValueEnum && rightType == ValueEnum {
		lf, _ := exprFieldName(left)
		rf, _ := exprFieldName(right)
		if lf != "" && rf != "" && lf != rf {
			return fmt.Errorf("cannot compare enum field %q with enum field %q (different enum domains)", lf, rf)
		}
		return nil
	}

	if isEnumType(leftType) && rightType != leftType {
		if err := checkEnumOperand(leftType, rightType, right); err != nil {
			return err
		}
	}
	if isEnumType(rightType) && leftType != rightType {
		if err := checkEnumOperand(rightType, leftType, left); err != nil {
			return err
		}
	}
	return nil
}

func checkEnumOperand(enumType, otherType ValueType, other Expr) error {
	if otherType == ValueString {
		if _, ok := other.(*StringLiteral); !ok {
			return fmt.Errorf("cannot compare %s with %s field", typeName(enumType), typeName(otherType))
		}
		return nil
	}
	return fmt.Errorf("cannot compare %s with %s", typeName(enumType), typeName(otherType))
}

// membershipCompatible checks strict type compatibility for in/not in
// expressions. Unlike typesCompatible, it does not treat all string-like
// types as interchangeable — only ID and Ref are interchangeable.
func membershipCompatible(a, b ValueType) bool {
	if a == b {
		return true
	}
	if a == -1 || b == -1 {
		return true
	}
	// ID and Ref are the same concept
	if (a == ValueID || a == ValueRef) && (b == ValueID || b == ValueRef) {
		return true
	}
	return false
}

// isRefCompatible returns true for types that can appear as operands
// in list<ref> add/remove operations.
func isRefCompatible(t ValueType) bool {
	switch t {
	case ValueRef, ValueID:
		return true
	default:
		return false
	}
}

func resolveEmptyPair(a, b ValueType) (ValueType, ValueType) {
	if a == -1 && b != -1 {
		a = b
	}
	if b == -1 && a != -1 {
		b = a
	}
	return a, b
}

// projectedListType returns the list type produced by targets.<field> for a
// field whose underlying type is fieldType. It rejects scalar field types
// that have no list<T> representation in ruki today (int, date, timestamp,
// duration, bool, recurrence).
func projectedListType(fieldType ValueType, fieldName string) (ValueType, error) {
	switch fieldType {
	case ValueID, ValueRef, ValueListRef:
		return ValueListRef, nil
	case ValueString, ValueStatus, ValueTaskType, ValueEnum, ValueListString:
		return ValueListString, nil
	default:
		return 0, fmt.Errorf("targets.%s is not supported: no list<%s> representation", fieldName, typeName(fieldType))
	}
}

// isTargetsEnumProjection reports whether collection is `targets.<field>`
// referring to a schema field whose type matches the enum-typed value on the
// other side of the membership check. This lets `status in targets.status`,
// `type in targets.type`, and `<custom-enum> in targets.<same-enum>` validate
// despite the projection widening the element type to string. For custom
// enums it further requires that both operands name the same enum field so
// cross-domain comparisons stay rejected.
func (p *Parser) isTargetsEnumProjection(collection Expr, valType ValueType, value Expr) bool {
	qr, ok := collection.(*QualifiedRef)
	if !ok || qr.Qualifier != "targets" {
		return false
	}
	fs, ok := p.schema.Field(qr.Name)
	if !ok {
		return false
	}
	if fs.Type != valType {
		return false
	}
	if valType == ValueEnum {
		lhs, _ := exprFieldName(value)
		return lhs != "" && lhs == qr.Name
	}
	return true
}

func listElementType(t ValueType) ValueType {
	switch t {
	case ValueListString:
		return ValueString
	case ValueListRef:
		return ValueRef
	default:
		return -1
	}
}

func checkCompareOp(t ValueType, op string) error {
	switch op {
	case "=", "!=":
		return nil // all types support equality
	case "<", ">", "<=", ">=":
		switch t {
		case ValueInt, ValueDate, ValueTimestamp, ValueDuration:
			return nil
		default:
			return fmt.Errorf("operator %s not supported for %s", op, typeName(t))
		}
	default:
		return fmt.Errorf("unknown operator %q", op)
	}
}

func typeName(t ValueType) string {
	switch t {
	case ValueString:
		return "string"
	case ValueInt:
		return "int"
	case ValueDate:
		return "date"
	case ValueTimestamp:
		return "timestamp"
	case ValueDuration:
		return "duration"
	case ValueBool:
		return "bool"
	case ValueID:
		return "id"
	case ValueRef:
		return "ref"
	case ValueRecurrence:
		return "recurrence"
	case ValueListString:
		return "list<string>"
	case ValueListRef:
		return "list<ref>"
	case ValueStatus:
		return "status"
	case ValueTaskType:
		return "type"
	case ValueEnum:
		return "enum"
	case -1:
		return "empty"
	default:
		return "unknown"
	}
}

// exprContainsFieldRef returns true if the expression tree contains any
// *FieldRef or *QualifiedRef node. Used to reject field references in
// pipe commands, where positional args ($1, $2) should be used instead.
func exprContainsFieldRef(expr Expr) bool {
	switch e := expr.(type) {
	case *FieldRef:
		return true
	case *QualifiedRef:
		return true
	case *BoolLiteral:
		return false
	case *BinaryExpr:
		return exprContainsFieldRef(e.Left) || exprContainsFieldRef(e.Right)
	case *FunctionCall:
		for _, arg := range e.Args {
			if exprContainsFieldRef(arg) {
				return true
			}
		}
		return false
	case *ListLiteral:
		for _, elem := range e.Elements {
			if exprContainsFieldRef(elem) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// exprFieldName extracts the field name from a FieldRef or QualifiedRef.
// Returns ("", false) for any other expression type.
//
//nolint:unparam // bool return is used by callers; string is used in enum domain checks
func exprFieldName(expr Expr) (string, bool) {
	switch e := expr.(type) {
	case *FieldRef:
		return e.Name, true
	case *QualifiedRef:
		return e.Name, true
	default:
		return "", false
	}
}

// normalizeEnumValue validates a raw string against a custom enum field's
// AllowedValues (case-insensitive). Returns the canonical value and true,
// or ("", false) if not found.
//
//nolint:unparam // canonical string return reserved for future use in normalization paths
func (p *Parser) normalizeEnumValue(fieldName, raw string) (string, bool) {
	fs, ok := p.schema.Field(fieldName)
	if !ok {
		return "", false
	}
	for _, av := range fs.AllowedValues {
		if strings.EqualFold(av, raw) {
			return av, true
		}
	}
	return "", false
}
