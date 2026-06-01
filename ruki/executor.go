package ruki

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	collectionutil "github.com/boolean-maybe/tiki/ruki/collections"
	"github.com/boolean-maybe/tiki/ruki/duration"
	"github.com/boolean-maybe/tiki/ruki/idfmt"
	"github.com/boolean-maybe/tiki/ruki/recurrence"
)

// Executor evaluates parsed ruki statements against a set of tikis.
type Executor struct {
	schema       Schema
	factory      DocumentFactory
	userFunc     func() string
	runtime      ExecutorRuntime
	currentInput ExecutionInput
}

type evalContext struct {
	current  Document
	outer    Document
	allTikis []Document
	// inAssignmentRHS is true while the executor is evaluating the RHS
	// of an `update set <field> = <expr>` or `create <field> = <expr>`
	// assignment. It enables a narrow carve-out to absent-field hard-
	// error semantics: bare or qualified references to workflow-declared
	// fields that are absent on the target tiki auto-zero during `+`/`-`
	// arithmetic and plain reference reads, so idioms like `set tags = tags + [x]`,
	// `set priority = priority - 1`, and `create tags = old.tags`
	// work on docs that lack the field. Unregistered names still
	// hard-error so typos like `set taggs = taggs + [x]` fail loudly.
	// Scoped tightly: not active in WHERE, ORDER BY, subquery filters,
	// or bare reads outside an assignment RHS.
	inAssignmentRHS bool
}

func (ctx evalContext) withCurrent(current Document) evalContext {
	ctx.current = current
	return ctx
}

// NewExecutor constructs an Executor with the given schema, document factory,
// and user function. The factory builds the blank Document a create statement
// fills in; it may be nil for executors that never run a template-less create.
// If userFunc is nil, calling user() at runtime will return an error.
func NewExecutor(schema Schema, factory DocumentFactory, userFunc func() string, runtime ExecutorRuntime) *Executor {
	return &Executor{
		schema:   schema,
		factory:  factory,
		userFunc: userFunc,
		runtime:  runtime.normalize(),
	}
}

// Result holds the output of executing a statement.
// Exactly one variant is non-nil.
type Result struct {
	Select    *TikiProjection
	Update    *UpdateResult
	Create    *CreateResult
	Delete    *DeleteResult
	Pipe      *PipeResult
	Clipboard *ClipboardResult
	Scalar    *ScalarResult
}

// ScalarResult holds a single value produced by a top-level expression
// statement, along with its inferred type so runtime formatters can
// distinguish dates from timestamps, etc.
type ScalarResult struct {
	Value interface{}
	Type  ValueType
}

// ClipboardResult holds the row data from a clipboard-piped select.
// The service layer writes these to the system clipboard.
type ClipboardResult struct {
	Rows [][]string
}

// PipeResult holds the shell command and per-row positional args from a piped select.
// The ruki executor builds this; the service layer performs the actual shell execution.
type PipeResult struct {
	Command string
	Rows    [][]string
}

// UpdateResult holds the cloned, mutated tikis produced by an UPDATE statement.
type UpdateResult struct {
	Updated []Document
}

// CreateResult holds the new tiki produced by a CREATE statement.
type CreateResult struct {
	Tiki Document
}

// DeleteResult holds the tikis matched by a DELETE statement's WHERE clause.
type DeleteResult struct {
	Deleted []Document
}

// TikiProjection holds the filtered, sorted tikis and the requested field list.
type TikiProjection struct {
	Tikis  []Document
	Fields []string // user-requested fields; nil/empty = all fields
}

// Execute dispatches on the statement type and returns results.
// Preferred input is *ValidatedStatement; raw *Statement is accepted as a
// low-level path and will be semantically validated for executor runtime mode.
func (e *Executor) Execute(stmt any, tikis []Document, inputs ...ExecutionInput) (*Result, error) {
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
		if e.runtime.Mode == ExecutorRuntimePlugin &&
			(validated.usesIDFunc || validated.usesFilepathFunc || validated.usesTargetQualifier) {
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
	case rawStmt.Select != nil:
		return e.executeSelect(rawStmt.Select, tikis)
	case rawStmt.Create != nil:
		return e.executeCreate(rawStmt.Create, tikis, requiresCreateTemplate)
	case rawStmt.Update != nil:
		return e.executeUpdate(rawStmt.Update, tikis)
	case rawStmt.Delete != nil:
		return e.executeDelete(rawStmt.Delete, tikis)
	case rawStmt.Expr != nil:
		return e.executeExpr(rawStmt.Expr, tikis)
	default:
		return nil, fmt.Errorf("empty statement")
	}
}

func (e *Executor) executeExpr(es *ExprStmt, tikis []Document) (*Result, error) {
	val, err := e.evalExpr(es.Expr, evalContext{allTikis: tikis})
	if err != nil {
		return nil, err
	}
	return &Result{Scalar: &ScalarResult{Value: val, Type: es.Type}}, nil
}

func (e *Executor) executeSelect(sel *SelectStmt, tikis []Document) (*Result, error) {
	filtered, err := e.filterTikis(sel.Where, tikis)
	if err != nil {
		return nil, err
	}

	if len(sel.OrderBy) > 0 {
		if err := e.sortTikis(filtered, sel.OrderBy); err != nil {
			return nil, err
		}
	}

	if sel.Limit != nil && *sel.Limit < len(filtered) {
		filtered = filtered[:*sel.Limit]
	}

	if sel.Pipe != nil {
		switch {
		case sel.Pipe.Run != nil:
			return e.buildPipeResult(sel.Pipe.Run, sel.Fields, filtered, tikis)
		case sel.Pipe.Clipboard != nil:
			return e.buildClipboardResult(sel.Fields, filtered)
		}
	}

	return &Result{
		Select: &TikiProjection{
			Tikis:  filtered,
			Fields: sel.Fields,
		},
	}, nil
}

func (e *Executor) buildPipeResult(pipe *RunAction, fields []string, matched []Document, allTikis []Document) (*Result, error) {
	// evaluate command once with a nil-sentinel tiki — validation ensures no field refs
	cmdVal, err := e.evalExpr(pipe.Command, evalContext{allTikis: allTikis})
	if err != nil {
		return nil, fmt.Errorf("pipe command: %w", err)
	}
	cmdStr, ok := cmdVal.(string)
	if !ok {
		return nil, fmt.Errorf("pipe command must evaluate to string, got %T", cmdVal)
	}

	rows, err := e.buildFieldRows(fields, matched)
	if err != nil {
		return nil, err
	}
	return &Result{Pipe: &PipeResult{Command: cmdStr, Rows: rows}}, nil
}

func (e *Executor) buildClipboardResult(fields []string, matched []Document) (*Result, error) {
	rows, err := e.buildFieldRows(fields, matched)
	if err != nil {
		return nil, err
	}
	return &Result{Clipboard: &ClipboardResult{Rows: rows}}, nil
}

// buildFieldRows extracts the requested fields from matched tikis as string rows.
// Absent-field reads produce empty cells rather than propagating the hard
// error: pipe and clipboard sinks are presentation-layer consumers and a
// missing field should render blank, not abort the whole operation.
func (e *Executor) buildFieldRows(fields []string, matched []Document) ([][]string, error) {
	rows := make([][]string, len(matched))
	for i, t := range matched {
		row := make([]string, len(fields))
		for j, f := range fields {
			val, err := e.extractFieldForDisplay(t, f)
			if err != nil {
				return nil, err
			}
			row[j] = pipeArgString(val)
		}
		rows[i] = row
	}
	return rows, nil
}

// pipeArgString space-joins list fields (tags, dependsOn) instead of using Go's
// default fmt.Sprint which produces "[a b c]" with brackets.
func pipeArgString(val interface{}) string {
	if list, ok := val.([]interface{}); ok {
		parts := make([]string, len(list))
		for i, elem := range list {
			parts[i] = normalizeToString(elem)
		}
		return strings.Join(parts, " ")
	}
	return normalizeToString(val)
}

func (e *Executor) executeUpdate(upd *UpdateStmt, tikis []Document) (*Result, error) {
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

func (e *Executor) executeCreate(cr *CreateStmt, tikis []Document, requireTemplate bool) (*Result, error) {
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

func (e *Executor) executeDelete(del *DeleteStmt, tikis []Document) (*Result, error) {
	matched, err := e.filterTikis(del.Where, tikis)
	if err != nil {
		return nil, err
	}

	return &Result{Delete: &DeleteResult{Deleted: matched}}, nil
}

func (e *Executor) setField(t Document, name string, val interface{}) error {
	// Identity/audit fields live on the Tiki struct, not the Fields map.
	switch name {
	case "id", "createdBy", "createdAt", "updatedAt", "filepath", "path":
		return fmt.Errorf("field %q is immutable", name)

	case "title":
		if val == nil {
			return fmt.Errorf("cannot set title to empty")
		}
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("title must be a string, got %T", val)
		}
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("cannot set title to empty")
		}
		t.SetTitle(s)
		return nil

	case "description", "body":
		if val == nil {
			t.SetBody("")
			return nil
		}
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("description must be a string, got %T", val)
		}
		t.SetBody(s)
		return nil
	}

	// Workflow-declared fields: dispatch through the schema by type.
	fs, ok := e.schema.Field(name)
	if !ok {
		return fmt.Errorf("unknown field %q", name)
	}
	if val == nil {
		t.Delete(name)
		return nil
	}
	coerced, err := coerceCustomFieldValue(fs, val)
	if err != nil {
		return fmt.Errorf("field %q: %w", name, err)
	}
	t.Set(name, coerced)
	return nil
}

// coerceSetString converts a recurrence.Recurrence wrapper or plain string to a
// string before enum canonicalization.
func coerceSetString(val interface{}) (string, bool) {
	switch v := val.(type) {
	case string:
		return v, true
	case recurrence.Recurrence:
		return string(v), true
	default:
		return "", false
	}
}

func toStringSlice(val interface{}) []string {
	switch list := val.(type) {
	case []interface{}:
		result := make([]string, len(list))
		for i, elem := range list {
			result[i] = normalizeToString(elem)
		}
		return result
	case []string:
		result := make([]string, len(list))
		copy(result, list)
		return result
	default:
		return nil
	}
}

func coerceCustomFieldValue(fs FieldSpec, val interface{}) (interface{}, error) {
	switch fs.Type {
	case ValueString:
		s, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", val)
		}
		return s, nil
	case ValueInt:
		n, ok := val.(int)
		if !ok {
			return nil, fmt.Errorf("expected int, got %T", val)
		}
		return n, nil
	case ValueBool:
		if b, ok := val.(bool); ok {
			return b, nil
		}
		if s, ok := val.(string); ok {
			if b, err := parseBoolString(s); err == nil {
				return b, nil
			}
		}
		return nil, fmt.Errorf("expected bool, got %T", val)
	case ValueTimestamp, ValueDate:
		tv, ok := val.(time.Time)
		if !ok {
			return nil, fmt.Errorf("expected time.Time, got %T", val)
		}
		return tv, nil
	case ValueDuration:
		// duration values are stored as their string form in Fields; ruki
		// arithmetic re-parses on demand.
		if s, ok := val.(string); ok {
			return s, nil
		}
		return nil, fmt.Errorf("expected duration string, got %T", val)
	case ValueRecurrence:
		s, ok := coerceSetString(val)
		if !ok {
			return nil, fmt.Errorf("expected recurrence string, got %T", val)
		}
		return s, nil
	case ValueEnum:
		return coerceEnumFieldValue(fs, val)
	case ValueListString:
		return collectionutil.NormalizeStringSet(toStringSlice(val)), nil
	case ValueListRef:
		refs := normalizeRefList(toStringSlice(val))
		if err := validateBareRefs(refs, fs.Name); err != nil {
			return nil, err
		}
		return refs, nil
	case ValueRef, ValueID:
		s, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", val)
		}
		ref := strings.ToUpper(strings.TrimSpace(s))
		if !idfmt.IsValidID(ref) {
			return nil, fmt.Errorf("%s reference %q is not a bare document id (expected %d uppercase alphanumeric chars)", fs.Name, ref, idfmt.IDLength)
		}
		return ref, nil
	default:
		return nil, fmt.Errorf("unsupported custom field type")
	}
}

func coerceEnumFieldValue(fs FieldSpec, val interface{}) (string, error) {
	s, ok := coerceSetString(val)
	if !ok {
		return "", fmt.Errorf("expected string, got %T", val)
	}
	canonical, ok := canonicalEnumValue(fs, s)
	if !ok {
		return "", fmt.Errorf("invalid enum value %q", s)
	}
	return canonical, nil
}

func canonicalEnumValue(fs FieldSpec, raw string) (string, bool) {
	for _, av := range fs.AllowedValues {
		if strings.EqualFold(av, raw) {
			return av, true
		}
	}
	return "", false
}

// normalizeRefList applies set-like normalization for document ID references.
func normalizeRefList(ss []string) []string {
	return collectionutil.NormalizeRefSet(ss)
}

// validateBareRefs rejects any entry that is not a bare document ID.
func validateBareRefs(refs []string, fieldName string) error {
	for _, r := range refs {
		if !idfmt.IsValidID(r) {
			return fmt.Errorf("%s reference %q is not a bare document id (expected %d uppercase alphanumeric chars)", fieldName, r, idfmt.IDLength)
		}
	}
	return nil
}

// --- filtering ---

func (e *Executor) filterTikis(where Condition, tikis []Document) ([]Document, error) {
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

// --- condition evaluation ---

func (e *Executor) evalCondition(c Condition, ctx evalContext) (bool, error) {
	switch c := c.(type) {
	case *BinaryCondition:
		return e.evalBinaryCondition(c, ctx)
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
		return e.evalCompare(c, ctx)
	case *IsEmptyExpr:
		return e.evalIsEmpty(c, ctx)
	case *InExpr:
		return e.evalIn(c, ctx)
	case *QuantifierExpr:
		return e.evalQuantifier(c, ctx)
	default:
		return false, fmt.Errorf("unknown condition type %T", c)
	}
}

func conditionBoolValue(val interface{}) (bool, error) {
	b, ok := val.(bool)
	if !ok {
		return false, fmt.Errorf("condition expression must evaluate to bool, got %T", val)
	}
	return b, nil
}

func (e *Executor) evalBinaryCondition(c *BinaryCondition, ctx evalContext) (bool, error) {
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
}

// evalCompare evaluates equality/inequality/ordering comparisons with
// missing-field-aware semantics (Phase 4 updated plan):
//
//   - missing = value → false
//   - missing != value → true
//   - missing = empty / missing != empty → follow is-empty/zero-value path
//     (missing is treated as empty), so missing = empty → true,
//     missing != empty → false
//   - missing <, >, <=, >= → hard-error (no defined ordering for absent)
//
// The rule applies symmetrically to both sides of the operator. Plain
// AbsentFieldError propagation still carries any other errors (e.g. type
// mismatches) so those surface unchanged.
func (e *Executor) evalCompare(c *CompareExpr, ctx evalContext) (bool, error) {
	leftVal, leftAbsent, err := evalComparand(e, c.Left, ctx)
	if err != nil {
		return false, err
	}
	rightVal, rightAbsent, err := evalComparand(e, c.Right, ctx)
	if err != nil {
		return false, err
	}

	// If either side is an absent field reference, apply defined semantics.
	if leftAbsent || rightAbsent {
		return missingFieldCompareResult(c.Op, leftAbsent, rightAbsent, c.Left, c.Right, leftVal, rightVal)
	}

	return e.compareValues(leftVal, rightVal, c.Op, c.Left, c.Right)
}

// evalComparand evaluates an expression as a comparison operand. The
// second return reports whether the expression resolved to an absent
// registered-field read (so missing-field semantics apply). Any non-
// absent error propagates.
//
// Shared helper for comparison-like predicates (=, !=, is empty, in,
// quantifier). Free-function shape so both the base Executor and the
// trigger override can reuse it via their own evalExpr.
func absorbAbsent(evalFn func(Expr, evalContext) (interface{}, error), expr Expr, ctx evalContext) (interface{}, bool, error) {
	v, err := evalFn(expr, ctx)
	if err == nil {
		return v, false, nil
	}
	if _, ok := err.(*AbsentFieldError); ok {
		return nil, true, nil
	}
	return nil, false, err
}

// evalComparand is the base-executor convenience wrapper around
// absorbAbsent. Equivalent to absorbAbsent(e.evalExpr, ...).
func evalComparand(e *Executor, expr Expr, ctx evalContext) (interface{}, bool, error) {
	return absorbAbsent(e.evalExpr, expr, ctx)
}

// missingFieldCompareResult implements the updated Phase-4 rules for
// comparisons involving a missing field on either side.
func missingFieldCompareResult(op string, leftAbsent, rightAbsent bool, leftExpr, rightExpr Expr, leftVal, rightVal interface{}) (bool, error) {
	_, leftIsEmpty := leftExpr.(*EmptyLiteral)
	_, rightIsEmpty := rightExpr.(*EmptyLiteral)

	switch op {
	case "=":
		if leftIsEmpty || rightIsEmpty {
			// missing is treated as empty: absent = empty is true.
			if leftAbsent || rightAbsent {
				return true, nil
			}
			// neither absent, fall through to zero-value compare path.
			return compareWithNil(leftVal, rightVal, op, leftExpr, rightExpr)
		}
		// absent = concrete-value → false
		return false, nil
	case "!=":
		if leftIsEmpty || rightIsEmpty {
			// absent != empty is false (absent IS empty under this rule).
			if leftAbsent || rightAbsent {
				return false, nil
			}
			return compareWithNil(leftVal, rightVal, op, leftExpr, rightExpr)
		}
		// absent != concrete-value → true
		return true, nil
	case "<", ">", "<=", ">=":
		// Ordering on an absent field is undefined; hard-error.
		var name string
		if leftAbsent {
			name = fieldRefName(leftExpr)
		} else {
			name = fieldRefName(rightExpr)
		}
		return false, fmt.Errorf("ordering comparison %q on absent field %q has no defined result; guard with has(%s)", op, name, name)
	default:
		return false, fmt.Errorf("unknown operator %q", op)
	}
}

// fieldRefName extracts the underlying name from a FieldRef or
// QualifiedRef for error messages; returns "" for anything else.
func fieldRefName(expr Expr) string {
	switch ref := expr.(type) {
	case *FieldRef:
		return ref.Name
	case *QualifiedRef:
		return ref.Qualifier + "." + ref.Name
	}
	return ""
}

// evalIsEmpty treats an absent-field read as empty, matching the updated
// Phase-4 rule that `missing is empty` is true and `missing is not empty`
// is false. Non-absent errors still propagate.
func (e *Executor) evalIsEmpty(c *IsEmptyExpr, ctx evalContext) (bool, error) {
	val, absent, err := evalComparand(e, c.Expr, ctx)
	if err != nil {
		return false, err
	}
	empty := absent || isZeroValue(val)
	if c.Negated {
		return !empty, nil
	}
	return empty, nil
}

// evalIn gives missing LHS a defined result: `missing in [...]` is false,
// `missing not in [...]` is true. Parity with = / != semantics keeps the
// absent-field behavior consistent across predicates.
func (e *Executor) evalIn(c *InExpr, ctx evalContext) (bool, error) {
	val, valAbsent, err := evalComparand(e, c.Value, ctx)
	if err != nil {
		return false, err
	}
	collVal, collAbsent, err := evalComparand(e, c.Collection, ctx)
	if err != nil {
		return false, err
	}

	// If either side is an absent field ref, the value is "not a member"
	// by definition: `in` → false, `not in` → true.
	if valAbsent || collAbsent {
		return c.Negated, nil
	}

	// list membership mode
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

	// substring mode
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

// evalQuantifier treats an absent list field as an empty list:
// `missing any cond` → false (no elements to satisfy), `missing all cond`
// → true (vacuous truth). Matches the missing-field symmetry in the
// other predicate operators.
func (e *Executor) evalQuantifier(q *QuantifierExpr, ctx evalContext) (bool, error) {
	listVal, absent, err := evalComparand(e, q.Expr, ctx)
	if err != nil {
		return false, err
	}
	if absent {
		return q.Kind == "all", nil
	}
	refs, ok := listVal.([]interface{})
	if !ok {
		return false, fmt.Errorf("quantifier: expression is not a list")
	}

	// find referenced tikis
	refTikis := make([]Document, 0, len(refs))
	for _, ref := range refs {
		refID := normalizeToString(ref)
		for _, at := range ctx.allTikis {
			if strings.EqualFold(at.ID(), refID) {
				refTikis = append(refTikis, at)
				break
			}
		}
	}

	switch q.Kind {
	case "any":
		for _, rt := range refTikis {
			// Soft-false per subquery-iteration rule: a quantifier body
			// evaluating against an absent field on a referenced tiki
			// does not kill the outer query.
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
			return true, nil // vacuous truth
		}
		for _, rt := range refTikis {
			match, err := e.evalCondition(q.Condition, ctx.withCurrent(rt))
			if err != nil {
				// absent-field on one referenced tiki means "no match"
				// for that tiki → `all` fails.
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

// --- expression evaluation ---

func (e *Executor) evalExpr(expr Expr, ctx evalContext) (interface{}, error) {
	switch expr := expr.(type) {
	case *FieldRef:
		return e.readFieldRefWithCarveOut(ctx.current, expr.Name, ctx)
	case *QualifiedRef:
		switch expr.Qualifier {
		case "outer":
			if ctx.outer == nil {
				return nil, fmt.Errorf("outer.%s is not available outside a subquery", expr.Name)
			}
			return e.readFieldRefWithCarveOut(ctx.outer, expr.Name, ctx)
		case "target":
			return e.evalTargetField(expr.Name, ctx)
		case "targets":
			return e.evalTargetsField(expr.Name, ctx)
		case "old", "new":
			return nil, fmt.Errorf("qualified references (old./new.) are not supported in standalone SELECT")
		default:
			return nil, fmt.Errorf("unknown qualifier %q", expr.Qualifier)
		}
	case *StringLiteral:
		return expr.Value, nil
	case *IntLiteral:
		return expr.Value, nil
	case *BoolLiteral:
		return expr.Value, nil
	case *DateLiteral:
		return expr.Value, nil
	case *DurationLiteral:
		d, err := duration.ToDuration(expr.Value, expr.Unit)
		if err != nil {
			return nil, err
		}
		return d, nil
	case *ListLiteral:
		return e.evalListLiteral(expr, ctx)
	case *EmptyLiteral:
		return nil, nil
	case *FunctionCall:
		return e.evalFunctionCall(expr, ctx)
	case *BinaryExpr:
		return e.evalBinaryExpr(expr, ctx)
	case *SubQuery:
		return nil, fmt.Errorf("subquery is only valid as argument to count(), choose(), or exists()")
	default:
		return nil, fmt.Errorf("unknown expression type %T", expr)
	}
}

func (e *Executor) evalListLiteral(ll *ListLiteral, ctx evalContext) (interface{}, error) {
	result := make([]interface{}, len(ll.Elements))
	for i, elem := range ll.Elements {
		val, err := e.evalExpr(elem, ctx)
		if err != nil {
			return nil, err
		}
		result[i] = val
	}
	return result, nil
}

// --- function evaluation ---

func (e *Executor) evalFunctionCall(fc *FunctionCall, ctx evalContext) (interface{}, error) {
	switch fc.Name {
	case "id":
		return e.evalID()
	case "ids":
		return e.evalIDs()
	case "selected_count":
		return e.evalSelectedCount()
	case "filepath":
		return e.evalFilepath(ctx)
	case "filepaths":
		return e.evalFilepaths(ctx)
	case "now":
		return time.Now(), nil
	case "user":
		if e.userFunc == nil {
			return nil, fmt.Errorf("user() is unavailable (no current user configured)")
		}
		return e.userFunc(), nil
	case "count":
		return e.evalCount(fc, ctx.current, ctx.allTikis)
	case "exists":
		return e.evalExists(fc, ctx.current, ctx.allTikis)
	case "has":
		return e.evalHas(fc, ctx)
	case "next_date":
		return e.evalNextDate(fc, ctx)
	case "next_enum":
		return e.evalEnumStep(fc, ctx, +1)
	case "prev_enum":
		return e.evalEnumStep(fc, ctx, -1)
	case "blocks":
		return e.evalBlocks(fc, ctx)
	case "input":
		return e.evalInput()
	case "choose":
		return e.evalChoose()
	case "call":
		return nil, fmt.Errorf("call() is not supported yet")
	default:
		return nil, fmt.Errorf("unknown function %q", fc.Name)
	}
}

func (e *Executor) evalInput() (interface{}, error) {
	if !e.currentInput.HasInput {
		return nil, &MissingInputValueError{}
	}
	return e.currentInput.InputValue, nil
}

func (e *Executor) evalChoose() (interface{}, error) {
	if !e.currentInput.HasChoose {
		return nil, &MissingChooseValueError{}
	}
	return e.currentInput.ChooseValue, nil
}

// evalHas implements the has(<field>) presence predicate. It returns true
// when the referenced tiki has an explicit value for the named field, false
// otherwise. Presence-safe by construction.
func (e *Executor) evalHas(fc *FunctionCall, ctx evalContext) (interface{}, error) {
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
		return nil, fmt.Errorf("has() argument must be a field reference, e.g. has(status) or has(target.status)")
	}
	switch qualifier {
	case "":
		if ctx.current == nil {
			return false, nil
		}
		return tikiHas(ctx.current, name), nil
	case "outer":
		if ctx.outer == nil {
			return nil, fmt.Errorf("has(outer.%s) is not available outside a subquery", name)
		}
		return tikiHas(ctx.outer, name), nil
	case "target":
		if e.runtime.Mode != ExecutorRuntimePlugin {
			return nil, fmt.Errorf("has(target.%s): target. qualifier is only available in plugin runtime", name)
		}
		if err := checkSingleSelectionForID(e.currentInput); err != nil {
			return nil, err
		}
		id, _ := e.currentInput.SingleSelectedTikiID()
		t, ok := findTikiByID(ctx.allTikis, id)
		if !ok {
			return nil, fmt.Errorf("has(target.%s): selected tiki %q not found", name, id)
		}
		return tikiHas(t, name), nil
	case "targets":
		if e.runtime.Mode != ExecutorRuntimePlugin {
			return nil, fmt.Errorf("has(targets.%s): targets. qualifier is only available in plugin runtime", name)
		}
		selectedIDs := e.currentInput.SelectedTikiIDList()
		for _, id := range selectedIDs {
			t, ok := findTikiByID(ctx.allTikis, id)
			if !ok {
				return nil, fmt.Errorf("has(targets.%s): selected tiki %q not found", name, id)
			}
			if tikiHas(t, name) {
				return true, nil
			}
		}
		return false, nil
	case "new", "old":
		return nil, fmt.Errorf("has(%s.%s): %s. qualifier is only available in trigger guards and actions", qualifier, name, qualifier)
	default:
		return nil, fmt.Errorf("has(%s.%s): unknown qualifier %q", qualifier, name, qualifier)
	}
}

// tikiHas reports whether the tiki carries an explicit value for name.
// Identity fields are always present.
func tikiHas(t Document, name string) bool {
	if t == nil {
		return false
	}
	if IsIdentityField(name) {
		return true
	}
	return t.Has(name)
}

func (e *Executor) evalID() (interface{}, error) {
	if e.runtime.Mode != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("id() is only available in plugin runtime")
	}
	if err := checkSingleSelectionForID(e.currentInput); err != nil {
		return nil, err
	}
	id, _ := e.currentInput.SingleSelectedTikiID()
	return id, nil
}

func (e *Executor) evalIDs() (interface{}, error) {
	if e.runtime.Mode != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("ids() is only available in plugin runtime")
	}
	selected := e.currentInput.SelectedTikiIDList()
	result := make([]interface{}, len(selected))
	for i, id := range selected {
		result[i] = id
	}
	return result, nil
}

func (e *Executor) evalSelectedCount() (interface{}, error) {
	if e.runtime.Mode != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("selected_count() is only available in plugin runtime")
	}
	return e.currentInput.SelectionCount(), nil
}

func (e *Executor) evalFilepath(ctx evalContext) (interface{}, error) {
	if e.runtime.Mode != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("filepath() is only available in plugin runtime")
	}
	if err := checkSingleSelectionForBuiltin(e.currentInput, "filepath", "filepaths"); err != nil {
		return nil, err
	}
	id, _ := e.currentInput.SingleSelectedTikiID()
	t, ok := findTikiByID(ctx.allTikis, id)
	if !ok {
		return nil, fmt.Errorf("filepath(): selected tiki %q not found", id)
	}
	return t.Path(), nil
}

func (e *Executor) evalFilepaths(ctx evalContext) (interface{}, error) {
	if e.runtime.Mode != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("filepaths() is only available in plugin runtime")
	}
	selected := e.currentInput.SelectedTikiIDList()
	result := make([]interface{}, 0, len(selected))
	for _, id := range selected {
		t, ok := findTikiByID(ctx.allTikis, id)
		if !ok {
			return nil, fmt.Errorf("filepaths(): selected tiki %q not found", id)
		}
		result = append(result, t.Path())
	}
	return result, nil
}

// evalTargetField evaluates target.<field>.
func (e *Executor) evalTargetField(name string, ctx evalContext) (interface{}, error) {
	if e.runtime.Mode != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("target. qualifier is only available in plugin runtime")
	}
	if err := checkSingleSelectionForID(e.currentInput); err != nil {
		return nil, err
	}
	id, _ := e.currentInput.SingleSelectedTikiID()
	t, ok := findTikiByID(ctx.allTikis, id)
	if !ok {
		return nil, fmt.Errorf("target.%s: selected tiki %q not found", name, id)
	}
	return e.extractField(t, name)
}

// evalTargetsField evaluates targets.<field>. It projects the named field
// across all selected tikis, flattens list-valued fields, and deduplicates
// while preserving first-seen order.
func (e *Executor) evalTargetsField(name string, ctx evalContext) (interface{}, error) {
	if e.runtime.Mode != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("targets. qualifier is only available in plugin runtime")
	}
	selectedIDs := e.currentInput.SelectedTikiIDList()
	if len(selectedIDs) == 0 {
		return []interface{}{}, nil
	}
	result := make([]interface{}, 0, len(selectedIDs))
	seen := make(map[string]struct{}, len(selectedIDs))
	for _, id := range selectedIDs {
		t, ok := findTikiByID(ctx.allTikis, id)
		if !ok {
			return nil, fmt.Errorf("targets.%s: selected tiki %q not found", name, id)
		}
		// targets. projection is presentation-layer: skip tikis missing
		// the field rather than propagating the absent-field error.
		val, err := e.extractField(t, name)
		if err != nil {
			continue
		}
		if list, isList := val.([]interface{}); isList {
			for _, elem := range list {
				appendUniqueElem(&result, seen, elem)
			}
			continue
		}
		if val == nil {
			continue
		}
		appendUniqueElem(&result, seen, val)
	}
	return result, nil
}

// findTikiByID returns the tiki with the given id from the list (case-insensitive).
func findTikiByID(tikis []Document, id string) (Document, bool) {
	for _, t := range tikis {
		if strings.EqualFold(t.ID(), id) {
			return t, true
		}
	}
	return nil, false
}

// appendUniqueElem appends v to *out when its normalized string key has not
// been seen before. Shared dedupe primitive for targets.<field> projection.
func appendUniqueElem(out *[]interface{}, seen map[string]struct{}, v interface{}) {
	key := normalizeToString(v)
	if _, exists := seen[key]; exists {
		return
	}
	seen[key] = struct{}{}
	*out = append(*out, v)
}

// checkSingleSelectionForBuiltin enforces a scalar selection-builtin
// contract: exactly one selection, or an error naming the offending
// builtin. scalar names the failing builtin (e.g. "id", "filepath");
// plural is the multi-selection counterpart suggested when too many
// items are selected (e.g. "ids", "filepaths").
func checkSingleSelectionForBuiltin(in ExecutionInput, scalar, plural string) error {
	count := in.SelectionCount()
	switch {
	case count == 0:
		return &MissingSelectedTikiIDError{BuiltinName: scalar}
	case count > 1:
		return &AmbiguousSelectedTikiIDError{BuiltinName: scalar, PluralName: plural, Count: count}
	}
	return nil
}

// checkSingleSelectionForID enforces the scalar id() contract. Backwards-
// compatible wrapper around checkSingleSelectionForBuiltin.
func checkSingleSelectionForID(in ExecutionInput) error {
	return checkSingleSelectionForBuiltin(in, "id", "ids")
}

func (e *Executor) evalCount(fc *FunctionCall, parent Document, allTikis []Document) (interface{}, error) {
	sq, ok := fc.Args[0].(*SubQuery)
	if !ok {
		return nil, fmt.Errorf("count() argument must be a select subquery")
	}
	if sq.Where == nil {
		return len(allTikis), nil
	}
	count := 0
	for _, t := range allTikis {
		// Soft-false per subquery-iteration rule.
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

func (e *Executor) evalExists(fc *FunctionCall, parent Document, allTikis []Document) (interface{}, error) {
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

// EvalSubQueryFilter evaluates a subquery WHERE clause against a set of tikis,
// returning the matching tikis. Used by the controller to build candidate
// lists for choose() before showing the picker.
func (e *Executor) EvalSubQueryFilter(sq *SubQuery, tikis []Document, input ExecutionInput, parents ...Document) ([]Document, error) {
	e.currentInput = input
	defer func() { e.currentInput = ExecutionInput{} }()

	if sq == nil || sq.Where == nil {
		result := make([]Document, len(tikis))
		copy(result, tikis)
		return result, nil
	}
	parent := chooseFilterParent(tikis, input, parents...)
	var result []Document
	for _, t := range tikis {
		// Soft-false per subquery-iteration rule: choose() candidates that
		// hit an absent-field error are simply not offered.
		match, err := e.evalCondition(sq.Where, evalContext{current: t, outer: parent, allTikis: tikis})
		if err != nil {
			continue
		}
		if match {
			result = append(result, t)
		}
	}
	return result, nil
}

func chooseFilterParent(tikis []Document, input ExecutionInput, parents ...Document) Document {
	if len(parents) > 0 {
		return parents[0]
	}
	selected, ok := input.SingleSelectedTikiID()
	if !ok {
		return nil
	}
	for _, t := range tikis {
		if strings.EqualFold(t.ID(), selected) {
			return t
		}
	}
	return nil
}

func (e *Executor) evalNextDate(fc *FunctionCall, ctx evalContext) (interface{}, error) {
	// Only allow bare/qualified refs to recurrence, not arbitrary string
	// literals — next_date("daily") would bypass the recurrence type
	// contract. The validator enforces this upstream; the runtime check
	// below is a defense-in-depth for hand-built ASTs.
	if _, isField := fc.Args[0].(*FieldRef); !isField {
		if _, isQual := fc.Args[0].(*QualifiedRef); !isQual {
			// Still evaluate so we can surface a typed error. Only
			// recurrence.Recurrence is accepted from non-field callers.
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
	// Accept string (from Fields map, which holds recurrence as canonical
	// string) or recurrence.Recurrence.
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

// evalEnumStep evaluates next_enum(field) / prev_enum(field) using the base
// executor's evalExpr to read the field recurrence. See enumStepFromValue for the
// stepping logic; this function exists so the base dispatch (CLI / select /
// non-trigger contexts) can call into the shared computation while the
// trigger executor can use evalEnumStepWithLookup with its qualifier-aware
// evalExpr.
func (e *Executor) evalEnumStep(fc *FunctionCall, ctx evalContext, direction int) (interface{}, error) {
	return evalEnumStepWithLookup(e.schema, fc, direction, func(arg Expr) (interface{}, error) {
		return e.evalExpr(arg, ctx)
	})
}

// evalEnumStepWithLookup is the trigger-aware variant: callers pass a
// `lookup` closure that knows how to resolve old./new./outer. qualifiers in
// the calling context. The base Executor's evalExpr rejects old./new./
// target. as "not supported in standalone SELECT", so triggers MUST go
// through this entry point with their override's evalExpr.
func evalEnumStepWithLookup(schema Schema, fc *FunctionCall, direction int, lookup func(Expr) (interface{}, error)) (interface{}, error) {
	fnName := "next_enum"
	if direction < 0 {
		fnName = "prev_enum"
	}
	var fieldName string
	switch ref := fc.Args[0].(type) {
	case *FieldRef:
		fieldName = ref.Name
	case *QualifiedRef:
		fieldName = ref.Name
	default:
		return nil, fmt.Errorf("%s() argument must be an enum field reference", fnName)
	}

	fs, ok := schema.Field(fieldName)
	if !ok {
		return nil, fmt.Errorf("%s(): unknown field %q", fnName, fieldName)
	}
	if fs.Type != ValueEnum {
		return nil, fmt.Errorf("%s() argument must be an enum field, got %s for %q", fnName, typeName(fs.Type), fieldName)
	}
	if len(fs.AllowedValues) == 0 {
		return nil, fmt.Errorf("%s(): enum field %q has no allowed values", fnName, fieldName)
	}

	val, err := lookup(fc.Args[0])
	if err != nil {
		// Only clamp to a boundary when the field is genuinely absent.
		// Other errors (qualifier rejection, schema mismatch, etc.) must
		// propagate — silently treating them as "absent" would mask real
		// configuration mistakes and let trigger rules write the wrong
		// priority.
		var absentErr *AbsentFieldError
		if !errors.As(err, &absentErr) {
			return nil, err
		}
		return enumStepBoundary(fs, direction), nil
	}
	if val == nil {
		return enumStepBoundary(fs, direction), nil
	}
	// Treat empty string as "field present-but-blank" — common when a
	// helper initialises every workflow field to its zero recurrence. Use the
	// boundary rather than erroring so `prev_enum(priority)` on a freshly
	// created tiki still produces a sensible recurrence.
	if s, ok := val.(string); ok && s == "" {
		return enumStepBoundary(fs, direction), nil
	}

	rank, _, ok := enumRank(fs, normalizeToString(val))
	if !ok {
		return nil, fmt.Errorf("%s(): invalid enum value %q for field %q", fnName, normalizeToString(val), fieldName)
	}
	next := rank + direction
	if next < 0 {
		next = 0
	}
	if next >= len(fs.AllowedValues) {
		next = len(fs.AllowedValues) - 1
	}
	return fs.AllowedValues[next], nil
}

// enumStepBoundary returns the boundary value for an absent-field step in
// the given direction: prev steps land on the first declared value, next
// steps land on the last. Mirrors next_date's "zero-time on absent" idiom.
func enumStepBoundary(fs FieldSpec, direction int) string {
	if direction < 0 {
		return fs.AllowedValues[0]
	}
	return fs.AllowedValues[len(fs.AllowedValues)-1]
}

func (e *Executor) evalBlocks(fc *FunctionCall, ctx evalContext) (interface{}, error) {
	val, err := e.evalExpr(fc.Args[0], ctx)
	if err != nil {
		return nil, err
	}
	targetID := strings.ToUpper(normalizeToString(val))

	var blockers []interface{}
	for _, at := range ctx.allTikis {
		// Skip tikis without a dependsOn field per the blocks-scan
		// soft-false rule: absent lists don't block anything.
		deps, ok := tikiStringSlice(at, fieldDependsOn)
		if !ok {
			continue
		}
		for _, dep := range deps {
			if strings.EqualFold(dep, targetID) {
				blockers = append(blockers, at.ID())
				break
			}
		}
	}
	if blockers == nil {
		blockers = []interface{}{}
	}
	return blockers, nil
}

// tikiStringSlice reads a string-slice-typed field without propagating an
// absent-field error. Returns (slice, true) when the field is present (even
// if empty); (nil, false) when absent.
func tikiStringSlice(t Document, name string) ([]string, bool) {
	if t == nil {
		return nil, false
	}
	v, ok := t.Get(name)
	if !ok {
		return nil, false
	}
	switch s := v.(type) {
	case []string:
		return s, true
	case []interface{}:
		out := make([]string, 0, len(s))
		for _, elem := range s {
			out = append(out, normalizeToString(elem))
		}
		return out, true
	default:
		return nil, true
	}
}

// --- binary expression evaluation ---

func (e *Executor) evalBinaryExpr(b *BinaryExpr, ctx evalContext) (interface{}, error) {
	leftVal, err := e.evalExpr(b.Left, ctx)
	if err != nil {
		return nil, err
	}
	rightVal, err := e.evalExpr(b.Right, ctx)
	if err != nil {
		return nil, err
	}

	switch b.Op {
	case "+":
		return addValues(leftVal, rightVal)
	case "-":
		return subtractValues(leftVal, rightVal)
	default:
		return nil, fmt.Errorf("unknown binary operator %q", b.Op)
	}
}

// readFieldRefWithCarveOut reads a field reference off a tiki, applying the
// assignment-RHS auto-zero carve-out. When ctx.inAssignmentRHS is true and
// the field is absent on the target tiki, the function returns the type-
// appropriate zero for any workflow-declared field instead of hard-
// erroring. Unregistered field names fall through to the normal hard-error
// path so typos keep failing loudly.
//
// Outside an assignment RHS, the function behaves identically to
// extractField — absent reads error uniformly.
func (e *Executor) readFieldRefWithCarveOut(t Document, name string, ctx evalContext) (interface{}, error) {
	if ctx.inAssignmentRHS && t != nil && !IsIdentityField(name) && !t.Has(name) {
		if zero, ok := e.registeredFieldZero(name); ok {
			return zero, nil
		}
	}
	return e.extractField(t, name)
}

// registeredFieldZero returns the type-appropriate zero value for a field
// known to the schema. Returns (nil, false) for names the schema does not
// know, so the caller falls through to the normal hard-error path.
func (e *Executor) registeredFieldZero(name string) (interface{}, bool) {
	fs, ok := e.schema.Field(name)
	if !ok {
		return nil, false
	}
	return fieldZeroForType(fs.Type), true
}

// fieldZeroForType returns the zero value for a field by its ruki ValueType.
// Used to project an absent field as a typed zero so projections stay
// rectangular — every workflow-declared field is treated uniformly.
func fieldZeroForType(t ValueType) interface{} {
	switch t {
	case ValueListString, ValueListRef:
		return []interface{}{}
	case ValueInt:
		return 0
	case ValueBool:
		return false
	case ValueTimestamp, ValueDate:
		return time.Time{}
	case ValueString, ValueEnum, ValueRef, ValueID, ValueRecurrence, ValueDuration:
		return ""
	}
	return ""
}

func addValues(left, right interface{}) (interface{}, error) {
	switch l := left.(type) {
	case int:
		if r, ok := right.(int); ok {
			return l + r, nil
		}
	case time.Time:
		if r, ok := right.(time.Duration); ok {
			return l.Add(r), nil
		}
	case string:
		if r, ok := right.(string); ok {
			return l + r, nil
		}
	case []interface{}:
		if r, ok := right.([]interface{}); ok {
			return appendUniqueListValues(l, r), nil
		}
		return appendUniqueListValues(l, []interface{}{right}), nil
	case []string:
		return addValues(stringSliceToInterface(l), right)
	}
	return nil, fmt.Errorf("cannot add %T + %T", left, right)
}

func stringSliceToInterface(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func appendUniqueListValues(left, right []interface{}) []interface{} {
	result := make([]interface{}, 0, len(left)+len(right))
	seen := make(map[string]struct{}, len(left)+len(right))
	for _, elem := range left {
		key := normalizeToString(elem)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, elem)
	}
	for _, elem := range right {
		key := normalizeToString(elem)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, elem)
	}
	if result == nil {
		return []interface{}{}
	}
	return result
}

func subtractValues(left, right interface{}) (interface{}, error) {
	switch l := left.(type) {
	case int:
		if r, ok := right.(int); ok {
			return l - r, nil
		}
	case time.Time:
		switch r := right.(type) {
		case time.Duration:
			return l.Add(-r), nil
		case time.Time:
			return l.Sub(r), nil
		}
	case []interface{}:
		var toRemove []interface{}
		if r, ok := right.([]interface{}); ok {
			toRemove = r
		} else {
			toRemove = []interface{}{right}
		}
		removeSet := make(map[string]bool, len(toRemove))
		for _, elem := range toRemove {
			removeSet[normalizeToString(elem)] = true
		}
		var result []interface{}
		for _, elem := range l {
			if !removeSet[normalizeToString(elem)] {
				result = append(result, elem)
			}
		}
		if result == nil {
			result = []interface{}{}
		}
		return result, nil
	case []string:
		return subtractValues(stringSliceToInterface(l), right)
	}
	return nil, fmt.Errorf("cannot subtract %T - %T", left, right)
}

// --- sorting ---

func (e *Executor) sortTikis(tikis []Document, clauses []OrderByClause) error {
	// Pre-extract all sort keys so a missing field surfaces as an error
	// before sort.Slice starts swapping.
	specs := make([]FieldSpec, len(clauses))
	for i, c := range clauses {
		fs, ok := e.schema.Field(c.Field)
		if !ok {
			return fmt.Errorf("order by %q: unknown field", c.Field)
		}
		specs[i] = fs
	}
	keys := make([][]interface{}, len(tikis))
	for i, t := range tikis {
		row := make([]interface{}, len(clauses))
		for j, c := range clauses {
			v, err := e.extractField(t, c.Field)
			if err != nil {
				return fmt.Errorf("order by %q: %w", c.Field, err)
			}
			key, err := sortKeyForField(specs[j], v)
			if err != nil {
				return fmt.Errorf("order by %q: %w", c.Field, err)
			}
			row[j] = key
		}
		keys[i] = row
	}
	indices := make([]int, len(tikis))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(i, j int) bool {
		ki, kj := keys[indices[i]], keys[indices[j]]
		for idx, c := range clauses {
			cmp := compareForSort(ki[idx], kj[idx])
			if cmp == 0 {
				continue
			}
			if c.Desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
	// reorder tikis in place
	tmp := make([]Document, len(tikis))
	for i, idx := range indices {
		tmp[i] = tikis[idx]
	}
	copy(tikis, tmp)
	return nil
}

type enumSortKey struct {
	rank  int
	value string
}

func sortKeyForField(fs FieldSpec, v interface{}) (interface{}, error) {
	if fs.Type != ValueEnum || v == nil {
		return v, nil
	}
	rank, canonical, ok := enumRank(fs, normalizeToString(v))
	if !ok {
		return nil, fmt.Errorf("invalid enum value %q", normalizeToString(v))
	}
	return enumSortKey{rank: rank, value: canonical}, nil
}

func compareForSort(a, b interface{}) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1
	}
	if b == nil {
		return -1
	}

	switch av := a.(type) {
	case enumSortKey:
		bv, _ := b.(enumSortKey)
		if av.rank != bv.rank {
			return compareInts(av.rank, bv.rank)
		}
		return strings.Compare(av.value, bv.value)
	case int:
		bv, _ := b.(int)
		return compareInts(av, bv)
	case string:
		bv, _ := b.(string)
		return strings.Compare(av, bv)
	case bool:
		bv, _ := b.(bool)
		if av == bv {
			return 0
		}
		if !av && bv {
			return -1
		}
		return 1
	case time.Time:
		bv, _ := b.(time.Time)
		if av.Before(bv) {
			return -1
		}
		if av.After(bv) {
			return 1
		}
		return 0
	case recurrence.Recurrence:
		bv, _ := b.(recurrence.Recurrence)
		return strings.Compare(string(av), string(bv))
	case time.Duration:
		bv, _ := b.(time.Duration)
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
		return 0
	default:
		return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
	}
}

func enumRank(fs FieldSpec, raw string) (int, string, bool) {
	for i, av := range fs.AllowedValues {
		if strings.EqualFold(av, raw) {
			return i, av, true
		}
	}
	return 0, "", false
}

func compareInts(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// --- comparison ---

func (e *Executor) compareValues(left, right interface{}, op string, leftExpr, rightExpr Expr) (bool, error) {
	if left == nil || right == nil {
		return compareWithNil(left, right, op, leftExpr, rightExpr)
	}

	if leftList, ok := left.([]interface{}); ok {
		if rightList, ok := right.([]interface{}); ok {
			return compareListEquality(leftList, rightList, op)
		}
	}
	if leftList, ok := left.([]string); ok {
		return compareListEquality(stringSliceToInterface(leftList), ensureInterfaceSlice(right), op)
	}
	if rightList, ok := right.([]string); ok {
		return compareListEquality(ensureInterfaceSlice(left), stringSliceToInterface(rightList), op)
	}

	if lb, ok := left.(bool); ok {
		if rb, ok := right.(bool); ok {
			return compareBools(lb, rb, op)
		}
		if rs, ok := right.(string); ok {
			if rb, err := parseBoolString(rs); err == nil {
				return compareBools(lb, rb, op)
			}
		}
	}
	if rb, ok := right.(bool); ok {
		if ls, ok := left.(string); ok {
			if lb, err := parseBoolString(ls); err == nil {
				return compareBools(lb, rb, op)
			}
		}
	}

	compType := e.resolveComparisonType(leftExpr, rightExpr)

	switch compType {
	case ValueID:
		return compareStringsCI(normalizeToString(left), normalizeToString(right), op)
	case ValueEnum:
		fs, ok := e.enumComparisonField(leftExpr, rightExpr)
		if !ok {
			return false, fmt.Errorf("cannot resolve enum comparison domain")
		}
		return compareEnumValues(fs, left, right, op)
	}

	switch lv := left.(type) {
	case string:
		return compareStrings(lv, normalizeToString(right), op)
	case int:
		rv, ok := toInt(right)
		if !ok {
			return false, fmt.Errorf("cannot compare int with %T", right)
		}
		return compareIntValues(lv, rv, op)
	case time.Time:
		rv, ok := right.(time.Time)
		if !ok {
			return false, fmt.Errorf("cannot compare time with %T", right)
		}
		return compareTimes(lv, rv, op)
	case time.Duration:
		rv, ok := right.(time.Duration)
		if !ok {
			return false, fmt.Errorf("cannot compare duration with %T", right)
		}
		return compareDurations(lv, rv, op)
	case recurrence.Recurrence:
		return compareStrings(string(lv), normalizeToString(right), op)
	default:
		return false, fmt.Errorf("unsupported comparison type %T", left)
	}
}

func ensureInterfaceSlice(v interface{}) []interface{} {
	switch s := v.(type) {
	case []interface{}:
		return s
	case []string:
		return stringSliceToInterface(s)
	default:
		return []interface{}{v}
	}
}

// resolveComparisonType returns the dominant field type for a comparison.
func (e *Executor) resolveComparisonType(left, right Expr) ValueType {
	if t := e.exprFieldType(left); t == ValueID || t == ValueEnum {
		return t
	}
	if t := e.exprFieldType(right); t == ValueID || t == ValueEnum {
		return t
	}
	return -1
}

func (e *Executor) enumComparisonField(left, right Expr) (FieldSpec, bool) {
	if fs, ok := e.exprFieldSpec(left); ok && fs.Type == ValueEnum {
		return fs, true
	}
	if fs, ok := e.exprFieldSpec(right); ok && fs.Type == ValueEnum {
		return fs, true
	}
	return FieldSpec{}, false
}

func (e *Executor) exprFieldSpec(expr Expr) (FieldSpec, bool) {
	var name string
	switch ex := expr.(type) {
	case *FieldRef:
		name = ex.Name
	case *QualifiedRef:
		name = ex.Name
	case *FunctionCall:
		// next_enum(field) / prev_enum(field) preserve the argument's
		// enum domain — propagate the field identity so enum-aware
		// comparisons (enumRank, compareEnumValues) treat the result
		// the same as a direct field reference would. Without this,
		// `next_enum(priority) < "low"` falls back to plain string
		// comparison and "high" > "low" (lexicographic) instead of
		// rank-aware ordering.
		if (ex.Name == "next_enum" || ex.Name == "prev_enum") && len(ex.Args) == 1 {
			return e.exprFieldSpec(ex.Args[0])
		}
		return FieldSpec{}, false
	default:
		return FieldSpec{}, false
	}
	return e.schema.Field(name)
}

func (e *Executor) exprFieldType(expr Expr) ValueType {
	var name string
	switch ex := expr.(type) {
	case *FieldRef:
		name = ex.Name
	case *QualifiedRef:
		name = ex.Name
	case *FunctionCall:
		if ex.Name == "id" {
			return ValueID
		}
		// next_enum(field) / prev_enum(field) carry their argument's
		// enum domain — recurse so callers that use exprFieldType to
		// route equality comparisons (e.g. isEnumLikeField) treat the
		// result as a real enum value, not a plain int/string.
		if (ex.Name == "next_enum" || ex.Name == "prev_enum") && len(ex.Args) == 1 {
			return e.exprFieldType(ex.Args[0])
		}
		return -1
	default:
		return -1
	}
	fs, ok := e.schema.Field(name)
	if !ok {
		return -1
	}
	return fs.Type
}

// isEnumLikeField returns true for field types that use case-insensitive
// comparison in equality checks and should also use it for in/not-in.
func isEnumLikeField(t ValueType) bool {
	return t == ValueEnum || t == ValueID || t == ValueBool
}

func compareWithNil(left, right interface{}, op string, leftExpr, rightExpr Expr) (bool, error) {
	// when comparing against EmptyLiteral, use zero-value semantics
	_, leftIsEmpty := leftExpr.(*EmptyLiteral)
	_, rightIsEmpty := rightExpr.(*EmptyLiteral)
	if leftIsEmpty || rightIsEmpty {
		leftEmpty := isZeroValue(left)
		rightEmpty := isZeroValue(right)
		bothEmpty := leftEmpty && rightEmpty
		switch op {
		case "=":
			return bothEmpty, nil
		case "!=":
			return !bothEmpty, nil
		default:
			return false, nil
		}
	}

	bothNil := left == nil && right == nil
	switch op {
	case "=":
		return bothNil, nil
	case "!=":
		return !bothNil, nil
	default:
		return false, nil
	}
}

func compareListEquality(a, b []interface{}, op string) (bool, error) {
	switch op {
	case "=":
		return sortedSetEqual(a, b), nil
	case "!=":
		return !sortedSetEqual(a, b), nil
	default:
		return false, fmt.Errorf("operator %s not supported for list comparison", op)
	}
}

func sortedSetEqual(a, b []interface{}) bool {
	as := toSortedUniqueStrings(a)
	bs := toSortedUniqueStrings(b)
	if len(as) != len(bs) {
		return false
	}
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

func toSortedUniqueStrings(list []interface{}) []string {
	seen := make(map[string]struct{}, len(list))
	s := make([]string, 0, len(list))
	for _, v := range list {
		value := normalizeToString(v)
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		s = append(s, value)
	}
	sort.Strings(s)
	return s
}

func compareBools(a, b bool, op string) (bool, error) {
	switch op {
	case "=":
		return a == b, nil
	case "!=":
		return a != b, nil
	default:
		return false, fmt.Errorf("operator %s not supported for bool comparison", op)
	}
}

func compareStrings(a, b, op string) (bool, error) {
	switch op {
	case "=":
		return a == b, nil
	case "!=":
		return a != b, nil
	default:
		return false, fmt.Errorf("operator %s not supported for string comparison", op)
	}
}

func compareStringsCI(a, b, op string) (bool, error) {
	a = strings.ToUpper(a)
	b = strings.ToUpper(b)
	switch op {
	case "=":
		return a == b, nil
	case "!=":
		return a != b, nil
	default:
		return false, fmt.Errorf("operator %s not supported for id comparison", op)
	}
}

func compareEnumValues(fs FieldSpec, left, right interface{}, op string) (bool, error) {
	leftRank, leftValue, ok := enumRank(fs, normalizeToString(left))
	if !ok {
		return false, fmt.Errorf("invalid enum value %q", normalizeToString(left))
	}
	rightRank, rightValue, ok := enumRank(fs, normalizeToString(right))
	if !ok {
		return false, fmt.Errorf("invalid enum value %q", normalizeToString(right))
	}
	switch op {
	case "=":
		return leftValue == rightValue, nil
	case "!=":
		return leftValue != rightValue, nil
	case "<":
		return leftRank < rightRank, nil
	case ">":
		return leftRank > rightRank, nil
	case "<=":
		return leftRank <= rightRank, nil
	case ">=":
		return leftRank >= rightRank, nil
	default:
		return false, fmt.Errorf("unknown operator %q", op)
	}
}

func compareIntValues(a, b int, op string) (bool, error) {
	switch op {
	case "=":
		return a == b, nil
	case "!=":
		return a != b, nil
	case "<":
		return a < b, nil
	case ">":
		return a > b, nil
	case "<=":
		return a <= b, nil
	case ">=":
		return a >= b, nil
	default:
		return false, fmt.Errorf("unknown operator %q", op)
	}
}

func compareTimes(a, b time.Time, op string) (bool, error) {
	switch op {
	case "=":
		return a.Equal(b), nil
	case "!=":
		return !a.Equal(b), nil
	case "<":
		return a.Before(b), nil
	case ">":
		return a.After(b), nil
	case "<=":
		return !a.After(b), nil
	case ">=":
		return !a.Before(b), nil
	default:
		return false, fmt.Errorf("unknown operator %q", op)
	}
}

func compareDurations(a, b time.Duration, op string) (bool, error) {
	switch op {
	case "=":
		return a == b, nil
	case "!=":
		return a != b, nil
	case "<":
		return a < b, nil
	case ">":
		return a > b, nil
	case "<=":
		return a <= b, nil
	case ">=":
		return a >= b, nil
	default:
		return false, fmt.Errorf("unknown operator %q", op)
	}
}

// --- field extraction ---

// extractField reads a field off a tiki. Identity/audit fields always succeed.
// For any other name, an absent field hard-errors — callers that need
// presence-safety must go through has(<field>) or use extractFieldForDisplay
// for presentation-layer consumers that should render blank.
func (e *Executor) extractField(t Document, name string) (interface{}, error) {
	if t == nil {
		return nil, fmt.Errorf("no current row to read %q from", name)
	}
	switch name {
	case "id":
		return t.ID(), nil
	case "title":
		return t.Title(), nil
	case "description", "body":
		return t.Body(), nil
	case "createdBy":
		// CreatedBy doesn't round-trip through the Tiki model (Phase 4
		// does not carry author metadata). Return empty string so identity
		// extraction is lossless enough for pipe/display use cases.
		v, _ := t.Get("createdBy")
		if s, ok := v.(string); ok {
			return s, nil
		}
		return "", nil
	case "createdAt":
		return t.CreatedAt(), nil
	case "updatedAt":
		return t.UpdatedAt(), nil
	case "filepath", "path":
		return t.Path(), nil
	}

	v, ok := t.Get(name)
	if !ok {
		return nil, absentFieldError(t, name)
	}
	return normalizeExtractedValue(v), nil
}

// extractFieldForDisplay is the presentation-layer variant used by pipe/
// clipboard rendering and formatters. Absent fields return nil rather than
// erroring — a missing field renders as a blank cell, not an aborted query.
func (e *Executor) extractFieldForDisplay(t Document, name string) (interface{}, error) {
	v, err := e.extractField(t, name)
	if err != nil {
		// Only swallow absent-field errors; surface any other error
		// (e.g. subquery misuse) so bugs don't hide.
		if _, isAbsent := err.(*AbsentFieldError); isAbsent {
			return nil, nil
		}
		return nil, err
	}
	return v, nil
}

// normalizeExtractedValue converts []string to []interface{} so list-typed
// fields participate in ruki's list arithmetic uniformly.
func normalizeExtractedValue(v interface{}) interface{} {
	if ss, ok := v.([]string); ok {
		return toInterfaceSlice(ss)
	}
	return v
}

// AbsentFieldError is returned by extractField when a non-identity field is
// absent on a tiki. Callers that want presence-safety can detect it via
// errors.As; otherwise it propagates up to kill the query.
type AbsentFieldError struct {
	TikiID string
	Field  string
}

func (e *AbsentFieldError) Error() string {
	id := e.TikiID
	if id == "" {
		id = "<unidentified>"
	}
	return fmt.Sprintf("tiki %s: field %q is not set", id, e.Field)
}

func absentFieldError(t Document, name string) error {
	id := ""
	if t != nil {
		id = t.ID()
	}
	return &AbsentFieldError{TikiID: id, Field: name}
}

// --- helpers ---

// parseBoolString converts a string "true"/"false" (case-insensitive) to a bool.
func parseBoolString(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("not a bool string: %q", s)
	}
}

func toInterfaceSlice(ss []string) []interface{} {
	if ss == nil {
		return []interface{}{}
	}
	result := make([]interface{}, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}

func normalizeToString(v interface{}) string {
	switch v := v.(type) {
	case string:
		return v
	case recurrence.Recurrence:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

func toInt(v interface{}) (int, bool) {
	switch v := v.(type) {
	case int:
		return v, true
	default:
		return 0, false
	}
}

func isZeroValue(v interface{}) bool {
	if v == nil {
		return true
	}
	switch v := v.(type) {
	case string:
		return v == ""
	case int:
		return v == 0
	case time.Time:
		return v.IsZero()
	case time.Duration:
		return v == 0
	case bool:
		return !v
	case recurrence.Recurrence:
		return v == ""
	case []interface{}:
		return len(v) == 0
	case []string:
		return len(v) == 0
	default:
		return false
	}
}
