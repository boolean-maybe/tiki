package ruki

import (
	"fmt"
	"sort"

	"github.com/boolean-maybe/tiki/task"
)

// interactiveBuiltins identifies builtins requiring user interaction (UI prompt).
// Adding a new interactive builtin here automatically blocks it in triggers.
var interactiveBuiltins = map[string]bool{
	"input":  true,
	"choose": true,
}

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
	seal                 *validationSeal
	runtime              ExecutorRuntimeMode
	usesIDFunc           bool
	usesIDsFunc          bool
	usesSelectedCountFn  bool
	usesInputFunc        bool
	usesChooseFunc       bool
	usesTargetQualifier  bool
	usesTargetsQualifier bool
	chooseFilter         *SubQuery
	interactiveCalls     map[string]int
	statement            *Statement
}

func (v *ValidatedStatement) RuntimeMode() ExecutorRuntimeMode { return v.runtime }
func (v *ValidatedStatement) UsesIDBuiltin() bool              { return v.usesIDFunc }
func (v *ValidatedStatement) UsesIDsBuiltin() bool             { return v.usesIDsFunc }
func (v *ValidatedStatement) UsesSelectedCountBuiltin() bool   { return v.usesSelectedCountFn }
func (v *ValidatedStatement) UsesInputBuiltin() bool           { return v.usesInputFunc }
func (v *ValidatedStatement) UsesChooseBuiltin() bool          { return v.usesChooseFunc }
func (v *ValidatedStatement) UsesTargetQualifier() bool        { return v.usesTargetQualifier }
func (v *ValidatedStatement) UsesTargetsQualifier() bool       { return v.usesTargetsQualifier }
func (v *ValidatedStatement) ChooseFilter() *SubQuery          { return v.chooseFilter }

// HasAnyInteractive returns true if the statement uses any interactive builtin.
// Backed by the interactiveCalls map so future builtins added to the
// interactiveBuiltins set are automatically covered.
func (v *ValidatedStatement) HasAnyInteractive() bool {
	for _, count := range v.interactiveCalls {
		if count > 0 {
			return true
		}
	}
	return false
}
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
func (v *ValidatedStatement) IsExpr() bool {
	return v != nil && v.statement != nil && v.statement.Expr != nil
}

// ExprStatement returns the underlying top-level expression statement, or nil
// if the validated statement is not an expression statement.
func (v *ValidatedStatement) ExprStatement() *ExprStmt {
	if !v.IsExpr() {
		return nil
	}
	return v.statement.Expr
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

// ParseAndValidateStatementWithInput parses a statement with an input() type
// declaration and applies runtime-aware semantic validation. The inputType is
// set on the parser for the duration of the parse so that inferExprType can
// resolve input() calls.
func (p *Parser) ParseAndValidateStatementWithInput(input string, runtime ExecutorRuntimeMode, inputType ValueType) (*ValidatedStatement, error) {
	p.inputType = &inputType
	defer func() { p.inputType = nil }()

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
	flags, err := scanStatementSemanticsEx(stmt)
	if err != nil {
		return nil, err
	}
	if flags.hasCall {
		return nil, fmt.Errorf("call() is not supported yet")
	}
	if flags.usesID && v.runtime != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("id() is only available in plugin runtime")
	}
	if flags.usesIDs && v.runtime != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("ids() is only available in plugin runtime")
	}
	if flags.usesSelectedCount && v.runtime != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("selected_count() is only available in plugin runtime")
	}
	if flags.usesTarget && v.runtime != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("target. qualifier is only available in plugin runtime")
	}
	if flags.usesTargets && v.runtime != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("targets. qualifier is only available in plugin runtime")
	}
	inputCount := flags.interactiveCalls["input"]
	chooseCount := flags.interactiveCalls["choose"]
	if inputCount > 1 {
		return nil, fmt.Errorf("input() may only be used once per action")
	}
	if chooseCount > 1 {
		return nil, fmt.Errorf("choose() may only be used once per action")
	}
	if inputCount > 0 && chooseCount > 0 {
		return nil, fmt.Errorf("input() and choose() cannot be used in the same action")
	}
	if flags.hasAnyInteractive() && v.runtime != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("%s() requires user interaction and is only valid in plugin actions",
			flags.interactiveNames()[0])
	}
	if err := validateStatementAssignmentsSemantics(stmt); err != nil {
		return nil, err
	}

	var chooseFilter *SubQuery
	if chooseCount == 1 {
		chooseFilter = extractChooseSubQuery(stmt)
	}

	return &ValidatedStatement{
		seal:                 validatedSeal,
		runtime:              v.runtime,
		usesIDFunc:           flags.usesID,
		usesIDsFunc:          flags.usesIDs,
		usesSelectedCountFn:  flags.usesSelectedCount,
		usesInputFunc:        inputCount == 1,
		usesChooseFunc:       chooseCount == 1,
		usesTargetQualifier:  flags.usesTarget,
		usesTargetsQualifier: flags.usesTargets,
		chooseFilter:         chooseFilter,
		interactiveCalls:     flags.interactiveCalls,
		statement:            cloneStatement(stmt),
	}, nil
}

// ValidateTrigger applies runtime-aware semantic checks to a parsed event trigger.
func (v *SemanticValidator) ValidateTrigger(trig *Trigger) (*ValidatedTrigger, error) {
	if trig == nil {
		return nil, fmt.Errorf("nil trigger")
	}
	if _, _, err := scanTriggerSemantics(trig); err != nil {
		return nil, err
	}
	flags := scanTriggerSemanticsEx(trig)
	if flags.hasCall {
		return nil, fmt.Errorf("call() is not supported yet")
	}
	if flags.usesID && v.runtime != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("id() is only available in plugin runtime")
	}
	if flags.usesIDs {
		return nil, fmt.Errorf("ids() is not valid in triggers")
	}
	if flags.usesSelectedCount {
		return nil, fmt.Errorf("selected_count() is not valid in triggers")
	}
	if flags.usesTarget {
		return nil, fmt.Errorf("target. qualifier is not valid in triggers")
	}
	if flags.usesTargets {
		return nil, fmt.Errorf("targets. qualifier is not valid in triggers")
	}
	if flags.hasAnyInteractive() {
		return nil, fmt.Errorf("%s() requires user interaction and is not valid in triggers",
			flags.interactiveNames()[0])
	}
	if trig.Action != nil {
		if err := validateStatementAssignmentsSemantics(trig.Action); err != nil {
			return nil, err
		}
	}
	return &ValidatedTrigger{
		seal:       validatedSeal,
		runtime:    v.runtime,
		usesIDFunc: flags.usesID,
		trigger:    cloneTrigger(trig),
	}, nil
}

// ValidateTimeTrigger applies runtime-aware semantic checks to a parsed time trigger.
func (v *SemanticValidator) ValidateTimeTrigger(tt *TimeTrigger) (*ValidatedTimeTrigger, error) {
	if tt == nil {
		return nil, fmt.Errorf("nil time trigger")
	}
	if _, _, err := scanTimeTriggerSemantics(tt); err != nil {
		return nil, err
	}
	flags := scanTimeTriggerSemanticsEx(tt)
	if flags.hasCall {
		return nil, fmt.Errorf("call() is not supported yet")
	}
	if flags.usesID && v.runtime != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("id() is only available in plugin runtime")
	}
	if flags.usesIDs {
		return nil, fmt.Errorf("ids() is not valid in triggers")
	}
	if flags.usesSelectedCount {
		return nil, fmt.Errorf("selected_count() is not valid in triggers")
	}
	if flags.usesTarget {
		return nil, fmt.Errorf("target. qualifier is not valid in triggers")
	}
	if flags.usesTargets {
		return nil, fmt.Errorf("targets. qualifier is not valid in triggers")
	}
	if flags.hasAnyInteractive() {
		return nil, fmt.Errorf("%s() requires user interaction and is not valid in triggers",
			flags.interactiveNames()[0])
	}
	if tt.Action != nil {
		if err := validateStatementAssignmentsSemantics(tt.Action); err != nil {
			return nil, err
		}
	}
	return &ValidatedTimeTrigger{
		seal:        validatedSeal,
		runtime:     v.runtime,
		usesIDFunc:  flags.usesID,
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
	case stmt.Expr != nil:
		return scanExprSemantics(stmt.Expr.Expr)
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

func scanTriggerSemanticsEx(trig *Trigger) semanticFlags {
	var f semanticFlags
	if trig.Where != nil {
		wf, _ := scanConditionSemanticsEx(trig.Where)
		f.merge(wf)
	}
	if trig.Action != nil {
		af, _ := scanStatementSemanticsEx(trig.Action)
		f.merge(af)
	}
	if trig.Run != nil {
		rf, _ := scanExprSemanticsEx(trig.Run.Command)
		f.merge(rf)
	}
	return f
}

func scanTimeTriggerSemanticsEx(tt *TimeTrigger) semanticFlags {
	var f semanticFlags
	if tt == nil || tt.Action == nil {
		return f
	}
	af, _ := scanStatementSemanticsEx(tt.Action)
	f.merge(af)
	return f
}

func scanStatementSemanticsEx(stmt *Statement) (semanticFlags, error) {
	// countInteractiveUsage walks every Expr node in the statement and
	// accumulates full semanticFlags (interactive counts plus usesID /
	// usesIDs / usesSelectedCount / hasCall). The structural scan below
	// (scanStatementSemantics) remains as the authoritative
	// "empty statement" gate — a zero-variant Statement{} must error here,
	// not silently pass.
	flags, err := countInteractiveUsage(stmt)
	if err != nil {
		return flags, err
	}
	if _, _, err := scanStatementSemantics(stmt); err != nil {
		return flags, err
	}
	return flags, nil
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
	case *BoolExprCondition:
		return scanExprSemantics(c.Expr)
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

type semanticFlags struct {
	usesID            bool
	usesIDs           bool
	usesSelectedCount bool
	hasCall           bool
	usesTarget        bool
	usesTargets       bool
	interactiveCalls  map[string]int
}

func (f *semanticFlags) merge(other semanticFlags) {
	f.usesID = f.usesID || other.usesID
	f.usesIDs = f.usesIDs || other.usesIDs
	f.usesSelectedCount = f.usesSelectedCount || other.usesSelectedCount
	f.hasCall = f.hasCall || other.hasCall
	f.usesTarget = f.usesTarget || other.usesTarget
	f.usesTargets = f.usesTargets || other.usesTargets
	for name, count := range other.interactiveCalls {
		if f.interactiveCalls == nil {
			f.interactiveCalls = map[string]int{}
		}
		f.interactiveCalls[name] += count
	}
}

func (f *semanticFlags) totalInteractive() int {
	n := 0
	for _, count := range f.interactiveCalls {
		n += count
	}
	return n
}

func (f *semanticFlags) hasAnyInteractive() bool {
	return f.totalInteractive() > 0
}

func (f *semanticFlags) interactiveNames() []string {
	var names []string
	for name := range f.interactiveCalls {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func scanExprSemantics(expr Expr) (usesID bool, hasCall bool, err error) {
	flags, err := scanExprSemanticsEx(expr)
	if err != nil {
		return false, false, err
	}
	return flags.usesID, flags.hasCall, nil
}

func scanExprSemanticsEx(expr Expr) (semanticFlags, error) {
	var f semanticFlags
	if expr == nil {
		return f, nil
	}
	switch e := expr.(type) {
	case *QualifiedRef:
		switch e.Qualifier {
		case "target":
			f.usesTarget = true
		case "targets":
			f.usesTargets = true
		}
		return f, nil
	case *FunctionCall:
		switch e.Name {
		case "id":
			f.usesID = true
		case "ids":
			f.usesIDs = true
		case "selected_count":
			f.usesSelectedCount = true
		case "call":
			f.hasCall = true
		}
		if interactiveBuiltins[e.Name] {
			if f.interactiveCalls == nil {
				f.interactiveCalls = map[string]int{}
			}
			f.interactiveCalls[e.Name]++
		}
		for _, arg := range e.Args {
			af, err := scanExprSemanticsEx(arg)
			if err != nil {
				return f, err
			}
			f.merge(af)
		}
		return f, nil
	case *BinaryExpr:
		lf, err := scanExprSemanticsEx(e.Left)
		if err != nil {
			return f, err
		}
		rf, err := scanExprSemanticsEx(e.Right)
		if err != nil {
			return f, err
		}
		f.merge(lf)
		f.merge(rf)
		return f, nil
	case *ListLiteral:
		for _, elem := range e.Elements {
			ef, err := scanExprSemanticsEx(elem)
			if err != nil {
				return f, err
			}
			f.merge(ef)
		}
		return f, nil
	case *SubQuery:
		sf, err := scanConditionSemanticsEx(e.Where)
		if err != nil {
			return f, err
		}
		f.merge(sf)
		return f, nil
	default:
		return f, nil
	}
}

func scanConditionSemanticsEx(cond Condition) (semanticFlags, error) {
	var f semanticFlags
	if cond == nil {
		return f, nil
	}
	switch c := cond.(type) {
	case *BinaryCondition:
		lf, err := scanConditionSemanticsEx(c.Left)
		if err != nil {
			return f, err
		}
		rf, err := scanConditionSemanticsEx(c.Right)
		if err != nil {
			return f, err
		}
		f.merge(lf)
		f.merge(rf)
		return f, nil
	case *NotCondition:
		return scanConditionSemanticsEx(c.Inner)
	case *BoolExprCondition:
		return scanExprSemanticsEx(c.Expr)
	case *CompareExpr:
		lf, err := scanExprSemanticsEx(c.Left)
		if err != nil {
			return f, err
		}
		rf, err := scanExprSemanticsEx(c.Right)
		if err != nil {
			return f, err
		}
		f.merge(lf)
		f.merge(rf)
		return f, nil
	case *IsEmptyExpr:
		return scanExprSemanticsEx(c.Expr)
	case *InExpr:
		vf, err := scanExprSemanticsEx(c.Value)
		if err != nil {
			return f, err
		}
		cf, err := scanExprSemanticsEx(c.Collection)
		if err != nil {
			return f, err
		}
		f.merge(vf)
		f.merge(cf)
		return f, nil
	case *QuantifierExpr:
		ef, err := scanExprSemanticsEx(c.Expr)
		if err != nil {
			return f, err
		}
		cf, err := scanConditionSemanticsEx(c.Condition)
		if err != nil {
			return f, err
		}
		f.merge(ef)
		f.merge(cf)
		return f, nil
	default:
		return f, fmt.Errorf("unknown condition type %T", c)
	}
}

func countInteractiveUsage(stmt *Statement) (semanticFlags, error) {
	var total semanticFlags
	switch {
	case stmt.Select != nil:
		if stmt.Select.Where != nil {
			f, err := scanConditionSemanticsEx(stmt.Select.Where)
			if err != nil {
				return total, err
			}
			total.merge(f)
		}
		if stmt.Select.Pipe != nil && stmt.Select.Pipe.Run != nil {
			f, err := scanExprSemanticsEx(stmt.Select.Pipe.Run.Command)
			if err != nil {
				return total, err
			}
			total.merge(f)
		}
	case stmt.Create != nil:
		for _, a := range stmt.Create.Assignments {
			f, err := scanExprSemanticsEx(a.Value)
			if err != nil {
				return total, err
			}
			total.merge(f)
		}
	case stmt.Update != nil:
		if stmt.Update.Where != nil {
			f, err := scanConditionSemanticsEx(stmt.Update.Where)
			if err != nil {
				return total, err
			}
			total.merge(f)
		}
		for _, a := range stmt.Update.Set {
			f, err := scanExprSemanticsEx(a.Value)
			if err != nil {
				return total, err
			}
			total.merge(f)
		}
	case stmt.Delete != nil:
		if stmt.Delete.Where != nil {
			f, err := scanConditionSemanticsEx(stmt.Delete.Where)
			if err != nil {
				return total, err
			}
			total.merge(f)
		}
	case stmt.Expr != nil:
		f, err := scanExprSemanticsEx(stmt.Expr.Expr)
		if err != nil {
			return total, err
		}
		total.merge(f)
	}
	return total, nil
}

// extractChooseSubQuery walks the statement AST and returns the SubQuery
// from the first choose() call found. Returns nil if not found.
func extractChooseSubQuery(stmt *Statement) *SubQuery {
	var found *SubQuery
	walkExprs(stmt, func(e Expr) bool {
		fc, ok := e.(*FunctionCall)
		if !ok || fc.Name != "choose" || len(fc.Args) == 0 {
			return true
		}
		if sq, ok := fc.Args[0].(*SubQuery); ok {
			found = sq
			return false
		}
		return true
	})
	return found
}

// walkExprs visits every Expr node in a statement. The visitor returns false to stop.
func walkExprs(stmt *Statement, visit func(Expr) bool) {
	switch {
	case stmt.Create != nil:
		for _, a := range stmt.Create.Assignments {
			if !walkExpr(a.Value, visit) {
				return
			}
		}
	case stmt.Update != nil:
		if stmt.Update.Where != nil {
			if !walkConditionExprs(stmt.Update.Where, visit) {
				return
			}
		}
		for _, a := range stmt.Update.Set {
			if !walkExpr(a.Value, visit) {
				return
			}
		}
	case stmt.Select != nil:
		if stmt.Select.Where != nil {
			if !walkConditionExprs(stmt.Select.Where, visit) {
				return
			}
		}
	case stmt.Delete != nil:
		if stmt.Delete.Where != nil {
			walkConditionExprs(stmt.Delete.Where, visit)
		}
	case stmt.Expr != nil:
		walkExpr(stmt.Expr.Expr, visit)
	}
}

func walkExpr(e Expr, visit func(Expr) bool) bool {
	if e == nil {
		return true
	}
	if !visit(e) {
		return false
	}
	switch e := e.(type) {
	case *BinaryExpr:
		if !walkExpr(e.Left, visit) {
			return false
		}
		return walkExpr(e.Right, visit)
	case *FunctionCall:
		for _, arg := range e.Args {
			if !walkExpr(arg, visit) {
				return false
			}
		}
	case *ListLiteral:
		for _, elem := range e.Elements {
			if !walkExpr(elem, visit) {
				return false
			}
		}
	case *SubQuery:
		if e.Where != nil {
			return walkConditionExprs(e.Where, visit)
		}
	}
	return true
}

func walkConditionExprs(c Condition, visit func(Expr) bool) bool {
	if c == nil {
		return true
	}
	switch c := c.(type) {
	case *BinaryCondition:
		if !walkConditionExprs(c.Left, visit) {
			return false
		}
		return walkConditionExprs(c.Right, visit)
	case *NotCondition:
		return walkConditionExprs(c.Inner, visit)
	case *BoolExprCondition:
		return walkExpr(c.Expr, visit)
	case *CompareExpr:
		if !walkExpr(c.Left, visit) {
			return false
		}
		return walkExpr(c.Right, visit)
	case *IsEmptyExpr:
		return walkExpr(c.Expr, visit)
	case *InExpr:
		if !walkExpr(c.Value, visit) {
			return false
		}
		return walkExpr(c.Collection, visit)
	case *QuantifierExpr:
		if !walkExpr(c.Expr, visit) {
			return false
		}
		return walkConditionExprs(c.Condition, visit)
	}
	return true
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
	if stmt.Expr != nil {
		out.Expr = cloneExprStmt(stmt.Expr)
	}
	return out
}

func cloneExprStmt(es *ExprStmt) *ExprStmt {
	if es == nil {
		return nil
	}
	return &ExprStmt{Expr: cloneExpr(es.Expr), Type: es.Type}
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
	if sel.Limit != nil {
		v := *sel.Limit
		out.Limit = &v
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
	case *BoolExprCondition:
		return &BoolExprCondition{Expr: cloneExpr(c.Expr)}
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
	case *BoolLiteral:
		return &BoolLiteral{Value: e.Value}
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
