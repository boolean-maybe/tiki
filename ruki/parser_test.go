package ruki

import (
	"testing"
	"time"
)

// testSchema implements Schema for tests with standard tiki fields.
type testSchema struct{}

func (testSchema) Field(name string) (FieldSpec, bool) {
	fields := map[string]FieldSpec{
		"id":          {Name: "id", Type: ValueID},
		"title":       {Name: "title", Type: ValueString},
		"description": {Name: "description", Type: ValueString},
		"status":      {Name: "status", Type: ValueStatus},
		"type":        {Name: "type", Type: ValueTaskType},
		"tags":        {Name: "tags", Type: ValueListString},
		"dependsOn":   {Name: "dependsOn", Type: ValueListRef},
		"due":         {Name: "due", Type: ValueDate},
		"recurrence":  {Name: "recurrence", Type: ValueRecurrence},
		"assignee":    {Name: "assignee", Type: ValueString},
		"priority":    {Name: "priority", Type: ValueInt},
		"points":      {Name: "points", Type: ValueInt},
		"createdBy":   {Name: "createdBy", Type: ValueString},
		"createdAt":   {Name: "createdAt", Type: ValueTimestamp},
		"updatedAt":   {Name: "updatedAt", Type: ValueTimestamp},
	}
	f, ok := fields[name]
	return f, ok
}

func (testSchema) NormalizeStatus(raw string) (string, bool) {
	valid := map[string]string{
		"backlog":     "backlog",
		"ready":       "ready",
		"todo":        "ready",
		"in progress": "inProgress",
		"in_progress": "inProgress",
		"inProgress":  "inProgress",
		"review":      "review",
		"done":        "done",
		"cancelled":   "cancelled",
	}
	canonical, ok := valid[raw]
	return canonical, ok
}

func (testSchema) NormalizeType(raw string) (string, bool) {
	valid := map[string]string{
		"story":   "story",
		"feature": "story",
		"task":    "story",
		"bug":     "bug",
		"spike":   "spike",
		"epic":    "epic",
	}
	canonical, ok := valid[raw]
	return canonical, ok
}

func newTestParser() *Parser {
	return NewParser(testSchema{})
}

func TestParseSelect(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name      string
		input     string
		wantWhere bool
	}{
		{"select all", "select", false},
		{"select with where", `select where status = "done"`, true},
		{"select with and", `select where status = "done" and priority <= 2`, true},
		{"select with in", `select where "bug" in tags`, true},
		{"select with quantifier", `select where dependsOn any status != "done"`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if stmt.Select == nil {
				t.Fatal("expected Select, got nil")
				return
			}
			if tt.wantWhere && stmt.Select.Where == nil {
				t.Fatal("expected Where condition, got nil")
			}
			if !tt.wantWhere && stmt.Select.Where != nil {
				t.Fatal("expected nil Where, got condition")
			}
		})
	}
}

func TestParseCreate(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name       string
		input      string
		wantFields int
	}{
		{
			"basic create",
			`create title="Fix login" priority=2 status="ready" tags=["bug"]`,
			4,
		},
		{
			"single field",
			`create title="hello"`,
			1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if stmt.Create == nil {
				t.Fatal("expected Create, got nil")
			}
			if len(stmt.Create.Assignments) != tt.wantFields {
				t.Fatalf("expected %d assignments, got %d", tt.wantFields, len(stmt.Create.Assignments))
			}
		})
	}
}

func TestParseUpdate(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name    string
		input   string
		wantSet int
	}{
		{
			"update by id",
			`update where id = "TIKI-ABC123" set status="done"`,
			1,
		},
		{
			"update with complex where",
			`update where status = "ready" and "sprint-3" in tags set status="cancelled"`,
			1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if stmt.Update == nil {
				t.Fatal("expected Update, got nil")
			}
			if len(stmt.Update.Set) != tt.wantSet {
				t.Fatalf("expected %d set assignments, got %d", tt.wantSet, len(stmt.Update.Set))
			}
		})
	}
}

func TestParseDelete(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"delete by id", `delete where id = "TIKI-ABC123"`},
		{"delete with complex where", `delete where status = "cancelled" and "old" in tags`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if stmt.Delete == nil {
				t.Fatal("expected Delete, got nil")
				return
			}
			if stmt.Delete.Where == nil {
				t.Fatal("expected Where condition, got nil")
			}
		})
	}
}

func TestParseExpressions(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
		check func(t *testing.T, stmt *Statement)
	}{
		{
			"string literal in assignment",
			`create title="hello world"`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				sl, ok := stmt.Create.Assignments[0].Value.(*StringLiteral)
				if !ok {
					t.Fatalf("expected StringLiteral, got %T", stmt.Create.Assignments[0].Value)
					return
				}
				if sl.Value != "hello world" {
					t.Fatalf("expected %q, got %q", "hello world", sl.Value)
				}
			},
		},
		{
			"int literal in assignment",
			`create title="x" priority=2`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				il, ok := stmt.Create.Assignments[1].Value.(*IntLiteral)
				if !ok {
					t.Fatalf("expected IntLiteral, got %T", stmt.Create.Assignments[1].Value)
					return
				}
				if il.Value != 2 {
					t.Fatalf("expected 2, got %d", il.Value)
				}
			},
		},
		{
			"date literal in assignment",
			`create title="x" due=2026-03-25`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				dl, ok := stmt.Create.Assignments[1].Value.(*DateLiteral)
				if !ok {
					t.Fatalf("expected DateLiteral, got %T", stmt.Create.Assignments[1].Value)
				}
				expected := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
				if !dl.Value.Equal(expected) {
					t.Fatalf("expected %v, got %v", expected, dl.Value)
				}
			},
		},
		{
			"list literal in assignment",
			`create title="x" tags=["bug", "frontend"]`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				ll, ok := stmt.Create.Assignments[1].Value.(*ListLiteral)
				if !ok {
					t.Fatalf("expected ListLiteral, got %T", stmt.Create.Assignments[1].Value)
				}
				if len(ll.Elements) != 2 {
					t.Fatalf("expected 2 elements, got %d", len(ll.Elements))
				}
			},
		},
		{
			"empty literal in assignment",
			`create title="x" assignee=empty`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				if _, ok := stmt.Create.Assignments[1].Value.(*EmptyLiteral); !ok {
					t.Fatalf("expected EmptyLiteral, got %T", stmt.Create.Assignments[1].Value)
				}
			},
		},
		{
			"function call in assignment",
			`create title="x" due=next_date(recurrence)`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				fc, ok := stmt.Create.Assignments[1].Value.(*FunctionCall)
				if !ok {
					t.Fatalf("expected FunctionCall, got %T", stmt.Create.Assignments[1].Value)
					return
				}
				if fc.Name != "next_date" {
					t.Fatalf("expected next_date, got %s", fc.Name)
				}
				if len(fc.Args) != 1 {
					t.Fatalf("expected 1 arg, got %d", len(fc.Args))
				}
			},
		},
		{
			"binary plus expression",
			`create title="x" tags=tags + ["new"]`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				be, ok := stmt.Create.Assignments[1].Value.(*BinaryExpr)
				if !ok {
					t.Fatalf("expected BinaryExpr, got %T", stmt.Create.Assignments[1].Value)
					return
				}
				if be.Op != "+" {
					t.Fatalf("expected +, got %s", be.Op)
				}
			},
		},
		{
			"duration literal",
			`create title="x" due=2026-03-25 + 2day`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				be, ok := stmt.Create.Assignments[1].Value.(*BinaryExpr)
				if !ok {
					t.Fatalf("expected BinaryExpr, got %T", stmt.Create.Assignments[1].Value)
				}
				dur, ok := be.Right.(*DurationLiteral)
				if !ok {
					t.Fatalf("expected DurationLiteral, got %T", be.Right)
					return
				}
				if dur.Value != 2 || dur.Unit != "day" {
					t.Fatalf("expected 2day, got %d%s", dur.Value, dur.Unit)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			tt.check(t, stmt)
		})
	}
}

func TestParseConditions(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
		check func(t *testing.T, stmt *Statement)
	}{
		{
			"simple compare",
			`select where status = "done"`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				cmp, ok := stmt.Select.Where.(*CompareExpr)
				if !ok {
					t.Fatalf("expected CompareExpr, got %T", stmt.Select.Where)
					return
				}
				if cmp.Op != "=" {
					t.Fatalf("expected =, got %s", cmp.Op)
				}
			},
		},
		{
			"is empty",
			`select where assignee is empty`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				ie, ok := stmt.Select.Where.(*IsEmptyExpr)
				if !ok {
					t.Fatalf("expected IsEmptyExpr, got %T", stmt.Select.Where)
					return
				}
				if ie.Negated {
					t.Fatal("expected Negated=false")
				}
			},
		},
		{
			"is not empty",
			`select where description is not empty`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				ie, ok := stmt.Select.Where.(*IsEmptyExpr)
				if !ok {
					t.Fatalf("expected IsEmptyExpr, got %T", stmt.Select.Where)
				}
				if !ie.Negated {
					t.Fatal("expected Negated=true")
				}
			},
		},
		{
			"value in field",
			`select where "bug" in tags`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				in, ok := stmt.Select.Where.(*InExpr)
				if !ok {
					t.Fatalf("expected InExpr, got %T", stmt.Select.Where)
					return
				}
				if in.Negated {
					t.Fatal("expected Negated=false")
				}
			},
		},
		{
			"value not in list",
			`select where status not in ["done", "cancelled"]`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				in, ok := stmt.Select.Where.(*InExpr)
				if !ok {
					t.Fatalf("expected InExpr, got %T", stmt.Select.Where)
				}
				if !in.Negated {
					t.Fatal("expected Negated=true")
				}
			},
		},
		{
			"and precedence",
			`select where status = "done" and priority <= 2`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				bc, ok := stmt.Select.Where.(*BinaryCondition)
				if !ok {
					t.Fatalf("expected BinaryCondition, got %T", stmt.Select.Where)
					return
				}
				if bc.Op != "and" {
					t.Fatalf("expected and, got %s", bc.Op)
				}
			},
		},
		{
			"or precedence — and binds tighter",
			`select where priority = 1 or priority = 2 and status = "done"`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				// should parse as: priority=1 or (priority=2 and status="done")
				bc, ok := stmt.Select.Where.(*BinaryCondition)
				if !ok {
					t.Fatalf("expected BinaryCondition, got %T", stmt.Select.Where)
					return
				}
				if bc.Op != "or" {
					t.Fatalf("expected or at top, got %s", bc.Op)
				}
				// right side should be an and
				right, ok := bc.Right.(*BinaryCondition)
				if !ok {
					t.Fatalf("expected BinaryCondition on right, got %T", bc.Right)
					return
				}
				if right.Op != "and" {
					t.Fatalf("expected and on right, got %s", right.Op)
				}
			},
		},
		{
			"not condition",
			`select where not status = "done"`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				nc, ok := stmt.Select.Where.(*NotCondition)
				if !ok {
					t.Fatalf("expected NotCondition, got %T", stmt.Select.Where)
				}
				if _, ok := nc.Inner.(*CompareExpr); !ok {
					t.Fatalf("expected CompareExpr inside not, got %T", nc.Inner)
				}
			},
		},
		{
			"parenthesized condition",
			`select where (status = "done" or status = "cancelled") and priority = 1`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				bc, ok := stmt.Select.Where.(*BinaryCondition)
				if !ok {
					t.Fatalf("expected BinaryCondition, got %T", stmt.Select.Where)
					return
				}
				if bc.Op != "and" {
					t.Fatalf("expected and at top, got %s", bc.Op)
				}
				// left should be an or (the parenthesized group)
				left, ok := bc.Left.(*BinaryCondition)
				if !ok {
					t.Fatalf("expected BinaryCondition on left, got %T", bc.Left)
					return
				}
				if left.Op != "or" {
					t.Fatalf("expected or on left, got %s", left.Op)
				}
			},
		},
		{
			"quantifier any",
			`select where dependsOn any status != "done"`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				qe, ok := stmt.Select.Where.(*QuantifierExpr)
				if !ok {
					t.Fatalf("expected QuantifierExpr, got %T", stmt.Select.Where)
					return
				}
				if qe.Kind != "any" {
					t.Fatalf("expected any, got %s", qe.Kind)
				}
			},
		},
		{
			"quantifier all",
			`select where dependsOn all status = "done"`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				qe, ok := stmt.Select.Where.(*QuantifierExpr)
				if !ok {
					t.Fatalf("expected QuantifierExpr, got %T", stmt.Select.Where)
					return
				}
				if qe.Kind != "all" {
					t.Fatalf("expected all, got %s", qe.Kind)
				}
			},
		},
		{
			"quantifier binds to primary — and separates",
			`select where dependsOn any status != "done" and priority = 1`,
			func(t *testing.T, stmt *Statement) {
				t.Helper()
				// should parse as: (dependsOn any (status != "done")) and (priority = 1)
				bc, ok := stmt.Select.Where.(*BinaryCondition)
				if !ok {
					t.Fatalf("expected BinaryCondition at top, got %T", stmt.Select.Where)
					return
				}
				if bc.Op != "and" {
					t.Fatalf("expected and, got %s", bc.Op)
				}
				if _, ok := bc.Left.(*QuantifierExpr); !ok {
					t.Fatalf("expected QuantifierExpr on left, got %T", bc.Left)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			tt.check(t, stmt)
		})
	}
}

func TestParseQualifiedRefs(t *testing.T) {
	p := newTestParser()

	input := `select where status = "done"`
	stmt, err := p.ParseStatement(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	cmp, ok := stmt.Select.Where.(*CompareExpr)
	if !ok {
		t.Fatalf("expected CompareExpr, got %T", stmt.Select.Where)
	}
	fr, ok := cmp.Left.(*FieldRef)
	if !ok {
		t.Fatalf("expected FieldRef, got %T", cmp.Left)
		return
	}
	if fr.Name != "status" {
		t.Fatalf("expected status, got %s", fr.Name)
	}
}

func TestParseSubQuery(t *testing.T) {
	p := newTestParser()

	input := `select where count(select where status = "in progress" and assignee = "bob") >= 3`
	stmt, err := p.ParseStatement(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
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
		t.Fatalf("expected count, got %s", fc.Name)
	}

	sq, ok := fc.Args[0].(*SubQuery)
	if !ok {
		t.Fatalf("expected SubQuery arg, got %T", fc.Args[0])
		return
	}
	if sq.Where == nil {
		t.Fatal("expected SubQuery Where, got nil")
	}
}

func TestParseStatementErrors(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"empty input", ""},
		{"unknown keyword", "drop where id = 1"},
		{"missing where in update", `update set status="done"`},
		{"missing set in update", `update where id = "x"`},
		{"missing where in delete", `delete id = "x"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestParseSelectOrderBy(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name        string
		input       string
		wantWhere   bool
		wantOrderBy []OrderByClause
	}{
		{
			"order by single field",
			"select order by priority",
			false,
			[]OrderByClause{{Field: "priority", Desc: false}},
		},
		{
			"order by desc",
			"select order by priority desc",
			false,
			[]OrderByClause{{Field: "priority", Desc: true}},
		},
		{
			"order by asc",
			"select order by priority asc",
			false,
			[]OrderByClause{{Field: "priority", Desc: false}},
		},
		{
			"order by multiple fields",
			"select order by priority desc, createdAt asc",
			false,
			[]OrderByClause{
				{Field: "priority", Desc: true},
				{Field: "createdAt", Desc: false},
			},
		},
		{
			"order by mixed directions",
			"select order by status, priority desc, title",
			false,
			[]OrderByClause{
				{Field: "status", Desc: false},
				{Field: "priority", Desc: true},
				{Field: "title", Desc: false},
			},
		},
		{
			"where and order by",
			`select where status = "done" order by updatedAt desc`,
			true,
			[]OrderByClause{{Field: "updatedAt", Desc: true}},
		},
		{
			"where and order by multiple",
			`select where "bug" in tags order by priority asc, createdAt desc`,
			true,
			[]OrderByClause{
				{Field: "priority", Desc: false},
				{Field: "createdAt", Desc: true},
			},
		},
		{
			"select without order by still works",
			`select where status = "done"`,
			true,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if stmt.Select == nil {
				t.Fatal("expected Select")
				return
			}
			if tt.wantWhere && stmt.Select.Where == nil {
				t.Fatal("expected Where condition")
			}
			if !tt.wantWhere && stmt.Select.Where != nil {
				t.Fatal("unexpected Where condition")
			}
			if len(tt.wantOrderBy) == 0 && len(stmt.Select.OrderBy) != 0 {
				t.Fatalf("expected no OrderBy, got %v", stmt.Select.OrderBy)
			}
			if len(tt.wantOrderBy) != len(stmt.Select.OrderBy) {
				t.Fatalf("expected %d OrderBy clauses, got %d", len(tt.wantOrderBy), len(stmt.Select.OrderBy))
			}
			for i, want := range tt.wantOrderBy {
				got := stmt.Select.OrderBy[i]
				if got.Field != want.Field {
					t.Errorf("OrderBy[%d].Field = %q, want %q", i, got.Field, want.Field)
				}
				if got.Desc != want.Desc {
					t.Errorf("OrderBy[%d].Desc = %v, want %v", i, got.Desc, want.Desc)
				}
			}
		})
	}
}

func TestParseSelectFields(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name        string
		input       string
		wantFields  []string // nil = all fields
		wantWhere   bool
		wantOrderBy int
	}{
		{"bare select", "select", nil, false, 0},
		{"select star", "select *", nil, false, 0},
		{"single field", "select title", []string{"title"}, false, 0},
		{"two fields", "select id, title", []string{"id", "title"}, false, 0},
		{"many fields", "select id, title, status, priority", []string{"id", "title", "status", "priority"}, false, 0},
		{"fields + where", `select title, status where priority = 1`, []string{"title", "status"}, true, 0},
		{"single field + where", `select title where status = "done"`, []string{"title"}, true, 0},
		{"fields + order by", "select title order by priority", []string{"title"}, false, 1},
		{"fields + where + order by", `select id, title where status = "done" order by priority desc`, []string{"id", "title"}, true, 1},
		{"star + where", `select * where status = "done"`, nil, true, 0},
		{"star + order by", "select * order by title", nil, false, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if stmt.Select == nil {
				t.Fatal("expected Select")
			}

			// check fields
			if tt.wantFields == nil {
				if stmt.Select.Fields != nil {
					t.Fatalf("expected nil Fields (all), got %v", stmt.Select.Fields)
				}
			} else {
				if len(stmt.Select.Fields) != len(tt.wantFields) {
					t.Fatalf("expected %d fields, got %d: %v", len(tt.wantFields), len(stmt.Select.Fields), stmt.Select.Fields)
				}
				for i, want := range tt.wantFields {
					if stmt.Select.Fields[i] != want {
						t.Errorf("Fields[%d] = %q, want %q", i, stmt.Select.Fields[i], want)
					}
				}
			}

			// check where
			if tt.wantWhere && stmt.Select.Where == nil {
				t.Fatal("expected Where condition")
			}
			if !tt.wantWhere && stmt.Select.Where != nil {
				t.Fatal("unexpected Where condition")
			}

			// check order by
			if len(stmt.Select.OrderBy) != tt.wantOrderBy {
				t.Fatalf("expected %d OrderBy clauses, got %d", tt.wantOrderBy, len(stmt.Select.OrderBy))
			}
		})
	}
}

func TestParseSelectFieldsErrors(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"trailing comma", "select title,"},
		{"leading comma", "select , title"},
		{"star + named fields", "select *, title"},
		{"named fields + star", "select title, *"},
		{"double star", "select * *"},
		{"comma only", "select ,"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatalf("expected parse error for %q, got nil", tt.input)
			}
		})
	}
}

func TestParseComment(t *testing.T) {
	p := newTestParser()

	input := `-- this is a comment
select where status = "done"`
	stmt, err := p.ParseStatement(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if stmt.Select == nil {
		t.Fatal("expected Select")
	}
}
