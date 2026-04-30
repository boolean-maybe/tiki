package ruki

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/task"
	collectionutil "github.com/boolean-maybe/tiki/util/collections"
	"github.com/boolean-maybe/tiki/util/duration"
)

// Executor evaluates parsed ruki statements against a set of tasks.
type Executor struct {
	schema       Schema
	userFunc     func() string
	runtime      ExecutorRuntime
	currentInput ExecutionInput
}

type evalContext struct {
	current  *task.Task
	outer    *task.Task
	allTasks []*task.Task
}

func (ctx evalContext) withCurrent(current *task.Task) evalContext {
	ctx.current = current
	return ctx
}

// NewExecutor constructs an Executor with the given schema and user function.
// If userFunc is nil, calling user() at runtime will return an error.
func NewExecutor(schema Schema, userFunc func() string, runtime ExecutorRuntime) *Executor {
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
		if e.runtime.Mode == ExecutorRuntimePlugin &&
			(validated.usesIDFunc || validated.usesTargetQualifier) {
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
		return e.executeSelect(rawStmt.Select, tasks)
	case rawStmt.Create != nil:
		return e.executeCreate(rawStmt.Create, tasks, requiresCreateTemplate)
	case rawStmt.Update != nil:
		return e.executeUpdate(rawStmt.Update, tasks)
	case rawStmt.Delete != nil:
		return e.executeDelete(rawStmt.Delete, tasks)
	case rawStmt.Expr != nil:
		return e.executeExpr(rawStmt.Expr, tasks)
	default:
		return nil, fmt.Errorf("empty statement")
	}
}

func (e *Executor) executeExpr(es *ExprStmt, tasks []*task.Task) (*Result, error) {
	val, err := e.evalExpr(es.Expr, evalContext{allTasks: tasks})
	if err != nil {
		return nil, err
	}
	return &Result{Scalar: &ScalarResult{Value: val, Type: es.Type}}, nil
}

func (e *Executor) executeSelect(sel *SelectStmt, tasks []*task.Task) (*Result, error) {
	filtered, err := e.filterTasks(sel.Where, tasks)
	if err != nil {
		return nil, err
	}

	if len(sel.OrderBy) > 0 {
		e.sortTasks(filtered, sel.OrderBy)
	}

	if sel.Limit != nil && *sel.Limit < len(filtered) {
		filtered = filtered[:*sel.Limit]
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
	cmdVal, err := e.evalExpr(pipe.Command, evalContext{allTasks: allTasks})
	if err != nil {
		return nil, fmt.Errorf("pipe command: %w", err)
	}
	cmdStr, ok := cmdVal.(string)
	if !ok {
		return nil, fmt.Errorf("pipe command must evaluate to string, got %T", cmdVal)
	}

	rows := e.buildFieldRows(fields, matched)
	return &Result{Pipe: &PipeResult{Command: cmdStr, Rows: rows}}, nil
}

func (e *Executor) buildClipboardResult(fields []string, matched []*task.Task) (*Result, error) {
	rows := e.buildFieldRows(fields, matched)
	return &Result{Clipboard: &ClipboardResult{Rows: rows}}, nil
}

// buildFieldRows extracts the requested fields from matched tasks as string rows.
// Shared by both run() and clipboard() pipe targets.
func (e *Executor) buildFieldRows(fields []string, matched []*task.Task) [][]string {
	rows := make([][]string, len(matched))
	for i, t := range matched {
		row := make([]string, len(fields))
		for j, f := range fields {
			row[j] = pipeArgString(e.extractField(t, f))
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
			val, err := e.evalExpr(a.Value, evalContext{current: clone, allTasks: tasks})
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
		val, err := e.evalExpr(a.Value, evalContext{current: t, allTasks: tasks})
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
	case "id", "createdBy", "createdAt", "updatedAt", "filepath":
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
		promoteToWorkflow(t, "status")

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
		promoteToWorkflow(t, "type")

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
		promoteToWorkflow(t, "priority")

	case "points":
		if val == nil {
			t.Points = 0
			promoteToWorkflow(t, "points")
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
		promoteToWorkflow(t, "points")

	case "tags":
		if val == nil {
			t.Tags = nil
			promoteToWorkflow(t, "tags")
			return nil
		}
		t.Tags = collectionutil.NormalizeStringSet(toStringSlice(val))
		promoteToWorkflow(t, "tags")

	case "dependsOn":
		if val == nil {
			t.DependsOn = nil
			promoteToWorkflow(t, "dependsOn")
			return nil
		}
		refs := normalizeRefList(toStringSlice(val))
		if err := validateBareRefs(refs, "dependsOn"); err != nil {
			return err
		}
		t.DependsOn = refs
		promoteToWorkflow(t, "dependsOn")

	case "due":
		if val == nil {
			t.Due = time.Time{}
			promoteToWorkflow(t, "due")
			return nil
		}
		d, ok := val.(time.Time)
		if !ok {
			return fmt.Errorf("due must be a date, got %T", val)
		}
		t.Due = d
		promoteToWorkflow(t, "due")

	case "recurrence":
		if val == nil {
			t.Recurrence = ""
			promoteToWorkflow(t, "recurrence")
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
		promoteToWorkflow(t, "recurrence")

	case "assignee":
		if val == nil {
			t.Assignee = ""
			promoteToWorkflow(t, "assignee")
			return nil
		}
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("assignee must be a string, got %T", val)
		}
		t.Assignee = s
		promoteToWorkflow(t, "assignee")

	default:
		fs, ok := e.schema.Field(name)
		if !ok || !fs.Custom {
			return fmt.Errorf("unknown field %q", name)
		}
		if val == nil {
			delete(t.CustomFields, name)
			return nil
		}
		coerced, err := coerceCustomFieldValue(fs, val)
		if err != nil {
			return fmt.Errorf("field %q: %w", name, err)
		}
		if t.CustomFields == nil {
			t.CustomFields = make(map[string]interface{})
		}
		t.CustomFields[name] = coerced
	}
	return nil
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
	case ValueTimestamp:
		tv, ok := val.(time.Time)
		if !ok {
			return nil, fmt.Errorf("expected time.Time, got %T", val)
		}
		return tv, nil
	case ValueEnum:
		s, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", val)
		}
		for _, av := range fs.AllowedValues {
			if strings.EqualFold(av, s) {
				return av, nil
			}
		}
		return nil, fmt.Errorf("invalid enum value %q", s)
	case ValueListString:
		return collectionutil.NormalizeStringSet(toStringSlice(val)), nil
	case ValueListRef:
		refs := normalizeRefList(toStringSlice(val))
		if err := validateBareRefs(refs, fs.Name); err != nil {
			return nil, err
		}
		return refs, nil
	case ValueRef:
		s, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", val)
		}
		ref := strings.ToUpper(strings.TrimSpace(s))
		if !document.IsValidID(ref) {
			return nil, fmt.Errorf("%s reference %q is not a bare document id (expected %d uppercase alphanumeric chars)", fs.Name, ref, document.IDLength)
		}
		return ref, nil
	default:
		return nil, fmt.Errorf("unsupported custom field type")
	}
}

// normalizeRefList applies set-like normalization for document ID references.
func normalizeRefList(ss []string) []string {
	return collectionutil.NormalizeRefSet(ss)
}

// validateBareRefs rejects any entry that is not a bare document ID.
// fieldName is used in the error message so callers can distinguish dependsOn
// from custom ref fields.
func validateBareRefs(refs []string, fieldName string) error {
	for _, r := range refs {
		if !document.IsValidID(r) {
			return fmt.Errorf("%s reference %q is not a bare document id (expected %d uppercase alphanumeric chars)", fieldName, r, document.IDLength)
		}
	}
	return nil
}

// promoteToWorkflow marks a plain document as workflow-capable after a
// workflow-setting edit and records the assigned field in the presence
// map whenever recording it is load-bearing for save fidelity. Callers
// must invoke this from every setField branch that handles a workflow
// field (status, type, tags, dependsOn, due, recurrence, assignee,
// priority, points). Without promotion at each workflow-field
// assignment, the save path takes the plain-doc branch and silently
// drops the assigned value — data loss.
//
// The seeding decision depends on both wasPlain (did IsWorkflow flip?)
// and the presence-map state, giving three load-bearing cases. The
// discriminator for WHETHER to seed is actually "does the presence map
// already exist?", not wasPlain alone — see case 3 for the subtlety.
//
//  1. Plain doc getting a workflow field for the first time —
//     wasPlain=true, WorkflowFrontmatter nil. Seed the presence map so
//     the save path takes the SPARSE branch and writes only the keys the
//     caller set. Otherwise `update set points = 0` on a plain doc would
//     fall through to the full-schema synthesizer and materialize every
//     workflow field with registry defaults.
//
//  2. Fresh workflow create template — wasPlain=false,
//     WorkflowFrontmatter nil. The template carries creation defaults
//     (type, priority, points, custom field defaults) that belong on disk
//     for a newly-created document. marshalFrontmatter reads the nil
//     presence map as "write the FULL schema with all defaults". Seeding
//     presence here would flip the save into sparse mode and drop every
//     default the caller did not explicitly assign. Don't.
//
//  3. Loaded sparse workflow doc being edited — wasPlain=false,
//     WorkflowFrontmatter non-nil. The load path populated the map from
//     the source YAML (e.g. a file that only wrote `status:` produces
//     WorkflowFrontmatter={status}). Adding a new field via
//     `update set points = 0` must ALSO seed presence: without it, both
//     the presence map and MergeTypedWorkflowDeltas skip zero/empty
//     values, and the sparse save path writes only the original keys —
//     losing the user's explicit `points: 0` or `dependsOn: []`
//     assignment. Since the map is already sparse, adding a key keeps
//     the save in sparse mode (good) and ensures the assigned value
//     lands on disk.
//
// fieldName must be a workflow key (status/type/tags/dependsOn/due/
// recurrence/assignee/priority/points); passing anything else is a
// programming error and will still promote the task but not seed presence.
func promoteToWorkflow(t *task.Task, fieldName string) {
	if t == nil {
		return
	}
	wasPlain := !t.IsWorkflow
	t.IsWorkflow = true
	if fieldName == "" {
		return
	}
	// Case 2: already-workflow task with a nil presence map is a create
	// template — leave the map nil so the save path writes the full
	// workflow schema with all defaults.
	if !wasPlain && t.WorkflowFrontmatter == nil {
		return
	}
	if t.WorkflowFrontmatter == nil {
		// Case 1: first-time promotion of a plain doc. Allocate the map
		// so the save path switches to sparse mode and writes only the
		// keys the caller set.
		t.WorkflowFrontmatter = make(map[string]interface{})
	}
	// Cases 1 and 3: record the assigned field. Store a sentinel value —
	// marshalSparseWorkflowFrontmatter reads the CURRENT typed field, not
	// the value in this map, so we only need to record presence. An
	// empty string works for any key because marshalWorkflowField never
	// consults the map value.
	if _, exists := t.WorkflowFrontmatter[fieldName]; !exists {
		t.WorkflowFrontmatter[fieldName] = ""
	}
}

// isWorkflowFieldRef reports whether expr is a reference to a built-in
// workflow field whose presence is tracked by Phase 5 semantics. Combined
// with a nil runtime value at the call site, this is the "absent workflow
// field" signal used by predicate short-circuiting so `where dependsOn =
// []` and `where tags is empty` do not match plain documents.
//
// Scoping is intentional: custom user fields keep the older "nil == empty"
// semantics so that `where flag is empty` continues to match tasks that
// never set `flag`. Built-in identity/audit fields (id, title, createdAt,
// etc.) are always present on a loaded task and therefore not included.
// Both bare (`dependsOn`) and qualified (`old.dependsOn`, `new.tags`)
// references count.
func isWorkflowFieldRef(expr Expr) bool {
	var name string
	switch e := expr.(type) {
	case *FieldRef:
		name = e.Name
	case *QualifiedRef:
		name = e.Name
	default:
		return false
	}
	switch name {
	case "status", "type", "priority", "points", "tags",
		"dependsOn", "due", "recurrence", "assignee":
		return true
	}
	return false
}

// hasWorkflowField reports whether the task carries an explicit workflow
// value for the named field, implementing Phase 5 presence-aware semantics.
//
// Source of truth order:
//  1. WorkflowFrontmatter map — authoritative for store-loaded tasks, records
//     exactly which YAML keys were present on disk.
//  2. Fallback for in-memory tasks (tests, ruki create, hand-built fixtures):
//     the field counts as present iff the typed value is non-zero. This
//     matches user intent for callers that construct Tasks directly with
//     typed fields: if they set t.Priority = 3, they meant to set priority.
//
// The net effect for plain documents: a doc loaded with no workflow
// frontmatter returns nil for every workflow field, so predicates like
// `where priority = 0` do not accidentally match it. A doc loaded with
// `priority: 0` explicitly written returns int(0), so `where priority = 0`
// matches as the author intended.
func hasWorkflowField(t *task.Task, name string) bool {
	if t == nil {
		return false
	}
	if t.WorkflowFrontmatter != nil {
		_, ok := t.WorkflowFrontmatter[name]
		return ok
	}
	switch name {
	case "status":
		return t.Status != ""
	case "type":
		return t.Type != ""
	case "priority":
		return t.Priority != 0
	case "points":
		return t.Points != 0
	case "tags":
		return len(t.Tags) > 0
	case "dependsOn":
		return len(t.DependsOn) > 0
	case "due":
		return !t.Due.IsZero()
	case "recurrence":
		return t.Recurrence != ""
	case "assignee":
		return t.Assignee != ""
	}
	return false
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
		match, err := e.evalCondition(where, evalContext{current: t, allTasks: tasks})
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

func (e *Executor) evalCompare(c *CompareExpr, ctx evalContext) (bool, error) {
	leftVal, err := e.evalExpr(c.Left, ctx)
	if err != nil {
		return false, err
	}
	rightVal, err := e.evalExpr(c.Right, ctx)
	if err != nil {
		return false, err
	}
	return e.compareValues(leftVal, rightVal, c.Op, c.Left, c.Right)
}

func (e *Executor) evalIsEmpty(c *IsEmptyExpr, ctx evalContext) (bool, error) {
	val, err := e.evalExpr(c.Expr, ctx)
	if err != nil {
		return false, err
	}
	// Phase 5: an absent workflow field is not "empty" — it has no value
	// at all. Both `dependsOn is empty` and `dependsOn is not empty` on a
	// plain document return false so the caller must use has(dependsOn)
	// for explicit presence checks. Only field refs resolving to nil get
	// this short-circuit; a bare list literal or other expression that
	// evaluates to nil still participates in normal is-empty semantics.
	if val == nil && isWorkflowFieldRef(c.Expr) {
		return false, nil
	}
	empty := isZeroValue(val)
	if c.Negated {
		return !empty, nil
	}
	return empty, nil
}

func (e *Executor) evalIn(c *InExpr, ctx evalContext) (bool, error) {
	val, err := e.evalExpr(c.Value, ctx)
	if err != nil {
		return false, err
	}
	collVal, err := e.evalExpr(c.Collection, ctx)
	if err != nil {
		return false, err
	}

	// Phase 5: if either side references an absent workflow field, the
	// whole predicate evaluates false — for both `in` AND `not in`. This
	// closes the hole where `where assignee not in ["bob"]` matched plain
	// documents (absent assignee) by falling through to the
	// `return c.Negated` branches. has(<field>) is the only way to ask
	// "is this field present?". Custom field refs and string-literal
	// operands still follow their prior semantics below — the scoping
	// via isWorkflowFieldRef is what preserves backwards compatibility
	// for custom fields.
	leftAbsent := val == nil && isWorkflowFieldRef(c.Value)
	rightAbsent := collVal == nil && isWorkflowFieldRef(c.Collection)
	if leftAbsent || rightAbsent {
		return false, nil
	}

	// list membership mode
	if list, ok := collVal.([]interface{}); ok {
		// unset non-workflow value on the left (e.g. a custom field) is
		// not a member of any list — keep the pre-Phase-5 behavior.
		if val == nil {
			return c.Negated, nil
		}
		valStr := normalizeToString(val)
		// use case-insensitive comparison for enum-like fields
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

	// Non-workflow nil collection (e.g. absent custom field) — keep the
	// pre-Phase-5 c.Negated fallback so `custom not in [...]` on an
	// unset custom field matches.
	if collVal == nil {
		return c.Negated, nil
	}

	return false, fmt.Errorf("in: collection is not a list or string")
}

func (e *Executor) evalQuantifier(q *QuantifierExpr, ctx evalContext) (bool, error) {
	listVal, err := e.evalExpr(q.Expr, ctx)
	if err != nil {
		return false, err
	}
	// Phase 5: an absent list workflow field resolves to nil. A quantifier
	// over an absent field returns false for BOTH `any` and `all` — the
	// vacuous-truth shortcut for `all` intentionally does NOT apply when
	// the list is absent rather than empty, because "predicates on absent
	// fields evaluate false except explicit absence checks" (the
	// has(<field>) predicate is the escape hatch).
	if listVal == nil && isWorkflowFieldRef(q.Expr) {
		return false, nil
	}
	refs, ok := listVal.([]interface{})
	if !ok {
		return false, fmt.Errorf("quantifier: expression is not a list")
	}

	// find referenced tasks
	refTasks := make([]*task.Task, 0, len(refs))
	for _, ref := range refs {
		refID := normalizeToString(ref)
		for _, at := range ctx.allTasks {
			if strings.EqualFold(at.ID, refID) {
				refTasks = append(refTasks, at)
				break
			}
		}
	}

	switch q.Kind {
	case "any":
		for _, rt := range refTasks {
			match, err := e.evalCondition(q.Condition, ctx.withCurrent(rt))
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
			match, err := e.evalCondition(q.Condition, ctx.withCurrent(rt))
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

func (e *Executor) evalExpr(expr Expr, ctx evalContext) (interface{}, error) {
	switch expr := expr.(type) {
	case *FieldRef:
		return e.extractField(ctx.current, expr.Name), nil
	case *QualifiedRef:
		switch expr.Qualifier {
		case "outer":
			if ctx.outer == nil {
				return nil, fmt.Errorf("outer.%s is not available outside a subquery", expr.Name)
			}
			return e.extractField(ctx.outer, expr.Name), nil
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
	case "now":
		return time.Now(), nil
	case "user":
		if e.userFunc == nil {
			return nil, fmt.Errorf("user() is unavailable (no current user configured)")
		}
		return e.userFunc(), nil
	case "count":
		return e.evalCount(fc, ctx.current, ctx.allTasks)
	case "exists":
		return e.evalExists(fc, ctx.current, ctx.allTasks)
	case "has":
		return e.evalHas(fc, ctx)
	case "next_date":
		return e.evalNextDate(fc, ctx)
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
// when the referenced task has an explicit value for the named workflow
// field, false otherwise. Unlike equality comparisons which silently
// return false on absent fields, has() lets a caller distinguish "present
// with zero value" from "absent entirely" — the primary use case is ruki
// queries that want to surface only workflow documents (e.g. where
// has(status)).
//
// Argument contract: a single bare or qualified field reference. The
// runtime resolves the same qualifier set as ordinary field references
// in each context:
//   - bare has(X) → current row (the task ruki is iterating over, or
//     the subquery candidate)
//   - has(outer.X) → the parent-query row (only inside a subquery body)
//   - has(target.X) → the exactly-one selected task (plugin runtime)
//   - has(targets.X) → true iff ANY selected task has the field present
//     (plugin runtime)
//   - has(new.X) / has(old.X) → handled by the trigger executor
//     override; the base executor is never in a trigger context so
//     reaching those qualifiers here is a programming error
//
// String literals and other expression kinds are rejected at validation
// time so they never reach this function.
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
		return hasWorkflowField(ctx.current, name), nil
	case "outer":
		if ctx.outer == nil {
			return nil, fmt.Errorf("has(outer.%s) is not available outside a subquery", name)
		}
		return hasWorkflowField(ctx.outer, name), nil
	case "target":
		// Reuse evalTargetField's runtime-mode and selection checks so
		// errors are identical to those a bare target.X would raise.
		// The field value it returns is nil iff the field is absent on
		// the selected task — which is exactly hasWorkflowField's
		// answer for scalars. For robustness against the list case
		// (where an absent list still returns nil via the extractField
		// change), look up the task and consult hasWorkflowField
		// directly after the policy/selection gates pass.
		if e.runtime.Mode != ExecutorRuntimePlugin {
			return nil, fmt.Errorf("has(target.%s): target. qualifier is only available in plugin runtime", name)
		}
		if err := checkSingleSelectionForID(e.currentInput); err != nil {
			return nil, err
		}
		id, _ := e.currentInput.SingleSelectedTaskID()
		t, ok := findTaskByID(ctx.allTasks, id)
		if !ok {
			return nil, fmt.Errorf("has(target.%s): selected task %q not found", name, id)
		}
		return hasWorkflowField(t, name), nil
	case "targets":
		// has(targets.X) returns true iff ANY selected task has field X
		// present. Mirrors evalTargetsField's runtime-mode gate. Zero
		// selected tasks → false (no task to have the field).
		if e.runtime.Mode != ExecutorRuntimePlugin {
			return nil, fmt.Errorf("has(targets.%s): targets. qualifier is only available in plugin runtime", name)
		}
		selectedIDs := e.currentInput.SelectedTaskIDList()
		for _, id := range selectedIDs {
			t, ok := findTaskByID(ctx.allTasks, id)
			if !ok {
				return nil, fmt.Errorf("has(targets.%s): selected task %q not found", name, id)
			}
			if hasWorkflowField(t, name) {
				return true, nil
			}
		}
		return false, nil
	case "new", "old":
		// new./old. are only evaluable in trigger contexts, where the
		// trigger executor override intercepts has() before the base
		// runs. Reaching this branch means the validator let a trigger-
		// only qualifier through in a non-trigger context, which is a
		// programming error — surface it clearly rather than silently
		// returning false.
		return nil, fmt.Errorf("has(%s.%s): %s. qualifier is only available in trigger guards and actions", qualifier, name, qualifier)
	default:
		return nil, fmt.Errorf("has(%s.%s): unknown qualifier %q", qualifier, name, qualifier)
	}
}

func (e *Executor) evalID() (interface{}, error) {
	if e.runtime.Mode != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("id() is only available in plugin runtime")
	}
	if err := checkSingleSelectionForID(e.currentInput); err != nil {
		return nil, err
	}
	id, _ := e.currentInput.SingleSelectedTaskID()
	return id, nil
}

func (e *Executor) evalIDs() (interface{}, error) {
	if e.runtime.Mode != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("ids() is only available in plugin runtime")
	}
	selected := e.currentInput.SelectedTaskIDList()
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

// evalTargetField evaluates target.<field>. It enforces the same exactly-one
// selection contract as id() and extracts the named field from the selected task.
func (e *Executor) evalTargetField(name string, ctx evalContext) (interface{}, error) {
	if e.runtime.Mode != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("target. qualifier is only available in plugin runtime")
	}
	if err := checkSingleSelectionForID(e.currentInput); err != nil {
		return nil, err
	}
	id, _ := e.currentInput.SingleSelectedTaskID()
	t, ok := findTaskByID(ctx.allTasks, id)
	if !ok {
		return nil, fmt.Errorf("target.%s: selected task %q not found", name, id)
	}
	return e.extractField(t, name), nil
}

// evalTargetsField evaluates targets.<field>. It projects the named field
// across all selected tasks, flattens list-valued fields, and deduplicates
// while preserving first-seen order (by selection order, then field value
// order within a task).
func (e *Executor) evalTargetsField(name string, ctx evalContext) (interface{}, error) {
	if e.runtime.Mode != ExecutorRuntimePlugin {
		return nil, fmt.Errorf("targets. qualifier is only available in plugin runtime")
	}
	selectedIDs := e.currentInput.SelectedTaskIDList()
	if len(selectedIDs) == 0 {
		return []interface{}{}, nil
	}
	result := make([]interface{}, 0, len(selectedIDs))
	seen := make(map[string]struct{}, len(selectedIDs))
	for _, id := range selectedIDs {
		t, ok := findTaskByID(ctx.allTasks, id)
		if !ok {
			return nil, fmt.Errorf("targets.%s: selected task %q not found", name, id)
		}
		val := e.extractField(t, name)
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

// findTaskByID returns the task with the given id from the list (case-insensitive).
func findTaskByID(tasks []*task.Task, id string) (*task.Task, bool) {
	for _, t := range tasks {
		if strings.EqualFold(t.ID, id) {
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

// checkSingleSelectionForID enforces the scalar id() contract: exactly one
// selected task id. Zero yields MissingSelectedTaskIDError; more than one
// yields AmbiguousSelectedTaskIDError.
func checkSingleSelectionForID(in ExecutionInput) error {
	count := in.SelectionCount()
	switch {
	case count == 0:
		return &MissingSelectedTaskIDError{}
	case count > 1:
		return &AmbiguousSelectedTaskIDError{Count: count}
	}
	return nil
}

func (e *Executor) evalCount(fc *FunctionCall, parent *task.Task, allTasks []*task.Task) (interface{}, error) {
	sq, ok := fc.Args[0].(*SubQuery)
	if !ok {
		return nil, fmt.Errorf("count() argument must be a select subquery")
	}
	if sq.Where == nil {
		return len(allTasks), nil
	}
	count := 0
	for _, t := range allTasks {
		match, err := e.evalCondition(sq.Where, evalContext{current: t, outer: parent, allTasks: allTasks})
		if err != nil {
			return nil, err
		}
		if match {
			count++
		}
	}
	return count, nil
}

func (e *Executor) evalExists(fc *FunctionCall, parent *task.Task, allTasks []*task.Task) (interface{}, error) {
	sq, ok := fc.Args[0].(*SubQuery)
	if !ok {
		return nil, fmt.Errorf("exists() argument must be a select subquery")
	}
	if sq.Where == nil {
		return len(allTasks) > 0, nil
	}
	for _, t := range allTasks {
		match, err := e.evalCondition(sq.Where, evalContext{current: t, outer: parent, allTasks: allTasks})
		if err != nil {
			return nil, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

// EvalSubQueryFilter evaluates a subquery WHERE clause against a set of tasks,
// returning the matching tasks. Used by the controller to build candidate lists
// for choose() before showing the picker.
func (e *Executor) EvalSubQueryFilter(sq *SubQuery, tasks []*task.Task, input ExecutionInput, parents ...*task.Task) ([]*task.Task, error) {
	e.currentInput = input
	defer func() { e.currentInput = ExecutionInput{} }()

	if sq == nil || sq.Where == nil {
		result := make([]*task.Task, len(tasks))
		copy(result, tasks)
		return result, nil
	}
	parent := chooseFilterParent(tasks, input, parents...)
	var result []*task.Task
	for _, t := range tasks {
		match, err := e.evalCondition(sq.Where, evalContext{current: t, outer: parent, allTasks: tasks})
		if err != nil {
			return nil, err
		}
		if match {
			result = append(result, t)
		}
	}
	return result, nil
}

func chooseFilterParent(tasks []*task.Task, input ExecutionInput, parents ...*task.Task) *task.Task {
	if len(parents) > 0 {
		return parents[0]
	}
	// outer parent is only well-defined for exactly-one selection;
	// multi-select has no single "current" task to bind against.
	selected, ok := input.SingleSelectedTaskID()
	if !ok {
		return nil
	}
	for _, t := range tasks {
		if strings.EqualFold(t.ID, selected) {
			return t
		}
	}
	return nil
}

func (e *Executor) evalNextDate(fc *FunctionCall, ctx evalContext) (interface{}, error) {
	val, err := e.evalExpr(fc.Args[0], ctx)
	if err != nil {
		return nil, err
	}
	// Phase 5: absent recurrence propagates as nil so `is not empty` filters
	// out documents without recurrence rather than surfacing a type error.
	if val == nil {
		return nil, nil
	}
	rec, ok := val.(task.Recurrence)
	if !ok {
		return nil, fmt.Errorf("next_date() argument must be a recurrence value")
	}
	return task.NextOccurrence(rec), nil
}

func (e *Executor) evalBlocks(fc *FunctionCall, ctx evalContext) (interface{}, error) {
	val, err := e.evalExpr(fc.Args[0], ctx)
	if err != nil {
		return nil, err
	}
	targetID := strings.ToUpper(normalizeToString(val))

	var blockers []interface{}
	for _, at := range ctx.allTasks {
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

func (e *Executor) evalBinaryExpr(b *BinaryExpr, ctx evalContext) (interface{}, error) {
	leftVal, err := e.evalExpr(b.Left, ctx)
	if err != nil {
		return nil, err
	}
	rightVal, err := e.evalExpr(b.Right, ctx)
	if err != nil {
		return nil, err
	}

	// Phase 5: absent list fields resolve to nil in predicate contexts, but
	// arithmetic contexts (`dependsOn + "X"`, `tags - ["a"]`) construct a
	// NEW list and the absence of a left-hand list should be treated as
	// "start from empty, then add/remove". The assignment target then
	// triggers promotion via setField, so the resulting write records the
	// explicit new value. Without this coercion, `set dependsOn =
	// dependsOn + "X"` on a plain document would fail with "cannot add
	// nil + string" — breaking one of the main promotion idioms.
	leftVal = e.coerceAbsentListForArithmetic(b.Left, leftVal)
	rightVal = e.coerceAbsentListForArithmetic(b.Right, rightVal)

	switch b.Op {
	case "+":
		return addValues(leftVal, rightVal)
	case "-":
		return subtractValues(leftVal, rightVal)
	default:
		return nil, fmt.Errorf("unknown binary operator %q", b.Op)
	}
}

// coerceAbsentListForArithmetic replaces a nil result from a field-ref
// evaluation with an empty []interface{} when the target field is list-
// valued. Scalar field refs that resolve to nil stay nil so their existing
// arithmetic errors surface unchanged. Custom list fields are included via
// schema lookup so `set myRefs = myRefs + [...]` also works when myRefs
// was absent.
func (e *Executor) coerceAbsentListForArithmetic(expr Expr, val interface{}) interface{} {
	if val != nil {
		return val
	}
	var name string
	switch ex := expr.(type) {
	case *FieldRef:
		name = ex.Name
	case *QualifiedRef:
		name = ex.Name
	default:
		return val
	}
	if name == "tags" || name == "dependsOn" {
		return []interface{}{}
	}
	if fs, ok := e.schema.Field(name); ok {
		if fs.Type == ValueListString || fs.Type == ValueListRef {
			return []interface{}{}
		}
	}
	return val
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
	}
	return nil, fmt.Errorf("cannot add %T + %T", left, right)
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
	}
	return nil, fmt.Errorf("cannot subtract %T - %T", left, right)
}

// --- sorting ---

func (e *Executor) sortTasks(tasks []*task.Task, clauses []OrderByClause) {
	sort.SliceStable(tasks, func(i, j int) bool {
		for _, c := range clauses {
			vi := e.extractField(tasks[i], c.Field)
			vj := e.extractField(tasks[j], c.Field)
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
	// Phase 5 semantics: absent values sort AFTER present ones for ascending,
	// so a nil on the left is "greater" than a present value. Descending
	// naturally inverts via c.Desc in sortTasks, so absent values appear at
	// the END of ascending sorts and the BEGINNING of descending sorts —
	// matching the plan's deterministic-placement rule.
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
			return -1 // false < true
		}
		return 1
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
	// Phase 5: a comparison involving an absent workflow field evaluates
	// false for every operator — the plan's rule is "predicates on absent
	// fields evaluate false except explicit absence checks". That covers
	// `=`, `!=`, `<`, `>`, and every other comparison op. The has(<field>)
	// predicate is the only way to ask "is this present". This also closes
	// the hole where `where dependsOn = []` on a plain document matched
	// because EmptyLiteral → nil and the compareWithNil branch treated
	// nil-vs-EmptyLiteral as both-empty.
	leftAbsent := left == nil && isWorkflowFieldRef(leftExpr)
	rightAbsent := right == nil && isWorkflowFieldRef(rightExpr)
	if leftAbsent || rightAbsent {
		return false, nil
	}
	if left == nil || right == nil {
		return compareWithNil(left, right, op, leftExpr, rightExpr)
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
		// resilience: coerce string-encoded bool on right side
		if rs, ok := right.(string); ok {
			if rb, err := parseBoolString(rs); err == nil {
				return compareBools(lb, rb, op)
			}
		}
	}
	if rb, ok := right.(bool); ok {
		// resilience: coerce string-encoded bool on left side
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
	case ValueStatus:
		ls := e.normalizeStatusStr(normalizeToString(left))
		rs := e.normalizeStatusStr(normalizeToString(right))
		return compareStrings(ls, rs, op)
	case ValueTaskType:
		ls := e.normalizeTypeStr(normalizeToString(left))
		rs := e.normalizeTypeStr(normalizeToString(right))
		return compareStrings(ls, rs, op)
	case ValueEnum:
		ls := strings.ToLower(normalizeToString(left))
		rs := strings.ToLower(normalizeToString(right))
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
	if t := e.exprFieldType(left); t == ValueID || t == ValueStatus || t == ValueTaskType || t == ValueEnum {
		return t
	}
	if t := e.exprFieldType(right); t == ValueID || t == ValueStatus || t == ValueTaskType || t == ValueEnum {
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

// isEnumLikeField returns true for field types that use case-insensitive
// comparison in equality checks and should also use it for in/not-in.
// Includes ValueBool so that "True"/"true"/"TRUE" all match in bool in-lists.
func isEnumLikeField(t ValueType) bool {
	return t == ValueEnum || t == ValueStatus || t == ValueTaskType || t == ValueID || t == ValueBool
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

func compareWithNil(left, right interface{}, op string, leftExpr, rightExpr Expr) (bool, error) {
	// when comparing against EmptyLiteral, use zero-value semantics:
	// nil and typed zeros both count as "empty"
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

	// concrete comparison: nil (unset field) only equals nil
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

func (e *Executor) extractField(t *task.Task, name string) interface{} {
	switch name {
	case "id":
		return t.ID
	case "title":
		return t.Title
	case "description":
		return t.Description
	case "status":
		if !hasWorkflowField(t, "status") {
			return nil
		}
		return t.Status
	case "type":
		if !hasWorkflowField(t, "type") {
			return nil
		}
		return t.Type
	case "priority":
		if !hasWorkflowField(t, "priority") {
			return nil
		}
		return t.Priority
	case "points":
		if !hasWorkflowField(t, "points") {
			return nil
		}
		return t.Points
	case "tags":
		// Phase 5 list semantics: return nil (not []) for absent list
		// workflow fields so predicates like `tags is empty`,
		// `tags = []`, and `all tags ...` evaluate false on plain
		// documents instead of treating the absent field as a present
		// empty list. Presence is exposed via has(tags). Predicate sites
		// that receive nil treat it as "predicate false except for the
		// explicit absence-check path" — see evalIsEmpty, evalQuantifier,
		// and compareValues.
		if !hasWorkflowField(t, "tags") {
			return nil
		}
		return toInterfaceSlice(t.Tags)
	case "dependsOn":
		if !hasWorkflowField(t, "dependsOn") {
			return nil
		}
		return toInterfaceSlice(t.DependsOn)
	case "due":
		if !hasWorkflowField(t, "due") {
			return nil
		}
		return t.Due
	case "recurrence":
		if !hasWorkflowField(t, "recurrence") {
			return nil
		}
		return t.Recurrence
	case "assignee":
		if !hasWorkflowField(t, "assignee") {
			return nil
		}
		return t.Assignee
	case "createdBy":
		return t.CreatedBy
	case "createdAt":
		return t.CreatedAt
	case "updatedAt":
		return t.UpdatedAt
	case "filepath":
		return t.FilePath
	default:
		fs, ok := e.schema.Field(name)
		if !ok || !fs.Custom {
			return nil
		}
		if t.CustomFields != nil {
			if v, exists := t.CustomFields[name]; exists {
				if fs.Type == ValueListString || fs.Type == ValueListRef {
					if ss, ok := v.([]string); ok {
						return toInterfaceSlice(ss)
					}
				}
				return v
			}
		}
		// unset custom field: list types return empty list (consistent
		// with built-in tags/dependsOn), scalars return nil
		if fs.Type == ValueListString || fs.Type == ValueListRef {
			return []interface{}{}
		}
		return nil
	}
}

// --- helpers ---

// parseBoolString converts a string "true"/"false" (case-insensitive) to a bool.
// Returns an error for any other string.
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
