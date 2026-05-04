package ruki

import (
	"errors"
	"testing"

	"github.com/boolean-maybe/tiki/task"
)

func TestInputBuiltin_WithoutDeclaredType_ValidationError(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatement("update where id = id() set assignee = input()", ExecutorRuntimePlugin)
	if err == nil {
		t.Fatal("expected error for input() without declared type")
	}
	if got := err.Error(); got == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestInputBuiltin_StringIntoStringField_OK(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatementWithInput(
		"update where id = id() set assignee = input()",
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vs.UsesInputBuiltin() {
		t.Fatal("expected UsesInputBuiltin() = true")
	}
	if !vs.UsesIDBuiltin() {
		t.Fatal("expected UsesIDBuiltin() = true")
	}
}

func TestInputBuiltin_IntIntoIntField_OK(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatementWithInput(
		"update where id = id() set priority = input()",
		ExecutorRuntimePlugin,
		ValueInt,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInputBuiltin_TypeMismatch_Error(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatementWithInput(
		"update where id = id() set assignee = input()",
		ExecutorRuntimePlugin,
		ValueInt,
	)
	if err == nil {
		t.Fatal("expected type mismatch error for int input into string field")
	}
}

func TestInputBuiltin_DuplicateInput_Error(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatementWithInput(
		"update where id = id() set assignee = input(), title = input()",
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err == nil {
		t.Fatal("expected error for duplicate input()")
	}
}

func TestInputBuiltin_WithArguments_Error(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatementWithInput(
		`update where id = id() set assignee = input("x")`,
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err == nil {
		t.Fatal("expected error for input() with arguments")
	}
}

func TestInputBuiltin_Executor_ReturnsValue(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatementWithInput(
		"update where id = id() set assignee = input()",
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err != nil {
		t.Fatal(err)
	}

	e := NewExecutor(testSchema{}, func() string { return "alice" }, ExecutorRuntime{Mode: ExecutorRuntimePlugin})

	testTask := &task.Task{ID: "TIKI-000001", Title: "test", Status: "ready", Type: "task", Priority: 3}
	result, err := e.testExec(vs, []*task.Task{testTask}, ExecutionInput{
		SelectedTaskIDs: []string{"TIKI-000001"},
		InputValue:      "bob",
		HasInput:        true,
	})
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}
	if result.Update == nil {
		t.Fatal("expected update result")
	}
	if len(result.Update.Updated) == 0 {
		t.Fatal("expected at least one updated task")
	}
	updated := result.Update.Updated[0]
	if updated.Assignee != "bob" {
		t.Fatalf("expected assignee = bob, got %v", updated.Assignee)
	}
}

func TestInputBuiltin_Executor_MissingInput(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatementWithInput(
		"update where id = id() set assignee = input()",
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err != nil {
		t.Fatal(err)
	}

	e := NewExecutor(testSchema{}, func() string { return "alice" }, ExecutorRuntime{Mode: ExecutorRuntimePlugin})

	testTask := &task.Task{ID: "TIKI-000001", Title: "test", Status: "ready", Type: "task", Priority: 3}
	_, err = e.testExec(vs, []*task.Task{testTask}, ExecutionInput{
		SelectedTaskIDs: []string{"TIKI-000001"},
	})
	if err == nil {
		t.Fatal("expected error for missing input")
	}
	var missingInput *MissingInputValueError
	if !errors.As(err, &missingInput) {
		t.Fatalf("expected MissingInputValueError, got %T: %v", err, err)
	}
}

func TestInputBuiltin_InWhereClause_Detected(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatementWithInput(
		`update where assignee = input() set status = "ready"`,
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vs.UsesInputBuiltin() {
		t.Fatal("expected UsesInputBuiltin() = true for input() in where clause")
	}
}

func TestInputBuiltin_DuplicateAcrossWhereAndSet(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatementWithInput(
		"update where assignee = input() set title = input()",
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err == nil {
		t.Fatal("expected error for duplicate input() across where and set")
	}
}

func TestInputBuiltin_InSelectWhere_Detected(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatementWithInput(
		`select where assignee = input()`,
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vs.UsesInputBuiltin() {
		t.Fatal("expected UsesInputBuiltin() = true for input() in select where")
	}
}

func TestInputBuiltin_InDeleteWhere_Detected(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatementWithInput(
		`delete where assignee = input()`,
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vs.UsesInputBuiltin() {
		t.Fatal("expected UsesInputBuiltin() = true for input() in delete where")
	}
}

func TestInputBuiltin_InSubquery_Detected(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatementWithInput(
		`select where count(select where assignee = input()) >= 1`,
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vs.UsesInputBuiltin() {
		t.Fatal("expected UsesInputBuiltin() = true for input() inside subquery")
	}
}

func TestInputBuiltin_DuplicateAcrossWhereAndSubquery(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatementWithInput(
		`select where assignee = input() and count(select where assignee = input()) >= 1`,
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err == nil {
		t.Fatal("expected error for duplicate input() across where and subquery")
	}
}

func TestInputBuiltin_InPipeCommand_Detected(t *testing.T) {
	p := newTestParser()
	vs, err := p.ParseAndValidateStatementWithInput(
		`select id where id = id() | run(input())`,
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vs.UsesInputBuiltin() {
		t.Fatal("expected UsesInputBuiltin() = true for input() in pipe command")
	}
}

func TestInputBuiltin_DuplicateAcrossWhereAndPipe(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatementWithInput(
		`select id where assignee = input() | run(input())`,
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err == nil {
		t.Fatal("expected error for duplicate input() across where and pipe")
	}
}

func TestInputBuiltin_InputTypeNotLeaked(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseAndValidateStatementWithInput(
		"update where id = id() set assignee = input()",
		ExecutorRuntimePlugin,
		ValueString,
	)
	if err != nil {
		t.Fatal(err)
	}

	// after the call, inputType should be cleared — next parse without input should fail
	_, err = p.ParseAndValidateStatement("update where id = id() set assignee = input()", ExecutorRuntimePlugin)
	if err == nil {
		t.Fatal("expected error: inputType should not leak across calls")
	}
}
