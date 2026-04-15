package ruki

import (
	"strings"
	"testing"
)

func TestValidation_TypeMismatch(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"priority equals string",
			`select where priority = "high"`,
			"cannot compare",
		},
		{
			"string field ordered compare",
			`select where status < "done"`,
			"operator < not supported",
		},
		{
			"int to string assignment",
			`create title="x" priority="high"`,
			"cannot assign string to int field",
		},
		{
			"string to int field",
			`create title="x" points="five"`,
			"cannot assign string to int field",
		},
		{
			"int to string field",
			`create title="x" assignee=42`,
			"cannot assign int to string field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_UnknownField(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"unknown field in where", `select where foo = "bar"`, "unknown field"},
		{"unknown field in assignment", `create title="x" foo="bar"`, "unknown field"},
		{"unknown qualified field in statement", `select where old.foo = "bar"`, "old. qualifier is not valid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_UnknownFunction(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`select where foo(1) = 1`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown function") {
		t.Fatalf("expected unknown function error, got: %v", err)
	}
}

func TestValidation_FunctionArgCount(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"now with args", `select where now(1) = now()`},
		{"count no args", `select where count() >= 1`},
		{"user with args", `select where user(1) = "bob"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "argument") {
				t.Fatalf("expected argument count error, got: %v", err)
			}
		})
	}
}

func TestValidation_QuantifierRequiresListRef(t *testing.T) {
	p := newTestParser()

	// tags is list<string>, not list<ref>
	_, err := p.ParseStatement(`select where tags any status = "done"`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "requires list<ref>") {
		t.Fatalf("expected list<ref> error, got: %v", err)
	}
}

func TestValidation_CountRequiresSubquery(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`select where count(1) >= 3`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "subquery") {
		t.Fatalf("expected subquery error, got: %v", err)
	}
}

func TestValidation_UnknownStatus(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`select where status = "nonexistent"`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown status") {
		t.Fatalf("expected unknown status error, got: %v", err)
	}
}

func TestValidation_UnknownType(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`select where type = "nonexistent"`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("expected unknown type error, got: %v", err)
	}
}

func TestValidation_ValidStatusAndType(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"valid status", `select where status = "done"`},
		{"valid status alias", `select where status = "in progress"`},
		{"valid type", `select where type = "bug"`},
		{"valid type alias", `select where type = "feature"`},
		{"valid status in assignment", `create title="x" status="done"`},
		{"valid type in assignment", `create title="x" type="bug"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_BinaryExprTypes(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"date minus date yields duration — assigned to date is wrong",
			`create title="x" due=2026-03-25 - 2026-03-20`,
			"cannot assign duration to date field",
		},
		{
			"string minus string",
			`create title="x" - "y"`,
			"cannot subtract",
		},
		{
			"int plus string",
			`create title="x" priority=1 + "a"`,
			"cannot add",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_ValidBinaryExprTypes(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"string concat", `create title="hello" + " world"`},
		{"list append", `create title="x" tags=tags + ["new"]`},
		{"list remove", `create title="x" tags=tags - ["old"]`},
		{"date plus duration", `create title="x" due=2026-03-25 + 2day`},
		{"date minus duration", `create title="x" due=2026-03-25 - 1week`},
		{"list ref append", `create title="x" dependsOn=dependsOn + ["TIKI-ABC123"]`},
		{"list ref remove", `create title="x" dependsOn=dependsOn - ["TIKI-ABC123"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_RunCommandMustBeString(t *testing.T) {
	p := newTestParser()

	// int expression in run() should be rejected
	_, err := p.ParseTrigger(`after update run(1 + 2)`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "must be string") {
		t.Fatalf("expected string type error, got: %v", err)
	}

	// valid: string expression in run()
	_, err = p.ParseTrigger(`after update run("echo hello")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_EmptyAssignments(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"empty to string field", `create title="x" assignee=empty`},
		{"empty to list field", `create title="x" tags=empty`},
		{"empty to date field", `create title="x" due=empty`},
		{"empty to int field", `create title="x" priority=empty`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_IsEmptyOnAllTypes(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"string is empty", `select where assignee is empty`},
		{"list is empty", `select where tags is empty`},
		{"date is empty", `select where due is empty`},
		{"int is empty", `select where priority is empty`},
		{"string is not empty", `select where title is not empty`},
		{"function result is empty", `select where blocks(id) is empty`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_SelectNotAllowedAsTriggerAction(t *testing.T) {
	p := newTestParser()

	// select is rejected at parse level since the action grammar doesn't include it
	_, err := p.ParseTrigger(`after update select`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestValidation_FunctionArgTypes(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"blocks with non-id arg",
			`select where blocks(priority) is empty`,
			"blocks() argument must be an id or ref",
		},
		{
			"call with non-string arg",
			`create title=call(42)`,
			"call() argument must be string",
		},
		{
			"next_date with non-recurrence arg",
			`create title="x" due=next_date(42)`,
			"next_date() argument must be recurrence",
		},
		{
			"next_date with string field arg",
			`create title="x" due=next_date(title)`,
			"next_date() argument must be recurrence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_ValidFunctionUsages(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"blocks with id field", `select where blocks(id) is empty`},
		{"blocks with id ref", `select where blocks("TIKI-ABC123") is empty`},
		{"call with string", `create title=call("echo hi")`},
		{"user", `select where assignee = user()`},
		{"now", `select where updatedAt < now()`},
		{"count with subquery", `select where count(select where status = "done") >= 1`},
		{"count with bare select", `select where count(select) >= 0`},
		{"next_date with recurrence field", `create title="x" due=next_date(recurrence)`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_InExprTypes(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"string in list of ints — element type mismatch",
			`select where "bug" in [1, 2]`,
			"element type mismatch",
		},
		{
			"int in int — not a list or string",
			`select where 1 in priority`,
			"cannot check",
		},
		{
			"int in string — not string value",
			`select where 1 in title`,
			"cannot check",
		},
		{
			"string in status — substring requires string not enum",
			`select where "done" in status`,
			"cannot check",
		},
		{
			"string in id — substring requires string not id",
			`select where "x" in id`,
			"cannot check",
		},
		{
			"status in string — substring requires string not enum",
			`select where status in "hello"`,
			"cannot check",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_EnumInList(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"invalid status in list",
			`select where status in ["done", "bogus"]`,
			"unknown status",
		},
		{
			"invalid type in list",
			`select where type in ["bug", "bogus"]`,
			"unknown type",
		},
		{
			"all invalid statuses in list",
			`select where status in ["nope", "nada"]`,
			"unknown status",
		},
		{
			"invalid status in not-in list",
			`select where status not in ["done", "bogus"]`,
			"unknown status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_ValidInExpr(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"string in tags field", `select where "bug" in tags`},
		{"id in dependsOn field", `select where id in dependsOn`},
		{"status in list", `select where status in ["done", "cancelled"]`},
		{"status not in list", `select where status not in ["done"]`},
		{"int in list", `select where priority in [1, 2, 3]`},
		{"string in string field — substring", `select where "d" in title`},
		{"string in assignee — substring", `select where "x" in assignee`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_TimestampArithmetic(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"timestamp plus duration", `select where updatedAt < now() + 1day`},
		{"timestamp minus duration", `select where updatedAt > now() - 1week`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_EmptyComparisons(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"field equals empty", `select where assignee = empty`},
		{"empty not equal field", `select where priority != empty`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_UnknownStatusInAssignment(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`create title="x" status="nonexistent"`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown status") {
		t.Fatalf("expected unknown status error, got: %v", err)
	}
}

func TestValidation_UnknownTypeInAssignment(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`create title="x" type="nonexistent"`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("expected unknown type error, got: %v", err)
	}
}

func TestValidation_StatusOnLeftSide(t *testing.T) {
	p := newTestParser()

	// status literal on the left side of comparison
	_, err := p.ParseStatement(`select where "nonexistent" = status`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown status") {
		t.Fatalf("expected unknown status error, got: %v", err)
	}
}

func TestValidation_TypeOnLeftSide(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`select where "nonexistent" = type`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("expected unknown type error, got: %v", err)
	}
}

func TestValidation_DurationCompare(t *testing.T) {
	p := newTestParser()

	// duration supports ordering operators
	_, err := p.ParseStatement(`select where updatedAt - createdAt > 7day`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_ListHomogeneity(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`select where status in ["done", 1]`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "list elements must be the same type") {
		t.Fatalf("expected homogeneity error, got: %v", err)
	}
}

func TestValidation_NestedConditions(t *testing.T) {
	p := newTestParser()

	// exercise not + or paths
	tests := []struct {
		name  string
		input string
	}{
		{"not with or", `select where not (status = "done" or priority = 1)`},
		{"double not", `select where not not status = "done"`},
		{"or chain", `select where status = "done" or status = "ready" or status = "backlog"`},
		{"and chain", `select where priority = 1 and status = "done" and assignee = "bob"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_TriggerCreateAction(t *testing.T) {
	p := newTestParser()

	// after-trigger with create action
	_, err := p.ParseTrigger(`after update where new.status = "done" create title="follow-up" priority=3`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_TriggerDeleteAction(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseTrigger(`after update where new.status = "done" delete where id = old.id`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_ParenExpr(t *testing.T) {
	p := newTestParser()

	// parenthesized expression
	_, err := p.ParseStatement(`create title="x" priority=(1 + 2)`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_MoreBinaryExprErrors(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"date plus date",
			`select where due = 2026-03-25 + 2026-03-20`,
			"cannot add",
		},
		{
			"int minus string",
			`create title="x" priority=1 - "a"`,
			"cannot subtract",
		},
		{
			"duration minus duration",
			`create title="x" due=1day - 2day`,
			"cannot subtract",
		},
		{
			"list ordered compare",
			`select where tags < ["a"]`,
			"operator < not supported",
		},
		{
			"recurrence ordered compare",
			`select where recurrence < recurrence`,
			"operator < not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_TimestampMinusTimestamp(t *testing.T) {
	p := newTestParser()

	// timestamp - timestamp = duration; comparing to duration
	_, err := p.ParseStatement(`select where updatedAt - createdAt > 1day`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_EmptyInBinaryExpr(t *testing.T) {
	p := newTestParser()

	// empty + empty — should resolve but might fail on the operator
	_, err := p.ParseStatement(`create title="x" tags=empty + empty`)
	// this may error — just exercise the code path
	_ = err
}

func TestValidation_DateCompareOps(t *testing.T) {
	p := newTestParser()

	ops := []string{"=", "!=", "<", ">", "<=", ">="}
	for _, op := range ops {
		t.Run(op, func(t *testing.T) {
			input := `select where due ` + op + ` 2026-03-25`
			_, err := p.ParseStatement(input)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", op, err)
			}
		})
	}
}

func TestValidation_IntCompareOps(t *testing.T) {
	p := newTestParser()

	ops := []string{"=", "!=", "<", ">", "<=", ">="}
	for _, op := range ops {
		t.Run(op, func(t *testing.T) {
			input := `select where priority ` + op + ` 3`
			_, err := p.ParseStatement(input)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", op, err)
			}
		})
	}
}

func TestValidation_StringCompareOps(t *testing.T) {
	p := newTestParser()

	// equality should work
	_, err := p.ParseStatement(`select where title = "hello"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// inequality should work
	_, err = p.ParseStatement(`select where title != "hello"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ordering should fail
	_, err = p.ParseStatement(`select where title < "hello"`)
	if err == nil {
		t.Fatal("expected error for title < string")
	}
}

func TestValidation_IDCompare(t *testing.T) {
	p := newTestParser()

	// id equality
	_, err := p.ParseStatement(`select where id = "TIKI-ABC123"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// id ordering should fail
	_, err = p.ParseStatement(`select where id < "TIKI-ABC123"`)
	if err == nil {
		t.Fatal("expected error for id < string")
	}
}

func TestValidation_BareSubqueryRejected(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"subquery in where comparison",
			`select where select = 1`,
			"subquery",
		},
		{
			"subquery in create assignment",
			`create title=select`,
			"subquery",
		},
		{
			"subquery in update assignment",
			`update where status = "done" set title=select`,
			"subquery",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_QualifiedRefInStatement(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"old ref in select",
			`select where old.status = "done"`,
			"old.",
		},
		{
			"new ref in select",
			`select where new.status = "done"`,
			"new.",
		},
		{
			"old ref in create",
			`create title=old.title`,
			"old.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_EnumAssignmentStrictness(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"string field to status",
			`create title="x" status=title`,
			"cannot assign string to status",
		},
		{
			"id field to status",
			`create title="x" status=id`,
			"cannot assign id to status",
		},
		{
			"string field to type",
			`create title="x" type=title`,
			"cannot assign string to type",
		},
		{
			"status field to string",
			`update where id="x" set title=status`,
			"cannot assign status to string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_InExprStrictTypes(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"id in tags",
			`select where id in tags`,
			"element type mismatch",
		},
		{
			"status in tags",
			`select where status in tags`,
			"element type mismatch",
		},
		{
			"status in dependsOn",
			`select where status in dependsOn`,
			"element type mismatch",
		},
		{
			"type in tags",
			`select where type in tags`,
			"element type mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_ListRefOperandStrictness(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"list ref plus status",
			`create title="x" dependsOn=dependsOn + status`,
			"cannot add",
		},
		{
			"list ref plus type",
			`create title="x" dependsOn=dependsOn + type`,
			"cannot add",
		},
		{
			"list ref minus status",
			`create title="x" dependsOn=dependsOn - status`,
			"cannot subtract",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_QualifiedRefInTrigger(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"old ref in after create guard",
			`after create where old.status = "done" update where id = new.id set status="done"`,
			"old.",
		},
		{
			"new ref in before delete",
			`before delete where new.status = "done" deny "x"`,
			"new.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseTrigger(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_DependsOnListLiteralAssignment(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"single ref", `create title="x" dependsOn=["TIKI-ABC123"]`},
		{"multiple refs", `create title="x" dependsOn=["TIKI-ABC123", "TIKI-DEF456"]`},
		{"update set dependsOn", `update where id="TIKI-1" set dependsOn=["TIKI-ABC123"]`},
		{"empty list", `create title="x" dependsOn=[]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_ListStringRejectsNonStringElements(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"int elements in tags",
			`create title="x" tags=[1, 2]`,
			"cannot assign",
		},
		{
			"date elements in tags",
			`create title="x" tags=[2026-03-25]`,
			"cannot assign",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_ListRefRejectsListStringField(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"list ref plus list string field",
			`create title="x" dependsOn=dependsOn + tags`,
			"cannot add list<string> field to list<ref>",
		},
		{
			"list ref minus list string field",
			`create title="x" dependsOn=dependsOn - tags`,
			"cannot subtract list<string> field from list<ref>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}

	// regression: list literals still allowed with list<ref>
	valid := []struct {
		name  string
		input string
	}{
		{"list ref plus list ref field", `create title="x" dependsOn=dependsOn + dependsOn`},
		{"list ref plus string literal list", `create title="x" dependsOn=dependsOn + ["TIKI-ABC123"]`},
		{"list ref minus string literal list", `create title="x" dependsOn=dependsOn - ["TIKI-ABC123"]`},
	}

	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_CountSubqueryValidated(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"unknown field in count subquery",
			`select where count(select where nosuchfield = "x") >= 1`,
			"unknown field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}

	// valid: count with valid subquery still works
	_, err := p.ParseStatement(`select where count(select where status = "done") >= 1`)
	if err != nil {
		t.Fatalf("unexpected error for valid count subquery: %v", err)
	}

	// valid: count subquery can reference new. in trigger context (parameterized query)
	_, err = p.ParseTrigger(`before update where new.status = "in progress" and count(select where assignee = new.assignee and status = "in progress") >= 3 deny "WIP limit"`)
	if err != nil {
		t.Fatalf("unexpected error for count subquery with new.: %v", err)
	}
}

func TestValidation_QuantifierNoQualifiers(t *testing.T) {
	p := newTestParser()

	// old. inside quantifier body should be rejected even in update trigger
	_, err := p.ParseTrigger(`before update where new.dependsOn any old.status = "done" deny "blocked"`)
	if err == nil {
		t.Fatal("expected error for old. in quantifier, got nil")
	}
	if !strings.Contains(err.Error(), "old.") {
		t.Fatalf("expected old. qualifier error, got: %v", err)
	}

	// new. inside quantifier body should also be rejected
	_, err = p.ParseTrigger(`before update where new.dependsOn any new.status = "done" deny "blocked"`)
	if err == nil {
		t.Fatal("expected error for new. in quantifier, got nil")
	}
	if !strings.Contains(err.Error(), "new.") {
		t.Fatalf("expected new. qualifier error, got: %v", err)
	}

	// unqualified field inside quantifier should still work
	_, err = p.ParseTrigger(`before update where new.dependsOn any status = "done" deny "blocked"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_QualifiedRefValidInTrigger(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{
			"old and new in before update",
			`before update where old.status = "in progress" and new.status = "done" deny "skip"`,
		},
		{
			"new in after create",
			`after create where new.priority <= 2 update where id = new.id set assignee="bob"`,
		},
		{
			"old in after delete",
			`after delete update where old.id in dependsOn set dependsOn=dependsOn - [old.id]`,
		},
		{
			"old and new in after update",
			`after update where new.status = "done" update where id = old.id set recurrence=empty`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseTrigger(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_ListRefRejectsStringFields(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"list ref plus title field",
			`create title="x" dependsOn=dependsOn + title`,
			"cannot add",
		},
		{
			"list ref plus assignee field",
			`create title="x" dependsOn=dependsOn + assignee`,
			"cannot add",
		},
		{
			"list ref minus title field",
			`create title="x" dependsOn=dependsOn - title`,
			"cannot subtract",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}

	// string literals should still be allowed
	valid := []struct {
		name  string
		input string
	}{
		{"list ref plus string literal", `create title="x" dependsOn=dependsOn + "TIKI-ABC123"`},
		{"list ref minus string literal", `create title="x" dependsOn=dependsOn - "TIKI-ABC123"`},
	}

	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_CompareEnumStrictness(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"status equals title field",
			`select where status = title`,
			"cannot compare",
		},
		{
			"status equals id field",
			`select where status = id`,
			"cannot compare",
		},
		{
			"status equals type field",
			`select where status = type`,
			"cannot compare",
		},
		{
			"type equals assignee field",
			`select where type = assignee`,
			"cannot compare",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}

	// these must remain valid
	valid := []struct {
		name  string
		input string
	}{
		{"status equals string literal", `select where status = "done"`},
		{"type equals string literal", `select where type = "bug"`},
		{"id equals string literal", `select where id = "TIKI-ABC123"`},
		{"string field equals string literal", `select where assignee = "alice"`},
	}

	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_BlocksRejectsStringFields(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"blocks with title field",
			`select where blocks(title) is empty`,
			"blocks() argument must be an id or ref",
		},
		{
			"blocks with assignee field",
			`select where blocks(assignee) is empty`,
			"blocks() argument must be an id or ref",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}

	// these must remain valid
	valid := []struct {
		name  string
		input string
	}{
		{"blocks with id field", `select where blocks(id) is empty`},
		{"blocks with string literal", `select where blocks("TIKI-ABC123") is empty`},
	}

	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_DuplicateAssignments(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"duplicate in create",
			`create title="a" title="b"`,
			"duplicate assignment",
		},
		{
			"duplicate in update set",
			`update where id="x" set status="ready" status="done"`,
			"duplicate assignment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_EnumInRejectsFieldRefs(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"status in list containing field ref",
			`select where status in [title]`,
			"element type mismatch",
		},
		{
			"type in list containing field ref",
			`select where type in [assignee]`,
			"element type mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}

	// string literals should still be allowed
	valid := []struct {
		name  string
		input string
	}{
		{"status in string literal list", `select where status in ["done", "ready"]`},
		{"type in string literal list", `select where type in ["bug", "epic"]`},
	}

	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidation_SelectFields(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"unknown field", "select foo", `unknown field "foo" in select`},
		{"duplicate field", "select title, title", `duplicate field "title" in select`},
		{"unknown among valid", "select title, foo", `unknown field "foo" in select`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_OrderBy(t *testing.T) {
	p := newTestParser()

	t.Run("valid cases", func(t *testing.T) {
		valid := []struct {
			name  string
			input string
		}{
			{"int field", "select order by priority"},
			{"date field", "select order by due"},
			{"timestamp field", "select order by createdAt desc"},
			{"string field", "select order by title asc"},
			{"status field", "select order by status"},
			{"type field", "select order by type desc"},
			{"id field", "select order by id"},
			{"multiple fields", "select order by priority desc, createdAt"},
			{"with where", `select where status = "done" order by priority`},
		}
		for _, tt := range valid {
			t.Run(tt.name, func(t *testing.T) {
				_, err := p.ParseStatement(tt.input)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		}
	})

	t.Run("invalid cases", func(t *testing.T) {
		invalid := []struct {
			name    string
			input   string
			wantErr string
		}{
			{
				"unknown field",
				"select order by nonexistent",
				"unknown field",
			},
			{
				"list<string> not orderable",
				"select order by tags",
				"cannot order by",
			},
			{
				"list<ref> not orderable",
				"select order by dependsOn",
				"cannot order by",
			},
			{
				"recurrence not orderable",
				"select order by recurrence",
				"cannot order by",
			},
			{
				"duplicate field",
				"select order by priority, priority desc",
				"duplicate field",
			},
		}
		for _, tt := range invalid {
			t.Run(tt.name, func(t *testing.T) {
				_, err := p.ParseStatement(tt.input)
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
			})
		}
	})
}

func TestValidation_OrderByInSubquery(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`select where count(select where status = "done" order by priority) > 0`)
	if err == nil {
		t.Fatal("expected error for order by inside subquery")
	}
	if !strings.Contains(err.Error(), "order by is not valid inside a subquery") {
		t.Fatalf("expected subquery error, got: %v", err)
	}
}

func TestValidation_ListAssignmentRejectsFieldRefs(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"tags with field ref element",
			`create title="x" tags=["bug", id]`,
			"cannot assign",
		},
		{
			"dependsOn with non-literal element",
			`create title="x" dependsOn=["TIKI-ABC123", title]`,
			"cannot assign",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_TypeNameCoverage(t *testing.T) {
	tests := []struct {
		typ  ValueType
		want string
	}{
		{ValueString, "string"},
		{ValueInt, "int"},
		{ValueDate, "date"},
		{ValueTimestamp, "timestamp"},
		{ValueDuration, "duration"},
		{ValueBool, "bool"},
		{ValueID, "id"},
		{ValueRef, "ref"},
		{ValueRecurrence, "recurrence"},
		{ValueListString, "list<string>"},
		{ValueListRef, "list<ref>"},
		{ValueStatus, "status"},
		{ValueTaskType, "type"},
		{-1, "empty"},
		{ValueType(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := typeName(tt.typ)
			if got != tt.want {
				t.Errorf("typeName(%d) = %q, want %q", tt.typ, got, tt.want)
			}
		})
	}
}

func TestValidation_ResolveEmptyPairBothEmpty(t *testing.T) {
	a, b := resolveEmptyPair(-1, -1)
	if a != -1 || b != -1 {
		t.Errorf("expected both -1, got %d, %d", a, b)
	}
}

func TestValidation_MembershipCompatibleEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		a, b ValueType
		want bool
	}{
		{"same type", ValueString, ValueString, true},
		{"empty a", -1, ValueString, true},
		{"empty b", ValueString, -1, true},
		{"id and ref", ValueID, ValueRef, true},
		{"ref and id", ValueRef, ValueID, true},
		{"string and int", ValueString, ValueInt, false},
		{"status and string", ValueStatus, ValueString, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := membershipCompatible(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("membershipCompatible(%d, %d) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestValidation_IsRefCompatible(t *testing.T) {
	tests := []struct {
		typ  ValueType
		want bool
	}{
		{ValueRef, true},
		{ValueID, true},
		{ValueString, false},
		{ValueInt, false},
		{ValueListRef, false},
	}

	for _, tt := range tests {
		t.Run(typeName(tt.typ), func(t *testing.T) {
			got := isRefCompatible(tt.typ)
			if got != tt.want {
				t.Errorf("isRefCompatible(%d) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}

func TestValidation_CheckCompareOpUnknown(t *testing.T) {
	err := checkCompareOp(ValueInt, "~")
	if err == nil {
		t.Fatal("expected error for unknown operator")
	}
	if !strings.Contains(err.Error(), "unknown operator") {
		t.Errorf("expected 'unknown operator', got: %v", err)
	}
}

func TestValidation_IsOrderableType(t *testing.T) {
	orderable := []ValueType{ValueInt, ValueDate, ValueTimestamp, ValueDuration, ValueString, ValueStatus, ValueTaskType, ValueID, ValueRef}
	for _, vt := range orderable {
		if !isOrderableType(vt) {
			t.Errorf("expected %s to be orderable", typeName(vt))
		}
	}

	notOrderable := []ValueType{ValueBool, ValueListString, ValueListRef, ValueRecurrence}
	for _, vt := range notOrderable {
		if isOrderableType(vt) {
			t.Errorf("expected %s to NOT be orderable", typeName(vt))
		}
	}
}

func TestValidation_IsStringLike(t *testing.T) {
	stringLike := []ValueType{ValueString, ValueStatus, ValueTaskType, ValueID, ValueRef}
	for _, vt := range stringLike {
		if !isStringLike(vt) {
			t.Errorf("expected %s to be string-like", typeName(vt))
		}
	}

	notStringLike := []ValueType{ValueInt, ValueDate, ValueBool, ValueListString}
	for _, vt := range notStringLike {
		if isStringLike(vt) {
			t.Errorf("expected %s to NOT be string-like", typeName(vt))
		}
	}
}

func TestValidation_IsEnumType(t *testing.T) {
	if !isEnumType(ValueStatus) {
		t.Error("expected ValueStatus to be enum")
	}
	if !isEnumType(ValueTaskType) {
		t.Error("expected ValueTaskType to be enum")
	}
	if isEnumType(ValueString) {
		t.Error("expected ValueString to NOT be enum")
	}
}

func TestValidation_TypesCompatible(t *testing.T) {
	tests := []struct {
		name string
		a, b ValueType
		want bool
	}{
		{"same type", ValueInt, ValueInt, true},
		{"empty a", -1, ValueString, true},
		{"empty b", ValueString, -1, true},
		{"string-like pair", ValueString, ValueID, true},
		{"status and string", ValueStatus, ValueString, true},
		{"int and string", ValueInt, ValueString, false},
		{"date and int", ValueDate, ValueInt, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := typesCompatible(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("typesCompatible(%s, %s) = %v, want %v", typeName(tt.a), typeName(tt.b), got, tt.want)
			}
		})
	}
}

func TestValidation_InferBinaryExprUnknownOp(t *testing.T) {
	p := newTestParser()

	b := &BinaryExpr{
		Op:    "*",
		Left:  &IntLiteral{Value: 1},
		Right: &IntLiteral{Value: 2},
	}
	_, err := p.inferBinaryExprType(b)
	if err == nil {
		t.Fatal("expected error for unknown binary operator")
	}
	if !strings.Contains(err.Error(), "unknown binary operator") {
		t.Errorf("expected 'unknown binary operator', got: %v", err)
	}
}

func TestValidation_InferExprTypeSubqueryBare(t *testing.T) {
	p := newTestParser()

	sq := &SubQuery{}
	_, err := p.inferExprType(sq)
	if err == nil {
		t.Fatal("expected error for bare subquery")
	}
	if !strings.Contains(err.Error(), "subquery is only valid as argument to count") {
		t.Errorf("unexpected error: %v", err)
	}
}

type valFakeCondition struct{}

func (*valFakeCondition) conditionNode() {}

func TestValidation_ValidateConditionUnknownType(t *testing.T) {
	p := newTestParser()

	err := p.validateCondition(&valFakeCondition{})
	if err == nil {
		t.Fatal("expected error for unknown condition type")
	}
	if !strings.Contains(err.Error(), "unknown condition type") {
		t.Errorf("unexpected error: %v", err)
	}
}

type valFakeExpr struct{}

func (*valFakeExpr) exprNode() {}

func TestValidation_InferExprTypeUnknownType(t *testing.T) {
	p := newTestParser()

	_, err := p.inferExprType(&valFakeExpr{})
	if err == nil {
		t.Fatal("expected error for unknown expression type")
	}
	if !strings.Contains(err.Error(), "unknown expression type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_ListElementType(t *testing.T) {
	tests := []struct {
		typ  ValueType
		want ValueType
	}{
		{ValueListString, ValueString},
		{ValueListRef, ValueRef},
		{ValueString, -1},
		{ValueInt, -1},
	}
	for _, tt := range tests {
		got := listElementType(tt.typ)
		if got != tt.want {
			t.Errorf("listElementType(%s) = %d, want %d", typeName(tt.typ), got, tt.want)
		}
	}
}

func TestValidation_BeforeTriggerValidation(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			"before with action",
			`before update update where id = "x" set status="done"`,
			"before-trigger must not have an action",
		},
		{
			"before without deny",
			`before update where status = "done"`,
			"before-trigger must have deny",
		},
		{
			"after with deny",
			`after update deny "no"`,
			"after-trigger must not have deny",
		},
		{
			"after without action",
			`after update where status = "done"`,
			"after-trigger must have an action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseTrigger(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidation_InferListTypeRefElements(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`create title="x" dependsOn=["TIKI-A", "TIKI-B"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Create == nil {
		t.Fatal("expected create statement")
	}
}

func TestValidation_InferListTypeEmptyList(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`create title="x" tags=[]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Create == nil {
		t.Fatal("expected create statement")
	}
}

func TestValidation_DateMinusDuration(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`select where due > 2026-03-25 - 1week`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_ListRefPlusIdField(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`create title="x" dependsOn=dependsOn + id`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_ListRefMinusIdField(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`create title="x" dependsOn=dependsOn - id`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_ListRefMinusListRef(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`create title="x" dependsOn=dependsOn - dependsOn`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_IntMinusInt(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`create title="x" priority=3 - 1`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_ListStringPlusListString(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`create title="x" tags=tags + tags`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_ListStringMinusListString(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`create title="x" tags=tags - tags`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_ListRefPlusListRef(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`create title="x" dependsOn=dependsOn + dependsOn`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_ContainsIsUnknownFunction(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`select where contains(title, "bug") = contains(title, "fix")`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown function") {
		t.Fatalf("expected 'unknown function' error, got: %v", err)
	}
}

func TestValidation_CheckCompareCompatBothEnums(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`select where status = type`)
	if err == nil {
		t.Fatal("expected error comparing status with type")
	}
	if !strings.Contains(err.Error(), "cannot compare") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_ListRefAssignFromListString(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseStatement(`create title="x" dependsOn=tags`)
	if err == nil {
		t.Fatal("expected error assigning list<string> to list<ref>")
	}
	if !strings.Contains(err.Error(), "cannot assign") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_CheckCompareOpOrderingOnString(t *testing.T) {
	// < on string should fail
	err := checkCompareOp(ValueString, "<")
	if err == nil {
		t.Fatal("expected error for ordering operator on string")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("expected 'not supported' error, got: %v", err)
	}
}

func TestValidation_InferBinaryExprTypeUnknownOp(t *testing.T) {
	p := newTestParser()
	_, err := p.inferBinaryExprType(&BinaryExpr{
		Op:    "*",
		Left:  &IntLiteral{Value: 1},
		Right: &IntLiteral{Value: 2},
	})
	if err == nil {
		t.Fatal("expected error for unknown binary operator")
	}
	if !strings.Contains(err.Error(), "unknown binary operator") {
		t.Errorf("expected 'unknown binary operator' error, got: %v", err)
	}
}

func TestValidation_ValidateInCannotCheck(t *testing.T) {
	p := newTestParser()
	// "42 in priority" — int in int, not valid
	_, err := p.ParseStatement(`select where 42 in priority`)
	if err == nil {
		t.Fatal("expected error for int in int")
	}
	if !strings.Contains(err.Error(), "cannot check") {
		t.Errorf("expected 'cannot check' error, got: %v", err)
	}
}

func TestValidation_TriggerActionMustNotBeSelect(t *testing.T) {
	p := newTestParser()
	// construct trigger with select action directly — parser normally prevents this
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{Select: &SelectStmt{}},
	}
	err := p.validateTrigger(trig)
	if err == nil {
		t.Fatal("expected error for trigger with select action")
	}
	if !strings.Contains(err.Error(), "trigger action must not be select") {
		t.Errorf("expected 'trigger action must not be select' error, got: %v", err)
	}
}

func TestValidation_ListElementTypeMismatch(t *testing.T) {
	p := newTestParser()
	// list with mixed types: string and int
	_, err := p.ParseStatement(`create title="x" tags=["a", 42]`)
	if err == nil {
		t.Fatal("expected error for mixed list element types")
	}
	if !strings.Contains(err.Error(), "same type") {
		t.Errorf("expected 'same type' error, got: %v", err)
	}
}

func TestValidation_ResolveEmptyPair(t *testing.T) {
	// both -1: remain unchanged
	a, b := resolveEmptyPair(-1, -1)
	if a != -1 || b != -1 {
		t.Errorf("resolveEmptyPair(-1, -1) = (%d, %d), want (-1, -1)", a, b)
	}

	// left is empty, right is concrete
	a, b = resolveEmptyPair(-1, ValueInt)
	if a != ValueInt || b != ValueInt {
		t.Errorf("resolveEmptyPair(-1, ValueInt) = (%d, %d), want (ValueInt, ValueInt)", a, b)
	}

	// right is empty, left is concrete
	a, b = resolveEmptyPair(ValueString, -1)
	if a != ValueString || b != ValueString {
		t.Errorf("resolveEmptyPair(ValueString, -1) = (%d, %d), want (ValueString, ValueString)", a, b)
	}

	// both concrete
	a, b = resolveEmptyPair(ValueInt, ValueString)
	if a != ValueInt || b != ValueString {
		t.Errorf("resolveEmptyPair(ValueInt, ValueString) = (%d, %d), want (ValueInt, ValueString)", a, b)
	}
}

// --- tests for uncovered error-propagation branches ---

func TestValidation_ValidateStatement_EmptyCreate(t *testing.T) {
	p := newTestParser()
	// construct create with no assignments — parser normally prevents this
	err := p.validateStatement(&Statement{Create: &CreateStmt{}})
	if err == nil {
		t.Fatal("expected error for empty create")
	}
	if !strings.Contains(err.Error(), "at least one assignment") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_ValidateStatement_EmptyUpdate(t *testing.T) {
	p := newTestParser()
	err := p.validateStatement(&Statement{Update: &UpdateStmt{
		Where: &CompareExpr{Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "x"}},
	}})
	if err == nil {
		t.Fatal("expected error for empty update set")
	}
	if !strings.Contains(err.Error(), "at least one assignment") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_ValidateStatement_UpdateWhereError(t *testing.T) {
	p := newTestParser()
	// update with bad where — unknown field in condition
	err := p.validateStatement(&Statement{Update: &UpdateStmt{
		Where: &CompareExpr{Left: &FieldRef{Name: "nosuchfield"}, Op: "=", Right: &StringLiteral{Value: "x"}},
		Set:   []Assignment{{Field: "title", Value: &StringLiteral{Value: "x"}}},
	}})
	if err == nil {
		t.Fatal("expected error for unknown field in update where")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_ValidateStatement_EmptyStatement(t *testing.T) {
	p := newTestParser()
	err := p.validateStatement(&Statement{})
	if err == nil {
		t.Fatal("expected error for empty statement")
	}
	if !strings.Contains(err.Error(), "empty statement") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_ValidateTrigger_ActionValidateError(t *testing.T) {
	p := newTestParser()
	// trigger with action that has unknown field in assignment
	p.qualifiers = triggerQualifiers("update")
	err := p.validateTrigger(&Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{Create: &CreateStmt{
			Assignments: []Assignment{{Field: "nosuchfield", Value: &StringLiteral{Value: "x"}}},
		}},
	})
	if err == nil {
		t.Fatal("expected error for bad trigger action")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_ValidateTrigger_RunCommandInferError(t *testing.T) {
	p := newTestParser()
	p.qualifiers = triggerQualifiers("update")
	err := p.validateTrigger(&Trigger{
		Timing: "after",
		Event:  "update",
		Run:    &RunAction{Command: &valFakeExpr{}},
	})
	if err == nil {
		t.Fatal("expected error for bad run command")
	}
	if !strings.Contains(err.Error(), "run command") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_ValidateTrigger_RunCommandNonString(t *testing.T) {
	p := newTestParser()
	p.qualifiers = triggerQualifiers("update")
	err := p.validateTrigger(&Trigger{
		Timing: "after",
		Event:  "update",
		Run:    &RunAction{Command: &IntLiteral{Value: 42}},
	})
	if err == nil {
		t.Fatal("expected error for non-string run command")
	}
	if !strings.Contains(err.Error(), "must be string") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_ValidateCondition_BinaryLeftError(t *testing.T) {
	p := newTestParser()
	// binary condition where left side has unknown field
	err := p.validateCondition(&BinaryCondition{
		Op:    "and",
		Left:  &CompareExpr{Left: &FieldRef{Name: "nosuchfield"}, Op: "=", Right: &StringLiteral{Value: "x"}},
		Right: &CompareExpr{Left: &FieldRef{Name: "title"}, Op: "=", Right: &StringLiteral{Value: "y"}},
	})
	if err == nil {
		t.Fatal("expected error for bad binary condition left")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_ValidateIn_ValueTypeError(t *testing.T) {
	p := newTestParser()
	err := p.validateIn(&InExpr{
		Value:      &valFakeExpr{},
		Collection: &ListLiteral{Elements: []Expr{&StringLiteral{Value: "x"}}},
	})
	if err == nil {
		t.Fatal("expected error for bad value type in 'in' expr")
	}
	if !strings.Contains(err.Error(), "unknown expression type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_ValidateIn_InferListElementError(t *testing.T) {
	p := newTestParser()
	// list literal with bad first element — causes inferListElementType to error
	err := p.validateIn(&InExpr{
		Value:      &StringLiteral{Value: "x"},
		Collection: &ListLiteral{Elements: []Expr{&valFakeExpr{}}},
	})
	if err == nil {
		t.Fatal("expected error for bad list element in 'in' expr")
	}
}

func TestValidation_InferExprType_QualifiedRefUnknownField(t *testing.T) {
	p := newTestParser()
	p.qualifiers = qualifierPolicy{allowOld: true, allowNew: true}
	_, err := p.inferExprType(&QualifiedRef{Qualifier: "new", Name: "nosuchfield"})
	if err == nil {
		t.Fatal("expected error for unknown qualified field")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_InferListType_FirstElementError(t *testing.T) {
	p := newTestParser()
	_, err := p.inferListType(&ListLiteral{Elements: []Expr{&valFakeExpr{}}})
	if err == nil {
		t.Fatal("expected error for bad first list element")
	}
}

func TestValidation_InferListType_SecondElementError(t *testing.T) {
	p := newTestParser()
	_, err := p.inferListType(&ListLiteral{Elements: []Expr{
		&StringLiteral{Value: "ok"},
		&valFakeExpr{},
	}})
	if err == nil {
		t.Fatal("expected error for bad second list element")
	}
}

func TestValidation_InferListElementType_NonLiteralError(t *testing.T) {
	p := newTestParser()
	// non-literal expr that returns an error from inferExprType
	_, err := p.inferListElementType(&valFakeExpr{})
	if err == nil {
		t.Fatal("expected error for non-literal list element type")
	}
}

func TestValidation_InferListElementType_NonListFallback(t *testing.T) {
	p := newTestParser()
	// a field ref that's not a list type — should return the type as-is
	typ, err := p.inferListElementType(&FieldRef{Name: "title"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != ValueString {
		t.Errorf("expected ValueString, got %s", typeName(typ))
	}
}

func TestValidation_InferFuncCallType_VariableArgRange(t *testing.T) {
	// there are no builtins with minArgs != maxArgs in the current code,
	// but exercise the branch by directly calling inferFuncCallType
	// with too many args on a fixed-arity function (covers the minArgs==maxArgs message)
	p := newTestParser()
	_, err := p.inferFuncCallType(&FunctionCall{
		Name: "now",
		Args: []Expr{&StringLiteral{Value: "extra"}},
	})
	if err == nil {
		t.Fatal("expected error for wrong arg count")
	}
	if !strings.Contains(err.Error(), "argument") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_InferFuncCallType_BlocksStringNonLiteral(t *testing.T) {
	p := newTestParser()
	// blocks() with a string-typed non-literal arg (field ref to a string field)
	_, err := p.inferFuncCallType(&FunctionCall{
		Name: "blocks",
		Args: []Expr{&FieldRef{Name: "assignee"}}, // string type, but not a literal
	})
	if err == nil {
		t.Fatal("expected error for blocks() with string non-literal")
	}
	if !strings.Contains(err.Error(), "blocks() argument must be an id or ref") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_InferFuncCallType_CallArgError(t *testing.T) {
	p := newTestParser()
	_, err := p.inferFuncCallType(&FunctionCall{
		Name: "call",
		Args: []Expr{&valFakeExpr{}},
	})
	if err == nil {
		t.Fatal("expected error for call() with bad arg")
	}
}

func TestValidation_InferFuncCallType_NextDateArgError(t *testing.T) {
	p := newTestParser()
	_, err := p.inferFuncCallType(&FunctionCall{
		Name: "next_date",
		Args: []Expr{&valFakeExpr{}},
	})
	if err == nil {
		t.Fatal("expected error for next_date() with bad arg")
	}
}

func TestValidation_InferBinaryExprType_LeftError(t *testing.T) {
	p := newTestParser()
	_, err := p.inferBinaryExprType(&BinaryExpr{
		Op:    "+",
		Left:  &valFakeExpr{},
		Right: &IntLiteral{Value: 1},
	})
	if err == nil {
		t.Fatal("expected error for bad left in binary expr")
	}
}

func TestValidation_InferBinaryExprType_RightError(t *testing.T) {
	p := newTestParser()
	_, err := p.inferBinaryExprType(&BinaryExpr{
		Op:    "+",
		Left:  &IntLiteral{Value: 1},
		Right: &valFakeExpr{},
	})
	if err == nil {
		t.Fatal("expected error for bad right in binary expr")
	}
}

func TestValidation_CheckAssignmentCompat_UnresolvedEmpty(t *testing.T) {
	p := newTestParser()
	// rhsType == -1 but rhs is not an EmptyLiteral — exercises the rhsType == -1 branch
	err := p.checkAssignmentCompat(ValueString, -1, &BinaryExpr{
		Op:    "+",
		Left:  &EmptyLiteral{},
		Right: &EmptyLiteral{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_CheckCompareCompat_RightEnumCheck(t *testing.T) {
	p := newTestParser()
	// right side is enum, left side is non-enum non-literal
	err := p.checkCompareCompat(ValueString, ValueStatus, &FieldRef{Name: "title"}, &FieldRef{Name: "status"})
	if err == nil {
		t.Fatal("expected error for comparing string field with status field")
	}
	if !strings.Contains(err.Error(), "cannot compare") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- additional coverage for validate.go uncovered branches ---

func TestValidation_BlocksNonLiteralString(t *testing.T) {
	p := newTestParser()
	// blocks() with a non-literal string field (assignee is a string field, not ref/id)
	_, err := p.ParseStatement(`select where count(select where id in blocks(assignee)) > 0`)
	if err == nil {
		t.Fatal("expected error for blocks() with string field argument")
	}
	if !strings.Contains(err.Error(), "blocks() argument must be an id or ref") {
		t.Fatalf("expected blocks() argument error, got: %v", err)
	}
}

func TestValidation_LimitZero(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseStatement("select limit 0")
	if err == nil {
		t.Fatal("expected error for limit 0")
	}
	if !strings.Contains(err.Error(), "limit must be a positive integer, got 0") {
		t.Fatalf("expected limit validation error, got: %v", err)
	}
}

func TestValidation_InExprListElementTypeError(t *testing.T) {
	p := newTestParser()
	// construct a case where inferListElementType fails:
	// use a list literal whose first element is unknown
	// This is hard to trigger via parser, so test the internal function directly
	_, err := p.inferListElementType(&ListLiteral{Elements: []Expr{
		&FunctionCall{Name: "unknown_func"},
	}})
	if err == nil {
		t.Fatal("expected error for unknown function in list element")
	}
}
