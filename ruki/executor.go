package ruki

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/util/duration"
)

// Executor evaluates parsed ruki statements against a set of tasks.
type Executor struct {
	schema       Schema
	userFunc     func() string
	runtime      ExecutorRuntime
	currentInput ExecutionInput
}

// NewExecutor constructs an Executor with the given schema and user function.
// If userFunc is nil, calling user() at runtime will return "".
func NewExecutor(schema Schema, userFunc func() string, runtime ExecutorRuntime) *Executor {
	if userFunc == nil {
		userFunc = func() string { return "" }
	}
	return &Executor{
		schema:   schema,
		userFunc: userFunc,
		runtime:  runtime.normalize(),
	}
}

// Result holds the output of executing a statement.
// Exactly one variant is non-nil.
type Result struct {
	Select    *TaskProjection
	Update    *UpdateResult
	Create    *CreateResult
	Delete    *DeleteResult
	Pipe      *PipeResult
	Clipboard *ClipboardResult
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

// UpdateResult holds the cloned, mutated tasks produced by an UPDATE statement.
type UpdateResult struct {
	Updated []*task.Task
}

// CreateResult holds the new task produced by a CREATE statement.
type CreateResult struct {
	Task *task.Task
}

// DeleteResult holds the tasks matched by a DELETE statement's WHERE clause.
type DeleteResult struct {
	Deleted []*task.Task
}

// TaskProjection holds the filtered, sorted tasks and the requested field list.
type TaskProjection struct {
	Tasks  []*task.Task
	Fields []string // user-requested fields; nil/empty = all fields
}

// Execute dispatches on the statement type and returns results.
// Preferred input is *ValidatedStatement; raw *Statement is accepted as a
// low-level path and will be semantically validated for executor runtime mode.
func (e *Executor) Execute(stmt any, tasks []*task.Task, inputs ...ExecutionInput) (*Result, error) {
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
	case rawStmt.Select != nil:
		return e.executeSelect(rawStmt.Select, tasks)
	case rawStmt.Create != nil:
		return e.executeCreate(rawStmt.Create, tasks, requiresCreateTemplate)
	case rawStmt.Update != nil:
		return e.executeUpdate(rawStmt.Update, tasks)
	case rawStmt.Delete != nil:
		return e.executeDelete(rawStmt.Delete, tasks)
	default:
		return nil, fmt.Errorf("empty statement")
	}
}

func (e *Executor) executeSelect(sel *SelectStmt, tasks []*task.Task) (*Result, error) {
	filtered, err := e.filterTasks(sel.Where, tasks)
	if err != nil {
		return nil, err
	}

	if len(sel.OrderBy) > 0 {
		e.sortTasks(filtered, sel.OrderBy)
	}

	if sel.Pipe != nil {
		switch {
		case sel.Pipe.Run != nil:
			return e.buildPipeResult(sel.Pipe.Run, sel.Fields, filtered, tasks)
		case sel.Pipe.Clipboard != nil:
			return e.buildClipboardResult(sel.Fields, filtered)
		}
	}

	return &Result{
		Select: &TaskProjection{
			Tasks:  filtered,
			Fields: sel.Fields,
		},
	}, nil
}

func (e *Executor) buildPipeResult(pipe *RunAction, fields []string, matched []*task.Task, allTasks []*task.Task) (*Result, error) {
	// evaluate command once with a nil-sentinel task — validation ensures no field refs
	cmdVal, err := e.evalExpr(pipe.Command, nil, allTasks)
	if err != nil {
		return nil, fmt.Errorf("pipe command: %w", err)
	}
	cmdStr, ok := cmdVal.(string)
	if !ok {
		return nil, fmt.Errorf("pipe command must evaluate to string, got %T", cmdVal)
	}

	rows := buildFieldRows(fields, matched)
	return &Result{Pipe: &PipeResult{Command: cmdStr, Rows: rows}}, nil
}

func (e *Executor) buildClipboardResult(fields []string, matched []*task.Task) (*Result, error) {
	rows := buildFieldRows(fields, matched)
	return &Result{Clipboard: &ClipboardResult{Rows: rows}}, nil
}

// buildFieldRows extracts the requested fields from matched tasks as string rows.
// Shared by both run() and clipboard() pipe targets.
func buildFieldRows(fields []string, matched []*task.Task) [][]string {
	rows := make([][]string, len(matched))
	for i, t := range matched {
		row := make([]string, len(fields))
		for j, f := range fields {
			row[j] = pipeArgString(extractField(t, f))
		}
		rows[i] = row
	}
	return rows
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

func (e *Executor) executeUpdate(upd *UpdateStmt, tasks []*task.Task) (*Result, error) {
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

func (e *Executor) executeCreate(cr *CreateStmt, tasks []*task.Task, requireTemplate bool) (*Result, error) {
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

func (e *Executor) executeDelete(del *DeleteStmt, tasks []*task.Task) (*Result, error) {
	matched, err := e.filterTasks(del.Where, tasks)
	if err != nil {
		return nil, err
	}

	return &Result{Delete: &DeleteResult{Deleted: matched}}, nil
}

func (e *Executor) setField(t *task.Task, name string, val interface{}) error {
	switch name {
	case "id", "createdBy", "createdAt", "updatedAt":
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
		t.Title = s

	case "description":
		if val == nil {
			t.Description = ""
			return nil
		}
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("description must be a string, got %T", val)
		}
		t.Description = s

	case "status":
		if val == nil {
			return fmt.Errorf("cannot set status to empty")
		}
		s, ok := val.(string)
		if !ok {
			sv, ok2 := val.(task.Status)
			if !ok2 {
				return fmt.Errorf("status must be a string, got %T", val)
			}
			s = string(sv)
		}
		norm, valid := e.schema.NormalizeStatus(s)
		if !valid {
			return fmt.Errorf("unknown status %q", s)
		}
		t.Status = task.Status(norm)

	case "type":
		if val == nil {
			return fmt.Errorf("cannot set type to empty")
		}
		s, ok := val.(string)
		if !ok {
			tv, ok2 := val.(task.Type)
			if !ok2 {
				return fmt.Errorf("type must be a string, got %T", val)
			}
			s = string(tv)
		}
		norm, valid := e.schema.NormalizeType(s)
		if !valid {
			return fmt.Errorf("unknown type %q", s)
		}
		t.Type = task.Type(norm)

	case "priority":
		if val == nil {
			return fmt.Errorf("cannot set priority to empty")
		}
		n, ok := val.(int)
		if !ok {
			return fmt.Errorf("priority must be an int, got %T", val)
		}
		if !task.IsValidPriority(n) {
			return fmt.Errorf("priority must be between %d and %d", task.MinPriority, task.MaxPriority)
		}
		t.Priority = n

	case "points":
		if val == nil {
			t.Points = 0
			return nil
		}
		n, ok := val.(int)
		if !ok {
			return fmt.Errorf("points must be an int, got %T", val)
		}
		if !task.IsValidPoints(n) {
			return fmt.Errorf("invalid points value: %d", n)
		}
		t.Points = n

	case "tags":
		if val == nil {
			t.Tags = nil
			return nil
		}
		t.Tags = toStringSlice(val)

	case "dependsOn":
		if val == nil {
			t.DependsOn = nil
			return nil
		}
		t.DependsOn = toStringSlice(val)

	case "due":
		if val == nil {
			t.Due = time.Time{}
			return nil
		}
		d, ok := val.(time.Time)
		if !ok {
			return fmt.Errorf("due must be a date, got %T", val)
		}
		t.Due = d

	case "recurrence":
		if val == nil {
			t.Recurrence = ""
			return nil
		}
		switch v := val.(type) {
		case string:
			t.Recurrence = task.Recurrence(v)
		case task.Recurrence:
			t.Recurrence = v
		default:
			return fmt.Errorf("recurrence must be a string, got %T", val)
		}

	case "assignee":
		if val == nil {
			t.Assignee = ""
			return nil
		}
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("assignee must be a string, got %T", val)
		}
		t.Assignee = s

	default:
		return fmt.Errorf("unknown field %q", name)
	}
	return nil
}

func toStringSlice(val interface{}) []string {
	list, ok := val.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, len(list))
	for i, elem := range list {
		result[i] = normalizeToString(elem)
	}
	return result
}

// --- filtering ---

func (e *Executor) filterTasks(where Condition, tasks []*task.Task) ([]*task.Task, error) {
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

// --- condition evaluation ---

func (e *Executor) evalCondition(c Condition, t *task.Task, allTasks []*task.Task) (bool, error) {
	switch c := c.(type) {
	case *BinaryCondition:
		return e.evalBinaryCondition(c, t, allTasks)
	case *NotCondition:
		val, err := e.evalCondition(c.Inner, t, allTasks)
		if err != nil {
			return false, err
		}
		return !val, nil
	case *CompareExpr:
		return e.evalCompare(c, t, allTasks)
	case *IsEmptyExpr:
		return e.evalIsEmpty(c, t, allTasks)
	case *InExpr:
		return e.evalIn(c, t, allTasks)
	case *QuantifierExpr:
		return e.evalQuantifier(c, t, allTasks)
	default:
		return false, fmt.Errorf("unknown condition type %T", c)
	}
}

func (e *Executor) evalBinaryCondition(c *BinaryCondition, t *task.Task, allTasks []*task.Task) (bool, error) {
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
}

func (e *Executor) evalCompare(c *CompareExpr, t *task.Task, allTasks []*task.Task) (bool, error) {
	leftVal, err := e.evalExpr(c.Left, t, allTasks)
	if err != nil {
		return false, err
	}
	rightVal, err := e.evalExpr(c.Right, t, allTasks)
	if err != nil {
		return false, err
	}
	return e.compareValues(leftVal, rightVal, c.Op, c.Left, c.Right)
}

func (e *Executor) evalIsEmpty(c *IsEmptyExpr, t *task.Task, allTasks []*task.Task) (bool, error) {
	val, err := e.evalExpr(c.Expr, t, allTasks)
	if err != nil {
		return false, err
	}
	empty := isZeroValue(val)
	if c.Negated {
		return !empty, nil
	}
	return empty, nil
}

func (e *Executor) evalIn(c *InExpr, t *task.Task, allTasks []*task.Task) (bool, error) {
	val, err := e.evalExpr(c.Value, t, allTasks)
	if err != nil {
		return false, err
	}
	collVal, err := e.evalExpr(c.Collection, t, allTasks)
	if err != nil {
		return false, err
	}

	// list membership mode
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

	// substring mode — guarded assertions, no panics on hand-built ASTs
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

func (e *Executor) evalQuantifier(q *QuantifierExpr, t *task.Task, allTasks []*task.Task) (bool, error) {
	listVal, err := e.evalExpr(q.Expr, t, allTasks)
	if err != nil {
		return false, err
	}
	refs, ok := listVal.([]interface{})
	if !ok {
		return false, fmt.Errorf("quantifier: expression is not a list")
	}

	// find referenced tasks
	refTasks := make([]*task.Task, 0, len(refs))
	for _, ref := range refs {
		refID := normalizeToString(ref)
		for _, at := range allTasks {
			if strings.EqualFold(at.ID, refID) {
				refTasks = append(refTasks, at)
				break
			}
		}
	}

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
			return true, nil // vacuous truth
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

// --- expression evaluation ---

func (e *Executor) evalExpr(expr Expr, t *task.Task, allTasks []*task.Task) (interface{}, error) {
	switch expr := expr.(type) {
	case *FieldRef:
		return extractField(t, expr.Name), nil
	case *QualifiedRef:
		return nil, fmt.Errorf("qualified references (old./new.) are not supported in standalone SELECT")
	case *StringLiteral:
		return expr.Value, nil
	case *IntLiteral:
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
		return e.evalListLiteral(expr, t, allTasks)
	case *EmptyLiteral:
		return nil, nil
	case *FunctionCall:
		return e.evalFunctionCall(expr, t, allTasks)
	case *BinaryExpr:
		return e.evalBinaryExpr(expr, t, allTasks)
	case *SubQuery:
		return nil, fmt.Errorf("subquery is only valid as argument to count()")
	default:
		return nil, fmt.Errorf("unknown expression type %T", expr)
	}
}

func (e *Executor) evalListLiteral(ll *ListLiteral, t *task.Task, allTasks []*task.Task) (interface{}, error) {
	result := make([]interface{}, len(ll.Elements))
	for i, elem := range ll.Elements {
		val, err := e.evalExpr(elem, t, allTasks)
		if err != nil {
			return nil, err
		}
		result[i] = val
	}
	return result, nil
}

// --- function evaluation ---

func (e *Executor) evalFunctionCall(fc *FunctionCall, t *task.Task, allTasks []*task.Task) (interface{}, error) {
	switch fc.Name {
	case "id":
		return e.evalID()
	case "now":
		return time.Now(), nil
	case "user":
		return e.userFunc(), nil
	case "count":
		return e.evalCount(fc, allTasks)
	case "next_date":
		return e.evalNextDate(fc, t, allTasks)
	case "blocks":
		return e.evalBlocks(fc, t, allTasks)
	case "call":
		return nil, fmt.Errorf("call() is not supported yet")
	default:
		return nil, fmt.Errorf("unknown function %q", fc.Name)
	}
}

func (e *Executor) evalID() (interface{}, error) {
	if e.runtime.Mode != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("id() is only available in plugin runtime")
	}
	id := strings.TrimSpace(e.currentInput.SelectedTaskID)
	if id == "" {
		return nil, &MissingSelectedTaskIDError{}
	}
	return id, nil
}

func (e *Executor) evalCount(fc *FunctionCall, allTasks []*task.Task) (interface{}, error) {
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

func (e *Executor) evalNextDate(fc *FunctionCall, t *task.Task, allTasks []*task.Task) (interface{}, error) {
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

func (e *Executor) evalBlocks(fc *FunctionCall, t *task.Task, allTasks []*task.Task) (interface{}, error) {
	val, err := e.evalExpr(fc.Args[0], t, allTasks)
	if err != nil {
		return nil, err
	}
	targetID := strings.ToUpper(normalizeToString(val))

	var blockers []interface{}
	for _, at := range allTasks {
		for _, dep := range at.DependsOn {
			if strings.EqualFold(dep, targetID) {
				blockers = append(blockers, at.ID)
				break
			}
		}
	}
	if blockers == nil {
		blockers = []interface{}{}
	}
	return blockers, nil
}

// --- binary expression evaluation ---

func (e *Executor) evalBinaryExpr(b *BinaryExpr, t *task.Task, allTasks []*task.Task) (interface{}, error) {
	leftVal, err := e.evalExpr(b.Left, t, allTasks)
	if err != nil {
		return nil, err
	}
	rightVal, err := e.evalExpr(b.Right, t, allTasks)
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
			result := make([]interface{}, len(l), len(l)+len(r))
			copy(result, l)
			return append(result, r...), nil
		}
		result := make([]interface{}, len(l), len(l)+1)
		copy(result, l)
		return append(result, right), nil
	}
	return nil, fmt.Errorf("cannot add %T + %T", left, right)
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
	}
	return nil, fmt.Errorf("cannot subtract %T - %T", left, right)
}

// --- sorting ---

func (e *Executor) sortTasks(tasks []*task.Task, clauses []OrderByClause) {
	sort.SliceStable(tasks, func(i, j int) bool {
		for _, c := range clauses {
			vi := extractField(tasks[i], c.Field)
			vj := extractField(tasks[j], c.Field)
			cmp := compareForSort(vi, vj)
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
}

func compareForSort(a, b interface{}) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	switch av := a.(type) {
	case int:
		bv, _ := b.(int)
		return compareInts(av, bv)
	case string:
		bv, _ := b.(string)
		return strings.Compare(av, bv)
	case task.Status:
		bv, _ := b.(task.Status)
		return strings.Compare(string(av), string(bv))
	case task.Type:
		bv, _ := b.(task.Type)
		return strings.Compare(string(av), string(bv))
	case time.Time:
		bv, _ := b.(time.Time)
		if av.Before(bv) {
			return -1
		}
		if av.After(bv) {
			return 1
		}
		return 0
	case task.Recurrence:
		bv, _ := b.(task.Recurrence)
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
		return compareWithNil(left, right, op)
	}

	if leftList, ok := left.([]interface{}); ok {
		if rightList, ok := right.([]interface{}); ok {
			return compareListEquality(leftList, rightList, op)
		}
	}

	if lb, ok := left.(bool); ok {
		if rb, ok := right.(bool); ok {
			return compareBools(lb, rb, op)
		}
	}

	compType := e.resolveComparisonType(leftExpr, rightExpr)

	switch compType {
	case ValueID:
		return compareStringsCI(normalizeToString(left), normalizeToString(right), op)
	case ValueStatus:
		ls := e.normalizeStatusStr(normalizeToString(left))
		rs := e.normalizeStatusStr(normalizeToString(right))
		return compareStrings(ls, rs, op)
	case ValueTaskType:
		ls := e.normalizeTypeStr(normalizeToString(left))
		rs := e.normalizeTypeStr(normalizeToString(right))
		return compareStrings(ls, rs, op)
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
	case task.Status:
		return compareStrings(string(lv), normalizeToString(right), op)
	case task.Type:
		return compareStrings(string(lv), normalizeToString(right), op)
	case task.Recurrence:
		return compareStrings(string(lv), normalizeToString(right), op)
	default:
		return false, fmt.Errorf("unsupported comparison type %T", left)
	}
}

// resolveComparisonType returns the dominant field type for a comparison,
// checking both sides for enum/id fields that need special handling.
func (e *Executor) resolveComparisonType(left, right Expr) ValueType {
	if t := e.exprFieldType(left); t == ValueID || t == ValueStatus || t == ValueTaskType {
		return t
	}
	if t := e.exprFieldType(right); t == ValueID || t == ValueStatus || t == ValueTaskType {
		return t
	}
	return -1
}

func (e *Executor) exprFieldType(expr Expr) ValueType {
	var name string
	switch e := expr.(type) {
	case *FieldRef:
		name = e.Name
	case *QualifiedRef:
		name = e.Name
	case *FunctionCall:
		if e.Name == "id" {
			return ValueID
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

func (e *Executor) normalizeStatusStr(s string) string {
	if norm, ok := e.schema.NormalizeStatus(s); ok {
		return norm
	}
	return s
}

func (e *Executor) normalizeTypeStr(s string) string {
	if norm, ok := e.schema.NormalizeType(s); ok {
		return norm
	}
	return s
}

func compareWithNil(left, right interface{}, op string) (bool, error) {
	// treat nil as empty; treat zero-valued non-nil as also matching empty
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

func compareListEquality(a, b []interface{}, op string) (bool, error) {
	switch op {
	case "=":
		return sortedMultisetEqual(a, b), nil
	case "!=":
		return !sortedMultisetEqual(a, b), nil
	default:
		return false, fmt.Errorf("operator %s not supported for list comparison", op)
	}
}

func sortedMultisetEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	as := toSortedStrings(a)
	bs := toSortedStrings(b)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

func toSortedStrings(list []interface{}) []string {
	s := make([]string, len(list))
	for i, v := range list {
		s[i] = normalizeToString(v)
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

func extractField(t *task.Task, name string) interface{} {
	switch name {
	case "id":
		return t.ID
	case "title":
		return t.Title
	case "description":
		return t.Description
	case "status":
		return t.Status
	case "type":
		return t.Type
	case "priority":
		return t.Priority
	case "points":
		return t.Points
	case "tags":
		return toInterfaceSlice(t.Tags)
	case "dependsOn":
		return toInterfaceSlice(t.DependsOn)
	case "due":
		return t.Due
	case "recurrence":
		return t.Recurrence
	case "assignee":
		return t.Assignee
	case "createdBy":
		return t.CreatedBy
	case "createdAt":
		return t.CreatedAt
	case "updatedAt":
		return t.UpdatedAt
	default:
		return nil
	}
}

// --- helpers ---

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
	case task.Status:
		return string(v)
	case task.Type:
		return string(v)
	case task.Recurrence:
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
	case task.Status:
		return v == ""
	case task.Type:
		return v == ""
	case task.Recurrence:
		return v == ""
	case []interface{}:
		return len(v) == 0
	default:
		return false
	}
}
