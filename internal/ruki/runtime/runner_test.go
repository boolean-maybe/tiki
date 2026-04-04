package runtime

import (
	"bytes"
	"fmt"
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
		t.Fatal("expected error for unsupported statement")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error should mention not supported: %v", err)
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

// --- UPDATE via runner ---

func TestRunQueryUpdatePersists(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(s, `update where id = "TIKI-AAA001" set title="Updated API"`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := s.GetTask("TIKI-AAA001")
	if updated == nil {
		t.Fatal("task not found after update")
	}
	if updated.Title != "Updated API" {
		t.Errorf("expected title 'Updated API', got %q", updated.Title)
	}
}

func TestRunQueryUpdateSummarySuccess(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(s, `update where status = "ready" set priority=5`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "updated 1 tasks") {
		t.Errorf("expected 'updated 1 tasks' in output, got: %s", out)
	}
}

func TestRunQueryUpdateZeroMatches(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(s, `update where id = "NONEXISTENT" set title="x"`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "updated 0 tasks") {
		t.Errorf("expected 'updated 0 tasks' in output, got: %s", out)
	}
}

func TestRunQueryUpdateListArithmeticE2E(t *testing.T) {
	s := setupRunnerTest(t)
	// set up a task with tags
	_ = s.CreateTask(&task.Task{ID: "TIKI-TAG001", Title: "Tagged", Status: "ready", Tags: []string{"old"}})

	var buf bytes.Buffer
	err := RunQuery(s, `update where id = "TIKI-TAG001" set tags=tags+"new"`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := s.GetTask("TIKI-TAG001")
	if len(updated.Tags) != 2 || updated.Tags[0] != "old" || updated.Tags[1] != "new" {
		t.Errorf("expected tags [old new], got %v", updated.Tags)
	}
}

func TestRunQueryUpdatePartialFailure(t *testing.T) {
	s := setupRunnerTest(t)

	// create a second ready task so we update multiple
	_ = s.CreateTask(&task.Task{ID: "TIKI-CCC003", Title: "Third", Status: "ready", Priority: 3})

	// delete the first task's file to cause UpdateTask to fail on it
	// (InMemoryStore won't fail on UpdateTask, so we test with a wrapper)
	fs := &failingUpdateStore{Store: s, failID: "TIKI-AAA001"}

	var buf bytes.Buffer
	err := RunQuery(fs, `update where status = "ready" set priority=5`, &buf)

	out := buf.String()
	if err == nil {
		t.Fatal("expected error for partial failure")
	}
	if !strings.Contains(out, "failed") {
		t.Errorf("expected 'failed' in output, got: %s", out)
	}
	if !strings.Contains(err.Error(), "partially failed") {
		t.Errorf("expected 'partially failed' in error, got: %v", err)
	}
}

// failingUpdateStore wraps a Store and fails UpdateTask for a specific task ID.
type failingUpdateStore struct {
	store.Store
	failID string
}

func (f *failingUpdateStore) UpdateTask(t *task.Task) error {
	if t.ID == f.failID {
		return fmt.Errorf("simulated update failure for %s", t.ID)
	}
	return f.Store.UpdateTask(t)
}

// failingUserStore wraps a Store and makes GetCurrentUser fail.
type failingUserStore struct {
	store.Store
}

func (f *failingUserStore) GetCurrentUser() (string, string, error) {
	return "", "", fmt.Errorf("simulated user resolution failure")
}

func TestRunQueryResolveUserError(t *testing.T) {
	s := setupRunnerTest(t)
	fs := &failingUserStore{Store: s}

	var buf bytes.Buffer
	err := RunQuery(fs, "select", &buf)
	if err == nil {
		t.Fatal("expected error for user resolution failure")
	}
	if !strings.Contains(err.Error(), "resolve current user") {
		t.Errorf("expected 'resolve current user' error, got: %v", err)
	}
}

func TestRunQueryResolveUserErrorUpdate(t *testing.T) {
	s := setupRunnerTest(t)
	fs := &failingUserStore{Store: s}

	var buf bytes.Buffer
	err := RunQuery(fs, `update where id = "TIKI-AAA001" set title="x"`, &buf)
	if err == nil {
		t.Fatal("expected error for user resolution failure on update")
	}
	if !strings.Contains(err.Error(), "resolve current user") {
		t.Errorf("expected 'resolve current user' error, got: %v", err)
	}
}

func TestRunQueryExecuteError(t *testing.T) {
	s := setupRunnerTest(t)

	// delete statement is parsed but executor returns "not supported" error
	var buf bytes.Buffer
	err := RunQuery(s, `delete where id = "X"`, &buf)
	if err == nil {
		t.Fatal("expected error for unsupported delete")
	}
	if !strings.Contains(err.Error(), "execute") {
		t.Errorf("expected execute error, got: %v", err)
	}
}

func TestRunQueryUpdateInvalidPointsE2E(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(s, `update where id = "TIKI-AAA001" set points=999`, &buf)
	if err == nil {
		t.Fatal("expected error for invalid points")
	}
	if !strings.Contains(err.Error(), "invalid points") {
		t.Errorf("expected 'invalid points' error, got: %v", err)
	}
}
