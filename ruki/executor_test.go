package ruki

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/task"
)

func newTestExecutor() *Executor {
	return NewExecutor(testSchema{}, func() string { return "alice" })
}

func testDate(m time.Month, d int) time.Time {
	return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC)
}

func makeTasks() []*task.Task {
	return []*task.Task{
		{
			ID: "TIKI-000001", Title: "Setup CI", Status: "ready", Type: "story",
			Priority: 2, Tags: []string{"infra"}, Assignee: "alice",
			Due: testDate(4, 10), CreatedAt: testDate(3, 1),
		},
		{
			ID: "TIKI-000002", Title: "Fix login bug", Status: "in_progress", Type: "bug",
			Priority: 1, Tags: []string{"bug", "frontend"}, Assignee: "bob",
			Due: testDate(4, 5), DependsOn: []string{"TIKI-000001"},
			CreatedAt: testDate(3, 2),
		},
		{
			ID: "TIKI-000003", Title: "Write docs", Status: "done", Type: "story",
			Priority: 3, Tags: []string{"docs"}, Assignee: "alice",
			Due: testDate(4, 15), Points: 5, CreatedAt: testDate(3, 3),
		},
		{
			ID: "TIKI-000004", Title: "Plan sprint", Status: "backlog", Type: "spike",
			Priority: 2, Tags: []string{}, Assignee: "",
			CreatedAt: testDate(3, 4),
		},
	}
}

// --- nil guards ---

func TestExecuteNilStatement(t *testing.T) {
	e := newTestExecutor()
	_, err := e.Execute(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "nil statement") {
		t.Fatalf("expected 'nil statement' error, got: %v", err)
	}
}

func TestNewExecutorNilUserFunc(t *testing.T) {
	e := NewExecutor(testSchema{}, nil)
	tasks := []*task.Task{
		{ID: "T1", Title: "x", Status: "ready", Assignee: ""},
	}
	stmt := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &FieldRef{Name: "assignee"},
				Op:    "=",
				Right: &FunctionCall{Name: "user", Args: nil},
			},
		},
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 {
		t.Fatalf("expected 1 task (empty assignee matches empty user()), got %d", len(result.Select.Tasks))
	}
}

// --- basic select ---

func TestExecuteSelectAll(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement("select")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Select == nil {
		t.Fatal("expected Select result")
	}
	if len(result.Select.Tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(result.Select.Tasks))
	}
	if result.Select.Fields != nil {
		t.Fatalf("expected nil Fields, got %v", result.Select.Fields)
	}
}

func TestExecuteSelectWithFields(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement("select title, status")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(result.Select.Fields))
	}
	if result.Select.Fields[0] != "title" || result.Select.Fields[1] != "status" {
		t.Fatalf("unexpected fields: %v", result.Select.Fields)
	}
	if len(result.Select.Tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(result.Select.Tasks))
	}
}

// --- WHERE filtering ---

func TestExecuteSelectWhere(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	tests := []struct {
		name      string
		input     string
		wantCount int
		wantIDs   []string
	}{
		{
			"status equals", `select where status = "done"`,
			1, []string{"TIKI-000003"},
		},
		{
			"priority less than", `select where priority <= 2`,
			3, []string{"TIKI-000001", "TIKI-000002", "TIKI-000004"},
		},
		{
			"and condition", `select where status = "ready" and priority = 2`,
			1, []string{"TIKI-000001"},
		},
		{
			"or condition", `select where status = "done" or status = "backlog"`,
			2, []string{"TIKI-000003", "TIKI-000004"},
		},
		{
			"not condition", `select where not status = "done"`,
			3, []string{"TIKI-000001", "TIKI-000002", "TIKI-000004"},
		},
		{
			"in list", `select where status in ["done", "backlog"]`,
			2, []string{"TIKI-000003", "TIKI-000004"},
		},
		{
			"not in list", `select where status not in ["done", "backlog"]`,
			2, []string{"TIKI-000001", "TIKI-000002"},
		},
		{
			"value in tags", `select where "bug" in tags`,
			1, []string{"TIKI-000002"},
		},
		{
			"is empty", `select where assignee is empty`,
			1, []string{"TIKI-000004"},
		},
		{
			"is not empty", `select where assignee is not empty`,
			3, []string{"TIKI-000001", "TIKI-000002", "TIKI-000003"},
		},
		{
			"tags is empty", `select where tags is empty`,
			1, []string{"TIKI-000004"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			result, err := e.Execute(stmt, tasks)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if len(result.Select.Tasks) != tt.wantCount {
				ids := make([]string, len(result.Select.Tasks))
				for i, tk := range result.Select.Tasks {
					ids[i] = tk.ID
				}
				t.Fatalf("expected %d tasks, got %d: %v", tt.wantCount, len(result.Select.Tasks), ids)
			}
			for i, wantID := range tt.wantIDs {
				if result.Select.Tasks[i].ID != wantID {
					t.Errorf("task[%d].ID = %q, want %q", i, result.Select.Tasks[i].ID, wantID)
				}
			}
		})
	}
}

// --- ORDER BY ---

func TestExecuteSelectOrderBy(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	tests := []struct {
		name    string
		input   string
		wantIDs []string
	}{
		{
			"order by priority asc",
			"select order by priority",
			[]string{"TIKI-000002", "TIKI-000001", "TIKI-000004", "TIKI-000003"},
		},
		{
			"order by priority desc",
			"select order by priority desc",
			[]string{"TIKI-000003", "TIKI-000001", "TIKI-000004", "TIKI-000002"},
		},
		{
			"order by title asc",
			"select order by title",
			[]string{"TIKI-000002", "TIKI-000004", "TIKI-000001", "TIKI-000003"},
		},
		{
			"order by due",
			"select order by due",
			[]string{"TIKI-000004", "TIKI-000002", "TIKI-000001", "TIKI-000003"},
		},
		{
			"multi-field sort",
			"select order by priority, createdAt",
			[]string{"TIKI-000002", "TIKI-000001", "TIKI-000004", "TIKI-000003"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			result, err := e.Execute(stmt, tasks)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if len(result.Select.Tasks) != len(tt.wantIDs) {
				t.Fatalf("expected %d tasks, got %d", len(tt.wantIDs), len(result.Select.Tasks))
			}
			for i, wantID := range tt.wantIDs {
				if result.Select.Tasks[i].ID != wantID {
					t.Errorf("task[%d].ID = %q, want %q", i, result.Select.Tasks[i].ID, wantID)
				}
			}
		})
	}
}

func TestExecuteSelectNoOrderByPreservesInputOrder(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	tasks := []*task.Task{
		{ID: "TIKI-CCC", Title: "C", Status: "ready", Priority: 1},
		{ID: "TIKI-AAA", Title: "A", Status: "ready", Priority: 1},
		{ID: "TIKI-BBB", Title: "B", Status: "ready", Priority: 1},
	}

	stmt, err := p.ParseStatement("select")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	wantIDs := []string{"TIKI-CCC", "TIKI-AAA", "TIKI-BBB"}
	for i, wantID := range wantIDs {
		if result.Select.Tasks[i].ID != wantID {
			t.Errorf("task[%d].ID = %q, want %q — input order not preserved", i, result.Select.Tasks[i].ID, wantID)
		}
	}
}

// --- enum normalization ---

func TestExecuteEnumNormalization(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	tasks := []*task.Task{
		{ID: "TIKI-A", Title: "A", Status: "done", Type: "story"},
		{ID: "TIKI-B", Title: "B", Status: "in_progress", Type: "bug"},
		{ID: "TIKI-C", Title: "C", Status: "ready", Type: "story"},
	}

	tests := []struct {
		name      string
		input     string
		wantCount int
		wantIDs   []string
	}{
		{
			"status literal exact", `select where status = "done"`,
			1, []string{"TIKI-A"},
		},
		{
			"status alias todo->ready", `select where status = "todo"`,
			1, []string{"TIKI-C"},
		},
		{
			"status alias in progress", `select where status = "in progress"`,
			1, []string{"TIKI-B"},
		},
		{
			"type alias feature->story", `select where type = "feature"`,
			2, []string{"TIKI-A", "TIKI-C"},
		},
		{
			"type alias task->story", `select where type = "task"`,
			2, []string{"TIKI-A", "TIKI-C"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			result, err := e.Execute(stmt, tasks)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if len(result.Select.Tasks) != tt.wantCount {
				ids := make([]string, len(result.Select.Tasks))
				for i, tk := range result.Select.Tasks {
					ids[i] = tk.ID
				}
				t.Fatalf("expected %d tasks, got %d: %v", tt.wantCount, len(result.Select.Tasks), ids)
			}
			for i, wantID := range tt.wantIDs {
				if result.Select.Tasks[i].ID != wantID {
					t.Errorf("task[%d].ID = %q, want %q", i, result.Select.Tasks[i].ID, wantID)
				}
			}
		})
	}
}

// --- ID case-insensitive comparison ---

func TestExecuteIDCaseInsensitive(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`select where id = "tiki-000001"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Select.Tasks))
	}
	if result.Select.Tasks[0].ID != "TIKI-000001" {
		t.Fatalf("expected TIKI-000001, got %s", result.Select.Tasks[0].ID)
	}
}

// --- list set equality ---

func TestExecuteListSetEquality(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	tests := []struct {
		name      string
		tasks     []*task.Task
		input     string
		wantCount int
	}{
		{
			"order-insensitive match",
			[]*task.Task{
				{ID: "T1", Title: "x", Status: "ready", Tags: []string{"a", "b"}},
			},
			`select where tags = ["b", "a"]`,
			1,
		},
		{
			"multiplicity matters — different lengths",
			[]*task.Task{
				{ID: "T1", Title: "x", Status: "ready", Tags: []string{"a"}},
			},
			`select where tags = ["a", "b"]`,
			0,
		},
		{
			"multiplicity matters — duplicate vs single",
			[]*task.Task{
				{ID: "T1", Title: "x", Status: "ready", Tags: []string{"a", "a"}},
			},
			`select where tags = ["a"]`,
			0,
		},
		{
			"empty list equality",
			[]*task.Task{
				{ID: "T1", Title: "x", Status: "ready", Tags: []string{}},
			},
			`select where tags = []`,
			1,
		},
		{
			"nil tags equals empty list",
			[]*task.Task{
				{ID: "T1", Title: "x", Status: "ready"},
			},
			`select where tags = []`,
			1,
		},
		{
			"list inequality",
			[]*task.Task{
				{ID: "T1", Title: "x", Status: "ready", Tags: []string{"a"}},
			},
			`select where tags != ["a", "b"]`,
			1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			result, err := e.Execute(stmt, tt.tasks)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if len(result.Select.Tasks) != tt.wantCount {
				t.Fatalf("expected %d tasks, got %d", tt.wantCount, len(result.Select.Tasks))
			}
		})
	}
}

// --- comparison matrix ---

func TestExecuteComparisonMatrix(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	tasks := []*task.Task{
		{
			ID: "TIKI-A", Title: "Alpha", Status: "done", Type: "bug",
			Priority: 5, Points: 3, Due: testDate(6, 15),
			CreatedAt: testDate(1, 1), UpdatedAt: testDate(3, 1),
		},
	}

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// string =, !=
		{"string eq match", `select where title = "Alpha"`, true},
		{"string eq no match", `select where title = "Beta"`, false},
		{"string neq", `select where title != "Beta"`, true},
		// int =, !=, <, >, <=, >=
		{"int eq", `select where priority = 5`, true},
		{"int neq", `select where priority != 5`, false},
		{"int lt", `select where priority < 10`, true},
		{"int gt", `select where priority > 10`, false},
		{"int lte", `select where priority <= 5`, true},
		{"int gte", `select where priority >= 5`, true},
		// date ordering
		{"date lt", `select where due < 2026-07-01`, true},
		{"date gt", `select where due > 2026-07-01`, false},
		{"date eq", `select where due = 2026-06-15`, true},
		// timestamp field-to-field
		{"timestamp lt field", `select where createdAt < updatedAt`, true},
		{"timestamp eq self", `select where createdAt = createdAt`, true},
		// status =, !=
		{"status eq", `select where status = "done"`, true},
		{"status neq", `select where status != "done"`, false},
		// type =, !=
		{"type eq", `select where type = "bug"`, true},
		{"type neq", `select where type != "bug"`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			result, err := e.Execute(stmt, tasks)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			got := len(result.Select.Tasks) > 0
			if got != tt.want {
				t.Fatalf("expected match=%v, got %v", tt.want, got)
			}
		})
	}
}

// --- functions ---

func TestExecuteInSubstring(t *testing.T) {
	e := newTestExecutor()
	tasks := makeTasks()

	tests := []struct {
		name    string
		query   string
		wantIDs []string
	}{
		{"match", `select where "bug" in title`, []string{"TIKI-000002"}},
		{"negated", `select where "bug" not in title`, []string{"TIKI-000001", "TIKI-000003", "TIKI-000004"}},
		{"assignee", `select where "ali" in assignee`, []string{"TIKI-000001", "TIKI-000003"}},
		{"no match", `select where "xyz" in title`, nil},
		{"empty needle", `select where "" in title`, []string{"TIKI-000001", "TIKI-000002", "TIKI-000003", "TIKI-000004"}},
		{"case sensitive", `select where "BUG" in title`, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := newTestParser().ParseStatement(tt.query)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			result, err := e.Execute(stmt, tasks)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			got := make([]string, len(result.Select.Tasks))
			for i, tk := range result.Select.Tasks {
				got[i] = tk.ID
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("expected %v, got %v", tt.wantIDs, got)
			}
			for i := range got {
				if got[i] != tt.wantIDs[i] {
					t.Fatalf("expected %v, got %v", tt.wantIDs, got)
				}
			}
		})
	}
}

func TestExecuteUser(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`select where assignee = user()`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 2 {
		t.Fatalf("expected 2 tasks assigned to alice, got %d", len(result.Select.Tasks))
	}
}

func TestExecuteCount(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`select where count(select where status = "done") >= 1`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// count(done) = 1 which is >= 1, so the condition is true for every task
	if len(result.Select.Tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(result.Select.Tasks))
	}
}

func TestExecuteNextDate(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	tasks := []*task.Task{
		{
			ID: "T1", Title: "Daily", Status: "ready",
			Recurrence: task.RecurrenceDaily,
		},
		{
			ID: "T2", Title: "No recurrence", Status: "ready",
			Recurrence: task.RecurrenceNone,
		},
	}

	stmt, err := p.ParseStatement(`select where next_date(recurrence) is not empty`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 || result.Select.Tasks[0].ID != "T1" {
		t.Fatalf("expected T1, got %v", result.Select.Tasks)
	}
}

func TestExecuteBlocks(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks() // TIKI-000002 depends on TIKI-000001

	stmt, err := p.ParseStatement(`select where id in blocks("TIKI-000001")`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 || result.Select.Tasks[0].ID != "TIKI-000002" {
		t.Fatalf("expected TIKI-000002, got %v", result.Select.Tasks)
	}
}

// --- call() phase-1 rejection ---

func TestExecuteCallRejected(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`select where call("echo hello") = "hello"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = e.Execute(stmt, tasks)
	if err == nil {
		t.Fatal("expected error for call()")
	}
	if !strings.Contains(err.Error(), "call()") {
		t.Fatalf("expected error mentioning call(), got: %v", err)
	}
}

// --- CREATE execution ---

func TestExecuteCreateBasic(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	stmt, err := p.ParseStatement(`create title="Fix login" priority=2 status="ready"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Create == nil {
		t.Fatal("expected Create result")
	}
	tk := result.Create.Task
	if tk.Title != "Fix login" {
		t.Errorf("title = %q, want %q", tk.Title, "Fix login")
	}
	if tk.Priority != 2 {
		t.Errorf("priority = %d, want 2", tk.Priority)
	}
	if tk.Status != "ready" {
		t.Errorf("status = %q, want %q", tk.Status, "ready")
	}
}

func TestExecuteCreateWithUser(t *testing.T) {
	e := newTestExecutor() // userFunc returns "alice"
	p := newTestParser()

	stmt, err := p.ParseStatement(`create title="test" assignee=user()`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Create.Task.Assignee != "alice" {
		t.Errorf("assignee = %q, want %q", result.Create.Task.Assignee, "alice")
	}
}

func TestExecuteCreateEnumNormalization(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	stmt, err := p.ParseStatement(`create title="test" status="todo" type="feature"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	tk := result.Create.Task
	if tk.Status != "ready" {
		t.Errorf("status = %q, want normalized %q", tk.Status, "ready")
	}
	if tk.Type != "story" {
		t.Errorf("type = %q, want normalized %q", tk.Type, "story")
	}
}

func TestExecuteCreateImmutableFieldRejected(t *testing.T) {
	e := newTestExecutor()

	for _, field := range []string{"id", "createdBy", "createdAt", "updatedAt"} {
		t.Run(field, func(t *testing.T) {
			stmt := &Statement{
				Create: &CreateStmt{
					Assignments: []Assignment{
						{Field: "title", Value: &StringLiteral{Value: "x"}},
						{Field: field, Value: &StringLiteral{Value: "test"}},
					},
				},
			}
			_, err := e.Execute(stmt, nil)
			if err == nil {
				t.Fatal("expected error for immutable field")
			}
			if !strings.Contains(err.Error(), "immutable") {
				t.Errorf("expected immutable error, got: %v", err)
			}
		})
	}
}

func TestExecuteCreateEmptyTitleRejected(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	stmt, err := p.ParseStatement(`create title=""`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = e.Execute(stmt, nil)
	if err == nil {
		t.Fatal("expected error for empty title")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty error, got: %v", err)
	}
}

func TestExecuteCreateExprError(t *testing.T) {
	e := newTestExecutor()

	stmt := &Statement{
		Create: &CreateStmt{
			Assignments: []Assignment{
				{Field: "title", Value: &QualifiedRef{Qualifier: "old", Name: "title"}},
			},
		},
	}
	_, err := e.Execute(stmt, nil)
	if err == nil {
		t.Fatal("expected error from eval expression")
	}
}

func TestExecuteCreateListField(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	stmt, err := p.ParseStatement(`create title="test" tags=["a","b"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	tags := result.Create.Task.Tags
	if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Errorf("tags = %v, want [a b]", tags)
	}
}

func TestExecuteCreateDateField(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	stmt, err := p.ParseStatement(`create title="test" due=2026-06-01`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	due := result.Create.Task.Due
	if due.Year() != 2026 || due.Month() != 6 || due.Day() != 1 {
		t.Errorf("due = %v, want 2026-06-01", due)
	}
}

func TestExecuteCreatePriorityOutOfRange(t *testing.T) {
	e := newTestExecutor()

	for _, prio := range []int{0, 99, -1} {
		t.Run(fmt.Sprintf("priority=%d", prio), func(t *testing.T) {
			stmt := &Statement{
				Create: &CreateStmt{
					Assignments: []Assignment{
						{Field: "title", Value: &StringLiteral{Value: "x"}},
						{Field: "priority", Value: &IntLiteral{Value: prio}},
					},
				},
			}
			_, err := e.Execute(stmt, nil)
			if err == nil {
				t.Fatal("expected error for out-of-range priority")
			}
		})
	}
}

func TestExecuteCreateEmptyTasks(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	stmt, err := p.ParseStatement(`create title="test"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, []*task.Task{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Create.Task.Title != "test" {
		t.Errorf("title = %q, want %q", result.Create.Task.Title, "test")
	}
}

func TestExecuteCreateWithTemplate(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	// set template with tags=["idea"] and priority=7
	e.SetTemplate(&task.Task{
		Tags:     []string{"idea"},
		Priority: 7,
		Status:   "ready",
		Type:     "story",
	})

	stmt, err := p.ParseStatement(`create title="x" tags=tags+["new"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	tk := result.Create.Task
	// tags should be template's ["idea"] + ["new"]
	if len(tk.Tags) != 2 || tk.Tags[0] != "idea" || tk.Tags[1] != "new" {
		t.Errorf("tags = %v, want [idea new]", tk.Tags)
	}
	// priority should be preserved from template (not set by assignment)
	if tk.Priority != 7 {
		t.Errorf("priority = %d, want 7 (template default)", tk.Priority)
	}
}

func TestExecuteCreateWithoutTemplate(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	// no SetTemplate call — template is nil

	stmt, err := p.ParseStatement(`create title="x" priority=3`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	tk := result.Create.Task
	if tk.Title != "x" {
		t.Errorf("title = %q, want %q", tk.Title, "x")
	}
	if tk.Priority != 3 {
		t.Errorf("priority = %d, want 3", tk.Priority)
	}
	// unset fields should be zero-valued
	if tk.Points != 0 {
		t.Errorf("points = %d, want 0 (zero-value)", tk.Points)
	}
}

// --- DELETE execution ---

func TestExecuteDeleteBasic(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`delete where id = "TIKI-000001"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Delete == nil {
		t.Fatal("expected Delete result")
	}
	if len(result.Delete.Deleted) != 1 {
		t.Fatalf("expected 1 deleted, got %d", len(result.Delete.Deleted))
	}
	if result.Delete.Deleted[0].ID != "TIKI-000001" {
		t.Errorf("deleted ID = %q, want TIKI-000001", result.Delete.Deleted[0].ID)
	}
}

func TestExecuteDeleteMultipleMatches(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`delete where type = "story"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// TIKI-000001 and TIKI-000003 are stories
	if len(result.Delete.Deleted) != 2 {
		t.Fatalf("expected 2 deleted, got %d", len(result.Delete.Deleted))
	}
}

func TestExecuteDeleteNoMatches(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`delete where id = "NONEXISTENT"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Delete.Deleted) != 0 {
		t.Fatalf("expected 0 deleted, got %d", len(result.Delete.Deleted))
	}
}

func TestExecuteDeleteWhereError(t *testing.T) {
	e := newTestExecutor()
	tasks := makeTasks()

	stmt := &Statement{
		Delete: &DeleteStmt{
			Where: &CompareExpr{
				Left:  &QualifiedRef{Qualifier: "old", Name: "status"},
				Op:    "=",
				Right: &StringLiteral{Value: "done"},
			},
		},
	}
	_, err := e.Execute(stmt, tasks)
	if err == nil {
		t.Fatal("expected error from WHERE evaluation")
	}
}

// --- quantifier ---

func TestExecuteQuantifier(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	tests := []struct {
		name      string
		input     string
		wantCount int
	}{
		{
			"any — dep not done",
			`select where dependsOn any status != "done"`,
			1, // TIKI-000002 depends on TIKI-000001 (status=ready, not done)
		},
		{
			"all — all deps done (vacuously true for no deps)",
			`select where dependsOn all status = "done"`,
			3, // TIKI-000001, TIKI-000003, TIKI-000004 (no deps = vacuous truth)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			result, err := e.Execute(stmt, tasks)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if len(result.Select.Tasks) != tt.wantCount {
				ids := make([]string, len(result.Select.Tasks))
				for i, tk := range result.Select.Tasks {
					ids[i] = tk.ID
				}
				t.Fatalf("expected %d tasks, got %d: %v", tt.wantCount, len(result.Select.Tasks), ids)
			}
		})
	}
}

// --- date arithmetic in WHERE ---

func TestExecuteDateArithmetic(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	tasks := []*task.Task{
		{ID: "T1", Title: "Soon", Status: "ready", Due: testDate(4, 5)},
		{ID: "T2", Title: "Later", Status: "ready", Due: testDate(5, 1)},
	}

	stmt, err := p.ParseStatement(`select where due <= 2026-04-01 + 7day`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 || result.Select.Tasks[0].ID != "T1" {
		t.Fatalf("expected T1, got %v", result.Select.Tasks)
	}
}

// --- qualified ref rejection ---

func TestExecuteQualifiedRefRejected(t *testing.T) {
	e := newTestExecutor()

	stmt := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &QualifiedRef{Qualifier: "old", Name: "status"},
				Op:    "=",
				Right: &StringLiteral{Value: "done"},
			},
		},
	}

	_, err := e.Execute(stmt, makeTasks())
	if err == nil {
		t.Fatal("expected error for qualified ref")
	}
	if !strings.Contains(err.Error(), "qualified") {
		t.Fatalf("expected qualified ref error, got: %v", err)
	}
}

// --- stable sort test ---

func TestExecuteSortStable(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	tasks := []*task.Task{
		{ID: "T1", Title: "First", Status: "ready", Priority: 1},
		{ID: "T2", Title: "Second", Status: "ready", Priority: 1},
		{ID: "T3", Title: "Third", Status: "ready", Priority: 1},
		{ID: "T4", Title: "Fourth", Status: "ready", Priority: 2},
	}

	stmt, err := p.ParseStatement("select order by priority")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// priority=1 tasks should preserve input order: T1, T2, T3
	wantIDs := []string{"T1", "T2", "T3", "T4"}
	for i, wantID := range wantIDs {
		if result.Select.Tasks[i].ID != wantID {
			t.Errorf("task[%d].ID = %q, want %q", i, result.Select.Tasks[i].ID, wantID)
		}
	}
}

// --- empty statement ---

func TestExecuteEmptyStatement(t *testing.T) {
	e := newTestExecutor()
	_, err := e.Execute(&Statement{}, nil)
	if err == nil || !strings.Contains(err.Error(), "empty statement") {
		t.Fatalf("expected 'empty statement' error, got: %v", err)
	}
}

// --- compareWithNil ---

func TestExecuteCompareWithNil(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{
		{ID: "T1", Title: "x", Status: "ready", Assignee: ""},
		{ID: "T2", Title: "y", Status: "ready", Assignee: "bob"},
	}

	tests := []struct {
		name    string
		op      string
		wantIDs []string
	}{
		{"field = empty", "=", []string{"T1"}},
		{"field != empty", "!=", []string{"T2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := &Statement{
				Select: &SelectStmt{
					Where: &CompareExpr{
						Left:  &FieldRef{Name: "assignee"},
						Op:    tt.op,
						Right: &EmptyLiteral{},
					},
				},
			}
			result, err := e.Execute(stmt, tasks)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if len(result.Select.Tasks) != len(tt.wantIDs) {
				t.Fatalf("expected %d tasks, got %d", len(tt.wantIDs), len(result.Select.Tasks))
			}
			for i, wantID := range tt.wantIDs {
				if result.Select.Tasks[i].ID != wantID {
					t.Errorf("task[%d].ID = %q, want %q", i, result.Select.Tasks[i].ID, wantID)
				}
			}
		})
	}

	// ordering op with nil returns false, no error
	stmt := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &FieldRef{Name: "assignee"},
				Op:    "<",
				Right: &EmptyLiteral{},
			},
		},
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 0 {
		t.Fatalf("expected 0 tasks for < nil, got %d", len(result.Select.Tasks))
	}
}

// --- duration comparison ---

func TestExecuteDurationComparison(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready"}}

	tests := []struct {
		name string
		op   string
		l, r int
		want bool
	}{
		{"eq true", "=", 2, 2, true},
		{"eq false", "=", 1, 2, false},
		{"neq", "!=", 1, 2, true},
		{"lt", "<", 1, 2, true},
		{"gt", ">", 2, 1, true},
		{"lte", "<=", 2, 2, true},
		{"gte", ">=", 3, 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := &Statement{
				Select: &SelectStmt{
					Where: &CompareExpr{
						Left:  &DurationLiteral{Value: tt.l, Unit: "day"},
						Op:    tt.op,
						Right: &DurationLiteral{Value: tt.r, Unit: "day"},
					},
				},
			}
			result, err := e.Execute(stmt, tasks)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			got := len(result.Select.Tasks) > 0
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

// --- subtractValues ---

func TestExecuteSubtractValues(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{
		{ID: "T1", Title: "x", Status: "ready", Priority: 10, Due: testDate(6, 15)},
	}

	tests := []struct {
		name  string
		left  Expr
		right Expr
		op    string
		cmp   Expr
		want  bool
	}{
		{
			"int - int",
			&BinaryExpr{Op: "-", Left: &FieldRef{Name: "priority"}, Right: &IntLiteral{Value: 5}},
			nil, "=", &IntLiteral{Value: 5}, true,
		},
		{
			"date - duration",
			&BinaryExpr{Op: "-", Left: &FieldRef{Name: "due"}, Right: &DurationLiteral{Value: 1, Unit: "day"}},
			nil, "=", &DateLiteral{Value: testDate(6, 14)}, true,
		},
		{
			"date - date yields duration",
			&BinaryExpr{
				Op:    "-",
				Left:  &DateLiteral{Value: testDate(6, 17)},
				Right: &DateLiteral{Value: testDate(6, 15)},
			},
			nil, "=", &DurationLiteral{Value: 2, Unit: "day"}, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := &Statement{
				Select: &SelectStmt{
					Where: &CompareExpr{Left: tt.left, Op: tt.op, Right: tt.cmp},
				},
			}
			result, err := e.Execute(stmt, tasks)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			got := len(result.Select.Tasks) > 0
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

// --- addValues additional branches ---

func TestExecuteAddValuesIntAndString(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{
		{ID: "T1", Title: "x", Status: "ready", Priority: 3},
	}

	// int + int
	stmt := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &BinaryExpr{Op: "+", Left: &IntLiteral{Value: 2}, Right: &IntLiteral{Value: 3}},
				Op:    "=",
				Right: &IntLiteral{Value: 5},
			},
		},
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 {
		t.Fatalf("expected 1, got %d", len(result.Select.Tasks))
	}

	// string + string
	stmt2 := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &BinaryExpr{Op: "+", Left: &StringLiteral{Value: "hello"}, Right: &StringLiteral{Value: " world"}},
				Op:    "=",
				Right: &StringLiteral{Value: "hello world"},
			},
		},
	}
	result, err = e.Execute(stmt2, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 {
		t.Fatalf("expected 1, got %d", len(result.Select.Tasks))
	}
}

// --- sorting by status, type, recurrence, and nil fields ---

func TestExecuteSortByStatusTypeRecurrence(t *testing.T) {
	e := newTestExecutor()

	tasks := []*task.Task{
		{ID: "T1", Title: "a", Status: "done", Type: "bug", Recurrence: "0 0 * * *"},
		{ID: "T2", Title: "b", Status: "backlog", Type: "story", Recurrence: ""},
		{ID: "T3", Title: "c", Status: "ready", Type: "epic", Recurrence: "0 0 1 * *"},
	}

	tests := []struct {
		name    string
		field   string
		wantIDs []string
	}{
		{"by status", "status", []string{"T2", "T1", "T3"}},
		{"by type", "type", []string{"T1", "T3", "T2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := &Statement{
				Select: &SelectStmt{
					OrderBy: []OrderByClause{{Field: tt.field}},
				},
			}
			result, err := e.Execute(stmt, tasks)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			for i, wantID := range tt.wantIDs {
				if result.Select.Tasks[i].ID != wantID {
					t.Errorf("task[%d].ID = %q, want %q", i, result.Select.Tasks[i].ID, wantID)
				}
			}
		})
	}
}

// --- extractField additional branches ---

func TestExtractFieldAllFields(t *testing.T) {
	tk := &task.Task{
		ID: "T1", Title: "hi", Description: "desc", Status: "ready",
		Type: "bug", Priority: 1, Points: 3, Tags: []string{"a"},
		DependsOn: []string{"T2"}, Due: testDate(1, 1),
		Recurrence: task.RecurrenceDaily, Assignee: "bob",
		CreatedBy: "alice", CreatedAt: testDate(1, 1), UpdatedAt: testDate(2, 1),
	}

	fields := []string{
		"id", "title", "description", "status", "type", "priority",
		"points", "tags", "dependsOn", "due", "recurrence", "assignee",
		"createdBy", "createdAt", "updatedAt",
	}
	for _, f := range fields {
		v := extractField(tk, f)
		if v == nil {
			t.Errorf("extractField(%q) returned nil", f)
		}
	}
	if v := extractField(tk, "nonexistent"); v != nil {
		t.Errorf("extractField(nonexistent) should be nil, got %v", v)
	}
}

// --- isZeroValue full coverage ---

func TestIsZeroValue(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"non-empty string", "x", false},
		{"zero int", 0, true},
		{"non-zero int", 1, false},
		{"zero time", time.Time{}, true},
		{"non-zero time", testDate(1, 1), false},
		{"zero duration", time.Duration(0), true},
		{"non-zero duration", time.Hour, false},
		{"false bool", false, true},
		{"true bool", true, false},
		{"empty status", task.Status(""), true},
		{"non-empty status", task.Status("done"), false},
		{"empty type", task.Type(""), true},
		{"non-empty type", task.Type("bug"), false},
		{"empty recurrence", task.Recurrence(""), true},
		{"non-empty recurrence", task.RecurrenceDaily, false},
		{"empty list", []interface{}{}, true},
		{"non-empty list", []interface{}{"a"}, false},
		{"unknown type", struct{}{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isZeroValue(tt.val); got != tt.want {
				t.Errorf("isZeroValue(%v) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

// --- durationToTimeDelta full coverage ---

func TestDurationLiteralUnknownUnitError(t *testing.T) {
	e := &Executor{}
	tasks := []*task.Task{{ID: "TIKI-AAA001", Title: "test"}}
	// unknown unit should produce an error, not silently default to days
	stmt, err := newTestParser().ParseStatement(`select where due > 2026-01-01 + 1day`)
	if err != nil {
		t.Fatal(err)
	}
	// manually inject an unknown unit into the AST
	cmp, ok := stmt.Select.Where.(*CompareExpr)
	if !ok {
		t.Fatal("expected *CompareExpr")
	}
	add, ok := cmp.Right.(*BinaryExpr)
	if !ok {
		t.Fatal("expected *BinaryExpr")
	}
	add.Right = &DurationLiteral{Value: 1, Unit: "bogus"}

	_, err = e.Execute(stmt, tasks)
	if err == nil {
		t.Fatal("expected error for unknown duration unit, got nil")
	}
}

// --- normalizeToString coverage ---

func TestNormalizeToString(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want string
	}{
		{"string", "hello", "hello"},
		{"status", task.Status("done"), "done"},
		{"type", task.Type("bug"), "bug"},
		{"recurrence", task.Recurrence("0 0 * * *"), "0 0 * * *"},
		{"int", 42, "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeToString(tt.val); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- compareForSort nil handling and all type branches ---

func TestCompareForSort(t *testing.T) {
	tests := []struct {
		name string
		a, b interface{}
		want int
	}{
		{"nil nil", nil, nil, 0},
		{"nil left", nil, 1, -1},
		{"nil right", 1, nil, 1},
		{"int lt", 1, 2, -1},
		{"int eq", 2, 2, 0},
		{"int gt", 3, 2, 1},
		{"string", "a", "b", -1},
		{"status", task.Status("a"), task.Status("b"), -1},
		{"type", task.Type("a"), task.Type("b"), -1},
		{"time before", testDate(1, 1), testDate(2, 1), -1},
		{"time equal", testDate(1, 1), testDate(1, 1), 0},
		{"time after", testDate(3, 1), testDate(2, 1), 1},
		{"recurrence", task.Recurrence("a"), task.Recurrence("b"), -1},
		{"duration lt", time.Hour, 2 * time.Hour, -1},
		{"duration eq", time.Hour, time.Hour, 0},
		{"duration gt", 2 * time.Hour, time.Hour, 1},
		{"fallback", struct{ x int }{1}, struct{ x int }{2}, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareForSort(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

// --- toInt non-int branch ---

func TestToInt(t *testing.T) {
	if v, ok := toInt(42); !ok || v != 42 {
		t.Errorf("expected (42, true), got (%d, %v)", v, ok)
	}
	if _, ok := toInt("not int"); ok {
		t.Error("expected false for string")
	}
}

// --- count with nil where (counts all) ---

func TestExecuteCountNoWhere(t *testing.T) {
	e := newTestExecutor()
	tasks := makeTasks()

	stmt := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left: &FunctionCall{
					Name: "count",
					Args: []Expr{&SubQuery{Where: nil}},
				},
				Op:    "=",
				Right: &IntLiteral{Value: 4},
			},
		},
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 4 {
		t.Fatalf("expected 4, got %d", len(result.Select.Tasks))
	}
}

// --- ID != comparison ---

func TestExecuteIDNotEqual(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-A", Title: "a", Status: "ready"},
		{ID: "TIKI-B", Title: "b", Status: "ready"},
	}

	stmt, err := p.ParseStatement(`select where id != "tiki-a"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 || result.Select.Tasks[0].ID != "TIKI-B" {
		t.Fatalf("expected TIKI-B only, got %v", result.Select.Tasks)
	}
}

// --- recurrence field comparison ---

func TestExecuteRecurrenceComparison(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{
		{ID: "T1", Title: "x", Status: "ready", Recurrence: task.RecurrenceDaily},
		{ID: "T2", Title: "y", Status: "ready", Recurrence: task.RecurrenceNone},
	}

	stmt := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &FieldRef{Name: "recurrence"},
				Op:    "=",
				Right: &FieldRef{Name: "recurrence"},
			},
		},
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 2 {
		t.Fatalf("expected 2, got %d", len(result.Select.Tasks))
	}
}

// --- type normalization with unknown value fallback ---

func TestExecuteNormalizeFallback(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{
		{ID: "T1", Title: "x", Status: "unknown_status_xyz", Type: "unknown_type_xyz"},
	}

	// status with unknown value passes through unchanged
	stmt := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &FieldRef{Name: "status"},
				Op:    "=",
				Right: &StringLiteral{Value: "unknown_status_xyz"},
			},
		},
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 {
		t.Fatalf("expected 1, got %d", len(result.Select.Tasks))
	}

	// type with unknown value passes through unchanged
	stmt2 := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &FieldRef{Name: "type"},
				Op:    "=",
				Right: &StringLiteral{Value: "unknown_type_xyz"},
			},
		},
	}
	result, err = e.Execute(stmt2, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 {
		t.Fatalf("expected 1, got %d", len(result.Select.Tasks))
	}
}

// --- time comparison all operators ---

func TestExecuteTimeComparisonAllOps(t *testing.T) {
	e := newTestExecutor()
	d1 := testDate(3, 1)
	d2 := testDate(6, 1)
	tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready"}}

	tests := []struct {
		name string
		op   string
		l, r time.Time
		want bool
	}{
		{"eq", "=", d1, d1, true},
		{"neq", "!=", d1, d2, true},
		{"lt", "<", d1, d2, true},
		{"gt", ">", d2, d1, true},
		{"lte", "<=", d1, d1, true},
		{"gte", ">=", d1, d1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := &Statement{
				Select: &SelectStmt{
					Where: &CompareExpr{
						Left:  &DateLiteral{Value: tt.l},
						Op:    tt.op,
						Right: &DateLiteral{Value: tt.r},
					},
				},
			}
			result, err := e.Execute(stmt, tasks)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			got := len(result.Select.Tasks) > 0
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

// --- error propagation tests ---

func TestExecuteErrorPropagation(t *testing.T) {
	e := newTestExecutor()
	tasks := makeTasks()

	badExpr := &QualifiedRef{Qualifier: "old", Name: "status"}

	tests := []struct {
		name string
		stmt *Statement
	}{
		{
			"evalCompare left error",
			&Statement{Select: &SelectStmt{
				Where: &CompareExpr{Left: badExpr, Op: "=", Right: &StringLiteral{Value: "x"}},
			}},
		},
		{
			"evalCompare right error",
			&Statement{Select: &SelectStmt{
				Where: &CompareExpr{Left: &StringLiteral{Value: "x"}, Op: "=", Right: badExpr},
			}},
		},
		{
			"evalIsEmpty error",
			&Statement{Select: &SelectStmt{
				Where: &IsEmptyExpr{Expr: badExpr},
			}},
		},
		{
			"evalIn value error",
			&Statement{Select: &SelectStmt{
				Where: &InExpr{Value: badExpr, Collection: &FieldRef{Name: "tags"}},
			}},
		},
		{
			"evalIn collection error",
			&Statement{Select: &SelectStmt{
				Where: &InExpr{Value: &StringLiteral{Value: "x"}, Collection: badExpr},
			}},
		},
		{
			"evalQuantifier expr error",
			&Statement{Select: &SelectStmt{
				Where: &QuantifierExpr{Expr: badExpr, Kind: "any", Condition: &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}}},
			}},
		},
		{
			"evalBinaryExpr left error",
			&Statement{Select: &SelectStmt{
				Where: &CompareExpr{
					Left:  &BinaryExpr{Op: "+", Left: badExpr, Right: &IntLiteral{Value: 1}},
					Op:    "=",
					Right: &IntLiteral{Value: 1},
				},
			}},
		},
		{
			"evalBinaryExpr right error",
			&Statement{Select: &SelectStmt{
				Where: &CompareExpr{
					Left:  &BinaryExpr{Op: "+", Left: &IntLiteral{Value: 1}, Right: badExpr},
					Op:    "=",
					Right: &IntLiteral{Value: 1},
				},
			}},
		},
		{
			"evalListLiteral element error",
			&Statement{Select: &SelectStmt{
				Where: &CompareExpr{
					Left:  &ListLiteral{Elements: []Expr{badExpr}},
					Op:    "=",
					Right: &ListLiteral{Elements: []Expr{&StringLiteral{Value: "x"}}},
				},
			}},
		},
		{
			"unknown function",
			&Statement{Select: &SelectStmt{
				Where: &CompareExpr{
					Left:  &FunctionCall{Name: "nonexistent", Args: nil},
					Op:    "=",
					Right: &IntLiteral{Value: 1},
				},
			}},
		},
		{
			"subquery not in count",
			&Statement{Select: &SelectStmt{
				Where: &CompareExpr{
					Left:  &SubQuery{Where: nil},
					Op:    "=",
					Right: &IntLiteral{Value: 1},
				},
			}},
		},
		{
			"binary expr unknown op",
			&Statement{Select: &SelectStmt{
				Where: &CompareExpr{
					Left:  &BinaryExpr{Op: "*", Left: &IntLiteral{Value: 1}, Right: &IntLiteral{Value: 1}},
					Op:    "=",
					Right: &IntLiteral{Value: 1},
				},
			}},
		},
		{
			"add type mismatch",
			&Statement{Select: &SelectStmt{
				Where: &CompareExpr{
					Left:  &BinaryExpr{Op: "+", Left: &IntLiteral{Value: 1}, Right: &StringLiteral{Value: "x"}},
					Op:    "=",
					Right: &IntLiteral{Value: 1},
				},
			}},
		},
		{
			"subtract type mismatch",
			&Statement{Select: &SelectStmt{
				Where: &CompareExpr{
					Left:  &BinaryExpr{Op: "-", Left: &IntLiteral{Value: 1}, Right: &StringLiteral{Value: "x"}},
					Op:    "=",
					Right: &IntLiteral{Value: 1},
				},
			}},
		},
		{
			"not condition inner error",
			&Statement{Select: &SelectStmt{
				Where: &NotCondition{Inner: &CompareExpr{Left: badExpr, Op: "=", Right: &StringLiteral{Value: "x"}}},
			}},
		},
		{
			"binary condition left error",
			&Statement{Select: &SelectStmt{
				Where: &BinaryCondition{
					Op:    "and",
					Left:  &CompareExpr{Left: badExpr, Op: "=", Right: &StringLiteral{Value: "x"}},
					Right: &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
				},
			}},
		},
		{
			"next_date arg error",
			&Statement{Select: &SelectStmt{
				Where: &CompareExpr{
					Left:  &FunctionCall{Name: "next_date", Args: []Expr{badExpr}},
					Op:    "=",
					Right: &DateLiteral{Value: testDate(1, 1)},
				},
			}},
		},
		{
			"blocks arg error",
			&Statement{Select: &SelectStmt{
				Where: &CompareExpr{
					Left:  &FunctionCall{Name: "blocks", Args: []Expr{badExpr}},
					Op:    "=",
					Right: &ListLiteral{Elements: nil},
				},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.Execute(tt.stmt, tasks)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

// --- in substring with literal collection ---

func TestExecuteInSubstringLiteral(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready"}}

	stmt := &Statement{
		Select: &SelectStmt{
			Where: &InExpr{
				Value:      &StringLiteral{Value: "x"},
				Collection: &StringLiteral{Value: "not a list"},
			},
		},
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "x" is not a substring of "not a list" → no match
	if len(result.Select.Tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(result.Select.Tasks))
	}
}

// --- in fail-fast for non-list/non-string runtime values (hand-built AST) ---

func TestExecuteInNonListNonString(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready"}}

	t.Run("int collection", func(t *testing.T) {
		stmt := &Statement{
			Select: &SelectStmt{
				Where: &InExpr{
					Value:      &IntLiteral{Value: 1},
					Collection: &IntLiteral{Value: 42},
				},
			},
		}
		_, err := e.Execute(stmt, tasks)
		if err == nil || !strings.Contains(err.Error(), "not a list or string") {
			t.Fatalf("expected 'not a list or string' error, got: %v", err)
		}
	})

	t.Run("string collection non-string value", func(t *testing.T) {
		stmt := &Statement{
			Select: &SelectStmt{
				Where: &InExpr{
					Value:      &IntLiteral{Value: 1},
					Collection: &StringLiteral{Value: "abc"},
				},
			},
		}
		_, err := e.Execute(stmt, tasks)
		if err == nil || !strings.Contains(err.Error(), "substring check requires string value") {
			t.Fatalf("expected 'substring check requires string value' error, got: %v", err)
		}
	})
}

// --- quantifier with non-list expression ---

func TestExecuteQuantifierNonList(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready"}}

	stmt := &Statement{
		Select: &SelectStmt{
			Where: &QuantifierExpr{
				Expr:      &StringLiteral{Value: "not a list"},
				Kind:      "any",
				Condition: &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
			},
		},
	}
	_, err := e.Execute(stmt, tasks)
	if err == nil || !strings.Contains(err.Error(), "not a list") {
		t.Fatalf("expected 'not a list' error, got: %v", err)
	}
}

// --- next_date with non-recurrence value ---

func TestExecuteNextDateNonRecurrence(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready"}}

	stmt := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &FunctionCall{Name: "next_date", Args: []Expr{&StringLiteral{Value: "not recurrence"}}},
				Op:    "=",
				Right: &DateLiteral{Value: testDate(1, 1)},
			},
		},
	}
	_, err := e.Execute(stmt, tasks)
	if err == nil || !strings.Contains(err.Error(), "recurrence") {
		t.Fatalf("expected recurrence error, got: %v", err)
	}
}

// --- count non-subquery arg ---

func TestExecuteCountNonSubquery(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready"}}

	stmt := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &FunctionCall{Name: "count", Args: []Expr{&IntLiteral{Value: 1}}},
				Op:    "=",
				Right: &IntLiteral{Value: 1},
			},
		},
	}
	_, err := e.Execute(stmt, tasks)
	if err == nil || !strings.Contains(err.Error(), "subquery") {
		t.Fatalf("expected subquery error, got: %v", err)
	}
}

// --- quantifier condition error propagation ---

func TestExecuteQuantifierConditionError(t *testing.T) {
	e := newTestExecutor()
	badCond := &CompareExpr{Left: &QualifiedRef{Qualifier: "old", Name: "status"}, Op: "=", Right: &StringLiteral{Value: "x"}}

	tasks := []*task.Task{
		{ID: "T1", Title: "x", Status: "ready", DependsOn: []string{"T2"}},
		{ID: "T2", Title: "y", Status: "done"},
	}

	// any with error in condition
	stmt := &Statement{Select: &SelectStmt{
		Where: &QuantifierExpr{Expr: &FieldRef{Name: "dependsOn"}, Kind: "any", Condition: badCond},
	}}
	_, err := e.Execute(stmt, tasks)
	if err == nil {
		t.Fatal("expected error from quantifier any")
	}

	// all with error in condition
	stmt2 := &Statement{Select: &SelectStmt{
		Where: &QuantifierExpr{Expr: &FieldRef{Name: "dependsOn"}, Kind: "all", Condition: badCond},
	}}
	_, err = e.Execute(stmt2, tasks)
	if err == nil {
		t.Fatal("expected error from quantifier all")
	}

	// unknown quantifier kind
	stmt3 := &Statement{Select: &SelectStmt{
		Where: &QuantifierExpr{
			Expr: &FieldRef{Name: "dependsOn"}, Kind: "none",
			Condition: &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
		},
	}}
	_, err = e.Execute(stmt3, tasks)
	if err == nil || !strings.Contains(err.Error(), "unknown quantifier") {
		t.Fatalf("expected unknown quantifier error, got: %v", err)
	}
}

// --- count subquery condition error ---

func TestExecuteCountSubqueryError(t *testing.T) {
	e := newTestExecutor()
	tasks := makeTasks()

	stmt := &Statement{Select: &SelectStmt{
		Where: &CompareExpr{
			Left: &FunctionCall{Name: "count", Args: []Expr{
				&SubQuery{Where: &CompareExpr{
					Left: &QualifiedRef{Qualifier: "old", Name: "status"}, Op: "=", Right: &StringLiteral{Value: "x"},
				}},
			}},
			Op:    "=",
			Right: &IntLiteral{Value: 1},
		},
	}}
	_, err := e.Execute(stmt, tasks)
	if err == nil {
		t.Fatal("expected error from count subquery")
	}
}

// --- resolveComparisonType right-side field ---

func TestExecuteResolveComparisonTypeRightSide(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{
		{ID: "TIKI-A", Title: "x", Status: "ready"},
	}

	// literal on left, field on right — resolveComparisonType checks right side
	stmt := &Statement{Select: &SelectStmt{
		Where: &CompareExpr{
			Left:  &StringLiteral{Value: "tiki-a"},
			Op:    "=",
			Right: &FieldRef{Name: "id"},
		},
	}}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 {
		t.Fatalf("expected 1, got %d", len(result.Select.Tasks))
	}

	// literal on left, status field on right
	stmt2 := &Statement{Select: &SelectStmt{
		Where: &CompareExpr{
			Left:  &StringLiteral{Value: "todo"},
			Op:    "=",
			Right: &FieldRef{Name: "status"},
		},
	}}
	result, err = e.Execute(stmt2, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Select.Tasks) != 1 {
		t.Fatalf("expected 1 (todo->ready), got %d", len(result.Select.Tasks))
	}

	// literal on left, type field on right
	stmt3 := &Statement{Select: &SelectStmt{
		Where: &CompareExpr{
			Left:  &StringLiteral{Value: "feature"},
			Op:    "!=",
			Right: &FieldRef{Name: "type"},
		},
	}}
	result, err = e.Execute(stmt3, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// status is "ready", type is "" — "feature" normalizes to "story", "" doesn't normalize → not equal
	if len(result.Select.Tasks) != 1 {
		t.Fatalf("expected 1, got %d", len(result.Select.Tasks))
	}
}

// --- exprFieldType with unknown field ---

func TestExprFieldType(t *testing.T) {
	e := newTestExecutor()
	// non-FieldRef returns -1
	if got := e.exprFieldType(&StringLiteral{Value: "x"}); got != -1 {
		t.Errorf("expected -1 for StringLiteral, got %d", got)
	}
	// unknown field returns -1
	if got := e.exprFieldType(&FieldRef{Name: "nonexistent"}); got != -1 {
		t.Errorf("expected -1 for unknown field, got %d", got)
	}
	// known field returns its type
	if got := e.exprFieldType(&FieldRef{Name: "status"}); got != ValueStatus {
		t.Errorf("expected ValueStatus, got %d", got)
	}
}

// --- unknown condition type ---

type fakeCondition struct{}

func (*fakeCondition) conditionNode() {}

func TestExecuteUnknownConditionType(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready"}}

	stmt := &Statement{Select: &SelectStmt{Where: &fakeCondition{}}}
	_, err := e.Execute(stmt, tasks)
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected unknown condition type error, got: %v", err)
	}
}

// --- unknown binary condition operator ---

func TestExecuteUnknownBinaryConditionOp(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready"}}

	stmt := &Statement{Select: &SelectStmt{
		Where: &BinaryCondition{
			Op:    "xor",
			Left:  &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
			Right: &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
		},
	}}
	_, err := e.Execute(stmt, tasks)
	if err == nil || !strings.Contains(err.Error(), "unknown binary operator") {
		t.Fatalf("expected unknown binary operator error, got: %v", err)
	}
}

// --- unknown expression type ---

type fakeExpr struct{}

func (*fakeExpr) exprNode() {}

func TestExecuteUnknownExprType(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready"}}

	stmt := &Statement{Select: &SelectStmt{
		Where: &CompareExpr{Left: &fakeExpr{}, Op: "=", Right: &IntLiteral{Value: 1}},
	}}
	_, err := e.Execute(stmt, tasks)
	if err == nil || !strings.Contains(err.Error(), "unknown expression type") {
		t.Fatalf("expected unknown expression type error, got: %v", err)
	}
}

// --- blocks returning empty list ---

func TestExecuteBlocksNoBlockers(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{
		{ID: "T1", Title: "x", Status: "ready"},
		{ID: "T2", Title: "y", Status: "ready"},
	}

	stmt := &Statement{Select: &SelectStmt{
		Where: &CompareExpr{
			Left:  &FunctionCall{Name: "blocks", Args: []Expr{&FieldRef{Name: "id"}}},
			Op:    "=",
			Right: &ListLiteral{Elements: nil},
		},
	}}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// no task depends on any other, so blocks() returns [] for all → all match
	if len(result.Select.Tasks) != 2 {
		t.Fatalf("expected 2, got %d", len(result.Select.Tasks))
	}
}

// --- now() function ---

func TestExecuteNow(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{
		{ID: "T1", Title: "x", Status: "ready", Due: testDate(12, 31)},
	}

	stmt := &Statement{Select: &SelectStmt{
		Where: &CompareExpr{
			Left:  &FieldRef{Name: "due"},
			Op:    ">",
			Right: &FunctionCall{Name: "now", Args: nil},
		},
	}}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// 2026-12-31 should be after now
	if len(result.Select.Tasks) != 1 {
		t.Fatalf("expected 1, got %d", len(result.Select.Tasks))
	}
}

// --- sort by status and type (covers compareForSort branches already
// tested via TestCompareForSort, but exercises them through sortTasks) ---

func TestExecuteSortByStatus(t *testing.T) {
	e := newTestExecutor()

	tasks := []*task.Task{
		{ID: "T1", Title: "a", Status: "ready"},
		{ID: "T2", Title: "b", Status: "done"},
		{ID: "T3", Title: "c", Status: "backlog"},
	}

	stmt := &Statement{Select: &SelectStmt{
		OrderBy: []OrderByClause{{Field: "status", Desc: true}},
	}}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// desc: ready > done > backlog
	if result.Select.Tasks[0].ID != "T1" {
		t.Errorf("expected T1 first (ready), got %s", result.Select.Tasks[0].ID)
	}
}

// --- compareValues unsupported type fallback ---

func TestExecuteCompareUnsupportedType(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready"}}

	// DurationLiteral on left compared with int on right — type mismatch
	stmt := &Statement{Select: &SelectStmt{
		Where: &CompareExpr{
			Left:  &DurationLiteral{Value: 1, Unit: "day"},
			Op:    "=",
			Right: &IntLiteral{Value: 1},
		},
	}}
	_, err := e.Execute(stmt, tasks)
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
}

// --- comparison helper error branches ---

func TestComparisonHelperErrors(t *testing.T) {
	// all comparison helpers reject unknown operators
	if _, err := compareStrings("a", "b", "<"); err == nil {
		t.Error("compareStrings should reject <")
	}
	if _, err := compareStringsCI("a", "b", "<"); err == nil {
		t.Error("compareStringsCI should reject <")
	}
	if _, err := compareBools(true, false, "<"); err == nil {
		t.Error("compareBools should reject <")
	}
	if _, err := compareIntValues(1, 2, "~"); err == nil {
		t.Error("compareIntValues should reject ~")
	}
	if _, err := compareTimes(time.Now(), time.Now(), "~"); err == nil {
		t.Error("compareTimes should reject ~")
	}
	if _, err := compareDurations(time.Hour, time.Hour, "~"); err == nil {
		t.Error("compareDurations should reject ~")
	}
	if _, err := compareListEquality(nil, nil, "<"); err == nil {
		t.Error("compareListEquality should reject <")
	}
}

// --- sortedMultisetEqual with same-length but different elements ---

func TestSortedMultisetEqualMismatch(t *testing.T) {
	a := []interface{}{"x", "y"}
	b := []interface{}{"x", "z"}
	if sortedMultisetEqual(a, b) {
		t.Error("expected false for different elements")
	}
}

// --- compareValues with task.Status/Type falling through (non-field context) ---

func TestCompareValuesStatusTypeDirect(t *testing.T) {
	e := newTestExecutor()

	// status value vs string without FieldRef context — falls through to task.Status case
	ok, err := e.compareValues(task.Status("done"), "done", "=", &StringLiteral{Value: "done"}, &StringLiteral{Value: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected status done = done")
	}

	// type value vs string
	ok, err = e.compareValues(task.Type("bug"), "bug", "=", &StringLiteral{Value: "bug"}, &StringLiteral{Value: "bug"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected type bug = bug")
	}

	// int vs non-int
	_, err = e.compareValues(42, "not int", "=", &IntLiteral{Value: 42}, &StringLiteral{Value: "not int"})
	if err == nil {
		t.Error("expected error for int vs string")
	}

	// time vs non-time
	_, err = e.compareValues(time.Now(), "not time", "=", &DateLiteral{Value: time.Now()}, &StringLiteral{Value: "not time"})
	if err == nil {
		t.Error("expected error for time vs string")
	}

	// unsupported type
	_, err = e.compareValues(struct{}{}, struct{}{}, "=", &IntLiteral{Value: 1}, &IntLiteral{Value: 1})
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

// --- evalQuantifier: all where one ref fails condition ---

func TestExecuteQuantifierAllFailing(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{
		{ID: "T1", Title: "x", Status: "ready", DependsOn: []string{"T2", "T3"}},
		{ID: "T2", Title: "y", Status: "done"},
		{ID: "T3", Title: "z", Status: "ready"},
	}

	stmt, err := newTestParser().ParseStatement(`select where dependsOn all status = "done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// T1 depends on T2(done) and T3(ready) — not all done
	// T2 has no deps → vacuous truth
	// T3 has no deps → vacuous truth
	wantCount := 2
	if len(result.Select.Tasks) != wantCount {
		ids := make([]string, len(result.Select.Tasks))
		for i, tk := range result.Select.Tasks {
			ids[i] = tk.ID
		}
		t.Fatalf("expected %d tasks, got %d: %v", wantCount, len(result.Select.Tasks), ids)
	}
}

func TestExecuteQuantifierAllPassing(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{
		{ID: "T1", Title: "x", Status: "ready", DependsOn: []string{"T2", "T3"}},
		{ID: "T2", Title: "y", Status: "done"},
		{ID: "T3", Title: "z", Status: "done"},
	}

	stmt, err := newTestParser().ParseStatement(`select where dependsOn all status = "done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// all 3: T1 deps are T2+T3 both done, T2+T3 have no deps (vacuous)
	if len(result.Select.Tasks) != 3 {
		ids := make([]string, len(result.Select.Tasks))
		for i, tk := range result.Select.Tasks {
			ids[i] = tk.ID
		}
		t.Fatalf("expected 3 tasks, got %d: %v", len(result.Select.Tasks), ids)
	}
}

// --- UPDATE execution ---

func TestExecuteUpdateSingleField(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "old title", Status: "ready"},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set title="new title"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update == nil {
		t.Fatal("expected Update result")
	}
	if len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated task, got %d", len(result.Update.Updated))
	}
	if result.Update.Updated[0].Title != "new title" {
		t.Errorf("expected title 'new title', got %q", result.Update.Updated[0].Title)
	}
}

func TestExecuteUpdateMultipleFields(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Priority: 3},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set status="done" priority=1`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	u := result.Update.Updated[0]
	if u.Status != "done" {
		t.Errorf("expected status 'done', got %q", u.Status)
	}
	if u.Priority != 1 {
		t.Errorf("expected priority 1, got %d", u.Priority)
	}
}

func TestExecuteUpdateMatchesMultipleTasks(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "T1", Title: "a", Status: "ready", Priority: 1},
		{ID: "T2", Title: "b", Status: "ready", Priority: 2},
		{ID: "T3", Title: "c", Status: "done", Priority: 3},
	}

	stmt, err := p.ParseStatement(`update where status = "ready" set status="done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Update.Updated) != 2 {
		t.Fatalf("expected 2 updated, got %d", len(result.Update.Updated))
	}
	for _, u := range result.Update.Updated {
		if u.Status != "done" {
			t.Errorf("task %s status = %q, want done", u.ID, u.Status)
		}
	}
}

func TestExecuteUpdateMatchesNoTasks(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`update where id = "NONEXISTENT" set title="x"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Update.Updated) != 0 {
		t.Fatalf("expected 0 updated, got %d", len(result.Update.Updated))
	}
}

func TestExecuteUpdateWithComplexWhere(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`update where priority < 3 and "bug" in tags set status="done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result.Update.Updated) != 1 {
		t.Fatalf("expected 1 updated, got %d", len(result.Update.Updated))
	}
	if result.Update.Updated[0].ID != "TIKI-000002" {
		t.Errorf("expected TIKI-000002, got %s", result.Update.Updated[0].ID)
	}
}

func TestExecuteUpdateWithFieldReference(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", CreatedBy: "alice", Assignee: ""},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set assignee=createdBy`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update.Updated[0].Assignee != "alice" {
		t.Errorf("expected assignee 'alice', got %q", result.Update.Updated[0].Assignee)
	}
}

func TestExecuteUpdateWithFunction(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Recurrence: task.RecurrenceDaily},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set due=next_date(recurrence)`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update.Updated[0].Due.IsZero() {
		t.Error("expected non-zero due date after next_date()")
	}
}

func TestExecuteUpdateListField(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Tags: []string{"old"}},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set tags=["a","b"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	u := result.Update.Updated[0]
	if len(u.Tags) != 2 || u.Tags[0] != "a" || u.Tags[1] != "b" {
		t.Errorf("expected tags [a b], got %v", u.Tags)
	}
}

func TestExecuteUpdateListPlusList(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Tags: []string{"old"}},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set tags=tags+["new"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	u := result.Update.Updated[0]
	if len(u.Tags) != 2 || u.Tags[0] != "old" || u.Tags[1] != "new" {
		t.Errorf("expected tags [old new], got %v", u.Tags)
	}
}

func TestExecuteUpdateListPlusElement(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Tags: []string{"old"}},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set tags=tags+"new"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	u := result.Update.Updated[0]
	if len(u.Tags) != 2 || u.Tags[0] != "old" || u.Tags[1] != "new" {
		t.Errorf("expected tags [old new], got %v", u.Tags)
	}
}

func TestExecuteUpdateListMinusList(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Tags: []string{"old", "keep"}},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set tags=tags-["old"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	u := result.Update.Updated[0]
	if len(u.Tags) != 1 || u.Tags[0] != "keep" {
		t.Errorf("expected tags [keep], got %v", u.Tags)
	}
}

func TestExecuteUpdateListMinusElement(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Tags: []string{"old", "keep"}},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set tags=tags-"old"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	u := result.Update.Updated[0]
	if len(u.Tags) != 1 || u.Tags[0] != "keep" {
		t.Errorf("expected tags [keep], got %v", u.Tags)
	}
}

func TestExecuteUpdateListMinusDuplicates(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Tags: []string{"old", "old", "keep"}},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set tags=tags-"old"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	u := result.Update.Updated[0]
	if len(u.Tags) != 1 || u.Tags[0] != "keep" {
		t.Errorf("expected tags [keep], got %v", u.Tags)
	}
}

func TestExecuteUpdateDependsOnPlusElement(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", DependsOn: []string{}},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set dependsOn=dependsOn+"TIKI-Y"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	u := result.Update.Updated[0]
	if len(u.DependsOn) != 1 || u.DependsOn[0] != "TIKI-Y" {
		t.Errorf("expected dependsOn [TIKI-Y], got %v", u.DependsOn)
	}
}

func TestExecuteUpdateDependsOnPlusList(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", DependsOn: []string{"TIKI-Z"}},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set dependsOn=dependsOn+["TIKI-Y"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	u := result.Update.Updated[0]
	if len(u.DependsOn) != 2 || u.DependsOn[0] != "TIKI-Z" || u.DependsOn[1] != "TIKI-Y" {
		t.Errorf("expected dependsOn [TIKI-Z TIKI-Y], got %v", u.DependsOn)
	}
}

func TestExecuteUpdateDependsOnMinusElement(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", DependsOn: []string{"TIKI-Y", "TIKI-Z"}},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set dependsOn=dependsOn-"TIKI-Y"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	u := result.Update.Updated[0]
	if len(u.DependsOn) != 1 || u.DependsOn[0] != "TIKI-Z" {
		t.Errorf("expected dependsOn [TIKI-Z], got %v", u.DependsOn)
	}
}

func TestExecuteUpdateDependsOnMinusList(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", DependsOn: []string{"TIKI-Y", "TIKI-Z"}},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set dependsOn=dependsOn-["TIKI-Y"]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	u := result.Update.Updated[0]
	if len(u.DependsOn) != 1 || u.DependsOn[0] != "TIKI-Z" {
		t.Errorf("expected dependsOn [TIKI-Z], got %v", u.DependsOn)
	}
}

func TestExecuteUpdateTagsToEmpty(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Tags: []string{"a", "b"}},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set tags=empty`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update.Updated[0].Tags != nil {
		t.Errorf("expected nil tags, got %v", result.Update.Updated[0].Tags)
	}
}

func TestExecuteUpdateStringToEmpty(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Assignee: "alice"},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set assignee=empty`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update.Updated[0].Assignee != "" {
		t.Errorf("expected empty assignee, got %q", result.Update.Updated[0].Assignee)
	}
}

func TestExecuteUpdateDateToEmpty(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Due: testDate(6, 1)},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set due=empty`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.Update.Updated[0].Due.IsZero() {
		t.Errorf("expected zero due, got %v", result.Update.Updated[0].Due)
	}
}

func TestExecuteUpdateConstrainedFieldsRejectEmpty(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"title", `update where id = "T1" set title=empty`},
		{"priority", `update where id = "T1" set priority=empty`},
		{"status", `update where id = "T1" set status=empty`},
		{"type", `update where id = "T1" set type=empty`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready", Type: "bug", Priority: 2}}
			stmt, err := p.ParseStatement(tt.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, err = e.Execute(stmt, tasks)
			if err == nil {
				t.Fatal("expected error for empty on constrained field")
			}
			if !strings.Contains(err.Error(), "empty") {
				t.Errorf("expected error mentioning empty, got: %v", err)
			}
		})
	}
}

func TestExecuteUpdateImmutableFieldRejected(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready"},
	}

	fields := []string{"id", "createdBy", "createdAt", "updatedAt"}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			stmt := &Statement{
				Update: &UpdateStmt{
					Where: &CompareExpr{
						Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "TIKI-000001"},
					},
					Set: []Assignment{{Field: field, Value: &StringLiteral{Value: "test"}}},
				},
			}
			_, err := e.Execute(stmt, tasks)
			if err == nil {
				t.Fatal("expected error for immutable field")
			}
			if !strings.Contains(err.Error(), "immutable") {
				t.Errorf("expected immutable error, got: %v", err)
			}
		})
	}
}

func TestExecuteUpdateEnumNormalization(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "done"},
	}

	// "todo" is an alias for "ready" in the test schema
	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set status="todo"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update.Updated[0].Status != "ready" {
		t.Errorf("expected canonical 'ready', got %q", result.Update.Updated[0].Status)
	}
}

func TestExecuteUpdateOriginalUnmodified(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "old", Status: "ready"},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set title="new"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if tasks[0].Title != "old" {
		t.Errorf("original task was mutated: title = %q, want 'old'", tasks[0].Title)
	}
}

// --- list arithmetic (addValues/subtractValues) ---

func TestAddValuesList(t *testing.T) {
	tests := []struct {
		name  string
		left  []interface{}
		right interface{}
		want  []interface{}
	}{
		{"list + list", []interface{}{"a"}, []interface{}{"b"}, []interface{}{"a", "b"}},
		{"list + element", []interface{}{"a"}, "b", []interface{}{"a", "b"}},
		{"empty + list", []interface{}{}, []interface{}{"a"}, []interface{}{"a"}},
		{"list + empty list", []interface{}{"a"}, []interface{}{}, []interface{}{"a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := addValues(tt.left, tt.right)
			if err != nil {
				t.Fatalf("addValues error: %v", err)
			}
			got, ok := result.([]interface{})
			if !ok {
				t.Fatalf("expected []interface{}, got %T", result)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d elements, got %d: %v", len(tt.want), len(got), got)
			}
			for i := range tt.want {
				if normalizeToString(got[i]) != normalizeToString(tt.want[i]) {
					t.Errorf("element %d: got %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSubtractValuesList(t *testing.T) {
	tests := []struct {
		name  string
		left  []interface{}
		right interface{}
		want  []interface{}
	}{
		{"list - list", []interface{}{"a", "b", "c"}, []interface{}{"b"}, []interface{}{"a", "c"}},
		{"list - element", []interface{}{"a", "b"}, "a", []interface{}{"b"}},
		{"remove all occurrences", []interface{}{"a", "a", "b"}, "a", []interface{}{"b"}},
		{"remove nothing", []interface{}{"a", "b"}, "c", []interface{}{"a", "b"}},
		{"remove all", []interface{}{"a"}, "a", []interface{}{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := subtractValues(tt.left, tt.right)
			if err != nil {
				t.Fatalf("subtractValues error: %v", err)
			}
			got, ok := result.([]interface{})
			if !ok {
				t.Fatalf("expected []interface{}, got %T", result)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d elements, got %d: %v", len(tt.want), len(got), got)
			}
			for i := range tt.want {
				if normalizeToString(got[i]) != normalizeToString(tt.want[i]) {
					t.Errorf("element %d: got %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExecuteUpdateRecurrenceToEmpty(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Recurrence: task.RecurrenceDaily},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set recurrence=empty`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update.Updated[0].Recurrence != "" {
		t.Errorf("expected empty recurrence, got %q", result.Update.Updated[0].Recurrence)
	}
}

func TestExecuteUpdatePointsToEmpty(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Points: 5},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set points=empty`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update.Updated[0].Points != 0 {
		t.Errorf("expected 0 points, got %d", result.Update.Updated[0].Points)
	}
}

func TestExecuteUpdateUnknownField(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{
		{ID: "T1", Title: "x", Status: "ready"},
	}

	stmt := &Statement{
		Update: &UpdateStmt{
			Where: &CompareExpr{
				Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "T1"},
			},
			Set: []Assignment{{Field: "nonexistent", Value: &StringLiteral{Value: "x"}}},
		},
	}
	_, err := e.Execute(stmt, tasks)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("expected 'unknown field' error, got: %v", err)
	}
}

func TestExecuteUpdateTypeNormalization(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Type: "bug"},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set type="feature"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update.Updated[0].Type != "story" {
		t.Errorf("expected normalized type 'story', got %q", result.Update.Updated[0].Type)
	}
}

func TestExecuteUpdateDescriptionToEmpty(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", Description: "some desc"},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set description=empty`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update.Updated[0].Description != "" {
		t.Errorf("expected empty description, got %q", result.Update.Updated[0].Description)
	}
}

func TestExecuteUpdateTitleToEmptyRejected(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "old title", Status: "ready"},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set title=empty`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = e.Execute(stmt, tasks)
	if err == nil {
		t.Fatal("expected error for title=empty")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected error mentioning empty, got: %v", err)
	}
}

func TestExecuteUpdateDependsOnToEmpty(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "x", Status: "ready", DependsOn: []string{"T2"}},
	}

	stmt, err := p.ParseStatement(`update where id = "TIKI-000001" set dependsOn=empty`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Update.Updated[0].DependsOn != nil {
		t.Errorf("expected nil dependsOn, got %v", result.Update.Updated[0].DependsOn)
	}
}

// --- executeUpdate error branches ---

func TestExecuteUpdateWhereError(t *testing.T) {
	e := newTestExecutor()
	tasks := makeTasks()

	// WHERE clause with a bad expr triggers filterTasks error
	stmt := &Statement{
		Update: &UpdateStmt{
			Where: &CompareExpr{
				Left:  &QualifiedRef{Qualifier: "old", Name: "status"},
				Op:    "=",
				Right: &StringLiteral{Value: "done"},
			},
			Set: []Assignment{{Field: "title", Value: &StringLiteral{Value: "x"}}},
		},
	}
	_, err := e.Execute(stmt, tasks)
	if err == nil {
		t.Fatal("expected error from WHERE evaluation")
	}
}

func TestExecuteUpdateEvalExprError(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{
		{ID: "T1", Title: "x", Status: "ready"},
	}

	// assignment value that fails evalExpr
	stmt := &Statement{
		Update: &UpdateStmt{
			Where: &CompareExpr{
				Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "T1"},
			},
			Set: []Assignment{{Field: "title", Value: &QualifiedRef{Qualifier: "old", Name: "title"}}},
		},
	}
	_, err := e.Execute(stmt, tasks)
	if err == nil {
		t.Fatal("expected error from evalExpr in assignment")
	}
}

// --- setField type mismatch branches ---

func TestSetFieldTypeMismatches(t *testing.T) {
	e := newTestExecutor()
	tk := &task.Task{ID: "T1", Title: "x", Status: "ready", Type: "bug", Priority: 2}

	tests := []struct {
		name  string
		field string
		val   interface{}
	}{
		{"title non-string", "title", 42},
		{"description non-string", "description", 42},
		{"status non-string", "status", 42},
		{"type non-string", "type", 42},
		{"priority non-int", "priority", "abc"},
		{"points non-int", "points", "abc"},
		{"due non-time", "due", "abc"},
		{"recurrence non-string", "recurrence", 42},
		{"assignee non-string", "assignee", 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := e.setField(tk, tt.field, tt.val)
			if err == nil {
				t.Fatalf("expected error for %s with %T", tt.field, tt.val)
			}
		})
	}
}

func TestSetFieldStatusAsTaskStatus(t *testing.T) {
	e := newTestExecutor()
	tk := &task.Task{ID: "T1", Title: "x", Status: "ready"}

	// passing task.Status value directly (not string)
	err := e.setField(tk, "status", task.Status("done"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tk.Status != "done" {
		t.Errorf("expected status done, got %q", tk.Status)
	}
}

func TestSetFieldTypeAsTaskType(t *testing.T) {
	e := newTestExecutor()
	tk := &task.Task{ID: "T1", Title: "x", Status: "ready", Type: "bug"}

	// passing task.Type value directly (not string)
	err := e.setField(tk, "type", task.Type("story"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tk.Type != "story" {
		t.Errorf("expected type story, got %q", tk.Type)
	}
}

func TestSetFieldUnknownStatusError(t *testing.T) {
	e := newTestExecutor()
	tk := &task.Task{ID: "T1", Title: "x", Status: "ready"}

	err := e.setField(tk, "status", "nonexistent_status")
	if err == nil || !strings.Contains(err.Error(), "unknown status") {
		t.Fatalf("expected unknown status error, got: %v", err)
	}
}

func TestSetFieldUnknownTypeError(t *testing.T) {
	e := newTestExecutor()
	tk := &task.Task{ID: "T1", Title: "x", Status: "ready", Type: "bug"}

	err := e.setField(tk, "type", "nonexistent_type")
	if err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("expected unknown type error, got: %v", err)
	}
}

func TestSetFieldDescriptionToEmpty(t *testing.T) {
	e := newTestExecutor()
	tk := &task.Task{ID: "T1", Title: "x", Status: "ready", Description: "some desc"}

	err := e.setField(tk, "description", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tk.Description != "" {
		t.Errorf("expected empty description, got %q", tk.Description)
	}
}

func TestSetFieldPointsToEmpty(t *testing.T) {
	e := newTestExecutor()
	tk := &task.Task{ID: "T1", Title: "x", Status: "ready", Points: 5}

	err := e.setField(tk, "points", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tk.Points != 0 {
		t.Errorf("expected 0 points, got %d", tk.Points)
	}
}

func TestSetFieldRecurrenceFromString(t *testing.T) {
	e := newTestExecutor()
	tk := &task.Task{ID: "T1", Title: "x", Status: "ready"}

	err := e.setField(tk, "recurrence", "0 0 * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tk.Recurrence != task.Recurrence("0 0 * * *") {
		t.Errorf("expected recurrence '0 0 * * *', got %q", tk.Recurrence)
	}
}

func TestSetFieldRecurrenceFromTaskRecurrence(t *testing.T) {
	e := newTestExecutor()
	tk := &task.Task{ID: "T1", Title: "x", Status: "ready"}

	err := e.setField(tk, "recurrence", task.RecurrenceDaily)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tk.Recurrence != task.RecurrenceDaily {
		t.Errorf("expected daily recurrence, got %q", tk.Recurrence)
	}
}

func TestToStringSliceNonList(t *testing.T) {
	result := toStringSlice("not a list")
	if result != nil {
		t.Errorf("expected nil for non-list input, got %v", result)
	}
}

func TestExecuteUpdatePriorityOutOfRange(t *testing.T) {
	e := newTestExecutor()

	tests := []struct {
		name string
		val  int
	}{
		{"too low", 0},
		{"too high", 99},
		{"negative", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready", Priority: 2}}
			stmt := &Statement{
				Update: &UpdateStmt{
					Where: &CompareExpr{
						Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "T1"},
					},
					Set: []Assignment{{Field: "priority", Value: &IntLiteral{Value: tt.val}}},
				},
			}
			_, err := e.Execute(stmt, tasks)
			if err == nil {
				t.Fatal("expected error for out-of-range priority")
			}
			if !strings.Contains(err.Error(), "priority must be between") {
				t.Errorf("expected range error, got: %v", err)
			}
		})
	}
}

func TestExecuteUpdatePriorityValidRange(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	for _, prio := range []int{1, 3, 5} {
		tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready", Priority: 2}}
		stmt, err := p.ParseStatement(fmt.Sprintf(`update where id = "T1" set priority=%d`, prio))
		if err != nil {
			t.Fatalf("parse priority=%d: %v", prio, err)
		}
		result, err := e.Execute(stmt, tasks)
		if err != nil {
			t.Fatalf("execute priority=%d: %v", prio, err)
		}
		if result.Update.Updated[0].Priority != prio {
			t.Errorf("expected priority %d, got %d", prio, result.Update.Updated[0].Priority)
		}
	}
}

func TestExecuteUpdatePointsOutOfRange(t *testing.T) {
	e := newTestExecutor()

	tests := []struct {
		name string
		val  int
	}{
		{"negative", -1},
		{"exceeds max", 999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready", Points: 3}}
			stmt := &Statement{
				Update: &UpdateStmt{
					Where: &CompareExpr{
						Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "T1"},
					},
					Set: []Assignment{{Field: "points", Value: &IntLiteral{Value: tt.val}}},
				},
			}
			_, err := e.Execute(stmt, tasks)
			if err == nil {
				t.Fatal("expected error for invalid points value")
			}
			if !strings.Contains(err.Error(), "invalid points") {
				t.Errorf("expected 'invalid points' error, got: %v", err)
			}
		})
	}
}

func TestExecuteUpdatePointsValidValues(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	for _, pts := range []int{0, 1, 5, 10} {
		tasks := []*task.Task{{ID: "T1", Title: "x", Status: "ready", Points: 3}}
		stmt, err := p.ParseStatement(fmt.Sprintf(`update where id = "T1" set points=%d`, pts))
		if err != nil {
			t.Fatalf("parse points=%d: %v", pts, err)
		}
		result, err := e.Execute(stmt, tasks)
		if err != nil {
			t.Fatalf("execute points=%d: %v", pts, err)
		}
		if result.Update.Updated[0].Points != pts {
			t.Errorf("expected points %d, got %d", pts, result.Update.Updated[0].Points)
		}
	}
}

func TestExecuteUpdateTitleWhitespaceRejected(t *testing.T) {
	e := newTestExecutor()
	tasks := []*task.Task{{ID: "T1", Title: "old", Status: "ready"}}

	stmt := &Statement{
		Update: &UpdateStmt{
			Where: &CompareExpr{
				Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "T1"},
			},
			Set: []Assignment{{Field: "title", Value: &StringLiteral{Value: "   "}}},
		},
	}
	_, err := e.Execute(stmt, tasks)
	if err == nil {
		t.Fatal("expected error for whitespace-only title")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty error, got: %v", err)
	}
}

func TestSetFieldDescriptionString(t *testing.T) {
	e := newTestExecutor()
	tk := &task.Task{ID: "T1", Title: "x", Status: "ready"}

	err := e.setField(tk, "description", "new desc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tk.Description != "new desc" {
		t.Errorf("expected 'new desc', got %q", tk.Description)
	}
}

func TestSetFieldPointsInt(t *testing.T) {
	e := newTestExecutor()
	tk := &task.Task{ID: "T1", Title: "x", Status: "ready"}

	err := e.setField(tk, "points", 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tk.Points != 8 {
		t.Errorf("expected 8, got %d", tk.Points)
	}
}

// --- bool comparison coverage ---

func TestCompareBoolsEqual(t *testing.T) {
	ok, err := compareBools(true, true, "=")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("true = true should be true")
	}

	ok, err = compareBools(true, false, "=")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("true = false should be false")
	}
}

func TestCompareBoolsNotEqual(t *testing.T) {
	ok, err := compareBools(true, false, "!=")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("true != false should be true")
	}

	ok, err = compareBools(true, true, "!=")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("true != true should be false")
	}
}

func TestCompareBoolsUnsupportedOp(t *testing.T) {
	_, err := compareBools(true, false, "<")
	if err == nil {
		t.Fatal("expected error for unsupported bool operator")
	}
	if !strings.Contains(err.Error(), "not supported for bool") {
		t.Fatalf("expected 'not supported for bool' error, got: %v", err)
	}
}

func TestCompareValues_BoolDispatch(t *testing.T) {
	e := newTestExecutor()

	// both sides are bool — should dispatch to compareBools
	ok, err := e.compareValues(true, false, "!=", &IntLiteral{Value: 1}, &IntLiteral{Value: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("true != false should be true via compareValues")
	}
}
