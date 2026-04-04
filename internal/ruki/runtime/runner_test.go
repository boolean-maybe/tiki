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

func TestRunSelectQueryRejectsNonSelect(t *testing.T) {
	s := setupRunnerTest(t)

	tests := []struct {
		name  string
		query string
	}{
		{"rejects create", `create title="via legacy"`},
		{"rejects update", `update where id = "TIKI-AAA001" set title="x"`},
		{"rejects delete", `delete where id = "TIKI-AAA001"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := RunSelectQuery(s, tt.query, &buf)
			if err == nil {
				t.Fatal("expected error for non-SELECT statement via RunSelectQuery")
			}
			if !strings.Contains(err.Error(), "only supports SELECT") {
				t.Errorf("expected 'only supports SELECT' error, got: %v", err)
			}
		})
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

	// call() is rejected at runtime, triggering an execute error
	var buf bytes.Buffer
	err := RunQuery(s, `select where call("echo") = "x"`, &buf)
	if err == nil {
		t.Fatal("expected execute error")
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

// --- CREATE via runner ---

func TestRunQueryCreatePersists(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(s, `create title="New Task" status="ready" priority=1`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "created TIKI-") {
		t.Fatalf("expected 'created TIKI-' in output, got: %s", out)
	}

	// verify task exists in store
	allTasks := s.GetAllTasks()
	var found *task.Task
	for _, tk := range allTasks {
		if tk.Title == "New Task" {
			found = tk
			break
		}
	}
	if found == nil {
		t.Fatal("created task not found in store")
	}
	if !strings.HasPrefix(found.ID, "TIKI-") || len(found.ID) != 11 {
		t.Errorf("ID = %q, want TIKI-XXXXXX format (11 chars)", found.ID)
	}
	if found.Priority != 1 {
		t.Errorf("priority = %d, want 1", found.Priority)
	}
}

func TestRunQueryCreateMissingTitle(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(s, `create priority=1 status="ready"`, &buf)
	if err == nil {
		t.Fatal("expected error for missing title")
	}
	if !strings.Contains(err.Error(), "title") {
		t.Errorf("expected title error, got: %v", err)
	}
}

func TestRunQueryCreateTemplateDefaults(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(s, `create title="Templated" tags=tags+["extra"]`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allTasks := s.GetAllTasks()
	var found *task.Task
	for _, tk := range allTasks {
		if tk.Title == "Templated" {
			found = tk
			break
		}
	}
	if found == nil {
		t.Fatal("created task not found in store")
	}
	// InMemoryStore template has tags=["idea"], so result should be ["idea", "extra"]
	if len(found.Tags) != 2 || found.Tags[0] != "idea" || found.Tags[1] != "extra" {
		t.Errorf("tags = %v, want [idea extra]", found.Tags)
	}
	// priority should be template default (7)
	if found.Priority != 7 {
		t.Errorf("priority = %d, want 7 (template default)", found.Priority)
	}
}

func TestRunQueryCreateTemplateFailure(t *testing.T) {
	s := setupRunnerTest(t)
	fs := &failingTemplateStore{Store: s}

	var buf bytes.Buffer
	err := RunQuery(fs, `create title="test"`, &buf)
	if err == nil {
		t.Fatal("expected error for template failure")
	}
	if !strings.Contains(err.Error(), "create template") {
		t.Errorf("expected 'create template' error, got: %v", err)
	}
}

// failingTemplateStore wraps a Store and fails NewTaskTemplate.
type failingTemplateStore struct {
	store.Store
}

func (f *failingTemplateStore) NewTaskTemplate() (*task.Task, error) {
	return nil, fmt.Errorf("simulated template failure")
}

func TestRunQueryCreateNilTemplate(t *testing.T) {
	s := setupRunnerTest(t)
	fs := &nilTemplateStore{Store: s}

	var buf bytes.Buffer
	err := RunQuery(fs, `create title="test"`, &buf)
	if err == nil {
		t.Fatal("expected error for nil template")
	}
	if !strings.Contains(err.Error(), "nil template") {
		t.Errorf("expected 'nil template' error, got: %v", err)
	}
}

// nilTemplateStore wraps a Store and returns (nil, nil) from NewTaskTemplate.
type nilTemplateStore struct {
	store.Store
}

func (f *nilTemplateStore) NewTaskTemplate() (*task.Task, error) {
	return nil, nil
}

func TestRunQueryCreateTaskFailure(t *testing.T) {
	s := setupRunnerTest(t)
	fs := &failingCreateStore{Store: s}

	var buf bytes.Buffer
	err := RunQuery(fs, `create title="test"`, &buf)
	if err == nil {
		t.Fatal("expected error for CreateTask failure")
	}
	if !strings.Contains(err.Error(), "create task") {
		t.Errorf("expected 'create task' error, got: %v", err)
	}
}

// failingCreateStore wraps a Store and fails CreateTask.
type failingCreateStore struct {
	store.Store
}

func (f *failingCreateStore) CreateTask(t *task.Task) error {
	return fmt.Errorf("simulated create failure")
}

// --- DELETE via runner ---

func TestRunQueryDeletePersists(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(s, `delete where id = "TIKI-AAA001"`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "deleted 1 tasks") {
		t.Errorf("expected 'deleted 1 tasks' in output, got: %s", out)
	}
	if s.GetTask("TIKI-AAA001") != nil {
		t.Error("task should be deleted from store")
	}
}

func TestRunQueryDeleteZeroMatches(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(s, `delete where id = "NONEXISTENT"`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "deleted 0 tasks") {
		t.Errorf("expected 'deleted 0 tasks' in output, got: %s", out)
	}
}

func TestRunQueryDeletePartialFailure(t *testing.T) {
	s := setupRunnerTest(t)
	// add a second ready task so we match multiple
	_ = s.CreateTask(&task.Task{ID: "TIKI-CCC003", Title: "Third", Status: "ready", Priority: 3})

	fs := &failingDeleteStore{Store: s, failID: "TIKI-AAA001"}

	var buf bytes.Buffer
	err := RunQuery(fs, `delete where status = "ready"`, &buf)

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

// failingDeleteStore wraps a Store and silently no-ops DeleteTask for a specific ID.
type failingDeleteStore struct {
	store.Store
	failID string
}

func (f *failingDeleteStore) DeleteTask(id string) {
	if id == f.failID {
		return // simulate silent failure
	}
	f.Store.DeleteTask(id)
}
