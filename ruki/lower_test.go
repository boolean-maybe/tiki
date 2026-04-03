package ruki

import (
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

// TestLower_FuncCallArgs exercises function call lowering with multiple args.
func TestLower_FuncCallArgs(t *testing.T) {
	p := newTestParser()

	stmt, err := p.ParseStatement(`select where contains(title, "bug") = contains(title, "fix")`)
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
	if fc.Name != "contains" {
		t.Errorf("expected 'contains', got %q", fc.Name)
	}
	if len(fc.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(fc.Args))
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
	}
	if fc.Name != "count" {
		t.Errorf("expected 'count', got %q", fc.Name)
	}
	sq, ok := fc.Args[0].(*SubQuery)
	if !ok {
		t.Fatalf("expected SubQuery arg, got %T", fc.Args[0])
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
