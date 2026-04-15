package ruki

import (
	"strings"
	"testing"
)

// TestLower_DurationUnits exercises all duration unit branches in parseDurationLiteral
// through the parser (which calls lowerUnary → parseDurationLiteral).
func TestLower_DurationUnits(t *testing.T) {
	p := newTestParser()

	units := []struct {
		literal string
	}{
		{"1day"},
		{"2days"},
		{"1week"},
		{"3weeks"},
		{"1month"},
		{"2months"},
		{"1year"},
		{"1hour"},
		{"1hours"},
		{"30min"},
		{"30mins"},
		{"10sec"},
		{"10secs"},
	}

	for _, tt := range units {
		t.Run(tt.literal, func(t *testing.T) {
			input := `select where due > 2026-01-01 + ` + tt.literal
			_, err := p.ParseStatement(input)
			if err != nil {
				t.Fatalf("unexpected error for duration %s: %v", tt.literal, err)
			}
		})
	}
}

// TestLower_DateLiteralInCondition exercises the date literal branch in lowerUnary.
func TestLower_DateLiteralInCondition(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where due = 2026-06-15`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Select == nil {
		t.Fatal("expected select statement")
	}
	cmp, ok := stmt.Select.Where.(*CompareExpr)
	if !ok {
		t.Fatalf("expected CompareExpr, got %T", stmt.Select.Where)
	}
	dl, ok := cmp.Right.(*DateLiteral)
	if !ok {
		t.Fatalf("expected DateLiteral, got %T", cmp.Right)
		return
	}
	if dl.Value.Year() != 2026 || dl.Value.Month() != 6 || dl.Value.Day() != 15 {
		t.Errorf("unexpected date: %v", dl.Value)
	}
}

// TestLower_IntLiteral exercises the int literal branch.
func TestLower_IntLiteral(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where priority = 3`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmp, ok := stmt.Select.Where.(*CompareExpr)
	if !ok {
		t.Fatalf("expected CompareExpr, got %T", stmt.Select.Where)
	}
	il, ok := cmp.Right.(*IntLiteral)
	if !ok {
		t.Fatalf("expected IntLiteral, got %T", cmp.Right)
		return
	}
	if il.Value != 3 {
		t.Errorf("expected 3, got %d", il.Value)
	}
}

// TestLower_EmptyLiteral exercises the empty literal branch.
func TestLower_EmptyLiteral(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where assignee is empty`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ie, ok := stmt.Select.Where.(*IsEmptyExpr)
	if !ok {
		t.Fatalf("expected IsEmptyExpr, got %T", stmt.Select.Where)
		return
	}
	if ie.Negated {
		t.Error("expected non-negated is empty")
	}
}

// TestLower_IsNotEmpty exercises the is not empty branch.
func TestLower_IsNotEmpty(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where assignee is not empty`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ie, ok := stmt.Select.Where.(*IsEmptyExpr)
	if !ok {
		t.Fatalf("expected IsEmptyExpr, got %T", stmt.Select.Where)
	}
	if !ie.Negated {
		t.Error("expected negated is not empty")
	}
}

// TestLower_NotIn exercises the not in branch.
func TestLower_NotIn(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where status not in ["done"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	in, ok := stmt.Select.Where.(*InExpr)
	if !ok {
		t.Fatalf("expected InExpr, got %T", stmt.Select.Where)
	}
	if !in.Negated {
		t.Error("expected negated not in")
	}
}

// TestLower_In exercises the in branch.
func TestLower_In(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where status in ["done", "ready"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	in, ok := stmt.Select.Where.(*InExpr)
	if !ok {
		t.Fatalf("expected InExpr, got %T", stmt.Select.Where)
		return
	}
	if in.Negated {
		t.Error("expected non-negated in")
	}
}

// TestLower_QuantifierAll exercises the all quantifier branch.
func TestLower_QuantifierAll(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where dependsOn all status = "done"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	q, ok := stmt.Select.Where.(*QuantifierExpr)
	if !ok {
		t.Fatalf("expected QuantifierExpr, got %T", stmt.Select.Where)
		return
	}
	if q.Kind != "all" {
		t.Errorf("expected 'all', got %q", q.Kind)
	}
}

// TestLower_ParenCondition exercises the parenthesized condition branch.
func TestLower_ParenCondition(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where (status = "done" or status = "ready")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bc, ok := stmt.Select.Where.(*BinaryCondition)
	if !ok {
		t.Fatalf("expected BinaryCondition, got %T", stmt.Select.Where)
		return
	}
	if bc.Op != "or" {
		t.Errorf("expected 'or', got %q", bc.Op)
	}
}

// TestLower_ParenExpr exercises the parenthesized expression branch.
func TestLower_ParenExpr(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`create title="x" priority=(1 + 2)`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Create == nil {
		t.Fatal("expected create statement")
	}
}

// TestLower_NotCondition exercises the not condition branch.
func TestLower_NotCondition(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where not status = "done"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nc, ok := stmt.Select.Where.(*NotCondition)
	if !ok {
		t.Fatalf("expected NotCondition, got %T", stmt.Select.Where)
		return
	}
	if nc.Inner == nil {
		t.Fatal("expected non-nil inner condition")
	}
}

// TestLower_SelectStar exercises the select * branch.
func TestLower_SelectStar(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select *`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Select.Fields != nil {
		t.Errorf("expected nil fields for select *, got %v", stmt.Select.Fields)
	}
}

// TestLower_SelectBare exercises bare select (no fields, no star).
func TestLower_SelectBare(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Select.Fields != nil {
		t.Errorf("expected nil fields for bare select, got %v", stmt.Select.Fields)
	}
}

// TestLower_SelectFields exercises the specific fields branch.
func TestLower_SelectFields(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select id, title, status`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Select.Fields == nil {
		t.Fatal("expected non-nil fields")
	}
	expected := []string{"id", "title", "status"}
	if len(stmt.Select.Fields) != len(expected) {
		t.Fatalf("expected %d fields, got %d", len(expected), len(stmt.Select.Fields))
	}
	for i, f := range stmt.Select.Fields {
		if f != expected[i] {
			t.Errorf("fields[%d] = %q, want %q", i, f, expected[i])
		}
	}
}

// TestLower_OrderByAscDesc exercises the order by lowering with explicit asc/desc.
func TestLower_OrderByAscDesc(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select order by priority desc, title asc`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmt.Select.OrderBy) != 2 {
		t.Fatalf("expected 2 order by clauses, got %d", len(stmt.Select.OrderBy))
	}
	if !stmt.Select.OrderBy[0].Desc {
		t.Error("expected first clause to be desc")
	}
	if stmt.Select.OrderBy[1].Desc {
		t.Error("expected second clause to be asc (not desc)")
	}
}

// TestLower_ListLiteral exercises list literal lowering.
func TestLower_ListLiteral(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`create title="x" tags=["a", "b", "c"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Create == nil {
		t.Fatal("expected create statement")
	}
	// find the tags assignment
	for _, a := range stmt.Create.Assignments {
		if a.Field == "tags" {
			ll, ok := a.Value.(*ListLiteral)
			if !ok {
				t.Fatalf("expected ListLiteral, got %T", a.Value)
			}
			if len(ll.Elements) != 3 {
				t.Errorf("expected 3 elements, got %d", len(ll.Elements))
			}
			return
		}
	}
	t.Fatal("tags assignment not found")
}

// TestLower_BinaryExpr exercises binary expression lowering.
func TestLower_BinaryExpr(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`create title="hello" + " world"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Create == nil {
		t.Fatal("expected create statement")
	}
	for _, a := range stmt.Create.Assignments {
		if a.Field == "title" {
			be, ok := a.Value.(*BinaryExpr)
			if !ok {
				t.Fatalf("expected BinaryExpr, got %T", a.Value)
			}
			if be.Op != "+" {
				t.Errorf("expected '+', got %q", be.Op)
			}
			return
		}
	}
	t.Fatal("title assignment not found")
}

// TestLower_SubQuery exercises subquery lowering inside count().
func TestLower_SubQuery(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where count(select where status = "done") >= 1`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmp, ok := stmt.Select.Where.(*CompareExpr)
	if !ok {
		t.Fatalf("expected CompareExpr, got %T", stmt.Select.Where)
	}
	fc, ok := cmp.Left.(*FunctionCall)
	if !ok {
		t.Fatalf("expected FunctionCall, got %T", cmp.Left)
		return
	}
	if fc.Name != "count" {
		t.Errorf("expected 'count', got %q", fc.Name)
	}
	sq, ok := fc.Args[0].(*SubQuery)
	if !ok {
		t.Fatalf("expected SubQuery arg, got %T", fc.Args[0])
		return
	}
	if sq.Where == nil {
		t.Fatal("expected non-nil where in subquery")
	}
}

// TestLower_SubQueryBare exercises subquery without where inside count().
func TestLower_SubQueryBare(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where count(select) >= 0`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmp, ok := stmt.Select.Where.(*CompareExpr)
	if !ok {
		t.Fatalf("expected CompareExpr, got %T", stmt.Select.Where)
	}
	fc, ok := cmp.Left.(*FunctionCall)
	if !ok {
		t.Fatalf("expected FunctionCall, got %T", cmp.Left)
	}
	sq, ok := fc.Args[0].(*SubQuery)
	if !ok {
		t.Fatalf("expected SubQuery, got %T", fc.Args[0])
		return
	}
	if sq.Where != nil {
		t.Error("expected nil where in bare subquery")
	}
}

// TestLower_QualifiedRef exercises qualified ref lowering in triggers.
func TestLower_QualifiedRef(t *testing.T) {
	p := newTestParser()

	trig, err := p.ParseTrigger(`before update where old.status = "in progress" deny "no"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmp, ok := trig.Where.(*CompareExpr)
	if !ok {
		t.Fatalf("expected CompareExpr, got %T", trig.Where)
	}
	qr, ok := cmp.Left.(*QualifiedRef)
	if !ok {
		t.Fatalf("expected QualifiedRef, got %T", cmp.Left)
		return
	}
	if qr.Qualifier != "old" || qr.Name != "status" {
		t.Errorf("expected old.status, got %s.%s", qr.Qualifier, qr.Name)
	}
}

// TestLower_TriggerDeny exercises the deny lowering path.
func TestLower_TriggerDeny(t *testing.T) {
	p := newTestParser()

	trig, err := p.ParseTrigger(`before delete deny "cannot delete"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trig.Deny == nil {
		t.Fatal("expected non-nil deny")
	}
	if *trig.Deny != "cannot delete" {
		t.Errorf("expected 'cannot delete', got %q", *trig.Deny)
	}
}

// TestUnquoteString exercises the unquoteString helper directly.
func TestUnquoteString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple quoted", `"hello"`, "hello"},
		{"escaped quote", `"say \"hi\""`, `say "hi"`},
		{"no quotes", "bare", "bare"},
		{"empty quotes", `""`, ""},
		{"single char", `"x"`, "x"},
		{"invalid escape fallback", "\"bad\\qescape\"", "bad\\qescape"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unquoteString(tt.input)
			if got != tt.want {
				t.Errorf("unquoteString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestLower_EmptyStatement exercises the default branch of lowerStatement.
func TestLower_EmptyStatement(t *testing.T) {
	_, err := lowerStatement(&statementGrammar{})
	if err == nil {
		t.Fatal("expected error for empty statement grammar")
		return
	}
	if err.Error() != "empty statement" {
		t.Errorf("expected 'empty statement', got: %v", err)
	}
}

// TestLower_EmptyTriggerAction exercises the default branch of lowerTriggerAction.
func TestLower_EmptyTriggerAction(t *testing.T) {
	trig := &Trigger{Timing: "after", Event: "update"}
	err := lowerTriggerAction(&actionGrammar{}, trig)
	if err == nil {
		t.Fatal("expected error for empty trigger action")
		return
	}
	if err.Error() != "empty trigger action" {
		t.Errorf("expected 'empty trigger action', got: %v", err)
	}
}

// TestLower_EmptyExpression exercises the default branch of lowerUnary.
func TestLower_EmptyExpression(t *testing.T) {
	_, err := lowerUnary(&unaryExpr{})
	if err == nil {
		t.Fatal("expected error for empty expression")
		return
	}
	if err.Error() != "empty expression" {
		t.Errorf("expected 'empty expression', got: %v", err)
	}
}

// TestLower_InvalidDateLiteral exercises parseDateLiteral error path.
func TestLower_InvalidDateLiteral(t *testing.T) {
	_, err := parseDateLiteral("not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid date literal")
	}
}

// TestLower_InvalidDurationLiteral exercises parseDurationLiteral error paths.
func TestLower_InvalidDurationLiteral(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no digits", "days"},
		{"no unit", "123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDurationLiteral(tt.input)
			if err == nil {
				t.Fatal("expected error for invalid duration")
			}
		})
	}
}

// TestLower_SubQueryOrderByRejected exercises the order by rejection in lowerSubQuery.
func TestLower_SubQueryOrderByRejected(t *testing.T) {
	dir := "asc"
	sq := &subQueryExpr{
		OrderBy: &orderByGrammar{
			First: orderByField{Field: "priority", Direction: &dir},
		},
	}
	_, err := lowerSubQuery(sq)
	if err == nil {
		t.Fatal("expected error for order by in subquery")
		return
	}
	if err.Error() != "order by is not valid inside a subquery" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestLower_ExprCondBareExpression exercises the default branch of lowerExprCond.
func TestLower_ExprCondBareExpression(t *testing.T) {
	field := "title"
	ec := &exprCond{
		Left: exprGrammar{
			Left: unaryExpr{FieldRef: &field},
		},
	}
	_, err := lowerExprCond(ec)
	if err == nil {
		t.Fatal("expected error for bare expression as condition")
		return
	}
	if err.Error() != "expression used as condition without comparison operator" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestLower_TriggerRunAction exercises the run action branch in lowerTriggerAction.
func TestLower_TriggerRunAction(t *testing.T) {
	p := newTestParser()

	trig, err := p.ParseTrigger(`after update run("echo done")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trig.Run == nil {
		t.Fatal("expected non-nil run action")
	}
}

// TestLower_TriggerUpdateAction exercises the update action branch in lowerTriggerAction.
func TestLower_TriggerUpdateAction(t *testing.T) {
	p := newTestParser()

	trig, err := p.ParseTrigger(`after update where new.status = "done" update where id = new.id set priority=1`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trig.Action == nil || trig.Action.Update == nil {
		t.Fatal("expected update action in trigger")
	}
}

// TestLower_TriggerDeleteAction exercises the delete action branch in lowerTriggerAction.
func TestLower_TriggerDeleteAction(t *testing.T) {
	p := newTestParser()

	trig, err := p.ParseTrigger(`after update where new.status = "done" delete where id = new.id`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trig.Action == nil || trig.Action.Delete == nil {
		t.Fatal("expected delete action in trigger")
	}
}

// TestLower_TriggerCreateAction exercises the create action branch in lowerTriggerAction.
func TestLower_TriggerCreateAction(t *testing.T) {
	p := newTestParser()

	trig, err := p.ParseTrigger(`after update where new.status = "done" create title="follow-up"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trig.Action == nil || trig.Action.Create == nil {
		t.Fatal("expected create action in trigger")
	}
}

// TestLower_TriggerWithoutWhereOrAction exercises trigger lowering with no where and no action.
func TestLower_TriggerWithoutWhereOrAction(t *testing.T) {
	p := newTestParser()

	trig, err := p.ParseTrigger(`before delete deny "forbidden"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trig.Where != nil {
		t.Error("expected nil where")
	}
	if trig.Deny == nil {
		t.Fatal("expected non-nil deny")
	}
}

// TestLower_DeleteStatement exercises the delete lowering path.
func TestLower_DeleteStatement(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`delete where status = "done"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Delete == nil {
		t.Fatal("expected delete statement")
	}
	if stmt.Delete.Where == nil {
		t.Fatal("expected non-nil where in delete")
	}
}

// TestLower_UpdateStatement exercises the update lowering path.
func TestLower_UpdateStatement(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`update where status = "done" set priority=1`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Update == nil {
		t.Fatal("expected update statement")
	}
}

// TestLower_CreateStatement exercises the create lowering path.
func TestLower_CreateStatement(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`create title="hello"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Create == nil {
		t.Fatal("expected create statement")
	}
}

// TestLower_OrCondChain exercises the or condition chaining in lowerOrCond.
func TestLower_OrCondChain(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where status = "done" or status = "ready" or status = "backlog"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bc, ok := stmt.Select.Where.(*BinaryCondition)
	if !ok {
		t.Fatalf("expected BinaryCondition, got %T", stmt.Select.Where)
		return
	}
	if bc.Op != "or" {
		t.Errorf("expected 'or', got %q", bc.Op)
	}
}

// TestLower_AndCondChain exercises the and condition chaining in lowerAndCond.
func TestLower_AndCondChain(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where priority = 1 and status = "done" and assignee = "bob"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bc, ok := stmt.Select.Where.(*BinaryCondition)
	if !ok {
		t.Fatalf("expected BinaryCondition, got %T", stmt.Select.Where)
		return
	}
	if bc.Op != "and" {
		t.Errorf("expected 'and', got %q", bc.Op)
	}
}

func TestLower_InvalidDateLiteralOutOfRange(t *testing.T) {
	p := newTestParser()
	// 9999-99-99 matches the lexer regex but is not a valid date
	_, err := p.ParseStatement(`select where due > 9999-99-99`)
	if err == nil {
		t.Fatal("expected error for invalid date literal")
	}
	if !strings.Contains(err.Error(), "invalid date literal") {
		t.Errorf("expected 'invalid date literal' error, got: %v", err)
	}
}

func TestLower_ParseDateAndDurationInExpressions(t *testing.T) {
	p := newTestParser()
	// valid date with duration subtraction
	stmt, err := p.ParseStatement(`select where due > 2025-06-15 - 2week`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.Select == nil {
		t.Fatal("expected select statement")
	}
}

// --- error propagation tests for lower.go coverage ---

func TestLower_CreateWithBadAssignment(t *testing.T) {
	// lowerCreate error path: assignment value fails to lower
	badDate := "9999-99-99"
	g := &createGrammar{
		Assignments: []assignmentGrammar{
			{Field: "due", Value: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
		},
	}
	_, err := lowerCreate(g)
	if err == nil {
		t.Fatal("expected error for invalid date in create assignment")
	}
}

func TestLower_UpdateWithBadWhere(t *testing.T) {
	// lowerUpdate error path: where condition fails to lower
	badDate := "0000-00-00"
	g := &updateGrammar{
		Where: orCond{
			Left: andCond{
				Left: notCond{
					Primary: &primaryCond{
						Expr: &exprCond{
							Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
							Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
						},
					},
				},
			},
		},
		Set: []assignmentGrammar{
			{Field: "title", Value: exprGrammar{Left: unaryExpr{StrLit: strPtr("x")}}},
		},
	}
	_, err := lowerUpdate(g)
	if err == nil {
		t.Fatal("expected error for invalid date in update where")
	}
}

func TestLower_UpdateWithBadSet(t *testing.T) {
	// lowerUpdate error path: set assignment fails to lower
	field := "status"
	badDate := "9999-99-99"
	g := &updateGrammar{
		Where: orCond{
			Left: andCond{
				Left: notCond{
					Primary: &primaryCond{
						Expr: &exprCond{
							Left:    exprGrammar{Left: unaryExpr{FieldRef: &field}},
							Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{StrLit: strPtr("done")}}},
						},
					},
				},
			},
		},
		Set: []assignmentGrammar{
			{Field: "due", Value: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
		},
	}
	_, err := lowerUpdate(g)
	if err == nil {
		t.Fatal("expected error for invalid date in update set")
	}
}

func TestLower_DeleteWithBadWhere(t *testing.T) {
	// lowerDelete error path: where condition fails to lower
	badDate := "0000-00-00"
	g := &deleteGrammar{
		Where: orCond{
			Left: andCond{
				Left: notCond{
					Primary: &primaryCond{
						Expr: &exprCond{
							Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
							Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
						},
					},
				},
			},
		},
	}
	_, err := lowerDelete(g)
	if err == nil {
		t.Fatal("expected error for invalid date in delete where")
	}
}

func TestLower_AssignmentWithBadExpr(t *testing.T) {
	// lowerAssignments error path
	badDate := "9999-99-99"
	gs := []assignmentGrammar{
		{Field: "due", Value: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
	}
	_, err := lowerAssignments(gs)
	if err == nil {
		t.Fatal("expected error for invalid date in assignment")
	}
}

func TestLower_TriggerWithBadWhere(t *testing.T) {
	// lowerTrigger error path: where condition fails to lower
	badDate := "0000-00-00"
	g := &triggerGrammar{
		Timing: "before",
		Event:  "update",
		Where: &orCond{
			Left: andCond{
				Left: notCond{
					Primary: &primaryCond{
						Expr: &exprCond{
							Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
							Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
						},
					},
				},
			},
		},
		Deny: &denyGrammar{Message: `"no"`},
	}
	_, err := lowerTrigger(g)
	if err == nil {
		t.Fatal("expected error for invalid date in trigger where")
	}
}

func TestLower_TriggerActionWithBadRun(t *testing.T) {
	// lowerTriggerAction error path: run command fails to lower
	badDate := "9999-99-99"
	trig := &Trigger{Timing: "after", Event: "update"}
	err := lowerTriggerAction(&actionGrammar{
		Run: &runGrammar{
			Command: exprGrammar{Left: unaryExpr{DateLit: &badDate}},
		},
	}, trig)
	if err == nil {
		t.Fatal("expected error for invalid date in run command")
	}
}

func TestLower_TriggerActionWithBadCreate(t *testing.T) {
	badDate := "9999-99-99"
	trig := &Trigger{Timing: "after", Event: "update"}
	err := lowerTriggerAction(&actionGrammar{
		Create: &createGrammar{
			Assignments: []assignmentGrammar{
				{Field: "due", Value: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
			},
		},
	}, trig)
	if err == nil {
		t.Fatal("expected error for invalid date in trigger create action")
	}
}

func TestLower_TriggerActionWithBadUpdate(t *testing.T) {
	badDate := "0000-00-00"
	trig := &Trigger{Timing: "after", Event: "update"}
	err := lowerTriggerAction(&actionGrammar{
		Update: &updateGrammar{
			Where: orCond{
				Left: andCond{
					Left: notCond{
						Primary: &primaryCond{
							Expr: &exprCond{
								Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
								Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
							},
						},
					},
				},
			},
			Set: []assignmentGrammar{
				{Field: "title", Value: exprGrammar{Left: unaryExpr{StrLit: strPtr("x")}}},
			},
		},
	}, trig)
	if err == nil {
		t.Fatal("expected error for invalid date in trigger update action")
	}
}

func TestLower_TriggerActionWithBadDelete(t *testing.T) {
	badDate := "0000-00-00"
	trig := &Trigger{Timing: "after", Event: "update"}
	err := lowerTriggerAction(&actionGrammar{
		Delete: &deleteGrammar{
			Where: orCond{
				Left: andCond{
					Left: notCond{
						Primary: &primaryCond{
							Expr: &exprCond{
								Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
								Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
							},
						},
					},
				},
			},
		},
	}, trig)
	if err == nil {
		t.Fatal("expected error for invalid date in trigger delete action")
	}
}

func TestLower_OrCondWithBadRight(t *testing.T) {
	badDate := "0000-00-00"
	field := "status"
	g := &orCond{
		Left: andCond{
			Left: notCond{
				Primary: &primaryCond{
					Expr: &exprCond{
						Left:    exprGrammar{Left: unaryExpr{FieldRef: &field}},
						Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{StrLit: strPtr("done")}}},
					},
				},
			},
		},
		Right: []andCond{
			{
				Left: notCond{
					Primary: &primaryCond{
						Expr: &exprCond{
							Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
							Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
						},
					},
				},
			},
		},
	}
	_, err := lowerOrCond(g)
	if err == nil {
		t.Fatal("expected error for invalid date in or-cond right branch")
	}
}

func TestLower_AndCondWithBadRight(t *testing.T) {
	badDate := "0000-00-00"
	field := "status"
	g := &andCond{
		Left: notCond{
			Primary: &primaryCond{
				Expr: &exprCond{
					Left:    exprGrammar{Left: unaryExpr{FieldRef: &field}},
					Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{StrLit: strPtr("done")}}},
				},
			},
		},
		Right: []notCond{
			{
				Primary: &primaryCond{
					Expr: &exprCond{
						Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
						Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
					},
				},
			},
		},
	}
	_, err := lowerAndCond(g)
	if err == nil {
		t.Fatal("expected error for invalid date in and-cond right branch")
	}
}

func TestLower_NotCondWithBadInner(t *testing.T) {
	badDate := "0000-00-00"
	g := &notCond{
		Not: &notCond{
			Primary: &primaryCond{
				Expr: &exprCond{
					Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
					Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
				},
			},
		},
	}
	_, err := lowerNotCond(g)
	if err == nil {
		t.Fatal("expected error for invalid date in not-cond inner")
	}
}

func TestLower_ExprCondWithBadCompareRight(t *testing.T) {
	field := "due"
	badDate := "0000-00-00"
	g := &exprCond{
		Left:    exprGrammar{Left: unaryExpr{FieldRef: &field}},
		Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
	}
	_, err := lowerExprCond(g)
	if err == nil {
		t.Fatal("expected error for invalid date in compare right")
	}
}

func TestLower_ExprCondWithBadInCollection(t *testing.T) {
	field := "status"
	badDate := "0000-00-00"
	g := &exprCond{
		Left: exprGrammar{Left: unaryExpr{FieldRef: &field}},
		In:   &inTail{Collection: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
	}
	_, err := lowerExprCond(g)
	if err == nil {
		t.Fatal("expected error for invalid date in in-collection")
	}
}

func TestLower_ExprCondWithBadNotInCollection(t *testing.T) {
	field := "status"
	badDate := "0000-00-00"
	g := &exprCond{
		Left:  exprGrammar{Left: unaryExpr{FieldRef: &field}},
		NotIn: &notInTail{Collection: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
	}
	_, err := lowerExprCond(g)
	if err == nil {
		t.Fatal("expected error for invalid date in not-in collection")
	}
}

func TestLower_ExprCondWithBadAnyCondition(t *testing.T) {
	field := "dependsOn"
	badDate := "0000-00-00"
	g := &exprCond{
		Left: exprGrammar{Left: unaryExpr{FieldRef: &field}},
		Any: &quantifierTail{
			Condition: primaryCond{
				Expr: &exprCond{
					Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
					Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
				},
			},
		},
	}
	_, err := lowerExprCond(g)
	if err == nil {
		t.Fatal("expected error for invalid date in any-condition")
	}
}

func TestLower_ExprCondWithBadAllCondition(t *testing.T) {
	field := "dependsOn"
	badDate := "0000-00-00"
	g := &exprCond{
		Left: exprGrammar{Left: unaryExpr{FieldRef: &field}},
		All: &allQuantTail{
			Condition: primaryCond{
				Expr: &exprCond{
					Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
					Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
				},
			},
		},
	}
	_, err := lowerExprCond(g)
	if err == nil {
		t.Fatal("expected error for invalid date in all-condition")
	}
}

func TestLower_ExprWithBadTail(t *testing.T) {
	field := "priority"
	badDate := "0000-00-00"
	g := &exprGrammar{
		Left: unaryExpr{FieldRef: &field},
		Tail: []exprBinTail{
			{Op: "+", Right: unaryExpr{DateLit: &badDate}},
		},
	}
	_, err := lowerExpr(g)
	if err == nil {
		t.Fatal("expected error for invalid date in expr tail")
	}
}

func TestLower_FuncCallWithBadArg(t *testing.T) {
	badDate := "0000-00-00"
	g := &funcCallExpr{
		Name: "count",
		Args: []exprGrammar{
			{Left: unaryExpr{DateLit: &badDate}},
		},
	}
	_, err := lowerFuncCall(g)
	if err == nil {
		t.Fatal("expected error for invalid date in func call arg")
	}
}

func TestLower_ListLitWithBadElement(t *testing.T) {
	badDate := "0000-00-00"
	g := &listLitExpr{
		Elements: []exprGrammar{
			{Left: unaryExpr{DateLit: &badDate}},
		},
	}
	_, err := lowerListLit(g)
	if err == nil {
		t.Fatal("expected error for invalid date in list literal element")
	}
}

func TestLower_SubQueryWithBadWhere(t *testing.T) {
	badDate := "0000-00-00"
	g := &subQueryExpr{
		Where: &orCond{
			Left: andCond{
				Left: notCond{
					Primary: &primaryCond{
						Expr: &exprCond{
							Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
							Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
						},
					},
				},
			},
		},
	}
	_, err := lowerSubQuery(g)
	if err == nil {
		t.Fatal("expected error for invalid date in subquery where")
	}
}

func TestLower_StatementWithBadSelect(t *testing.T) {
	badDate := "0000-00-00"
	g := &statementGrammar{
		Select: &selectGrammar{
			Where: &orCond{
				Left: andCond{
					Left: notCond{
						Primary: &primaryCond{
							Expr: &exprCond{
								Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
								Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
							},
						},
					},
				},
			},
		},
	}
	_, err := lowerStatement(g)
	if err == nil {
		t.Fatal("expected error for invalid date in select where")
	}
}

func TestLower_StatementWithBadCreate(t *testing.T) {
	badDate := "9999-99-99"
	g := &statementGrammar{
		Create: &createGrammar{
			Assignments: []assignmentGrammar{
				{Field: "due", Value: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
			},
		},
	}
	_, err := lowerStatement(g)
	if err == nil {
		t.Fatal("expected error for invalid date in create statement")
	}
}

func TestLower_StatementWithBadUpdate(t *testing.T) {
	badDate := "0000-00-00"
	g := &statementGrammar{
		Update: &updateGrammar{
			Where: orCond{
				Left: andCond{
					Left: notCond{
						Primary: &primaryCond{
							Expr: &exprCond{
								Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
								Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
							},
						},
					},
				},
			},
			Set: []assignmentGrammar{
				{Field: "title", Value: exprGrammar{Left: unaryExpr{StrLit: strPtr("x")}}},
			},
		},
	}
	_, err := lowerStatement(g)
	if err == nil {
		t.Fatal("expected error for invalid date in update statement")
	}
}

func TestLower_StatementWithBadDelete(t *testing.T) {
	badDate := "0000-00-00"
	g := &statementGrammar{
		Delete: &deleteGrammar{
			Where: orCond{
				Left: andCond{
					Left: notCond{
						Primary: &primaryCond{
							Expr: &exprCond{
								Left:    exprGrammar{Left: unaryExpr{DateLit: &badDate}},
								Compare: &compareTail{Op: "=", Right: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
							},
						},
					},
				},
			},
		},
	}
	_, err := lowerStatement(g)
	if err == nil {
		t.Fatal("expected error for invalid date in delete statement")
	}
}
