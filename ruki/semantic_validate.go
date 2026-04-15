package ruki

import (
	"fmt"

	"github.com/boolean-maybe/tiki/task"
)

type validationSeal struct{}

var validatedSeal = &validationSeal{}

// UnvalidatedWrapperError is returned when a validated wrapper was not created
// by semantic validator constructors.
type UnvalidatedWrapperError struct {
	Wrapper string
}

func (e *UnvalidatedWrapperError) Error() string {
	return fmt.Sprintf("%s wrapper is not semantically validated", e.Wrapper)
}

// ValidatedStatement is an immutable, semantically validated statement wrapper.
type ValidatedStatement struct {
	seal       *validationSeal
	runtime    ExecutorRuntimeMode
	usesIDFunc bool
	statement  *Statement
}

func (v *ValidatedStatement) RuntimeMode() ExecutorRuntimeMode { return v.runtime }
func (v *ValidatedStatement) UsesIDBuiltin() bool              { return v.usesIDFunc }
func (v *ValidatedStatement) RequiresCreateTemplate() bool {
	return v != nil && v.statement != nil && v.statement.Create != nil
}

func (v *ValidatedStatement) IsSelect() bool {
	return v != nil && v.statement != nil && v.statement.Select != nil
}
func (v *ValidatedStatement) IsUpdate() bool {
	return v != nil && v.statement != nil && v.statement.Update != nil
}
func (v *ValidatedStatement) IsCreate() bool {
	return v != nil && v.statement != nil && v.statement.Create != nil
}
func (v *ValidatedStatement) IsDelete() bool {
	return v != nil && v.statement != nil && v.statement.Delete != nil
}
func (v *ValidatedStatement) IsPipe() bool {
	return v != nil && v.statement != nil && v.statement.Select != nil && v.statement.Select.Pipe != nil
}
func (v *ValidatedStatement) IsClipboardPipe() bool {
	return v.IsPipe() && v.statement.Select.Pipe.Clipboard != nil
}

func (v *ValidatedStatement) mustBeSealed() error {
	if v == nil || v.seal != validatedSeal || v.statement == nil {
		return &UnvalidatedWrapperError{Wrapper: "statement"}
	}
	return nil
}

// ValidatedTrigger is an immutable, semantically validated event-trigger wrapper.
type ValidatedTrigger struct {
	seal       *validationSeal
	runtime    ExecutorRuntimeMode
	usesIDFunc bool
	trigger    *Trigger
}

func (v *ValidatedTrigger) RuntimeMode() ExecutorRuntimeMode { return v.runtime }
func (v *ValidatedTrigger) UsesIDBuiltin() bool              { return v.usesIDFunc }
func (v *ValidatedTrigger) Timing() string {
	if v == nil || v.trigger == nil {
		return ""
	}
	return v.trigger.Timing
}
func (v *ValidatedTrigger) Event() string {
	if v == nil || v.trigger == nil {
		return ""
	}
	return v.trigger.Event
}
func (v *ValidatedTrigger) HasRunAction() bool {
	return v != nil && v.trigger != nil && v.trigger.Run != nil
}
func (v *ValidatedTrigger) DenyMessage() (string, bool) {
	if v == nil || v.trigger == nil || v.trigger.Deny == nil {
		return "", false
	}
	return *v.trigger.Deny, true
}
func (v *ValidatedTrigger) RequiresCreateTemplate() bool {
	return v != nil && v.trigger != nil && v.trigger.Action != nil && v.trigger.Action.Create != nil
}
func (v *ValidatedTrigger) TriggerClone() *Trigger {
	if v == nil {
		return nil
	}
	return cloneTrigger(v.trigger)
}

func (v *ValidatedTrigger) mustBeSealed() error {
	if v == nil || v.seal != validatedSeal || v.trigger == nil {
		return &UnvalidatedWrapperError{Wrapper: "trigger"}
	}
	return nil
}

// ValidatedTimeTrigger is an immutable, semantically validated time-trigger wrapper.
type ValidatedTimeTrigger struct {
	seal        *validationSeal
	runtime     ExecutorRuntimeMode
	usesIDFunc  bool
	timeTrigger *TimeTrigger
}

func (v *ValidatedTimeTrigger) RuntimeMode() ExecutorRuntimeMode { return v.runtime }
func (v *ValidatedTimeTrigger) UsesIDBuiltin() bool              { return v.usesIDFunc }
func (v *ValidatedTimeTrigger) IntervalLiteral() DurationLiteral {
	if v == nil || v.timeTrigger == nil {
		return DurationLiteral{}
	}
	return v.timeTrigger.Interval
}
func (v *ValidatedTimeTrigger) RequiresCreateTemplate() bool {
	return v != nil && v.timeTrigger != nil && v.timeTrigger.Action != nil && v.timeTrigger.Action.Create != nil
}
func (v *ValidatedTimeTrigger) TimeTriggerClone() *TimeTrigger {
	if v == nil {
		return nil
	}
	return cloneTimeTrigger(v.timeTrigger)
}

func (v *ValidatedTimeTrigger) mustBeSealed() error {
	if v == nil || v.seal != validatedSeal || v.timeTrigger == nil {
		return &UnvalidatedWrapperError{Wrapper: "time trigger"}
	}
	return nil
}

// ValidatedRule is a discriminated union for ParseAndValidateRule.
type ValidatedRule interface {
	isValidatedRule()
	RuntimeMode() ExecutorRuntimeMode
}

// ValidatedEventRule wraps a validated event trigger.
type ValidatedEventRule struct {
	seal    *validationSeal
	trigger *ValidatedTrigger
}

func (ValidatedEventRule) isValidatedRule() {}
func (r ValidatedEventRule) RuntimeMode() ExecutorRuntimeMode {
	if r.trigger == nil {
		return ""
	}
	return r.trigger.RuntimeMode()
}
func (r ValidatedEventRule) Trigger() *ValidatedTrigger { return r.trigger }

// ValidatedTimeRule wraps a validated time trigger.
type ValidatedTimeRule struct {
	seal *validationSeal
	time *ValidatedTimeTrigger
}

func (ValidatedTimeRule) isValidatedRule() {}
func (r ValidatedTimeRule) RuntimeMode() ExecutorRuntimeMode {
	if r.time == nil {
		return ""
	}
	return r.time.RuntimeMode()
}
func (r ValidatedTimeRule) TimeTrigger() *ValidatedTimeTrigger { return r.time }

// SemanticValidator performs runtime-aware semantic validation after parse/type validation.
type SemanticValidator struct {
	runtime ExecutorRuntimeMode
}

// NewSemanticValidator creates a semantic validator for a specific runtime mode.
func NewSemanticValidator(runtime ExecutorRuntimeMode) *SemanticValidator {
	if runtime == "" {
		runtime = ExecutorRuntimeCLI
	}
	return &SemanticValidator{runtime: runtime}
}

// ParseAndValidateStatement parses a statement and applies runtime-aware semantic validation.
func (p *Parser) ParseAndValidateStatement(input string, runtime ExecutorRuntimeMode) (*ValidatedStatement, error) {
	stmt, err := p.ParseStatement(input)
	if err != nil {
		return nil, err
	}
	return NewSemanticValidator(runtime).ValidateStatement(stmt)
}

// ParseAndValidateTrigger parses an event trigger and applies runtime-aware semantic validation.
func (p *Parser) ParseAndValidateTrigger(input string, runtime ExecutorRuntimeMode) (*ValidatedTrigger, error) {
	trig, err := p.ParseTrigger(input)
	if err != nil {
		return nil, err
	}
	return NewSemanticValidator(runtime).ValidateTrigger(trig)
}

// ParseAndValidateTimeTrigger parses a time trigger and applies runtime-aware semantic validation.
func (p *Parser) ParseAndValidateTimeTrigger(input string, runtime ExecutorRuntimeMode) (*ValidatedTimeTrigger, error) {
	tt, err := p.ParseTimeTrigger(input)
	if err != nil {
		return nil, err
	}
	return NewSemanticValidator(runtime).ValidateTimeTrigger(tt)
}

// ParseAndValidateRule parses a trigger rule union and applies the correct semantic runtime
// validation branch (event trigger vs time trigger).
func (p *Parser) ParseAndValidateRule(input string) (ValidatedRule, error) {
	rule, err := p.ParseRule(input)
	if err != nil {
		return nil, err
	}
	switch {
	case rule == nil:
		return nil, fmt.Errorf("empty rule")
	case rule.Trigger != nil:
		vt, err := NewSemanticValidator(ExecutorRuntimeEventTrigger).ValidateTrigger(rule.Trigger)
		if err != nil {
			return nil, err
		}
		return ValidatedEventRule{seal: validatedSeal, trigger: vt}, nil
	case rule.TimeTrigger != nil:
		vt, err := NewSemanticValidator(ExecutorRuntimeTimeTrigger).ValidateTimeTrigger(rule.TimeTrigger)
		if err != nil {
			return nil, err
		}
		return ValidatedTimeRule{seal: validatedSeal, time: vt}, nil
	default:
		return nil, fmt.Errorf("empty rule")
	}
}

// ValidateStatement applies runtime-aware semantic checks to a parsed statement.
func (v *SemanticValidator) ValidateStatement(stmt *Statement) (*ValidatedStatement, error) {
	if stmt == nil {
		return nil, fmt.Errorf("nil statement")
	}
	usesID, hasCall, err := scanStatementSemantics(stmt)
	if err != nil {
		return nil, err
	}
	if hasCall {
		return nil, fmt.Errorf("call() is not supported yet")
	}
	if usesID && v.runtime != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("id() is only available in plugin runtime")
	}
	if err := validateStatementAssignmentsSemantics(stmt); err != nil {
		return nil, err
	}
	return &ValidatedStatement{
		seal:       validatedSeal,
		runtime:    v.runtime,
		usesIDFunc: usesID,
		statement:  cloneStatement(stmt),
	}, nil
}

// ValidateTrigger applies runtime-aware semantic checks to a parsed event trigger.
func (v *SemanticValidator) ValidateTrigger(trig *Trigger) (*ValidatedTrigger, error) {
	if trig == nil {
		return nil, fmt.Errorf("nil trigger")
	}
	usesID, hasCall, err := scanTriggerSemantics(trig)
	if err != nil {
		return nil, err
	}
	if hasCall {
		return nil, fmt.Errorf("call() is not supported yet")
	}
	if usesID && v.runtime != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("id() is only available in plugin runtime")
	}
	if trig.Action != nil {
		if err := validateStatementAssignmentsSemantics(trig.Action); err != nil {
			return nil, err
		}
	}
	return &ValidatedTrigger{
		seal:       validatedSeal,
		runtime:    v.runtime,
		usesIDFunc: usesID,
		trigger:    cloneTrigger(trig),
	}, nil
}

// ValidateTimeTrigger applies runtime-aware semantic checks to a parsed time trigger.
func (v *SemanticValidator) ValidateTimeTrigger(tt *TimeTrigger) (*ValidatedTimeTrigger, error) {
	if tt == nil {
		return nil, fmt.Errorf("nil time trigger")
	}
	usesID, hasCall, err := scanTimeTriggerSemantics(tt)
	if err != nil {
		return nil, err
	}
	if hasCall {
		return nil, fmt.Errorf("call() is not supported yet")
	}
	if usesID && v.runtime != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("id() is only available in plugin runtime")
	}
	if tt.Action != nil {
		if err := validateStatementAssignmentsSemantics(tt.Action); err != nil {
			return nil, err
		}
	}
	return &ValidatedTimeTrigger{
		seal:        validatedSeal,
		runtime:     v.runtime,
		usesIDFunc:  usesID,
		timeTrigger: cloneTimeTrigger(tt),
	}, nil
}

func validateStatementAssignmentsSemantics(stmt *Statement) error {
	switch {
	case stmt.Create != nil:
		return validateAssignmentsSemantics(stmt.Create.Assignments)
	case stmt.Update != nil:
		return validateAssignmentsSemantics(stmt.Update.Set)
	default:
		return nil
	}
}

func validateAssignmentsSemantics(assignments []Assignment) error {
	for _, a := range assignments {
		switch a.Field {
		case "id", "createdBy", "createdAt", "updatedAt":
			return fmt.Errorf("field %q is immutable", a.Field)
		}
		switch a.Field {
		case "title", "status", "type", "priority":
			if _, ok := a.Value.(*EmptyLiteral); ok {
				return fmt.Errorf("field %q cannot be empty", a.Field)
			}
		}
		switch a.Field {
		case "priority":
			if lit, ok := a.Value.(*IntLiteral); ok && !task.IsValidPriority(lit.Value) {
				return fmt.Errorf("priority value out of range: %d", lit.Value)
			}
		case "points":
			if lit, ok := a.Value.(*IntLiteral); ok && !task.IsValidPoints(lit.Value) {
				return fmt.Errorf("points value out of range: %d", lit.Value)
			}
		}
	}
	return nil
}

func scanStatementSemantics(stmt *Statement) (usesID bool, hasCall bool, err error) {
	switch {
	case stmt.Select != nil:
		return scanSelectSemantics(stmt.Select)
	case stmt.Create != nil:
		return scanAssignmentsSemantics(stmt.Create.Assignments)
	case stmt.Update != nil:
		u1, c1, err := scanConditionSemantics(stmt.Update.Where)
		if err != nil {
			return false, false, err
		}
		u2, c2, err := scanAssignmentsSemantics(stmt.Update.Set)
		if err != nil {
			return false, false, err
		}
		return u1 || u2, c1 || c2, nil
	case stmt.Delete != nil:
		return scanConditionSemantics(stmt.Delete.Where)
	default:
		return false, false, fmt.Errorf("empty statement")
	}
}

func scanSelectSemantics(sel *SelectStmt) (usesID bool, hasCall bool, err error) {
	if sel == nil {
		return false, false, nil
	}
	u, c, err := scanConditionSemantics(sel.Where)
	if err != nil {
		return false, false, err
	}
	if sel.Pipe != nil && sel.Pipe.Run != nil {
		u2, c2, err := scanExprSemantics(sel.Pipe.Run.Command)
		if err != nil {
			return false, false, err
		}
		u, c = u || u2, c || c2
	}
	return u, c, nil
}

func scanTriggerSemantics(trig *Trigger) (usesID bool, hasCall bool, err error) {
	var u, c bool
	if trig.Where != nil {
		uu, cc, err := scanConditionSemantics(trig.Where)
		if err != nil {
			return false, false, err
		}
		u, c = u || uu, c || cc
	}
	if trig.Action != nil {
		uu, cc, err := scanStatementSemantics(trig.Action)
		if err != nil {
			return false, false, err
		}
		u, c = u || uu, c || cc
	}
	if trig.Run != nil {
		uu, cc, err := scanExprSemantics(trig.Run.Command)
		if err != nil {
			return false, false, err
		}
		u, c = u || uu, c || cc
	}
	return u, c, nil
}

func scanTimeTriggerSemantics(tt *TimeTrigger) (usesID bool, hasCall bool, err error) {
	if tt == nil || tt.Action == nil {
		return false, false, nil
	}
	return scanStatementSemantics(tt.Action)
}

func scanAssignmentsSemantics(assignments []Assignment) (usesID bool, hasCall bool, err error) {
	for _, a := range assignments {
		u, c, err := scanExprSemantics(a.Value)
		if err != nil {
			return false, false, err
		}
		usesID = usesID || u
		hasCall = hasCall || c
	}
	return usesID, hasCall, nil
}

func scanConditionSemantics(cond Condition) (usesID bool, hasCall bool, err error) {
	if cond == nil {
		return false, false, nil
	}
	switch c := cond.(type) {
	case *BinaryCondition:
		u1, c1, err := scanConditionSemantics(c.Left)
		if err != nil {
			return false, false, err
		}
		u2, c2, err := scanConditionSemantics(c.Right)
		if err != nil {
			return false, false, err
		}
		return u1 || u2, c1 || c2, nil
	case *NotCondition:
		return scanConditionSemantics(c.Inner)
	case *CompareExpr:
		u1, c1, err := scanExprSemantics(c.Left)
		if err != nil {
			return false, false, err
		}
		u2, c2, err := scanExprSemantics(c.Right)
		if err != nil {
			return false, false, err
		}
		return u1 || u2, c1 || c2, nil
	case *IsEmptyExpr:
		return scanExprSemantics(c.Expr)
	case *InExpr:
		u1, c1, err := scanExprSemantics(c.Value)
		if err != nil {
			return false, false, err
		}
		u2, c2, err := scanExprSemantics(c.Collection)
		if err != nil {
			return false, false, err
		}
		return u1 || u2, c1 || c2, nil
	case *QuantifierExpr:
		u1, c1, err := scanExprSemantics(c.Expr)
		if err != nil {
			return false, false, err
		}
		u2, c2, err := scanConditionSemantics(c.Condition)
		if err != nil {
			return false, false, err
		}
		return u1 || u2, c1 || c2, nil
	default:
		return false, false, fmt.Errorf("unknown condition type %T", c)
	}
}

func scanExprSemantics(expr Expr) (usesID bool, hasCall bool, err error) {
	if expr == nil {
		return false, false, nil
	}
	switch e := expr.(type) {
	case *FunctionCall:
		if e.Name == "id" {
			usesID = true
		}
		if e.Name == "call" {
			hasCall = true
		}
		for _, arg := range e.Args {
			u, c, err := scanExprSemantics(arg)
			if err != nil {
				return false, false, err
			}
			usesID = usesID || u
			hasCall = hasCall || c
		}
		return usesID, hasCall, nil
	case *BinaryExpr:
		u1, c1, err := scanExprSemantics(e.Left)
		if err != nil {
			return false, false, err
		}
		u2, c2, err := scanExprSemantics(e.Right)
		if err != nil {
			return false, false, err
		}
		return u1 || u2, c1 || c2, nil
	case *ListLiteral:
		for _, elem := range e.Elements {
			u, c, err := scanExprSemantics(elem)
			if err != nil {
				return false, false, err
			}
			usesID = usesID || u
			hasCall = hasCall || c
		}
		return usesID, hasCall, nil
	case *SubQuery:
		return scanConditionSemantics(e.Where)
	default:
		return false, false, nil
	}
}

func cloneStatement(stmt *Statement) *Statement {
	if stmt == nil {
		return nil
	}
	out := &Statement{}
	if stmt.Select != nil {
		out.Select = cloneSelect(stmt.Select)
	}
	if stmt.Create != nil {
		out.Create = cloneCreate(stmt.Create)
	}
	if stmt.Update != nil {
		out.Update = cloneUpdate(stmt.Update)
	}
	if stmt.Delete != nil {
		out.Delete = cloneDelete(stmt.Delete)
	}
	return out
}

func cloneSelect(sel *SelectStmt) *SelectStmt {
	if sel == nil {
		return nil
	}
	var fields []string
	if sel.Fields != nil {
		fields = append([]string(nil), sel.Fields...)
	}
	var orderBy []OrderByClause
	if sel.OrderBy != nil {
		orderBy = append([]OrderByClause(nil), sel.OrderBy...)
	}
	out := &SelectStmt{
		Fields:  fields,
		Where:   cloneCondition(sel.Where),
		OrderBy: orderBy,
	}
	if sel.Pipe != nil {
		out.Pipe = &PipeAction{}
		if sel.Pipe.Run != nil {
			out.Pipe.Run = &RunAction{Command: cloneExpr(sel.Pipe.Run.Command)}
		}
		if sel.Pipe.Clipboard != nil {
			out.Pipe.Clipboard = &ClipboardAction{}
		}
	}
	return out
}

func cloneCreate(cr *CreateStmt) *CreateStmt {
	if cr == nil {
		return nil
	}
	out := &CreateStmt{Assignments: make([]Assignment, len(cr.Assignments))}
	for i, a := range cr.Assignments {
		out.Assignments[i] = cloneAssignment(a)
	}
	return out
}

func cloneUpdate(up *UpdateStmt) *UpdateStmt {
	if up == nil {
		return nil
	}
	out := &UpdateStmt{
		Where: cloneCondition(up.Where),
		Set:   make([]Assignment, len(up.Set)),
	}
	for i, a := range up.Set {
		out.Set[i] = cloneAssignment(a)
	}
	return out
}

func cloneDelete(del *DeleteStmt) *DeleteStmt {
	if del == nil {
		return nil
	}
	return &DeleteStmt{Where: cloneCondition(del.Where)}
}

func cloneTrigger(trig *Trigger) *Trigger {
	if trig == nil {
		return nil
	}
	out := &Trigger{
		Timing: trig.Timing,
		Event:  trig.Event,
		Where:  cloneCondition(trig.Where),
		Action: cloneStatement(trig.Action),
	}
	if trig.Run != nil {
		out.Run = &RunAction{Command: cloneExpr(trig.Run.Command)}
	}
	if trig.Deny != nil {
		s := *trig.Deny
		out.Deny = &s
	}
	return out
}

func cloneTimeTrigger(tt *TimeTrigger) *TimeTrigger {
	if tt == nil {
		return nil
	}
	return &TimeTrigger{
		Interval: tt.Interval,
		Action:   cloneStatement(tt.Action),
	}
}

func cloneAssignment(a Assignment) Assignment {
	return Assignment{
		Field: a.Field,
		Value: cloneExpr(a.Value),
	}
}

func cloneCondition(cond Condition) Condition {
	if cond == nil {
		return nil
	}
	switch c := cond.(type) {
	case *BinaryCondition:
		return &BinaryCondition{
			Op:    c.Op,
			Left:  cloneCondition(c.Left),
			Right: cloneCondition(c.Right),
		}
	case *NotCondition:
		return &NotCondition{Inner: cloneCondition(c.Inner)}
	case *CompareExpr:
		return &CompareExpr{
			Left:  cloneExpr(c.Left),
			Op:    c.Op,
			Right: cloneExpr(c.Right),
		}
	case *IsEmptyExpr:
		return &IsEmptyExpr{
			Expr:    cloneExpr(c.Expr),
			Negated: c.Negated,
		}
	case *InExpr:
		return &InExpr{
			Value:      cloneExpr(c.Value),
			Collection: cloneExpr(c.Collection),
			Negated:    c.Negated,
		}
	case *QuantifierExpr:
		return &QuantifierExpr{
			Expr:      cloneExpr(c.Expr),
			Kind:      c.Kind,
			Condition: cloneCondition(c.Condition),
		}
	default:
		return nil
	}
}

func cloneExpr(expr Expr) Expr {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *FieldRef:
		return &FieldRef{Name: e.Name}
	case *QualifiedRef:
		return &QualifiedRef{Qualifier: e.Qualifier, Name: e.Name}
	case *StringLiteral:
		return &StringLiteral{Value: e.Value}
	case *IntLiteral:
		return &IntLiteral{Value: e.Value}
	case *DateLiteral:
		return &DateLiteral{Value: e.Value}
	case *DurationLiteral:
		return &DurationLiteral{Value: e.Value, Unit: e.Unit}
	case *ListLiteral:
		elems := make([]Expr, len(e.Elements))
		for i, elem := range e.Elements {
			elems[i] = cloneExpr(elem)
		}
		return &ListLiteral{Elements: elems}
	case *EmptyLiteral:
		return &EmptyLiteral{}
	case *FunctionCall:
		args := make([]Expr, len(e.Args))
		for i, arg := range e.Args {
			args[i] = cloneExpr(arg)
		}
		return &FunctionCall{Name: e.Name, Args: args}
	case *BinaryExpr:
		return &BinaryExpr{
			Op:    e.Op,
			Left:  cloneExpr(e.Left),
			Right: cloneExpr(e.Right),
		}
	case *SubQuery:
		return &SubQuery{Where: cloneCondition(e.Where)}
	default:
		return nil
	}
}
