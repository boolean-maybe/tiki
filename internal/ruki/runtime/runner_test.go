package runtime

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

func setupRunnerTest(t *testing.T) store.Store {
	t.Helper()
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "in_progress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
	})

	s := store.NewInMemoryStore()
	_ = s.CreateTask(&task.Task{ID: "TIKI-AAA001", Title: "Build API", Status: "ready", Priority: 1})
	_ = s.CreateTask(&task.Task{ID: "TIKI-BBB002", Title: "Write Docs", Status: "done", Priority: 2})
	return s
}

func TestRunSelectQuerySuccess(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunSelectQuery(s, `select id, title where status = "ready"`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "TIKI-AAA001") {
		t.Errorf("expected TIKI-AAA001 in output:\n%s", out)
	}
	if strings.Contains(out, "TIKI-BBB002") {
		t.Errorf("TIKI-BBB002 should be filtered out:\n%s", out)
	}
}

func TestRunSelectQueryBareSelect(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunSelectQuery(s, "select", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// bare select returns all tasks with all fields
	if !strings.Contains(out, "TIKI-AAA001") || !strings.Contains(out, "TIKI-BBB002") {
		t.Errorf("bare select should return all tasks:\n%s", out)
	}
}

func TestRunSelectQuerySemicolon(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunSelectQuery(s, "select id, title;", &buf)
	if err != nil {
		t.Fatalf("trailing semicolon should be accepted: %v", err)
	}

	if !strings.Contains(buf.String(), "TIKI-AAA001") {
		t.Errorf("semicolon query should produce results:\n%s", buf.String())
	}
}

func TestRunSelectQueryParseError(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunSelectQuery(s, "select from where", &buf)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("error should mention parse: %v", err)
	}
}

func TestRunSelectQueryNonSelectRejected(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunSelectQuery(s, `create title="test"`, &buf)
	if err == nil {
		t.Fatal("expected error for non-select")
	}
	if !strings.Contains(err.Error(), "only select") {
		t.Errorf("error should mention only select: %v", err)
	}
}

func TestRunSelectQueryEmptyQuery(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunSelectQuery(s, "", &buf)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty: %v", err)
	}
}

func TestRunSelectQuerySemicolonOnly(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunSelectQuery(s, ";", &buf)
	if err == nil {
		t.Fatal("expected error for semicolon-only query")
	}
}

func TestRunSelectQueryUserFunction(t *testing.T) {
	s := setupRunnerTest(t)
	// InMemoryStore returns "memory-user"
	_ = s.CreateTask(&task.Task{ID: "TIKI-CCC003", Title: "My Task", Status: "ready", Assignee: "memory-user"})

	var buf bytes.Buffer
	err := RunSelectQuery(s, `select id where assignee = user()`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "TIKI-CCC003") {
		t.Errorf("user() should resolve to memory-user:\n%s", out)
	}
}

func TestRunSelectQueryWhitespaceOnly(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunSelectQuery(s, "   ", &buf)
	if err == nil {
		t.Fatal("expected error for whitespace-only query")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty: %v", err)
	}
}

func TestRunSelectQueryWithOrderBy(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunSelectQuery(s, `select id, title order by priority`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "TIKI-AAA001") || !strings.Contains(out, "TIKI-BBB002") {
		t.Errorf("order by query should return all tasks:\n%s", out)
	}
}
