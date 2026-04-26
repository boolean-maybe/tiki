package runtime

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

func setupRunnerTest(t *testing.T) store.Store {
	t.Helper()
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "inProgress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
	})

	s := store.NewInMemoryStore()
	_ = s.CreateTask(&task.Task{ID: "TIKI-AAA001", Title: "Build API", Status: "ready", Priority: 1})
	_ = s.CreateTask(&task.Task{ID: "TIKI-BBB002", Title: "Write Docs", Status: "done", Priority: 2})
	return s
}

// gateFor wraps a store in a bare gate (no field validators) for tests.
func gateFor(s store.Store) *service.TaskMutationGate {
	g := service.NewTaskMutationGate()
	g.SetStore(s)
	return g
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
	err := RunQuery(gateFor(s), `update where id = "TIKI-AAA001" set title="Updated API"`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := s.GetTask("TIKI-AAA001")
	if updated == nil {
		t.Fatal("task not found after update")
		return
	}
	if updated.Title != "Updated API" {
		t.Errorf("expected title 'Updated API', got %q", updated.Title)
	}
}

func TestRunQueryUpdateSummarySuccess(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(gateFor(s), `update where status = "ready" set priority=5`, &buf)
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
	err := RunQuery(gateFor(s), `update where id = "NONEXISTENT" set title="x"`, &buf)
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
	err := RunQuery(gateFor(s), `update where id = "TIKI-TAG001" set tags=tags+"new"`, &buf)
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
	err := RunQuery(gateFor(fs), `update where status = "ready" set priority=5`, &buf)

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
	err := RunQuery(gateFor(fs), "select", &buf)
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
	err := RunQuery(gateFor(fs), `update where id = "TIKI-AAA001" set title="x"`, &buf)
	if err == nil {
		t.Fatal("expected error for user resolution failure on update")
	}
	if !strings.Contains(err.Error(), "resolve current user") {
		t.Errorf("expected 'resolve current user' error, got: %v", err)
	}
}

func TestRunQueryExecuteError(t *testing.T) {
	s := setupRunnerTest(t)

	// call() is rejected during semantic validation in RunQuery.
	var buf bytes.Buffer
	err := RunQuery(gateFor(s), `select where call("echo") = "x"`, &buf)
	if err == nil {
		t.Fatal("expected semantic validation error")
	}
	if !strings.Contains(err.Error(), "call() is not supported yet") {
		t.Errorf("expected call() semantic validation error, got: %v", err)
	}
}

func TestRunQueryUpdateInvalidPointsE2E(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(gateFor(s), `update where id = "TIKI-AAA001" set points=999`, &buf)
	if err == nil {
		t.Fatal("expected error for invalid points")
	}
	if !strings.Contains(err.Error(), "points value out of range") {
		t.Errorf("expected points range error, got: %v", err)
	}
}

// --- CREATE via runner ---

func TestRunQueryCreatePersists(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(gateFor(s), `create title="New Task" status="ready" priority=1`, &buf)
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
		return
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

	// use BuildGate (with field validators) to catch empty title
	g := service.BuildGate()
	g.SetStore(s)

	var buf bytes.Buffer
	err := RunQuery(g, `create priority=1 status="ready"`, &buf)
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
	err := RunQuery(gateFor(s), `create title="Templated" tags=tags+["extra"]`, &buf)
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
		return
	}
	// InMemoryStore template has tags=["idea"], so result should be ["idea", "extra"]
	if len(found.Tags) != 2 || found.Tags[0] != "idea" || found.Tags[1] != "extra" {
		t.Errorf("tags = %v, want [idea extra]", found.Tags)
	}
	// priority should be template default (3 = medium)
	if found.Priority != 3 {
		t.Errorf("priority = %d, want 3 (template default)", found.Priority)
	}
}

func TestRunQueryCreateTemplateFailure(t *testing.T) {
	s := setupRunnerTest(t)
	fs := &failingTemplateStore{Store: s}

	var buf bytes.Buffer
	err := RunQuery(gateFor(fs), `create title="test"`, &buf)
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
	err := RunQuery(gateFor(fs), `create title="test"`, &buf)
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
	err := RunQuery(gateFor(fs), `create title="test"`, &buf)
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
	err := RunQuery(gateFor(s), `delete where id = "TIKI-AAA001"`, &buf)
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
	err := RunQuery(gateFor(s), `delete where id = "NONEXISTENT"`, &buf)
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
	err := RunQuery(gateFor(fs), `delete where status = "ready"`, &buf)

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

func TestRunQueryEmptyQuery(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(gateFor(s), "", &buf)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' error, got: %v", err)
	}
}

func TestRunQueryParseError(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(gateFor(s), "select from where", &buf)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected 'parse' error, got: %v", err)
	}
}

func TestRunSelectQueryResolveUserError(t *testing.T) {
	s := setupRunnerTest(t)
	fs := &failingUserStore{Store: s}

	var buf bytes.Buffer
	err := RunSelectQuery(fs, "select", &buf)
	if err == nil {
		t.Fatal("expected error for user resolution failure")
	}
	if !strings.Contains(err.Error(), "resolve current user") {
		t.Errorf("expected 'resolve current user' error, got: %v", err)
	}
}

// --- UPDATE via RunQuery (covers the result.Update branch in RunQuery) ---

func TestRunQuerySelectViaRunQuery(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(gateFor(s), `select id where status = "ready"`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "TIKI-AAA001") {
		t.Errorf("expected TIKI-AAA001 in output:\n%s", buf.String())
	}
}

// --- DELETE partial failure via silent DeleteTask no-op detection ---

func TestRunQueryDeleteSilentFailure(t *testing.T) {
	s := setupRunnerTest(t)
	// failingDeleteStore silently ignores delete for failID
	fs := &failingDeleteStore{Store: s, failID: "TIKI-AAA001"}

	var buf bytes.Buffer
	err := RunQuery(gateFor(fs), `delete where id = "TIKI-AAA001"`, &buf)
	if err == nil {
		t.Fatal("expected error for silent delete failure")
	}
	if !strings.Contains(err.Error(), "partially failed") {
		t.Errorf("expected 'partially failed' error, got: %v", err)
	}
}

func TestRunSelectQueryExecuteError(t *testing.T) {
	s := setupRunnerTest(t)

	// call() is rejected during semantic validation in RunSelectQuery.
	var buf bytes.Buffer
	err := RunSelectQuery(s, `select where call("echo") = "x"`, &buf)
	if err == nil {
		t.Fatal("expected semantic validation error")
	}
	if !strings.Contains(err.Error(), "call() is not supported yet") {
		t.Errorf("expected call() semantic validation error, got: %v", err)
	}
}

// failingDeleteTaskStore wraps a Store and makes DeleteTask error via the gate.
type failingDeleteTaskStore struct {
	store.Store
	failID string
}

func (f *failingDeleteTaskStore) DeleteTask(id string) {
	if id != f.failID {
		f.Store.DeleteTask(id)
	}
	// for failID: silently no-op
}

func TestRunQueryDeleteGateError(t *testing.T) {
	s := setupRunnerTest(t)

	// use a gate with a validator that rejects the delete
	g := service.NewTaskMutationGate()
	fds := &failingDeleteTaskStore{Store: s, failID: "TIKI-AAA001"}
	g.SetStore(fds)

	var buf bytes.Buffer
	err := RunQuery(g, `delete where id = "TIKI-AAA001"`, &buf)
	// the store silently fails to delete, so persistDelete detects task still exists
	if err == nil {
		t.Fatal("expected error for delete gate failure")
	}
}

func TestRunQueryUserFunction(t *testing.T) {
	s := setupRunnerTest(t)

	// select where assignee = user() — exercises the user() closure (line 32)
	var buf bytes.Buffer
	err := RunSelectQuery(s, `select where assignee = user()`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRunQueryUserFunctionViaRunQuery exercises the user() closure inside RunQuery
// (not RunSelectQuery). The closure at line 32 captures the resolved user name.
func TestRunQueryUserFunctionViaRunQuery(t *testing.T) {
	s := setupRunnerTest(t)
	// InMemoryStore.GetCurrentUser returns "memory-user"
	_ = s.CreateTask(&task.Task{ID: "TIKI-CCC003", Title: "Owned", Status: "ready", Assignee: "memory-user"})

	var buf bytes.Buffer
	err := RunQuery(gateFor(s), `select id where assignee = user()`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "TIKI-CCC003") {
		t.Errorf("expected TIKI-CCC003 in output:\n%s", out)
	}
	// tasks without the matching assignee should be filtered out
	if strings.Contains(out, "TIKI-AAA001") {
		t.Errorf("TIKI-AAA001 should be filtered out:\n%s", out)
	}
}

// --- top-level expression statements via RunQuery ---

func TestRunQueryScalarCount(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(gateFor(s), `count(select where status = "ready")`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// must emit the scalar count with a trailing newline and no table frame
	if buf.String() != "1\n" {
		t.Errorf("expected \"1\\n\", got %q", buf.String())
	}
}

func TestRunQueryScalarExists(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(gateFor(s), `exists(select where status = "done")`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "true\n" {
		t.Errorf("expected \"true\\n\", got %q", buf.String())
	}
}

func TestRunQueryScalarExistsFalse(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(gateFor(s), `exists(select where priority = 99)`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "false\n" {
		t.Errorf("expected \"false\\n\", got %q", buf.String())
	}
}

func TestRunQueryScalarArithmetic(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(gateFor(s), `1 + 2`, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "3\n" {
		t.Errorf("expected \"3\\n\", got %q", buf.String())
	}
}

func TestRunQueryScalarRejectsBareFieldRef(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQuery(gateFor(s), `priority`, &buf)
	if err == nil {
		t.Fatal("expected parse error for bare field at top level")
	}
	if !strings.Contains(err.Error(), "top level") && !strings.Contains(err.Error(), "not valid at the top level") {
		t.Errorf("expected top-level rejection error, got: %v", err)
	}
}

// --- RunQueryWithOptions: JSON output ---

func TestRunQueryWithOptionsSelectJSON(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQueryWithOptions(gateFor(s), `select id, title where status = "ready"`, &buf,
		RunQueryOptions{OutputFormat: OutputJSON})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := strings.TrimSpace(buf.String())
	// should be a JSON array, not a table
	if !strings.HasPrefix(out, "[") || !strings.HasSuffix(out, "]") {
		t.Errorf("expected JSON array, got:\n%s", out)
	}
	if strings.Contains(out, "+---") || strings.Contains(out, "|") {
		t.Errorf("expected no table borders in JSON output:\n%s", out)
	}
	if !strings.Contains(out, `"TIKI-AAA001"`) || !strings.Contains(out, `"Build API"`) {
		t.Errorf("missing expected row data:\n%s", out)
	}
}

func TestRunQueryWithOptionsScalarJSON(t *testing.T) {
	s := setupRunnerTest(t)

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"count", `count(select where status = "ready")`, "1"},
		{"exists true", `exists(select where status = "done")`, "true"},
		{"exists false", `exists(select where priority = 99)`, "false"},
		{"arithmetic", `1 + 2`, "3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := RunQueryWithOptions(gateFor(s), tt.query, &buf,
				RunQueryOptions{OutputFormat: OutputJSON})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := strings.TrimSpace(buf.String()); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunQueryWithOptionsUpdateJSON(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQueryWithOptions(gateFor(s), `update where id = "TIKI-AAA001" set title="x"`, &buf,
		RunQueryOptions{OutputFormat: OutputJSON})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != `{"failed":0,"updated":1}` {
		t.Errorf("got %q", got)
	}
}

func TestRunQueryWithOptionsCreateJSON(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQueryWithOptions(gateFor(s), `create title="New" status="ready"`, &buf,
		RunQueryOptions{OutputFormat: OutputJSON})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	// shape: {"created":"TIKI-XXXXXX"}
	if !strings.HasPrefix(out, `{"created":"TIKI-`) || !strings.HasSuffix(out, `"}`) {
		t.Errorf("unexpected create JSON: %q", out)
	}
}

func TestRunQueryWithOptionsDeleteJSON(t *testing.T) {
	s := setupRunnerTest(t)

	var buf bytes.Buffer
	err := RunQueryWithOptions(gateFor(s), `delete where id = "TIKI-AAA001"`, &buf,
		RunQueryOptions{OutputFormat: OutputJSON})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != `{"deleted":1,"failed":0}` {
		t.Errorf("got %q", got)
	}
}

// a ruki statement may legitimately start with a `--` line comment. This
// locks in the full path: such a statement is accepted by the runtime,
// parsed past the leading comment, and executed normally.
func TestRunQueryWithOptionsDashLeadingComment(t *testing.T) {
	s := setupRunnerTest(t)

	stmt := "-- backlog count\ncount(select where status = \"ready\")"

	var buf bytes.Buffer
	err := RunQueryWithOptions(gateFor(s), stmt, &buf, RunQueryOptions{OutputFormat: OutputJSON})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "1" {
		t.Errorf("got %q, want 1", got)
	}
}

func TestRunQueryWithOptionsTableIsDefault(t *testing.T) {
	s := setupRunnerTest(t)

	// zero-value options ≡ OutputTable, and should match RunQuery output
	var bufOpts bytes.Buffer
	if err := RunQueryWithOptions(gateFor(s), `select id`, &bufOpts, RunQueryOptions{}); err != nil {
		t.Fatal(err)
	}
	var bufDefault bytes.Buffer
	if err := RunQuery(gateFor(s), `select id`, &bufDefault); err != nil {
		t.Fatal(err)
	}
	if bufOpts.String() != bufDefault.String() {
		t.Errorf("zero options should equal RunQuery output\ngot:  %q\nwant: %q", bufOpts.String(), bufDefault.String())
	}
}

func TestRunQueryWithOptionsUpdatePartialFailureJSON(t *testing.T) {
	s := setupRunnerTest(t)
	_ = s.CreateTask(&task.Task{ID: "TIKI-CCC003", Title: "Third", Status: "ready", Priority: 3})
	fs := &failingUpdateStore{Store: s, failID: "TIKI-AAA001"}

	var buf bytes.Buffer
	err := RunQueryWithOptions(gateFor(fs), `update where status = "ready" set priority=5`, &buf,
		RunQueryOptions{OutputFormat: OutputJSON})
	if err == nil {
		t.Fatal("expected partial failure error")
	}
	// JSON summary should still be written before the error is returned
	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "{") || !strings.Contains(out, `"failed":1`) {
		t.Errorf("expected JSON summary with failed=1, got: %q", out)
	}
}

func TestRunQueryDeleteValidatorRejection(t *testing.T) {
	s := setupRunnerTest(t)

	g := service.NewTaskMutationGate()
	g.SetStore(s)
	g.OnDelete(func(_, _ *task.Task, _ []*task.Task) *service.Rejection {
		return &service.Rejection{Reason: "deletes forbidden"}
	})

	var buf bytes.Buffer
	err := RunQuery(g, `delete where id = "TIKI-AAA001"`, &buf)
	if err == nil {
		t.Fatal("expected error when delete is rejected by validator")
	}
	if !strings.Contains(err.Error(), "partially failed") {
		t.Errorf("expected 'partially failed' error, got: %v", err)
	}
	// task should still exist
	if s.GetTask("TIKI-AAA001") == nil {
		t.Error("task should not have been deleted")
	}
}
