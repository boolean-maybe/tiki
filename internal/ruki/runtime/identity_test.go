package runtime

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store/tikistore"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

// ensureStatusesLoaded installs the same status registry used by other runner
// tests so user()-based SELECTs and UPDATEs can validate against a real schema.
func ensureStatusesLoaded(t *testing.T) {
	t.Helper()
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "inProgress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
	})
}

// isolateConfigRuntime mirrors the tikistore test helper: it sandboxes cwd
// and XDG_CONFIG_HOME so identity env vars are the sole source of truth
// and the developer's local `config.yaml` cannot leak in.
func isolateConfigRuntime(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	config.ResetPathManager()
	t.Cleanup(config.ResetPathManager)
}

func newNoGitStoreWithIdentity(t *testing.T, name, email string) *tikistore.TikiStore {
	t.Helper()
	isolateConfigRuntime(t)
	t.Setenv("TIKI_STORE_GIT", "false")
	t.Setenv("TIKI_IDENTITY_NAME", name)
	t.Setenv("TIKI_IDENTITY_EMAIL", email)
	if _, err := config.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	tmpDir := t.TempDir()
	s, err := tikistore.NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	return s
}

func TestRunSelectQuery_UserFromConfigNoGit(t *testing.T) {
	ensureStatusesLoaded(t)
	s := newNoGitStoreWithIdentity(t, "Configured Alice", "alice@example.com")

	if err := s.CreateTask(&task.Task{
		ID: "TIKI-XYZ001", Title: "Mine", Status: "ready",
		Priority: 2, Assignee: "Configured Alice",
	}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := s.CreateTask(&task.Task{
		ID: "TIKI-XYZ002", Title: "Theirs", Status: "ready",
		Priority: 2, Assignee: "Bob",
	}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	var buf bytes.Buffer
	if err := RunSelectQuery(s, `select id where assignee = user()`, &buf); err != nil {
		t.Fatalf("RunSelectQuery: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "TIKI-XYZ001") {
		t.Errorf("expected TIKI-XYZ001 in output:\n%s", out)
	}
	if strings.Contains(out, "TIKI-XYZ002") {
		t.Errorf("TIKI-XYZ002 should be filtered out:\n%s", out)
	}
}

func TestRunQuery_UpdateAssigneeUserInNoGit(t *testing.T) {
	ensureStatusesLoaded(t)
	s := newNoGitStoreWithIdentity(t, "Configured Alice", "alice@example.com")

	if err := s.CreateTask(&task.Task{
		ID: "TIKI-UPD001", Title: "Assign me", Status: "ready", Priority: 2,
	}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	gate := service.NewTaskMutationGate()
	gate.SetStore(s)

	var buf bytes.Buffer
	err := RunQuery(gate, `update where id = "TIKI-UPD001" set assignee = user()`, &buf)
	if err != nil {
		t.Fatalf("RunQuery update: %v", err)
	}

	got := s.GetTask("TIKI-UPD001")
	if got == nil {
		t.Fatal("task TIKI-UPD001 not found after update")
	}
	if got.Assignee != "Configured Alice" {
		t.Errorf("assignee = %q, want 'Configured Alice'", got.Assignee)
	}
}

// TestRunQuery_UserEmailOnlyConfig exercises the fix for a contract mismatch
// between the identity resolver and resolveUserFunc: email-only config used
// to short-circuit the resolver chain but leave `user()` unavailable because
// the runner only projected `name`. Now email is promoted to name when name
// is empty, so user() returns the email string end-to-end.
func TestRunQuery_UserEmailOnlyConfig(t *testing.T) {
	ensureStatusesLoaded(t)
	isolateConfigRuntime(t)
	t.Setenv("TIKI_STORE_GIT", "false")
	t.Setenv("TIKI_IDENTITY_NAME", "")
	t.Setenv("TIKI_IDENTITY_EMAIL", "me@example.com")
	if _, err := config.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	tmpDir := t.TempDir()
	s, err := tikistore.NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	if err := s.CreateTask(&task.Task{
		ID: "TIKI-EML001", Title: "Mine", Status: "ready",
		Priority: 2, Assignee: "me@example.com",
	}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	var buf bytes.Buffer
	if err := RunSelectQuery(s, `select id where assignee = user()`, &buf); err != nil {
		t.Fatalf("RunSelectQuery: %v", err)
	}
	if !strings.Contains(buf.String(), "TIKI-EML001") {
		t.Errorf("expected TIKI-EML001 in output when user() resolves from email-only config:\n%s", buf.String())
	}
}

func TestRunSelectQuery_UserResolvesInNoGitNoConfig(t *testing.T) {
	ensureStatusesLoaded(t)
	isolateConfigRuntime(t)
	t.Setenv("TIKI_STORE_GIT", "false")
	t.Setenv("TIKI_IDENTITY_NAME", "")
	t.Setenv("TIKI_IDENTITY_EMAIL", "")
	if _, err := config.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	tmpDir := t.TempDir()
	s, err := tikistore.NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// On dev/CI hosts os/user.Current() resolves, so user() should succeed via
	// the OS fallback — not return the legacy "git is disabled" error.
	var buf bytes.Buffer
	err = RunSelectQuery(s, `select id where assignee = user()`, &buf)
	if err != nil && strings.Contains(err.Error(), "git is disabled") {
		t.Errorf("got legacy git-specific error after identity refactor: %v", err)
	}
}
