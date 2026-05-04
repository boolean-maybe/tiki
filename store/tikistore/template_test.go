package tikistore

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

// TestSetAuthorFromIdentity_EmailOnly_NoAngleEchoing reproduces a regression
// from the user()/display-string promotion work: when only identity.email is
// configured, currentUser returned (email, email) so author formatting
// produced `me@example.com <me@example.com>`. Attribution should just be the
// raw email in that case.
func TestSetAuthorFromIdentity_EmailOnly_NoAngleEchoing(t *testing.T) {
	isolateConfig(t)
	setIdentityEnv(t, "", "me@example.com")

	// build a store with no git util so attribution flows through the
	// identity resolver's config layer only
	s := &TikiStore{identity: newIdentityResolver(nil)}

	task := &taskpkg.Task{}
	s.setAuthorFromIdentity(task)

	if task.CreatedBy != "me@example.com" {
		t.Errorf("CreatedBy = %q, want 'me@example.com' (no angle form when only email is configured)", task.CreatedBy)
	}
}

func TestSetAuthorFromIdentity_NameAndEmail_UsesAngleForm(t *testing.T) {
	isolateConfig(t)
	setIdentityEnv(t, "Alice", "alice@example.com")

	s := &TikiStore{identity: newIdentityResolver(nil)}

	task := &taskpkg.Task{}
	s.setAuthorFromIdentity(task)

	if task.CreatedBy != "Alice <alice@example.com>" {
		t.Errorf("CreatedBy = %q, want 'Alice <alice@example.com>'", task.CreatedBy)
	}
}

func TestSetAuthorFromIdentity_NameOnly_UsesName(t *testing.T) {
	isolateConfig(t)
	setIdentityEnv(t, "Alice", "")

	s := &TikiStore{identity: newIdentityResolver(nil)}

	task := &taskpkg.Task{}
	s.setAuthorFromIdentity(task)

	if task.CreatedBy != "Alice" {
		t.Errorf("CreatedBy = %q, want 'Alice'", task.CreatedBy)
	}
}

func TestBuildCustomFieldDefaults_NoCustomFields(t *testing.T) {
	workflow.ClearCustomFields()
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	defaults := buildCustomFieldDefaults()
	if defaults != nil {
		t.Errorf("expected nil map when no custom fields, got %v", defaults)
	}
}

func TestBuildCustomFieldDefaults_WithDefaults(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "medium", "high"}, DefaultValue: "medium"},
		{Name: "blocked", Type: workflow.TypeBool, DefaultValue: false},
		{Name: "notes", Type: workflow.TypeString},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	defaults := buildCustomFieldDefaults()
	if defaults == nil {
		t.Fatal("expected non-nil defaults map")
	}
	if v, ok := defaults["severity"]; !ok || v != "medium" {
		t.Errorf("severity = %v, want \"medium\"", v)
	}
	if v, ok := defaults["blocked"]; !ok || v != false {
		t.Errorf("blocked = %v, want false", v)
	}
	if _, ok := defaults["notes"]; ok {
		t.Error("notes should not have a default")
	}
}

func TestBuildCustomFieldDefaults_SliceDefaultCopied(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	original := []string{"a", "b", " a ", "b", ""}
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "labels", Type: workflow.TypeListString, DefaultValue: original},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	defaults := buildCustomFieldDefaults()
	got, ok := defaults["labels"].([]string)
	if !ok {
		t.Fatal("expected []string default")
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("labels = %v, want [a b]", got)
	}
	// mutating the returned slice should not affect the original
	got[0] = "mutated"
	if original[0] == "mutated" {
		t.Error("returned slice should be a copy, not the original")
	}
}

// TestNewTaskTemplate_WorkflowCaptureWithDefaultStatus verifies that when the
// active workflow declares a `default: true` status, NewTaskTemplate produces
// a workflow-capable task with populated defaults. This is the historical
// behavior preserved for task-tracker workflows (kanban, todo, bug-tracker).
func TestNewTaskTemplate_WorkflowCaptureWithDefaultStatus(t *testing.T) {
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "backlog", Label: "Backlog", Default: true},
		{Key: "done", Label: "Done", Done: true},
	})
	t.Cleanup(func() {
		config.ResetStatusRegistry([]workflow.StatusDef{
			{Key: "backlog", Label: "Backlog", Default: true},
			{Key: "done", Label: "Done", Done: true},
		})
	})

	s, err := NewTikiStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tmpl, err := s.NewTaskTemplate()
	if err != nil {
		t.Fatalf("NewTaskTemplate: %v", err)
	}
	if !tmpl.IsWorkflow {
		t.Error("IsWorkflow = false, want true when default status is configured")
	}
	if tmpl.Status != "backlog" {
		t.Errorf("Status = %q, want %q", tmpl.Status, "backlog")
	}
	if tmpl.Priority == 0 {
		t.Error("Priority = 0, want populated workflow default")
	}
}

// TestNewTaskTemplate_PlainCaptureWithoutDefaultStatus verifies the Phase 7
// semantics: a workflow with no `default: true` status signals "capture as
// plain document." NewTaskTemplate returns a bare template with IsWorkflow=false
// and no workflow field defaults; the piped/ruki capture paths then persist a
// plain .md with only id+title in the frontmatter.
func TestNewTaskTemplate_PlainCaptureWithoutDefaultStatus(t *testing.T) {
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "alpha", Label: "Alpha"},
		{Key: "done", Label: "Done", Done: true},
	})
	t.Cleanup(func() {
		config.ResetStatusRegistry([]workflow.StatusDef{
			{Key: "backlog", Label: "Backlog", Default: true},
			{Key: "done", Label: "Done", Done: true},
		})
	})

	s, err := NewTikiStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tmpl, err := s.NewTaskTemplate()
	if err != nil {
		t.Fatalf("NewTaskTemplate: %v", err)
	}
	if tmpl.IsWorkflow {
		t.Error("IsWorkflow = true, want false when no default status configured")
	}
	if tmpl.Status != "" {
		t.Errorf("Status = %q, want empty", tmpl.Status)
	}
	if tmpl.Priority != 0 {
		t.Errorf("Priority = %d, want 0 (no workflow defaults on plain docs)", tmpl.Priority)
	}
	if tmpl.Points != 0 {
		t.Errorf("Points = %d, want 0 (no workflow defaults on plain docs)", tmpl.Points)
	}
	if len(tmpl.Tags) != 0 {
		t.Errorf("Tags = %v, want empty (no workflow defaults on plain docs)", tmpl.Tags)
	}
	if tmpl.ID == "" {
		t.Error("ID was not populated; plain capture must still generate an id")
	}
}

// TestCreateTask_HonorsPlainIsWorkflow verifies the end-to-end Phase 7 contract:
// a plain template produced by NewTaskTemplate survives CreateTask without
// being auto-promoted to a workflow item. This is the path piped capture uses.
func TestCreateTask_HonorsPlainIsWorkflow(t *testing.T) {
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "alpha", Label: "Alpha"},
		{Key: "done", Label: "Done", Done: true},
	})
	t.Cleanup(func() {
		config.ResetStatusRegistry([]workflow.StatusDef{
			{Key: "backlog", Label: "Backlog", Default: true},
			{Key: "done", Label: "Done", Done: true},
		})
	})

	s, err := NewTikiStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tmpl, err := s.NewTaskTemplate()
	if err != nil {
		t.Fatalf("NewTaskTemplate: %v", err)
	}
	tmpl.Title = "piped note"

	if err := s.CreateTask(tmpl); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	stored := s.GetTask(tmpl.ID)
	if stored == nil {
		t.Fatalf("GetTask returned nil after CreateTask(%s)", tmpl.ID)
	}
	if stored.IsWorkflow {
		t.Error("stored.IsWorkflow = true, want false — plain capture was promoted to workflow item")
	}
	// Phase 5: GetAllTasks includes plain docs; callers filter with has(status) / hasAnyWorkflowField.
	if got := len(s.GetAllTasks()); got != 1 {
		t.Errorf("GetAllTasks returned %d, want 1 (plain doc is included since Phase 5)", got)
	}
	if all := s.GetAllTasks(); all[0].IsWorkflow {
		t.Error("plain doc projected from GetAllTasks must not have IsWorkflow=true")
	}
}
