package ruki

import (
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/tiki"
)

// tikiFromTask is the Phase 4 test helper: most existing fixtures are
// task.Task{...} slices; wrap them through this helper at the executor
// boundary so tests keep reading naturally.
//
// The convention: any non-zero schema field makes the task workflow-declaring.
// The helper applies full-schema presence — every schema field is set in the
// tiki map — so formatters and filters behave as if the tiki was created via
// NewTikiTemplate (all fields present). Tests that want absent-field semantics
// should skip this helper and build the tiki with tiki.New() directly.
func tikiFromTask(t *task.Task) *tiki.Tiki {
	if t == nil {
		return nil
	}
	tk := tiki.New()
	tk.ID = t.ID
	tk.Title = t.Title
	tk.Body = t.Description
	tk.CreatedAt = t.CreatedAt
	tk.UpdatedAt = t.UpdatedAt
	tk.Path = t.FilePath
	if t.CreatedBy != "" {
		tk.Set("createdBy", t.CreatedBy)
	}

	workflow := t.IsWorkflow || hasAnyWorkflowValue(t)
	if workflow {
		// only set fields that are non-zero, mirroring setWorkflowFieldFromTask
		// behavior: absent zero-values must stay absent so has(field) returns false.
		if t.Status != "" {
			tk.Set(tiki.FieldStatus, string(t.Status))
		}
		if t.Type != "" {
			tk.Set(tiki.FieldType, string(t.Type))
		}
		if t.Priority != 0 {
			tk.Set(tiki.FieldPriority, t.Priority)
		}
		if t.Points != 0 {
			tk.Set(tiki.FieldPoints, t.Points)
		}
		if t.Assignee != "" {
			tk.Set(tiki.FieldAssignee, t.Assignee)
		}
		if !t.Due.IsZero() {
			tk.Set(tiki.FieldDue, t.Due)
		}
		if t.Recurrence != "" {
			tk.Set(tiki.FieldRecurrence, string(t.Recurrence))
		}
		if t.Tags != nil {
			tk.Set(tiki.FieldTags, append([]string(nil), t.Tags...))
		}
		if t.DependsOn != nil {
			tk.Set(tiki.FieldDependsOn, append([]string(nil), t.DependsOn...))
		}
		// full-schema presence: initialize list fields to empty slice when explicitly nil
		// so formatters get "present empty" vs absent for their rendering decisions.
		if !tk.Has(tiki.FieldTags) {
			tk.Set(tiki.FieldTags, []string{})
		}
		if !tk.Has(tiki.FieldDependsOn) {
			tk.Set(tiki.FieldDependsOn, []string{})
		}
		if !tk.Has(tiki.FieldAssignee) {
			tk.Set(tiki.FieldAssignee, t.Assignee) // explicitly-set empty string
		}
		if !tk.Has(tiki.FieldDue) {
			tk.Set(tiki.FieldDue, t.Due) // zero time
		}
		if !tk.Has(tiki.FieldRecurrence) {
			tk.Set(tiki.FieldRecurrence, string(t.Recurrence)) // empty string
		}
		if !tk.Has(tiki.FieldPoints) {
			tk.Set(tiki.FieldPoints, t.Points) // zero int
		}
		if !tk.Has(tiki.FieldPriority) {
			tk.Set(tiki.FieldPriority, t.Priority) // zero int
		}
	}
	for k, v := range t.CustomFields {
		tk.Set(k, v)
	}
	return tk
}

func hasAnyWorkflowValue(t *task.Task) bool {
	if t == nil {
		return false
	}
	return t.Status != "" || t.Type != "" || t.Priority != 0 || t.Points != 0 ||
		t.Tags != nil || t.DependsOn != nil || !t.Due.IsZero() ||
		t.Recurrence != "" || t.Assignee != ""
}

// tikisFromTasks converts a task fixture slice to tikis.
func tikisFromTasks(tasks []*task.Task) []*tiki.Tiki {
	out := make([]*tiki.Tiki, 0, len(tasks))
	for _, t := range tasks {
		if tk := tikiFromTask(t); tk != nil {
			out = append(out, tk)
		}
	}
	return out
}

// testExecAction wraps TriggerExecutor.ExecAction so existing tests continue
// to read task-shaped results.
func (te *TriggerExecutor) testExecAction(trig any, tc *TriggerContext, inputs ...ExecutionInput) (*testResult, error) {
	r, err := te.ExecAction(trig, tc, inputs...)
	if err != nil {
		return nil, err
	}
	return wrapResult(r), nil
}

// testExecTimeAction wraps TriggerExecutor.ExecTimeTriggerAction with
// task-shaped input/output for legacy fixtures.
func (te *TriggerExecutor) testExecTimeAction(tt any, allTasks []*task.Task, inputs ...ExecutionInput) (*testResult, error) {
	r, err := te.ExecTimeTriggerAction(tt, tikisFromTasks(allTasks), inputs...)
	if err != nil {
		return nil, err
	}
	return wrapResult(r), nil
}

// tikisToTasks unwraps tikis back to task.Task for assertion ergonomics.
//
// Tests use local custom schemas (chooseTestSchema, newCustomExecutor's
// registered fields) that are NOT mirrored in the global workflow registry.
// This shim is deliberately more permissive: every non-schema-known field
// lands in CustomFields so assertion ergonomics stay intact.
func tikisToTasks(tks []*tiki.Tiki) []*task.Task {
	out := make([]*task.Task, 0, len(tks))
	for _, tk := range tks {
		if t := tikiToTaskForTest(tk); t != nil {
			out = append(out, t)
		}
	}
	return out
}

func tikiToTaskForTest(tk *tiki.Tiki) *task.Task {
	if tk == nil {
		return nil
	}
	createdBy := ""
	if s, _, _ := tk.StringField("createdBy"); s != "" {
		createdBy = s
	}
	t := &task.Task{
		ID:         tk.ID,
		Title:      tk.Title,
		CreatedBy:  createdBy,
		CreatedAt:  tk.CreatedAt,
		UpdatedAt:  tk.UpdatedAt,
		FilePath:   tk.Path,
		IsWorkflow: false,
	}

	if v, ok, _ := tk.StringField(tiki.FieldStatus); ok {
		t.Status = task.Status(v)
	}
	if v, ok, _ := tk.StringField(tiki.FieldType); ok {
		t.Type = task.Type(v)
	}
	if v, ok, _ := tk.IntField(tiki.FieldPriority); ok {
		t.Priority = v
	}
	if v, ok, _ := tk.IntField(tiki.FieldPoints); ok {
		t.Points = v
	}
	if v, ok, _ := tk.StringField(tiki.FieldAssignee); ok {
		t.Assignee = v
	}
	if v, ok, _ := tk.TimeField(tiki.FieldDue); ok {
		t.Due = v
	}
	if v, ok, _ := tk.StringField(tiki.FieldRecurrence); ok {
		t.Recurrence = task.Recurrence(v)
	}
	if v, ok, _ := tk.StringSliceField(tiki.FieldTags); ok {
		t.Tags = v
	}
	if v, ok, _ := tk.StringSliceField(tiki.FieldDependsOn); ok {
		t.DependsOn = v
	}
	t.Description = tk.Body

	// IsWorkflow mirrors the old ToTask behavior: true when any schema-known
	// field is present in the tiki map.
	for _, f := range tiki.SchemaKnownFields {
		if tk.Has(f) {
			t.IsWorkflow = true
			break
		}
	}

	// collect all non-schema-known fields into CustomFields for test assertions
	schemaSet := make(map[string]bool, len(tiki.SchemaKnownFields))
	for _, f := range tiki.SchemaKnownFields {
		schemaSet[f] = true
	}
	for k, v := range tk.Fields {
		if !schemaSet[k] {
			if t.CustomFields == nil {
				t.CustomFields = map[string]interface{}{}
			}
			t.CustomFields[k] = v
		}
	}
	return t
}

// testExec is a thin task-shaped wrapper around Executor.Execute: accepts
// []*task.Task for fixture compatibility, converts to tikis for the real
// call, and converts result-bearing variants back so existing test
// assertions keep working.
func (e *Executor) testExec(stmt any, tasks []*task.Task, inputs ...ExecutionInput) (*testResult, error) {
	result, err := e.Execute(stmt, tikisFromTasks(tasks), inputs...)
	if err != nil {
		return nil, err
	}
	return wrapResult(result), nil
}

// testResult mirrors Result but exposes task-shaped fields so existing
// assertions that read Select.Tasks / Update.Updated[*].Status /
// Create.Task continue to compile without per-test rewrites.
type testResult struct {
	Select    *testSelect
	Update    *testUpdate
	Create    *testCreate
	Delete    *testDelete
	Pipe      *PipeResult
	Clipboard *ClipboardResult
	Scalar    *ScalarResult

	// raw carries the underlying tiki-shaped result for tests that want
	// to assert on Tiki.Fields directly.
	raw *Result
}

type testSelect struct {
	Tasks  []*task.Task
	Fields []string
}

type testUpdate struct {
	Updated []*task.Task
}

type testCreate struct {
	Task *task.Task
}

type testDelete struct {
	Deleted []*task.Task
}

func wrapResult(r *Result) *testResult {
	if r == nil {
		return nil
	}
	out := &testResult{
		Pipe:      r.Pipe,
		Clipboard: r.Clipboard,
		Scalar:    r.Scalar,
		raw:       r,
	}
	if r.Select != nil {
		out.Select = &testSelect{Tasks: tikisToTasks(r.Select.Tikis), Fields: r.Select.Fields}
	}
	if r.Update != nil {
		out.Update = &testUpdate{Updated: tikisToTasks(r.Update.Updated)}
	}
	if r.Create != nil {
		out.Create = &testCreate{Task: tikiToTaskForTest(r.Create.Tiki)}
	}
	if r.Delete != nil {
		out.Delete = &testDelete{Deleted: tikisToTasks(r.Delete.Deleted)}
	}
	return out
}
