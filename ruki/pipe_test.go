package ruki

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/task"
)

// --- lexer ---

func TestTokenizePipe(t *testing.T) {
	pipeType := rukiLexer.Symbols()["Pipe"]

	t.Run("bare pipe", func(t *testing.T) {
		tokens := tokenize(t, "|")
		if len(tokens) != 1 {
			t.Fatalf("expected 1 token, got %d: %v", len(tokens), tokens)
		}
		if tokens[0].Type != pipeType {
			t.Errorf("expected Pipe token, got type %d", tokens[0].Type)
		}
	})

	t.Run("pipe in context", func(t *testing.T) {
		tokens := tokenize(t, `select id | run("echo $1")`)
		found := false
		for _, tok := range tokens {
			if tok.Type == pipeType {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find Pipe token in tokenized output")
		}
	})
}

// --- parser ---

func TestParsePipeSelect(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"basic pipe", `select id where id = id() | run("echo $1")`},
		{"multi-field pipe", `select id, title where status = "done" | run("myscript $1 $2")`},
		{"pipe without where", `select id | run("echo $1")`},
		{"pipe with order by", `select id, priority where status = "ready" order by priority | run("echo $1 $2")`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseAndValidateStatement(tt.input, ExecutorRuntimePlugin)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if !stmt.IsSelect() {
				t.Fatal("expected IsSelect() true")
			}
			if !stmt.IsPipe() {
				t.Fatal("expected IsPipe() true")
			}
		})
	}
}

func TestParsePipeDoesNotBreakPlainSelect(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"bare select", "select"},
		{"select star", "select *"},
		{"select with where", `select where status = "done"`},
		{"select with fields", `select id, title where status = "done"`},
		{"select with order by", `select order by priority`},
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
			if stmt.Select.Pipe != nil {
				t.Fatal("expected no Pipe on plain select")
			}
		})
	}
}

// --- lowering ---

func TestLowerPipeProducesSelectAndPipe(t *testing.T) {
	p := newTestParser()
	stmt, err := p.ParseStatement(`select id where id = id() | run("echo $1")`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if stmt.Select == nil {
		t.Fatal("expected Select non-nil")
		return
	}
	if stmt.Select.Pipe == nil {
		t.Fatal("expected Select.Pipe non-nil")
		return
	}
	if stmt.Select.Pipe.Run == nil {
		t.Fatal("expected Select.Pipe.Run non-nil")
		return
	}
	if stmt.Select.Pipe.Run.Command == nil {
		t.Fatal("expected Select.Pipe.Run.Command non-nil")
	}
}

// --- validation ---

func TestPipeRejectSelectStar(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseStatement(`select * | run("echo")`)
	if err == nil {
		t.Fatal("expected error for select * with pipe")
	}
	if !strings.Contains(err.Error(), "explicit field names") {
		t.Errorf("expected 'explicit field names' error, got: %v", err)
	}
}

func TestPipeRejectBareSelect(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseStatement(`select | run("echo")`)
	if err == nil {
		t.Fatal("expected error for bare select with pipe")
	}
	if !strings.Contains(err.Error(), "explicit field names") {
		t.Errorf("expected 'explicit field names' error, got: %v", err)
	}
}

func TestPipeRejectNonStringCommand(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseStatement(`select id | run(42)`)
	if err == nil {
		t.Fatal("expected error for non-string pipe command")
	}
	if !strings.Contains(err.Error(), "pipe command must be string") {
		t.Errorf("expected 'pipe command must be string' error, got: %v", err)
	}
}

func TestPipeRejectFieldRefInCommand(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"bare field ref", `select id | run(title)`},
		{"qualified ref", `select id | run(old.title)`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseStatement(tt.input)
			if err == nil {
				t.Fatal("expected error for field ref in pipe command")
			}
			// either the field-ref check catches it, or type inference
			// rejects it earlier (e.g. "old. qualifier not valid")
		})
	}
}

func TestExprContainsFieldRef(t *testing.T) {
	tests := []struct {
		name     string
		expr     Expr
		expected bool
	}{
		{"string literal", &StringLiteral{Value: "echo"}, false},
		{"int literal", &IntLiteral{Value: 42}, false},
		{"bare field ref", &FieldRef{Name: "title"}, true},
		{"qualified ref", &QualifiedRef{Qualifier: "old", Name: "title"}, true},
		{"field in binary expr", &BinaryExpr{
			Op:    "+",
			Left:  &StringLiteral{Value: "echo "},
			Right: &FieldRef{Name: "title"},
		}, true},
		{"field in function arg", &FunctionCall{
			Name: "concat",
			Args: []Expr{&FieldRef{Name: "title"}},
		}, true},
		{"clean function", &FunctionCall{
			Name: "id",
			Args: nil,
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exprContainsFieldRef(tt.expr)
			if got != tt.expected {
				t.Errorf("exprContainsFieldRef() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// --- semantic validation ---

func TestIsPipeMethod(t *testing.T) {
	p := newTestParser()

	pipeStmt, err := p.ParseAndValidateStatement(`select id where id = id() | run("echo $1")`, ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !pipeStmt.IsPipe() {
		t.Error("expected IsPipe() = true for pipe statement")
	}

	plainStmt, err := p.ParseAndValidateStatement(`select where status = "done"`, ExecutorRuntimeCLI)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if plainStmt.IsPipe() {
		t.Error("expected IsPipe() = false for plain select")
	}

	updateStmt, err := p.ParseAndValidateStatement(`update where id = id() set status = "done"`, ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if updateStmt.IsPipe() {
		t.Error("expected IsPipe() = false for update statement")
	}
}

func TestPipeIDDetectedInWhereClause(t *testing.T) {
	p := newTestParser()
	stmt, err := p.ParseAndValidateStatement(`select id where id = id() | run("echo $1")`, ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !stmt.UsesIDBuiltin() {
		t.Error("expected UsesIDBuiltin() = true for pipe with id() in where")
	}
}

// --- executor ---

func TestExecutePipeReturnsResult(t *testing.T) {
	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`select id, title where status = "done" | run("echo $1 $2")`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := e.Execute(stmt, tasks, ExecutionInput{SelectedTaskID: "TIKI-000003"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Pipe == nil {
		t.Fatal("expected Pipe result")
		return
	}
	if result.Select != nil {
		t.Fatal("expected Select to be nil when pipe is present")
	}
	if result.Pipe.Command != "echo $1 $2" {
		t.Errorf("command = %q, want %q", result.Pipe.Command, "echo $1 $2")
	}
	if len(result.Pipe.Rows) != 1 {
		t.Fatalf("expected 1 row (status=done), got %d", len(result.Pipe.Rows))
	}
	row := result.Pipe.Rows[0]
	if len(row) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(row))
	}
	if row[0] != "TIKI-000003" {
		t.Errorf("row[0] (id) = %q, want %q", row[0], "TIKI-000003")
	}
	if row[1] != "Write docs" {
		t.Errorf("row[1] (title) = %q, want %q", row[1], "Write docs")
	}
}

func TestExecuteSelectStillWorksWithoutPipe(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`select where status = "done"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Select == nil {
		t.Fatal("expected Select result for plain select")
		return
	}
	if result.Pipe != nil {
		t.Fatal("expected no Pipe result for plain select")
	}
}

func TestExecutePipeListFieldSpaceJoined(t *testing.T) {
	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	p := newTestParser()
	tasks := []*task.Task{
		{ID: "TIKI-000001", Title: "Test", Status: "ready", Type: "story",
			Priority: 1, Tags: []string{"a", "b", "c"}},
	}

	stmt, err := p.ParseStatement(`select id, tags where id = id() | run("echo $1 $2")`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := e.Execute(stmt, tasks, ExecutionInput{SelectedTaskID: "TIKI-000001"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Pipe == nil {
		t.Fatal("expected Pipe result")
	}
	if len(result.Pipe.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Pipe.Rows))
	}
	row := result.Pipe.Rows[0]
	if row[1] != "a b c" {
		t.Errorf("tags field = %q, want %q", row[1], "a b c")
	}
}

// --- clipboard pipe: parser ---

func TestParseClipboardPipe(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{"basic clipboard", `select id where id = id() | clipboard()`},
		{"multi-field clipboard", `select id, title where status = "done" | clipboard()`},
		{"clipboard without where", `select id | clipboard()`},
		{"clipboard with order by", `select id, priority order by priority | clipboard()`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := p.ParseAndValidateStatement(tt.input, ExecutorRuntimePlugin)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if !stmt.IsPipe() {
				t.Fatal("expected IsPipe() true")
			}
			if !stmt.IsClipboardPipe() {
				t.Fatal("expected IsClipboardPipe() true")
			}
		})
	}
}

// --- clipboard pipe: lowering ---

func TestLowerClipboardPipe(t *testing.T) {
	p := newTestParser()
	stmt, err := p.ParseStatement(`select id | clipboard()`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if stmt.Select.Pipe == nil {
		t.Fatal("expected Select.Pipe non-nil")
		return
	}
	if stmt.Select.Pipe.Clipboard == nil {
		t.Fatal("expected Select.Pipe.Clipboard non-nil")
		return
	}
	if stmt.Select.Pipe.Run != nil {
		t.Fatal("expected Select.Pipe.Run nil for clipboard pipe")
	}
}

// --- clipboard pipe: validation ---

func TestClipboardPipeRejectSelectStar(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseStatement(`select * | clipboard()`)
	if err == nil {
		t.Fatal("expected error for select * with clipboard pipe")
	}
	if !strings.Contains(err.Error(), "explicit field names") {
		t.Errorf("expected 'explicit field names' error, got: %v", err)
	}
}

func TestClipboardPipeRejectBareSelect(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseStatement(`select | clipboard()`)
	if err == nil {
		t.Fatal("expected error for bare select with clipboard pipe")
	}
	if !strings.Contains(err.Error(), "explicit field names") {
		t.Errorf("expected 'explicit field names' error, got: %v", err)
	}
}

// --- clipboard pipe: semantic ---

func TestIsClipboardPipeMethod(t *testing.T) {
	p := newTestParser()

	clipStmt, err := p.ParseAndValidateStatement(`select id where id = id() | clipboard()`, ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !clipStmt.IsPipe() {
		t.Error("expected IsPipe() = true")
	}
	if !clipStmt.IsClipboardPipe() {
		t.Error("expected IsClipboardPipe() = true")
	}

	runStmt, err := p.ParseAndValidateStatement(`select id where id = id() | run("echo $1")`, ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !runStmt.IsPipe() {
		t.Error("expected IsPipe() = true for run pipe")
	}
	if runStmt.IsClipboardPipe() {
		t.Error("expected IsClipboardPipe() = false for run pipe")
	}

	plainStmt, err := p.ParseAndValidateStatement(`select where status = "done"`, ExecutorRuntimeCLI)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if plainStmt.IsClipboardPipe() {
		t.Error("expected IsClipboardPipe() = false for plain select")
	}
}

// --- clipboard pipe: executor ---

func TestExecuteClipboardPipeReturnsResult(t *testing.T) {
	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimePlugin})
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`select id, title where status = "done" | clipboard()`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := e.Execute(stmt, tasks, ExecutionInput{SelectedTaskID: "TIKI-000003"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Clipboard == nil {
		t.Fatal("expected Clipboard result")
		return
	}
	if result.Pipe != nil {
		t.Fatal("expected Pipe to be nil when clipboard is present")
	}
	if result.Select != nil {
		t.Fatal("expected Select to be nil when clipboard is present")
	}
	if len(result.Clipboard.Rows) != 1 {
		t.Fatalf("expected 1 row (status=done), got %d", len(result.Clipboard.Rows))
	}
	row := result.Clipboard.Rows[0]
	if len(row) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(row))
	}
	if row[0] != "TIKI-000003" {
		t.Errorf("row[0] (id) = %q, want %q", row[0], "TIKI-000003")
	}
	if row[1] != "Write docs" {
		t.Errorf("row[1] (title) = %q, want %q", row[1], "Write docs")
	}
}

func TestExecuteClipboardMultipleRows(t *testing.T) {
	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimeCLI})
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`select id, title | clipboard()`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Clipboard == nil {
		t.Fatal("expected Clipboard result")
	}
	if len(result.Clipboard.Rows) != len(tasks) {
		t.Fatalf("expected %d rows, got %d", len(tasks), len(result.Clipboard.Rows))
	}
}

// --- limit + pipe ---

func TestExecuteClipboardWithLimit(t *testing.T) {
	e := NewExecutor(testSchema{}, nil, ExecutorRuntime{Mode: ExecutorRuntimeCLI})
	p := newTestParser()
	tasks := makeTasks()

	stmt, err := p.ParseStatement(`select id, priority order by priority limit 2 | clipboard()`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := e.Execute(stmt, tasks)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Clipboard == nil {
		t.Fatal("expected Clipboard result")
	}
	if len(result.Clipboard.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Clipboard.Rows))
	}
	// sorted by priority asc: TIKI-000002 (pri 1), TIKI-000001 (pri 2)
	if result.Clipboard.Rows[0][0] != "TIKI-000002" {
		t.Errorf("row[0][0] = %q, want %q", result.Clipboard.Rows[0][0], "TIKI-000002")
	}
	if result.Clipboard.Rows[1][0] != "TIKI-000001" {
		t.Errorf("row[1][0] = %q, want %q", result.Clipboard.Rows[1][0], "TIKI-000001")
	}
}

// --- pipeArgString ---

func TestPipeArgString(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want string
	}{
		{"string", "hello", "hello"},
		{"int", 3, "3"},
		{"string slice", []interface{}{"a", "b"}, "a b"},
		{"empty slice", []interface{}{}, ""},
		{"nil", nil, "<nil>"},
		{"status", task.Status("done"), "done"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pipeArgString(tt.val)
			if got != tt.want {
				t.Errorf("pipeArgString(%v) = %q, want %q", tt.val, got, tt.want)
			}
		})
	}
}
