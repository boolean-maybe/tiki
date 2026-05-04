package ruki

import (
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/tiki"
)

// tikiFromTask is the Phase 4 test helper: most existing fixtures are
// task.Task{...} slices; wrap them through tiki.FromTask at the executor
// boundary so tests keep reading naturally.
//
// Hand-built fixtures in these tests use the convention "if a schema-known
// typed field is non-zero, the task is workflow-declaring", mirroring the
// pre-Phase-3 behavior of NewTaskTemplate. Mark IsWorkflow on a clone so
// tiki.FromTask's derived-presence branch emits those fields.
func tikiFromTask(t *task.Task) *tiki.Tiki {
	if t == nil {
		return nil
	}
	src := t
	workflow := t.IsWorkflow || hasAnyWorkflowValue(t)
	if workflow && !t.IsWorkflow {
		src = t.Clone()
		src.IsWorkflow = true
	}
	out := tiki.FromTask(src)
	if out == nil {
		return nil
	}
	if !workflow {
		return out
	}
	// Test-only "full-schema presence" convention: a workflow-declaring
	// test fixture carries presence for every schema-known field even
	// when the typed value is the Go zero. This mirrors the pre-Phase-3
	// test ergonomics where `Assignee: ""` counted as "present empty"
	// so `assignee is empty` matched. Tests that specifically want
	// absent-field semantics should skip this helper and build the
	// tiki with tiki.New() directly.
	if !out.Has(tiki.FieldAssignee) {
		out.Set(tiki.FieldAssignee, t.Assignee)
	}
	if !out.Has(tiki.FieldTags) {
		if t.Tags != nil {
			out.Set(tiki.FieldTags, append([]string(nil), t.Tags...))
		} else {
			out.Set(tiki.FieldTags, []string{})
		}
	}
	if !out.Has(tiki.FieldDependsOn) {
		if t.DependsOn != nil {
			out.Set(tiki.FieldDependsOn, append([]string(nil), t.DependsOn...))
		} else {
			out.Set(tiki.FieldDependsOn, []string{})
		}
	}
	if !out.Has(tiki.FieldDue) {
		out.Set(tiki.FieldDue, t.Due)
	}
	if !out.Has(tiki.FieldRecurrence) {
		out.Set(tiki.FieldRecurrence, string(t.Recurrence))
	}
	if !out.Has(tiki.FieldPoints) {
		out.Set(tiki.FieldPoints, t.Points)
	}
	if !out.Has(tiki.FieldPriority) {
		out.Set(tiki.FieldPriority, t.Priority)
	}
	return out
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
// tiki.ToTask consults that global registry and would route these local
// custom fields to UnknownFields — breaking assertions like
// updated.CustomFields["severity"]. The test shim here is deliberately
// more permissive than production ToTask: every non-schema-known Fields
// key lands in CustomFields so assertion ergonomics stay intact. Tests
// that specifically exercise the Custom/Unknown split exercise
// tiki.ToTask directly instead.
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
	t := tiki.ToTask(tk)
	if t == nil {
		return nil
	}
	// Merge UnknownFields into CustomFields for test assertions — see
	// tikisToTasks docstring.
	if len(t.UnknownFields) > 0 {
		if t.CustomFields == nil {
			t.CustomFields = map[string]interface{}{}
		}
		for k, v := range t.UnknownFields {
			t.CustomFields[k] = v
		}
		t.UnknownFields = nil
	}
	return t
}

// testExec is a thin task-shaped wrapper around Executor.Execute: accepts
// []*task.Task for fixture compatibility, converts to tikis for the real
// call, and converts result-bearing variants back so existing test
// assertions keep working. Call sites should treat it as a drop-in
// replacement for the old task-shaped Execute method; tests that need to
// inspect the tiki-shaped result directly should call Execute with
// tikisFromTasks explicitly.
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
		out.Create = &testCreate{Task: tiki.ToTask(r.Create.Tiki)}
	}
	if r.Delete != nil {
		out.Delete = &testDelete{Deleted: tikisToTasks(r.Delete.Deleted)}
	}
	return out
}
