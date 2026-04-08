package ruki

import (
	"errors"
	"strings"
	"testing"
)

func TestExecuteRawStatementRejectsCallBeforeEvaluation(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where 1 = 2 and call("echo hello") = "x"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = e.Execute(stmt, makeTasks())
	if err == nil {
		t.Fatal("expected semantic validation error")
	}
	if !strings.Contains(err.Error(), "call() is not supported yet") {
		t.Fatalf("expected call() semantic validation error, got: %v", err)
	}
}

func TestExecuteRawStatementRejectsIDOutsidePluginRuntime(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where 1 = 2 and id() = "TIKI-000001"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = e.Execute(stmt, makeTasks())
	if err == nil {
		t.Fatal("expected semantic validation error")
	}
	if !strings.Contains(err.Error(), "id() is only available in plugin runtime") {
		t.Fatalf("expected id() runtime error, got: %v", err)
	}
}

func TestExecuteValidatedStatementRuntimeMismatch(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	validated, err := p.ParseAndValidateStatement(`select`, ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	_, err = e.Execute(validated, makeTasks())
	if err == nil {
		t.Fatal("expected runtime mismatch error")
	}
	var mismatch *RuntimeMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected RuntimeMismatchError, got: %v", err)
	}
}

func TestExecuteUnsealedValidatedStatementRejected(t *testing.T) {
	e := newTestExecutor()
	unsealed := &ValidatedStatement{
		statement: &Statement{Select: &SelectStmt{}},
	}

	_, err := e.Execute(unsealed, makeTasks())
	if err == nil {
		t.Fatal("expected unvalidated wrapper error")
	}
	var unvalidated *UnvalidatedWrapperError
	if !errors.As(err, &unvalidated) {
		t.Fatalf("expected UnvalidatedWrapperError, got: %v", err)
	}
}

func TestExecuteValidatedCreateRequiresTemplate(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	validated, err := p.ParseAndValidateStatement(`create title="x"`, ExecutorRuntimeCLI)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	_, err = e.Execute(validated, nil)
	if err == nil {
		t.Fatal("expected missing create template error")
	}
	var missing *MissingCreateTemplateError
	if !errors.As(err, &missing) {
		t.Fatalf("expected MissingCreateTemplateError, got: %v", err)
	}
}

func TestExecutePluginIDRequiresSelectedTaskID(t *testing.T) {
	p := newTestParser()
	e := NewExecutor(testSchema{}, func() string { return "alice" }, ExecutorRuntime{Mode: ExecutorRuntimePlugin})

	validated, err := p.ParseAndValidateStatement(`select where id() = "TIKI-000001"`, ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	_, err = e.Execute(validated, makeTasks())
	if err == nil {
		t.Fatal("expected missing selected task id error")
	}
	var missing *MissingSelectedTaskIDError
	if !errors.As(err, &missing) {
		t.Fatalf("expected MissingSelectedTaskIDError, got: %v", err)
	}
}

func TestValidatedTriggerCloneIsolated(t *testing.T) {
	p := newTestParser()

	validated, err := p.ParseAndValidateTrigger(`before create deny "blocked"`, ExecutorRuntimeEventTrigger)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	clone := validated.TriggerClone()
	if clone == nil {
		t.Fatal("expected non-nil trigger clone")
	}

	clone.Timing = "after"
	clone.Event = "delete"
	clone.Deny = nil

	after := validated.TriggerClone()
	if after == nil {
		t.Fatal("expected non-nil trigger clone after mutation")
	}
	if after.Timing != "before" || after.Event != "create" {
		t.Fatalf("validated trigger was mutated: timing=%q event=%q", after.Timing, after.Event)
	}
	if after.Deny == nil || *after.Deny != "blocked" {
		t.Fatalf("expected deny message to remain unchanged, got %#v", after.Deny)
	}
}

func TestValidatedTimeTriggerCloneIsolated(t *testing.T) {
	p := newTestParser()

	validated, err := p.ParseAndValidateTimeTrigger(`every 2day create title="x"`, ExecutorRuntimeTimeTrigger)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	clone := validated.TimeTriggerClone()
	if clone == nil {
		t.Fatal("expected non-nil time trigger clone")
	}

	clone.Interval = DurationLiteral{Value: 9, Unit: "week"}
	clone.Action = nil

	after := validated.TimeTriggerClone()
	if after == nil {
		t.Fatal("expected non-nil time trigger clone after mutation")
	}
	if after.Interval.Value != 2 || after.Interval.Unit != "day" {
		t.Fatalf("validated time trigger interval was mutated: %+v", after.Interval)
	}
	if after.Action == nil || after.Action.Create == nil {
		t.Fatal("expected action to remain unchanged")
	}
}
