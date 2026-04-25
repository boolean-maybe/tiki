package ruki

import (
	"errors"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/task"
)

// --- validation tests ---

func TestChooseBuiltin_SelectWithWhereFilter_InfersRef(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`update where id = id() set assignee = choose(select where type = "epic")`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vs.UsesChooseBuiltin() {
		t.Fatal("expected UsesChooseBuiltin() = true")
	}
	if vs.ChooseFilter() == nil {
		t.Fatal("expected non-nil ChooseFilter()")
	}
	if vs.ChooseFilter().Where == nil {
		t.Fatal("expected non-nil WHERE in ChooseFilter()")
	}
}

func TestChooseBuiltin_BareSelect_ParsesOK(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`update where id = id() set assignee = choose(select)`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vs.UsesChooseBuiltin() {
		t.Fatal("expected UsesChooseBuiltin() = true")
	}
	if vs.ChooseFilter() == nil {
		t.Fatal("expected non-nil ChooseFilter()")
	}
	if vs.ChooseFilter().Where != nil {
		t.Fatal("expected nil WHERE in ChooseFilter() for bare select")
	}
}

func TestChooseBuiltin_NoArg_Fails(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatement(
		`update where id = id() set assignee = choose()`,
		ExecutorRuntimePlugin,
	)
	if err == nil {
		t.Fatal("expected error for choose() with no arguments")
	}
}

func TestChooseBuiltin_StringArg_Fails(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatement(
		`update where id = id() set assignee = choose("epic")`,
		ExecutorRuntimePlugin,
	)
	if err == nil {
		t.Fatal("expected error for choose() with string argument")
	}
	if !strings.Contains(err.Error(), "subquery") {
		t.Fatalf("expected subquery error, got: %v", err)
	}
}

func TestChooseBuiltin_DuplicateChoose_Fails(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatement(
		`update where id = id() set assignee = choose(select where status = "ready") title = choose(select)`,
		ExecutorRuntimePlugin,
	)
	if err == nil {
		t.Fatal("expected error for duplicate choose()")
	}
	if !strings.Contains(err.Error(), "once") {
		t.Fatalf("expected 'once' in error, got: %v", err)
	}
}

func TestChooseBuiltin_MutualExclusionWithInput_Fails(t *testing.T) {
	p := newTestParser()
	inputType := ValueString
	p.inputType = &inputType
	_, err := p.ParseAndValidateStatement(
		`update where id = id() set assignee = input() title = choose(select)`,
		ExecutorRuntimePlugin,
	)
	if err == nil {
		t.Fatal("expected error for input() + choose() in same statement")
	}
	if !strings.Contains(err.Error(), "cannot be used in the same action") {
		t.Fatalf("expected mutual exclusion error, got: %v", err)
	}
}

func TestChooseBuiltin_HasAnyInteractive(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`update where id = id() set assignee = choose(select)`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vs.HasAnyInteractive() {
		t.Fatal("expected HasAnyInteractive() = true")
	}
}

func TestChooseBuiltin_NoInteractive(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`update where id = id() set status = "done"`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vs.HasAnyInteractive() {
		t.Fatal("expected HasAnyInteractive() = false for non-interactive statement")
	}
}

// --- executor tests ---

func TestChooseBuiltin_Executor_ReturnsValue(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`update where id = id() set assignee = choose(select)`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatal(err)
	}

	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	testTask := &task.Task{ID: "TIKI-000001", Title: "test", Status: "ready", Type: "task", Priority: 3}
	result, err := e.Execute(vs, []*task.Task{testTask}, ExecutionInput{
		SelectedTaskID: "TIKI-000001",
		ChooseValue:    "TIKI-000002",
		HasChoose:      true,
	})
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) == 0 {
		t.Fatal("expected update result")
	}
	if result.Update.Updated[0].Assignee != "TIKI-000002" {
		t.Fatalf("expected assignee=TIKI-000002, got %q", result.Update.Updated[0].Assignee)
	}
}

func TestChooseBuiltin_Executor_MissingChoose(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatement(
		`update where id = id() set assignee = choose(select)`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatal(err)
	}

	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	testTask := &task.Task{ID: "TIKI-000001", Title: "test", Status: "ready", Type: "task", Priority: 3}
	_, err = e.Execute(vs, []*task.Task{testTask}, ExecutionInput{
		SelectedTaskID: "TIKI-000001",
	})
	if err == nil {
		t.Fatal("expected error for missing choose value")
	}
	var missingChoose *MissingChooseValueError
	if !errors.As(err, &missingChoose) {
		t.Fatalf("expected MissingChooseValueError, got %T: %v", err, err)
	}
}

// --- EvalSubQueryFilter tests ---

func TestEvalSubQueryFilter_WithIDExclusion(t *testing.T) {
	p := newTestParser()
	stmt, err := p.ParseStatement(`update where id = id() set assignee = choose(select where id != id())`)
	if err != nil {
		t.Fatal(err)
	}
	// extract the SubQuery from the choose() call
	sq := extractChooseSubQuery(stmt)
	if sq == nil {
		t.Fatal("failed to extract choose subquery")
	}

	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "self", Status: "ready", Type: "task", Priority: 3},
		{ID: "TIKI-000002", Title: "other", Status: "ready", Type: "task", Priority: 3},
		{ID: "TIKI-000003", Title: "third", Status: "ready", Type: "task", Priority: 3},
	}
	candidates, err := e.EvalSubQueryFilter(sq, tasks, ExecutionInput{SelectedTaskID: "TIKI-000001"})
	if err != nil {
		t.Fatalf("filter error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates (excluding self), got %d", len(candidates))
	}
	for _, c := range candidates {
		if c.ID == "TIKI-000001" {
			t.Fatal("context task should be excluded by id != id()")
		}
	}
}

func TestEvalSubQueryFilter_BareSelect_ReturnsAll(t *testing.T) {
	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "a", Status: "ready", Type: "task", Priority: 3},
		{ID: "TIKI-000002", Title: "b", Status: "ready", Type: "task", Priority: 3},
	}
	candidates, err := e.EvalSubQueryFilter(&SubQuery{}, tasks, ExecutionInput{})
	if err != nil {
		t.Fatalf("filter error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates for bare select, got %d", len(candidates))
	}
}

func TestEvalSubQueryFilter_WithOuterSelectedTask(t *testing.T) {
	p := newTestParser()
	stmt, err := p.ParseStatement(`update where id = id() set dependsOn = dependsOn + choose(select where id != outer.id)`)
	if err != nil {
		t.Fatal(err)
	}
	sq := extractChooseSubQuery(stmt)
	if sq == nil {
		t.Fatal("failed to extract choose subquery")
	}

	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "self", Status: "ready", Type: "task", Priority: 3},
		{ID: "TIKI-000002", Title: "other", Status: "ready", Type: "task", Priority: 3},
	}
	candidates, err := e.EvalSubQueryFilter(sq, tasks, ExecutionInput{SelectedTaskID: "TIKI-000001"})
	if err != nil {
		t.Fatalf("filter error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != "TIKI-000002" {
		t.Fatalf("expected only TIKI-000002, got %v", taskIDs(candidates))
	}
}

// --- coerceCustomFieldValue ref test ---

func TestCoerceCustomFieldValue_Ref(t *testing.T) {
	fs := FieldSpec{Name: "epic", Type: ValueRef, Custom: true}
	val, err := coerceCustomFieldValue(fs, "  tiki-abc  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "TIKI-ABC" {
		t.Fatalf("expected TIKI-ABC, got %q", val)
	}
}

func TestCoerceCustomFieldValue_Ref_WrongType(t *testing.T) {
	fs := FieldSpec{Name: "epic", Type: ValueRef, Custom: true}
	_, err := coerceCustomFieldValue(fs, 42)
	if err == nil {
		t.Fatal("expected error for non-string ref value")
	}
}

// --- end-to-end: choose with custom ref field ---

// chooseTestSchema extends testSchema with a custom ref field.
type chooseTestSchema struct {
	testSchema
}

func (chooseTestSchema) Field(name string) (FieldSpec, bool) {
	if name == "epic" {
		return FieldSpec{Name: "epic", Type: ValueRef, Custom: true}, true
	}
	return testSchema{}.Field(name)
}

func TestChooseBuiltin_EndToEnd_CustomRefField(t *testing.T) {
	p := NewParser(chooseTestSchema{})
	vs, err := p.ParseAndValidateStatement(
		`update where id = id() set epic = choose(select where type = "epic")`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	e := NewExecutor(chooseTestSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "target", Status: "ready", Type: "task", Priority: 3},
	}
	result, err := e.Execute(vs, tasks, ExecutionInput{
		SelectedTaskID: "TIKI-000001",
		ChooseValue:    "TIKI-EPIC01",
		HasChoose:      true,
	})
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) == 0 {
		t.Fatal("expected update result")
	}
	updated := result.Update.Updated[0]
	epicVal, ok := updated.CustomFields["epic"]
	if !ok {
		t.Fatal("expected 'epic' in CustomFields")
	}
	if epicVal != "TIKI-EPIC01" {
		t.Fatalf("expected epic=TIKI-EPIC01, got %q", epicVal)
	}
}

// --- trigger blocking tests ---

func TestChooseBuiltin_InEventTrigger_Rejected(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateTrigger(
		`after update update where id = new.id set assignee = choose(select)`,
		ExecutorRuntimeEventTrigger,
	)
	if err == nil {
		t.Fatal("expected error for choose() in event trigger")
	}
	if !strings.Contains(err.Error(), "choose() requires user interaction") {
		t.Fatalf("expected interaction error, got: %v", err)
	}
}

func TestChooseBuiltin_InTimeTrigger_Rejected(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateTimeTrigger(
		`every 1hour update where status = "ready" set assignee = choose(select)`,
		ExecutorRuntimeTimeTrigger,
	)
	if err == nil {
		t.Fatal("expected error for choose() in time trigger")
	}
	if !strings.Contains(err.Error(), "choose() requires user interaction") {
		t.Fatalf("expected interaction error, got: %v", err)
	}
}

func TestChooseBuiltin_InTriggerWhereGuard_Rejected(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateTrigger(
		`after update where new.assignee = choose(select) update where id = new.id set status = "done"`,
		ExecutorRuntimeEventTrigger,
	)
	// choose() in trigger WHERE guard should fail at parse time (subquery not valid in condition context)
	// or at semantic validation (interactive builtin in trigger)
	if err == nil {
		t.Fatal("expected error for choose() in trigger WHERE guard")
	}
}

// --- CLI runtime rejection tests ---

func TestChooseBuiltin_InCLIRuntime_Rejected(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatement(
		`update where status = "backlog" set assignee = choose(select)`,
		ExecutorRuntimeCLI,
	)
	if err == nil {
		t.Fatal("expected error for choose() in CLI runtime")
	}
	if !strings.Contains(err.Error(), "only valid in plugin actions") {
		t.Fatalf("expected plugin-only error, got: %v", err)
	}
}

func TestChooseBuiltin_InPluginRuntime_Accepted(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatement(
		`update where id = id() set assignee = choose(select)`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("unexpected error for choose() in plugin runtime: %v", err)
	}
}

func TestInputBuiltin_InCLIRuntime_AlsoRejected(t *testing.T) {
	p := newTestParser()
	inputType := ValueString
	p.inputType = &inputType
	_, err := p.ParseAndValidateStatement(
		`update where status = "backlog" set assignee = input()`,
		ExecutorRuntimeCLI,
	)
	if err == nil {
		t.Fatal("expected error for input() in CLI runtime")
	}
	if !strings.Contains(err.Error(), "only valid in plugin actions") {
		t.Fatalf("expected plugin-only error, got: %v", err)
	}
}
